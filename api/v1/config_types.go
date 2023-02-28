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

// ConfigSpec defines the desired state of Config
type ConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Enum=ArgoCD;Plain
	// Wether output is processed as argocd secret object or plain secret
	DesiredOutput string `json:"desiredoutput"`
	// The Gardener Project Name
	Project string `json:"project"`
	// The Name of the shoot cluster to generate a secret for
	Shoot string `json:"shoot"`
	// +kubebuilder:default=""
	// The stage of the cluster
	Stage string `json:"stage,omitempty"`
	// +kubebuilder:default=""
	// The Cloudprovider where the cluster runs
	CloudProvider string `json:"cloudprovider,omitempty"`
	// The Frequency to Generate new Tokens
	Frequency *metav1.Duration `json:"frequency"`
}

// ConfigStatus defines the observed state of Config
type ConfigStatus struct {
	Phase           string       `json:"phase,omitempty"`
	LastUpdatedTime *metav1.Time `json:"lastUpdatedTime,omitempty"`
	ProjectName     string       `json:"projectName,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Config is the Schema for the configs API
type Config struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigSpec   `json:"spec,omitempty"`
	Status ConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ConfigList contains a list of Config
type ConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Config `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Config{}, &ConfigList{})
}
