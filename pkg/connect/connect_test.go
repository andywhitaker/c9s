package connect

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"c9s/pkg/k8s"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func TestUsernameAndKindDefaults(t *testing.T) {
	nosKinds := []string{
		"nokia_srlinux", "nokia_sros", "arista_ceos", "arista_veos",
		"cisco_xrd", "cisco_xr9kv", "cisco_n9kv", "cisco_csr1000v", "cisco_c8000v",
		"juniper_vptx", "juniper_vmx", "juniper_vqfx", "juniper_crpd",
		"sonic", "sonic-vs", "cumulus-vx",
	}

	for _, k := range nosKinds {
		method := DefaultMethodForKind(k)
		if method != MethodSSH {
			t.Errorf("kind %q: expected default method %q, got %q", k, MethodSSH, method)
		}
	}

	genericKinds := []string{"linux", "alpine", "custom-app", ""}
	for _, k := range genericKinds {
		method := DefaultMethodForKind(k)
		if method != MethodExec {
			t.Errorf("kind %q: expected default method %q, got %q", k, MethodExec, method)
		}
	}

	userDefaults := map[string]string{
		"nokia_srlinux":  "admin",
		"nokia_sros":     "admin",
		"arista_ceos":    "admin",
		"arista_veos":    "admin",
		"cisco_n9kv":     "admin",
		"juniper_vptx":   "admin",
		"juniper_vmx":    "admin",
		"juniper_vqfx":   "admin",
		"sonic":          "admin",
		"sonic-vs":       "admin",
		"cisco_xrd":      "cisco",
		"cisco_xr9kv":    "cisco",
		"cisco_csr1000v": "cisco",
		"cisco_c8000v":   "cisco",
		"juniper_crpd":   "root",
		"cumulus-vx":     "cumulus",
		"linux":          "admin",
		"unknown_kind":   "admin",
		"":               "admin",
	}

	for k, expectedUser := range userDefaults {
		u := DefaultUserForKind(k)
		if u != expectedUser {
			t.Errorf("kind %q: expected default user %q, got %q", k, expectedUser, u)
		}
	}

	// Test user@node argument precedence
	if u := ResolveUsername("customuser", "flaguser", "nokia_srlinux", nil); u != "customuser" {
		t.Errorf("expected user@node to take precedence, got %q", u)
	}

	// Test -u / --username flag precedence over env/config/defaults
	if u := ResolveUsername("", "flaguser", "nokia_srlinux", nil); u != "flaguser" {
		t.Errorf("expected flag to take precedence when user@node is empty, got %q", u)
	}
}

func TestBuildSSHArgs(t *testing.T) {
	tests := []struct {
		name       string
		user       string
		host       string
		port       int
		customCmd  []string
		expected   []string
	}{
		{
			name: "default port 22, no custom command",
			user: "admin",
			host: "172.18.255.10",
			port: 22,
			expected: []string{
				"ssh",
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "LogLevel=ERROR",
				"admin@172.18.255.10",
			},
		},
		{
			name: "port 0 defaults to omitting -p",
			user: "admin",
			host: "172.18.255.10",
			port: 0,
			expected: []string{
				"ssh",
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "LogLevel=ERROR",
				"admin@172.18.255.10",
			},
		},
		{
			name: "custom port 2222",
			user: "cisco",
			host: "10.0.0.1",
			port: 2222,
			expected: []string{
				"ssh",
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "LogLevel=ERROR",
				"-p", "2222",
				"cisco@10.0.0.1",
			},
		},
		{
			name: "no user, only host",
			user: "",
			host: "10.0.0.1",
			port: 22,
			expected: []string{
				"ssh",
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "LogLevel=ERROR",
				"10.0.0.1",
			},
		},
		{
			name:      "with custom command args",
			user:      "root",
			host:      "192.168.1.1",
			port:      2201,
			customCmd: []string{"show", "version", "--detail"},
			expected: []string{
				"ssh",
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "LogLevel=ERROR",
				"-p", "2201",
				"root@192.168.1.1",
				"show", "version", "--detail",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := BuildSSHArgs(tt.user, tt.host, tt.port, tt.customCmd)
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("\nexpected: %v\nactual:   %v", tt.expected, actual)
			}
		})
	}
}

