package actions

import (
	"context"
	"testing"

	"github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8fake "k8s.io/client-go/kubernetes/fake"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDebug_Perform(t *testing.T) {
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
	k8sClient := clientfake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
	fakeCoreV1 := k8fake.NewSimpleClientset(objs...).CoreV1()

	assertPodExists(t, k8sClient, 1)

	spec := v1alpha1.DebugSpec{
		PodRef: v1alpha1.PodReference{
			Namespace: "{{ .Data.namespace }}",
			Name:      "{{ .Data.pod }}",
		},
		Name:    "debugger",
		Image:   "busybox",
		StdIn:   true,
		Command: []string{"touch"},
		Args:    []string{"/tmp/file.txt"},
	}

	debugAction := NewDebugAction(fakeCoreV1, k8sClient, spec)
	labels := make(events.EventData)
	labels["pod"] = PodName
	labels["namespace"] = PodNamespace

	debugAction.Perform(context.TODO(), events.Event{
		Data: &labels,
	})

	pod, err := fakeCoreV1.Pods(PodNamespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
	if err != nil {
		t.Error(err)
	}

	require.Equal(t, 1, len(pod.Spec.EphemeralContainers))
	container := pod.Spec.EphemeralContainers[0]
	assert.Equal(t, "debugger", container.Name)
	assert.Equal(t, "busybox", container.Image)
	assert.Equal(t, true, container.Stdin)
	assert.Equal(t, "touch", container.Command[0])
	assert.Equal(t, "/tmp/file.txt", container.Args[0])
}
