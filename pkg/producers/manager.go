package producers

import (
	"context"
	"sync"

	"github.com/dvilaverde/k8s-countermeasures/pkg/eventbus"
	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/dvilaverde/k8s-countermeasures/pkg/manager"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ manager.Manager[EventProducer] = &Manager{}

type ActiveEventSources map[manager.ObjectKey]EventProducer

type Manager struct {
	client   client.Client
	shutdown chan struct{}

	sourcesMux sync.Mutex
	sources    ActiveEventSources

	// TODO: GlobalPublisher is used to publish any events
	// over to the event manager, which will distribute further
	// to all the receivers.
	EventBus *eventbus.EventBus
}

func (m *Manager) Remove(name types.NamespacedName) error {
	m.sourcesMux.Lock()
	defer m.sourcesMux.Unlock()

	for k := range m.sources {
		if k.NamespacedName == name {
			delete(m.sources, k)
		}
	}

	return nil
}

func (m *Manager) Exists(objectMeta metav1.ObjectMeta) bool {
	key := manager.ToKey(objectMeta)

	m.sourcesMux.Lock()
	defer m.sourcesMux.Unlock()
	_, ok := m.sources[key]
	return ok
}

func (m *Manager) Add(producer EventProducer) error {
	m.sourcesMux.Lock()
	defer m.sourcesMux.Unlock()

	key := producer.Key()
	m.sources[key] = producer

	producer.Subscribe(events.OnEventFunc(func(event events.Event) error {
		if (event.Source == events.SourceName{}) {

			// when there is an empty source lets populate it before propagating the event.
			event.Source = events.SourceName{
				Name:      key.Name,
				Namespace: key.Namespace,
			}
		}
		return m.EventBus.EnqueueEvent(event)
	}))
	// place the event source on a goroutine so that it wont' block this method
	go producer.Start(m.shutdown)

	return nil
}

// Start satisfies the runnable interface and started by the Operator SDK manager.
func (m *Manager) Start(ctx context.Context) error {
	logger := log.FromContext(ctx)
	logger.Info("starting event source manager")

	m.sourcesMux.Lock()
	m.sources = make(ActiveEventSources)
	m.shutdown = make(chan struct{})
	m.sourcesMux.Unlock()

	<-ctx.Done()

	logger.Info("stopping event source manager")
	close(m.shutdown)
	return nil
}

// InjectClient injectable client
func (m *Manager) InjectClient(client client.Client) error {
	m.client = client
	return nil
}
