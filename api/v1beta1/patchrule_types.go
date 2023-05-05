/*
Copyright 2023 Doodle.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResumeProfileSpec defines the desired state of ResumeProfile
type ResumeProfileSpec struct {
	// Prometheus holds information about where to find prometheus
	// +required
	PodSelector []metav1.LabelSelector `json:"podSelector"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Active",type="string",JSONPath=".status.conditions[?(@.type==\"Active\")].status",description=""
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type==\"Active\")].reason",description=""
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// ResumeProfile is the Schema for the patchrules API
type ResumeProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ResumeProfileSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// ResumeProfileList contains a list of ResumeProfile
type ResumeProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResumeProfile `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResumeProfile{}, &ResumeProfileList{})
}
