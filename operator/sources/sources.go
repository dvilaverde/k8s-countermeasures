package sources

import (
	"context"
	"sync"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/operator/events"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EventPublisher interface {
	Publish(events.Event) error
}
type EventPublisherFunc func(events.Event) error

func (pub EventPublisherFunc) Publish(event events.Event) error {
	return pub(event)
}

type ObjectKey struct {
	types.NamespacedName
	Generation int64
}
type ActiveEventSources map[ObjectKey]EventSource
type ActiveCounterMeasures map[ObjectKey]struct{}

type EventManager interface {
	RemoveSource(types.NamespacedName) error

	// Exists Must check for the correct Generation vs existing, if
	// the Generations don't match then will return false.
	SourceExists(metav1.ObjectMeta) bool

	AddSource(EventSource) error

	AddCounterMeasure(*v1alpha1.CounterMeasure) error
	RemoveCounterMeasure(types.NamespacedName) error
	CounterMeasureExists(metav1.ObjectMeta) bool
}

type EventSource interface {
	Key() ObjectKey
	Start() error
	Subscribe(EventPublisher) error
	Stop() error
}

var _ EventManager = &Manager{}

type Manager struct {
	logger logr.Logger
	client client.Client

	publisher EventPublisher

	sourcesMux sync.Mutex
	sources    ActiveEventSources

	measuresMux sync.Mutex
	measures    ActiveCounterMeasures
}

func (m *Manager) RemoveSource(name types.NamespacedName) error {
	m.sourcesMux.Lock()
	defer m.sourcesMux.Unlock()

	for k := range m.sources {
		if k.NamespacedName == name {
			delete(m.sources, k)
		}
	}

	return nil
}

func (m *Manager) SourceExists(objectMeta metav1.ObjectMeta) bool {
	key := ObjectKey{
		NamespacedName: types.NamespacedName{Namespace: objectMeta.Namespace, Name: objectMeta.Name},
		Generation:     objectMeta.Generation,
	}

	m.sourcesMux.Lock()
	defer m.sourcesMux.Unlock()
	_, ok := m.sources[key]
	return ok
}

func (m *Manager) AddSource(es EventSource) error {
	m.sourcesMux.Lock()
	defer m.sourcesMux.Unlock()

	key := es.Key()
	m.sources[key] = es

	es.Subscribe(EventPublisherFunc(func(event events.Event) error {
		if (event.Source == events.SourceName{}) {

			// when there is an empty source lets populate it before propagating the event.
			event.Source = events.SourceName{
				Name:      key.Name,
				Namespace: key.Namespace,
			}
		}
		return m.publisher.Publish(event)
	}))
	es.Start()

	return nil
}

// AddCounterMeasure install a countermeasure to route events to
func (m *Manager) AddCounterMeasure(cm *v1alpha1.CounterMeasure) error {
	return nil
}

// RemoveCounterMeasure uninstall a countermeasure from the event subscription
func (m *Manager) RemoveCounterMeasure(name types.NamespacedName) error {
	m.measuresMux.Lock()
	defer m.measuresMux.Unlock()

	for k := range m.measures {
		if k.NamespacedName == name {
			delete(m.measures, k)
		}
	}

	return nil
}

// CounterMeasureExists uninstall a countermeasure from the event subscription
func (m *Manager) CounterMeasureExists(objectName metav1.ObjectMeta) bool {
	key := ObjectKey{
		NamespacedName: types.NamespacedName{Namespace: objectName.Namespace, Name: objectName.Name},
		Generation:     objectName.Generation,
	}

	m.measuresMux.Lock()
	defer m.measuresMux.Unlock()

	_, ok := m.measures[key]
	return ok
}

// Start satisfies the runnable interface and started by the Operator SDK manager.
func (m *Manager) Start(ctx context.Context) error {

	m.logger.Info("starting event source manager")

	<-ctx.Done()

	m.logger.Info("stopping event source manager")
	for _, v := range m.sources {
		v.Stop()
	}
	return nil
}

// InjectLogger injectable logger
func (m *Manager) InjectLogger(logr logr.Logger) error {
	m.logger = logr
	return nil
}

// InjectClient injectable client
func (m *Manager) InjectClient(client client.Client) error {
	m.client = client
	return nil
}
