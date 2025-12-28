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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/ashwinyue/minerx/api/v1alpha1"
	"github.com/ashwinyue/minerx/pkg/condition"
)

const (
	chainFinalizer = "chain.onex.io/finalizer"
)

// ChainReconciler reconciles a Chain object
type ChainReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps.onex.io,resources=chains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.onex.io,resources=chains/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.onex.io,resources=chains/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps.onex.io,resources=miners,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// SetupWithManager sets up the controller with the Manager.
func (r *ChainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.Chain{}).
		Complete(r)
}

func (r *ChainReconciler) lowestNonZeroResult(a, b ctrl.Result) ctrl.Result {
	if a.RequeueAfter < b.RequeueAfter {
		return a
	}
	if a.Requeue {
		return a
	}
	return b
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ChainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	chain := &appsv1alpha1.Chain{}
	if err := r.Get(ctx, req.NamespacedName, chain); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Chain resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Chain")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling Chain", "Name", chain.Name, "Namespace", chain.Namespace)

	if !chain.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, chain)
	}

	return r.reconcile(ctx, chain)
}

func (r *ChainReconciler) reconcileDelete(ctx context.Context, chain *appsv1alpha1.Chain) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(chain, chainFinalizer) {
		controllerutil.RemoveFinalizer(chain, chainFinalizer)
		if err := r.Update(ctx, chain); err != nil {
			log.Error(err, "Failed to remove finalizer from Chain")
			return ctrl.Result{}, err
		}
	}

	log.Info("Chain deleted successfully")
	return ctrl.Result{}, nil
}

func (r *ChainReconciler) reconcile(ctx context.Context, chain *appsv1alpha1.Chain) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(chain, chainFinalizer) {
		controllerutil.AddFinalizer(chain, chainFinalizer)
		if err := r.Update(ctx, chain); err != nil {
			log.Error(err, "Failed to add finalizer to Chain")
			return ctrl.Result{}, err
		}
	}

	phases := []func(context.Context, *appsv1alpha1.Chain) (ctrl.Result, error){
		r.reconcileConfigMap,
		r.reconcileMiner,
	}

	result := ctrl.Result{}
	for _, phase := range phases {
		phaseResult, err := phase(ctx, chain)
		if err != nil {
			log.Error(err, "Failed to execute reconciliation phase")
			return ctrl.Result{}, err
		}
		result = r.lowestNonZeroResult(result, phaseResult)
	}

	// Update status
	chain.Status.ObservedGeneration = chain.Generation
	if err := r.Status().Update(ctx, chain); err != nil {
		log.Error(err, "Failed to update Chain status")
		return ctrl.Result{}, err
	}

	log.Info("Chain reconciled successfully")
	return result, nil
}

func (r *ChainReconciler) reconcileConfigMap(ctx context.Context, chain *appsv1alpha1.Chain) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	reconciled, err := r.IsConfigMapReconciled(ctx, chain)
	if err != nil {
		log.Error(err, "Failed to check if ConfigMap is reconciled")
		return ctrl.Result{}, err
	}
	if reconciled {
		return ctrl.Result{}, nil
	}

	cm, err := r.createConfigMap(ctx, chain)
	if err != nil {
		log.Error(err, "Failed to create ConfigMap")
		condition.SetFalse(chain, condition.ConfigMapsCreatedCondition, condition.FailedReason, fmt.Sprintf("Failed to create ConfigMap: %v", err))
		return ctrl.Result{}, err
	}

	chain.Status.ConfigMapRef = &appsv1alpha1.LocalObjectReference{Name: cm.Name}

	log.Info("Created ConfigMap", "configMap", cm.Name)
	condition.SetTrue(chain, condition.ConfigMapsCreatedCondition)

	return ctrl.Result{}, nil
}

func (r *ChainReconciler) IsConfigMapReconciled(ctx context.Context, chain *appsv1alpha1.Chain) (bool, error) {
	log := log.FromContext(ctx)

	cmList := &corev1.ConfigMapList{}
	selectorMap := map[string]string{"chain.onex.io/name": chain.Name}
	if err := r.List(ctx, cmList, client.InNamespace(chain.Namespace), client.MatchingLabels(selectorMap)); err != nil {
		log.Error(err, "Failed to list ConfigMaps")
		return false, err
	}

	return len(cmList.Items) != 0, nil
}

func (r *ChainReconciler) reconcileMiner(ctx context.Context, chain *appsv1alpha1.Chain) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	reconciled, err := r.IsMinerReconciled(ctx, chain)
	if err != nil {
		log.Error(err, "Failed to check if Miner is reconciled")
		return ctrl.Result{}, err
	}
	if reconciled {
		return ctrl.Result{}, nil
	}

	miner, err := r.createMinerForChain(ctx, chain)
	if err != nil {
		log.Error(err, "Failed to create Miner")
		condition.SetFalse(chain, condition.MinersCreatedCondition, condition.FailedReason, fmt.Sprintf("Failed to create Miner: %v", err))
		return ctrl.Result{}, err
	}

	if chain.Status.MinerRef == nil {
		chain.Status.MinerRef = &appsv1alpha1.LocalObjectReference{Name: miner.Name}
	}

	log.Info("Created Miner", "miner", miner.Name)
	condition.SetTrue(chain, condition.MinersCreatedCondition)

	return ctrl.Result{}, nil
}

func (r *ChainReconciler) IsMinerReconciled(ctx context.Context, chain *appsv1alpha1.Chain) (bool, error) {
	log := log.FromContext(ctx)

	mList := &appsv1alpha1.MinerList{}
	selectorMap := map[string]string{"chain.onex.io/name": chain.Name}
	if err := r.List(ctx, mList, client.InNamespace(chain.Namespace), client.MatchingLabels(selectorMap)); err != nil {
		log.Error(err, "Failed to list Miners")
		return false, err
	}

	return len(mList.Items) != 0, nil
}

func (r *ChainReconciler) createConfigMap(ctx context.Context, chain *appsv1alpha1.Chain) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", chain.Name),
			Namespace:    chain.Namespace,
			Labels:       map[string]string{"chain.onex.io/name": chain.Name},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(chain, chainKind),
			},
		},
		Data: map[string]string{
			"chainName": chain.Name,
			"image":     chain.Spec.Image,
		},
	}

	return cm, nil
}

func (r *ChainReconciler) createMinerForChain(ctx context.Context, chain *appsv1alpha1.Chain) (*appsv1alpha1.Miner, error) {
	miner := &appsv1alpha1.Miner{
		ObjectMeta: metav1.ObjectMeta{
			Name:      chain.Name,
			Namespace: chain.Namespace,
			Labels:    map[string]string{"chain.onex.io/name": chain.Name},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(chain, chainKind),
			},
		},
		Spec: appsv1alpha1.MinerSpec{
			ChainName:     chain.Name,
			MinerType:     appsv1alpha1.MinerType(chain.Spec.MinerType),
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}

	return miner, nil
}

var chainKind = appsv1alpha1.GroupVersion.WithKind("Chain")
