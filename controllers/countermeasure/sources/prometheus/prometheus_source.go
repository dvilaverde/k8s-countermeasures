package prometheus

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	utilwait "k8s.io/apimachinery/pkg/util/wait"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	cm "github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure"
	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/events"
	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/sources"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type callback struct {
	name             types.NamespacedName
	alertSpec        *v1alpha1.PrometheusAlertSpec
	handler          sources.Handler
	suppressedAlerts map[string]time.Time
}

type EventSource struct {
	logger     logr.Logger
	client     client.Client
	p8sBuilder Builder

	interval time.Duration

	callbackMux    sync.Mutex
	p8Services     map[string]*PrometheusService
	p8sToCallbacks map[string][]*callback
}

func NewEventSource(p8ServiceBuilder Builder, interval time.Duration) *EventSource {
	return &EventSource{
		interval:   interval,
		p8sBuilder: p8ServiceBuilder,
	}
}

func (d *EventSource) Start(ctx context.Context) error {
	logger := log.FromContext(ctx)

	d.p8Services = make(map[string]*PrometheusService)
	d.p8sToCallbacks = make(map[string][]*callback)

	go utilwait.Until(d.poll, d.interval, ctx.Done())

	logger.Info("starting prometheus alert polling")

	return nil
}

// InjectLogger injectable logger
// https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/runtime/inject/inject.go
func (d *EventSource) InjectLogger(logr logr.Logger) error {
	d.logger = logr
	return nil
}

// InjectClient injectable client
// https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/runtime/inject/inject.go
func (d *EventSource) InjectClient(client client.Client) error {
	d.client = client
	return nil
}

func (d *EventSource) NotifyOn(countermeasure v1alpha1.CounterMeasure, handler sources.Handler) (sources.CancelFunc, error) {
	promConfig := countermeasure.Spec.Prometheus

	p8SvcKey := cm.ServiceToKey(promConfig.Service)

	d.callbackMux.Lock()
	defer d.callbackMux.Unlock()

	newCallback := &callback{
		name:             types.NamespacedName{Name: countermeasure.Name, Namespace: countermeasure.Namespace},
		alertSpec:        countermeasure.Spec.Prometheus.Alert.DeepCopy(),
		handler:          handler,
		suppressedAlerts: make(map[string]time.Time),
	}

	// the register the alert to the synchronized map
	if _, ok := d.p8sToCallbacks[p8SvcKey]; !ok {
		d.p8sToCallbacks[p8SvcKey] = append(make([]*callback, 0), newCallback)
	} else {
		// if a callback with this name already exists it needs to be removed first
		d.deleteCallbackByName(p8SvcKey, newCallback.name)
		d.p8sToCallbacks[p8SvcKey] = append(d.p8sToCallbacks[p8SvcKey], newCallback)
	}

	// use the promConfig to lookup the service
	_, ok := d.p8Services[p8SvcKey]
	if !ok {
		client, err := d.createP8sClient(&countermeasure)
		if err != nil {
			return func() {}, err
		}
		d.p8Services[p8SvcKey] = client
	}

	nsName := cm.ToNamespaceName(&countermeasure.ObjectMeta)
	return d.cancelFunction(nsName), nil
}

func (d *EventSource) Supports(countermeasure *v1alpha1.CounterMeasureSpec) bool {
	return countermeasure != nil && countermeasure.Prometheus != nil
}

// cancelFunction create a cancel function
func (d *EventSource) cancelFunction(key types.NamespacedName) func() {
	return func() {
		d.callbackMux.Lock()
		defer d.callbackMux.Unlock()

		for p8SvcKey := range d.p8sToCallbacks {
			d.deleteCallbackByName(p8SvcKey, key)
		}
	}
}

// deleteCallbackByName delete a callback by callback name
func (d *EventSource) deleteCallbackByName(p8sServiceKey string, name types.NamespacedName) {
	callbacks := d.p8sToCallbacks[p8sServiceKey]
	for idx, callback := range callbacks {
		if callback.name == name {
			if len(d.p8sToCallbacks[p8sServiceKey]) == 1 {
				delete(d.p8Services, p8sServiceKey)
				delete(d.p8sToCallbacks, p8sServiceKey)
			} else {
				d.p8sToCallbacks[p8sServiceKey] = append(callbacks[:idx], callbacks[idx+1:]...)
			}
		}
	}
}

