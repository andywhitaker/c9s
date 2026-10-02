package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetSummaryRowsDeterministicSorting(t *testing.T) {
	scheme := runtime.NewScheme()
	client := &Client{
		KubeClient: fake.NewSimpleClientset(),
		DynamicClient: dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
			scheme,
			map[schema.GroupVersionResource]string{
				nodeGVR: "NodeList",
			},
		),
		ContextName: "kind-try-c9s",
		ClusterName: "kind-try-c9s",
	}

	expectedNodes := map[string]string{
		"zebra":   "linux",
		"alpha":   "nokia_srlinux",
		"charlie": "linux",
		"bravo":   "nokia_srlinux",
	}

	rows, err := client.GetSummaryRows(context.Background(), "testlab", expectedNodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(rows))
	}

	expectedOrder := []string{"alpha", "bravo", "charlie", "zebra"}
	for i, expected := range expectedOrder {
		if rows[i].NodeName != expected {
			t.Errorf("row %d: expected NodeName %q, got %q", i, expected, rows[i].NodeName)
		}
		if rows[i].Index != i+1 {
			t.Errorf("row %d: expected Index %d, got %d", i, i+1, rows[i].Index)
		}
		if rows[i].Namespace != "lab-testlab" {
			t.Errorf("row %d: expected Namespace %q, got %q", i, "lab-testlab", rows[i].Namespace)
		}
	}
}

func TestGetSummaryRowsAutoDiscovery(t *testing.T) {
	scheme := runtime.NewScheme()

	nodeCR := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "c9s.run/v1alpha1",
			"kind":       "Node",
			"metadata": map[string]interface{}{
				"name":      "node1",
				"namespace": "lab-testlab",
			},
			"spec": map[string]interface{}{
				"kind":  "nokia_srlinux",
				"image": "ghcr.io/nokia/srlinux:latest",
			},
			"status": map[string]interface{}{
				"readiness": "ready",
				"exposedPorts": map[string]interface{}{
					"loadBalancerAddress": "172.18.255.10",
				},
				"directManagement": map[string]interface{}{
					"ipv4": "172.20.20.10/24",
				},
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node2-pod",
			Namespace: "lab-testlab",
			Labels: map[string]string{
				"c9s.run/name": "node2",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "node2",
					Image: "alpine:latest",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.244.0.15",
		},
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node2",
			Namespace: "lab-testlab",
			Labels: map[string]string{
				"c9s.run/topologyNode":        "node2",
				"c9s.run/topologyServiceType": "expose",
			},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "172.18.255.20"},
				},
			},
		},
	}

	client := &Client{
		KubeClient: fake.NewSimpleClientset(pod, svc),
		DynamicClient: dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
			scheme,
			map[schema.GroupVersionResource]string{
				nodeGVR: "NodeList",
			},
			nodeCR,
		),
		ContextName: "kind-try-c9s",
		ClusterName: "kind-try-c9s",
	}

	rows, err := client.GetSummaryRows(context.Background(), "testlab", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// node1 from crMap
	if rows[0].NodeName != "node1" || rows[0].Kind != "nokia_srlinux" || rows[0].Image != "ghcr.io/nokia/srlinux:latest" ||
		rows[0].IPv4 != "172.18.255.10" || rows[0].IPv4Internal != "172.20.20.10/24" || rows[0].Namespace != "lab-testlab" {
		t.Errorf("unexpected row 0: %+v", rows[0])
	}

	// node2 from podMap and svcMap
	if rows[1].NodeName != "node2" || rows[1].Kind != "unknown" || rows[1].Image != "alpine:latest" ||
		rows[1].IPv4 != "172.18.255.20" || rows[1].IPv4Internal != "10.244.0.15" || rows[1].Namespace != "lab-testlab" {
		t.Errorf("unexpected row 1: %+v", rows[1])
	}
}

