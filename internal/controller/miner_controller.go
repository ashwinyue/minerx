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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/ashwinyue/minerx/api/v1alpha1"
	"github.com/ashwinyue/minerx/pkg/condition"
)

const (
	minerFinalizer    = "miner.onex.io/finalizer"
	defaultPodTimeout = 10 * time.Second
)

// MinerReconciler reconciles a Miner object
type MinerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps.onex.io,resources=miners,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.onex.io,resources=miners/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.onex.io,resources=miners/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *MinerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	miner := &appsv1alpha1.Miner{}
	if err := r.Get(ctx, req.NamespacedName, miner); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Miner resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Miner")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling Miner", "Name", miner.Name, "Namespace", miner.Namespace, "Chain", miner.Spec.ChainName)

	if !miner.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, miner)
	}

	return r.reconcile(ctx, miner)
}

func (r *MinerReconciler) reconcileDelete(ctx context.Context, miner *appsv1alpha1.Miner) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	condition.SetFalse(miner, condition.MinerPodHealthyCondition, condition.DeletingReason, "Deleting pod")

	if err := r.Status().Update(ctx, miner); err != nil {
		log.Error(err, "Failed to update Miner status")
		return ctrl.Result{}, err
	}

	// Delete pod
	podName := miner.Name
	pod := &corev1.Pod{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: miner.Namespace, Name: podName}, pod); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	} else {
		// Determine deletion timeout
		timeout := defaultPodTimeout
		if miner.Spec.PodDeletionTimeout != nil {
			timeout = miner.Spec.PodDeletionTimeout.Duration
		}

		// Try to delete pod with retry
		deleteErr := wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
			if err := r.Delete(ctx, pod); err != nil && !errors.IsNotFound(err) {
				return false, nil
			}
			return true, nil
		})

		if deleteErr != nil {
			log.Error(deleteErr, "Timed out deleting pod")
			condition.SetFalse(miner, condition.MinerPodHealthyCondition, condition.MinerDeletionFailedReason, "Failed to delete pod")
			return ctrl.Result{}, deleteErr
		}
	}

	// Remove finalizer
	if controllerutil.ContainsFinalizer(miner, minerFinalizer) {
		controllerutil.RemoveFinalizer(miner, minerFinalizer)
		if err := r.Update(ctx, miner); err != nil {
			log.Error(err, "Failed to remove finalizer from Miner")
			return ctrl.Result{}, err
		}
	}

	log.Info("Miner deleted successfully")
	return ctrl.Result{}, nil
}

func (r *MinerReconciler) reconcile(ctx context.Context, miner *appsv1alpha1.Miner) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(miner, minerFinalizer) {
		controllerutil.AddFinalizer(miner, minerFinalizer)
		if err := r.Update(ctx, miner); err != nil {
			log.Error(err, "Failed to add finalizer to Miner")
			return ctrl.Result{}, err
		}
	}

	// Create or update pod
	if err := r.reconcilePod(ctx, miner); err != nil {
		return ctrl.Result{}, err
	}

	// Update phase
	if miner.Status.Phase == "" {
		miner.Status.Phase = appsv1alpha1.MinerPhaseProvisioning
	}

	// Sync pod status
	if err := r.syncPodStatus(ctx, miner); err != nil {
		return ctrl.Result{}, err
	}

	// Update status
	miner.Status.LastUpdated = &metav1.Time{Time: time.Now()}
	if err := r.Status().Update(ctx, miner); err != nil {
		log.Error(err, "Failed to update Miner status")
		return ctrl.Result{}, err
	}

	log.Info("Miner reconciled successfully")
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *MinerReconciler) reconcilePod(ctx context.Context, miner *appsv1alpha1.Miner) error {
	log := log.FromContext(ctx)

	pod := &corev1.Pod{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: miner.Namespace, Name: miner.Name}, pod); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		// Pod doesn't exist, create it
		desiredPod := r.createPodSpec(miner)
		if err := r.Create(ctx, desiredPod); err != nil {
			log.Error(err, "Failed to create pod")
			condition.SetFalse(miner, condition.InfrastructureReadyCondition, condition.FailedReason, fmt.Sprintf("Failed to create pod: %v", err))
			return err
		}

		miner.Status.PodRef = &corev1.ObjectReference{
			Kind:       "Pod",
			Namespace:  desiredPod.Namespace,
			Name:       desiredPod.Name,
			UID:        desiredPod.UID,
			APIVersion: "v1",
		}

		log.Info("Created pod", "pod", desiredPod.Name)
		condition.SetTrue(miner, condition.InfrastructureReadyCondition)
	}

	return nil
}

