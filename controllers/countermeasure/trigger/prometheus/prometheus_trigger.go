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

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	cm "github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure"
	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/trigger"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type callback struct {
	name      types.NamespacedName
	alertSpec *v1alpha1.PrometheusAlertSpec
	handler   trigger.Handler
}

type Trigger struct {
	logger     logr.Logger
	client     client.Client
	p8sBuilder Builder

	interval time.Duration

	callbackMux    sync.Mutex
	p8Services     map[string]*PrometheusService
	p8sToCallbacks map[string][]callback
}

func NewTrigger(p8ServiceBuilder Builder, interval time.Duration) *Trigger {
	return &Trigger{
		interval:   interval,
		p8sBuilder: p8ServiceBuilder,
	}
}

func (d *Trigger) Start(ctx context.Context) error {
	logger := log.FromContext(ctx)

	d.p8Services = make(map[string]*PrometheusService)
	d.p8sToCallbacks = make(map[string][]callback)

	go utilwait.Until(d.poll, d.interval, ctx.Done())

	logger.Info("starting prometheus alert polling")

	return nil
}

// InjectLogger injectable logger
// https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/runtime/inject/inject.go
func (d *Trigger) InjectLogger(logr logr.Logger) error {
	d.logger = logr
	return nil
}

// InjectClient injectable client
// https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/runtime/inject/inject.go
func (d *Trigger) InjectClient(client client.Client) error {
	d.client = client
	return nil
}

func (d *Trigger) NotifyOn(countermeasure v1alpha1.CounterMeasure, handler trigger.Handler) (trigger.CancelFunc, error) {
	promConfig := countermeasure.Spec.Prometheus

	p8SvcKey := cm.ServiceToKey(promConfig.Service)

	d.callbackMux.Lock()
	defer d.callbackMux.Unlock()

	newCallback := callback{
		name:      types.NamespacedName{Name: countermeasure.Name, Namespace: countermeasure.Namespace},
		alertSpec: countermeasure.Spec.Prometheus.Alert.DeepCopy(),
		handler:   handler,
	}

	// the register the alert to the synchronized map
	if _, ok := d.p8sToCallbacks[p8SvcKey]; !ok {
		d.p8sToCallbacks[p8SvcKey] = append(make([]callback, 0), newCallback)
	} else {
		// if a callback with this name already exists it needs to be removed first
		d.deleteCallbackByName(p8SvcKey, newCallback.name)
		d.p8sToCallbacks[p8SvcKey] = append(d.p8sToCallbacks[p8SvcKey], newCallback)
	}

	// use the promConfig to lookup the service
	_, ok := d.p8Services[p8SvcKey]
	if !ok {
		client, err := d.createP8sClient(*promConfig)
		if err != nil {
			return func() {}, err
		}
		d.p8Services[p8SvcKey] = client
	}

	nsName := cm.ToNamespaceName(&countermeasure.ObjectMeta)
	return d.cancelFunction(nsName), nil
}

func (d *Trigger) Supports(countermeasure *v1alpha1.CounterMeasureSpec) bool {
	return countermeasure != nil && countermeasure.Prometheus != nil
}

// cancelFunction create a cancel function
func (d *Trigger) cancelFunction(key types.NamespacedName) func() {
	return func() {
		d.callbackMux.Lock()
		defer d.callbackMux.Unlock()

		for p8SvcKey := range d.p8sToCallbacks {
			d.deleteCallbackByName(p8SvcKey, key)
		}
	}
}

// deleteCallbackByName delete a callback by callback name
func (d *Trigger) deleteCallbackByName(p8sServiceKey string, name types.NamespacedName) {
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
func (d *Trigger) poll() {
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
			d.logger.Error(err, "failed to get alerts from prometheus service", "prometheus_service", svc)
		}
	}
}

func (d *Trigger) createP8sClient(p8sService v1alpha1.PrometheusSpec) (*PrometheusService, error) {
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

	var username, password string
	if p8sService.Auth != nil {
		auth := p8sService.Auth
		if secret, err := d.getSecret(auth); err != nil {
			username = string(secret.Data["username"])
			password = string(secret.Data["password"])
		} else {
			d.logger.Error(err, "could not lookup secret %s in namespace %s", auth.Name, auth.Namespace)
		}
	}

	p8sClient, err := d.p8sBuilder(address, username, password)
	if err != nil {
		return nil, err
	}
	return p8sClient, nil
}

func (d *Trigger) getSecret(ref *corev1.SecretReference) (corev1.Secret, error) {
	secret := corev1.Secret{}

	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	err := d.client.Get(context.Background(), key, &secret)
	if err != nil {
		return corev1.Secret{}, nil
	}

	// TODO: support auth TLS using secret ref
	if secret.Type != corev1.SecretTypeBasicAuth {
		return corev1.Secret{}, errors.New("only the basic auth type (kubernetes.io/basic-auth) is currently supported")
	}

	return secret, nil
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
