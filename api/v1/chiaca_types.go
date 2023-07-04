/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChiaCASpec defines the desired state of ChiaCA
type ChiaCASpec struct {
	// Image defines the CA generator image to run
	// +kubebuilder:default="registry.gitlab.com/bbhome/helm/chia-operator/chiaca-generator:latest"
	Image string `json:"image,omitempty"`

	// ImagePullSecret defines an ImagePullSecret for the CA generator image
	ImagePullSecret string `json:"imagePullSecret,omitempty"`

	// Secret defines the name of the secret to contain CA files
	Secret string `json:"secret"`
}

// ChiaCAStatus defines the observed state of ChiaCA
type ChiaCAStatus struct {
	// Ready says whether the CA is ready, this should be true when the SSL secret is in the target namespace
	// +kubebuilder:default=false
	Ready bool `json:"ready,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ChiaCA is the Schema for the chiacas API
type ChiaCA struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChiaCASpec   `json:"spec,omitempty"`
	Status ChiaCAStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ChiaCAList contains a list of ChiaCA
type ChiaCAList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChiaCA `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ChiaCA{}, &ChiaCAList{})
}
