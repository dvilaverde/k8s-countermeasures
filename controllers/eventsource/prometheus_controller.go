/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package eventsource

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/eventsource/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/operator/reconciler"
	"github.com/dvilaverde/k8s-countermeasures/operator/sources"
)

// PrometheusReconciler reconciles a Prometheus object
type PrometheusReconciler struct {
	reconciler.ReconcilerBase
	eventManager sources.EventManager
}

//+kubebuilder:rbac:groups=eventsource.vilaverde.rocks,resources=prometheuses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=eventsource.vilaverde.rocks,resources=prometheuses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=eventsource.vilaverde.rocks,resources=prometheuses/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *PrometheusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logr := log.FromContext(ctx)

	eventSourceCR := &v1alpha1.Prometheus{}
	err := r.GetClient().Get(ctx, req.NamespacedName, eventSourceCR)
	if err != nil {
		if errors.IsNotFound(err) {
			logr.Info("Prometheus event source resource not found", "name", req.Name, "namespace", req.Namespace)

			// Notify the monitoring service to stop monitoring the NamespaceName
			err := r.eventManager.Remove(req.NamespacedName)
			return ctrl.Result{}, err
		}

		// could not read the Prometheus resource, throw error, so it can be re-queued.
		logr.Error(err, "Error getting Prometheus event source resource object")
		return ctrl.Result{}, err
	}

	// check for the existence of the event source, in case it's already added and running
	// there is no need to re-install. This handles re-queues due to status changes.
	installed, err := r.eventManager.Exists(eventSourceCR)
	if !installed {
		// install now that we've determined this event source needs to be added
		err = r.eventManager.Add(eventSourceCR)
	}

	return r.HandleOutcome(ctx, eventSourceCR, err)
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrometheusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Prometheus{}).
		Complete(r)
}

func (r *PrometheusReconciler) HandleSuccess(ctx context.Context, es *v1alpha1.Prometheus) (ctrl.Result, error) {

	logger := log.FromContext(ctx)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// Let's re-fetch the prometheus Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster, and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		ns := types.NamespacedName{Namespace: es.ObjectMeta.Namespace, Name: es.ObjectMeta.Name}
		if err := r.GetClient().Get(ctx, ns, es); err != nil {
			logger.Error(err, "failed to reload prometheus event source")
			return err
		}

		meta.SetStatusCondition(&es.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.TypePolling,
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: es.Generation,
			Status:             metav1.ConditionTrue,
			Reason:             v1alpha1.ReasonSucceeded,
		})

		es.Status.State = v1alpha1.Polling
		return r.GetClient().Status().Update(ctx, es)
	})

	if err != nil {
		if errors.IsConflict(err) {
			logger.Info("409 conflict - failed to update prometheus event source status, reconcile re-queued.")
		} else {
			logger.Error(err, "failed to update prometheus event source status")
		}
	}

	return ctrl.Result{}, err
}

func (r *PrometheusReconciler) HandleError(ctx context.Context, es *v1alpha1.Prometheus, err error) (ctrl.Result, error) {
	return r.HandleErrorAndRequeue(ctx, es, err, 0)
}

func (r *PrometheusReconciler) HandleErrorAndRequeue(ctx context.Context, es *v1alpha1.Prometheus, err error, requeueAfter time.Duration) (ctrl.Result, error) {

	r.GetRecorder().Event(es, "Warning", "ProcessingError", err.Error())

	logger := log.FromContext(ctx)
	retryErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// Let's re-fetch the prometheus Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster, and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		ns := types.NamespacedName{Namespace: es.ObjectMeta.Namespace, Name: es.ObjectMeta.Name}
		if err := r.GetClient().Get(ctx, ns, es); err != nil {
			logger.Error(err, "failed to reload prometheus event source")
			return err
		}

		meta.SetStatusCondition(&es.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.TypePolling,
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: es.Generation,
			Status:             metav1.ConditionTrue,
			Reason:             v1alpha1.ReasonSucceeded,
			Message:            err.Error(),
		})

		es.Status.State = v1alpha1.Error
		return r.GetClient().Status().Update(ctx, es)
	})

	if retryErr != nil {
		logger.Error(retryErr, "failed to update prometheus status")
		return ctrl.Result{}, retryErr
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *PrometheusReconciler) HandleOutcome(ctx context.Context, es *v1alpha1.Prometheus, err error) (ctrl.Result, error) {
	if err != nil {
		return r.HandleError(ctx, es, err)
	}

	return r.HandleSuccess(ctx, es)
}

// MarkInitializing mark this event source with the transient initializing state
func (r *PrometheusReconciler) MarkInitializing(ctx context.Context, es *v1alpha1.Prometheus) error {
	logger := log.FromContext(ctx)
	var err error

	if es.Status.Conditions == nil || len(es.Status.Conditions) == 0 {

		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			// Let's re-fetch the prometheus Custom Resource after update the status
			// so that we have the latest state of the resource on the cluster, and we will avoid
			// raise the issue "the object has been modified, please apply
			// your changes to the latest version and try again" which would re-trigger the reconciliation
			// if we try to update it again in the following operations
			ns := types.NamespacedName{Namespace: es.ObjectMeta.Namespace, Name: es.ObjectMeta.Name}
			if err := r.GetClient().Get(ctx, ns, es); err != nil {
				logger.Error(err, "failed to reload prometheus")
				return err
			}

			meta.SetStatusCondition(&es.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.TypePolling,
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: es.Generation,
				Reason:             v1alpha1.ReasonReconciling,
				Message:            "Initializing",
			})

			es.Status.State = v1alpha1.Unknown
			return r.GetClient().Status().Update(ctx, es)
		})

		if err != nil {
			logger.Error(err, "failed to update Prometheus eventsource status")
		}
	}

	return err
}
