package eventbus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/go-logr/logr"
	"golang.org/x/time/rate"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type message struct {
	event events.Event
	topic string
}

type Subscribers map[string][]chan events.Event

type EventBus struct {
	logger             logr.Logger
	workerShutdownChan chan struct{}
	workqueue          workqueue.RateLimitingInterface
	workers            int

	// map of topics to subscribers
	subscribersMux sync.RWMutex
	subscribers    Subscribers
}

func NewRateLimiter() workqueue.RateLimiter {
	return workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(1000*time.Millisecond, 5000*time.Second),
		// 10 qps, 100 bucket size.  This is only for retry speed and its only the overall factor (not per item)
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
	)
}

// NewEventBus creates a new EventBus that uses multiple workers to publish events to any subscribers
func NewEventBus(workers int) *EventBus {
	return &EventBus{
		workers:            workers,
		workerShutdownChan: make(chan struct{}),
		subscribersMux:     sync.RWMutex{},
		subscribers:        make(Subscribers),
		workqueue:          workqueue.NewNamedRateLimitingQueue(NewRateLimiter(), "EventDispatcher"),
	}
}

// Start implements the Runnable interface so that it can be started by the Operator SDK manager.
func (d *EventBus) Start(ctx context.Context) error {
	log := log.FromContext(ctx)
	log.Info(fmt.Sprintf("starting event dispatcher with %d workers", d.workers))

	// Launch two workers to process Foo resources
	for i := 0; i < d.workers; i++ {
		go wait.Until(d.runWorker, time.Second, d.workerShutdownChan)
	}

	<-ctx.Done()
	log.Info("stopping event dispatcher")

	d.workqueue.ShutDownWithDrain()

	d.subscribersMux.RLock()
	defer d.subscribersMux.RUnlock()

	// class all the EventConsumer channels
	for _, subscribersSlice := range d.subscribers {
		for _, consumer := range subscribersSlice {
			close(consumer)
		}
	}
	d.subscribers = make(Subscribers)

	d.workerShutdownChan <- struct{}{}

	return nil
}

// Publish queue an event to a topic
func (d *EventBus) Publish(topic string, event events.Event) error {
	d.workqueue.Add(message{
		topic: topic,
		event: event,
	})
	return nil
}

// Subscribe registers a subscription on the topic
func (d *EventBus) Subscribe(topic string) (Consumer, error) {
	d.subscribersMux.Lock()
	defer d.subscribersMux.Unlock()

	consumerCh := make(chan events.Event, CONSUMER_BUFFER_SIZE)
	consumer := NewConsumer(d, topic, consumerCh)
	d.subscribers[topic] = append(d.subscribers[topic], consumerCh)

	return consumer, nil
}

func (d *EventBus) unsubscribe(topic string, ch chan events.Event) error {
	d.subscribersMux.Lock()
	defer d.subscribersMux.Unlock()

	subscribers := d.subscribers[topic]
	for idx, c := range subscribers {
		if ch == c {
			d.subscribers[topic] = append(subscribers[:idx], subscribers[idx+1:]...)
			defer close(ch)
			break
		}
	}

	return nil
}

// InjectLogger injectable logger
// https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/runtime/inject/inject.go
func (d *EventBus) InjectLogger(logr logr.Logger) error {
	d.logger = logr
	return nil
}

func (d *EventBus) runWorker() {
	for d.processNextWorkItem() {
	}
}

func (d *EventBus) processNextWorkItem() bool {
	msg, shutdown := d.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer d.workqueue.Done(obj)
		var m message
		var ok bool
		// We expect Events to come off the workqueue.
		if m, ok = obj.(message); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			d.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected Event in workqueue but got %#v", obj))
			return nil
		}

		// Notify the subscribers about the event
		d.subscribersMux.RLock()
		defer d.subscribersMux.RUnlock()

		subscribers, ok := SubscriberMatch(d.subscribers, m.topic)

		if ok {
			for _, consumer := range subscribers {
				select {
				case consumer <- m.event:
				default:
					// Put the item back on the workqueue to handle any transient errors.
					d.workqueue.AddRateLimited(m)
					return fmt.Errorf("consumer not ready for event '%s', requeuing", m.event.Key())
				}
			}

			d.logger.Info(fmt.Sprintf("Event successfully delivered '%s to %d subscribers",
				m.event.Key(),
				len(subscribers)))
		}

		// Finally, if no error occurs we Forget this item so it does not get queued again.
		d.workqueue.Forget(obj)
		return nil
	}(msg)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true

}
