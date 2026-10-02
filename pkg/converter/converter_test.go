package converter

import (
	"os"
	"path/filepath"
	"testing"

	"c9s/pkg/clab"
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

	// Verify raw containerlab definition is preserved
	if res.TopologyCR.Spec.Definition.Containerlab != raw {
		t.Errorf("expected raw containerlab definition preserved in Topology CR")
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