func TestGetSummaryRowsIPFallbacks(t *testing.T) {
	scheme := runtime.NewScheme()

	nodeCR := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "c9s.run/v1alpha1",
			"kind":       "Node",
			"metadata": map[string]interface{}{
				"name":      "node-no-ip",
				"namespace": "lab-testlab",
			},
			"spec": map[string]interface{}{
				"kind":  "linux",
				"image": "alpine:latest",
			},
		},
	}

	client := &Client{
		KubeClient: fake.NewSimpleClientset(),
		DynamicClient: dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
			scheme,
			map[schema.GroupVersionResource]string{
				nodeGVR: "NodeList",
			},
			nodeCR,
		),
		ContextName: "kind-try-c9s",
		ClusterName: "kind-try-c9s",
	}

	rows, err := client.GetSummaryRows(context.Background(), "testlab", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	if rows[0].IPv4 != "N/A" {
		t.Errorf("expected IPv4 to fallback to N/A, got %q", rows[0].IPv4)
	}
	if rows[0].IPv4Internal != "N/A" {
		t.Errorf("expected IPv4Internal to fallback to N/A, got %q", rows[0].IPv4Internal)
	}
	if rows[0].IPv6 != "" {
		t.Errorf("expected IPv6 to fallback to empty string, got %q", rows[0].IPv6)
	}
	if rows[0].IPv6Internal != "" {
		t.Errorf("expected IPv6Internal to fallback to empty string, got %q", rows[0].IPv6Internal)
	}
}

