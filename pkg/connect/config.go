package connect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConnectConfig defines connection-specific configuration settings.
type ConnectConfig struct {
	Method   string            `yaml:"method,omitempty"`
	User     string            `yaml:"user,omitempty"`
	Username string            `yaml:"username,omitempty"`
	Methods  map[string]string `yaml:"methods,omitempty"`
	Users    map[string]string `yaml:"users,omitempty"`
}

// Config represents the ~/.c9s/config file structure.
type Config struct {
	ConnectMethod string            `yaml:"connect_method,omitempty"`
	ConnectUser   string            `yaml:"connect_user,omitempty"`
	Method        string            `yaml:"method,omitempty"`
	User          string            `yaml:"user,omitempty"`
	Username      string            `yaml:"username,omitempty"`
	Connect       ConnectConfig     `yaml:"connect,omitempty"`
	Methods       map[string]string `yaml:"methods,omitempty"`
	Users         map[string]string `yaml:"users,omitempty"`
	Raw           map[string]any    `yaml:"-"`
}

// LoadConfig loads configuration from the default ~/.c9s directory.
// If the config file does not exist, an empty Config is returned with no error.
func LoadConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	if home == "" {
		return &Config{}, nil
	}
	return LoadConfigFromDir(filepath.Join(home, ".c9s"))
}

// LoadConfigFromDir searches for config, config.yaml, or config.yml in dir.
func LoadConfigFromDir(dir string) (*Config, error) {
	candidates := []string{
		filepath.Join(dir, "config"),
		filepath.Join(dir, "config.yaml"),
		filepath.Join(dir, "config.yml"),
	}

	for _, path := range candidates {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			return LoadConfigFile(path)
		}
	}

	return &Config{}, nil
}

// LoadConfigFile loads and parses a YAML configuration file from the specified path.
func LoadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("failed to read config file %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %q: %w", path, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err == nil {
		cfg.Raw = raw
	}

	return &cfg, nil
}

// GetConnectMethod retrieves the configured connection method, checking kind-specific overrides
// before falling back to global method settings.
func (c *Config) GetConnectMethod(kind string) string {
	if c == nil {
		return ""
	}

	if kind != "" {
		k := strings.ToLower(strings.TrimSpace(kind))
		kUnderscore := strings.ReplaceAll(k, "-", "_")
		kHyphen := strings.ReplaceAll(k, "_", "-")
		variants := []string{k, kUnderscore, kHyphen}

		if c.Connect.Methods != nil {
			for _, variant := range variants {
				if v, ok := c.Connect.Methods[variant]; ok && v != "" {
					return v
				}
			}
		}

		if c.Methods != nil {
			for _, variant := range variants {
				if v, ok := c.Methods[variant]; ok && v != "" {
					return v
				}
			}
		}

		if c.Raw != nil {
			for _, prefix := range []string{"connect_method_", "method_"} {
				for _, variant := range variants {
					if val, ok := c.Raw[prefix+variant]; ok {
						if s, ok := val.(string); ok && s != "" {
							return s
						}
					}
				}
			}
		}
		return ""
	}

	// Global fallback
	if c.Connect.Method != "" {
		return c.Connect.Method
	}
	if c.ConnectMethod != "" {
		return c.ConnectMethod
	}
	if c.Method != "" {
		return c.Method
	}
	if c.Raw != nil {
		for _, key := range []string{"connect_method", "method"} {
			if val, ok := c.Raw[key]; ok {
				if s, ok := val.(string); ok && s != "" {
					return s
				}
			}
		}
	}

	return ""
}

// GetConnectUser retrieves the configured username, checking kind-specific overrides
// before falling back to global user settings.
func (c *Config) GetConnectUser(kind string) string {
	if c == nil {
		return ""
	}

	if kind != "" {
		k := strings.ToLower(strings.TrimSpace(kind))
		kUnderscore := strings.ReplaceAll(k, "-", "_")
		kHyphen := strings.ReplaceAll(k, "_", "-")
		variants := []string{k, kUnderscore, kHyphen}

		if c.Connect.Users != nil {
			for _, variant := range variants {
				if v, ok := c.Connect.Users[variant]; ok && v != "" {
					return v
				}
			}
		}

		if c.Users != nil {
			for _, variant := range variants {
				if v, ok := c.Users[variant]; ok && v != "" {
					return v
				}
			}
		}

		if c.Raw != nil {
			for _, prefix := range []string{"connect_user_", "user_", "username_"} {
				for _, variant := range variants {
					if val, ok := c.Raw[prefix+variant]; ok {
						if s, ok := val.(string); ok && s != "" {
							return s
						}
					}
				}
			}
		}
		return ""
	}

	// Global fallback
	if c.Connect.User != "" {
		return c.Connect.User
	}
	if c.Connect.Username != "" {
		return c.Connect.Username
	}
	if c.ConnectUser != "" {
		return c.ConnectUser
	}
	if c.User != "" {
		return c.User
	}
	if c.Username != "" {
		return c.Username
	}
	if c.Raw != nil {
		for _, key := range []string{"connect_user", "user", "username"} {
			if val, ok := c.Raw[key]; ok {
				if s, ok := val.(string); ok && s != "" {
					return s
				}
			}
		}
	}

	return ""
}