func TestBuildKubectlArgs(t *testing.T) {
	tests := []struct {
		name          string
		namespace     string
		kubeconfig    string
		kubeCtx       string
		podName       string
		containerName string
		customCmd     []string
		expected      []string
	}{
		{
			name:          "default shell fallback without custom command",
			namespace:     "lab-testlab",
			kubeconfig:    "",
			kubeCtx:       "",
			podName:       "node1-pod",
			containerName: "node1",
			customCmd:     nil,
			expected: []string{
				"kubectl", "exec", "-it", "-n", "lab-testlab",
				"node1-pod", "-c", "node1", "--",
				"sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh",
			},
		},
		{
			name:          "with custom command",
			namespace:     "lab-testlab",
			kubeconfig:    "",
			kubeCtx:       "",
			podName:       "node1-pod",
			containerName: "node1",
			customCmd:     []string{"uname", "-a"},
			expected: []string{
				"kubectl", "exec", "-it", "-n", "lab-testlab",
				"node1-pod", "-c", "node1", "--",
				"uname", "-a",
			},
		},
		{
			name:          "with kubeconfig and context",
			namespace:     "lab-testlab",
			kubeconfig:    "/home/user/.kube/config",
			kubeCtx:       "my-cluster",
			podName:       "node2-pod",
			containerName: "node2-container",
			customCmd:     []string{"bash"},
			expected: []string{
				"kubectl", "exec", "-it", "-n", "lab-testlab",
				"--kubeconfig", "/home/user/.kube/config",
				"--context", "my-cluster",
				"node2-pod", "-c", "node2-container", "--",
				"bash",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := BuildKubectlArgs(tt.namespace, tt.kubeconfig, tt.kubeCtx, tt.podName, tt.containerName, tt.customCmd)
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("\nexpected: %v\nactual:   %v", tt.expected, actual)
			}
		})
	}
}

func TestResolveNamespace(t *testing.T) {
	// 1. Explicit safe namespace flag
	ns, err := ResolveNamespace("lab-mylab", "")
	if err != nil {
		t.Fatalf("unexpected error for valid namespace: %v", err)
	}
	if ns != "lab-mylab" {
		t.Errorf("expected 'lab-mylab', got %q", ns)
	}

	// 2. Explicit unsafe namespace flag
	_, err = ResolveNamespace("default", "")
	if err == nil {
		t.Fatalf("expected error for protected namespace 'default', got nil")
	}

	_, err = ResolveNamespace("kube-system", "")
	if err == nil {
		t.Fatalf("expected error for protected namespace 'kube-system', got nil")
	}

	_, err = ResolveNamespace("invalid-prefix", "")
	if err == nil {
		t.Fatalf("expected error for namespace without lab- prefix, got nil")
	}

	// 3. Topology file flag
	tmpDir := t.TempDir()
	topoPath := filepath.Join(tmpDir, "topo.clab.yml")
	topoContent := "name: test-topology\ntopology:\n  nodes:\n    srl1:\n      kind: nokia_srlinux\n"
	if err := os.WriteFile(topoPath, []byte(topoContent), 0644); err != nil {
		t.Fatalf("failed to write temp topo: %v", err)
	}

	ns, err = ResolveNamespace("", topoPath)
	if err != nil {
		t.Fatalf("unexpected error with valid topo file: %v", err)
	}
	if ns != "lab-test-topology" {
		t.Errorf("expected 'lab-test-topology', got %q", ns)
	}

	// 4. Non-existent topo file
	_, err = ResolveNamespace("", filepath.Join(tmpDir, "nonexistent.clab.yml"))
	if err == nil {
		t.Fatalf("expected error for non-existent topo file, got nil")
	}

	// 5. Auto-discovery in directory with 1 file
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to tmpDir: %v", err)
	}

	ns, err = ResolveNamespace("", "")
	if err != nil {
		t.Fatalf("unexpected error in auto-discovery: %v", err)
	}
	if ns != "lab-test-topology" {
		t.Errorf("expected 'lab-test-topology', got %q", ns)
	}

	// 6. Ambiguous auto-discovery with 2 files
	secondTopo := filepath.Join(tmpDir, "second.clab.yml")
	if err := os.WriteFile(secondTopo, []byte("name: second\ntopology:\n  nodes:\n    n1:\n      kind: linux\n"), 0644); err != nil {
		t.Fatalf("failed to write second topo: %v", err)
	}

	_, err = ResolveNamespace("", "")
	if err == nil {
		t.Fatalf("expected error for ambiguous topology files, got nil")
	}
	if !strings.Contains(err.Error(), "specify target namespace (-n/--namespace) or topology file (-t/--topo)") {
		t.Errorf("expected clear guidance message in error, got %v", err)
	}

	// 7. Auto-discovery in empty directory
	emptyDir := t.TempDir()
	if err := os.Chdir(emptyDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	_, err = ResolveNamespace("", "")
	if err == nil {
		t.Fatalf("expected error for zero topology files, got nil")
	}
	if !strings.Contains(err.Error(), "specify target namespace (-n/--namespace) or topology file (-t/--topo)") {
		t.Errorf("expected clear guidance message in error, got %v", err)
	}
}

