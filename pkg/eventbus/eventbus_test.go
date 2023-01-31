package eventbus

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

func TestMain(m *testing.M) {
	// disable the k8s.io/apimachinery runtime error handlers for the
	// tests we're intentionally causing retries and errors on to reduce noise
	utilruntime.ErrorHandlers = make([]func(error), 0)
	code := m.Run()
	os.Exit(code)
}

func TestDispatcher_Subscribe(t *testing.T) {

	bus := NewEventBus(1)
	bus.InjectLogger(testr.New(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start the dispatcher in a go routine so that the rest of the test can continue
	go bus.Start(ctx)

	data := make(events.EventData)
	data["d1"] = "v1"
	e1 := events.Event{
		Name:       "e1",
		ActiveTime: time.Now(),
		Data:       &data,
	}

	consumer, err := bus.Subscribe("e1")
	assert.NoError(t, err)

	bus.Publish("e1", e1)

	bus.subscribersMux.Lock()
	assert.Equal(t, 1, len(bus.subscribers))
	bus.subscribersMux.Unlock()

	event := consumer.OnEventSync(context.TODO())
	assert.Equal(t, e1.Name, event.Name)
	assert.Equal(t, e1.ActiveTime, event.ActiveTime)
	assert.Equal(t, 1, len(*event.Data))
	assert.Equal(t, "v1", event.Data.Get("d1"))

	bus.Publish("e1", e1)

	event = <-consumer.OnEvent()
	assert.Equal(t, e1.Name, event.Name)
	assert.Equal(t, e1.ActiveTime, event.ActiveTime)
	assert.Equal(t, 1, len(*event.Data))
	assert.Equal(t, "v1", event.Data.Get("d1"))
}

func TestDispatcher_UnSubscribe(t *testing.T) {

	bus := NewEventBus(1)
	bus.InjectLogger(testr.New(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start the dispatcher in a go routine so that the rest of the test can continue
	go bus.Start(ctx)

	data := make(events.EventData)
	data["d1"] = "v1"
	e1 := events.Event{
		Name:       "e1",
		ActiveTime: time.Now(),
		Data:       &data,
	}

	consumer, err := bus.Subscribe("e1")
	assert.NoError(t, err)

	bus.Publish("e1", e1)

	event := consumer.OnEventSync(context.TODO())
	assert.Equal(t, e1.Name, event.Name)
	assert.Equal(t, e1.ActiveTime, event.ActiveTime)
	assert.Equal(t, 1, len(*event.Data))
	assert.Equal(t, "v1", event.Data.Get("d1"))

	consumer.UnSubscribe()

	bus.Publish("e1", e1)
	timer := time.NewTimer(time.Millisecond * 1000000000)
	select {
	case e, ok := <-consumer.OnEvent():
		if ok {
			t.Fatal("Received an event when none was expected")
		}
		assert.Nil(t, e.Data)
	case <-timer.C:
		println("no event")
	}
}

func TestDispatcher_SubscribeRetry(t *testing.T) {

	bus := NewEventBus(1)
	bus.InjectLogger(logr.Discard())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start the dispatcher in a go routine so that the rest of the test can continue
	go bus.Start(ctx)

	consumer, err := bus.Subscribe("e1")
	assert.NoError(t, err)

	for i := 0; i < 20; i++ {
		data := make(events.EventData)
		data["d1"] = fmt.Sprintf("%d", i)
		e1 := events.Event{
			Name:       "e1",
			ActiveTime: time.Now(),
			Data:       &data,
		}

		bus.Publish("e1", e1)
		time.Sleep(30 * time.Millisecond)
	}

	// wait for some retries to expire
	time.Sleep(2 * time.Second)

	count := 0
	for e := range consumer.OnEvent() {
		i, err := strconv.Atoi(e.Data.Get("d1"))
		assert.NoError(t, err)
		assert.Equal(t, count, i)
		count++

		// if we don't get the 20 events this test will time out.
		if count >= 20 {
			cancel()
		}
	}

	assert.Equal(t, 20, count)
}

func TestDispatcher_SubscribeError(t *testing.T) {
	bus := NewEventBus(1)
	bus.InjectLogger(logr.Discard())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start the dispatcher in a go routine so that the rest of the test can continue
	go bus.Start(ctx)

	consumer, err := bus.Subscribe("e1")
	assert.NoError(t, err)

	for i := 0; i < 20; i++ {
		data := make(events.EventData)
		data["d1"] = fmt.Sprintf("%d", i)
		e1 := events.Event{
			Name:       "e1",
			ActiveTime: time.Now(),
			Data:       &data,
		}

		bus.Publish("e1", e1)
		time.Sleep(30 * time.Millisecond)
	}

	// wait for the retries to expire
	time.Sleep(6 * time.Second)

	cancel()

	count := 0
	for e := range consumer.OnEvent() {
		i, err := strconv.Atoi(e.Data.Get("d1"))
		assert.NoError(t, err)
		assert.Equal(t, count, i)
		count++
	}

	assert.Equal(t, 10, count)
}
