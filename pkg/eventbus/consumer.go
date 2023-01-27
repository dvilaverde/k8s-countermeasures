package eventbus

import (
	"encoding/hex"
	"hash/fnv"
	"strings"
	"time"

	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
)

const CONSUMER_BUFFER_SIZE = 10

type EventConsumer struct {
	topic        string
	eventChannel chan events.Event
	consumerId   string
	bus          *EventBus
}

func NewConsumer(bus *EventBus, topic string) EventConsumer {

	var sb strings.Builder
	sb.WriteString(topic)
	sb.WriteString(time.Now().Format(time.RFC3339Nano))

	h := fnv.New32a()
	h.Write([]byte(sb.String()))
	es := hex.EncodeToString(h.Sum(nil))

	return EventConsumer{
		topic:        topic,
		eventChannel: make(chan events.Event, CONSUMER_BUFFER_SIZE),
		consumerId:   es,
		bus:          bus,
	}
}

func (c EventConsumer) OnEventSync() events.Event {
	return <-c.eventChannel
}

func (c EventConsumer) OnEvent() <-chan events.Event {
	return c.eventChannel
}

func (c EventConsumer) UnSubscribe() error {
	return c.bus.unsubscribe(c)
}
