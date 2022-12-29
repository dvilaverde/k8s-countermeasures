package actions

import (
	"bytes"
	"context"
	"text/template"

	operatorv1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Delete struct {
	client client.Client
	spec   operatorv1alpha1.DeleteSpec
}

func NewDeleteAction(client client.Client, spec operatorv1alpha1.DeleteSpec) *Delete {
	return &Delete{
		client: client,
		spec:   spec,
	}
}

func (d *Delete) Perform(ctx context.Context, actionData ActionData) error {
	object := &unstructured.Unstructured{}

	target := d.spec.TargetObjectRef
	gvk, err := target.ToGroupVersionKind()
	if err != nil {
		return err
	}

	object.SetGroupVersionKind(gvk)

	nsTemplate := template.Must(template.New("namespace").Parse(d.spec.TargetObjectRef.Namespace))
	nameTemplate := template.Must(template.New("name").Parse(d.spec.TargetObjectRef.Name))

	var nsBuf bytes.Buffer
	var nameBuf bytes.Buffer

	nsTemplate.Execute(&nsBuf, actionData)
	nameTemplate.Execute(&nameBuf, actionData)

	objectName := client.ObjectKey{
		Namespace: nsBuf.String(),
		Name:      nameBuf.String(),
	}

	err = d.client.Get(ctx, objectName, object)
	if err != nil {
		return err
	}

	d.client.Delete(ctx, object, client.GracePeriodSeconds(15))

	// TODO: update the status of the CR
	return nil
}
