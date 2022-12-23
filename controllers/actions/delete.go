package actions

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Delete struct {
	client client.Client
}

func NewDeleteAction(client client.Client) *Delete {
	return &Delete{
		client: client,
	}
}

func (d *Delete) OnDetection(counterMeasureName types.NamespacedName, labels map[string]string) {
	pod := &corev1.Pod{}

	ctx := context.Background()

	podName := types.NamespacedName{
		Namespace: labels["namespace"],
		Name:      labels["pod"],
	}

	err := d.client.Get(ctx, podName, pod)
	if err == nil {
		d.client.Delete(ctx, pod, client.GracePeriodSeconds(15))
	}

}
