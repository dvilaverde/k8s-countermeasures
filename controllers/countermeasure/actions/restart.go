package actions

import (
	"bytes"
	"context"
	"text/template"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/assets"
)

type Restart struct {
	client client.Client
	spec   v1alpha1.RestartSpec
}

func NewRestartAction(client client.Client, spec v1alpha1.RestartSpec) *Restart {
	return &Restart{
		client: client,
		spec:   spec,
	}
}

// Perform will apply the restart patch to the deployment
func (r *Restart) Perform(ctx context.Context, actionData ActionData) error {
	object := &unstructured.Unstructured{}

	gvk := schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}
	object.SetGroupVersionKind(gvk)

	nsTemplate := template.Must(template.New("namespace").Parse(r.spec.DeploymentRef.Namespace))
	nameTemplate := template.Must(template.New("name").Parse(r.spec.DeploymentRef.Name))

	var nsBuf bytes.Buffer
	var nameBuf bytes.Buffer

	nsTemplate.Execute(&nsBuf, actionData)
	nameTemplate.Execute(&nameBuf, actionData)

	objectName := client.ObjectKey{
		Namespace: nsBuf.String(),
		Name:      nameBuf.String(),
	}

	err := r.client.Get(ctx, objectName, object)
	if err != nil {
		return err
	}

	// do the patch to the labels to force a restart
	patch := assets.GetPatch("restart-patch.yaml")

	err = r.client.Patch(ctx, object, patch)
	if err != nil {
		return err
	}

	// TODO: update the status of the CR
	return nil
}
