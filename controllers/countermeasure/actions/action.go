package actions

import (
	"context"

	operatorv1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ActionData struct {
	Labels map[string]string
}

type Action interface {
	Perform(context.Context, ActionData) error
}

type ActionHandlerSequence struct {
	actions []Action
	index   int
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
			seq.actions = append(seq.actions, NewDeleteAction(client, *a.Delete))
		} else if a.Restart != nil {
			seq.actions = append(seq.actions, NewRestartAction(client, *a.Restart))
		}
	}

	return seq, nil
}

func (seq *ActionHandlerSequence) OnDetection(ns types.NamespacedName, labels map[string]string) {

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
