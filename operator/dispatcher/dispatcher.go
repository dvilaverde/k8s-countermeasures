package dispatcher

import (
	"context"
	"fmt"
	"time"

	"github.com/dvilaverde/k8s-countermeasures/operator/events"
	"github.com/go-logr/logr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

type EventListener interface {
	OnEvent(events.Event) error
}

type Dispatcher struct {
	logger        logr.Logger
	eventListener EventListener
	workqueue     workqueue.RateLimitingInterface
	workers       int
}

// NewDispatcher creates a new Dispatcher that uses multiple workers to dispatch events to an action listener
func NewDispatcher(eventListener EventListener, workers int) *Dispatcher {
	return &Dispatcher{
		workers:       workers,
		eventListener: eventListener,
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "EventDispatcher"),
	}
}

// Start implements the Runnable interface so that it can be started by the Operator SDK manager.
func (d *Dispatcher) Start(ctx context.Context) error {
	d.logger.Info(fmt.Sprintf("starting event dispatcher with %d workers", d.workers))

	// Launch two workers to process Foo resources
	for i := 0; i < d.workers; i++ {
		go wait.Until(d.runWorker, time.Second, ctx.Done())
	}

	<-ctx.Done()
	d.logger.Info("stopping event dispatcher")
	return nil
}

// EnqueueEvent queue an event to be processed
func (d *Dispatcher) EnqueueEvent(event events.Event) error {
	d.workqueue.Add(event)
	return nil
}

// InjectLogger injectable logger
// https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/runtime/inject/inject.go
func (d *Dispatcher) InjectLogger(logr logr.Logger) error {
	d.logger = logr
	return nil
}

func (d *Dispatcher) runWorker() {
	for d.processNextWorkItem() {
	}
}

func (d *Dispatcher) processNextWorkItem() bool {
	item, shutdown := d.workqueue.Get()

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
		var event events.Event
		var ok bool
		// We expect Events to come off the workqueue.
		if event, ok = obj.(events.Event); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			d.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected Event in workqueue but got %#v", obj))
			return nil
		}

		// Run the Actions for a CounterMeasure, it will need context about the CR containing
		// the action.
		var err error
		if err = d.eventListener.OnEvent(event); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			d.workqueue.AddRateLimited(event)
			return fmt.Errorf("error syncing '%s': %s, requeuing", event.Key(), err.Error())
		}

		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		d.workqueue.Forget(obj)
		d.logger.Info(fmt.Sprintf("Action successfully executed '%s'", event.Key()))
		return nil
	}(item)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true

}
