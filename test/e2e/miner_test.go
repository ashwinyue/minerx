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
)

var _ = Describe("Miner Controller E2E Tests", func() {
	var (
		ctx       context.Context
		namespace string
		testMiner *appsv1alpha1.Miner
		testChain *appsv1alpha1.Chain
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespace = "default"

		// Create a Chain for testing
		testChain = &appsv1alpha1.Chain{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-chain-miner",
				Namespace: namespace,
			},
			Spec: appsv1alpha1.ChainSpec{
				DisplayName:            "Test Chain for Miner",
				MinerType:              "small",
				Image:                  "nginx:alpine",
				MinMineIntervalSeconds: 43200,
			},
		}
		Expect(k8sClient.Create(ctx, testChain)).To(Succeed())
		time.Sleep(2 * time.Second) // Wait for Chain to be reconciled
	})

	AfterEach(func() {
		// Cleanup test resources
		if testMiner != nil {
			k8sClient.Delete(ctx, testMiner)
		}
		if testChain != nil {
			k8sClient.Delete(ctx, testChain)
		}
		// Wait for deletion to complete
		time.Sleep(3 * time.Second)
	})

	Context("When creating a Miner", func() {
		It("should create a Pod with correct configuration", func() {
			By("Creating a Miner resource")
			testMiner = &appsv1alpha1.Miner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-miner-e2e",
					Namespace: namespace,
				},
				Spec: appsv1alpha1.MinerSpec{
					DisplayName:   "E2E Test Miner",
					MinerType:     "small",
					ChainName:     testChain.Name,
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			}

			Expect(k8sClient.Create(ctx, testMiner)).To(Succeed())

			By("Waiting for Pod to be created")
			Eventually(func() bool {
				var pod corev1.Pod
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testMiner.Name, Namespace: namespace}, &pod)
				return err == nil
			}, 10*time.Second, 1*time.Second).Should(BeTrue())

			By("Verifying Pod configuration")
			var pod corev1.Pod
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testMiner.Name, Namespace: namespace}, &pod)).To(Succeed())

			Expect(pod.Labels["app"]).To(Equal("miner"))
			Expect(pod.Labels["miner.onex.io/name"]).To(Equal(testMiner.Name))
			Expect(pod.Labels["chain.onex.io/name"]).To(Equal(testChain.Name))
			Expect(pod.Annotations["miner.onex.io/name"]).To(Equal(testMiner.Name))

			Expect(pod.OwnerReferences).To(HaveLen(1))
			Expect(pod.OwnerReferences[0].Kind).To(Equal("Miner"))
			Expect(pod.OwnerReferences[0].Name).To(Equal(testMiner.Name))
			Expect(pod.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyAlways))
		})

		It("should select correct image based on MinerType", func() {
			tests := []struct {
				minerType string
				image     string
			}{
				{"small", "nginx:alpine"},
				{"medium", "nginx"},
				{"large", "redis:alpine"},
				{"unknown", "busybox"},
			}

			for _, tt := range tests {
				By("Creating Miner with type: " + tt.minerType)
				miner := &appsv1alpha1.Miner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-miner-" + tt.minerType,
						Namespace: namespace,
					},
					Spec: appsv1alpha1.MinerSpec{
						MinerType:     tt.minerType,
						ChainName:     testChain.Name,
						RestartPolicy: corev1.RestartPolicyAlways,
					},
				}

				Expect(k8sClient.Create(ctx, miner)).To(Succeed())

				By("Verifying Pod image for " + tt.minerType)
				Eventually(func() bool {
					var pod corev1.Pod
					err := k8sClient.Get(ctx, types.NamespacedName{Name: miner.Name, Namespace: namespace}, &pod)
					if err != nil {
						return false
					}
					for _, container := range pod.Spec.Containers {
						if container.Image == tt.image {
							return true
						}
					}
					return false
				}, 10*time.Second, 1*time.Second).Should(BeTrue())

				// Cleanup
				k8sClient.Delete(ctx, miner)
				time.Sleep(2 * time.Second)
			}
		})

		It("should update Miner status correctly", func() {
			By("Creating a Miner resource")
			testMiner = &appsv1alpha1.Miner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-miner-status",
					Namespace: namespace,
				},
				Spec: appsv1alpha1.MinerSpec{
					DisplayName:   "Status Test Miner",
					MinerType:     "medium",
					ChainName:     testChain.Name,
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			}

			Expect(k8sClient.Create(ctx, testMiner)).To(Succeed())

			By("Waiting for Pod to be created and status to be updated")
			Eventually(func() bool {
				var miner appsv1alpha1.Miner
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testMiner.Name, Namespace: namespace}, &miner)
				if err != nil {
					return false
				}
				// Check status fields are set
				return miner.Status.PodRef != nil &&
					miner.Status.Phase == appsv1alpha1.MinerPhaseProvisioning &&
					miner.Status.LastUpdated != nil
			}, 10*time.Second, 1*time.Second).Should(BeTrue())

			By("Verifying PodRef is set")
			var miner appsv1alpha1.Miner
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testMiner.Name, Namespace: namespace}, &miner)).To(Succeed())

			Expect(miner.Status.PodRef).NotTo(BeNil())
			Expect(miner.Status.PodRef.Kind).To(Equal("Pod"))
			Expect(miner.Status.PodRef.Name).To(Equal(testMiner.Name))

			By("Verifying Phase is Provisioning")
			Expect(miner.Status.Phase).To(Equal(appsv1alpha1.MinerPhaseProvisioning))

			By("Verifying LastUpdated is set")
			Expect(miner.Status.LastUpdated).NotTo(BeNil())
		})
	})

	Context("When Pod becomes Ready", func() {
		It("should update Miner Phase to Running", func() {
			By("Creating a Miner resource")
			testMiner = &appsv1alpha1.Miner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-miner-ready",
					Namespace: namespace,
				},
				Spec: appsv1alpha1.MinerSpec{
					DisplayName:   "Ready Test Miner",
					MinerType:     "small",
					ChainName:     testChain.Name,
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			}

			Expect(k8sClient.Create(ctx, testMiner)).To(Succeed())

			By("Waiting for Pod to become Ready")
			Eventually(func() bool {
				var pod corev1.Pod
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testMiner.Name, Namespace: namespace}, &pod)
				if err != nil {
					return false
				}
				// Check if Pod is Ready
				for _, cond := range pod.Status.Conditions {
					if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
						return true
					}
				}
				return false
			}, 30*time.Second, 2*time.Second).Should(BeTrue())

			By("Waiting for Miner Phase to be updated")
			Eventually(func() bool {
				var miner appsv1alpha1.Miner
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testMiner.Name, Namespace: namespace}, &miner)
				if err != nil {
					return false
				}
				return miner.Status.Phase == appsv1alpha1.MinerPhaseRunning
			}, 10*time.Second, 1*time.Second).Should(BeTrue())

			By("Verifying conditions are set correctly")
			var miner appsv1alpha1.Miner
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testMiner.Name, Namespace: namespace}, &miner)).To(Succeed())

			podHealthyConditionFound := false
			infrastructureReadyConditionFound := false
			bootstrapReadyConditionFound := false

			for _, cond := range miner.Status.Conditions {
				if cond.Type == string(condition.MinerPodHealthyCondition) && cond.Status == metav1.ConditionTrue {
					podHealthyConditionFound = true
				}
				if cond.Type == string(condition.InfrastructureReadyCondition) && cond.Status == metav1.ConditionTrue {
					infrastructureReadyConditionFound = true
				}
				if cond.Type == string(condition.BootstrapReadyCondition) && cond.Status == metav1.ConditionTrue {
					bootstrapReadyConditionFound = true
				}
			}

			Expect(podHealthyConditionFound).To(BeTrue())
			Expect(infrastructureReadyConditionFound).To(BeTrue())
			Expect(bootstrapReadyConditionFound).To(BeTrue())

			By("Verifying Addresses are populated")
			Expect(miner.Status.Addresses).NotTo(BeEmpty())
		})
	})

	Context("When Pod fails", func() {
		It("should update Miner Phase to Failed", func() {
			By("Creating a Miner resource that will fail")
			testMiner = &appsv1alpha1.Miner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-miner-failed",
					Namespace: namespace,
				},
				Spec: appsv1alpha1.MinerSpec{
					DisplayName:   "Failed Test Miner",
					MinerType:     "medium",
					ChainName:     testChain.Name,
					RestartPolicy: corev1.RestartPolicyNever,
				},
			}

			Expect(k8sClient.Create(ctx, testMiner)).To(Succeed())

			By("Waiting for Pod to fail")
			Eventually(func() bool {
				var pod corev1.Pod
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testMiner.Name, Namespace: namespace}, &pod)
				if err != nil {
					return false
				}
				// Check if Pod has failed
				return pod.Status.Phase == corev1.PodFailed
			}, 20*time.Second, 1*time.Second).Should(BeTrue())

			By("Waiting for Miner Phase to be updated to Failed")
			Eventually(func() bool {
				var miner appsv1alpha1.Miner
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testMiner.Name, Namespace: namespace}, &miner)
				if err != nil {
					return false
				}
				return miner.Status.Phase == appsv1alpha1.MinerPhaseFailed
			}, 10*time.Second, 1*time.Second).Should(BeTrue())

			By("Verifying failure reason is set")
			var miner appsv1alpha1.Miner
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testMiner.Name, Namespace: namespace}, &miner)).To(Succeed())

			Expect(miner.Status.FailureReason).NotTo(BeNil())
		})
	})

	Context("When deleting a Miner", func() {
		It("should delete the Pod and update status", func() {
			By("Creating a Miner resource")
			testMiner = &appsv1alpha1.Miner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-miner-delete",
					Namespace: namespace,
				},
				Spec: appsv1alpha1.MinerSpec{
					DisplayName:   "Delete Test Miner",
					MinerType:     "small",
					ChainName:     testChain.Name,
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			}

			Expect(k8sClient.Create(ctx, testMiner)).To(Succeed())

			By("Waiting for Pod to be created")
			Eventually(func() bool {
				var pod corev1.Pod
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testMiner.Name, Namespace: namespace}, &pod)
				return err == nil
			}, 10*time.Second, 1*time.Second).Should(BeTrue())

			By("Deleting Miner")
			Expect(k8sClient.Delete(ctx, testMiner)).To(Succeed())

			By("Waiting for Pod to be deleted")
			Eventually(func() bool {
				var pod corev1.Pod
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testMiner.Name, Namespace: namespace}, &pod)
				return err != nil
			}, 20*time.Second, 1*time.Second).Should(BeFalse())

			By("Waiting for Miner Phase to be Deleting")
			Eventually(func() bool {
				var miner appsv1alpha1.Miner
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testMiner.Name, Namespace: namespace}, &miner)
				if err != nil {
					return false
				}
				return miner.Status.Phase == appsv1alpha1.MinerPhaseDeleting
			}, 10*time.Second, 1*time.Second).Should(BeTrue())

			By("Waiting for Miner to be deleted")
			Eventually(func() bool {
				var miner appsv1alpha1.Miner
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testMiner.Name, Namespace: namespace}, &miner)
				return err != nil
			}, 20*time.Second, 1*time.Second).Should(BeFalse())
		})
	})
})
