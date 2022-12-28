package actions

import (
	"context"
	"time"

	"github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Delete Action", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		PodName      = "test-pod"
		PodNamespace = "test-namespace"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When Delete Action is performed", func() {

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      PodNamespace,
				Namespace: PodNamespace,
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

		It("Should delete the troubled pod", func() {
			By("By calling delete on the k8s client")
			ctx := context.Background()
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      PodName,
					Namespace: PodNamespace,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "foo",
							Image: "bar:latest",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).Should(Succeed())

			podLookupKey := types.NamespacedName{Name: PodName, Namespace: PodNamespace}
			createdPod := &corev1.Pod{}

			// We'll need to retry getting this newly created Pod, given that creation may not immediately happen.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, podLookupKey, createdPod)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Let's make sure our Command string value was properly converted/handled.
			Expect(createdPod.Spec.Containers[0].Image).Should(Equal("bar:latest"))

			spec := v1alpha1.DeleteSpec{
				TargetObjectRef: v1alpha1.ObjectReference{
					Namespace:  "{{ .Labels.namespace }}",
					Name:       "{{ .Labels.pod }}",
					Kind:       "Pod",
					ApiVersion: "v1",
				},
			}

			deleteAction := NewDeleteAction(k8sClient, spec)

			labels := make(map[string]string)
			labels["pod"] = PodName
			labels["namespace"] = PodNamespace

			deleteAction.Perform(context.TODO(), ActionData{
				Labels: labels,
			})

			// Eventually we expect the pod is deleted
			Eventually(func() bool {
				err := k8sClient.Get(ctx, podLookupKey, createdPod)
				return err == nil
			}, timeout, interval).Should(BeFalse())
		})
	})

})
