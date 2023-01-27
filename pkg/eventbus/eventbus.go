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

type EventBus struct {
	logger             logr.Logger
	workerShutdownChan chan struct{}
	workqueue          workqueue.RateLimitingInterface
	workers            int

	// map of topics to consumers
	consumersMux sync.RWMutex
	consumers    map[string][]EventConsumer
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
		consumersMux:       sync.RWMutex{},
		consumers:          make(map[string][]EventConsumer),
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

	d.consumersMux.RLock()
	defer d.consumersMux.RUnlock()

	// class all the EventConsumer channels
	for _, consumers := range d.consumers {
		for _, consumer := range consumers {
			close(consumer.eventChannel)
		}
	}

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
func (d *EventBus) Subscribe(topic string) (EventConsumer, error) {
	d.consumersMux.Lock()
	defer d.consumersMux.Unlock()

	consumer := NewConsumer(d, topic)
	d.consumers[topic] = append(d.consumers[topic], consumer)

	return consumer, nil
}

func (d *EventBus) unsubscribe(consumer EventConsumer) error {
	d.consumersMux.Lock()
	defer d.consumersMux.Unlock()

	subscribers := d.consumers[consumer.topic]
	for idx, c := range subscribers {
		if c.consumerId == consumer.consumerId {
			d.consumers[consumer.topic] = append(subscribers[:idx], subscribers[idx+1:]...)
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
		d.consumersMux.RLock()
		defer d.consumersMux.RUnlock()

		for _, consumer := range d.consumers[m.topic] {
			select {
			case consumer.eventChannel <- m.event:
			default:
				// Put the item back on the workqueue to handle any transient errors.
				d.workqueue.AddRateLimited(m)
				return fmt.Errorf("consumer '%s' not ready for event '%s', requeuing", consumer.consumerId, m.event.Key())
			}
		}

		// Finally, if no error occurs we Forget this item so it does not get queued again.
		d.workqueue.Forget(obj)
		d.logger.Info(fmt.Sprintf("Event successfully delivered '%s'", m.event.Key()))
		return nil
	}(msg)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true

}
