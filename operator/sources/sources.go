package sources

import (
	"context"
	"fmt"
	"sync"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	esv1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/eventsource/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/operator/events"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Handler interface {
	OnDetection(types.NamespacedName, []events.Event, chan<- string)
}

type HandlerFunc func(types.NamespacedName, []events.Event, chan<- string)

func (handler HandlerFunc) OnDetection(name types.NamespacedName, event []events.Event, done chan<- string) {
	handler(name, event, done)
}

type CancelFunc func()

type Source interface {
	NotifyOn(countermeasure v1alpha1.CounterMeasure, callback Handler) (CancelFunc, error)

	Supports(countermeasure *v1alpha1.CounterMeasureSpec) bool
}

// NEW CODE

type EventManager interface {
	Remove(types.NamespacedName) error

	// Exists Must check for the correct Generation vs existing, if the Generations don't match then will return false.
	Exists(*esv1alpha1.Prometheus) (bool, error)

	Add(*esv1alpha1.Prometheus) error
}

type monitoredResource struct {
	cancel     CancelFunc
	generation int64
}

type EventPublisher interface {
	Publish(events.Event) error
}
type EventPublisherFunc func(events.Event) error

type EventProvider interface {
}

type NamedSourceEventPublisher struct {
	SourceName        types.NamespacedName
	DelegatePublisher EventPublisher
}

func (ep *NamedSourceEventPublisher) Publish(event events.Event) error {
	if (event.Source == events.SourceName{}) {
		// when there is an empty source lets populate it before propagating the event.
		event.Source = events.SourceName{
			Name:      ep.SourceName.Name,
			Namespace: ep.SourceName.Namespace,
		}
	}

	return ep.DelegatePublisher.Publish(event)
}

type Manager struct {
	logger logr.Logger
	client client.Client

	publisher EventPublisher

	// a map of the actively monitored custom resources to a cancellation function
	activelyMonitored map[types.NamespacedName]monitoredResource

	providerMux sync.Mutex
	providers   []EventProvider
}

// Register will add an event provider to the event manager, the event provider will
// provide events to the dispatcher for taking action on.
func (m *EventManager) Register(provider EventProvider) error {
	m.providerMux.Lock()
	defer m.providerMux.Unlock()

	m.providers = append(m.providers, provider)
	return nil
}

// Start satisfies the runnable interface and started by the Operator SDK manager.
func (m *EventManager) Start(ctx context.Context) error {

	m.logger.Info("starting event source manager")
	m.activelyMonitored = make(map[types.NamespacedName]monitoredResource)

	<-ctx.Done()
	return nil
}

func (m *EventManager) IsMonitoring(name types.NamespacedName) bool {

	// if the generation hasn't changed from what we're monitoring then short return
	_, ok := m.activelyMonitored[name]
	return ok
}

func (m *EventManager) BeginMonitoring(name types.NamespacedName) error {
	// get the client and check if the current resource matches what is being
	// currently monitored.
	resource := &v1alpha1.CounterMeasure{}
	err := m.client.Get(context.Background(), name, resource)
	if err != nil {
		return err
	}

	if monitorHandle, ok := m.activelyMonitored[name]; ok {
		// check the generation and if different then cancel the monitor to cleanup
		// resources on updates
		if monitorHandle.generation != resource.Generation {
			monitorHandle.cancel()
		}

		// now we're clear to start monitoring the updated version of the CounterMeasure
	}

	// TODO: create a function that can be used to cancel everything, although maybe not needed
	//       if using a new CR:
	//
	//       apiVersion: operator.vilaverde.rocks
	//		 Kind: EventSource
	//
	m.activelyMonitored[name] = monitoredResource{
		cancel:     nil,
		generation: resource.Generation,
	}

	return nil
}

func (m *EventManager) EndMonitoring(name types.NamespacedName) error {
	if monitorHandle, ok := m.activelyMonitored[name]; ok {
		monitorHandle.cancel()
		// delete the key from the list of actively monitored CRs
		delete(m.activelyMonitored, name)

		m.logger.Info("stopped monitoring countermeasure", "name", name.Name, "namespace", name.Namespace)
	} else {
		return fmt.Errorf("trying to stop monitoring of '%s/%s' but it is not actively monitored",
			name.Namespace,
			name.Name)
	}

	return nil
}

// InjectLogger injectable logger
func (m *EventManager) InjectLogger(logr logr.Logger) error {
	m.logger = logr
	return nil
}

// InjectClient injectable client
func (m *EventManager) InjectClient(client client.Client) error {
	m.client = client
	return nil
}
