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
	// MinerSetFinalizer is the finalizer used by the MinerSet controller to
	// clean up referenced template resources if necessary when a MinerSet is being deleted.
	MinerSetFinalizer = "minerset.onex.io/finalizer"
)

// MinerTemplateSpec defines the miner template
type MinerTemplateSpec struct {
	// Standard object's metadata.
	// +optional
	ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the desired behavior of the miner.
	// +optional
	Spec MinerSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// ObjectMeta is metadata that will be autopopulated for the pod created.
type ObjectMeta struct {
	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects.
	// +optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,1,rep,name=labels"`

	// Annotations is an unstructured key value map stored with an object.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,2,rep,name=annotations"`
}

// DeletePolicy defines the delete policy for the pods when a MinerSet scales down.
type DeletePolicy string

const (
	// DeletePolicyRandom chooses a random pod to delete.
	DeletePolicyRandom DeletePolicy = "Random"

	// DeletePolicyNewest deletes the newest pod.
	DeletePolicyNewest DeletePolicy = "Newest"

	// DeletePolicyOldest deletes the oldest pod.
	DeletePolicyOldest DeletePolicy = "Oldest"
)

// MinerSetSpec defines the desired state of MinerSet
type MinerSetSpec struct {
	// Replicas is the number of desired replicas.
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Selector is a label query over pods that should match the replica count.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
	// +optional
	Selector metav1.LabelSelector `json:"selector,omitempty"`

	// Template is the object that describes the miner that will be created
	// if insufficient replicas are detected.
	Template MinerTemplateSpec `json:"template,omitempty"`

	// DisplayName is the display name of the MinerSet.
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// DeletePolicy defines the delete policy for the pods when a MinerSet scales down.
	// Default to Random.
	// +kubebuilder:validation:Enum=Random;Newest;Oldest
	// +optional
	DeletePolicy DeletePolicy `json:"deletePolicy,omitempty"`

	// MinReadySeconds is the minimum number of seconds for which a newly created pod should
	// be ready without any of its container crashing, for it to be considered available.
	// Defaults to 0.
	// +optional
	MinReadySeconds int32 `json:"minReadySeconds,omitempty"`

	// ProgressDeadlineSeconds is the maximum duration in seconds that a MinerSet may take
	// to progress, before it is considered to be failed.
	// Defaults to 600 seconds.
	// +kubebuilder:validation:Minimum=1
	// +optional
	ProgressDeadlineSeconds *int32 `json:"progressDeadlineSeconds,omitempty"`
}

// MinerSetStatus defines the observed state of MinerSet
type MinerSetStatus struct {
	// Replicas is the most recently observed number of replicas.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// FullyLabeledReplicas is the number of pods that have all of the requested labels.
	// +optional
	FullyLabeledReplicas int32 `json:"fullyLabeledReplicas,omitempty"`

	// ReadyReplicas is the number of ready pods.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// AvailableReplicas is the number of available pods.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the MinerSet.
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the MinerSet.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions represent the latest available observations of the MinerSet's current state.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MinerSet is the Schema for the minersets API
type MinerSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MinerSetSpec   `json:"spec,omitempty"`
	Status MinerSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MinerSetList contains a list of MinerSet
type MinerSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MinerSet `json:"items"`
}

// GetConditions returns the conditions of the minerset.
func (ms *MinerSet) GetConditions() []metav1.Condition {
	return ms.Status.Conditions
}

// SetConditions sets the conditions of the minerset.
func (ms *MinerSet) SetConditions(conditions []metav1.Condition) {
	ms.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&MinerSet{}, &MinerSetList{})
}
