package converter

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"c9s/pkg/clab"
	"c9s/pkg/crd"
	"c9s/pkg/safeguards"

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
	totalBytes := 0

	// Sort node names deterministically
	var sortedNodes []string
	for nodeName, node := range parsed.Topology.Nodes {
		if node != nil && node.StartupConfig != "" {
			sortedNodes = append(sortedNodes, nodeName)
		}
	}
	sort.Strings(sortedNodes)

	for _, nodeName := range sortedNodes {
		node := parsed.Topology.Nodes[nodeName]
		var data []byte
		var filename string
		var filePath string

		if strings.Contains(node.StartupConfig, "\n") {
			// Inline configuration multiline string
			data = []byte(node.StartupConfig)
			filename = fmt.Sprintf("%s.cfg", nodeName)
			filePath = filename
		} else {
			// External file path
			var err error
			data, err = safeguards.SafeReadConfigFile(topoDir, node.StartupConfig)
			if err != nil {
				return nil, fmt.Errorf("node %q startup-config error: %w", nodeName, err)
			}
			filename = filepath.Base(node.StartupConfig)
			filePath = node.StartupConfig
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
				Containerlab: rawYaml,
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
