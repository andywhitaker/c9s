package safeguards

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateLabName(t *testing.T) {
	tests := []struct {
		name    string
		labName string
		wantErr bool
	}{
		{"valid simple", "mylab", false},
		{"valid with hyphen and numbers", "srl-lab-01", false},
		{"valid single char", "a", false},
		{"empty name", "", true},
		{"uppercase rejected", "MyLab", true},
		{"underscore rejected", "my_lab", true},
		{"dots rejected", "my.lab", true},
		{"starts with hyphen rejected", "-mylab", true},
		{"ends with hyphen rejected", "mylab-", true},
		{"leading whitespace rejected", " mylab", true},
		{"trailing whitespace rejected", "mylab ", true},
		{"leading and trailing whitespace rejected", "  mylab  ", true},
		{"whitespace only rejected", "   ", true},
		{"too long (>59 chars)", "this-lab-name-is-definitely-way-too-long-to-be-a-valid-k8s-c9s-lab", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLabName(tt.labName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateLabName(%q) error = %v, wantErr %v", tt.labName, err, tt.wantErr)
			}
		})
	}
}

func TestDeriveNamespace(t *testing.T) {
	tests := []struct {
		name    string
		labName string
		wantNS  string
		wantErr bool
	}{
		{"valid lab", "srl-multitool", "lab-srl-multitool", false},
		{"valid lab 2", "demo", "lab-demo", false},
		{"invalid lab name", "DEMO", "", true},
		{"leading whitespace rejected", " mylab", "", true},
		{"trailing whitespace rejected", "mylab ", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, err := DeriveNamespace(tt.labName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DeriveNamespace(%q) error = %v, wantErr %v", tt.labName, err, tt.wantErr)
			}
			if ns != tt.wantNS {
				t.Errorf("DeriveNamespace(%q) = %q, want %q", tt.labName, ns, tt.wantNS)
			}
		})
	}
}

func TestEnforceNamespaceSafety(t *testing.T) {
	tests := []struct {
		namespace string
		wantErr   bool
	}{
		{"default", true},
		{"kube-system", true},
		{"kube-public", true},
		{"lab-mylab", false},
		{"lab-srl01", false},
		{"random-ns", true},
		{"lab-", true}, // too short
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			err := EnforceNamespaceSafety(tt.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnforceNamespaceSafety(%q) error = %v, wantErr %v", tt.namespace, err, tt.wantErr)
			}
		})
	}
}

func TestSafeReadConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	validFile := filepath.Join(tmpDir, "srl1.cfg")
	if err := os.WriteFile(validFile, []byte("set / interface ethernet-1/1 admin-state enable\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Valid file read
	data, err := SafeReadConfigFile(tmpDir, "srl1.cfg")
	if err != nil {
		t.Fatalf("expected successful read, got %v", err)
	}
	if string(data) != "set / interface ethernet-1/1 admin-state enable\n" {
		t.Fatalf("unexpected content: %s", string(data))
	}

	// 2. Relative subdir read
	subDir := filepath.Join(tmpDir, "configs")
	_ = os.Mkdir(subDir, 0755)
	subFile := filepath.Join(subDir, "router.cfg")
	_ = os.WriteFile(subFile, []byte("router config\n"), 0644)

	data, err = SafeReadConfigFile(tmpDir, "configs/router.cfg")
	if err != nil {
		t.Fatalf("expected successful read of subfile, got %v", err)
	}
	if string(data) != "router config\n" {
		t.Fatalf("unexpected content: %s", string(data))
	}

	// 3. Path traversal attack blocked
	_, err = SafeReadConfigFile(tmpDir, "../outside.cfg")
	if err == nil {
		t.Fatal("expected error for path traversal ../outside.cfg, but got none")
	}

	_, err = SafeReadConfigFile(tmpDir, "../../../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal to /etc/passwd, but got none")
	}

	// 4. Non-existent file
	_, err = SafeReadConfigFile(tmpDir, "nonexistent.cfg")
	if err == nil {
		t.Fatal("expected error for non-existent file, but got none")
	}
}
