package clab

// Topology represents a containerlab topology definition.
type Topology struct {
	Name     string             `yaml:"name"`
	Prefix   *string            `yaml:"prefix,omitempty"`
	Mgmt     *MgmtConfig        `yaml:"mgmt,omitempty"`
	Topology TopologyDefinition `yaml:"topology"`
}

// MgmtConfig represents management network configuration.
type MgmtConfig struct {
	Network    string `yaml:"network,omitempty"`
	IPv4Subnet string `yaml:"ipv4-subnet,omitempty"`
	IPv6Subnet string `yaml:"ipv6-subnet,omitempty"`
}

// TopologyDefinition holds nodes and links.
type TopologyDefinition struct {
	Defaults *NodeDefinition           `yaml:"defaults,omitempty"`
	Kinds    map[string]*NodeDefinition `yaml:"kinds,omitempty"`
	Nodes    map[string]*NodeDefinition `yaml:"nodes"`
	Links    []LinkDefinition          `yaml:"links,omitempty"`
}

// NodeDefinition models a containerlab node.
type NodeDefinition struct {
	Kind          string            `yaml:"kind,omitempty"`
	Image         string            `yaml:"image,omitempty"`
	StartupConfig string            `yaml:"startup-config,omitempty"`
	Exec          []string          `yaml:"exec,omitempty"`
	Binds         []string          `yaml:"binds,omitempty"`
	Env           map[string]string `yaml:"env,omitempty"`
	Labels        map[string]string `yaml:"labels,omitempty"`
	MgmtIPv4      string            `yaml:"mgmt-ipv4,omitempty"`
	MgmtIPv6      string            `yaml:"mgmt-ipv6,omitempty"`
	Type          string            `yaml:"type,omitempty"`
}

// LinkDefinition models an inter-node connection.
type LinkDefinition struct {
	Endpoints []string          `yaml:"endpoints"`
	Labels    map[string]string `yaml:"labels,omitempty"`
	Vars      map[string]any    `yaml:"vars,omitempty"`
}