func TestConfigFileAndEnvOverrides(t *testing.T) {
	// Clean env before and after
	envVars := []string{
		"C9S_CONNECT_METHOD",
		"C9S_CONNECT_METHOD_LINUX",
		"C9S_CONNECT_METHOD_NOKIA_SRLINUX",
		"C9S_CONNECT_USER",
		"C9S_CONNECT_USER_LINUX",
		"C9S_CONNECT_USER_NOKIA_SRLINUX",
	}
	for _, k := range envVars {
		orig := os.Getenv(k)
		defer os.Setenv(k, orig)
		_ = os.Unsetenv(k)
	}

	// 1. Mutual exclusion of --ssh and --exec
	_, err := ResolveMethod(true, true, "linux", nil)
	if err == nil {
		t.Fatalf("expected error when both --ssh and --exec are true")
	}

	// 2. Flags override all
	m, err := ResolveMethod(true, false, "linux", nil)
	if err != nil || m != MethodSSH {
		t.Fatalf("expected --ssh to force SSH on linux, got %q (err: %v)", m, err)
	}
	m, err = ResolveMethod(false, true, "nokia_srlinux", nil)
	if err != nil || m != MethodExec {
		t.Fatalf("expected --exec to force exec on nokia_srlinux, got %q (err: %v)", m, err)
	}

	// 3. Config file loading
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
connect:
  method: ssh
  user: globalcfguser
  methods:
    linux: ssh
    custom_nos: exec
  users:
    linux: linuxcfguser
    nokia_srlinux: srlcfguser
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfigFromDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config from dir: %v", err)
	}

	// Test kind-specific method from config
	m, err = ResolveMethod(false, false, "linux", cfg)
	if err != nil || m != MethodSSH {
		t.Errorf("expected config method 'ssh' for linux, got %q", m)
	}
	m, err = ResolveMethod(false, false, "custom_nos", cfg)
	if err != nil || m != MethodExec {
		t.Errorf("expected config method 'exec' for custom_nos, got %q", m)
	}

	// Test kind-specific user from config
	u := ResolveUsername("", "", "linux", cfg)
	if u != "linuxcfguser" {
		t.Errorf("expected 'linuxcfguser', got %q", u)
	}
	u = ResolveUsername("", "", "nokia_srlinux", cfg)
	if u != "srlcfguser" {
		t.Errorf("expected 'srlcfguser', got %q", u)
	}

	// Test global fallback from config
	u = ResolveUsername("", "", "other_kind", cfg)
	if u != "globalcfguser" {
		t.Errorf("expected 'globalcfguser', got %q", u)
	}

	// 4. Environment variable overrides config
	_ = os.Setenv("C9S_CONNECT_USER_LINUX", "envlinuxuser")
	u = ResolveUsername("", "", "linux", cfg)
	if u != "envlinuxuser" {
		t.Errorf("expected env 'envlinuxuser' to override config, got %q", u)
	}

	_ = os.Setenv("C9S_CONNECT_METHOD_LINUX", "exec")
	m, err = ResolveMethod(false, false, "linux", cfg)
	if err != nil || m != MethodExec {
		t.Errorf("expected env C9S_CONNECT_METHOD_LINUX=exec to override config ssh, got %q", m)
	}

	// Test non-existent config ignored cleanly without error
	emptyDir := t.TempDir()
	emptyCfg, err := LoadConfigFromDir(emptyDir)
	if err != nil {
		t.Fatalf("expected nil error for missing config, got %v", err)
	}
	if emptyCfg == nil {
		t.Fatalf("expected non-nil empty Config struct")
	}
}

