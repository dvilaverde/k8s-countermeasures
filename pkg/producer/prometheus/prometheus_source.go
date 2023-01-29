package prometheus

import (
	"errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/dvilaverde/k8s-countermeasures/pkg/manager"
	"github.com/dvilaverde/k8s-countermeasures/pkg/producer"
	ctrl "sigs.k8s.io/controller-runtime"
)

var prometheusLogger = ctrl.Log.WithName("prometheus_eventsource")

type PrometheusConfig struct {
	PollInterval   time.Duration
	IncludePending bool
	Key            manager.ObjectKey
	Client         *PrometheusService
}

type EventProducer struct {
	config   PrometheusConfig
	producer producer.EventProducer
}

var _ producer.KeyedEventProducer = &EventProducer{}

// NewEventProducer creation function for a new Prometheus EventProducer
func NewEventProducer(cfg PrometheusConfig, prd producer.EventProducer) *EventProducer {
	return &EventProducer{
		config:   cfg,
		producer: prd,
	}
}

// Start called to start this EventProducer publishing to the event bus.
func (d *EventProducer) Start(done <-chan struct{}) error {

	prometheusLogger.Info("starting prometheus alert polling")
	go utilwait.Until(d.poll, d.config.PollInterval, done)

	<-done

	prometheusLogger.Info("stopping prometheus alert polling")
	return nil
}

// Publish send the event to the bus, retrying on any errors
func (d *EventProducer) Publish(topic string, event events.Event) error {
	return retry.OnError(retry.DefaultBackoff, func(err error) bool { return true }, func() error {
		return d.producer.Publish(topic, event)
	})
}

// poll fetch alerts from each prometheus service and notify the callbacks on any active alerts
func (d *EventProducer) poll() {

	alerts, err := d.getClient().GetActiveAlerts()
	if err != nil {
		prometheusLogger.Error(err, "failed to get alerts from prometheus service", "prometheus_service", d.getName())
		return
	}

	eventsToPublish, err := alerts.ToEvents(d.config.IncludePending)
	if (err != nil && !errors.Is(err, &AlertNotFiring{})) {
		utilruntime.HandleError(err)
		return
	}

	if len(eventsToPublish) == 0 {
		// normal condition, so just end processing
		return
	}

	for _, event := range eventsToPublish {
		if err := d.Publish(event.Name, event); err != nil {
			prometheusLogger.Error(err, fmt.Sprintf("failed to publish event %v", event.Name))
		}
	}
}

func (d *EventProducer) Key() manager.ObjectKey {
	return d.config.Key
}

func (d *EventProducer) getClient() *PrometheusService {
	return d.config.Client
}

func (d *EventProducer) getName() types.NamespacedName {
	return d.config.Key.NamespacedName
}