// poll fetch alerts from each prometheus service and notify the callbacks on any active alerts
func (d *EventSource) poll() {
	d.callbackMux.Lock()
	defer d.callbackMux.Unlock()

	for svc, callbacks := range d.p8sToCallbacks {
		p8 := d.p8Services[svc]
		alerts, err := p8.GetActiveAlerts()
		if err != nil {
			d.logger.Error(err, "failed to get alerts from prometheus service", "prometheus_service", svc)
			return
		}

		if len(alerts.alerts) == 0 {
			continue
		}

		for _, cb := range callbacks {
			// remove any previously suppressed alerts
			cb.removeExpiredSuppressions()

			alertSpec := cb.alertSpec
			activeEvents, err := alerts.ToEvents(alertSpec.AlertName, alertSpec.IncludePending)
			if err != nil {
				var errPointer *AlertNotFiring
				if !errors.As(err, &errPointer) {
					d.logger.Error(err, "could not get events for alert", "alertName", alertSpec.AlertName)
				}
			}

			var unsuppressed []events.Event
			if len(cb.suppressedAlerts) == 0 {
				unsuppressed = activeEvents
			} else {
				// filter any events that are being suppressed
				unsuppressed = make([]events.Event, 0)
				for _, e := range activeEvents {
					if _, ok := cb.suppressedAlerts[e.Key()]; !ok {
						unsuppressed = append(unsuppressed, e)
					}
				}
			}

			if len(unsuppressed) > 0 {
				// start handling the actions on a goroutine so we can continue checking the
				// other alerts and callbacks
				callbackChannel := make(chan string)
				go cb.handler.OnDetection(cb.name, unsuppressed, callbackChannel)

				if cb.alertSpec.SuppressionPolicy != nil {
					go func(c *callback, e []events.Event) {
						eventTimes := make(map[string]time.Time)

						for _, event := range e {
							eventTimes[event.Key()] = event.ActiveTime
						}

						for k := range callbackChannel {
							if v, ok := eventTimes[k]; ok {
								c.suppressedAlerts[k] = v
							}
						}
					}(cb, unsuppressed)
				} else {
					go func() {
						for {
							if _, ok := <-callbackChannel; !ok {
								break
							}
						}
					}()
				}
			}
		}
	}
}

func (d *EventSource) createP8sClient(countermeasure *v1alpha1.CounterMeasure) (*PrometheusService, error) {
	promConfig := countermeasure.Spec.Prometheus
	svc := promConfig.Service

	serviceObject := &corev1.Service{}
	if err := d.client.Get(context.Background(), svc.GetNamespacedName(), serviceObject); err != nil {
		return nil, err
	}

	svcPort, found := findNamedPort(serviceObject, svc.TargetPort)
	var port int32
	if found {
		port = svcPort.Port
	} else {
		port = svc.Port
	}

	scheme := "http"
	if svc.UseTls {
		scheme = "https"
	}

	address := fmt.Sprintf("%v://%v.%v.svc:%v", scheme, svc.Name, svc.Namespace, port)

	var username, password string
	if promConfig.Auth != nil {
		secretRef := promConfig.Auth.SecretReference.DeepCopy()
		if len(secretRef.Namespace) == 0 {
			secretRef.Namespace = countermeasure.ObjectMeta.Namespace
		}
		secret, err := d.getSecret(secretRef)
		if err != nil {
			d.logger.Error(err, fmt.Sprintf("could not lookup secret %s in namespace %s", secretRef.Name, secretRef.Namespace))
		}

		username = string(secret.Data["username"])
		password = string(secret.Data["password"])
	}

	p8sClient, err := d.p8sBuilder(address, username, password)
	if err != nil {
		return nil, err
	}
	return p8sClient, nil
}

func (d *EventSource) getSecret(ref *corev1.SecretReference) (corev1.Secret, error) {
	secret := corev1.Secret{}

	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	err := d.client.Get(context.Background(), key, &secret)
	if err != nil {
		return corev1.Secret{}, err
	}

	// TODO: support auth TLS using secret ref
	if secret.Type != corev1.SecretTypeBasicAuth {
		return corev1.Secret{}, errors.New("only the basic auth type (kubernetes.io/basic-auth) is currently supported")
	}

	return secret, nil
}

func (c *callback) removeExpiredSuppressions() {

	if c.alertSpec.SuppressionPolicy == nil {
		return
	}

	now := time.Now()
	suppressDuration := c.alertSpec.SuppressionPolicy.Duration.Duration
	for k, v := range c.suppressedAlerts {
		if v.Add(suppressDuration).Before(now) {
			delete(c.suppressedAlerts, k)
		}
	}
}

func findNamedPort(service *corev1.Service, namedPort string) (corev1.ServicePort, bool) {
	portCount := len(service.Spec.Ports)
	if portCount == 1 {
		return service.Spec.Ports[0], true
	}

	if portCount > 1 {
		// find the port by the name
		for _, port := range service.Spec.Ports {
			if port.Name == namedPort {
				return port, true
			}
		}
	}

	return corev1.ServicePort{}, false
}
