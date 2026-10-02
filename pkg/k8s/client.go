package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"c9s/pkg/crd"
	"c9s/pkg/safeguards"
	"c9s/pkg/ui"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	topologyGVR = schema.GroupVersionResource{
		Group:    crd.GroupName,
		Version:  crd.GroupVersion,
		Resource: "topologies",
	}

	nodeGVR = schema.GroupVersionResource{
		Group:    crd.GroupName,
		Version:  crd.GroupVersion,
		Resource: "nodes",
	}
)

// Client wraps Kubernetes typed and dynamic clients with strict safety checks.
type Client struct {
	KubeClient    kubernetes.Interface
	DynamicClient dynamic.Interface
	ContextName   string
	ClusterName   string
}

// NewClient creates and validates a Kubernetes client.
func NewClient(kubeconfigPath, contextOverride string) (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}

	tempConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	currentContext, clusterName, err := safeguards.ValidateContextAndCluster(tempConfig, contextOverride)
	if err != nil {
		return nil, err
	}

	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: currentContext,
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build kubernetes rest config: %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &Client{
		KubeClient:    kubeClient,
		DynamicClient: dynClient,
		ContextName:   currentContext,
		ClusterName:   clusterName,
	}, nil
}

// ApplyNamespace idempotently creates or updates the target lab namespace.
func (c *Client) ApplyNamespace(ctx context.Context, ns *corev1.Namespace) error {
	if err := safeguards.EnforceNamespaceSafety(ns.Name); err != nil {
		return err
	}

	existing, err := c.KubeClient.CoreV1().Namespaces().Get(ctx, ns.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err := c.KubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create namespace %q: %w", ns.Name, err)
			}
			return nil
		}
		return fmt.Errorf("failed to get namespace %q: %w", ns.Name, err)
	}

	// Update labels if needed
	needsUpdate := false
	if existing.Labels == nil {
		existing.Labels = make(map[string]string)
	}
	for k, v := range ns.Labels {
		if existing.Labels[k] != v {
			existing.Labels[k] = v
			needsUpdate = true
		}
	}

	if needsUpdate {
		_, err = c.KubeClient.CoreV1().Namespaces().Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update namespace %q: %w", ns.Name, err)
		}
	}

	return nil
}

// ApplyConfigMap idempotently creates or updates a ConfigMap.
func (c *Client) ApplyConfigMap(ctx context.Context, cm *corev1.ConfigMap) error {
	if err := safeguards.EnforceNamespaceSafety(cm.Namespace); err != nil {
		return err
	}

	existing, err := c.KubeClient.CoreV1().ConfigMaps(cm.Namespace).Get(ctx, cm.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err := c.KubeClient.CoreV1().ConfigMaps(cm.Namespace).Create(ctx, cm, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create configmap %q in %q: %w", cm.Name, cm.Namespace, err)
			}
			return nil
		}
		return fmt.Errorf("failed to get configmap %q in %q: %w", cm.Name, cm.Namespace, err)
	}

	existing.Data = cm.Data
	existing.Labels = cm.Labels
	_, err = c.KubeClient.CoreV1().ConfigMaps(cm.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update configmap %q in %q: %w", cm.Name, cm.Namespace, err)
	}

	return nil
}

// ApplyTopology idempotently applies the Clabernetes Topology Custom Resource.
func (c *Client) ApplyTopology(ctx context.Context, topo *crd.Topology) error {
	if err := safeguards.EnforceNamespaceSafety(topo.Namespace); err != nil {
		return err
	}

	// Convert typed Topology to unstructured
	rawJSON, err := json.Marshal(topo)
	if err != nil {
		return fmt.Errorf("failed to marshal topology CR: %w", err)
	}

	var unstruct unstructured.Unstructured
	if err := json.Unmarshal(rawJSON, &unstruct.Object); err != nil {
		return fmt.Errorf("failed to unmarshal into unstructured topology: %w", err)
	}

	res := c.DynamicClient.Resource(topologyGVR).Namespace(topo.Namespace)

	existing, err := res.Get(ctx, topo.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = res.Create(ctx, &unstruct, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create Topology CR %q: %w", topo.Name, err)
			}
			return nil
		}
		return fmt.Errorf("failed to get Topology CR %q: %w", topo.Name, err)
	}

	// Update existing CR
	unstruct.SetResourceVersion(existing.GetResourceVersion())
	_, err = res.Update(ctx, &unstruct, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update Topology CR %q: %w", topo.Name, err)
	}

	return nil
}