func TestExecFnInterception(t *testing.T) {
	origExec := ExecFn
	defer func() { ExecFn = origExec }()

	var interceptedArgv0 string
	var interceptedArgv []string
	var interceptedEnvv []string

	ExecFn = func(argv0 string, argv []string, envv []string) error {
		interceptedArgv0 = argv0
		interceptedArgv = argv
		interceptedEnvv = envv
		return nil
	}

	// Test ExecuteSSH interception
	err := ExecuteSSH("admin", "192.168.1.100", 2222, []string{"show", "version"})
	if err != nil {
		t.Fatalf("unexpected error in ExecuteSSH: %v", err)
	}

	if !strings.HasSuffix(interceptedArgv0, "ssh") {
		t.Errorf("expected argv0 to be ssh path, got %q", interceptedArgv0)
	}
	expectedSSHArgv := []string{
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-p", "2222",
		"admin@192.168.1.100",
		"show", "version",
	}
	if !reflect.DeepEqual(interceptedArgv, expectedSSHArgv) {
		t.Errorf("expected ssh argv %v, got %v", expectedSSHArgv, interceptedArgv)
	}
	if len(interceptedEnvv) == 0 {
		t.Errorf("expected non-empty environment")
	}

	// Test ExecuteKubectl interception
	err = ExecuteKubectl("lab-mylab", "", "", "srl-pod", "srl", []string{"bash"})
	if err != nil {
		t.Fatalf("unexpected error in ExecuteKubectl: %v", err)
	}

	if !strings.HasSuffix(interceptedArgv0, "kubectl") {
		t.Errorf("expected argv0 to be kubectl path, got %q", interceptedArgv0)
	}
	expectedKubectlArgv := []string{
		"kubectl", "exec", "-it", "-n", "lab-mylab",
		"srl-pod", "-c", "srl", "--",
		"bash",
	}
	if !reflect.DeepEqual(interceptedArgv, expectedKubectlArgv) {
		t.Errorf("expected kubectl argv %v, got %v", expectedKubectlArgv, interceptedArgv)
	}
}

