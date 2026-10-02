package clab

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
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
		if node.Kind == "" && node.Image == "" {
			return fmt.Errorf("node %q requires at least 'kind' or 'image'", nodeName)
		}
	}

	for i, link := range t.Topology.Links {
		if len(link.Endpoints) != 2 {
			return fmt.Errorf("link #%d must have exactly 2 endpoints, got %d (%v)", i+1, len(link.Endpoints), link.Endpoints)
		}
		for _, ep := range link.Endpoints {
			parts := strings.Split(ep, ":")
			if len(parts) != 2 {
				return fmt.Errorf("link #%d endpoint %q is invalid, expected 'node:interface'", i+1, ep)
			}
			nodeName := parts[0]
			if _, exists := t.Topology.Nodes[nodeName]; !exists {
				return fmt.Errorf("link #%d references undefined node %q", i+1, nodeName)
			}
		}
	}

	return nil
}
