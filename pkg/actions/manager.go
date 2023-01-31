package actions

import (
	"context"
	"sync"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	sourceV1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/eventsource/v1alpha1"

	"github.com/dvilaverde/k8s-countermeasures/pkg/actions/state"
	"github.com/dvilaverde/k8s-countermeasures/pkg/eventbus"
	"github.com/dvilaverde/k8s-countermeasures/pkg/manager"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controller "sigs.k8s.io/controller-runtime/pkg/manager"
)

var managerLog = ctrl.Log.WithName("action_manager")

var _ manager.Manager[*v1alpha1.CounterMeasure] = &Manager{}

type Manager struct {
	client     client.Client
	restConfig *rest.Config
	recorder   record.EventRecorder

	consumersMux sync.RWMutex
	consumers    map[types.NamespacedName]eventbus.Consumer

	state          *state.ActionState
	eventbus       *eventbus.EventBus
	ActionRegistry Registry
}

// NewFromManager construct a new action manager
func NewFromManager(mgr controller.Manager, bus *eventbus.EventBus) *Manager {
	actionRegistry := Registry{}
	actionRegistry.Initialize()

	return &Manager{
		eventbus:       bus,
		client:         mgr.GetClient(),
		restConfig:     mgr.GetConfig(),
		recorder:       mgr.GetEventRecorderFor("action_manager"),
		ActionRegistry: actionRegistry,
		consumersMux:   sync.RWMutex{},
		consumers:      make(map[types.NamespacedName]eventbus.Consumer),
		state:          state.NewState(),
	}
}

// Add install a countermeasure to route events to
func (m *Manager) Add(cm *v1alpha1.CounterMeasure) error {

	onEvent := cm.Spec.OnEvent

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

		ctx := context.Background()
		esList := &sourceV1alpha1.PrometheusList{}
		err = m.client.List(ctx, esList, &client.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			return err
		}

		// for _, item := range esList.Items {
		// 	// todo: merge the provider subscriptions
		// }

	}

	consumer, err := m.eventbus.Subscribe(onEvent.EventName)
	if err != nil {
		return err
	}

	m.consumersMux.Lock()
	defer m.consumersMux.Unlock()

	key := manager.ToKey(cm.ObjectMeta)
	m.consumers[key.NamespacedName] = consumer
	m.state.Add(cm.DeepCopy())

	go func(c eventbus.Consumer, key manager.ObjectKey) {
		for evt := range c.OnEvent() {
			if entry := m.state.GetCounterMeasure(key); entry != nil {
				// if this action is already running then prevent it from running again.
				if entry.IsSuppressed(evt) {
					m.recorder.Event(entry.Countermeasure, "Normal", "Skipping", "Previous execution is still in progress or suppressed.")
					continue
				}

				actionContext := ActionContext{
					Client:         m.client,
					RestConfig:     m.restConfig,
					Recorder:       m.recorder,
					CounterMeasure: *entry.Countermeasure,
				}

				actionRunner, err := m.ActionRegistry.NewRunner(actionContext)
				if err != nil {
					utilruntime.HandleError(err)
					continue
				}

				m.state.CounterMeasureStart(evt, key)
				actionRunner.Run(actionContext, evt)
				m.state.CounterMeasureEnd(evt, key)
			}

		}
	}(consumer, key)

	return nil
}

// Remove uninstall a countermeasure from the event subscription
func (m *Manager) Remove(name types.NamespacedName) error {
	m.consumersMux.Lock()
	defer m.consumersMux.Unlock()

	// make sure to unsubscribe to stop the running goroutine, otherwise we'll
	// have a goroutine leak
	if consumer, ok := m.consumers[name]; ok {
		consumer.UnSubscribe()
	}

	delete(m.consumers, name)

	return m.state.Remove(name)
}

// Exists check if a countermeasure is installed
func (m *Manager) Exists(objectName metav1.ObjectMeta) bool {
	key := manager.ToKey(objectName)

	m.consumersMux.RLock()
	defer m.consumersMux.RUnlock()

	// return m.state.IsDeployed(key)
	_, ok := m.consumers[key.NamespacedName]
	return ok
}
