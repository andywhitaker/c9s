package main

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"c9s/pkg/connect"
	"c9s/pkg/k8s"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func TestConnectCmdFlagValidation(t *testing.T) {
	// Reset flags after test
	defer func() {
		connectNamespace = ""
		connectUsername = ""
		connectSSH = false
		connectExec = false
		connectClientOverride = nil
	}()

	// 1. Both --ssh and --exec should fail
	rootCmd.SetArgs([]string{"connect", "--ssh", "--exec", "node1", "-n", "lab-mylab"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatalf("expected error when both --ssh and --exec are passed, got nil")
	}
	if !strings.Contains(err.Error(), "cannot specify both --ssh and --exec") {
		t.Errorf("unexpected error message: %v", err)
	}

	// 2. Missing node name should fail
	rootCmd.SetArgs([]string{"connect", "-n", "lab-mylab"})
	err = rootCmd.Execute()
	if err == nil {
		t.Fatalf("expected error when node name is omitted, got nil")
	}

	// 3. Node name missing before '--' should fail
	rootCmd.SetArgs([]string{"connect", "-n", "lab-mylab", "--", "show", "version"})
	err = rootCmd.Execute()
	if err == nil {
		t.Fatalf("expected error when node is missing before '--', got nil")
	}

	// 4. Too many arguments before '--' should fail
	rootCmd.SetArgs([]string{"connect", "-n", "lab-mylab", "node1", "extra", "--", "show", "version"})
	err = rootCmd.Execute()
	if err == nil {
		t.Fatalf("expected error when multiple arguments appear before '--', got nil")
	}
}

func TestConnectCmdExecutionHandover(t *testing.T) {
	origExec := connect.ExecFn
	defer func() { connect.ExecFn = origExec }()

	var interceptedArgv []string
	connect.ExecFn = func(argv0 string, argv []string, envv []string) error {
		interceptedArgv = argv
		return nil
	}

	defer func() {
		connectNamespace = ""
		connectUsername = ""
		connectSSH = false
		connectExec = false
		connectClientOverride = nil
	}()

	ctx := context.Background()
	scheme := runtime.NewScheme()
	fakeDyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			k8s.NodeGVR: "NodeList",
		},
	)
	fakeKube := fake.NewSimpleClientset()
	connectClientOverride = &k8s.Client{
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
	_, err := fakeDyn.Resource(k8s.NodeGVR).Namespace("lab-mylab").Create(ctx, nodeCR, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create node CR: %v", err)
	}

	// Execute connect command with trailing custom command
	rootCmd.SetArgs([]string{"connect", "-n", "lab-mylab", "admin@srl1", "--", "show", "version"})
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error running rootCmd connect: %v", err)
	}

	expectedSSH := []string{
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"admin@172.18.255.10",
		"show", "version",
	}

	if !reflect.DeepEqual(interceptedArgv, expectedSSH) {
		t.Errorf("expected argv %v, got %v", expectedSSH, interceptedArgv)
	}
}
