package clab

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTopologyFile(t *testing.T) {
	validYAML := `
name: test-lab
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
	tmpDir := t.TempDir()
	topoPath := filepath.Join(tmpDir, "test.clab.yaml")
	if err := os.WriteFile(topoPath, []byte(validYAML), 0644); err != nil {
		t.Fatal(err)
	}

	parsed, raw, err := ParseTopologyFile(topoPath)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if parsed.Name != "test-lab" {
		t.Errorf("expected name 'test-lab', got %q", parsed.Name)
	}
	if len(parsed.Topology.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(parsed.Topology.Nodes))
	}
	if len(parsed.Topology.Links) != 1 {
		t.Errorf("expected 1 link, got %d", len(parsed.Topology.Links))
	}
	if raw == "" {
		t.Error("expected non-empty raw YAML")
	}

	// Verify node fields
	srl1 := parsed.Topology.Nodes["srl1"]
	if srl1.Kind != "nokia_srlinux" || srl1.StartupConfig != "srl1.cfg" {
		t.Errorf("srl1 node attributes mismatch: %+v", srl1)
	}

	multitool := parsed.Topology.Nodes["multitool"]
	if multitool.Kind != "linux" || len(multitool.Exec) != 1 {
		t.Errorf("multitool node attributes mismatch: %+v", multitool)
	}
}

func TestParseTopologyValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "missing lab name",
			yaml: `
topology:
  nodes:
    node1:
      kind: linux
`,
			wantErr: true,
		},
		{
			name: "empty nodes",
			yaml: `
name: empty-lab
topology:
  nodes: {}
`,
			wantErr: true,
		},
		{
			name: "invalid node name uppercase",
			yaml: `
name: valid-lab
topology:
  nodes:
    Node1:
      kind: linux
`,
			wantErr: true,
		},
		{
			name: "invalid node name underscore",
			yaml: `
name: valid-lab
topology:
  nodes:
    node_1:
      kind: linux
`,
			wantErr: true,
		},
		{
			name: "invalid node name leading hyphen",
			yaml: `
name: valid-lab
topology:
  nodes:
    -node1:
      kind: linux
`,
			wantErr: true,
		},
		{
			name: "invalid node name trailing hyphen",
			yaml: `
name: valid-lab
topology:
  nodes:
    node1-:
      kind: linux
`,
			wantErr: true,
		},
		{
			name: "invalid node name too long",
			yaml: `
name: valid-lab
topology:
  nodes:
    this-node-name-is-way-too-long-and-exceeds-the-sixty-three-character-limit:
      kind: linux
`,
			wantErr: true,
		},
		{
			name: "pass-through link endpoints and undefined node in link allowed",
			yaml: `
name: passthrough-lab
topology:
  nodes:
    node1:
      kind: linux
  links:
    - endpoints: ["node1:eth1", "external:eth1"]
    - endpoints: ["single-endpoint"]
`,
			wantErr: false,
		},
		{
			name: "node without direct kind or image allowed (pass-through to defaults/kinds/groups)",
			yaml: `
name: passthrough-node
topology:
  defaults:
    kind: linux
  nodes:
    node1: {}
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "test.clab.yml")
			_ = os.WriteFile(path, []byte(tt.yaml), 0644)

			_, _, err := ParseTopologyFile(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTopologyFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseTopologyWithDefaultsKindsGroups(t *testing.T) {
	yamlContent := `
name: multi-tier-lab
topology:
  defaults:
    kind: linux
    image: alpine:latest
  kinds:
    nokia_srlinux:
      image: ghcr.io/nokia/srlinux:latest
      type: ixr6
  groups:
    spine:
      kind: nokia_srlinux
    leaf:
      kind: linux
  nodes:
    spine1:
      group: spine
    leaf1:
      group: leaf
    worker1: {}
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "topo.clab.yml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	parsed, _, err := ParseTopologyFile(path)
	if err != nil {
		t.Fatalf("unexpected error parsing topology with defaults, kinds, and groups: %v", err)
	}

	if parsed.Name != "multi-tier-lab" {
		t.Errorf("expected name 'multi-tier-lab', got %q", parsed.Name)
	}
	if parsed.Topology.Defaults == nil || parsed.Topology.Defaults.Kind != "linux" {
		t.Errorf("defaults not parsed properly: %+v", parsed.Topology.Defaults)
	}
	if len(parsed.Topology.Kinds) != 1 || parsed.Topology.Kinds["nokia_srlinux"].Type != "ixr6" {
		t.Errorf("kinds not parsed properly: %+v", parsed.Topology.Kinds)
	}
	if len(parsed.Topology.Groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(parsed.Topology.Groups))
	}
	if parsed.Topology.Nodes["spine1"].Group != "spine" {
		t.Errorf("expected spine1 group 'spine', got %q", parsed.Topology.Nodes["spine1"].Group)
	}
	if parsed.Topology.Nodes["leaf1"].Group != "leaf" {
		t.Errorf("expected leaf1 group 'leaf', got %q", parsed.Topology.Nodes["leaf1"].Group)
	}
}

func TestExpandMagicVariables(t *testing.T) {
	labName := "testlab"
	nodeName := "router1"

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "configs/__clabNodeName__.cfg",
			expected: "configs/router1.cfg",
		},
		{
			input:    "__clabDir__/configs/__clabNodeName__.cfg",
			expected: "clab-testlab/configs/router1.cfg",
		},
		{
			input:    "__clabNodeDir__/startup.cfg",
			expected: "clab-testlab/router1/startup.cfg",
		},
		{
			input:    "__clabLabName__-__clabNodeName__.cfg",
			expected: "testlab-router1.cfg",
		},
		{
			input:    "all: __clabNodeDir__ on __clabDir__ for __clabLabName__ and __clabNodeName__",
			expected: "all: clab-testlab/router1 on clab-testlab for testlab and router1",
		},
	}

	for _, tt := range tests {
		actual := ExpandMagicVariables(tt.input, labName, nodeName)
		if actual != tt.expected {
			t.Errorf("ExpandMagicVariables(%q) = %q, want %q", tt.input, actual, tt.expected)
		}
	}
}

func TestExpandGitVariables(t *testing.T) {
	origGitCmd := gitCmdFn
	defer func() { gitCmdFn = origGitCmd }()

	// 1. Mock git available with branch containing slashes
	gitCmdFn = func(dir string, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "rev-parse" && args[1] == "--abbrev-ref" {
			return []byte("Feature/Cool-Feature\n"), nil
		}
		if len(args) >= 2 && args[0] == "rev-parse" && args[1] == "--short" {
			return []byte("a1b2c3d\n"), nil
		}
		return nil, os.ErrNotExist
	}

	res := ExpandGitVariables("lab-__gitBranch__-__gitHash__", "/any/dir")
	expected := "lab-feature-cool-feature-a1b2c3d"
	if res != expected {
		t.Errorf("expected %q, got %q", expected, res)
	}

	// 2. Mock git unavailable (fallback to "none")
	gitCmdFn = func(dir string, args ...string) ([]byte, error) {
		return nil, os.ErrNotExist
	}

	resFallback := ExpandGitVariables("lab-__gitBranch__-__gitHash__", "/any/dir")
	expectedFallback := "lab-none-none"
	if resFallback != expectedFallback {
		t.Errorf("expected %q, got %q", expectedFallback, resFallback)
	}

	// 3. End-to-end via ParseTopologyFile
	tmpDir := t.TempDir()
	topoPath := filepath.Join(tmpDir, "topo.clab.yml")
	content := `
name: ci-__gitBranch__
topology:
  nodes:
    node1:
      kind: linux
`
	if err := os.WriteFile(topoPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	parsed, _, err := ParseTopologyFile(topoPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Name != "ci-none" {
		t.Errorf("expected name 'ci-none', got %q", parsed.Name)
	}
}
