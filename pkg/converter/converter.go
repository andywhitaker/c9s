package converter

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"c9s/pkg/clab"
	"c9s/pkg/crd"
	"c9s/pkg/safeguards"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Result holds all Kubernetes resources generated from a containerlab topology.
type Result struct {
	LabName    string
	Namespace  *corev1.Namespace
	ConfigMaps []*corev1.ConfigMap
	TopologyCR *crd.Topology
}

// Convert transforms a parsed containerlab topology and raw YAML into Kubernetes resources.
func Convert(parsed *clab.Topology, topoDir, rawYaml string) (*Result, error) {
	labName := parsed.Name
	nsName, err := safeguards.DeriveNamespace(labName)
	if err != nil {
		return nil, err
	}

	// 1. Target Namespace
	ns := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			Labels: map[string]string{
				"pod-security.kubernetes.io/enforce": "privileged",
				"app.kubernetes.io/name":             "c9s",
				"app.kubernetes.io/managed-by":       "c9s",
				"c9s.dev/lab-name":                   labName,
				"c9s.run/lab":                        labName,
			},
		},
	}

	// 2. Process startup configs with {node}-{filename} keys and automatic size chunking
	const maxConfigMapBytes = 800 * 1024 // 800 KB safe ceiling below 1MB etcd limit

	type configEntry struct {
		nodeName string
		key      string
		filePath string
		data     []byte
	}

	var entries []configEntry
	resolvedConfigs := make(map[string]string)
	totalBytes := 0

	// Sort node names deterministically
	var sortedNodes []string
	for nodeName := range parsed.Topology.Nodes {
		sortedNodes = append(sortedNodes, nodeName)
	}
	sort.Strings(sortedNodes)

	for _, nodeName := range sortedNodes {
		node := parsed.Topology.Nodes[nodeName]
		if node == nil {
			continue
		}

		// 1. Resolve effective kind
		effectiveKind := ""
		if node.Kind != "" {
			effectiveKind = node.Kind
		} else if node.Group != "" && parsed.Topology.Groups != nil && parsed.Topology.Groups[node.Group] != nil && parsed.Topology.Groups[node.Group].Kind != "" {
			effectiveKind = parsed.Topology.Groups[node.Group].Kind
		} else if parsed.Topology.Defaults != nil && parsed.Topology.Defaults.Kind != "" {
			effectiveKind = parsed.Topology.Defaults.Kind
		}

		if node.Kind == "" && effectiveKind != "" {
			node.Kind = effectiveKind
		}

		// 2. Resolve effective startup-config
		effectiveStartupConfig := ""
		if node.StartupConfig != "" {
			effectiveStartupConfig = node.StartupConfig
		} else if node.Group != "" && parsed.Topology.Groups != nil && parsed.Topology.Groups[node.Group] != nil && parsed.Topology.Groups[node.Group].StartupConfig != "" {
			effectiveStartupConfig = parsed.Topology.Groups[node.Group].StartupConfig
		} else if effectiveKind != "" && parsed.Topology.Kinds != nil && parsed.Topology.Kinds[effectiveKind] != nil && parsed.Topology.Kinds[effectiveKind].StartupConfig != "" {
			effectiveStartupConfig = parsed.Topology.Kinds[effectiveKind].StartupConfig
		} else if parsed.Topology.Defaults != nil && parsed.Topology.Defaults.StartupConfig != "" {
			effectiveStartupConfig = parsed.Topology.Defaults.StartupConfig
		}

		if effectiveStartupConfig == "" {
			continue
		}

		// Expand magic variables in the startup-config
		expandedConfig := clab.ExpandMagicVariables(effectiveStartupConfig, parsed.Name, nodeName)

		var data []byte
		var filename string
		var filePath string

		if strings.Contains(expandedConfig, "\n") {
			// Inline configuration multiline string
			data = []byte(expandedConfig)
			filename = fmt.Sprintf("%s.cfg", nodeName)
			filePath = filename
		} else {
			// External file path
			var err error
			data, err = safeguards.SafeReadConfigFile(topoDir, expandedConfig)
			if err != nil {
				return nil, fmt.Errorf("node %q startup-config error: %w", nodeName, err)
			}
			filename = filepath.Base(expandedConfig)
			filePath = expandedConfig
		}

		if len(data) > maxConfigMapBytes {
			return nil, fmt.Errorf("node %q startup-config (%d bytes) exceeds single ConfigMap limit (%d bytes)",
				nodeName, len(data), maxConfigMapBytes)
		}

		key := fmt.Sprintf("%s-%s", nodeName, filename)

		entries = append(entries, configEntry{
			nodeName: nodeName,
			key:      key,
			filePath: filePath,
			data:     data,
		})
		resolvedConfigs[nodeName] = filePath
		totalBytes += len(data)
	}

	var configMaps []*corev1.ConfigMap
	filesFromCM := make(map[string][]crd.FileFromConfigMap)

	if len(entries) > 0 {
		// Determine if we need chunking or a single ConfigMap
		if totalBytes <= maxConfigMapBytes {
			// Single ConfigMap: "startup-configs"
			cmName := "startup-configs"
			cmData := make(map[string]string)
			for _, entry := range entries {
				cmData[entry.key] = string(entry.data)
				filesFromCM[entry.nodeName] = append(filesFromCM[entry.nodeName], crd.FileFromConfigMap{
					FilePath:      entry.filePath,
					ConfigMapName: cmName,
					ConfigMapPath: entry.key,
					Mode:          "read",
				})
			}

			configMaps = append(configMaps, &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      cmName,
					Namespace: nsName,
					Labels: map[string]string{
						"app.kubernetes.io/name":       "c9s",
						"app.kubernetes.io/managed-by": "c9s",
						"c9s.dev/lab-name":             labName,
						"c9s.run/lab":                  labName,
					},
				},
				Data: cmData,
			})
		} else {
			// Total size exceeds 800KB: automatically split across startup-configs-1, startup-configs-2, etc.
			chunkIndex := 1
			currentSize := 0
			currentData := make(map[string]string)
			var currentChunkEntries []configEntry

			flushChunk := func() {
				if len(currentData) == 0 {
					return
				}
				cmName := fmt.Sprintf("startup-configs-%d", chunkIndex)
				for _, entry := range currentChunkEntries {
					filesFromCM[entry.nodeName] = append(filesFromCM[entry.nodeName], crd.FileFromConfigMap{
						FilePath:      entry.filePath,
						ConfigMapName: cmName,
						ConfigMapPath: entry.key,
						Mode:          "read",
					})
				}
				configMaps = append(configMaps, &corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      cmName,
						Namespace: nsName,
						Labels: map[string]string{
							"app.kubernetes.io/name":       "c9s",
							"app.kubernetes.io/managed-by": "c9s",
							"c9s.dev/lab-name":             labName,
							"c9s.run/lab":                  labName,
						},
					},
					Data: currentData,
				})
				chunkIndex++
				currentSize = 0
				currentData = make(map[string]string)
				currentChunkEntries = nil
			}

			for _, entry := range entries {
				if currentSize+len(entry.data) > maxConfigMapBytes && len(currentData) > 0 {
					flushChunk()
				}
				currentData[entry.key] = string(entry.data)
				currentChunkEntries = append(currentChunkEntries, entry)
				currentSize += len(entry.data)
			}
			flushChunk()
		}
	}

	resolvedYaml := resolveContainerlabYaml(rawYaml, resolvedConfigs)

	// 3. Topology Custom Resource
	topoCR := &crd.Topology{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", crd.GroupName, crd.GroupVersion),
			Kind:       crd.KindTopology,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      labName,
			Namespace: nsName,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "c9s",
				"app.kubernetes.io/managed-by": "c9s",
				"c9s.dev/lab-name":             labName,
				"c9s.run/lab":                  labName,
			},
		},
		Spec: crd.TopologySpec{
			Definition: crd.TopologyDefinitionSpec{
				Containerlab: resolvedYaml,
			},
			Deployment: crd.TopologyDeploymentSpec{
				FilesFromConfigMap: filesFromCM,
			},
		},
	}

	return &Result{
		LabName:    labName,
		Namespace:  ns,
		ConfigMaps: configMaps,
		TopologyCR: topoCR,
	}, nil
}

func findMapValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func setMapKey(node *yaml.Node, key, value string) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1].Kind = yaml.ScalarNode
			node.Content[i+1].Tag = "!!str"
			node.Content[i+1].Value = value
			return
		}
	}
	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: key,
	}
	valNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: value,
	}
	node.Content = append(node.Content, keyNode, valNode)
}

func deleteMapKey(node *yaml.Node, key string) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	newContent := make([]*yaml.Node, 0, len(node.Content))
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			continue
		}
		newContent = append(newContent, node.Content[i], node.Content[i+1])
	}
	node.Content = newContent
}

func resolveContainerlabYaml(rawYaml string, resolvedConfigs map[string]string) string {
	if len(resolvedConfigs) == 0 {
		return rawYaml
	}

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(rawYaml), &doc); err != nil {
		return rawYaml
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return rawYaml
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return rawYaml
	}

	topoMap := findMapValue(root, "topology")
	if topoMap == nil || topoMap.Kind != yaml.MappingNode {
		return rawYaml
	}

	// 1. Update or set startup-config for each node in topology -> nodes
	nodesMap := findMapValue(topoMap, "nodes")
	if nodesMap != nil && nodesMap.Kind == yaml.MappingNode {
		for i := 0; i < len(nodesMap.Content)-1; i += 2 {
			nodeKey := nodesMap.Content[i]
			nodeVal := nodesMap.Content[i+1]
			nodeName := nodeKey.Value
			if cfg, ok := resolvedConfigs[nodeName]; ok {
				if nodeVal.Kind != yaml.MappingNode {
					nodeVal.Kind = yaml.MappingNode
					nodeVal.Tag = "!!map"
					nodeVal.Content = nil
				}
				setMapKey(nodeVal, "startup-config", cfg)
			}
		}
	}

	// 2. Clean up any template/inherited startup-config keys under defaults, kinds, groups
	if defaultsMap := findMapValue(topoMap, "defaults"); defaultsMap != nil {
		deleteMapKey(defaultsMap, "startup-config")
	}

	if kindsMap := findMapValue(topoMap, "kinds"); kindsMap != nil && kindsMap.Kind == yaml.MappingNode {
		for i := 1; i < len(kindsMap.Content); i += 2 {
			kindVal := kindsMap.Content[i]
			deleteMapKey(kindVal, "startup-config")
		}
	}

	if groupsMap := findMapValue(topoMap, "groups"); groupsMap != nil && groupsMap.Kind == yaml.MappingNode {
		for i := 1; i < len(groupsMap.Content); i += 2 {
			groupVal := groupsMap.Content[i]
			deleteMapKey(groupVal, "startup-config")
		}
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return rawYaml
	}
	_ = enc.Close()

	return buf.String()
}
