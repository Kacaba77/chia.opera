/*
Copyright 2023 Chia Network Inc.
*/

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChiaNodeSpec defines the desired state of ChiaNode
type ChiaNodeSpec struct {
	AdditionalMetadata `json:",inline"`

	// ChiaConfig defines the configuration options available to Chia component containers
	ChiaConfig ChiaNodeConfigSpec `json:"chia"`

	// ChiaExporterConfig defines the configuration options available to Chia component containers
	// +optional
	ChiaExporterConfig ChiaExporterConfigSpec `json:"chiaExporter"`

	//StorageConfig defines the Chia container's CHIA_ROOT storage config
	// +optional
	Storage *StorageConfig `json:"storage,omitempty"`

	// ServiceType is the type of the service that governs this ChiaNode StatefulSet.
	// +optional
	// +kubebuilder:default="ClusterIP"
	ServiceType string `json:"serviceType"`

	// ImagePullPolicy is the pull policy for containers in the pod
	// +optional
	// +kubebuilder:default="Always"
	ImagePullPolicy *corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Replicas is the desired number of replicas of the given Statefulset. defaults to 1.
	// +optional
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`

	// NodeSelector selects a node by key value pairs
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// PodSecurityContext defines the security context for the pod
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
}

// ChiaConfigSpec defines the desired state of Chia component configuration
type ChiaNodeConfigSpec struct {
	// Image defines the image to use for the chia component containers
	// +kubebuilder:default="ghcr.io/chia-network/chia:latest"
	// +optional
	Image string `json:"image"`

	// CASecretName is the name of the secret that contains the CA crt and key.
	CASecretName string `json:"caSecretName,omitempty"`

	// Testnet is set to true if the Chia container should switch to the latest default testnet's settings
	// +optional
	Testnet *bool `json:"testnet,omitempty"`

	// Timezone can be set to your local timezone for accurate timestamps. Defaults to UTC
	// +optional
	Timezone *string `json:"timezone,omitempty"`

	// LogLevel is set to the desired chia config log_level
	// +optional
	LogLevel *string `json:"logLevel,omitempty"`

	// Periodic probe of container liveness.
	// +optional
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`

	// Periodic probe of container service readiness.
	// +optional
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`

	// StartupProbe indicates that the Pod has successfully initialized.
	// +optional
	StartupProbe *corev1.Probe `json:"startupProbe,omitempty"`

	// Resources defines the compute resources for the Chia container
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// SecurityContext defines the security context for the chia container
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`
}

// ChiaNodeStatus defines the observed state of ChiaNode
type ChiaNodeStatus struct {
	// Ready says whether the node is ready, this should be true when the node statefulset is in the target namespace
	// +kubebuilder:default=false
	Ready bool `json:"ready,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ChiaNode is the Schema for the chianodes API
type ChiaNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChiaNodeSpec   `json:"spec,omitempty"`
	Status ChiaNodeStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ChiaNodeList contains a list of ChiaNode
type ChiaNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChiaNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ChiaNode{}, &ChiaNodeList{})
}
