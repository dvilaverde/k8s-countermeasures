package actions

import (
	"context"
	"testing"

	"github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	PodName      = "test-pod"
	PodNamespace = "test-namespace"
)

func TestDelete_Perform(t *testing.T) {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      PodName,
			Namespace: PodNamespace,
			Labels: map[string]string{
				"app": "test-app",
			},
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

	// Objects to track in the fake client.
	objs := []runtime.Object{pod}

	// Create a fake client to mock API calls.
	k8sClient := fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()

	assertPodExists(t, k8sClient, 1)

	spec := v1alpha1.DeleteSpec{
		TargetObjectRef: v1alpha1.ObjectReference{
			Namespace:  "{{ .Data.namespace }}",
			Name:       "{{ .Data.pod }}",
			Kind:       "Pod",
			ApiVersion: "v1",
		},
	}

	deleteAction := NewDeleteAction(k8sClient, spec)

	labels := make(events.EventData)
	labels["pod"] = PodName
	labels["namespace"] = PodNamespace

	deleteAction.Perform(context.TODO(), events.Event{
		Data: &labels,
	})

	assertPodExists(t, k8sClient, 0)
}

func assertPodExists(t *testing.T, k8sClient client.Client, expected int) {
	opt := client.MatchingLabels(map[string]string{"app": "test-app"})
	podList := &corev1.PodList{}
	err := k8sClient.List(context.TODO(), podList, opt)
	if err != nil {
		t.Error(err)
	}

	require.Equal(t, expected, len(podList.Items))
}
