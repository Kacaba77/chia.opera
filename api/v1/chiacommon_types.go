/*
Copyright 2023 Chia Network, Inc.
*/

package v1

// ChiaExporterConfigSpec defines the desired state of Chia exporter configuration
type ChiaExporterConfigSpec struct {
	// Image defines the image to use for the chia exporter containers
	// +kubebuilder:default="ghcr.io/chia-network/chia-exporter:latest"
	// +optional
	Image string `json:"image"`

	// Labels is a map of string keys and values to attach to the chia exporter k8s Service
	// +optional
	ServiceLabels map[string]string `json:"serviceLabels,omitempty"`
}

// ChiaKeysSpec defines the name of a kubernetes secret and key in that secret that contains the Chia mnemonic
type ChiaKeysSpec struct {
	// SecretName is the name of the kubernetes secret containing a mnemonic key
	Name string `json:"name"`

	// Key is the key of the data item in the Secret
	Key string `json:"key"`
}

// AdditionalMetadata contains labels and annotations to attach to created objects
type AdditionalMetadata struct {
	// Labels is a map of string keys and values to attach to created objects
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations is a map of string keys and values to attach to created objects
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

/*
Full storage config example:

storage:
  chiaRoot:
    // Only one of persistentVolumeClaim or hostPathVolume should be specified, persistentVolumeClaim will be preferred if both are specified
    persistentVolumeClaim:
	  claimName: "chiaroot-data"
	hostPathVolume:
      path: "/home/user/storage/chiaroot"

  plots:
    persistentVolumeClaim:
	  - claimName: "plot1"
	  - claimName: "plot2"
	hostPathVolume:
	  - path: "/home/user/storage/plots1"
	  - path: "/home/user/storage/plots2"
*/

// StorageConfig contains storage configuration settings
type StorageConfig struct {
	// Storage configuration for CHIA_ROOT
	// +optional
	ChiaRoot *ChiaRootConfig `json:"chiaRoot,omitempty"`

	// Storage configuration for harvester plots
	// +optional
	Plots *PlotsConfig `json:"plots,omitempty"`
}

// ChiaRootConfig optional config for CHIA_ROOT persistent storage, likely only needed for Chia full_nodes, but may help in startup time for other components.
// Both options may be specified but only one can be used, therefore PersistentVolumeClaims will be respected over HostPath volumes if both are specified.
type ChiaRootConfig struct {
	// PersistentVolumeClaim use an existing persistent volume claim to store CHIA_ROOT data
	// +optional
	PersistentVolumeClaim *PersistentVolumeClaimConfig `json:"persistentVolumeClaim,omitempty"`

	// HostPathVolume use an existing persistent volume claim to store CHIA_ROOT data
	// +optional
	HostPathVolume *HostPathVolumeConfig `json:"hostPathVolume,omitempty"`
}

// PlotsConfig optional config for harvester plots persistent storage, only needed for Chia harvesters.
// Supports adding both PVCs and hostPath volumes.
type PlotsConfig struct {
	// PersistentVolumeClaim use an existing persistent volume claim to mount plot directories
	// +optional
	PersistentVolumeClaim []*PersistentVolumeClaimConfig `json:"persistentVolumeClaim,omitempty"`

	// HostPathVolume use an existing directory on the host to mount plot directories
	// +optional
	HostPathVolume []*HostPathVolumeConfig `json:"hostPathVolume,omitempty"`
}

// PersistentVolumeClaimConfig config for PVC volumes in kubernetes
type PersistentVolumeClaimConfig struct {
	// ClaimName is the name of an existing PersistentVolumeClaim in the target namespace
	// +optional
	ClaimName string `json:"claimName,omitempty"`

	// StorageClass is the name of a storage class for the PVC -- this is only relevant for ChiaNode objects and is ignored for others
	// +kubebuilder:default=""
	// +optional
	StorageClass string `json:"storageClass,omitempty"`

	// StorageClass is the amount of storage requested -- this is only relevant for ChiaNode objects and is ignored for others
	// +optional
	ResourceRequest string `json:"resourceRequest,omitempty"`
}

// HostPathVolumeConfig config for hostPath volumes in kubernetes
type HostPathVolumeConfig struct {
	// Path use an existing directory on your Pod's host to mount in the Pod's containers.
	// If a HostPath is used, it is highly recommended that a NodeSelector is used to keep the Pod on the host that has the directory to mount.
	// +optional
	Path string `json:"path,omitempty"`
}