func (r *MinerReconciler) createPodSpec(miner *appsv1alpha1.Miner) *corev1.Pod {
	image := "busybox"
	command := []string{"sh", "-c", "sleep 3600"}

	if miner.Spec.MinerType == "small" {
		image = "nginx:alpine"
	} else if miner.Spec.MinerType == "medium" {
		image = "nginx"
	} else if miner.Spec.MinerType == "large" {
		image = "redis:alpine"
	}

	labels := map[string]string{
		"app":                "miner",
		"miner.onex.io/name": miner.Name,
		"chain.onex.io/name": miner.Spec.ChainName,
	}

	if miner.Labels != nil {
		for k, v := range miner.Labels {
			labels[k] = v
		}
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      miner.Name,
			Namespace: miner.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"miner.onex.io/name": miner.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         appsv1alpha1.GroupVersion.String(),
					Kind:               "Miner",
					Name:               miner.Name,
					UID:                miner.UID,
					Controller:         func(b bool) *bool { return &b }(true),
					BlockOwnerDeletion: func(b bool) *bool { return &b }(true),
				},
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "miner",
					Image:   image,
					Command: command,
				},
			},
			RestartPolicy: miner.Spec.RestartPolicy,
		},
	}

	if pod.Spec.RestartPolicy == "" {
		pod.Spec.RestartPolicy = corev1.RestartPolicyAlways
	}

	return pod
}

func (r *MinerReconciler) syncPodStatus(ctx context.Context, miner *appsv1alpha1.Miner) error {
	log := log.FromContext(ctx)

	pod := &corev1.Pod{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: miner.Namespace, Name: miner.Name}, pod); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Pod not found, setting phase to Pending")
			miner.Status.Phase = appsv1alpha1.MinerPhasePending
			condition.SetFalse(miner, condition.MinerPodHealthyCondition, condition.PodNotFoundReason, "Pod not found")
			return nil
		}
		return err
	}

	// Check pod phase
	switch pod.Status.Phase {
	case corev1.PodRunning:
		if r.isPodReady(pod) {
			miner.Status.Phase = appsv1alpha1.MinerPhaseRunning
			condition.SetTrue(miner, condition.MinerPodHealthyCondition)
			condition.SetTrue(miner, condition.BootstrapReadyCondition)

			// Update addresses
			var addresses []string
			for _, addr := range pod.Status.PodIPs {
				addresses = append(addresses, addr.IP)
			}
			miner.Status.Addresses = addresses
		} else {
			miner.Status.Phase = appsv1alpha1.MinerPhaseProvisioning
			condition.SetFalse(miner, condition.MinerPodHealthyCondition, condition.ProvisioningReason, "Pod is not ready yet")
		}
	case corev1.PodPending:
		miner.Status.Phase = appsv1alpha1.MinerPhaseProvisioning
		condition.SetFalse(miner, condition.MinerPodHealthyCondition, condition.ProvisioningReason, "Pod is pending")
	case corev1.PodFailed:
		miner.Status.Phase = appsv1alpha1.MinerPhaseFailed
		condition.SetFalse(miner, condition.MinerPodHealthyCondition, condition.FailedReason, "Pod failed")
		reason := "Pod failed"
		if pod.Status.Message != "" {
			reason = pod.Status.Message
		}
		miner.Status.FailureReason = &reason
	}

	return nil
}

func (r *MinerReconciler) isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.Miner{}).
		Named("miner").
		Complete(r)
}
