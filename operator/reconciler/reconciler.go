package reconciler

import (
	"strings"

	"github.com/dvilaverde/k8s-countermeasures/apis/eventsource/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type ReconcilerBase struct {
	apireader  client.Reader
	client     client.Client
	scheme     *runtime.Scheme
	restConfig *rest.Config
	recorder   record.EventRecorder
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
func NewFromManager(mgr manager.Manager, recorder record.EventRecorder) ReconcilerBase {
	return NewReconcilerBase(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), recorder, mgr.GetAPIReader())
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
