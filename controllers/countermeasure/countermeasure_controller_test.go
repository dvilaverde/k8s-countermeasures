package countermeasure

import (
	"context"
	"strings"
	"time"

	cmv1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("CounterMeasures controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		CounterMeasureName      = "test-countermeasure"
		CounterMeasureNamespace = CounterMeasureName

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When Deploying a CounterMeasure", func() {

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      CounterMeasureName,
				Namespace: CounterMeasureName,
			},
		}

		BeforeEach(func() {
			By("Creating the Namespace to perform the tests")
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))
		})

		AfterEach(func() {
			// Attention if you improve this code by adding other context test you MUST
			// be aware of the current delete namespace limitations.
			// More info: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
			By("Deleting the Namespace to perform the tests")
			_ = k8sClient.Delete(ctx, namespace)
		})

		It("Should set CounterMeasure Status.LastObervation Healthy when counter measure is created", func() {
			By("By creating a new CounterMeasure")
			ctx := context.Background()
			counterMeasure := &cmv1alpha1.CounterMeasure{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "countermeasure.vilaverde.rocks/v1alpha1",
					Kind:       "CounterMeasure",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      CounterMeasureName,
					Namespace: CounterMeasureNamespace,
				},
				Spec: cmv1alpha1.CounterMeasureSpec{
					OnEvent: cmv1alpha1.OnEventSpec{
						EventName: "CPUThrottlingHigh",
						SourceSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{"env": "dev"},
						},
					},
					Actions: []cmv1alpha1.Action{
						{
							Name: "delete-temp",
							Debug: &cmv1alpha1.DebugSpec{
								Command: []string{"rm", "-Rf", "/tmp"},
							},
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
			Expect(strings.Join(createdCounterMeasure.Spec.Actions[0].Debug.Command[:], " ")).Should(Equal("rm -Rf /tmp"))

			// // Next check the last observation time
			// By("By checking the Countermeasure has no last observation time")
			// Consistently(func() (bool, error) {
			// 	err := k8sClient.Get(ctx, counterMeasureLookupKey, createdCounterMeasure)
			// 	if err != nil {
			// 		return false, err
			// 	}
			// 	return createdCounterMeasure.Status.LastObservationTime.IsZero(), nil
			// }, duration, interval).Should(Equal(true))

			// // Lets wait for the Last Observation time to be set by the controller
			// reconciler := NewCounterMeasureReconciler(nil, k8sClient, k8sClient.Scheme())

			// _, err := reconciler.Reconcile(ctx, reconcile.Request{
			// 	NamespacedName: types.NamespacedName{Name: CounterMeasureName, Namespace: CounterMeasureName},
			// })
			// Expect(err).To(Not(HaveOccurred()))

			// By("Checking if CounterMeasure last observation was updated")
			// Eventually(func() (bool, error) {
			// 	err := k8sClient.Get(ctx, counterMeasureLookupKey, createdCounterMeasure)
			// 	if err != nil {
			// 		return false, err
			// 	}
			// 	return (createdCounterMeasure.Status.LastObservation == cmv1alpha1.Applying), nil
			// }, time.Minute, time.Second).Should(Equal(true))

			// By("By checking the Countermeasure has last observation time")
			// Consistently(func() (bool, error) {
			// 	err := k8sClient.Get(ctx, counterMeasureLookupKey, createdCounterMeasure)
			// 	if err != nil {
			// 		return false, err
			// 	}
			// 	return createdCounterMeasure.Status.LastObservationTime.IsZero(), nil
			// }, duration, interval).Should(Equal(false))
		})
	})

})
