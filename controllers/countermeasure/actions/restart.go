package actions

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/assets"
)

type Restart struct {
	BaseAction
	spec v1alpha1.RestartSpec
}

func NewRestartAction(client client.Client, spec v1alpha1.RestartSpec) *Restart {
	return NewRestartFromBase(BaseAction{
		client: client,
	}, spec)
}

func NewRestartFromBase(base BaseAction, spec v1alpha1.RestartSpec) *Restart {
	return &Restart{
		BaseAction: base,
		spec:       spec,
	}
}

func (r *Restart) GetTargetObjectName() string {
	return r.createObjectName("deployment", r.spec.DeploymentRef.Namespace, r.spec.DeploymentRef.Name)
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

	target := r.spec.DeploymentRef
	objectName := ObjectKeyFromTemplate(target.Namespace, target.Name, actionData)

	// do the patch to the labels to force a restart
	patch := assets.GetPatch("restart-patch.yaml")

	err := r.client.Get(ctx, objectName, object)
	if err == nil {
		opts := make([]client.PatchOption, 0)
		if r.DryRun {
			opts = append(opts, client.DryRunAll)
		}

		err = r.client.Patch(ctx, object, patch, opts...)
	}

	return err
}
