package connect

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"c9s/pkg/clab"
	"c9s/pkg/k8s"
	"c9s/pkg/safeguards"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	MethodSSH  = "ssh"
	MethodExec = "exec"
)

// ExecFn is injectable for unit testing without terminating process execution.
var ExecFn func(string, []string, []string) error = syscall.Exec

// NodeInfo stores resolved information about a target topology node.
type NodeInfo struct {
	NodeName      string
	Kind          string
	ExternalIP    string
	InternalIP    string
	SSHPort       int
	PodName       string
	ContainerName string
}

// ConnectOptions encapsulates all parameters required to connect to a node.
type ConnectOptions struct {
	Namespace     string
	TopoFile      string
	NodeTarget    string
	Username      string
	ForceSSH      bool
	ForceExec     bool
	Kubeconfig    string
	KubeContext   string
	CustomCommand []string
	Config        *Config
	Client        *k8s.Client
}

// ParseNodeTarget parses a node target string of the form [user@]node into user and nodeName.
func ParseNodeTarget(target string) (username, nodeName string, err error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", "", errors.New("node target cannot be empty")
	}

	if idx := strings.Index(target, "@"); idx != -1 {
		username = strings.TrimSpace(target[:idx])
		nodeName = strings.TrimSpace(target[idx+1:])
	} else {
		nodeName = target
	}

	if nodeName == "" {
		return "", "", errors.New("node name cannot be empty")
	}

	return username, nodeName, nil
}

// ResolveNamespace resolves and validates the target namespace based on flags and auto-discovery.
func ResolveNamespace(nsFlag, topoFlag string) (string, error) {
	if strings.TrimSpace(nsFlag) != "" {
		ns := strings.TrimSpace(nsFlag)
		if err := safeguards.EnforceNamespaceSafety(ns); err != nil {
			return "", err
		}
		return ns, nil
	}

	if strings.TrimSpace(topoFlag) != "" {
		resolvedPath, err := clab.FindTopologyFile(topoFlag)
		if err != nil {
			return "", err
		}
		parsed, _, err := clab.ParseTopologyFile(resolvedPath)
		if err != nil {
			return "", fmt.Errorf("topology parsing error: %w", err)
		}
		return safeguards.DeriveNamespace(parsed.Name)
	}

	// Auto-discovery in current directory
	resolvedPath, err := clab.FindTopologyFile("")
	if err != nil {
		return "", fmt.Errorf("no topology file found or ambiguous; please specify target namespace (-n/--namespace) or topology file (-t/--topo): %w", err)
	}

	parsed, _, err := clab.ParseTopologyFile(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("topology parsing error: %w", err)
	}

	return safeguards.DeriveNamespace(parsed.Name)
}

// IsNOSKind determines if a kind represents a Network Operating System (NOS).
func IsNOSKind(kind string) bool {
	k := strings.ToLower(strings.TrimSpace(kind))
	kUnderscore := strings.ReplaceAll(k, "-", "_")
	nos := map[string]bool{
		"nokia_srlinux":  true,
		"nokia_sros":     true,
		"arista_ceos":    true,
		"arista_veos":    true,
		"cisco_xrd":      true,
		"cisco_xr9kv":    true,
		"cisco_n9kv":     true,
		"cisco_csr1000v": true,
		"cisco_c8000v":   true,
		"juniper_vptx":   true,
		"juniper_vmx":    true,
		"juniper_vqfx":   true,
		"juniper_crpd":   true,
		"sonic":          true,
		"sonic_vs":       true,
		"cumulus_vx":     true,
		// Common abbreviations/aliases
		"srl":      true,
		"srlinux":  true,
		"sros":     true,
		"ceos":     true,
		"veos":     true,
		"xrd":      true,
		"xr9kv":    true,
		"n9kv":     true,
		"csr1000v": true,
		"c8000v":   true,
		"vptx":     true,
		"vmx":      true,
		"vqfx":     true,
		"crpd":     true,
	}
	return nos[k] || nos[kUnderscore]
}

// DefaultMethodForKind returns the default connection method (SSH for NOS, exec for Linux/generic).
func DefaultMethodForKind(kind string) string {
	if IsNOSKind(kind) {
		return MethodSSH
	}
	return MethodExec
}

// normalizeMethod validates and normalizes method names to MethodSSH or MethodExec.
func normalizeMethod(m string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(m))
	switch norm {
	case "ssh":
		return MethodSSH, nil
	case "exec", "kubectl exec", "kubectl_exec", "kubectlexec":
		return MethodExec, nil
	default:
		return "", fmt.Errorf("unsupported connection method %q (expected 'ssh' or 'exec')", m)
	}
}

