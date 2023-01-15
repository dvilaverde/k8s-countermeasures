package sources

import (
	"context"
	"sync"

	"github.com/dvilaverde/k8s-countermeasures/pkg/dispatcher"
	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/dvilaverde/k8s-countermeasures/pkg/manager"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ manager.Manager[EventSource] = &Manager{}

type ActiveEventSources map[manager.ObjectKey]EventSource

type Manager struct {
	client   client.Client
	shutdown chan struct{}

	sourcesMux sync.Mutex
	sources    ActiveEventSources

	// GlobalPublisher is used to publish any events
	// over to the event manager, which will distribute further
	// to all the receivers.
	Dispatcher *dispatcher.Dispatcher
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
	key := manager.ObjectKey{
		NamespacedName: types.NamespacedName{Namespace: objectMeta.Namespace, Name: objectMeta.Name},
		Generation:     objectMeta.Generation,
	}

	m.sourcesMux.Lock()
	defer m.sourcesMux.Unlock()
	_, ok := m.sources[key]
	return ok
}

func (m *Manager) Add(es EventSource) error {
	m.sourcesMux.Lock()
	defer m.sourcesMux.Unlock()

	key := es.Key()
	m.sources[key] = es

	es.Subscribe(events.OnEventFunc(func(event events.Event) error {
		if (event.Source == events.SourceName{}) {

			// when there is an empty source lets populate it before propagating the event.
			event.Source = events.SourceName{
				Name:      key.Name,
				Namespace: key.Namespace,
			}
		}
		return m.Dispatcher.EnqueueEvent(event)
	}))
	// place the event source on a goroutine so that it wont' block this method
	go es.Start(m.shutdown)

	return nil
}

// Start satisfies the runnable interface and started by the Operator SDK manager.
func (m *Manager) Start(ctx context.Context) error {
	logger := log.FromContext(ctx)
	logger.Info("starting event source manager")

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
