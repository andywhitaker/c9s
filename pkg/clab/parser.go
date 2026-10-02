package clab

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"c9s/pkg/safeguards"
	"gopkg.in/yaml.v3"
)

const (
	MaxTopoFileSizeBytes = 1 * 1024 * 1024 // 1 MB
	MaxYamlAliases       = 50
	MaxYamlDepth         = 25
	MaxNodeNameLength    = 63
)

var (
	// NodeNameRegex enforces Kubernetes DNS-1035 label syntax:
	// lowercase alphanumeric characters and '-', starting and ending with an alphanumeric character, max 63 chars.
	NodeNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)
)

// ParseTopologyFile reads, validates, and parses a containerlab topology YAML file.
func ParseTopologyFile(path string) (*Topology, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open topology file %q: %w", path, err)
	}
	defer f.Close()

	limitedReader := io.LimitReader(f, MaxTopoFileSizeBytes+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read topology file %q: %w", path, err)
	}
	if int64(len(data)) > MaxTopoFileSizeBytes {
		return nil, "", fmt.Errorf("topology file %q exceeds maximum allowable size of %d bytes", path, MaxTopoFileSizeBytes)
	}

	// Inspect YAML AST for safety (aliases and depth)
	var root yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&root); err != nil {
		return nil, "", fmt.Errorf("YAML syntax error in %q: %w", path, err)
	}

	aliasCount := 0
	if err := inspectNode(&root, 0, &aliasCount); err != nil {
		return nil, "", fmt.Errorf("security violation in YAML structure: %w", err)
	}

	var topo Topology
	if err := root.Decode(&topo); err != nil {
		return nil, "", fmt.Errorf("failed to decode containerlab topology in %q: %w", path, err)
	}

	topoDir := filepath.Dir(path)
	topo.Name = ExpandGitVariables(topo.Name, topoDir)

	// Validate Topology
	if err := validateTopology(&topo); err != nil {
		return nil, "", err
	}

	return &topo, string(data), nil
}

func inspectNode(node *yaml.Node, currentDepth int, aliasCount *int) error {
	if currentDepth > MaxYamlDepth {
		return fmt.Errorf("YAML nesting depth exceeds safe limit of %d", MaxYamlDepth)
	}
	if node.Kind == yaml.AliasNode {
		*aliasCount++
		if *aliasCount > MaxYamlAliases {
			return fmt.Errorf("YAML alias count exceeds safe limit of %d (possible entity expansion attack)", MaxYamlAliases)
		}
	}
	for _, child := range node.Content {
		if err := inspectNode(child, currentDepth+1, aliasCount); err != nil {
			return err
		}
	}
	return nil
}

var gitCmdFn = func(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.Output()
}

// ExpandMagicVariables replaces containerlab magic variables with their runtime values.
func ExpandMagicVariables(s string, labName, nodeName string) string {
	clabDir := fmt.Sprintf("clab-%s", labName)
	clabNodeDir := fmt.Sprintf("clab-%s/%s", labName, nodeName)

	replacer := strings.NewReplacer(
		"__clabNodeDir__", clabNodeDir,
		"__clabDir__", clabDir,
		"__clabNodeName__", nodeName,
		"__clabLabName__", labName,
	)
	return replacer.Replace(s)
}

// ExpandGitVariables expands __gitBranch__ and __gitHash__ in the given string using git if available.
func ExpandGitVariables(name string, dir string) string {
	if !strings.Contains(name, "__gitBranch__") && !strings.Contains(name, "__gitHash__") {
		return name
	}

	branch := "none"
	hash := "none"

	bOut, err := gitCmdFn(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		b := strings.TrimSpace(string(bOut))
		if b != "" && b != "HEAD" {
			b = strings.ReplaceAll(b, "/", "-")
			branch = strings.ToLower(b)
		} else if b == "HEAD" {
			branch = "head"
		}
	}

	hOut, err := gitCmdFn(dir, "rev-parse", "--short", "HEAD")
	if err == nil {
		h := strings.TrimSpace(string(hOut))
		if h != "" {
			hash = strings.ToLower(h)
		}
	}

	name = strings.ReplaceAll(name, "__gitBranch__", branch)
	name = strings.ReplaceAll(name, "__gitHash__", hash)
	return name
}

func validateTopology(t *Topology) error {
	if t.Name == "" {
		return errors.New("topology name is required (missing 'name' field)")
	}

	if err := safeguards.ValidateLabName(t.Name); err != nil {
		return err
	}

	if len(t.Topology.Nodes) == 0 {
		return errors.New("topology must contain at least one node under 'topology.nodes'")
	}

	for nodeName, node := range t.Topology.Nodes {
		if strings.TrimSpace(nodeName) == "" {
			return errors.New("node name cannot be empty")
		}
		if strings.TrimSpace(nodeName) != nodeName {
			return fmt.Errorf("node name %q cannot contain leading or trailing whitespace", nodeName)
		}
		if len(nodeName) > MaxNodeNameLength || !NodeNameRegex.MatchString(nodeName) {
			return fmt.Errorf("invalid node name %q: must comply with Kubernetes DNS-1035 label syntax (lowercase alphanumeric and hyphens, start and end with an alphanumeric character, max %d characters)", nodeName, MaxNodeNameLength)
		}
		if node == nil {
			return fmt.Errorf("node %q definition is empty", nodeName)
		}
	}

	return nil
}
