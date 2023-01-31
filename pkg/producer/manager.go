package producer

import (
	"context"
	"sync"

	"github.com/dvilaverde/k8s-countermeasures/pkg/eventbus"
	"github.com/dvilaverde/k8s-countermeasures/pkg/manager"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ manager.Manager[KeyedEventProducer] = &Manager{}

var (
	managerLog  = ctrl.Log.WithName("producer_manager")
	ALWAYS_TRUE = func(err error) bool { return true }
)

type Manager struct {
	client   client.Client
	shutdown chan struct{}

	producersMux sync.RWMutex
	producers    map[manager.ObjectKey]KeyedEventProducer

	// This event bus is used by the event producers to publish
	// events to all the subscribing consumers (i.e. CounterMeasures).
	eventBus *eventbus.EventBus
}

func NewManager(bus *eventbus.EventBus) *Manager {
	return &Manager{
		eventBus:     bus,
		producersMux: sync.RWMutex{},
		producers:    make(map[manager.ObjectKey]KeyedEventProducer),
	}
}

// Remove remove an event producer
func (m *Manager) Remove(name types.NamespacedName) error {
	m.producersMux.Lock()
	defer m.producersMux.Unlock()

	for k := range m.producers {
		if k.NamespacedName == name {
			delete(m.producers, k)
		}
	}

	return nil
}

// Exists returns true if this event producer has already been registred
func (m *Manager) Exists(objectMeta metav1.ObjectMeta) bool {
	key := manager.ToKey(objectMeta)

	m.producersMux.RLock()
	defer m.producersMux.RUnlock()
	_, ok := m.producers[key]
	return ok
}

// Add add an event producer to this manager, if this manager is already started
// then the producer will also be started, otherwise the producer will start when
// the manager starts.
func (m *Manager) Add(producer KeyedEventProducer) error {
	m.producersMux.Lock()
	defer m.producersMux.Unlock()

	key := producer.Key()
	m.producers[key] = producer

	// only start when the manager is started.
	if m.shutdown != nil {
		// place the event source on a goroutine so that it won't block this method
		go func() {
			retry.OnError(retry.DefaultRetry, ALWAYS_TRUE, func() error {
				return producer.Start(m.shutdown)
			})
		}()
	}

	return nil
}

// Start satisfies the runnable interface and started by the Operator SDK manager.
func (m *Manager) Start(ctx context.Context) error {
	logger := log.FromContext(ctx)
	logger.Info("starting event source manager")

	m.startProducers()

	<-ctx.Done()

	logger.Info("stopping event source manager")
	close(m.shutdown)
	return nil
}

// startProducers starts all producers and throws an error if any fail to start.
func (m *Manager) startProducers() error {
	m.producersMux.Lock()
	defer m.producersMux.Unlock()

	m.shutdown = make(chan struct{})

	if len(m.producers) > 0 {
		for key, producer := range m.producers {
			err := retry.OnError(retry.DefaultRetry, ALWAYS_TRUE, func() error {
				return producer.Start(m.shutdown)
			})
			if err != nil {
				managerLog.Error(err, "failed to start manager", "name", key.GetName())
				return err
			}
		}
	}

	return nil
}

// InjectClient injectable client
func (m *Manager) InjectClient(client client.Client) error {
	m.client = client
	return nil
}
