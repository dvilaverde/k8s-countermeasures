package actions

import (
	"bytes"
	"context"
	"text/template"

	"github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type Patch struct {
	BaseAction
	spec v1alpha1.PatchSpec
}

func NewPatchAction(client client.Client, spec v1alpha1.PatchSpec) *Patch {
	return &Patch{
		BaseAction: BaseAction{
			client: client,
		},
		spec: spec,
	}
}

// Perform will apply the patch to the object
func (p *Patch) Perform(ctx context.Context, actionData ActionData) error {

	gvk, err := p.spec.TargetObjectRef.ToGroupVersionKind()
	if err != nil {
		return err
	}

	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(gvk)

	target := p.spec.TargetObjectRef
	objectName := ObjectKeyFromTemplate(target.Namespace, target.Name, actionData)

	patch, err := p.createPatch(actionData)
	if err != nil {
		return err
	}

	err = p.client.Get(ctx, objectName, object)
	if err == nil {
		opts := make([]client.PatchOption, 0)
		if p.DryRun {
			opts = append(opts, client.DryRunAll)
		}
		err = p.client.Patch(ctx, object, patch, opts...)
	}

	if err != nil {
		return err
	}

	// TODO: update the status of the CR
	return nil
}

func (p *Patch) createPatch(data ActionData) (client.Patch, error) {

	tmpl, err := template.New("").Parse(p.spec.PatchTemplate)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return nil, err
	}

	json, err := yaml.YAMLToJSON(buf.Bytes())
	if err != nil {
		return nil, err
	}

	return client.RawPatch(p.spec.PatchType, json), nil
}
