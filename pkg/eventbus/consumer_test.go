package eventbus

import (
	"context"
	"testing"
	"time"

	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
)

func TestEventConsumer_Subscribe(t *testing.T) {
	bus := NewEventBus(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus.InjectLogger(testr.New(t))
	go bus.Start(ctx)

	consumer, err := bus.Subscribe("topic1")
	assert.GreaterOrEqual(t, len(consumer.Id()), 0)
	assert.NoError(t, err)

	err = bus.Publish("topic1", events.Event{
		Name: "event1",
	})
	assert.NoError(t, err)

	e := <-consumer.OnEvent()
	assert.Equal(t, "event1", e.Name)
}

func TestEventConsumer_SubscribeSync(t *testing.T) {
	bus := NewEventBus(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus.InjectLogger(testr.New(t))
	go bus.Start(ctx)

	consumer, err := bus.Subscribe("topic1")
	assert.NoError(t, err)

	err = bus.Publish("topic1", events.Event{
		Name: "event1",
	})
	assert.NoError(t, err)

	e := consumer.OnEventSync(context.TODO())
	assert.Equal(t, "event1", e.Name)
}

func TestEventConsumer_UnSubscribe(t *testing.T) {
	bus := NewEventBus(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus.InjectLogger(testr.New(t))
	go bus.Start(ctx)

	consumer, err := bus.Subscribe("topic1")
	assert.NoError(t, err)

	err = bus.Publish("topic1", events.Event{
		Name: "event1",
	})
	assert.NoError(t, err)

	e := <-consumer.OnEvent()
	assert.Equal(t, "event1", e.Name)

	err = consumer.UnSubscribe()
	assert.NoError(t, err)

	err = bus.Publish("topic1", events.Event{
		Name: "event2",
	})
	assert.NoError(t, err)

	select {
	case e := <-consumer.OnEvent():
		if e.Name == "event2" {
			t.Fatalf("received event after unsubscribe")
		}
	case <-time.After(time.Millisecond * 500):
		// expected this branch to occur since we don't expect any events after unsubscribe
	}
}

func TestEventConsumer_MergedSubscribe(t *testing.T) {
	bus := NewEventBus(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus.InjectLogger(testr.New(t))
	go bus.Start(ctx)

	consumer1, err := bus.Subscribe("alertA:namespace:provider1")
	assert.NoError(t, err)

	consumer2, err := bus.Subscribe("alertA:namespace:provider2")
	assert.NoError(t, err)

	consumer := MergeConsumers(consumer1, consumer2)
	assert.Greater(t, len(consumer.Id()), 0)
	assert.NotEqual(t, consumer1.Id(), consumer.Id())
	assert.NotEqual(t, consumer2.Id(), consumer.Id())

	err = bus.Publish("alertA:namespace:provider2", events.Event{
		Name: "event2",
	})
	assert.NoError(t, err)

	e := <-consumer.OnEvent()
	assert.Equal(t, "event2", e.Name)

	err = bus.Publish("alertA:namespace:provider1", events.Event{
		Name: "event1",
	})
	assert.NoError(t, err)

	e = <-consumer.OnEvent()
	assert.Equal(t, "event1", e.Name)
}

func TestEventConsumer_MergedSubscribeSync(t *testing.T) {
	bus := NewEventBus(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus.InjectLogger(testr.New(t))
	go bus.Start(ctx)

	consumer1, err := bus.Subscribe("alertA:namespace:provider1")
	assert.NoError(t, err)

	consumer2, err := bus.Subscribe("alertA:namespace:provider2")
	assert.NoError(t, err)

	consumer := MergeConsumers(consumer1, consumer2)

	err = bus.Publish("alertA:namespace:provider2", events.Event{
		Name: "event2",
	})
	assert.NoError(t, err)

	event := consumer.OnEventSync(context.TODO())
	assert.Equal(t, "event2", event.Name)
}

func TestEventConsumer_MergedUnsubscribe(t *testing.T) {
	bus := NewEventBus(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus.InjectLogger(testr.New(t))
	go bus.Start(ctx)

	consumer1, err := bus.Subscribe("alertA")
	assert.NoError(t, err)

	consumer2, err := bus.Subscribe("alertB")
	assert.NoError(t, err)

	consumer := MergeConsumers(consumer1, consumer2)
	assert.NoError(t, err)

	err = bus.Publish("alertA", events.Event{
		Name: "alertA",
	})
	assert.NoError(t, err)

	e := <-consumer.OnEvent()
	assert.Equal(t, "alertA", e.Name)

	err = bus.Publish("alertB", events.Event{
		Name: "alertB",
	})
	assert.NoError(t, err)

	e = <-consumer.OnEvent()
	assert.Equal(t, "alertB", e.Name)

	err = consumer.UnSubscribe()
	assert.NoError(t, err)

	err = bus.Publish("alertA", events.Event{
		Name: "alertA",
	})
	assert.NoError(t, err)

	select {
	case e := <-consumer.OnEvent():
		if e.Name == "alertA" {
			t.Fatalf("received event after unsubscribe")
		}
	case <-time.After(time.Millisecond * 500):
		// expected this branch to occur since we don't expect any events after unsubscribe
	}
}
