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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=63
type MinerType string

const (
	MinerTypeSmall  MinerType = "small"
	MinerTypeMedium MinerType = "medium"
	MinerTypeLarge  MinerType = "large"
)

// MinerPhase is the phase of a miner at the current time.
type MinerPhase string

const (
	// MinerPhasePending means the miner has been accepted by the system, but one or more of the
	// required resources have not been created.
	MinerPhasePending MinerPhase = "Pending"

	// MinerPhaseProvisioning means the system is provisioning infrastructure for the miner.
	MinerPhaseProvisioning MinerPhase = "Provisioning"

	// MinerPhaseRunning means the miner has become a running miner and is ready to mine.
	MinerPhaseRunning MinerPhase = "Running"

	// MinerPhaseDeleting means the miner has been requested to be deleted and a deletion
	// timestamp is set.
	MinerPhaseDeleting MinerPhase = "Deleting"

	// MinerPhaseFailed means the system may require user intervention.
	MinerPhaseFailed MinerPhase = "Failed"
)

// MinerSpec defines the desired state of Miner
type MinerSpec struct {
	// DisplayName is the display name of the miner.
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// MinerType is the type of the miner.
	// +kubebuilder:validation:Enum=small;medium;large
	// +optional
	MinerType MinerType `json:"minerType,omitempty"`

	// ChainName is the name of the chain this miner belongs to.
	// +kubebuilder:validation:MinLength=1
	ChainName string `json:"chainName"`

	// RestartPolicy for the miner.
	// +kubebuilder:validation:Enum=Always;OnFailure;Never
	// +optional
	RestartPolicy corev1.RestartPolicy `json:"restartPolicy,omitempty"`

	// PodDeletionTimeout defines how long the controller will attempt to delete the pod.
	// A duration of 0 will retry deletion indefinitely.
	// Defaults to 10 seconds.
	// +optional
	PodDeletionTimeout *metav1.Duration `json:"podDeletionTimeout,omitempty"`
}

// MinerStatus defines the observed state of Miner
type MinerStatus struct {
	// PodRef will point to the corresponding pod if it exists.
	// +optional
	PodRef *corev1.ObjectReference `json:"podRef,omitempty"`

	// LastUpdated identifies when this status was last observed.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the miner.
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the miner.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Addresses is a list of addresses assigned to the miner.
	// +optional
	Addresses []string `json:"addresses,omitempty"`

	// Phase represents the current phase of miner actuation.
	// One of: Failed, Provisioning, Pending, Running, Deleting
	// +optional
	Phase MinerPhase `json:"phase,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the miner's current state.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Miner is the Schema for the miners API
type Miner struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MinerSpec   `json:"spec,omitempty"`
	Status MinerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MinerList contains a list of Miner
type MinerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Miner `json:"items"`
}

// GetConditions returns the conditions of the miner.
func (m *Miner) GetConditions() []metav1.Condition {
	return m.Status.Conditions
}

// SetConditions sets the conditions of the miner.
func (m *Miner) SetConditions(conditions []metav1.Condition) {
	m.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&Miner{}, &MinerList{})
}
