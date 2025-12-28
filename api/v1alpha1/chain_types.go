/*
Copyright 2025 OneX Team.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ChainFinalizer is the finalizer used by the chain controller to
	// clean up referenced template resources if necessary when a chain is being deleted.
	ChainFinalizer = "chain.onex.io/finalizer"
)

// LocalObjectReference contains enough information to let you locate the
// referenced object inside the same namespace.
type LocalObjectReference struct {
	// Name of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	// +optional
	Name string `json:"name,omitempty"`
}

// ChainSpec defines the desired state of Chain
type ChainSpec struct {
	// DisplayName is the display name of the chain.
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// MinerType is the type of the genesis miner.
	// +kubebuilder:validation:Enum=small;medium;large
	// +optional
	MinerType string `json:"minerType,omitempty"`

	// Image is the blockchain node image.
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// MinMineIntervalSeconds is the minimum interval in seconds between mining operations.
	// +optional
	MinMineIntervalSeconds int32 `json:"minMineIntervalSeconds,omitempty"`

	// BootstrapAccount is the bootstrap account (will be auto-generated).
	// +optional
	BootstrapAccount *string `json:"bootstrapAccount,omitempty"`
}

// ChainStatus defines the observed state of Chain
type ChainStatus struct {
	// ConfigMapRef points to the config map that contains the chain configuration.
	// +optional
	ConfigMapRef *LocalObjectReference `json:"configMapRef,omitempty"`

	// MinerRef points to the genesis miner for this chain.
	// +optional
	MinerRef *LocalObjectReference `json:"minerRef,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the chain's current state.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Chain is the Schema for the chains API
type Chain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChainSpec   `json:"spec,omitempty"`
	Status ChainStatus `json:"status,omitempty"`
}

// GetConditions returns the conditions of the chain.
func (c *Chain) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

// SetConditions sets the conditions of the chain.
func (c *Chain) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// ChainList contains a list of Chain
type ChainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Chain `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Chain{}, &ChainList{})
}
