package actions

import (
	"context"
	"errors"
	"time"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	sourceV1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/eventsource/v1alpha1"

	"github.com/dvilaverde/k8s-countermeasures/pkg/actions/state"
	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/dvilaverde/k8s-countermeasures/pkg/manager"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controller "sigs.k8s.io/controller-runtime/pkg/manager"
)

var managerLog = ctrl.Log.WithName("action_manager")

var _ manager.Manager[*v1alpha1.CounterMeasure] = &Manager{}
var _ events.EventListener = &Manager{}

type Manager struct {
	client     client.Client
	restConfig *rest.Config
	recorder   record.EventRecorder

	state *state.ActionState

	ActionRegistry Registry
}

// NewFromManager construct a new action manager
func NewFromManager(mgr controller.Manager) *Manager {
	actionRegistry := Registry{}
	actionRegistry.Initialize()

	return &Manager{
		client:         mgr.GetClient(),
		restConfig:     mgr.GetConfig(),
		recorder:       mgr.GetEventRecorderFor("action_manager"),
		ActionRegistry: actionRegistry,
		state:          state.NewState(),
	}
}

// OnEvent called by the dispatcher when an event is received.
func (m *Manager) OnEvent(event events.Event) error {
	// first lookup the countermeasure as that is needed to build the actions
	measures := m.state.GetCounterMeasures(event.Name)

	for _, countermeasureEntry := range measures {

		// check if the countermeasure is only accepting events from certain event sources.
		if !countermeasureEntry.Accept(event) {
			continue
		}

		// if this action is already running then prevent it from running again.
		if countermeasureEntry.IsSuppressed(event) {
			m.recorder.Event(countermeasureEntry.Countermeasure, "Normal", "Skipping", "Previous execution is still in progress or suppressed.")
			continue
		}

		actionContext := ActionContext{
			Client:         m.client,
			RestConfig:     m.restConfig,
			Recorder:       m.recorder,
			CounterMeasure: *countermeasureEntry.Countermeasure,
		}

		actionRunner, err := m.ActionRegistry.NewRunner(actionContext)
		if err != nil {
			return err
		}

		m.state.CounterMeasureStart(event, countermeasureEntry.Key)
		go m.waitForCompletion(actionContext, event, actionRunner)
	}

	return nil
}

// Add install a countermeasure to route events to
func (m *Manager) Add(cm *v1alpha1.CounterMeasure) error {

	onEvent := cm.Spec.OnEvent

	var sources []manager.ObjectKey
	if onEvent.SourceSelector != nil {
		managerLog.Info("Lookup up sources using source selector for countermeasure.",
			"name", cm.Name,
			"namespace", cm.Namespace)

		// Resolve all the sources that this countermeasure will accept events from,
		// to allow for routing events to only those countermeasures subscribed to
		// a specific source.
		selector, err := metav1.LabelSelectorAsSelector(onEvent.SourceSelector)
		if err != nil {
			return err
		}

		ctx := context.TODO()
		esList := &sourceV1alpha1.PrometheusList{}
		err = m.client.List(ctx, esList, &client.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			return err
		}

		for _, item := range esList.Items {
			key := manager.ToKey(item.ObjectMeta)
			sources = append(sources, key)
		}
	}

	m.state.Add(cm.DeepCopy(), sources)
	return nil
}

// Measure uninstall a countermeasure from the event subscription
func (m *Manager) Remove(name types.NamespacedName) error {
	return m.state.Remove(name)
}

// Exists check if a countermeasure is installed
func (m *Manager) Exists(objectName metav1.ObjectMeta) bool {
	key := manager.ToKey(objectName)
	return m.state.IsDeployed(key)
}

// waitForCompletion spins up a goroutine to wait for the response from the action runner.
func (m *Manager) waitForCompletion(ctx ActionContext, event events.Event, runner ActionRunner) {
	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)
		runner.Run(ctx, event)
	}()

	select {
	case <-doneCh:
		// remove this from the active set, when the channel is closed
		// this active set is used to prevent the same countermeasure from
		// running concurrently.
		m.state.CounterMeasureEnd(event, manager.ToKey(ctx.CounterMeasure.ObjectMeta))
	case <-time.After(time.Hour):
		managerLog.Error(errors.New("timed out waiting for action to complete"), "action timeout")
	}
}
