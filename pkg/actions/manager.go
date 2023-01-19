package actions

import (
	"sync"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"

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

type ActiveCounterMeasures map[manager.ObjectKey]v1alpha1.CounterMeasure
type RunningSet map[manager.ObjectKey]struct{}

type Manager struct {
	client     client.Client
	restConfig *rest.Config
	recorder   record.EventRecorder

	measuresMux sync.Mutex
	deployed    ActiveCounterMeasures
	eventIndex  map[string][]manager.ObjectKey

	activeMux sync.RWMutex
	active    RunningSet

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
		activeMux:      sync.RWMutex{},
		measuresMux:    sync.Mutex{},
	}
}

// OnEvent called by the dispatcher when an event is received.
func (m *Manager) OnEvent(event events.Event) error {
	m.measuresMux.Lock()
	defer m.measuresMux.Unlock()

	// first lookup the countermeasure as that is needed to build the actions
	objecKeys, ok := m.eventIndex[event.Name]
	if !ok {
		return nil
	}

	for _, objectKey := range objecKeys {

		// TODO: needs a filter here to prevent sending events from sources the CM is not subscribed to.

		countermeasure := m.deployed[objectKey]
		m.activeMux.RLock()
		// if this action is already running then prevent it from running again.
		if _, ok := m.active[objectKey]; ok {
			m.recorder.Event(&countermeasure, "Normal", "Skipping", "Previous execution is still in progress.")
			continue
		}
		m.activeMux.RUnlock()

		actionContext := ActionContext{
			Client:         m.client,
			RestConfig:     m.restConfig,
			Recorder:       m.recorder,
			CounterMeasure: countermeasure,
		}

		actionRunner, err := m.ActionRegistry.NewRunner(actionContext)
		if err != nil {
			return err
		}

		// TODO: handle suppression
		done := actionRunner.OnDetection(actionContext, event)
		m.waitForCompletion(objectKey, done)
	}

	return nil
}

// waitForCompletion spins up a goroutine to wait for the response from the action runner.
func (m *Manager) waitForCompletion(objectKey manager.ObjectKey, doneCh <-chan struct{}) {
	m.activeMux.Lock()
	defer m.activeMux.Unlock()
	m.active[objectKey] = struct{}{}

	go func(key manager.ObjectKey) {
		m.activeMux.Lock()
		defer m.activeMux.Unlock()
		select {
		case <-doneCh:
			// remove this from the active set, when the channel is closed
			// this active set is used to prevent the same countermeasure from
			// running concurrently.
			delete(m.active, key)
		default:
		}
	}(objectKey)
}

// Add install a countermeasure to route events to
func (m *Manager) Add(cm *v1alpha1.CounterMeasure) error {

	onEvent := cm.Spec.OnEvent

	key := manager.ToKey(cm.ObjectMeta)
	copy := *cm.DeepCopy()

	m.measuresMux.Lock()
	defer m.measuresMux.Unlock()
	m.deployed[key] = copy
	m.eventIndex[onEvent.EventName] = append(m.eventIndex[onEvent.EventName], key)

	if onEvent.SourceSelector != nil {
		managerLog.Info("Lookup up sources using source selector for countermeasure.",
			"name", cm.Name,
			"namespace", cm.Namespace)
		// TODO: this manager needs access to the client so that it can resolve all the
		// sources that this countermeasure will accept events from.
	}

	return nil
}

// Measure uninstall a countermeasure from the event subscription
func (m *Manager) Remove(name types.NamespacedName) error {
	m.measuresMux.Lock()
	defer m.measuresMux.Unlock()

	for k := range m.deployed {
		if k.NamespacedName == name {
			delete(m.deployed, k)
		}
	}

	return nil
}

// Exists uninstall a countermeasure from the event subscription
func (m *Manager) Exists(objectName metav1.ObjectMeta) bool {
	key := manager.ToKey(objectName)

	m.measuresMux.Lock()
	defer m.measuresMux.Unlock()

	_, ok := m.deployed[key]
	return ok
}