func TestResolveNode(t *testing.T) {
	ctx := context.Background()

	scheme := runtime.NewScheme()
	fakeDyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			k8s.NodeGVR: "NodeList",
		},
	)
	fakeKube := fake.NewSimpleClientset()

	client := &k8s.Client{
		KubeClient:    fakeKube,
		DynamicClient: fakeDyn,
		ContextName:   "test-ctx",
		ClusterName:   "test-cluster",
	}

	// 1. Create Node CR with exposedPort, loadBalancerAddress, and directManagement.ipv4 with /24 mask
	nodeCR := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "c9s.run/v1alpha1",
			"kind":       "Node",
			"metadata": map[string]interface{}{
				"name":      "srl1",
				"namespace": "lab-test",
			},
			"spec": map[string]interface{}{
				"kind":  "nokia_srlinux",
				"image": "ghcr.io/nokia/srlinux:latest",
			},
			"status": map[string]interface{}{
				"readiness": "ready",
				"exposedPorts": map[string]interface{}{
					"loadBalancerAddress": "172.18.255.10",
					"tcp": []interface{}{
						map[string]interface{}{
							"destinationPort": int64(22),
							"exposePort":      int64(2201),
						},
					},
				},
				"directManagement": map[string]interface{}{
					"ipv4": "172.20.20.10/24",
				},
			},
		},
	}
	_, err := fakeDyn.Resource(k8s.NodeGVR).Namespace("lab-test").Create(ctx, nodeCR, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create fake Node CR: %v", err)
	}

	// Create corresponding Pod for srl1
	srlPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "srl1-pod-0",
			Namespace: "lab-test",
			Labels: map[string]string{
				"c9s.run/topologyNode": "srl1",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "srlinux"},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.244.0.5",
		},
	}
	_, err = fakeKube.CoreV1().Pods("lab-test").Create(ctx, srlPod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create fake Pod: %v", err)
	}

	info, err := ResolveNode(ctx, client, "lab-test", "srl1")
	if err != nil {
		t.Fatalf("unexpected error resolving srl1: %v", err)
	}

	if info.Kind != "nokia_srlinux" {
		t.Errorf("expected kind 'nokia_srlinux', got %q", info.Kind)
	}
	if info.ExternalIP != "172.18.255.10" {
		t.Errorf("expected external IP '172.18.255.10', got %q", info.ExternalIP)
	}
	if info.InternalIP != "172.20.20.10" {
		t.Errorf("expected internal IP '172.20.20.10' (stripped /24), got %q", info.InternalIP)
	}
	if info.SSHPort != 2201 {
		t.Errorf("expected exposed SSH port 2201, got %d", info.SSHPort)
	}
	if info.PodName != "srl1-pod-0" {
		t.Errorf("expected pod name 'srl1-pod-0', got %q", info.PodName)
	}
	if info.ContainerName != "srlinux" {
		t.Errorf("expected container name 'srlinux', got %q", info.ContainerName)
	}

	// 2. Test fallback to Pod discovery when Node CR does not exist
	linuxPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "client1-daemonset-xyz",
			Namespace: "lab-test",
			Labels: map[string]string{
				"c9s.run/topologyNode": "client1",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "alpine-client"},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.244.0.22",
		},
	}
	_, err = fakeKube.CoreV1().Pods("lab-test").Create(ctx, linuxPod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create fake linux pod: %v", err)
	}

	infoLinux, err := ResolveNode(ctx, client, "lab-test", "client1")
	if err != nil {
		t.Fatalf("unexpected error resolving client1: %v", err)
	}
	if infoLinux.Kind != "linux" {
		t.Errorf("expected fallback kind 'linux', got %q", infoLinux.Kind)
	}
	if infoLinux.InternalIP != "10.244.0.22" {
		t.Errorf("expected fallback internal IP '10.244.0.22', got %q", infoLinux.InternalIP)
	}
	if infoLinux.PodName != "client1-daemonset-xyz" {
		t.Errorf("expected pod name 'client1-daemonset-xyz', got %q", infoLinux.PodName)
	}
	if infoLinux.ContainerName != "alpine-client" {
		t.Errorf("expected container name 'alpine-client', got %q", infoLinux.ContainerName)
	}

	// 3. Test node prefix fallback when topologyNode label is absent
	prefixPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prefixnode-5d6f8",
			Namespace: "lab-test",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "prefix-box"},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.244.0.33",
		},
	}
	_, err = fakeKube.CoreV1().Pods("lab-test").Create(ctx, prefixPod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create fake prefix pod: %v", err)
	}

	infoPrefix, err := ResolveNode(ctx, client, "lab-test", "prefixnode")
	if err != nil {
		t.Fatalf("unexpected error resolving prefixnode: %v", err)
	}
	if infoPrefix.PodName != "prefixnode-5d6f8" {
		t.Errorf("expected pod name 'prefixnode-5d6f8', got %q", infoPrefix.PodName)
	}

	// 4. Test non-existent node
	_, err = ResolveNode(ctx, client, "lab-test", "nonexistent-node")
	if err == nil {
		t.Fatalf("expected error for nonexistent node, got nil")
	}
}

func TestParseNodeTarget(t *testing.T) {
	tests := []struct {
		input       string
		expectedUser string
		expectedNode string
		shouldErr   bool
	}{
		{"node1", "", "node1", false},
		{"admin@node1", "admin", "node1", false},
		{"cisco@r1", "cisco", "r1", false},
		{"@node1", "", "node1", false},
		{"", "", "", true},
		{"   ", "", "", true},
		{"admin@", "", "", true},
	}

	for _, tt := range tests {
		u, n, err := ParseNodeTarget(tt.input)
		if tt.shouldErr {
			if err == nil {
				t.Errorf("input %q: expected error, got user=%q, node=%q", tt.input, u, n)
			}
		} else {
			if err != nil {
				t.Errorf("input %q: unexpected error %v", tt.input, err)
			}
			if u != tt.expectedUser || n != tt.expectedNode {
				t.Errorf("input %q: expected (%q, %q), got (%q, %q)", tt.input, tt.expectedUser, tt.expectedNode, u, n)
			}
		}
	}
}

