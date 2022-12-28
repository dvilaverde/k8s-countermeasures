package actions

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	annotationLabel = "operator.vilaverde.rocks/restarted"
)

type Restart struct {
	client client.Client
}

func NewRestartAction(client client.Client) *Restart {
	return &Restart{
		client: client,
	}
}

// OnDetection essentially change an annotation on the deployment to force a rolling restart.
func (d *Restart) OnDetection(counterMeasureName types.NamespacedName, labels map[string]string) {
	deployment := &appsv1.Deployment{}

	ctx := context.Background()

	deploymentName := types.NamespacedName{
		Namespace: labels["namespace"],
		Name:      labels["pod"],
	}

	err := d.client.Get(ctx, deploymentName, deployment)
	if err == nil {

		patch := client.MergeFrom(deployment.DeepCopy())

		// update the annotations on the template spec metadata so that
		// a change is present in the deployment and causes a rolling restart.
		if deployment.Spec.Template.ObjectMeta.Annotations == nil {
			deployment.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
		}

		restartAtString := time.Now().Format(time.RFC3339)
		deployment.Spec.Template.ObjectMeta.Annotations[annotationLabel] = restartAtString

		// create a generic patch operation this can use with templates:
		// see https://pkg.go.dev/text/template
		// https://stackoverflow.com/questions/61200605/generic-client-get-for-custom-kubernetes-go-operator
		// https://github.com/redhat-cop/operator-utils/blob/master/pkg/util/templates/templates.go
		// https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/client/unstructured_client.go
		// see docs on client.RawPatch()
		err = d.client.Patch(ctx, deployment, patch)
		if err != nil {
			// TODO: do some logging update the state of the CR

		} else {
			// TODO: update the status of the CR
		}
	}

}
