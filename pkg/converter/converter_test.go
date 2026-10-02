package converter

import (
	"os"
	"path/filepath"
	"testing"

	"c9s/pkg/clab"

	"gopkg.in/yaml.v3"
)

func TestConvert(t *testing.T) {
	tmpDir := t.TempDir()

	cfgContent := "set / interface ethernet-1/1 admin-state enable\n"
	cfgFile := filepath.Join(tmpDir, "srl1.cfg")
	if err := os.WriteFile(cfgFile, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	topoYAML := `
name: my-lab
topology:
  nodes:
    srl1:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux:26.3
      startup-config: srl1.cfg
    multitool:
      kind: linux
      image: ghcr.io/srl-labs/network-multitool:latest
      exec:
        - ip addr add 192.0.2.1/31 dev eth1
  links:
    - endpoints: ["srl1:e1-1", "multitool:eth1"]
`
	topoFile := filepath.Join(tmpDir, "my-lab.clab.yml")
	if err := os.WriteFile(topoFile, []byte(topoYAML), 0644); err != nil {
		t.Fatal(err)
	}

	parsed, raw, err := clab.ParseTopologyFile(topoFile)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	res, err := Convert(parsed, tmpDir, raw)
	if err != nil {
		t.Fatalf("unexpected conversion error: %v", err)
	}

	// 1. Verify Namespace
	expectedNS := "lab-my-lab"
	if res.Namespace.Name != expectedNS {
		t.Errorf("expected namespace %q, got %q", expectedNS, res.Namespace.Name)
	}
	if res.Namespace.Labels["pod-security.kubernetes.io/enforce"] != "privileged" {
		t.Errorf("missing privileged pod-security label on namespace")
	}
	if res.Namespace.Labels["app.kubernetes.io/managed-by"] != "c9s" {
		t.Errorf("missing managed-by label on namespace")
	}

	// 2. Verify ConfigMap (single startup-configs ConfigMap with {node}-{filename} key)
	if len(res.ConfigMaps) != 1 {
		t.Fatalf("expected 1 ConfigMap, got %d", len(res.ConfigMaps))
	}
	cm := res.ConfigMaps[0]
	expectedCMName := "startup-configs"
	if cm.Name != expectedCMName {
		t.Errorf("expected CM name %q, got %q", expectedCMName, cm.Name)
	}
	if cm.Namespace != expectedNS {
		t.Errorf("expected CM namespace %q, got %q", expectedNS, cm.Namespace)
	}
	// Verify {node}-{filename} key
	expectedKey := "srl1-srl1.cfg"
	if val, ok := cm.Data[expectedKey]; !ok || val != cfgContent {
		t.Errorf("expected CM data to have key %q with content, got data: %+v", expectedKey, cm.Data)
	}

	// 3. Verify Topology CR
	if res.TopologyCR.APIVersion != "c9s.run/v1alpha1" {
		t.Errorf("expected APIVersion 'c9s.run/v1alpha1', got %q", res.TopologyCR.APIVersion)
	}
	if res.TopologyCR.Kind != "Topology" {
		t.Errorf("expected Kind 'Topology', got %q", res.TopologyCR.Kind)
	}
	if res.TopologyCR.Name != "my-lab" {
		t.Errorf("expected Topology name 'my-lab', got %q", res.TopologyCR.Name)
	}
	if res.TopologyCR.Namespace != expectedNS {
		t.Errorf("expected Topology namespace %q, got %q", expectedNS, res.TopologyCR.Namespace)
	}

	// Verify filesFromConfigMap
	files := res.TopologyCR.Spec.Deployment.FilesFromConfigMap["srl1"]
	if len(files) != 1 {
		t.Fatalf("expected 1 fileFromConfigMap for srl1, got %d", len(files))
	}
	f := files[0]
	if f.ConfigMapName != expectedCMName {
		t.Errorf("expected configMapName %q, got %q", expectedCMName, f.ConfigMapName)
	}
	if f.ConfigMapPath != expectedKey {
		t.Errorf("expected configMapPath %q, got %q", expectedKey, f.ConfigMapPath)
	}
	if f.FilePath != "srl1.cfg" {
		t.Errorf("expected filePath 'srl1.cfg', got %q", f.FilePath)
	}
	if f.Mode != "read" {
		t.Errorf("expected mode 'read', got %q", f.Mode)
	}

	// Verify resolved containerlab definition has startup-config set for srl1
	var topoDef map[string]interface{}
	if err := yaml.Unmarshal([]byte(res.TopologyCR.Spec.Definition.Containerlab), &topoDef); err != nil {
		t.Fatalf("failed to unmarshal resolved containerlab definition: %v", err)
	}
	topoMap, _ := topoDef["topology"].(map[string]interface{})
	nodesMap, _ := topoMap["nodes"].(map[string]interface{})
	srl1Node, _ := nodesMap["srl1"].(map[string]interface{})
	if srl1Node["startup-config"] != "srl1.cfg" {
		t.Errorf("expected srl1 startup-config 'srl1.cfg', got %v", srl1Node["startup-config"])
	}
}

func TestConvertChunking(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two 450 KB files (total ~900 KB > 800 KB limit)
	data1 := make([]byte, 450*1024)
	for i := range data1 {
		data1[i] = 'A'
	}
	data2 := make([]byte, 450*1024)
	for i := range data2 {
		data2[i] = 'B'
	}

	_ = os.WriteFile(filepath.Join(tmpDir, "node1.cfg"), data1, 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "node2.cfg"), data2, 0644)

	topoYAML := `
name: chunk-lab
topology:
  nodes:
    node1:
      kind: linux
      startup-config: node1.cfg
    node2:
      kind: linux
      startup-config: node2.cfg
`
	topoFile := filepath.Join(tmpDir, "chunk.clab.yml")
	_ = os.WriteFile(topoFile, []byte(topoYAML), 0644)

	parsed, raw, err := clab.ParseTopologyFile(topoFile)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	res, err := Convert(parsed, tmpDir, raw)
	if err != nil {
		t.Fatalf("unexpected conversion error: %v", err)
	}

	// Should be split into 2 ConfigMaps: startup-configs-1 and startup-configs-2
	if len(res.ConfigMaps) != 2 {
		t.Fatalf("expected 2 chunked ConfigMaps, got %d", len(res.ConfigMaps))
	}
	if res.ConfigMaps[0].Name != "startup-configs-1" || res.ConfigMaps[1].Name != "startup-configs-2" {
		t.Errorf("unexpected ConfigMap names: %s, %s", res.ConfigMaps[0].Name, res.ConfigMaps[1].Name)
	}

	// Verify filesFromConfigMap mappings point to correct chunks
	if res.TopologyCR.Spec.Deployment.FilesFromConfigMap["node1"][0].ConfigMapName != "startup-configs-1" {
		t.Errorf("node1 should point to startup-configs-1")
	}
	if res.TopologyCR.Spec.Deployment.FilesFromConfigMap["node2"][0].ConfigMapName != "startup-configs-2" {
		t.Errorf("node2 should point to startup-configs-2")
	}
}

