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
			name: "invalid link endpoints",
			yaml: `
name: bad-link
topology:
  nodes:
    node1:
      kind: linux
  links:
    - endpoints: ["node1:eth1"]
`,
			wantErr: true,
		},
		{
			name: "undefined node in link",
			yaml: `
name: ghost-node
topology:
  nodes:
    node1:
      kind: linux
  links:
    - endpoints: ["node1:eth1", "ghost:eth1"]
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
