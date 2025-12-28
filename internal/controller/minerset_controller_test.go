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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/ashwinyue/minerx/api/v1alpha1"
	"github.com/ashwinyue/minerx/pkg/condition"
)

var _ = Describe("MinerSet Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-minerset"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		replicas := int32(3)

		BeforeEach(func() {
			By("creating custom resource for Kind MinerSet")
			resource := &appsv1alpha1.MinerSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: appsv1alpha1.MinerSetSpec{
					Replicas: &replicas,
					Template: appsv1alpha1.MinerTemplateSpec{
						Spec: appsv1alpha1.MinerSpec{
							ChainName:     "test-chain",
							MinerType:     appsv1alpha1.MinerTypeSmall,
							RestartPolicy: "Always",
						},
						ObjectMeta: appsv1alpha1.ObjectMeta{
							Labels: map[string]string{
								"app": "miner",
							},
						},
					},
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "miner",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &appsv1alpha1.MinerSet{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if !errors.IsNotFound(err) {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}

			By("Cleaning up miners")
			minerList := &appsv1alpha1.MinerList{}
			Expect(k8sClient.List(ctx, minerList, client.InNamespace("default"))).To(Succeed())
			for _, miner := range minerList.Items {
				Expect(k8sClient.Delete(ctx, &miner)).To(Succeed())
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &MinerSetReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if miners were created")
			minerList := &appsv1alpha1.MinerList{}
			Expect(k8sClient.List(ctx, minerList, client.InNamespace("default"))).To(Succeed())
			Expect(len(minerList.Items)).To(Equal(int(replicas)))

			By("Checking MinerSet status")
			minerset := &appsv1alpha1.MinerSet{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, minerset)).To(Succeed())
			Expect(minerset.Status.Replicas).To(Equal(replicas))
		})

		It("should scale up miners", func() {
			By("Creating initial miners")
			controllerReconciler := &MinerSetReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Scaling up to 5 replicas")
			newReplicas := int32(5)
			minerset := &appsv1alpha1.MinerSet{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, minerset)).To(Succeed())
			minerset.Spec.Replicas = &newReplicas
			Expect(k8sClient.Update(ctx, minerset)).To(Succeed())

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if 5 miners were created")
			minerList := &appsv1alpha1.MinerList{}
			Expect(k8sClient.List(ctx, minerList, client.InNamespace("default"))).To(Succeed())
			Expect(len(minerList.Items)).To(Equal(int(newReplicas)))

			By("Checking conditions")
			Expect(k8sClient.Get(ctx, typeNamespacedName, minerset)).To(Succeed())
			hasMinersCreatedCondition := false
			for _, cond := range minerset.Status.Conditions {
				if cond.Type == string(condition.MinersCreatedCondition) {
					hasMinersCreatedCondition = true
				}
			}
			Expect(hasMinersCreatedCondition).To(BeTrue())
		})

		It("should scale down miners", func() {
			By("Creating initial miners")
			controllerReconciler := &MinerSetReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Scaling down to 1 replica")
			newReplicas := int32(1)
			minerset := &appsv1alpha1.MinerSet{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, minerset)).To(Succeed())
			minerset.Spec.Replicas = &newReplicas
			Expect(k8sClient.Update(ctx, minerset)).To(Succeed())

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if only 1 miner remains")
			minerList := &appsv1alpha1.MinerList{}
			Expect(k8sClient.List(ctx, minerList, client.InNamespace("default"))).To(Succeed())
			Expect(len(minerList.Items)).To(Equal(int(newReplicas)))
		})

		It("should adopt orphan miners", func() {
			By("Creating an orphan miner")
			orphanMiner := &appsv1alpha1.Miner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "orphan-miner",
					Namespace: "default",
					Labels: map[string]string{
						"app": "miner",
					},
				},
				Spec: appsv1alpha1.MinerSpec{
					ChainName: "test-chain",
					MinerType: appsv1alpha1.MinerTypeSmall,
				},
			}
			Expect(k8sClient.Create(ctx, orphanMiner)).To(Succeed())

			By("Reconciling MinerSet")
			controllerReconciler := &MinerSetReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if orphan miner was adopted")
			adoptedMiner := &appsv1alpha1.Miner{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "orphan-miner", Namespace: "default"}, adoptedMiner)).To(Succeed())
			Expect(adoptedMiner.OwnerReferences).NotTo(BeEmpty())
			Expect(adoptedMiner.OwnerReferences[0].Kind).To(Equal("MinerSet"))

			By("Cleaning up orphan miner")
			Expect(k8sClient.Delete(ctx, adoptedMiner)).To(Succeed())
		})
	})
})
