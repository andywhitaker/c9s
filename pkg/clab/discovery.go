package clab

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindTopologyFile finds a topology file or resolves the given path.
func FindTopologyFile(topoFlag string) (string, error) {
	if topoFlag != "" {
		absPath, err := filepath.Abs(topoFlag)
		if err != nil {
			return "", fmt.Errorf("failed to resolve topology path %q: %w", topoFlag, err)
		}

		info, err := os.Stat(absPath)
		if err != nil {
			return "", fmt.Errorf("topology file not found: %s", topoFlag)
		}

		if info.IsDir() {
			return findInDir(absPath)
		}

		return absPath, nil
	}

	// Look in current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	return findInDir(cwd)
}

func findInDir(dir string) (string, error) {
	patterns := []string{"*.clab.yml", "*.clab.yaml"}
	var matches []string

	for _, pattern := range patterns {
		found, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			continue
		}
		matches = append(matches, found...)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no containerlab topology (*.clab.yml or *.clab.yaml) found in %s; specify with -t/--topo", dir)
	}

	if len(matches) > 1 {
		var names []string
		for _, m := range matches {
			names = append(names, filepath.Base(m))
		}
		return "", fmt.Errorf("multiple topology files found (%s); use -t/--topo to select one", strings.Join(names, ", "))
	}

	return matches[0], nil
}
