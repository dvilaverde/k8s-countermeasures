package dispatcher

import (
	"context"
	"testing"
	"time"

	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/events"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
)

type MockListener struct {
	events []events.Event
}

func (l *MockListener) OnEvent(event events.Event) error {
	l.events = append(l.events, event)
	return nil
}

func TestDispatcher_Start(t *testing.T) {
	listener := &MockListener{}
	dispatcher := NewDispatcher(listener, 2)
	dispatcher.InjectLogger(testr.New(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start the dispatcher in a go routine so that the rest of the test can continue
	go dispatcher.Start(ctx)

	data := make(events.EventData)
	data["d1"] = "v1"
	e1 := events.Event{
		Name:       "e1",
		ActiveTime: time.Now(),
		Data:       &data,
	}

	dispatcher.EnqueueEvent(e1)
	assert.Eventually(t, func() bool {
		return len(listener.events) == 1
	}, time.Second*5, time.Millisecond*50, "expected the event to be dequeued")

	event := listener.events[0]
	assert.Equal(t, e1.Name, event.Name)
	assert.Equal(t, e1.ActiveTime, event.ActiveTime)
	rcvData := *event.Data
	assert.Equal(t, 1, len(rcvData))
	assert.Equal(t, "v1", rcvData["d1"])
}
