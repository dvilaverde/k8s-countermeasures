/*
Copyright 2022.

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

package v1alpha1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("CounterMeasures webhook", func() {
	// Define utility constants for object names and testing timeouts/durations and intervals.

	const (
		CounterMeasureName      = "test-webhook"
		CounterMeasureNamespace = "default"
	)

	Context("Deploying a good countermeasure", func() {
		It("should not return any errors", func() {
			By("deploying a good countermeaure")
			ctx := context.Background()
			counterMeasure := &CounterMeasure{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "countermeasure.vilaverde.rocks/v1alpha1",
					Kind:       "CounterMeasure",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      CounterMeasureName,
					Namespace: CounterMeasureNamespace,
				},
				Spec: CounterMeasureSpec{
					OnEvent: OnEventSpec{
						EventName: "CPUThrottlingHigh",
					},
					DryRun: true,
					Actions: []Action{
						{
							Name: "delete-temp",
							Debug: &DebugSpec{
								Command: []string{"rm", "-Rf", "/tmp"},
								Image:   "busybox:latest",
							},
						},
					},
				},
			}

			err := k8sClient.Create(ctx, counterMeasure)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Deploying a bad countermeasure", func() {
		It("should fail if not provided an event source and one or more actions", func() {
			counterMeasure := &CounterMeasure{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "countermeasure.vilaverde.rocks/v1alpha1",
					Kind:       "CounterMeasure",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      CounterMeasureName,
					Namespace: CounterMeasureNamespace,
				},
				Spec: CounterMeasureSpec{
					Actions: []Action{},
				},
			}

			err := k8sClient.Create(ctx, counterMeasure)
			Expect(err).Should(HaveOccurred())
			Ω(err.Error()).Should(Equal("admission webhook \"vcountermeasure.kb.io\" denied the request: [event name is required, one or more actions are required]"))
		})

		It("should fail if actions have too many types", func() {
			counterMeasure := &CounterMeasure{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "countermeasure.vilaverde.rocks/v1alpha1",
					Kind:       "CounterMeasure",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      CounterMeasureName,
					Namespace: CounterMeasureNamespace,
				},
				Spec: CounterMeasureSpec{
					OnEvent: OnEventSpec{
						EventName: "CPUThrottlingHigh",
					},
					Actions: []Action{
						{
							Debug: &DebugSpec{
								Image: "busybox:latest",
							},
							Restart: &RestartSpec{
								DeploymentRef: DeploymentReference{
									Name:      "name",
									Namespace: "ns",
								},
							},
						},
					},
				},
			}

			err := k8sClient.Create(ctx, counterMeasure)
			Expect(err).Should(HaveOccurred())
			Ω(err.Error()).Should(Equal("admission webhook \"vcountermeasure.kb.io\" denied the request: each action should only have 1 defined action type"))
		})

		It("should fail if there is no event source", func() {
			counterMeasure := &CounterMeasure{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "countermeasure.vilaverde.rocks/v1alpha1",
					Kind:       "CounterMeasure",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      CounterMeasureName,
					Namespace: CounterMeasureNamespace,
				},
				Spec: CounterMeasureSpec{
					Actions: []Action{
						{
							Debug: &DebugSpec{
								Image: "busybox:latest",
							},
						},
						{
							Restart: &RestartSpec{
								DeploymentRef: DeploymentReference{
									Name:      "name",
									Namespace: "ns",
								},
							},
						},
					},
				},
			}

			err := k8sClient.Create(ctx, counterMeasure)
			Expect(err).Should(HaveOccurred())
			Ω(err.Error()).Should(Equal("admission webhook \"vcountermeasure.kb.io\" denied the request: event name is required"))
		})
	})
})
