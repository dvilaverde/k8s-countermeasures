package producer

import (
	"context"
	"testing"
	"time"

	"github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/pkg/eventbus"
	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/dvilaverde/k8s-countermeasures/pkg/manager"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var dummySource KeyedEventProducer = &DummyEventSource{}

func TestManager_Remove(t *testing.T) {
	mgr := getManager(t)
	assert.NotNil(t, mgr)

	mgr.Remove(dummySource.Key().NamespacedName)
	assert.Equal(t, 0, len(mgr.producers))
}

func TestManager_Exists(t *testing.T) {
	mgr := getManager(t)
	assert.NotNil(t, mgr)

	cm := &v1alpha1.CounterMeasure{
		ObjectMeta: v1.ObjectMeta{
			Name:       dummySource.Key().Name,
			Namespace:  dummySource.Key().Namespace,
			Generation: dummySource.Key().Generation,
		},
	}

	assert.True(t, mgr.Exists(cm.ObjectMeta))

	cm.ObjectMeta.Generation = 2
	assert.False(t, mgr.Exists(cm.ObjectMeta))
}

func TestManager_Add(t *testing.T) {
	mgr := getManager(t)
	assert.NotNil(t, mgr)
}

func getManager(t *testing.T) *Manager {
	mgr := &Manager{
		eventBus:  eventbus.NewEventBus(1),
		producers: make(map[manager.ObjectKey]KeyedEventProducer),
	}
	mgr.eventBus.InjectLogger(testr.New(t))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	go mgr.Start(ctx)
	go mgr.eventBus.Start(ctx)

	assert.Eventually(t, func() bool {
		mgr.producersMux.Lock()
		defer mgr.producersMux.Unlock()
		return mgr.producers != nil
	}, time.Second, time.Millisecond*10, "expecting that the producers map to be initialized")

	mgr.Add(dummySource)
	assert.Equal(t, 1, len(mgr.producers))
	return mgr
}

type DummyEventSource struct {
}

func (d *DummyEventSource) Key() manager.ObjectKey {
	return manager.ObjectKey{
		NamespacedName: types.NamespacedName{Namespace: "ns", Name: "name"},
		Generation:     1,
	}
}

func (d *DummyEventSource) Publish(string, events.Event) error {
	return nil
}

func (d *DummyEventSource) Start(ch <-chan struct{}) error {
	<-ch
	return nil
}
