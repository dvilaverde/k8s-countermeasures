package prometheus

import (
	"errors"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"

	"github.com/dvilaverde/k8s-countermeasures/apis/eventsource/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/dvilaverde/k8s-countermeasures/pkg/manager"
	"github.com/dvilaverde/k8s-countermeasures/pkg/sources"
	ctrl "sigs.k8s.io/controller-runtime"
)

var prometheusLogger = ctrl.Log.WithName("prometheus_eventsource")

type EventSource struct {
	p8Client *PrometheusService
	key      manager.ObjectKey

	interval time.Duration
	pending  bool

	subscriptionMux sync.Mutex
	subscribers     []events.EventPublisher
}

var _ sources.EventSource = &EventSource{}

func NewEventSource(prometheus *v1alpha1.Prometheus, p8Client *PrometheusService) *EventSource {

	interval := prometheus.Spec.PollingInterval
	pending := prometheus.Spec.IncludePending

	return &EventSource{
		key: manager.ObjectKey{
			NamespacedName: types.NamespacedName{
				Namespace: prometheus.Namespace,
				Name:      prometheus.Name,
			},
			Generation: prometheus.Generation,
		},
		interval: interval.Duration,
		pending:  pending,
		p8Client: p8Client,
	}
}

func (d *EventSource) Start(done <-chan struct{}) error {

	prometheusLogger.Info("starting prometheus alert polling")
	go utilwait.Until(d.poll, d.interval, done)

	<-done

	prometheusLogger.Info("stopping prometheus alert polling")

	return nil
}

func (d *EventSource) Key() manager.ObjectKey {
	return d.key
}

func (d *EventSource) Subscribe(subscriber events.EventPublisher) error {
	d.subscriptionMux.Lock()
	defer d.subscriptionMux.Unlock()

	d.subscribers = append(d.subscribers, subscriber)

	return nil
}

// poll fetch alerts from each prometheus service and notify the callbacks on any active alerts
func (d *EventSource) poll() {
	// don't bother when there are no subscribers
	if len(d.subscribers) == 0 {
		return
	}

	d.subscriptionMux.Lock()
	defer d.subscriptionMux.Unlock()

	alerts, err := d.p8Client.GetActiveAlerts()
	if err != nil {
		prometheusLogger.Error(err, "failed to get alerts from prometheus service", "prometheus_service", d.Key().GetName())
		return
	}

	eventsToPublish, err := alerts.ToEvents(d.pending)
	if (err != nil && !errors.Is(err, &AlertNotFiring{})) {
		utilruntime.HandleError(err)
		return
	}

	if len(eventsToPublish) == 0 {
		// normal condition, so just end processing
		return
	}

	for _, event := range eventsToPublish {
		for _, subscriber := range d.subscribers {
			subscriber.Publish(event)
		}
	}
}
