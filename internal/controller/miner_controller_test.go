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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/ashwinyue/minerx/api/v1alpha1"
	"github.com/ashwinyue/minerx/pkg/condition"
)

var _ = Describe("Miner Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-miner"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating custom resource for Kind Miner")
			resource := &appsv1alpha1.Miner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: appsv1alpha1.MinerSpec{
					ChainName:     "test-chain",
					MinerType:     appsv1alpha1.MinerTypeSmall,
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &appsv1alpha1.Miner{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if !errors.IsNotFound(err) {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &MinerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if Pod was created")
			pod := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, pod)).To(Succeed())

			By("Checking Miner status")
			miner := &appsv1alpha1.Miner{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, miner)).To(Succeed())
			Expect(miner.Status.Phase).To(Equal(appsv1alpha1.MinerPhaseProvisioning))
			Expect(miner.Status.PodRef).NotTo(BeNil())
		})

		It("should update status when pod is ready", func() {
			By("Creating a ready pod")
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
					Annotations: map[string]string{
						"miner.onex.io/name": resourceName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "miner", Image: "nginx:alpine"},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())

			By("Reconciling the resource")
			controllerReconciler := &MinerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking Miner status is Running")
			miner := &appsv1alpha1.Miner{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, miner)).To(Succeed())
			Expect(miner.Status.Phase).To(Equal(appsv1alpha1.MinerPhaseRunning))

			By("Checking conditions")
			hasReadyCondition := false
			for _, cond := range miner.Status.Conditions {
				if cond.Type == string(condition.MinerPodHealthyCondition) {
					hasReadyCondition = true
					Expect(cond.Status).To(Equal(metav1.ConditionTrue))
				}
			}
			Expect(hasReadyCondition).To(BeTrue())
		})

		It("should handle deletion correctly", func() {
			By("Creating a pod")
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "miner", Image: "nginx:alpine"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())

			By("Deleting the miner")
			miner := &appsv1alpha1.Miner{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, miner)).To(Succeed())
			Expect(k8sClient.Delete(ctx, miner)).To(Succeed())

			By("Reconciling deletion")
			controllerReconciler := &MinerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if pod was deleted")
			Eventually(func() bool {
				podErr := k8sClient.Get(ctx, typeNamespacedName, &corev1.Pod{})
				return errors.IsNotFound(podErr)
			}, "10s").Should(BeTrue())
		})
	})
})
