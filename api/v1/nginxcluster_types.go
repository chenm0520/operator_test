/*
Copyright 2025.

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

// NginxClusterSpec defines the desired state of NginxCluster
type NginxClusterSpec struct {
	// Replicas is the number of nginx instances
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	Replicas int32 `json:"replicas,omitempty"`

	// Image is the nginx image to use
	// +kubebuilder:default="nginx:latest"
	Image string `json:"image,omitempty"`

	// NginxConf is the nginx configuration content
	NginxConf string `json:"nginxConf,omitempty"`
}

// NginxClusterStatus defines the observed state of NginxCluster
type NginxClusterStatus struct {
	// Replicas is the current number of replicas
	Replicas int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the number of ready replicas
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// ConfigHash is the hash of current nginx config
	ConfigHash string `json:"configHash,omitempty"`

	// LastUpdateTime is the timestamp of last configuration update
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.spec.replicas`
//+kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
//+kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image`
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// NginxCluster is the Schema for the nginxclusters API
type NginxCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NginxClusterSpec   `json:"spec,omitempty"`
	Status NginxClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NginxClusterList contains a list of NginxCluster
type NginxClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NginxCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NginxCluster{}, &NginxClusterList{})
}
