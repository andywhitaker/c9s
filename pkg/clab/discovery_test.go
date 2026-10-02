package clab

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindTopologyFile(t *testing.T) {
	t.Run("explicit file path provided", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "lab.clab.yml")
		_ = os.WriteFile(target, []byte("name: test"), 0644)

		found, err := FindTopologyFile(target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found != target {
			t.Errorf("expected %q, got %q", target, found)
		}
	})

	t.Run("auto-find in directory with one file", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "my-topo.clab.yaml")
		_ = os.WriteFile(target, []byte("name: test"), 0644)

		found, err := FindTopologyFile(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found != target {
			t.Errorf("expected %q, got %q", target, found)
		}
	})

	t.Run("zero files found error", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := FindTopologyFile(tmpDir)
		if err == nil {
			t.Fatal("expected error when no topology file is present, got nil")
		}
	})

	t.Run("ambiguous multiple files error", func(t *testing.T) {
		tmpDir := t.TempDir()
		_ = os.WriteFile(filepath.Join(tmpDir, "lab1.clab.yml"), []byte("name: 1"), 0644)
		_ = os.WriteFile(filepath.Join(tmpDir, "lab2.clab.yaml"), []byte("name: 2"), 0644)

		_, err := FindTopologyFile(tmpDir)
		if err == nil {
			t.Fatal("expected error for multiple topology files, got nil")
		}
	})
}
