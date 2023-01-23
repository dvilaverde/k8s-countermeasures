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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("Prometheus EventSource webhook", func() {
	// Define utility constants for object names and testing timeouts/durations and intervals.

	const (
		EventSourceName      = "test-prometheus"
		EventSourceNamespace = "default"
	)

	Context("Deploying a good p8s event source", func() {
		It("should not return any errors", func() {

			svc := &corev1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "prom-operated",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name: "web",
							Port: 8080,
							TargetPort: intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 8080,
							},
						},
					},
				},
			}
			err := k8sClient.Create(ctx, svc)
			Expect(err).NotTo(HaveOccurred())

			By("deploying a good p8s Custom resource")
			ctx := context.Background()
			p8s := &Prometheus{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "eventsource.vilaverde.rocks/v1alpha1",
					Kind:       "Prometheus",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      EventSourceName,
					Namespace: EventSourceNamespace,
				},
				Spec: PrometheusSpec{
					Service: ServiceReference{
						Name:      "prom-operated",
						Namespace: "default",
					},
				},
			}

			err = k8sClient.Create(ctx, p8s)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Deploying a bad prometheus event source", func() {
		It("should fail if there a reference to a service that does not exist", func() {
			p8s := &Prometheus{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "eventsource.vilaverde.rocks/v1alpha1",
					Kind:       "Prometheus",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      EventSourceName,
					Namespace: EventSourceNamespace,
				},
				Spec: PrometheusSpec{
					Service: ServiceReference{
						Name:      "prom-missing",
						Namespace: "default",
					},
				},
			}

			err := k8sClient.Create(ctx, p8s)
			Expect(err).Should(HaveOccurred())
			Ω(err.Error()).Should(Equal("admission webhook \"vprometheus.kb.io\" denied the request: prometheus service 'prom-missing' is not found in namespace 'default'"))
		})

		It("should fail if there a reference to a secret that does not exist", func() {
			p8s := &Prometheus{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "eventsource.vilaverde.rocks/v1alpha1",
					Kind:       "Prometheus",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      EventSourceName,
					Namespace: EventSourceNamespace,
				},
				Spec: PrometheusSpec{
					Service: ServiceReference{
						Name:      "prom-operated",
						Namespace: "default",
					},
					Auth: &AuthSpec{
						SecretReference: corev1.SecretReference{
							Name:      "secret-missing",
							Namespace: "default",
						},
					},
				},
			}

			err := k8sClient.Create(ctx, p8s)
			Expect(err).Should(HaveOccurred())
			Ω(err.Error()).Should(Equal("admission webhook \"vprometheus.kb.io\" denied the request: secret 'secret-missing' is not found in namespace 'default'"))
		})
	})
})
