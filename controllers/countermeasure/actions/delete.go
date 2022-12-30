package actions

import (
	"context"

	operatorv1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Delete struct {
	BaseAction
	spec operatorv1alpha1.DeleteSpec
}

func NewDeleteAction(client client.Client, spec operatorv1alpha1.DeleteSpec) *Delete {
	return &Delete{
		BaseAction: BaseAction{
			client: client,
		},
		spec: spec,
	}
}

func (d *Delete) Perform(ctx context.Context, actionData ActionData) error {
	target := d.spec.TargetObjectRef
	gvk, err := target.ToGroupVersionKind()
	if err != nil {
		return err
	}

	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(gvk)
	objectName := ObjectKeyFromTemplate(target.Namespace, target.Name, actionData)

	err = d.client.Get(ctx, objectName, object)
	if err == nil {
		opts := make([]client.DeleteOption, 0)
		if d.DryRun {
			opts = append(opts, client.DryRunAll)
		}
		err = d.client.Delete(ctx, object, opts...)
	}

	if err != nil {
		return err
	}

	// TODO: update the status of the CR
	return nil
}