// DeleteLab deletes the Topology CR and the lab's dedicated namespace.
func (c *Client) DeleteLab(ctx context.Context, labName string) error {
	nsName, err := safeguards.DeriveNamespace(labName)
	if err != nil {
		return err
	}

	if err := safeguards.EnforceNamespaceSafety(nsName); err != nil {
		return err
	}

	// Verify namespace exists and is managed by c9s
	nsObj, err := c.KubeClient.CoreV1().Namespaces().Get(ctx, nsName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			ui.Info("Namespace %q does not exist; nothing to destroy", nsName)
			return nil
		}
		return fmt.Errorf("failed to inspect namespace %q: %w", nsName, err)
	}

	labels := nsObj.GetLabels()
	if labels == nil || labels["app.kubernetes.io/managed-by"] != "c9s" {
		return fmt.Errorf("SECURITY REFUSAL: Namespace %q was not created by c9s (missing 'app.kubernetes.io/managed-by: c9s' label). Refusing to delete", nsName)
	}

	ui.Info("Removing Clabernetes Topology resource %q in %q...", labName, nsName)
	_ = c.DynamicClient.Resource(topologyGVR).Namespace(nsName).Delete(ctx, labName, metav1.DeleteOptions{})

	ui.Info("Deleting lab namespace %q...", nsName)
	propagation := metav1.DeletePropagationForeground
	err = c.KubeClient.CoreV1().Namespaces().Delete(ctx, nsName, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete namespace %q: %w", nsName, err)
	}

	return nil
}

