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

package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/ashwinyue/minerx/api/v1alpha1"
	"github.com/ashwinyue/minerx/pkg/condition"
)

const (
	minerSetFinalizer = "minerset.onex.io/finalizer"
	minerSetNameLabel = "minerset.onex.io/name"
	chainNameLabel    = "chain.onex.io/name"

	stateConfirmationTimeout  = 10 * time.Second
	stateConfirmationInterval = 100 * time.Millisecond
)

var (
	msKind = appsv1alpha1.GroupVersion.WithKind("MinerSet")
)

// MinerSetReconciler reconciles a MinerSet object
type MinerSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps.onex.io,resources=minersets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.onex.io,resources=minersets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.onex.io,resources=minersets/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps.onex.io,resources=miners,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.onex.io,resources=miners/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *MinerSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	ms := &appsv1alpha1.MinerSet{}
	if err := r.Get(ctx, req.NamespacedName, ms); err != nil {
		if errors.IsNotFound(err) {
			log.Info("MinerSet resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get MinerSet")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling MinerSet", "Name", ms.Name, "Namespace", ms.Namespace)

	if !ms.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, ms)
	}

	return r.reconcile(ctx, ms)
}

func (r *MinerSetReconciler) reconcileDelete(ctx context.Context, ms *appsv1alpha1.MinerSet) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(ms, minerSetFinalizer) {
		controllerutil.RemoveFinalizer(ms, minerSetFinalizer)
		if err := r.Update(ctx, ms); err != nil {
			log.Error(err, "Failed to remove finalizer from MinerSet")
			return ctrl.Result{}, err
		}
	}

	log.Info("MinerSet deleted successfully")
	return ctrl.Result{}, nil
}

func (r *MinerSetReconciler) reconcile(ctx context.Context, ms *appsv1alpha1.MinerSet) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(ms, minerSetFinalizer) {
		controllerutil.AddFinalizer(ms, minerSetFinalizer)
		if err := r.Update(ctx, ms); err != nil {
			log.Error(err, "Failed to add finalizer to MinerSet")
			return ctrl.Result{}, err
		}
	}

	// Set chain name label
	if ms.Labels == nil {
		ms.Labels = make(map[string]string)
	}
	ms.Labels[chainNameLabel] = ms.Spec.Template.Spec.ChainName

	// Convert selector to map
	selectorMap, err := metav1.LabelSelectorAsMap(&ms.Spec.Selector)
	if err != nil {
		log.Error(err, "Failed to convert MinerSet label selector to a map")
		return ctrl.Result{}, err
	}

	// List all Miners managed by this MinerSet
	allMiners := &appsv1alpha1.MinerList{}
	if err := r.List(ctx, allMiners, client.InNamespace(ms.Namespace), client.MatchingLabels(selectorMap)); err != nil {
		log.Error(err, "Failed to list miners")
		return ctrl.Result{}, err
	}

	// Filter Miners: exclude those controlled by others, adopt orphans
	filteredMiners := make([]*appsv1alpha1.Miner, 0, len(allMiners.Items))
	for idx := range allMiners.Items {
		miner := &allMiners.Items[idx]
		if shouldExcludeMiner(ms, miner) {
			continue
		}

		// Adopt orphaned miners
		if metav1.GetControllerOf(miner) == nil {
			if err := r.adoptOrphan(ctx, ms, miner); err != nil {
				log.Error(err, "Failed to adopt Miner", "miner", miner.Name)
				continue
			}
			log.Info("Adopted Miner", "miner", miner.Name)
		}

		filteredMiners = append(filteredMiners, miner)
	}

	// Sync replicas
	result, err := r.syncReplicas(ctx, ms, filteredMiners)
	if err != nil {
		return result, err
	}

	// Update status
	if err := r.updateStatus(ctx, ms, filteredMiners); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("MinerSet reconciled successfully")
	return result, nil
}

// syncReplicas scales Miner resources up or down
func (r *MinerSetReconciler) syncReplicas(ctx context.Context, ms *appsv1alpha1.MinerSet, miners []*appsv1alpha1.Miner) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if ms.Spec.Replicas == nil {
		return ctrl.Result{}, fmt.Errorf("Replicas field in MinerSet spec is nil")
	}

	diff := len(miners) - int(*ms.Spec.Replicas)
	switch {
	case diff < 0:
		// Scale up
		diff *= -1
		log.Info("Scaling up MinerSet", "replicas", *ms.Spec.Replicas, "current", len(miners))
		if err := r.createMiners(ctx, ms, diff); err != nil {
			return ctrl.Result{}, err
		}
		condition.SetTrue(ms, condition.MinersCreatedCondition)
		condition.SetFalse(ms, condition.ResizedCondition, condition.CreatingReason, "Creating miners")
	case diff > 0:
		// Scale down
		log.Info("Scaling down MinerSet", "replicas", *ms.Spec.Replicas, "current", len(miners), "deletePolicy", ms.Spec.DeletePolicy)
		minersToDelete := r.getMinersToDelete(ms, miners, diff)
		if err := r.deleteMiners(ctx, minersToDelete); err != nil {
			return ctrl.Result{}, err
		}
		condition.SetTrue(ms, condition.MinersCreatedCondition)
		condition.SetFalse(ms, condition.ResizedCondition, condition.DeletingReason, "Deleting miners")
	default:
		// Replicas match desired count
		condition.SetTrue(ms, condition.MinersCreatedCondition)
		condition.SetTrue(ms, condition.ResizedCondition)
	}

	return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
}

