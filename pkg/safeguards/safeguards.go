package safeguards

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
)

const (
	NamespacePrefix          = "lab-"
	MaxNamespaceLength       = 63
	MaxLabNameLength         = MaxNamespaceLength - len(NamespacePrefix) // 59
	MaxConfigFileSizeBytes   = 512 * 1024                                // 512 KB
	MaxConfigMapPayloadBytes = 800 * 1024                                // 800 KB
)

const DefaultContextName = "kind-try-c9s"

var (
	// LabNameRegex enforces DNS-1123 label standard.
	LabNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,57}[a-z0-9])?$`)

	// ProtectedNamespaces lists system namespaces that c9s must NEVER touch.
	ProtectedNamespaces = map[string]bool{
		"default":            true,
		"kube-system":        true,
		"kube-public":        true,
		"kube-node-lease":    true,
		"local-path-storage": true,
		"metallb-system":     true,
	}
)

// ValidateContextAndCluster resolves and validates the kubeconfig context, defaulting to kind-try-c9s.
func ValidateContextAndCluster(clientConfig clientcmd.ClientConfig, contextOverride string) (string, string, error) {
	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return "", "", fmt.Errorf("failed to read kubeconfig: %w", err)
	}

	currentContext := contextOverride
	if currentContext == "" {
		if _, exists := rawConfig.Contexts[DefaultContextName]; exists {
			currentContext = DefaultContextName
		} else {
			currentContext = rawConfig.CurrentContext
		}
	}
	if currentContext == "" {
		return "", "", errors.New("current kubeconfig context is empty; specify with --context or set a current-context")
	}

	contextObj, exists := rawConfig.Contexts[currentContext]
	if !exists || contextObj == nil {
		return "", "", fmt.Errorf("context %q not found in kubeconfig", currentContext)
	}

	clusterName := contextObj.Cluster
	return currentContext, clusterName, nil
}

// ValidateLabName validates the lab name according to RFC 1123 DNS-subdomain specifications.
func ValidateLabName(labName string) error {
	if strings.TrimSpace(labName) != labName {
		return errors.New("lab name cannot have leading or trailing whitespace")
	}
	if labName == "" {
		return errors.New("lab name cannot be empty")
	}
	if len(labName) > MaxLabNameLength {
		return fmt.Errorf("lab name %q exceeds maximum allowed length of %d characters", labName, MaxLabNameLength)
	}
	if !LabNameRegex.MatchString(labName) {
		return fmt.Errorf("invalid lab name %q: must consist of lowercase alphanumeric characters or '-', and start/end with an alphanumeric character (RFC 1123)", labName)
	}
	return nil
}

// DeriveNamespace computes and validates the target namespace: lab-{name_of_lab}.
func DeriveNamespace(labName string) (string, error) {
	if err := ValidateLabName(labName); err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(labName)
	ns := fmt.Sprintf("%s%s", NamespacePrefix, trimmed)
	if ProtectedNamespaces[ns] {
		return "", fmt.Errorf("CRITICAL SECURITY VIOLATION: Target namespace %q is a protected namespace", ns)
	}
	if !strings.HasPrefix(ns, NamespacePrefix) {
		return "", fmt.Errorf("CRITICAL SECURITY VIOLATION: Target namespace %q must start with prefix %q", ns, NamespacePrefix)
	}
	return ns, nil
}

// EnforceNamespaceSafety ensures an arbitrary namespace argument is safe to operate on.
func EnforceNamespaceSafety(namespace string) error {
	if namespace == "default" || ProtectedNamespaces[namespace] {
		return fmt.Errorf("CRITICAL SECURITY VIOLATION: Operations targeting %q namespace are strictly prohibited", namespace)
	}
	if !strings.HasPrefix(namespace, NamespacePrefix) || len(namespace) <= len(NamespacePrefix) {
		return fmt.Errorf("CRITICAL SECURITY VIOLATION: Namespace %q is not a valid managed c9s namespace (must be lab-<lab>)", namespace)
	}
	return nil
}

// SafeReadConfigFile safely verifies and reads a startup-config file, preventing path traversal.
func SafeReadConfigFile(baseDir, configPath string) ([]byte, error) {
	cleanConfigPath := strings.TrimSpace(configPath)
	if cleanConfigPath == "" {
		return nil, errors.New("config path cannot be empty")
	}

	realBaseDir, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve topology directory %q: %w", baseDir, err)
	}
	realBaseDir = filepath.Clean(realBaseDir)

	var candidatePath string
	if filepath.IsAbs(cleanConfigPath) {
		candidatePath = filepath.Clean(cleanConfigPath)
	} else {
		candidatePath = filepath.Join(realBaseDir, cleanConfigPath)
	}

	realTarget, err := filepath.EvalSymlinks(candidatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve startup config file %q: %w", cleanConfigPath, err)
	}
	realTarget = filepath.Clean(realTarget)

	// Verify confinement inside baseDir
	rel, err := filepath.Rel(realBaseDir, realTarget)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return nil, fmt.Errorf("SECURITY VIOLATION: Path traversal detected! File %q resolves outside allowed topology directory %q", cleanConfigPath, realBaseDir)
	}

	info, err := os.Stat(realTarget)
	if err != nil {
		return nil, fmt.Errorf("cannot stat config file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("SECURITY VIOLATION: Config file %q is not a regular file", cleanConfigPath)
	}
	if info.Size() > MaxConfigFileSizeBytes {
		return nil, fmt.Errorf("config file %q size (%d bytes) exceeds safe limit of %d bytes", cleanConfigPath, info.Size(), MaxConfigFileSizeBytes)
	}

	f, err := os.Open(realTarget)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, MaxConfigFileSizeBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	if int64(len(data)) > MaxConfigFileSizeBytes {
		return nil, fmt.Errorf("config file %q exceeded size limit during read", cleanConfigPath)
	}

	return data, nil
}