// GetSummaryRows inspects the lab resources and builds summary table rows.
func (c *Client) GetSummaryRows(ctx context.Context, labName string, expectedNodes map[string]string) ([]ui.TableRow, error) {
	nsName, err := safeguards.DeriveNamespace(labName)
	if err != nil {
		return nil, err
	}

	nodeCRs, _ := c.DynamicClient.Resource(nodeGVR).Namespace(nsName).List(ctx, metav1.ListOptions{})
	pods, _ := c.KubeClient.CoreV1().Pods(nsName).List(ctx, metav1.ListOptions{})
	services, _ := c.KubeClient.CoreV1().Services(nsName).List(ctx, metav1.ListOptions{})

	// Map CR nodes
	crMap := make(map[string]*unstructured.Unstructured)
	if nodeCRs != nil {
		for i := range nodeCRs.Items {
			item := &nodeCRs.Items[i]
			crMap[item.GetName()] = item
		}
	}

	// Map pods
	podMap := make(map[string]*corev1.Pod)
	if pods != nil {
		for i := range pods.Items {
			p := &pods.Items[i]
			if nodeName, ok := p.Labels["c9s.run/name"]; ok {
				podMap[nodeName] = p
			} else if nodeName, ok := p.Labels["c9s.run/topologyNode"]; ok {
				podMap[nodeName] = p
			}
		}
	}

	// Map services
	svcMap := make(map[string]*corev1.Service)
	if services != nil {
		for i := range services.Items {
			s := &services.Items[i]
			svcMap[s.Name] = s
			if s.Labels != nil && s.Labels["c9s.run/topologyServiceType"] == "expose" {
				if node, ok := s.Labels["c9s.run/topologyNode"]; ok {
					svcMap[node] = s
				} else if node, ok := s.Labels["c9s.run/name"]; ok {
					svcMap[node] = s
				}
			}
		}
	}

	if len(expectedNodes) == 0 {
		expectedNodes = make(map[string]string)
		for name, cr := range crMap {
			kind := "unknown"
			if k, found, _ := unstructured.NestedString(cr.Object, "spec", "kind"); found && k != "" {
				kind = k
			}
			expectedNodes[name] = kind
		}
		for name := range podMap {
			if _, exists := expectedNodes[name]; !exists {
				expectedNodes[name] = "unknown"
			}
		}
	}

	var rows []ui.TableRow
	nodeNames := make([]string, 0, len(expectedNodes))
	for nodeName := range expectedNodes {
		nodeNames = append(nodeNames, nodeName)
	}
	sort.Strings(nodeNames)

	idx := 1
	for _, nodeName := range nodeNames {
		kind := expectedNodes[nodeName]
		image := "unknown"
		state := "deploying"
		ipv4Ext := "N/A"
		ipv4Int := "N/A"
		ipv6Ext := ""
		ipv6Int := ""

		hasCR := false
		if cr, ok := crMap[nodeName]; ok {
			hasCR = true
			if img, found, _ := unstructured.NestedString(cr.Object, "spec", "image"); found {
				image = img
			}
			if k, found, _ := unstructured.NestedString(cr.Object, "spec", "kind"); found {
				kind = k
			}
			if readiness, found, _ := unstructured.NestedString(cr.Object, "status", "readiness"); found {
				state = readiness
			}
			if lb, found, _ := unstructured.NestedString(cr.Object, "status", "exposedPorts", "loadBalancerAddress"); found && lb != "" {
				if strings.Contains(lb, ":") {
					ipv6Ext = lb
				} else {
					ipv4Ext = lb
				}
			}
			if ip, found, _ := unstructured.NestedString(cr.Object, "status", "directManagement", "ipv4"); found && ip != "" {
				ipv4Int = ip
			}
			if ip6, found, _ := unstructured.NestedString(cr.Object, "status", "directManagement", "ipv6"); found && ip6 != "" {
				ipv6Int = ip6
			}
		}

		if svc, ok := svcMap[nodeName]; ok {
			for _, ingress := range svc.Status.LoadBalancer.Ingress {
				if ingress.IP != "" {
					if strings.Contains(ingress.IP, ":") {
						if ipv6Ext == "" {
							ipv6Ext = ingress.IP
						}
					} else {
						if ipv4Ext == "N/A" {
							ipv4Ext = ingress.IP
						}
					}
				}
			}
		}

		if p, ok := podMap[nodeName]; ok {
			if image == "unknown" && len(p.Spec.Containers) > 0 {
				image = p.Spec.Containers[0].Image
			}
			if state == "deploying" {
				state = string(p.Status.Phase)
			}
			// Only fall back to Pod IPs when there is no Node CR (pure Pod auto-discovery fallback).
			// When a Node CR exists, management IPs are managed by containerlab in directManagement.
			if !hasCR {
				if ipv4Int == "N/A" {
					if p.Status.PodIP != "" && !strings.Contains(p.Status.PodIP, ":") {
						ipv4Int = p.Status.PodIP
					} else {
						for _, pip := range p.Status.PodIPs {
							if pip.IP != "" && !strings.Contains(pip.IP, ":") {
								ipv4Int = pip.IP
								break
							}
						}
					}
				}
				if ipv6Int == "" {
					for _, pip := range p.Status.PodIPs {
						if strings.Contains(pip.IP, ":") {
							ipv6Int = pip.IP
							break
						}
					}
					if ipv6Int == "" && strings.Contains(p.Status.PodIP, ":") {
						ipv6Int = p.Status.PodIP
					}
				}
			}
		}

		rows = append(rows, ui.TableRow{
			Index:        idx,
			LabName:      labName,
			Namespace:    nsName,
			NodeName:     nodeName,
			Kind:         kind,
			Image:        image,
			State:        strings.Title(state),
			IPv4:         ipv4Ext,
			IPv4Internal: ipv4Int,
			IPv6:         ipv6Ext,
			IPv6Internal: ipv6Int,
		})
		idx++
	}

	return rows, nil
}

// ListManagedLabs lists all lab names currently managed by c9s.
func (c *Client) ListManagedLabs(ctx context.Context) ([]string, error) {
	opts := metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=c9s",
	}
	nsList, err := c.KubeClient.CoreV1().Namespaces().List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	var labs []string
	for _, ns := range nsList.Items {
		if strings.HasPrefix(ns.Name, safeguards.NamespacePrefix) {
			if ns.Labels != nil && ns.Labels["app.kubernetes.io/managed-by"] == "c9s" {
				labName := strings.TrimPrefix(ns.Name, safeguards.NamespacePrefix)
				if labName != "" {
					labs = append(labs, labName)
				}
			}
		}
	}
	sort.Strings(labs)
	return labs, nil
}