// ResolveMethod resolves the connection method taking flags, env, config, and kind defaults into account.
func ResolveMethod(flagSSH, flagExec bool, kind string, cfg *Config) (string, error) {
	if flagSSH && flagExec {
		return "", errors.New("cannot specify both --ssh and --exec")
	}
	if flagSSH {
		return MethodSSH, nil
	}
	if flagExec {
		return MethodExec, nil
	}

	kindEnv := strings.ToUpper(strings.ReplaceAll(kind, "-", "_"))
	if kindEnv != "" {
		if v := os.Getenv("C9S_CONNECT_METHOD_" + kindEnv); v != "" {
			return normalizeMethod(v)
		}
	}
	if cfg != nil && kind != "" {
		if v := cfg.GetConnectMethod(kind); v != "" {
			return normalizeMethod(v)
		}
	}
	if v := os.Getenv("C9S_CONNECT_METHOD"); v != "" {
		return normalizeMethod(v)
	}
	if cfg != nil {
		if v := cfg.GetConnectMethod(""); v != "" {
			return normalizeMethod(v)
		}
	}

	return DefaultMethodForKind(kind), nil
}

// DefaultUserForKind returns the default SSH username for a given kind.
func DefaultUserForKind(kind string) string {
	k := strings.ToLower(strings.TrimSpace(kind))
	kNorm := strings.ReplaceAll(k, "-", "_")
	switch kNorm {
	case "cisco_xrd", "cisco_xr9kv", "cisco_csr1000v", "cisco_c8000v", "xrd", "xr9kv", "csr1000v", "c8000v":
		return "cisco"
	case "juniper_crpd", "crpd":
		return "root"
	case "cumulus_vx":
		return "cumulus"
	default:
		return "admin"
	}
}

// ResolveUsername resolves the SSH username following user@node, -u flag, env, config, and kind defaults.
func ResolveUsername(userTarget, flagUser, kind string, cfg *Config) string {
	if userTarget != "" {
		return userTarget
	}
	if flagUser != "" {
		return flagUser
	}
	kindEnv := strings.ToUpper(strings.ReplaceAll(kind, "-", "_"))
	if kindEnv != "" {
		if v := os.Getenv("C9S_CONNECT_USER_" + kindEnv); v != "" {
			return v
		}
	}
	if cfg != nil && kind != "" {
		if v := cfg.GetConnectUser(kind); v != "" {
			return v
		}
	}
	if v := os.Getenv("C9S_CONNECT_USER"); v != "" {
		return v
	}
	if cfg != nil {
		if v := cfg.GetConnectUser(""); v != "" {
			return v
		}
	}

	return DefaultUserForKind(kind)
}

// ResolveTargetHost picks the external IP if valid and not "N/A", falling back to internal IP.
func ResolveTargetHost(externalIP, internalIP string) (string, error) {
	ext := strings.TrimSpace(externalIP)
	if ext != "" && !strings.EqualFold(ext, "n/a") {
		return ext, nil
	}
	in := strings.TrimSpace(internalIP)
	if in != "" && !strings.EqualFold(in, "n/a") {
		return in, nil
	}
	return "", errors.New("no accessible IP address found for target host")
}

func stripCIDR(ip string) string {
	parts := strings.Split(strings.TrimSpace(ip), "/")
	return parts[0]
}

func getIntFromMap(m map[string]any, key string) (int, bool) {
	val, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := val.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case string:
		if i, err := strconv.Atoi(n); err == nil {
			return i, true
		}
	}
	return 0, false
}

func findExposedPort(obj any, targetDestPort int) int {
	switch v := obj.(type) {
	case map[string]any:
		dest, hasDest := getIntFromMap(v, "destinationPort")
		exp, hasExp := getIntFromMap(v, "exposePort")
		if hasDest && hasExp && dest == targetDestPort {
			return exp
		}
		for _, child := range v {
			if p := findExposedPort(child, targetDestPort); p != 0 {
				return p
			}
		}
	case []any:
		for _, item := range v {
			if p := findExposedPort(item, targetDestPort); p != 0 {
				return p
			}
		}
	}
	return 0
}

