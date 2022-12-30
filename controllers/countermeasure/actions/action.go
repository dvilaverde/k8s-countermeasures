package actions

import (
	"bytes"
	"context"
	"text/template"

	operatorv1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Action interface {
	Perform(context.Context, ActionData) error
}

type ActionData struct {
	Labels map[string]string
}

type ActionHandlerSequence struct {
	actions []Action
	index   int
}

type BaseAction struct {
	DryRun bool

	client client.Client
}

func CounterMeasureToActions(countermeasure *operatorv1alpha1.CounterMeasure,
	client client.Client) (*ActionHandlerSequence, error) {
	seq := &ActionHandlerSequence{
		actions: make([]Action, 0),
		index:   0,
	}

	// TODO: refactor this into some form of action registry
	for _, a := range countermeasure.Spec.Actions {
		if a.Delete != nil {
			delete := NewDeleteAction(client, *a.Delete)
			delete.DryRun = countermeasure.Spec.DryRun
			seq.actions = append(seq.actions, delete)
		} else if a.Restart != nil {
			restart := NewRestartAction(client, *a.Restart)
			restart.DryRun = countermeasure.Spec.DryRun
			seq.actions = append(seq.actions, restart)
		} else if a.Patch != nil {
			patch := NewPatchAction(client, *a.Patch)
			patch.DryRun = countermeasure.Spec.DryRun
			seq.actions = append(seq.actions, patch)
		}
	}

	return seq, nil
}

func ObjectKeyFromTemplate(namespaceTemplate, nameTemplate string, data ActionData) client.ObjectKey {
	objectName := client.ObjectKey{}

	tmpl := template.Must(template.New("").Parse(namespaceTemplate))
	var buf bytes.Buffer
	tmpl.Execute(&buf, data)
	objectName.Namespace = buf.String()

	tmpl = template.Must(template.New("").Parse(nameTemplate))
	buf.Reset()
	tmpl.Execute(&buf, data)
	objectName.Name = buf.String()

	return objectName
}

func (seq *ActionHandlerSequence) OnDetection(ns types.NamespacedName, labels map[string]string) {

	// create a struct that will be used as data for the templates in the custom resource
	actionData := ActionData{
		Labels: labels,
	}

	ctx := context.Background()
	for _, action := range seq.actions {
		err := action.Perform(ctx, actionData)
		if err != nil {
			break
		}
	}
}