func (r *MinerSetReconciler) createMiners(ctx context.Context, ms *appsv1alpha1.MinerSet, count int) error {
	for i := 0; i < count; i++ {
		miner := r.computeDesiredMiner(ms, nil)
		if err := r.Create(ctx, miner); err != nil {
			return fmt.Errorf("failed to create miner %q: %w", miner.Name, err)
		}

		if err := r.waitForMinerCreation(ctx, miner); err != nil {
			return fmt.Errorf("failed waiting for miner %q creation: %w", miner.Name, err)
		}
	}
	return nil
}

func (r *MinerSetReconciler) deleteMiners(ctx context.Context, miners []*appsv1alpha1.Miner) error {
	for _, miner := range miners {
		if !miner.DeletionTimestamp.IsZero() {
			continue
		}
		if err := r.Delete(ctx, miner); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete miner %q: %w", miner.Name, err)
		}
	}
	return nil
}

func (r *MinerSetReconciler) computeDesiredMiner(ms *appsv1alpha1.MinerSet, existingMiner *appsv1alpha1.Miner) *appsv1alpha1.Miner {
	minerLabels := make(map[string]string)
	for k, v := range ms.Spec.Template.Labels {
		minerLabels[k] = v
	}
	minerLabels[minerSetNameLabel] = ms.Name
	minerLabels[chainNameLabel] = ms.Spec.Template.Spec.ChainName

	minerAnnotations := make(map[string]string)
	for k, v := range ms.Spec.Template.Annotations {
		minerAnnotations[k] = v
	}

	miner := &appsv1alpha1.Miner{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", ms.Name),
			Namespace:    ms.Namespace,
			Labels:       minerLabels,
			Annotations:  minerAnnotations,
			Finalizers:   []string{minerSetFinalizer},
		},
		Spec: *ms.Spec.Template.Spec.DeepCopy(),
	}

	if existingMiner != nil {
		miner.Name = existingMiner.Name
		miner.UID = existingMiner.UID
		miner.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(ms, msKind)})
	} else {
		miner.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(ms, msKind)}
	}

	return miner
}

func (r *MinerSetReconciler) getMinersToDelete(ms *appsv1alpha1.MinerSet, miners []*appsv1alpha1.Miner, count int) []*appsv1alpha1.Miner {
	if count >= len(miners) {
		return miners
	}

	var toDelete []*appsv1alpha1.Miner
	switch ms.Spec.DeletePolicy {
	case appsv1alpha1.DeletePolicyNewest:
		toDelete = miners[:count]
	case appsv1alpha1.DeletePolicyOldest:
		toDelete = miners[len(miners)-count:]
	default: // Random
		toDelete = miners[:count]
	}

	return toDelete
}

func (r *MinerSetReconciler) updateStatus(ctx context.Context, ms *appsv1alpha1.MinerSet, miners []*appsv1alpha1.Miner) error {
	log := log.FromContext(ctx)

	templateLabel := labels.Set(ms.Spec.Template.Labels).AsSelectorPreValidated()
	fullyLabeledReplicasCount := 0
	readyReplicasCount := 0
	availableReplicasCount := 0

	for _, miner := range miners {
		if templateLabel.Matches(labels.Set(miner.Labels)) {
			fullyLabeledReplicasCount++
		}

		if miner.Status.Phase == appsv1alpha1.MinerPhaseRunning {
			readyReplicasCount++
			if miner.Status.ObservedGeneration == miner.Generation {
				availableReplicasCount++
			}
		}
	}

	ms.Status.Replicas = int32(len(miners))
	ms.Status.FullyLabeledReplicas = int32(fullyLabeledReplicasCount)
	ms.Status.ReadyReplicas = int32(readyReplicasCount)
	ms.Status.AvailableReplicas = int32(availableReplicasCount)

	if ms.Status.ReadyReplicas == ms.Status.Replicas {
		condition.SetTrue(ms, condition.MinersReadyCondition)
	} else {
		condition.SetFalse(ms, condition.MinersReadyCondition, condition.UnavailableReason, "Not all miners are ready")
	}

	if err := r.Status().Update(ctx, ms); err != nil {
		log.Error(err, "Failed to update MinerSet status")
		return err
	}

	return nil
}

func (r *MinerSetReconciler) waitForMinerCreation(ctx context.Context, miner *appsv1alpha1.Miner) error {
	return wait.PollUntilContextTimeout(ctx, stateConfirmationInterval, stateConfirmationTimeout, true, func(ctx context.Context) (bool, error) {
		err := r.Get(ctx, types.NamespacedName{Namespace: miner.Namespace, Name: miner.Name}, &appsv1alpha1.Miner{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
}

func (r *MinerSetReconciler) adoptOrphan(ctx context.Context, ms *appsv1alpha1.MinerSet, miner *appsv1alpha1.Miner) error {
	patch := client.MergeFrom(miner.DeepCopy())
	miner.OwnerReferences = append(miner.OwnerReferences, *metav1.NewControllerRef(ms, msKind))
	return r.Patch(ctx, miner, patch)
}

func shouldExcludeMiner(ms *appsv1alpha1.MinerSet, miner *appsv1alpha1.Miner) bool {
	if metav1.GetControllerOf(miner) != nil && !metav1.IsControlledBy(miner, ms) {
		return true
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinerSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.MinerSet{}).
		Named("minerset").
		Complete(r)
}