// ResolveNode queries Kubernetes for the Clabernetes Node CR and falls back to Pod discovery.
func ResolveNode(ctx context.Context, client *k8s.Client, namespace, nodeName string) (*NodeInfo, error) {
	info := &NodeInfo{
		NodeName: nodeName,
	}

	var nodeCR *unstructured.Unstructured
	cr, err := client.DynamicClient.Resource(k8s.NodeGVR).Namespace(namespace).Get(ctx, nodeName, metav1.GetOptions{})
	if err == nil {
		nodeCR = cr
	} else if apierrors.IsNotFound(err) {
		// Try listing CRs to match by label or name
		crList, errList := client.DynamicClient.Resource(k8s.NodeGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if errList == nil && crList != nil {
			for i := range crList.Items {
				item := &crList.Items[i]
				if item.GetName() == nodeName {
					nodeCR = item
					break
				}
				if item.GetLabels() != nil {
					if item.GetLabels()["c9s.run/topologyNode"] == nodeName || item.GetLabels()["c9s.run/name"] == nodeName {
						nodeCR = item
						break
					}
				}
			}
		}
	} else {
		return nil, fmt.Errorf("failed to query node CR %q in namespace %q: %w", nodeName, namespace, err)
	}

	if nodeCR != nil {
		if k, found, _ := unstructured.NestedString(nodeCR.Object, "spec", "kind"); found && k != "" {
			info.Kind = k
		}
		if lb, found, _ := unstructured.NestedString(nodeCR.Object, "status", "exposedPorts", "loadBalancerAddress"); found && lb != "" {
			info.ExternalIP = lb
		}
		if rawIP, found, _ := unstructured.NestedString(nodeCR.Object, "status", "directManagement", "ipv4"); found && rawIP != "" {
			info.InternalIP = stripCIDR(rawIP)
		}
		if statusObj, found, _ := unstructured.NestedMap(nodeCR.Object, "status"); found {
			info.SSHPort = findExposedPort(statusObj, 22)
		}
	}

	// Query Services to supplement external IP and SSH port exposure
	svcs, _ := client.KubeClient.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	var matchingSvc *corev1.Service
	if svcs != nil {
		for i := range svcs.Items {
			s := &svcs.Items[i]
			if s.Name == nodeName || (s.Labels != nil && (s.Labels["c9s.run/topologyNode"] == nodeName || s.Labels["c9s.run/name"] == nodeName)) {
				matchingSvc = s
				break
			}
		}
	}

	if matchingSvc != nil {
		if info.ExternalIP == "" || strings.EqualFold(info.ExternalIP, "n/a") {
			for _, ing := range matchingSvc.Status.LoadBalancer.Ingress {
				if ing.IP != "" {
					info.ExternalIP = ing.IP
					break
				}
			}
		}
		if info.SSHPort == 0 {
			for _, sp := range matchingSvc.Spec.Ports {
				if sp.TargetPort.IntVal == 22 || sp.TargetPort.StrVal == "22" || sp.TargetPort.StrVal == "ssh" || (sp.TargetPort.IntVal == 0 && sp.Port == 22) {
					info.SSHPort = int(sp.Port)
					break
				}
			}
		}
	}

	// Query Pods to locate container name, pod name, and fallback info
	pods, _ := client.KubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	var matchingPod *corev1.Pod
	if pods != nil {
		// Prefer running pod matching topologyNode label or name
		for i := range pods.Items {
			p := &pods.Items[i]
			if p.Labels != nil {
				if p.Labels["c9s.run/topologyNode"] == nodeName || p.Labels["c9s.run/name"] == nodeName {
					matchingPod = p
					if p.Status.Phase == corev1.PodRunning {
						break
					}
				}
			}
			if p.Name == nodeName {
				matchingPod = p
				if p.Status.Phase == corev1.PodRunning {
					break
				}
			}
		}

		if matchingPod == nil {
			for i := range pods.Items {
				p := &pods.Items[i]
				if strings.HasPrefix(p.Name, nodeName+"-") {
					matchingPod = p
					if p.Status.Phase == corev1.PodRunning {
						break
					}
				}
			}
		}
	}

	if matchingPod != nil {
		info.PodName = matchingPod.Name
		if len(matchingPod.Spec.Containers) > 0 {
			info.ContainerName = matchingPod.Spec.Containers[0].Name
		} else {
			info.ContainerName = nodeName
		}

		if nodeCR == nil {
			if matchingPod.Status.PodIP != "" {
				info.InternalIP = matchingPod.Status.PodIP
			}
			if info.Kind == "" {
				if k, ok := matchingPod.Labels["c9s.run/kind"]; ok && k != "" {
					info.Kind = k
				} else {
					info.Kind = "linux"
				}
			}
		}
	}

	if nodeCR == nil && matchingPod == nil {
		return nil, fmt.Errorf("node %q not found in namespace %q", nodeName, namespace)
	}

	if info.SSHPort == 0 {
		info.SSHPort = 22
	}

	return info, nil
}

// BuildSSHArgs constructs the argv slice for SSH execution.
func BuildSSHArgs(user, host string, port int, customCmd []string) []string {
	args := []string{
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
	}

	if port != 0 && port != 22 {
		args = append(args, "-p", strconv.Itoa(port))
	}

	if user != "" {
		args = append(args, fmt.Sprintf("%s@%s", user, host))
	} else {
		args = append(args, host)
	}

	if len(customCmd) > 0 {
		args = append(args, customCmd...)
	}

	return args
}

// BuildKubectlArgs constructs the argv slice for kubectl exec execution.
func BuildKubectlArgs(namespace, kubeconfig, kubeCtx, podName, containerName string, customCmd []string) []string {
	args := []string{"kubectl", "exec", "-it", "-n", namespace}

	if kubeconfig != "" {
		args = append(args, "--kubeconfig", kubeconfig)
	}
	if kubeCtx != "" {
		args = append(args, "--context", kubeCtx)
	}

	args = append(args, podName, "-c", containerName, "--")

	if len(customCmd) > 0 {
		args = append(args, customCmd...)
	} else {
		args = append(args, "sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh")
	}

	return args
}

// ExecuteSSH performs the handover to SSH using ExecFn.
func ExecuteSSH(user, host string, port int, customCmd []string) error {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh binary not found in PATH: %w", err)
	}
	argv := BuildSSHArgs(user, host, port, customCmd)
	if err := ExecFn(sshPath, argv, os.Environ()); err != nil {
		return fmt.Errorf("ssh execution failed: %w", err)
	}
	return nil
}

