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

package e2e

import (
	"context"
	"time"

	appsv1alpha1 "github.com/ashwinyue/minerx/api/v1alpha1"
	"github.com/ashwinyue/minerx/pkg/condition"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("Chain Controller E2E Tests", func() {
	var (
		ctx       context.Context
		namespace string
		testChain *appsv1alpha1.Chain
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespace = "default"
	})

	AfterEach(func() {
		// Cleanup test resources
		if testChain != nil {
			k8sClient.Delete(ctx, testChain)
		}
		// Wait for deletion to complete
		time.Sleep(2 * time.Second)
	})

	Context("When creating a Chain", func() {
		It("should create ConfigMap and Miner", func() {
			By("Creating a Chain resource")
			testChain = &appsv1alpha1.Chain{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-chain-e2e",
					Namespace: namespace,
				},
				Spec: appsv1alpha1.ChainSpec{
					DisplayName:            "E2E Test Chain",
					MinerType:              "small",
					Image:                  "nginx:alpine",
					MinMineIntervalSeconds: 43200,
					BootstrapAccount:       strPtr("0x210d9eD12CEA87E33a98AA7Bcb4359eABA9e800e"),
				},
			}

			Expect(k8sClient.Create(ctx, testChain)).To(Succeed())

			By("Waiting for Chain to be reconciled")
			Eventually(func() bool {
				var chain appsv1alpha1.Chain
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testChain.Name, Namespace: namespace}, &chain)
				if err != nil {
					return false
				}
				// Check if ConfigMapsCreated condition is True
				for _, cond := range chain.Status.Conditions {
					if cond.Type == string(condition.ConfigMapsCreatedCondition) &&
						cond.Status == metav1.ConditionTrue {
						return true
					}
				}
				return false
			}, 10*time.Second, 1*time.Second).Should(BeTrue())

			By("Verifying ConfigMap is created")
			var cmList corev1.ConfigMapList
			Eventually(func() bool {
				err := k8sClient.List(ctx, &cmList,
					client.InNamespace(namespace),
					client.MatchingLabels(map[string]string{"chain.onex.io/name": testChain.Name}),
				)
				return err == nil && len(cmList.Items) > 0
			}, 10*time.Second, 1*time.Second).Should(BeTrue())

			cm := cmList.Items[0]
			Expect(cm.Labels["chain.onex.io/name"]).To(Equal(testChain.Name))
			Expect(cm.OwnerReferences).NotTo(BeEmpty())

			By("Verifying Miner is created")
			var miner appsv1alpha1.Miner
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testChain.Name, Namespace: namespace}, &miner)
				return err == nil
			}, 10*time.Second, 1*time.Second).Should(BeTrue())

			Expect(miner.Spec.ChainName).To(Equal(testChain.Name))
			Expect(miner.Spec.MinerType).To(Equal(appsv1alpha1.MinerType(testChain.Spec.MinerType)))
			Expect(miner.Labels["chain.onex.io/name"]).To(Equal(testChain.Name))
			Expect(miner.OwnerReferences).NotTo(BeEmpty())
		})

		It("should set correct status fields", func() {
			By("Creating a Chain resource")
			testChain = &appsv1alpha1.Chain{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-chain-status",
					Namespace: namespace,
				},
				Spec: appsv1alpha1.ChainSpec{
					DisplayName:            "E2E Status Test",
					MinerType:              "medium",
					Image:                  "nginx",
					MinMineIntervalSeconds: 43200,
				},
			}

			Expect(k8sClient.Create(ctx, testChain)).To(Succeed())

			By("Waiting for status to be updated")
			Eventually(func() bool {
				var chain appsv1alpha1.Chain
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testChain.Name, Namespace: namespace}, &chain)
				if err != nil {
					return false
				}
				// Check all required status fields are set
				return chain.Status.ConfigMapRef != nil &&
					chain.Status.MinerRef != nil &&
					chain.Status.ObservedGeneration > 0
			}, 15*time.Second, 1*time.Second).Should(BeTrue())

			var chain appsv1alpha1.Chain
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testChain.Name, Namespace: namespace}, &chain)).To(Succeed())

			By("Verifying ConfigMapRef is set")
			Expect(chain.Status.ConfigMapRef).NotTo(BeNil())
			Expect(chain.Status.ConfigMapRef.Name).To(ContainSubstring(testChain.Name))

			By("Verifying MinerRef is set")
			Expect(chain.Status.MinerRef).NotTo(BeNil())
			Expect(chain.Status.MinerRef.Name).To(Equal(testChain.Name))

			By("Verifying ObservedGeneration")
			Expect(chain.Status.ObservedGeneration).To(Equal(testChain.Generation))
		})
	})

	Context("When updating a Chain", func() {
		It("should not recreate resources on reconcile", func() {
			By("Creating initial Chain")
			testChain = &appsv1alpha1.Chain{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-chain-update",
					Namespace: namespace,
				},
				Spec: appsv1alpha1.ChainSpec{
					DisplayName:            "Update Test",
					MinerType:              "large",
					Image:                  "redis:alpine",
					MinMineIntervalSeconds: 43200,
				},
			}

			Expect(k8sClient.Create(ctx, testChain)).To(Succeed())

			By("Waiting for initial reconcile")
			Eventually(func() bool {
				var chain appsv1alpha1.Chain
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testChain.Name, Namespace: namespace}, &chain)
				if err != nil {
					return false
				}
				// Check both conditions are True
				configMapReady := false
				minerReady := false
				for _, cond := range chain.Status.Conditions {
					if cond.Type == string(condition.ConfigMapsCreatedCondition) && cond.Status == metav1.ConditionTrue {
						configMapReady = true
					}
					if cond.Type == string(condition.MinersCreatedCondition) && cond.Status == metav1.ConditionTrue {
						minerReady = true
					}
				}
				return configMapReady && minerReady
			}, 15*time.Second, 1*time.Second).Should(BeTrue())

			By("Getting initial resource counts")
			var cmList corev1.ConfigMapList
			Expect(k8sClient.List(ctx, &cmList,
				client.InNamespace(namespace),
				client.MatchingLabels(map[string]string{"chain.onex.io/name": testChain.Name}),
			)).To(Succeed())
			initialCMCount := len(cmList.Items)

			var minerList appsv1alpha1.MinerList
			Expect(k8sClient.List(ctx, &minerList,
				client.InNamespace(namespace),
				client.MatchingLabels(map[string]string{"chain.onex.io/name": testChain.Name}),
			)).To(Succeed())
			initialMinerCount := len(minerList.Items)

			By("Updating Chain spec")
			var chain appsv1alpha1.Chain
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testChain.Name, Namespace: namespace}, &chain)).To(Succeed())
			chain.Spec.Image = "nginx:latest"
			chain.Spec.MinerType = "small"
			Expect(k8sClient.Update(ctx, &chain)).To(Succeed())

			By("Waiting for reconcile to complete")
			time.Sleep(5 * time.Second)

			By("Verifying no additional resources were created")
			var updatedCMList corev1.ConfigMapList
			Expect(k8sClient.List(ctx, &updatedCMList,
				client.InNamespace(namespace),
				client.MatchingLabels(map[string]string{"chain.onex.io/name": testChain.Name}),
			)).To(Succeed())

			var updatedMinerList appsv1alpha1.MinerList
			Expect(k8sClient.List(ctx, &updatedMinerList,
				client.InNamespace(namespace),
				client.MatchingLabels(map[string]string{"chain.onex.io/name": testChain.Name}),
			)).To(Succeed())

			Expect(len(updatedCMList.Items)).To(Equal(initialCMCount))
			Expect(len(updatedMinerList.Items)).To(Equal(initialMinerCount))
		})
	})

	Context("When deleting a Chain", func() {
		It("should cascade delete ConfigMap and Miner", func() {
			By("Creating a Chain")
			testChain = &appsv1alpha1.Chain{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-chain-delete",
					Namespace: namespace,
				},
				Spec: appsv1alpha1.ChainSpec{
					DisplayName:            "Delete Test",
					MinerType:              "small",
					Image:                  "nginx:alpine",
					MinMineIntervalSeconds: 43200,
				},
			}

			Expect(k8sClient.Create(ctx, testChain)).To(Succeed())

			By("Waiting for resources to be created")
			Eventually(func() bool {
				var chain appsv1alpha1.Chain
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testChain.Name, Namespace: namespace}, &chain)
				if err != nil {
					return false
				}
				// Check both conditions are True
				configMapReady := false
				minerReady := false
				for _, cond := range chain.Status.Conditions {
					if cond.Type == string(condition.ConfigMapsCreatedCondition) && cond.Status == metav1.ConditionTrue {
						configMapReady = true
					}
					if cond.Type == string(condition.MinersCreatedCondition) && cond.Status == metav1.ConditionTrue {
						minerReady = true
					}
				}
				return configMapReady && minerReady
			}, 15*time.Second, 1*time.Second).Should(BeTrue())

			By("Getting resource references before deletion")
			var cmList corev1.ConfigMapList
			Expect(k8sClient.List(ctx, &cmList,
				client.InNamespace(namespace),
				client.MatchingLabels(map[string]string{"chain.onex.io/name": testChain.Name}),
			)).To(Succeed())
			cmName := cmList.Items[0].Name

			var miner appsv1alpha1.Miner
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testChain.Name, Namespace: namespace}, &miner)).To(Succeed())

			By("Deleting Chain")
			Expect(k8sClient.Delete(ctx, testChain)).To(Succeed())

			By("Verifying ConfigMap is deleted")
			Eventually(func() bool {
				var cm corev1.ConfigMap
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cmName, Namespace: namespace}, &cm)
				return err != nil
			}, 10*time.Second, 1*time.Second).Should(BeFalse())

			By("Verifying Miner is deleted")
			Eventually(func() bool {
				var miner appsv1alpha1.Miner
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testChain.Name, Namespace: namespace}, &miner)
				return err != nil
			}, 10*time.Second, 1*time.Second).Should(BeFalse())

			By("Verifying Chain is deleted")
			Eventually(func() bool {
				var chain appsv1alpha1.Chain
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testChain.Name, Namespace: namespace}, &chain)
				return err != nil
			}, 10*time.Second, 1*time.Second).Should(BeFalse())
		})
	})
})

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}
