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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/eventsource/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/pkg/manager"
	"github.com/dvilaverde/k8s-countermeasures/pkg/reconciler"
	"github.com/dvilaverde/k8s-countermeasures/pkg/sources"
	"github.com/dvilaverde/k8s-countermeasures/pkg/sources/prometheus"
)

// PrometheusReconciler reconciles a Prometheus object
type PrometheusReconciler struct {
	reconciler.ReconcilerBase
	SourceManager manager.Manager[sources.EventSource]
	Log           logr.Logger
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

	var (
		err           error
		logr          = log.FromContext(ctx)
		eventSourceCR = &v1alpha1.Prometheus{}
		sourceManager = r.SourceManager
	)

	err = r.GetClient().Get(ctx, req.NamespacedName, eventSourceCR)
	if err != nil {
		if errors.IsNotFound(err) {
			logr.Info("Prometheus event source resource not found", "name", req.Name, "namespace", req.Namespace)

			// Notify the monitoring service to stop monitoring the NamespaceName
			err := sourceManager.Remove(req.NamespacedName)
			return ctrl.Result{}, err
		}

		// could not read the Prometheus resource, throw error, so it can be re-queued.
		logr.Error(err, "Error getting Prometheus event source resource object")
		return ctrl.Result{}, err
	}

	// check for the existence of the event source, in case it's already added and running
	// there is no need to re-install. This handles re-queues due to status changes.
	installed := sourceManager.Exists(eventSourceCR.ObjectMeta)
	if !installed {
		// install now that we've determined this event source needs to be added
		client, err := r.createP8sClient(eventSourceCR)
		if err != nil {
			return r.HandleErrorAndRequeue(ctx, eventSourceCR.ObjectMeta, err, time.Duration(30*time.Second))
		}

		err = sourceManager.Add(prometheus.NewEventSource(eventSourceCR, client))
		return r.HandleOutcome(ctx, eventSourceCR.ObjectMeta, err)
	}

	return r.HandleSuccess(ctx, eventSourceCR.ObjectMeta)
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrometheusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.OnError = r.HandleErrorAndRequeue
	r.OnSuccess = r.HandleSuccess

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Prometheus{}).
		Complete(r)
}

func (r *PrometheusReconciler) HandleSuccess(ctx context.Context, objectMeta metav1.ObjectMeta) (ctrl.Result, error) {

	logger := log.FromContext(ctx)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		eventSourceCR := &v1alpha1.Prometheus{}
		// Let's re-fetch the prometheus Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster, and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		ns := types.NamespacedName{Namespace: objectMeta.Namespace, Name: objectMeta.Name}
		if err := r.GetClient().Get(ctx, ns, eventSourceCR); err != nil {
			logger.Error(err, "failed to reload prometheus event source")
			return err
		}

		meta.SetStatusCondition(&eventSourceCR.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.TypePolling,
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: objectMeta.Generation,
			Status:             metav1.ConditionTrue,
			Reason:             v1alpha1.ReasonSucceeded,
		})

		eventSourceCR.Status.State = v1alpha1.Polling
		return r.GetClient().Status().Update(ctx, eventSourceCR)
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

func (r *PrometheusReconciler) HandleErrorAndRequeue(ctx context.Context, objectMeta metav1.ObjectMeta, err error, requeueAfter time.Duration) (ctrl.Result, error) {

	logger := log.FromContext(ctx)
	retryErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		eventSourceCR := &v1alpha1.Prometheus{}
		// Let's re-fetch the prometheus Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster, and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		ns := types.NamespacedName{Namespace: objectMeta.Namespace, Name: objectMeta.Name}
		if err := r.GetClient().Get(ctx, ns, eventSourceCR); err != nil {
			logger.Error(err, "failed to reload prometheus event source")
			return err
		}

		meta.SetStatusCondition(&eventSourceCR.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.TypePolling,
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: objectMeta.Generation,
			Status:             metav1.ConditionTrue,
			Reason:             v1alpha1.ReasonSucceeded,
			Message:            err.Error(),
		})

		eventSourceCR.Status.State = v1alpha1.Error
		err = r.GetClient().Status().Update(ctx, eventSourceCR)
		if err == nil {
			r.GetRecorder().Event(eventSourceCR, "Warning", "ProcessingError", err.Error())
		}

		return err
	})

	if retryErr != nil {
		logger.Error(retryErr, "failed to update prometheus status")
		return ctrl.Result{}, retryErr
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
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

func (r *PrometheusReconciler) createP8sClient(prom *v1alpha1.Prometheus) (*prometheus.PrometheusService, error) {
	promConfig := prom.Spec
	svc := promConfig.Service

	serviceObject := &corev1.Service{}
	if err := r.GetClient().Get(context.Background(), svc.GetNamespacedName(), serviceObject); err != nil {
		return nil, err
	}

	svcPort, found := reconciler.FindNamedPort(serviceObject, svc.TargetPort)
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
			secretRef.Namespace = prom.ObjectMeta.Namespace
		}
		secret, err := r.GetSecret(secretRef)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("could not lookup secret %s in namespace %s", secretRef.Name, secretRef.Namespace))
			return nil, err
		}

		username = string(secret.Data["username"])
		password = string(secret.Data["password"])
	}

	return prometheus.NewPrometheusClient(address, username, password)
}
