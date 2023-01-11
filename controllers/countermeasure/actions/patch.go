package actions

import (
	"bytes"
	"context"
	"text/template"

	"github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/events"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type PatchData struct {
	events.EventData
	*unstructured.Unstructured
}

type Patch struct {
	BaseAction
	spec v1alpha1.PatchSpec
}

func NewPatchAction(client client.Client, spec v1alpha1.PatchSpec) *Patch {
	return NewPatchFromBase(BaseAction{
		client: client,
	}, spec)
}

func NewPatchFromBase(base BaseAction, spec v1alpha1.PatchSpec) *Patch {
	return &Patch{
		BaseAction: base,
		spec:       spec,
	}
}

func (d *Patch) GetType() string {
	return "patch"
}

func (p *Patch) GetTargetObjectName(event events.Event) string {
	target := p.spec.TargetObjectRef
	return p.createObjectName(target.Kind, target.Namespace, target.Name, event)
}

// Perform will apply the patch to the object
func (p *Patch) Perform(ctx context.Context, event events.Event) (bool, error) {

	gvk, err := p.spec.TargetObjectRef.ToGroupVersionKind()
	if err != nil {
		return false, err
	}

	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(gvk)

	target := p.spec.TargetObjectRef
	objectName := ObjectKeyFromTemplate(target.Namespace, target.Name, event)

	err = p.client.Get(ctx, objectName, object)
	if err != nil {
		return false, err
	}

	patch, err := p.createPatch(PatchData{
		EventData:    *event.Data,
		Unstructured: object,
	})

	if err != nil {
		return false, err
	}

	opts := make([]client.PatchOption, 0)
	if p.DryRun {
		opts = append(opts, client.DryRunAll)
	}

	err = p.client.Patch(ctx, object, patch, opts...)
	return err == nil, err
}

func (p *Patch) createPatch(data PatchData) (client.Patch, error) {

	tmpl, err := template.New("").Parse(p.spec.YAMLTemplate)
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
