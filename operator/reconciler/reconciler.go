package reconciler

import (
	"k8s.io/apimachinery/pkg/runtime"
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
