package prometheus

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	cm "github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure"
	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/detect"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type callback struct {
	name      types.NamespacedName
	alertSpec *v1alpha1.PrometheusAlertSpec
	handler   detect.Handler
}

type Detector struct {
	logger     logr.Logger
	client     client.Client
	p8sBulider Builder

	interval time.Duration

	callbackMux    sync.Mutex
	p8Services     map[string]*PrometheusService
	p8sToCallbacks map[string][]callback
}

func NewDetector(p8ServiceBuilder Builder, interval time.Duration) *Detector {
	return &Detector{
		interval:   interval,
		p8sBulider: p8ServiceBuilder,
	}
}

func (d *Detector) Start(ctx context.Context) error {
	// TODO: make the ticker configurable
	logger := log.FromContext(ctx)

	d.p8Services = make(map[string]*PrometheusService)
	d.p8sToCallbacks = make(map[string][]callback)

	ticker := time.NewTicker(d.interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				d.poll()
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()

	logger.Info("starting prometheus alert polling")

	return nil
}

// InjectLogger injectable logger
// https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/runtime/inject/inject.go
func (d *Detector) InjectLogger(logr logr.Logger) error {
	d.logger = logr
	return nil
}

// InjectClient injectable client
// https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/runtime/inject/inject.go
func (d *Detector) InjectClient(client client.Client) error {
	d.client = client
	return nil
}

func (d *Detector) NotifyOn(countermeasure v1alpha1.CounterMeasure, handler detect.Handler) (detect.CancelFunc, error) {
	promConfig := countermeasure.Spec.Prometheus

	p8SvcKey := cm.ServiceToKey(promConfig.Service)

	d.callbackMux.Lock()
	defer d.callbackMux.Unlock()

	// use the promConfig to lookup the service
	_, ok := d.p8Services[p8SvcKey]
	if !ok {
		client, err := d.createP8sClient(*promConfig)
		if err != nil {
			return func() {}, err
		}
		d.p8Services[p8SvcKey] = client
	}

	newCallback := callback{
		name:      types.NamespacedName{Name: countermeasure.Name, Namespace: countermeasure.Namespace},
		alertSpec: countermeasure.Spec.Prometheus.Alert.DeepCopy(),
		handler:   handler,
	}

	// the register the alert to the synchronized map
	if cb, ok := d.p8sToCallbacks[p8SvcKey]; !ok {
		d.p8sToCallbacks[p8SvcKey] = append(make([]callback, 0), newCallback)
	} else {
		d.p8sToCallbacks[p8SvcKey] = append(cb, newCallback)
	}

	nsName := cm.ToNamespaceName(&countermeasure.ObjectMeta)
	return cancelFunction(d, nsName), nil
}

func (d *Detector) Supports(countermeasure *v1alpha1.CounterMeasureSpec) bool {
	return countermeasure != nil && countermeasure.Prometheus != nil
}

// cancelFunction create a cancel function
func cancelFunction(d *Detector, key types.NamespacedName) func() {
	return func() {
		d.callbackMux.Lock()
		defer d.callbackMux.Unlock()

		for p8SvcKey, callbacks := range d.p8sToCallbacks {
			idx := -1
			for i, callback := range callbacks {
				if callback.name == key {
					idx = i
					break
				}
			}

			if idx != -1 {

				if len(d.p8sToCallbacks[p8SvcKey]) == 1 {
					delete(d.p8Services, p8SvcKey)
					delete(d.p8sToCallbacks, p8SvcKey)
				} else {
					d.p8sToCallbacks[p8SvcKey] = append(callbacks[:idx], callbacks[idx+1:]...)
				}
			}

		}
	}
}

// poll fetach alerts from each prometheus service and notify the callbacks on any active alerts
func (d *Detector) poll() {
	d.callbackMux.Lock()
	defer d.callbackMux.Unlock()

	for svc, callbacks := range d.p8sToCallbacks {
		p8 := d.p8Services[svc]
		alerts, err := p8.GetActiveAlerts()
		if err == nil {
			for _, cb := range callbacks {
				alertName := cb.alertSpec.AlertName
				pending := cb.alertSpec.IncludePending

				if alerts.IsAlertActive(alertName, pending) {
					labels, err := alerts.GetActiveAlertLabels(alertName, pending)
					if err != nil {
						d.logger.Error(err, "could not get active alert labels", "alertname", alertName)
					}
					go cb.handler.OnDetection(cb.name, labels)
				}
			}
		} else {
			d.logger.Info("failed to get alerts from prometheus service", "prometheus_service", svc)
		}
	}
}

func (d *Detector) createP8sClient(p8sService v1alpha1.PrometheusSpec) (*PrometheusService, error) {
	serviceObject := &corev1.Service{}
	if err := d.client.Get(context.Background(), p8sService.Service.GetNamespacedName(), serviceObject); err != nil {
		return nil, err
	}

	svc := p8sService.Service
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
	p8sClient, err := d.p8sBulider(address)
	if err != nil {
		return nil, err
	}
	return p8sClient, nil
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
