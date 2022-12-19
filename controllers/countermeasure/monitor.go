package countermeasure

import (
	"time"

	"k8s.io/client-go/util/workqueue"
)

type CounterMeasureMonitor struct {
	queue workqueue.RateLimitingInterface
}

func NewCounterMeasureMonitor() *CounterMeasureMonitor {
	return &CounterMeasureMonitor{
		queue: workqueue.NewNamedRateLimitingQueue(NewSourceRateLimiter(), "countermeasuremonitor"),
	}
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
