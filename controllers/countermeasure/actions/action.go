package actions

import (
	"bytes"
	"context"
	"text/template"

	operatorv1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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
	mgr manager.Manager) (*ActionHandlerSequence, error) {
	seq := &ActionHandlerSequence{
		actions: make([]Action, 0),
		index:   0,
	}

	client := mgr.GetClient()
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
		} else if a.Debug != nil {
			cs, _ := kubernetes.NewForConfig(mgr.GetConfig())
			// TODO handle ignored error
			debug := NewDebugAction(cs.CoreV1(), client, *a.Debug)
			debug.DryRun = countermeasure.Spec.DryRun
			seq.actions = append(seq.actions, debug)
		}
	}

	return seq, nil
}

func ObjectKeyFromTemplate(namespaceTemplate, nameTemplate string, data ActionData) client.ObjectKey {
	return client.ObjectKey{
		Namespace: evaluateTemplate(namespaceTemplate, data),
		Name:      evaluateTemplate(nameTemplate, data),
	}
}

func evaluateTemplate(templateString string, data ActionData) string {
	tmpl := template.Must(template.New("").Parse(templateString))
	var buf bytes.Buffer
	tmpl.Execute(&buf, data)
	return buf.String()
}

func (seq *ActionHandlerSequence) OnDetection(ns types.NamespacedName, labels map[string]string) {

	// create a struct that will be used as data for the templates in the custom resource
	actionData := ActionData{
		Labels: labels,
	}

	ctx := context.Background()
	for _, action := range seq.actions {
		err := action.Perform(ctx, actionData)
		// TODO: introduce some retrying logic here
		if err != nil {
			break
		}
	}
}