// ExecuteKubectl performs the handover to kubectl exec using ExecFn.
func ExecuteKubectl(namespace, kubeconfig, kubeCtx, podName, containerName string, customCmd []string) error {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return fmt.Errorf("kubectl binary not found in PATH: %w", err)
	}
	argv := BuildKubectlArgs(namespace, kubeconfig, kubeCtx, podName, containerName, customCmd)
	if err := ExecFn(kubectlPath, argv, os.Environ()); err != nil {
		return fmt.Errorf("kubectl exec execution failed: %w", err)
	}
	return nil
}

// RunConnect coordinates node resolution, method selection, and execution handover.
func RunConnect(ctx context.Context, opts ConnectOptions) error {
	userArg, nodeName, err := ParseNodeTarget(opts.NodeTarget)
	if err != nil {
		return err
	}

	ns, err := ResolveNamespace(opts.Namespace, opts.TopoFile)
	if err != nil {
		return err
	}

	cfg := opts.Config
	if cfg == nil {
		loaded, err := LoadConfig()
		if err != nil {
			return err
		}
		cfg = loaded
	}

	client := opts.Client
	if client == nil {
		c, err := k8s.NewClient(opts.Kubeconfig, opts.KubeContext)
		if err != nil {
			return err
		}
		client = c
	}

	nodeInfo, err := ResolveNode(ctx, client, ns, nodeName)
	if err != nil {
		return err
	}

	method, err := ResolveMethod(opts.ForceSSH, opts.ForceExec, nodeInfo.Kind, cfg)
	if err != nil {
		return err
	}

	switch method {
	case MethodSSH:
		username := ResolveUsername(userArg, opts.Username, nodeInfo.Kind, cfg)
		host, err := ResolveTargetHost(nodeInfo.ExternalIP, nodeInfo.InternalIP)
		if err != nil {
			return fmt.Errorf("cannot connect to node %q via SSH: %w", nodeName, err)
		}
		port := nodeInfo.SSHPort
		if port == 0 {
			port = 22
		}
		return ExecuteSSH(username, host, port, opts.CustomCommand)

	case MethodExec:
		if nodeInfo.PodName == "" {
			return fmt.Errorf("cannot connect to node %q via kubectl exec: no running pod found in namespace %q", nodeName, ns)
		}
		containerName := nodeInfo.ContainerName
		if containerName == "" {
			containerName = nodeName
		}
		return ExecuteKubectl(ns, opts.Kubeconfig, opts.KubeContext, nodeInfo.PodName, containerName, opts.CustomCommand)

	default:
		return fmt.Errorf("unknown connection method %q", method)
	}
}
