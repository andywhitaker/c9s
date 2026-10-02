package crd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName    = "c9s.run"
	GroupVersion = "v1alpha1"
	KindTopology = "Topology"
)

var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: GroupVersion}

// Topology is the Clabernetes custom resource for running a containerlab topology.
type Topology struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TopologySpec   `json:"spec"`
	Status TopologyStatus `json:"status,omitempty"`
}

type TopologySpec struct {
	Definition TopologyDefinitionSpec `json:"definition"`
	Deployment TopologyDeploymentSpec `json:"deployment,omitempty"`
}

type TopologyDefinitionSpec struct {
	Containerlab string `json:"containerlab"`
}

type TopologyDeploymentSpec struct {
	FilesFromConfigMap map[string][]FileFromConfigMap `json:"filesFromConfigMap,omitempty"`
}

type FileFromConfigMap struct {
	FilePath      string `json:"filePath"`                // Destination path in container
	ConfigMapName string `json:"configMapName"`             // ConfigMap resource name
	ConfigMapPath string `json:"configMapPath"`             // Key in ConfigMap (filename)
	Mode          string `json:"mode,omitempty"`           // "read" or "execute"
}

type TopologyStatus struct {
	TopologyReady  bool     `json:"topologyReady,omitempty"`
	TopologyState  string   `json:"topologyState,omitempty"`
	NodeCount      int      `json:"nodeCount,omitempty"`
	ReadyNodeCount int      `json:"readyNodeCount,omitempty"`
	LinkCount      int      `json:"linkCount,omitempty"`
}
