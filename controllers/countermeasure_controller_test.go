package controllers

import (
	"context"
	"strings"
	"time"

	cmv1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("CounterMeasures controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		CounterMeasureName      = "test-countermeasure"
		CounterMeasureNamespace = "default"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When Deploying a CounterMeasure", func() {
		It("Should set CounterMeasure Status.LastObervation Healthy when counter measure is created", func() {
			By("By creating a new CounterMeasure")
			ctx := context.Background()
			counterMeasure := &cmv1alpha1.CounterMeasure{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "operator.vilaverde.rocks/v1alpha1",
					Kind:       "CounterMeasure",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      CounterMeasureName,
					Namespace: CounterMeasureNamespace,
				},
				Spec: cmv1alpha1.CounterMeasureSpec{
					Prometheus: cmv1alpha1.PrometheusSpec{
						Service: &cmv1alpha1.ServiceReference{
							Name:      "prom-operated",
							Namespace: "monitoring",
						},
						Interval: metav1.Duration{
							Duration: duration,
						},
						Alert: &cmv1alpha1.PrometheusAlertSpec{
							AlertName:      "CPUThrottlingHigh",
							IncludePending: true,
						},
					},
					Actions: []cmv1alpha1.Action{
						{
							Name:    "delete-temp",
							Command: []string{"rm", "-Rf", "/tmp"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, counterMeasure)).Should(Succeed())

			counterMeasureLookupKey := types.NamespacedName{Name: CounterMeasureName, Namespace: CounterMeasureNamespace}
			createdCounterMeasure := &cmv1alpha1.CounterMeasure{}

			// We'll need to retry getting this newly created CounterMeasure, given that creation may not immediately happen.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, counterMeasureLookupKey, createdCounterMeasure)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Let's make sure our Command string value was properly converted/handled.
			Expect(strings.Join(createdCounterMeasure.Spec.Actions[0].Command[:], " ")).Should(Equal("rm -Rf /tmp"))

			// Next check the last observation time
			By("By checking the Countermeasure has no last observation time")
			Consistently(func() (bool, error) {
				err := k8sClient.Get(ctx, counterMeasureLookupKey, createdCounterMeasure)
				if err != nil {
					return false, err
				}
				return createdCounterMeasure.Status.LastObservationTime.IsZero(), nil
			}, duration, interval).Should(Equal(true))

			// Lets wait for the Last Observation time to be set by the controller

		})
	})

})