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

// SetupWithManager sets up the controller with the Manager.
func (r *ChainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.Chain{}).
		Complete(r)
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

	condition.SetTrue(chain, condition.InfrastructureReadyCondition)

	if err := r.Status().Update(ctx, chain); err != nil {
		log.Error(err, "Failed to update Chain status")
		return ctrl.Result{}, err
	}

	log.Info("Chain reconciled successfully")
	return ctrl.Result{}, nil
}

func createConfigMap(ctx context.Context, chain *appsv1alpha1.Chain) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      chain.Name,
			Namespace: chain.Namespace,
		},
		Data: map[string]string{
			"chainName": chain.Name,
			"image":     chain.Spec.Image,
		},
	}

	return cm, nil
}

func createMinerForChain(ctx context.Context, chain *appsv1alpha1.Chain) (*appsv1alpha1.Miner, error) {
	miner := &appsv1alpha1.Miner{
		ObjectMeta: metav1.ObjectMeta{
			Name:      chain.Name,
			Namespace: chain.Namespace,
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
