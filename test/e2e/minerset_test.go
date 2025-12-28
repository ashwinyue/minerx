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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("MinerSet Controller E2E Tests", func() {
	var (
		ctx          context.Context
		namespace    string
		testMinerSet *appsv1alpha1.MinerSet
		testChain    *appsv1alpha1.Chain
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespace = "default"

		// Create a Chain for testing
		testChain = &appsv1alpha1.Chain{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-chain-minerset",
				Namespace: namespace,
			},
			Spec: appsv1alpha1.ChainSpec{
				DisplayName:            "Test Chain for MinerSet",
				MinerType:              "medium",
				Image:                  "nginx",
				MinMineIntervalSeconds: 43200,
			},
		}
		Expect(k8sClient.Create(ctx, testChain)).To(Succeed())
		time.Sleep(2 * time.Second) // Wait for Chain to be reconciled
	})

	AfterEach(func() {
		// Cleanup test resources
		if testMinerSet != nil {
			k8sClient.Delete(ctx, testMinerSet)
		}
		if testChain != nil {
			k8sClient.Delete(ctx, testChain)
		}
		// Wait for deletion to complete
		time.Sleep(5 * time.Second)
	})

	Context("When creating a MinerSet", func() {
		It("should create the specified number of Miners", func() {
			replicas := int32(3)

			By("Creating a MinerSet resource")
			testMinerSet = &appsv1alpha1.MinerSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-minerset-create",
					Namespace: namespace,
				},
				Spec: appsv1alpha1.MinerSetSpec{
					DisplayName: "E2E Create Test",
					Replicas:    &replicas,
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "miner"},
					},
					Template: appsv1alpha1.MinerTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "miner"},
						},
						Spec: appsv1alpha1.MinerSpec{
							DisplayName:   "Template Miner",
							MinerType:     "medium",
							ChainName:     testChain.Name,
							RestartPolicy: corev1.RestartPolicyAlways,
						},
					},
					DeletePolicy: appsv1alpha1.DeletePolicyRandom,
				},
			}

			Expect(k8sClient.Create(ctx, testMinerSet)).To(Succeed())

			By("Waiting for Miners to be created")
			Eventually(func() bool {
				var minerList appsv1alpha1.MinerList
				err := k8sClient.List(ctx, &minerList,
					client.InNamespace(namespace),
					client.MatchingLabels(map[string]string{"minerset.onex.io/name": testMinerSet.Name}),
				)
				if err != nil {
					return false
				}
				return len(minerList.Items) == int(replicas)
			}, 30*time.Second, 2*time.Second).Should(BeTrue())

			By("Verifying Miners have correct labels")
			var minerList appsv1alpha1.MinerList
			Expect(k8sClient.List(ctx, &minerList,
				client.InNamespace(namespace),
				client.MatchingLabels(map[string]string{"minerset.onex.io/name": testMinerSet.Name}),
			)).To(Succeed())

			for _, miner := range minerList.Items {
				Expect(miner.Labels["app"]).To(Equal("miner"))
				Expect(miner.Labels["minerset.onex.io/name"]).To(Equal(testMinerSet.Name))
				Expect(miner.Labels["chain.onex.io/name"]).To(Equal(testChain.Name))
			}

			By("Verifying Miners have correct OwnerReferences")
			for _, miner := range minerList.Items {
				Expect(miner.OwnerReferences).To(HaveLen(1))
				Expect(miner.OwnerReferences[0].Kind).To(Equal("MinerSet"))
				Expect(miner.OwnerReferences[0].Name).To(Equal(testMinerSet.Name))
			}
		})

		It("should update MinerSet status correctly", func() {
			replicas := int32(3)

			By("Creating a MinerSet resource")
			testMinerSet = &appsv1alpha1.MinerSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-minerset-status",
					Namespace: namespace,
				},
				Spec: appsv1alpha1.MinerSetSpec{
					DisplayName: "Status Test",
					Replicas:    &replicas,
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "miner"},
					},
					Template: appsv1alpha1.MinerTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "miner"},
						},
						Spec: appsv1alpha1.MinerSpec{
							DisplayName:   "Template Miner",
							MinerType:     "small",
							ChainName:     testChain.Name,
							RestartPolicy: corev1.RestartPolicyAlways,
						},
					},
					DeletePolicy: appsv1alpha1.DeletePolicyRandom,
				},
			}

			Expect(k8sClient.Create(ctx, testMinerSet)).To(Succeed())

			By("Waiting for Miners to be created")
			Eventually(func() bool {
				var minerset appsv1alpha1.MinerSet
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testMinerSet.Name, Namespace: namespace}, &minerset)
				if err != nil {
					return false
				}
				return minerset.Status.Replicas == replicas
			}, 30*time.Second, 2*time.Second).Should(BeTrue())

			By("Verifying status fields")
			var minerset appsv1alpha1.MinerSet
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testMinerSet.Name, Namespace: namespace}, &minerset)).To(Succeed())

			Expect(minerset.Status.Replicas).To(Equal(replicas))
			Expect(minerset.Status.FullyLabeledReplicas).To(Equal(replicas))
			Expect(minerset.Status.ObservedGeneration).To(Equal(testMinerSet.Generation))

			By("Verifying conditions")
			minersCreatedFound := false
			resizedFound := false
			for _, cond := range minerset.Status.Conditions {
				if cond.Type == string(condition.MinersCreatedCondition) && cond.Status == metav1.ConditionTrue {
					minersCreatedFound = true
				}
				if cond.Type == string(condition.ResizedCondition) && cond.Status == metav1.ConditionTrue {
					resizedFound = true
				}
			}
			Expect(minersCreatedFound).To(BeTrue())
			Expect(resizedFound).To(BeTrue())
		})
	})

	Context("When scaling up a MinerSet", func() {
		It("should create additional Miners", func() {
			initialReplicas := int32(2)
			scaledReplicas := int32(5)

			By("Creating a MinerSet with initial replicas")
			testMinerSet = &appsv1alpha1.MinerSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-minerset-scaleup",
					Namespace: namespace,
				},
				Spec: appsv1alpha1.MinerSetSpec{
					DisplayName: "Scale Up Test",
					Replicas:    &initialReplicas,
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "miner"},
					},
					Template: appsv1alpha1.MinerTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "miner"},
						},
						Spec: appsv1alpha1.MinerSpec{
							DisplayName:   "Template Miner",
							MinerType:     "medium",
							ChainName:     testChain.Name,
							RestartPolicy: corev1.RestartPolicyAlways,
						},
					},
					DeletePolicy: appsv1alpha1.DeletePolicyRandom,
				},
			}

			Expect(k8sClient.Create(ctx, testMinerSet)).To(Succeed())

			By("Waiting for initial Miners to be created")
			Eventually(func() bool {
				var minerset appsv1alpha1.MinerSet
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testMinerSet.Name, Namespace: namespace}, &minerset)
				if err != nil {
					return false
				}
				return minerset.Status.Replicas == initialReplicas
			}, 30*time.Second, 2*time.Second).Should(BeTrue())

			By("Scaling up MinerSet")
			var minerset appsv1alpha1.MinerSet
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testMinerSet.Name, Namespace: namespace}, &minerset)).To(Succeed())
			minerset.Spec.Replicas = &scaledReplicas
			Expect(k8sClient.Update(ctx, minerset)).To(Succeed())

			By("Waiting for additional Miners to be created")
			Eventually(func() bool {
				var minerset appsv1alpha1.MinerSet
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testMinerSet.Name, Namespace: namespace}, &minerset)
				if err != nil {
					return false
				}
				return minerset.Status.Replicas == scaledReplicas
			}, 30*time.Second, 2*time.Second).Should(BeTrue())

			By("Verifying Resized condition")
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testMinerSet.Name, Namespace: namespace}, &minerset)).To(Succeed())

			// During scaling up, Resized condition should be False with Creating reason
			resizedWithCreatingReason := false
			for _, cond := range minerset.Status.Conditions {
				if cond.Type == string(condition.ResizedCondition) && cond.Status == metav1.ConditionFalse {
					resizedWithCreatingReason = true
				}
			}

			// Wait for all Miners to be ready
			time.Sleep(10 * time.Second)

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testMinerSet.Name, Namespace: namespace}, &minerset)).To(Succeed())
			resizedWithTrueStatus := false
			for _, cond := range minerset.Status.Conditions {
				if cond.Type == string(condition.ResizedCondition) && cond.Status == metav1.ConditionTrue {
					resizedWithTrueStatus = true
				}
			}
			Expect(resizedWithTrueStatus).To(BeTrue())
		})
	})

	Context("When scaling down a MinerSet", func() {
		It("should delete Miners according to delete policy", func() {
			initialReplicas := int32(5)
			scaledReplicas := int32(2)

			By("Creating a MinerSet with initial replicas")
			testMinerSet = &appsv1alpha1.MinerSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-minerset-scaledown",
					Namespace: namespace,
				},
				Spec: appsv1alpha1.MinerSetSpec{
					DisplayName: "Scale Down Test",
					Replicas:    &initialReplicas,
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "miner"},
					},
					Template: appsv1alpha1.MinerTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "miner"},
						},
						Spec: appsv1alpha1.MinerSpec{
							DisplayName:   "Template Miner",
							MinerType:     "small",
							ChainName:     testChain.Name,
							RestartPolicy: corev1.RestartPolicyAlways,
						},
					},
					DeletePolicy: appsv1alpha1.DeletePolicyRandom,
				},
			}

			Expect(k8sClient.Create(ctx, testMinerSet)).To(Succeed())

			By("Waiting for initial Miners to be created")
			Eventually(func() bool {
				var minerset appsv1alpha1.MinerSet
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testMinerSet.Name, Namespace: namespace}, &minerset)
				if err != nil {
					return false
				}
				return minerset.Status.Replicas == initialReplicas
			}, 30*time.Second, 2*time.Second).Should(BeTrue())

			By("Scaling down MinerSet")
			var minerset appsv1alpha1.MinerSet
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testMinerSet.Name, Namespace: namespace}, &minerset)).To(Succeed())
			minerset.Spec.Replicas = &scaledReplicas
			Expect(k8sClient.Update(ctx, minerset)).To(Succeed())

			By("Waiting for Miners to be deleted")
			Eventually(func() bool {
				var minerset appsv1alpha1.MinerSet
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testMinerSet.Name, Namespace: namespace}, &minerset)
				if err != nil {
					return false
				}
				return minerset.Status.Replicas == scaledReplicas
			}, 30*time.Second, 2*time.Second).Should(BeTrue())

			By("Verifying Resized condition")
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testMinerSet.Name, Namespace: namespace}, &minerset)).To(Succeed())

			// During scaling down, Resized condition should be False with Deleting reason
			resizedWithDeletingReason := false
			for _, cond := range minerset.Status.Conditions {
				if cond.Type == string(condition.ResizedCondition) && cond.Status == metav1.ConditionFalse {
					resizedWithDeletingReason = true
				}
			}
			Expect(resizedWithDeletingReason).To(BeTrue())

			// Wait for all remaining Miners to be ready
			time.Sleep(10 * time.Second)

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testMinerSet.Name, Namespace: namespace}, &minerset)).To(Succeed())
			resizedWithTrueStatus := false
			for _, cond := range minerset.Status.Conditions {
				if cond.Type == string(condition.ResizedCondition) && cond.Status == metav1.ConditionTrue {
					resizedWithTrueStatus = true
				}
			}
			Expect(resizedWithTrueStatus).To(BeTrue())
		})
	})

	Context("When adopting orphan Miners", func() {
		It("should adopt Miners matching the selector", func() {
			replicas := int32(3)

			By("Creating a MinerSet resource")
			testMinerSet = &appsv1alpha1.MinerSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-minerset-adopt",
					Namespace: namespace,
				},
				Spec: appsv1alpha1.MinerSetSpec{
					DisplayName: "Adopt Test",
					Replicas:    &replicas,
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "miner", "group": "test"},
					},
					Template: appsv1alpha1.MinerTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "miner", "group": "test"},
						},
						Spec: appsv1alpha1.MinerSpec{
							DisplayName:   "Template Miner",
							MinerType:     "medium",
							ChainName:     testChain.Name,
							RestartPolicy: corev1.RestartPolicyAlways,
						},
					},
					DeletePolicy: appsv1alpha1.DeletePolicyRandom,
				},
			}

			Expect(k8sClient.Create(ctx, testMinerSet)).To(Succeed())

			By("Creating an orphan Miner")
			orphanMiner := &appsv1alpha1.Miner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "orphan-miner",
					Namespace: namespace,
					Labels:    map[string]string{"app": "miner", "group": "test"},
				},
				Spec: appsv1alpha1.MinerSpec{
					DisplayName:   "Orphan Miner",
					MinerType:     "small",
					ChainName:     testChain.Name,
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			}

			Expect(k8sClient.Create(ctx, orphanMiner)).To(Succeed())

			By("Waiting for MinerSet to adopt the orphan")
			Eventually(func() bool {
				var miner appsv1alpha1.Miner
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "orphan-miner", Namespace: namespace}, &miner)
				if err != nil {
					return false
				}
				// Check if Miner has an OwnerReference
				for _, ownerRef := range miner.OwnerReferences {
					if ownerRef.Kind == "MinerSet" && ownerRef.Name == testMinerSet.Name {
						return true
					}
				}
				return false
			}, 15*time.Second, 1*time.Second).Should(BeTrue())

			By("Verifying MinerSet status includes adopted Miner")
			Eventually(func() bool {
				var minerset appsv1alpha1.MinerSet
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testMinerSet.Name, Namespace: namespace}, &minerset)
				if err != nil {
					return false
				}
				return minerset.Status.Replicas == replicas+1
			}, 15*time.Second, 1*time.Second).Should(BeTrue())
		})
	})

	Context("When deleting a MinerSet", func() {
		It("should delete all managed Miners", func() {
			replicas := int32(3)

			By("Creating a MinerSet resource")
			testMinerSet = &appsv1alpha1.MinerSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-minerset-delete",
					Namespace: namespace,
				},
				Spec: appsv1alpha1.MinerSetSpec{
					DisplayName: "Delete Test",
					Replicas:    &replicas,
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "miner"},
					},
					Template: appsv1alpha1.MinerTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "miner"},
						},
						Spec: appsv1alpha1.MinerSpec{
							DisplayName:   "Template Miner",
							MinerType:     "medium",
							ChainName:     testChain.Name,
							RestartPolicy: corev1.RestartPolicyAlways,
						},
					},
					DeletePolicy: appsv1alpha1.DeletePolicyRandom,
				},
			}

			Expect(k8sClient.Create(ctx, testMinerSet)).To(Succeed())

			By("Waiting for Miners to be created")
			Eventually(func() bool {
				var minerList appsv1alpha1.MinerList
				err := k8sClient.List(ctx, &minerList,
					client.InNamespace(namespace),
					client.MatchingLabels(map[string]string{"minerset.onex.io/name": testMinerSet.Name}),
				)
				if err != nil {
					return false
				}
				return len(minerList.Items) == int(replicas)
			}, 30*time.Second, 2*time.Second).Should(BeTrue())

			By("Deleting MinerSet")
			Expect(k8sClient.Delete(ctx, testMinerSet)).To(Succeed())

			By("Waiting for Miners to be deleted")
			Eventually(func() bool {
				var minerList appsv1alpha1.MinerList
				err := k8sClient.List(ctx, &minerList,
					client.InNamespace(namespace),
					client.MatchingLabels(map[string]string{"minerset.onex.io/name": testMinerSet.Name}),
				)
				if err != nil {
					return false
				}
				return len(minerList.Items) == 0
			}, 30*time.Second, 2*time.Second).Should(BeTrue())

			By("Waiting for MinerSet to be deleted")
			Eventually(func() bool {
				var minerset appsv1alpha1.MinerSet
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testMinerSet.Name, Namespace: namespace}, &minerset)
				return err != nil
			}, 10*time.Second, 1*time.Second).Should(BeTrue())
		})
	})
})