func TestResolveTargetHost(t *testing.T) {
	// External preferred
	h, err := ResolveTargetHost("1.2.3.4", "10.0.0.1")
	if err != nil || h != "1.2.3.4" {
		t.Errorf("expected '1.2.3.4', got %q (err: %v)", h, err)
	}

	// N/A external falls back to internal
	h, err = ResolveTargetHost("N/A", "10.0.0.1")
	if err != nil || h != "10.0.0.1" {
		t.Errorf("expected '10.0.0.1', got %q (err: %v)", h, err)
	}

	// empty external falls back to internal
	h, err = ResolveTargetHost("", "10.0.0.1")
	if err != nil || h != "10.0.0.1" {
		t.Errorf("expected '10.0.0.1', got %q (err: %v)", h, err)
	}

	// both unavailable returns error
	_, err = ResolveTargetHost("N/A", "N/A")
	if err == nil {
		t.Errorf("expected error when both IPs are N/A")
	}

	_, err = ResolveTargetHost("", "")
	if err == nil {
		t.Errorf("expected error when both IPs are empty")
	}
}

func TestRunConnectEndToEnd(t *testing.T) {
	origExec := ExecFn
	defer func() { ExecFn = origExec }()

	var executedArgv []string
	ExecFn = func(argv0 string, argv []string, envv []string) error {
		executedArgv = argv
		return nil
	}

	ctx := context.Background()
	scheme := runtime.NewScheme()
	fakeDyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			k8s.NodeGVR: "NodeList",
		},
	)
	fakeKube := fake.NewSimpleClientset()
	client := &k8s.Client{
		KubeClient:    fakeKube,
		DynamicClient: fakeDyn,
	}

	// Setup fake SRL node
	nodeCR := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "c9s.run/v1alpha1",
			"kind":       "Node",
			"metadata": map[string]interface{}{
				"name":      "srl1",
				"namespace": "lab-mylab",
			},
			"spec": map[string]interface{}{
				"kind": "nokia_srlinux",
			},
			"status": map[string]interface{}{
				"exposedPorts": map[string]interface{}{
					"loadBalancerAddress": "172.18.255.10",
				},
				"directManagement": map[string]interface{}{
					"ipv4": "172.20.20.10/24",
				},
			},
		},
	}
	_, _ = fakeDyn.Resource(k8s.NodeGVR).Namespace("lab-mylab").Create(ctx, nodeCR, metav1.CreateOptions{})

	// 1. Connect to srl1 via SSH (default method for NOS)
	err := RunConnect(ctx, ConnectOptions{
		Namespace:     "lab-mylab",
		NodeTarget:    "srl1",
		Client:        client,
		CustomCommand: []string{"show", "version"},
	})
	if err != nil {
		t.Fatalf("unexpected error in RunConnect SSH: %v", err)
	}

	expectedSSH := []string{
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"admin@172.18.255.10",
		"show", "version",
	}
	if !reflect.DeepEqual(executedArgv, expectedSSH) {
		t.Errorf("expected SSH argv %v, got %v", expectedSSH, executedArgv)
	}

	// 2. Connect to srl1 forced via --exec
	srlPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "srl1-pod",
			Namespace: "lab-mylab",
			Labels:    map[string]string{"c9s.run/topologyNode": "srl1"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "srlinux"}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	_, _ = fakeKube.CoreV1().Pods("lab-mylab").Create(ctx, srlPod, metav1.CreateOptions{})

	err = RunConnect(ctx, ConnectOptions{
		Namespace:  "lab-mylab",
		NodeTarget: "srl1",
		ForceExec:  true,
		Client:     client,
	})
	if err != nil {
		t.Fatalf("unexpected error in RunConnect ForceExec: %v", err)
	}

	expectedExec := []string{
		"kubectl", "exec", "-it", "-n", "lab-mylab",
		"srl1-pod", "-c", "srlinux", "--",
		"sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh",
	}
	if !reflect.DeepEqual(executedArgv, expectedExec) {
		t.Errorf("expected Exec argv %v, got %v", expectedExec, executedArgv)
	}
}