func TestConvertInheritanceAndMagicVariables(t *testing.T) {
	tmpDir := t.TempDir()
	configsDir := filepath.Join(tmpDir, "configs")
	if err := os.MkdirAll(configsDir, 0755); err != nil {
		t.Fatal(err)
	}

	spineS1Cfg := "spine s1 config\n"
	leafL1Cfg := "leaf l1 config\n"
	defaultH1Cfg := "default h1 config\n"
	customCfg := "custom node config\n"

	if err := os.WriteFile(filepath.Join(configsDir, "spine-s1.cfg"), []byte(spineS1Cfg), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configsDir, "testlab-srl-l1.cfg"), []byte(leafL1Cfg), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configsDir, "default-h1.cfg"), []byte(defaultH1Cfg), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "custom.cfg"), []byte(customCfg), 0644); err != nil {
		t.Fatal(err)
	}

	topoYAML := `
name: testlab
topology:
  defaults:
    kind: linux
    startup-config: configs/default-__clabNodeName__.cfg
  kinds:
    nokia_srlinux:
      startup-config: configs/__clabLabName__-srl-__clabNodeName__.cfg
  groups:
    spines:
      kind: nokia_srlinux
      startup-config: configs/spine-__clabNodeName__.cfg
    leaves:
      kind: nokia_srlinux
  nodes:
    s1:
      group: spines
    l1:
      group: leaves
    h1: {}
    custom:
      startup-config: custom.cfg
    inline-node:
      startup-config: |
        hostname __clabNodeName__
        lab __clabLabName__
`
	topoFile := filepath.Join(tmpDir, "topo.clab.yml")
	if err := os.WriteFile(topoFile, []byte(topoYAML), 0644); err != nil {
		t.Fatal(err)
	}

	parsed, raw, err := clab.ParseTopologyFile(topoFile)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	res, err := Convert(parsed, tmpDir, raw)
	if err != nil {
		t.Fatalf("unexpected conversion error: %v", err)
	}

	if len(res.ConfigMaps) != 1 {
		t.Fatalf("expected 1 ConfigMap, got %d", len(res.ConfigMaps))
	}
	cm := res.ConfigMaps[0]

	// Verify s1 inherited from group 'spines' (single resolved path)
	if val, ok := cm.Data["s1-spine-s1.cfg"]; !ok || val != spineS1Cfg {
		t.Errorf("s1 data mismatch: %v", val)
	}
	s1Files := res.TopologyCR.Spec.Deployment.FilesFromConfigMap["s1"]
	if len(s1Files) != 1 {
		t.Fatalf("expected 1 fileFromConfigMap entry for s1, got %d", len(s1Files))
	}
	if s1Files[0].FilePath != "configs/spine-s1.cfg" || s1Files[0].ConfigMapPath != "s1-spine-s1.cfg" {
		t.Errorf("s1 file mapping mismatch: %+v", s1Files[0])
	}
	if parsed.Topology.Nodes["s1"].Kind != "nokia_srlinux" {
		t.Errorf("expected s1 kind 'nokia_srlinux', got %q", parsed.Topology.Nodes["s1"].Kind)
	}

	// Verify l1 inherited from kinds['nokia_srlinux'] via group 'leaves' kind (single resolved path)
	if val, ok := cm.Data["l1-testlab-srl-l1.cfg"]; !ok || val != leafL1Cfg {
		t.Errorf("l1 data mismatch: %v", val)
	}
	l1Files := res.TopologyCR.Spec.Deployment.FilesFromConfigMap["l1"]
	if len(l1Files) != 1 {
		t.Fatalf("expected 1 fileFromConfigMap entry for l1, got %d", len(l1Files))
	}
	if l1Files[0].FilePath != "configs/testlab-srl-l1.cfg" || l1Files[0].ConfigMapPath != "l1-testlab-srl-l1.cfg" {
		t.Errorf("l1 file mapping mismatch: %+v", l1Files[0])
	}
	if parsed.Topology.Nodes["l1"].Kind != "nokia_srlinux" {
		t.Errorf("expected l1 kind 'nokia_srlinux', got %q", parsed.Topology.Nodes["l1"].Kind)
	}

	// Verify h1 inherited from defaults (single resolved path)
	if val, ok := cm.Data["h1-default-h1.cfg"]; !ok || val != defaultH1Cfg {
		t.Errorf("h1 data mismatch: %v", val)
	}
	h1Files := res.TopologyCR.Spec.Deployment.FilesFromConfigMap["h1"]
	if len(h1Files) != 1 {
		t.Fatalf("expected 1 fileFromConfigMap entry for h1, got %d", len(h1Files))
	}
	if h1Files[0].FilePath != "configs/default-h1.cfg" || h1Files[0].ConfigMapPath != "h1-default-h1.cfg" {
		t.Errorf("h1 file mapping mismatch: %+v", h1Files[0])
	}
	if parsed.Topology.Nodes["h1"].Kind != "linux" {
		t.Errorf("expected h1 kind 'linux', got %q", parsed.Topology.Nodes["h1"].Kind)
	}

	// Verify custom node overrides startup-config directly without magic variables (1 entry)
	if val, ok := cm.Data["custom-custom.cfg"]; !ok || val != customCfg {
		t.Errorf("custom data mismatch: %v", val)
	}
	customFiles := res.TopologyCR.Spec.Deployment.FilesFromConfigMap["custom"]
	if len(customFiles) != 1 || customFiles[0].FilePath != "custom.cfg" || customFiles[0].ConfigMapPath != "custom-custom.cfg" {
		t.Errorf("custom file mapping mismatch: %+v", customFiles)
	}

	// Verify inline-node expands magic variables without template path (1 entry)
	expectedInline := "hostname inline-node\nlab testlab\n"
	if val, ok := cm.Data["inline-node-inline-node.cfg"]; !ok || val != expectedInline {
		t.Errorf("inline-node data mismatch: expected %q, got %q", expectedInline, val)
	}
	inlineFiles := res.TopologyCR.Spec.Deployment.FilesFromConfigMap["inline-node"]
	if len(inlineFiles) != 1 || inlineFiles[0].FilePath != "inline-node.cfg" || inlineFiles[0].ConfigMapPath != "inline-node-inline-node.cfg" {
		t.Errorf("inline-node file mapping mismatch: %+v", inlineFiles)
	}

	// Verify each node has exactly 1 fileFromConfigMap entry
	for nodeName, fList := range res.TopologyCR.Spec.Deployment.FilesFromConfigMap {
		if len(fList) != 1 {
			t.Errorf("expected exactly 1 fileFromConfigMap entry for node %q, got %d", nodeName, len(fList))
		}
	}

	// Verify resolved containerlab YAML
	var clabDoc map[string]interface{}
	if err := yaml.Unmarshal([]byte(res.TopologyCR.Spec.Definition.Containerlab), &clabDoc); err != nil {
		t.Fatalf("failed to unmarshal resolved containerlab YAML: %v", err)
	}
	topo, ok := clabDoc["topology"].(map[string]interface{})
	if !ok {
		t.Fatalf("topology map missing in resolved containerlab YAML")
	}

	// Verify defaults, kinds, and groups do not contain startup-config
	if defs, ok := topo["defaults"].(map[string]interface{}); ok {
		if _, exists := defs["startup-config"]; exists {
			t.Errorf("topology.defaults should not have startup-config key")
		}
	}
	if kinds, ok := topo["kinds"].(map[string]interface{}); ok {
		if srl, ok := kinds["nokia_srlinux"].(map[string]interface{}); ok {
			if _, exists := srl["startup-config"]; exists {
				t.Errorf("topology.kinds.nokia_srlinux should not have startup-config key")
			}
		}
	}
	if groups, ok := topo["groups"].(map[string]interface{}); ok {
		if spines, ok := groups["spines"].(map[string]interface{}); ok {
			if _, exists := spines["startup-config"]; exists {
				t.Errorf("topology.groups.spines should not have startup-config key")
			}
		}
	}

	// Verify each node has resolved startup-config set
	nodes, ok := topo["nodes"].(map[string]interface{})
	if !ok {
		t.Fatalf("topology.nodes map missing in resolved containerlab YAML")
	}

	expectedConfigs := map[string]string{
		"s1":          "configs/spine-s1.cfg",
		"l1":          "configs/testlab-srl-l1.cfg",
		"h1":          "configs/default-h1.cfg",
		"custom":      "custom.cfg",
		"inline-node": "inline-node.cfg",
	}

	for nodeName, expectedCfg := range expectedConfigs {
		nodeMap, ok := nodes[nodeName].(map[string]interface{})
		if !ok {
			t.Errorf("node %q not found or not a map in resolved YAML", nodeName)
			continue
		}
		if nodeMap["startup-config"] != expectedCfg {
			t.Errorf("node %q expected startup-config %q, got %v", nodeName, expectedCfg, nodeMap["startup-config"])
		}
	}
}
