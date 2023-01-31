package eventbus

import (
	"context"
	"encoding/hex"
	"errors"
	"hash/fnv"
	"reflect"
	"strings"
	"time"

	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

const CONSUMER_BUFFER_SIZE = 10

var ErrChannelClosed = errors.New("channel closed")

type Consumer interface {
	Id() string
	OnEventSync(ctx context.Context) events.Event
	OnEvent() <-chan events.Event
	UnSubscribe() error
}

type EventConsumer struct {
	topic        string
	eventChannel chan events.Event
	consumerId   string
	bus          *EventBus
}

var _ Consumer = EventConsumer{}

type EventConsumerSet struct {
	consumers []Consumer
	mergedCh  chan events.Event
}

var _ Consumer = EventConsumerSet{}

func NewConsumer(bus *EventBus, topic string, consumerCh chan events.Event) Consumer {

	var sb strings.Builder
	sb.WriteString(topic)
	sb.WriteString(time.Now().Format(time.RFC3339Nano))

	h := fnv.New32a()
	h.Write([]byte(sb.String()))
	es := hex.EncodeToString(h.Sum(nil))

	return EventConsumer{
		topic:        topic,
		eventChannel: consumerCh,
		consumerId:   es,
		bus:          bus,
	}
}

func MergeConsumers(consumers ...Consumer) Consumer {
	consumerSlice := make([]Consumer, 0, len(consumers))
	consumerSlice = append(consumerSlice, consumers...)

	merged := make(chan events.Event)

	chs := make([]<-chan events.Event, 0, len(consumerSlice))
	for _, c := range consumerSlice {
		chs = append(chs, c.OnEvent())
	}

	go func(outCh chan<- events.Event, inputCh []<-chan events.Event) {
		ok := true
		for ok {
			event, err := Merge(context.Background(), chs)
			if err != nil {
				ok = !errors.Is(err, ErrChannelClosed)
				utilruntime.HandleError(err)
			}
			outCh <- event
		}
	}(merged, chs)

	return EventConsumerSet{
		consumers: consumerSlice,
		mergedCh:  merged,
	}
}

func (c EventConsumer) OnEventSync(ctx context.Context) events.Event {
	select {
	case evt := <-c.eventChannel:
		return evt
	case <-ctx.Done():
		// channel closed return empty event
	}
	return events.Event{}
}

func (c EventConsumer) Id() string {
	return c.consumerId
}

func (c EventConsumer) OnEvent() <-chan events.Event {
	return c.eventChannel
}

func (c EventConsumer) UnSubscribe() error {
	return c.bus.unsubscribe(c.topic, c.eventChannel)
}

func (c EventConsumerSet) Id() string {
	var sb strings.Builder
	for _, con := range c.consumers {
		sb.WriteString(con.Id())
	}

	h := fnv.New32a()
	h.Write([]byte(sb.String()))
	return hex.EncodeToString(h.Sum(nil))
}

func (cs EventConsumerSet) OnEventSync(ctx context.Context) events.Event {
	chs := make([]<-chan events.Event, len(cs.consumers))

	for _, c := range cs.consumers {
		chs = append(chs, c.OnEvent())
	}

	event, err := Merge(ctx, chs)
	if err != nil {
		utilruntime.HandleError(err)
	}

	return event
}

func (cs EventConsumerSet) OnEvent() <-chan events.Event {
	return cs.mergedCh
}

func (cs EventConsumerSet) UnSubscribe() error {
	for _, c := range cs.consumers {
		err := c.UnSubscribe()
		if err != nil {
			utilruntime.HandleError(err)
		}
	}

	return nil
}

func Merge[T any](ctx context.Context, chs []<-chan T) (T, error) {
	var msg T
	cases := make([]reflect.SelectCase, len(chs)+1)

	for i, ch := range chs {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}

	cases[len(chs)] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())}

	// ok will be true if the channel has not been closed.
	_, value, ok := reflect.Select(cases)
	if !ok {
		if ctx.Err() != nil {
			return msg, ctx.Err()
		}
		return msg, ErrChannelClosed
	}

	if ret, ok := value.Interface().(T); ok {
		return ret, nil
	}

	return msg, errors.New("failed to cast value")
}
