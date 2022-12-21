package countermeasure

import (
	"context"
	"time"

	operatorv1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"github.com/go-logr/logr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	log = ctrl.Log.WithName("monitor")
)

type CounterMeasureMonitor struct {
	client  client.Client
	logger  logr.Logger
	queue   workqueue.RateLimitingInterface
	workers int
}

func NewCounterMeasureMonitor(workers int) *CounterMeasureMonitor {
	return &CounterMeasureMonitor{
		queue:   workqueue.NewNamedRateLimitingQueue(NewSourceRateLimiter(), "countermeasuremonitor"),
		workers: workers,
	}
}

// InjectLogger injectable logger
// https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/runtime/inject/inject.go
func (c *CounterMeasureMonitor) InjectLogger(logr logr.Logger) error {
	c.logger = logr
	return nil
}

// InjectClient injectable client
// https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/runtime/inject/inject.go
func (c *CounterMeasureMonitor) InjectClient(client client.Client) error {
	c.client = client
	return nil
}

// Start implements the Runnable interface so this can participate in the manager lifecycle.
func (c *CounterMeasureMonitor) Start(ctx context.Context) error {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	log.Info("Starting CounterMeasure monitoring")

	log.Info("Starting monitor workers")
	// Launch two workers to process CounterMeasure resources
	for i := 0; i < c.workers; i++ {
		go wait.Until(c.runWorker, time.Second, ctx.Done())
	}

	log.Info("Started monitor workers")
	<-ctx.Done()
	return nil
}

func (c *CounterMeasureMonitor) StartMonitoring(countermeasure *operatorv1alpha1.CounterMeasure) error {
	key := ToKey(&countermeasure.ObjectMeta)
	c.queue.AddRateLimited(key)

	return nil
}

func (c *CounterMeasureMonitor) StopMonitoring(key string) error {

	// it's import to remove the key from the work queue, otherwise we'll leak
	// countermeasures being monitored.
	c.queue.Forget(key)

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *CounterMeasureMonitor) runWorker() {
	for c.processNextWorkItem() {
	}
	log.Info("counter measures monitor shutting down worker")
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *CounterMeasureMonitor) processNextWorkItem() bool {
	obj, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	defer c.queue.Done(obj)

	// TODO: do the actual work here
	// deleted, err := r.checkKey(key)
	// if err != nil {
	// 	utilruntime.HandleError(err)
	// }

	// if !deleted {
	// 	c.queue.AddRateLimited(key)
	// }

	return true
}

// SourceRateLimiter the rate limiter for the monitoring source
type SourceRateLimiter struct {
	interval time.Duration
}

func NewSourceRateLimiter() workqueue.RateLimiter {
	return &SourceRateLimiter{}
}

// When returns the interval of the rate limiter
func (r *SourceRateLimiter) When(item interface{}) time.Duration {
	// TODO: calculate the interval from the item
	return r.interval
}

// NumRequeues returns back how many failures the item has had
func (r *SourceRateLimiter) NumRequeues(item interface{}) int {
	return 1
}

// Forget indicates that an item is finished being retried.
func (r *SourceRateLimiter) Forget(item interface{}) {
}