func TestGetSummaryRowsIPv6Extraction(t *testing.T) {
	scheme := runtime.NewScheme()

	// Node 1: CR with directManagement.ipv6 and exposedPorts loadBalancerAddress containing ":"
	node1CR := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "c9s.run/v1alpha1",
			"kind":       "Node",
			"metadata": map[string]interface{}{
				"name":      "node1-cr-ipv6",
				"namespace": "lab-testlab",
			},
			"spec": map[string]interface{}{
				"kind":  "nokia_srlinux",
				"image": "ghcr.io/nokia/srlinux:latest",
			},
			"status": map[string]interface{}{
				"readiness": "ready",
				"exposedPorts": map[string]interface{}{
					"loadBalancerAddress": "2001:db8:ext::1",
				},
				"directManagement": map[string]interface{}{
					"ipv4": "172.20.20.1/24",
					"ipv6": "2001:db8:int::1/64",
				},
			},
		},
	}

	// Node 2: Pod with PodIPs dual-stack and Service with Ingress dual-stack
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node2-pod",
			Namespace: "lab-testlab",
			Labels: map[string]string{
				"c9s.run/name": "node2-dual",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "node2", Image: "alpine:latest"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.244.0.25",
			PodIPs: []corev1.PodIP{
				{IP: "10.244.0.25"},
				{IP: "fd00:10::25"},
			},
		},
	}
	svc2 := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node2-dual",
			Namespace: "lab-testlab",
			Labels: map[string]string{
				"c9s.run/topologyNode":        "node2-dual",
				"c9s.run/topologyServiceType": "expose",
			},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "172.18.255.25"},
					{IP: "2001:db8:lb::25"},
				},
			},
		},
	}

	// Node 3: Pod with single-stack IPv6 PodIP (PodIPs empty)
	pod3 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node3-pod",
			Namespace: "lab-testlab",
			Labels: map[string]string{
				"c9s.run/name": "node3-single-v6",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "node3", Image: "alpine:latest"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "fd00:30::99",
		},
	}

	// Node 4: IPv4-only device (no IPv6)
	pod4 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node4-pod",
			Namespace: "lab-testlab",
			Labels: map[string]string{
				"c9s.run/name": "node4-v4-only",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "node4", Image: "alpine:latest"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.244.0.40",
		},
	}

	client := &Client{
		KubeClient: fake.NewSimpleClientset(pod2, svc2, pod3, pod4),
		DynamicClient: dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
			scheme,
			map[schema.GroupVersionResource]string{
				nodeGVR: "NodeList",
			},
			node1CR,
		),
		ContextName: "kind-try-c9s",
		ClusterName: "kind-try-c9s",
	}

	rows, err := client.GetSummaryRows(context.Background(), "testlab", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(rows))
	}

	// Row 0: node1-cr-ipv6
	if rows[0].NodeName != "node1-cr-ipv6" {
		t.Errorf("row 0 node name mismatch: got %q", rows[0].NodeName)
	}
	if rows[0].IPv4 != "N/A" {
		t.Errorf("row 0 expected IPv4 N/A, got %q", rows[0].IPv4)
	}
	if rows[0].IPv4Internal != "172.20.20.1/24" {
		t.Errorf("row 0 expected IPv4Internal 172.20.20.1/24, got %q", rows[0].IPv4Internal)
	}
	if rows[0].IPv6 != "2001:db8:ext::1" {
		t.Errorf("row 0 expected IPv6 2001:db8:ext::1, got %q", rows[0].IPv6)
	}
	if rows[0].IPv6Internal != "2001:db8:int::1/64" {
		t.Errorf("row 0 expected IPv6Internal 2001:db8:int::1/64, got %q", rows[0].IPv6Internal)
	}

	// Row 1: node2-dual
	if rows[1].NodeName != "node2-dual" {
		t.Errorf("row 1 node name mismatch: got %q", rows[1].NodeName)
	}
	if rows[1].IPv4 != "172.18.255.25" {
		t.Errorf("row 1 expected IPv4 172.18.255.25, got %q", rows[1].IPv4)
	}
	if rows[1].IPv4Internal != "10.244.0.25" {
		t.Errorf("row 1 expected IPv4Internal 10.244.0.25, got %q", rows[1].IPv4Internal)
	}
	if rows[1].IPv6 != "2001:db8:lb::25" {
		t.Errorf("row 1 expected IPv6 2001:db8:lb::25, got %q", rows[1].IPv6)
	}
	if rows[1].IPv6Internal != "fd00:10::25" {
		t.Errorf("row 1 expected IPv6Internal fd00:10::25, got %q", rows[1].IPv6Internal)
	}

	// Row 2: node3-single-v6
	if rows[2].NodeName != "node3-single-v6" {
		t.Errorf("row 2 node name mismatch: got %q", rows[2].NodeName)
	}
	if rows[2].IPv4Internal != "N/A" {
		t.Errorf("row 2 expected IPv4Internal N/A, got %q", rows[2].IPv4Internal)
	}
	if rows[2].IPv6Internal != "fd00:30::99" {
		t.Errorf("row 2 expected IPv6Internal fd00:30::99, got %q", rows[2].IPv6Internal)
	}
	if rows[2].IPv6 != "" {
		t.Errorf("row 2 expected IPv6 empty, got %q", rows[2].IPv6)
	}

	// Row 3: node4-v4-only
	if rows[3].NodeName != "node4-v4-only" {
		t.Errorf("row 3 node name mismatch: got %q", rows[3].NodeName)
	}
	if rows[3].IPv4Internal != "10.244.0.40" {
		t.Errorf("row 3 expected IPv4Internal 10.244.0.40, got %q", rows[3].IPv4Internal)
	}
	if rows[3].IPv6 != "" {
		t.Errorf("row 3 expected IPv6 empty, got %q", rows[3].IPv6)
	}
	if rows[3].IPv6Internal != "" {
		t.Errorf("row 3 expected IPv6Internal empty, got %q", rows[3].IPv6Internal)
	}
}

