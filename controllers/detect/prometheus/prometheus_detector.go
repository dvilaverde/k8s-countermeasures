package prometheus

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/controllers/detect"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type callback struct {
	name      types.NamespacedName
	alertSpec *v1alpha1.PrometheusAlertSpec
	onDetect  detect.DetectedFunc
}

type Detector struct {
	logger     logr.Logger
	client     client.Client
	p8sBulider Builder

	interval time.Duration

	callbackMux sync.Mutex
	p8Services  map[string]*PrometheusService
	callbacks   map[string][]callback
}

func NewDetector(p8ServiceBuilder Builder, interval time.Duration) *Detector {
	return &Detector{
		interval:   interval,
		p8sBulider: p8ServiceBuilder,
	}
}

func (d *Detector) Start(ctx context.Context) error {
	// TODO: make the ticker configurable

	d.p8Services = make(map[string]*PrometheusService)
	d.callbacks = make(map[string][]callback)

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

func (d *Detector) NotifyOn(countermeasure v1alpha1.CounterMeasure, onDetect detect.DetectedFunc) error {
	promConfig := countermeasure.Spec.Prometheus

	key := promConfig.Service.Namespace + "/" + promConfig.Service.Name

	d.callbackMux.Lock()
	defer d.callbackMux.Unlock()

	// use the promConfig to lookup the service
	_, ok := d.p8Services[key]
	if !ok {
		client, err := d.createP8sClient(*promConfig)
		if err != nil {
			return err
		}
		d.p8Services[key] = client
	}

	newCallback := callback{
		name:      types.NamespacedName{Name: countermeasure.Name, Namespace: countermeasure.Namespace},
		alertSpec: countermeasure.Spec.Prometheus.Alert.DeepCopy(),
		onDetect:  onDetect,
	}

	// the register the alert to the synchronized map
	if cb, ok := d.callbacks[key]; !ok {
		d.callbacks[key] = append(make([]callback, 0), newCallback)
	} else {
		d.callbacks[key] = append(cb, newCallback)
	}

	return nil
}

func (d *Detector) Supports(countermeasure *v1alpha1.CounterMeasureSpec) bool {
	return countermeasure != nil && countermeasure.Prometheus != nil
}

// poll fetach alerts from each prometheus service and notify the callbacks on any active alerts
func (d *Detector) poll() {
	d.callbackMux.Lock()
	defer d.callbackMux.Unlock()

	for svc, callbacks := range d.callbacks {
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
					cb.onDetect(cb.name, labels)
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

	svcPort, found := findNamedPort(serviceObject, p8sService.Service.TargetPort)
	var port int32
	if found {
		port = svcPort.Port
	} else {
		port = p8sService.Service.Port
	}

	address := fmt.Sprintf("https://%v.%v.svc:%v", p8sService.Service.Name, p8sService.Service.Namespace, port)
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
