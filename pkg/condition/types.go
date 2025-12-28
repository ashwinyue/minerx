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

package condition

// ConditionType is the type of condition.
type ConditionType string

const (
	// ReadyCondition is the aggregate condition of all conditions.
	ReadyCondition ConditionType = "Ready"

	// InfrastructureReadyCondition indicates that infrastructure is ready.
	InfrastructureReadyCondition ConditionType = "InfrastructureReady"

	// BootstrapReadyCondition indicates that bootstrap is ready.
	BootstrapReadyCondition ConditionType = "BootstrapReady"

	// MinerPodHealthyCondition indicates that the miner pod is healthy.
	MinerPodHealthyCondition ConditionType = "PodHealthy"

	// MinerHealthCheckSucceededCondition indicates that health check succeeded.
	MinerHealthCheckSucceededCondition ConditionType = "HealthCheckSucceeded"

	// MinerOwnerRemediatedCondition indicates that the owner has remediated the miner.
	MinerOwnerRemediatedCondition ConditionType = "OwnerRemediated"

	// MinersCreatedCondition indicates that miners have been created.
	MinersCreatedCondition ConditionType = "MinersCreated"

	// MinersReadyCondition indicates that miners are ready.
	MinersReadyCondition ConditionType = "MinersReady"

	// ResizedCondition indicates that the miner set is being resized.
	ResizedCondition ConditionType = "Resized"

	// ConfigMapsCreatedCondition indicates that configmaps have been created.
	ConfigMapsCreatedCondition ConditionType = "ConfigMapsCreated"
)

// ConditionReason is the reason for the condition's last transition.
type ConditionReason string

const (
	// CreatingReason is the reason when creating resources.
	CreatingReason ConditionReason = "Creating"

	// CreatedReason is the reason when resources are created.
	CreatedReason ConditionReason = "Created"

	// DeletingReason is the reason when resources are being deleted.
	DeletingReason ConditionReason = "Deleting"

	// DeletedReason is the reason when resources are deleted.
	DeletedReason ConditionReason = "Deleted"

	// ProvisioningReason is the reason when resources are provisioning.
	ProvisioningReason ConditionReason = "Provisioning"

	// AvailableReason is the reason when resources are available.
	AvailableReason ConditionReason = "Available"

	// UnavailableReason is the reason when resources are unavailable.
	UnavailableReason ConditionReason = "Unavailable"

	// FailedReason is the reason when resources failed.
	FailedReason ConditionReason = "Failed"

	// PodNotFoundReason is the reason when pod is not found.
	PodNotFoundReason ConditionReason = "PodNotFound"

	// PodConditionsFailedReason is the reason when pod conditions failed.
	PodConditionsFailedReason ConditionReason = "PodConditionsFailed"

	// InvalidConfigurationReason is the reason when configuration is invalid.
	InvalidConfigurationReason ConditionReason = "InvalidConfiguration"

	// MinerCreationFailedReason is the reason when miner creation failed.
	MinerCreationFailedReason ConditionReason = "MinerCreationFailed"

	// MinerDeletionFailedReason is the reason when miner deletion failed.
	MinerDeletionFailedReason ConditionReason = "MinerDeletionFailed"
)