func TestGetSummaryRowsNodeCRDoesNotFallbackToPodIP(t *testing.T) {
	scheme := runtime.NewScheme()

	// Node 1: Has a Node CR (IPv4 only in directManagement), and also has a dual-stack Pod with IPv6 CNI overlay IP
	node1CR := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "c9s.run/v1alpha1",
			"kind":       "Node",
			"metadata": map[string]interface{}{
				"name":      "cr-node-v4only",
				"namespace": "lab-testlab",
			},
			"spec": map[string]interface{}{
				"kind":  "nokia_srlinux",
				"image": "ghcr.io/nokia/srlinux:latest",
			},
			"status": map[string]interface{}{
				"readiness": "ready",
				"exposedPorts": map[string]interface{}{
					"loadBalancerAddress": "172.18.255.10",
				},
				"directManagement": map[string]interface{}{
					"ipv4": "172.20.20.2/24",
					// directManagement.ipv6 is omitted (IPv4-only lab)
				},
			},
		},
	}
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cr-node-v4only-pod",
			Namespace: "lab-testlab",
			Labels: map[string]string{
				"c9s.run/name": "cr-node-v4only",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "srl", Image: "ghcr.io/nokia/srlinux:latest"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.244.0.100",
			PodIPs: []corev1.PodIP{
				{IP: "10.244.0.100"},
				{IP: "fd00:10:244::100"}, // Kubernetes dual-stack CNI overlay IP
			},
		},
	}

	// Node 2: Standalone Pod (no Node CR in crMap), pure pod auto-discovery fallback
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "standalone-dual-pod",
			Namespace: "lab-testlab",
			Labels: map[string]string{
				"c9s.run/name": "standalone-dual",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "client", Image: "alpine:latest"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.244.0.200",
			PodIPs: []corev1.PodIP{
				{IP: "10.244.0.200"},
				{IP: "fd00:10:244::200"},
			},
		},
	}

	client := &Client{
		KubeClient: fake.NewSimpleClientset(pod1, pod2),
		DynamicClient: dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
			scheme,
			map[schema.GroupVersionResource]string{
				nodeGVR: "NodeList",
			},
			node1CR,
		),
		ContextName: "kind-try-c9s",
		ClusterName: "kind-try-c9s",
	}

	rows, err := client.GetSummaryRows(context.Background(), "testlab", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// 1) cr-node-v4only: MUST NOT pick up p.Status.PodIPs IPv6
	if rows[0].NodeName != "cr-node-v4only" {
		t.Errorf("expected row 0 cr-node-v4only, got %q", rows[0].NodeName)
	}
	if rows[0].IPv4 != "172.18.255.10" {
		t.Errorf("expected row 0 IPv4 172.18.255.10, got %q", rows[0].IPv4)
	}
	if rows[0].IPv4Internal != "172.20.20.2/24" {
		t.Errorf("expected row 0 IPv4Internal from CR directManagement 172.20.20.2/24, got %q", rows[0].IPv4Internal)
	}
	if rows[0].IPv6 != "" {
		t.Errorf("expected row 0 IPv6 to be empty, got %q", rows[0].IPv6)
	}
	if rows[0].IPv6Internal != "" {
		t.Errorf("expected row 0 IPv6Internal to be empty (must NOT pick up CNI pod overlay IP), got %q", rows[0].IPv6Internal)
	}

	// 2) standalone-dual: pure pod fallback MUST pick up IPv6 from p.Status.PodIPs
	if rows[1].NodeName != "standalone-dual" {
		t.Errorf("expected row 1 standalone-dual, got %q", rows[1].NodeName)
	}
	if rows[1].IPv4Internal != "10.244.0.200" {
		t.Errorf("expected row 1 IPv4Internal from PodIP 10.244.0.200, got %q", rows[1].IPv4Internal)
	}
	if rows[1].IPv6Internal != "fd00:10:244::200" {
		t.Errorf("expected row 1 IPv6Internal from PodIPs fd00:10:244::200, got %q", rows[1].IPv6Internal)
	}
}

func TestListManagedLabs(t *testing.T) {
	namespaces := []runtime.Object{
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "lab-alpha",
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "c9s",
				},
			},
		},
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "lab-beta",
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "c9s",
				},
			},
		},
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "lab-unmanaged",
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "other",
				},
			},
		},
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default",
			},
		},
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "lab-",
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "c9s",
				},
			},
		},
	}

	client := &Client{
		KubeClient: fake.NewSimpleClientset(namespaces...),
	}

	labs, err := client.ListManagedLabs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(labs) != 2 {
		t.Fatalf("expected 2 labs, got %d: %v", len(labs), labs)
	}

	if labs[0] != "alpha" || labs[1] != "beta" {
		t.Errorf("unexpected labs: %v, expected [alpha beta]", labs)
	}
}
