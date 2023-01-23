package reconciler

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/dvilaverde/k8s-countermeasures/apis/eventsource/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type HandleErrorFunc func(context.Context, metav1.ObjectMeta, error, time.Duration) (ctrl.Result, error)
type HandleSuccessFunc func(context.Context, metav1.ObjectMeta) (ctrl.Result, error)

type ReconcilerBase struct {
	apireader  client.Reader
	client     client.Client
	scheme     *runtime.Scheme
	restConfig *rest.Config
	recorder   record.EventRecorder
	OnError    HandleErrorFunc
	OnSuccess  HandleSuccessFunc
}

func NewReconcilerBase(client client.Client, scheme *runtime.Scheme, restConfig *rest.Config, recorder record.EventRecorder, apireader client.Reader) ReconcilerBase {
	return ReconcilerBase{
		apireader:  apireader,
		client:     client,
		scheme:     scheme,
		restConfig: restConfig,
		recorder:   recorder,
	}
}

// NewFromManager creates a new ReconcilerBase from a Manager
func NewFromManager(mgr manager.Manager) ReconcilerBase {
	recorder := mgr.GetEventRecorderFor("countermeasure_controller")
	return NewReconcilerBase(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), recorder, mgr.GetAPIReader())
}

func (r *ReconcilerBase) HandleOutcome(ctx context.Context, objectMeta metav1.ObjectMeta, err error) (ctrl.Result, error) {
	if err != nil {
		return r.HandleError(ctx, objectMeta, err)
	}

	return r.OnSuccess(ctx, objectMeta)
}

func (r *ReconcilerBase) HandleError(ctx context.Context, objectMeta metav1.ObjectMeta, err error) (ctrl.Result, error) {
	return r.OnError(ctx, objectMeta, err, 0)
}

func (d *ReconcilerBase) GetSecret(ref *corev1.SecretReference) (corev1.Secret, error) {
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

// GetClient returns the OperatorSDK client
func (r *ReconcilerBase) GetClient() client.Client {
	return r.client
}

// GetRecorder returns the K8s event recorder for the custom resource
func (r *ReconcilerBase) GetRecorder() record.EventRecorder {
	return r.recorder
}

// GetRestConfig returns the rest config for the k8s client
func (r *ReconcilerBase) GetRestConfig() *rest.Config {
	return r.restConfig
}

func ServiceToKey(svcRef v1alpha1.ServiceReference) string {
	if svcRef.Namespace == "" {
		return svcRef.Name
	}

	return svcRef.Namespace + "/" + svcRef.Name
}

func ToNamespaceName(objectMeta *metav1.ObjectMeta) types.NamespacedName {
	return types.NamespacedName{
		Namespace: objectMeta.Namespace,
		Name:      objectMeta.Name,
	}
}

// ToKey will return a name from a ObjectMeta in the form of Namespace/Name. If
// no namespace is present then just Name.
func ToKey(objectMeta *metav1.ObjectMeta) string {
	if objectMeta.Namespace == "" {
		return objectMeta.Name
	}

	return objectMeta.Namespace + "/" + objectMeta.Name
}

// SplitKey will split the Key into a NamespaceName
func SplitKey(key string) types.NamespacedName {
	if strings.Contains(key, "/") {
		split := strings.Split(key, "/")
		return types.NamespacedName{
			Namespace: split[0],
			Name:      split[1],
		}
	}
	return types.NamespacedName{Name: key}
}

func FindNamedPort(service *corev1.Service, namedPort string) (corev1.ServicePort, bool) {
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
