package controllers

import (
	"context"
	"time"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

func (r *ReconcilerBase) HandleSuccess(ctx context.Context, cm *v1alpha1.CounterMeasure) (ctrl.Result, error) {

	meta.SetStatusCondition(&cm.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.TypeMonitoring,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: cm.Generation,
		Status:             metav1.ConditionTrue,
		Reason:             v1alpha1.ReasonSucceeded,
	})

	cm.Status.LastStatus = v1alpha1.Monitoring
	cm.Status.LastStatusChangeTime = &metav1.Time{Time: time.Now()}

	log := log.FromContext(ctx)
	if err := r.GetClient().Status().Update(ctx, cm); err != nil {
		log.Error(err, "failed to update countermeasure status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ReconcilerBase) HandleError(ctx context.Context, cm *v1alpha1.CounterMeasure, err error) (ctrl.Result, error) {
	return r.HandleErrorAndRequeue(ctx, cm, err, 0)
}

func (r *ReconcilerBase) HandleErrorAndRequeue(ctx context.Context, cm *v1alpha1.CounterMeasure, err error, requeueAfter time.Duration) (ctrl.Result, error) {

	r.GetRecorder().Event(cm, "Warning", "ProcessingError", err.Error())
	meta.SetStatusCondition(&cm.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.TypeMonitoring,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: cm.Generation,
		Status:             metav1.ConditionTrue,
		Reason:             v1alpha1.ReasonSucceeded,
		Message:            err.Error(),
	})

	cm.Status.LastStatus = v1alpha1.Error
	cm.Status.LastStatusChangeTime = &metav1.Time{Time: time.Now()}

	log := log.FromContext(ctx)
	if err := r.GetClient().Status().Update(ctx, cm); err != nil {
		log.Error(err, "failed to update countermeasure status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *ReconcilerBase) HandleOutcome(ctx context.Context, cm *v1alpha1.CounterMeasure, err error) (ctrl.Result, error) {
	if err != nil {
		return r.HandleError(ctx, cm, err)
	}

	return r.HandleSuccess(ctx, cm)
}

// MarkInitializing mark this countermeasure with the transient initializing state
func (r *ReconcilerBase) MarkInitializing(ctx context.Context, cm *v1alpha1.CounterMeasure) error {
	log := log.FromContext(ctx)
	if cm.Status.Conditions == nil || len(cm.Status.Conditions) == 0 {
		meta.SetStatusCondition(&cm.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.TypeMonitoring,
			Status:             metav1.ConditionUnknown,
			ObservedGeneration: cm.Generation,
			Reason:             v1alpha1.ReasonReconciling,
			Message:            "Initializing",
		})

		cm.Status.LastStatus = v1alpha1.Unknown
		cm.Status.LastStatusChangeTime = &metav1.Time{Time: time.Now()}

		if err := r.GetClient().Status().Update(ctx, cm); err != nil {
			log.Error(err, "failed to update countermeasure status")
			return err
		}

		// Let's re-fetch the countermeasure Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		ns := types.NamespacedName{Namespace: cm.ObjectMeta.Namespace, Name: cm.ObjectMeta.Name}
		if err := r.client.Get(ctx, ns, cm); err != nil {
			log.Error(err, "failed to reload countermeasure")
			return err
		}
	}

	return nil
}

// GetClient returns the OperatorSDK client
func (r *ReconcilerBase) GetClient() client.Client {
	return r.client
}

// GetClient returns the K8s event recorder for the custom resource
func (r *ReconcilerBase) GetRecorder() record.EventRecorder {
	return r.recorder
}

func (r *ReconcilerBase) GetRestConfig() *rest.Config {
	return r.restConfig
}
