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

package countermeasure

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/pkg/manager"
	"github.com/dvilaverde/k8s-countermeasures/pkg/reconciler"
	"k8s.io/apimachinery/pkg/api/meta"
)

// CounterMeasureReconciler reconciles a CounterMeasure object
type CounterMeasureReconciler struct {
	reconciler.ReconcilerBase
	ConsumerManager manager.Manager[*v1alpha1.CounterMeasure]
	Log             logr.Logger
}

// Refer to the following URL for the K8s API groups:
// https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.21/#-strong-api-groups-strong-
//
//+kubebuilder:rbac:groups=countermeasure.vilaverde.rocks,resources=countermeasures,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=countermeasure.vilaverde.rocks,resources=countermeasures/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=countermeasure.vilaverde.rocks,resources=countermeasures/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=*,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=autoscaling,resources=*,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=*,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=*,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=*,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=*,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CounterMeasure object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *CounterMeasureReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	counterMeasureCR := &v1alpha1.CounterMeasure{}
	err := r.GetClient().Get(ctx, req.NamespacedName, counterMeasureCR)
	if err != nil {
		if errors.IsNotFound(err) {
			// stop reconciliation since the Operator Custom Resource was not found
			logger.Info("CounterMeasure resource not found", "name", req.Name, "namespace", req.Namespace)
			// Notify the monitoring service to stop monitoring the NamespaceName
			r.ConsumerManager.Remove(req.NamespacedName)
			return ctrl.Result{}, nil
		}

		// could not read the CounterMeasure resource, throw error, so it can be re-queued.
		logger.Error(err, "Error getting CounterMeasure resource object")
		return ctrl.Result{}, err
	}

	if r.ConsumerManager.Exists(counterMeasureCR.ObjectMeta) {
		return r.HandleSuccess(ctx, counterMeasureCR.ObjectMeta)
	}

	logger.Info("Reconciling CounterMeasure", "name", req.Name, "namespace", req.Namespace)

	// Let's just set the status as Unknown when no status are available
	err = r.MarkInitializing(ctx, counterMeasureCR)
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Validating counter measure spec", "name", req.Name, "namespace", req.Namespace)

	if ok, err := r.isValid(ctx, counterMeasureCR); !ok {
		return r.HandleError(ctx, counterMeasureCR.ObjectMeta, err)
	}

	err = r.ConsumerManager.Add(counterMeasureCR)
	if err != nil {
		return r.HandleError(ctx, counterMeasureCR.ObjectMeta, err)
	}

	return r.HandleSuccess(ctx, counterMeasureCR.ObjectMeta)
}

func (r *CounterMeasureReconciler) isValid(ctx context.Context, cm *v1alpha1.CounterMeasure) (bool, error) {
	err := v1alpha1.ValidateSpec(&cm.Spec)
	return err == nil, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *CounterMeasureReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.OnError = r.HandleErrorAndRequeue
	r.OnSuccess = r.HandleSuccess

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.CounterMeasure{}).
		Complete(r)
}

func (r *CounterMeasureReconciler) HandleSuccess(ctx context.Context, objectMeta metav1.ObjectMeta) (ctrl.Result, error) {

	logger := log.FromContext(ctx)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		cm := &v1alpha1.CounterMeasure{}
		// Let's re-fetch the countermeasure Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster, and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		ns := types.NamespacedName{Namespace: objectMeta.Namespace, Name: objectMeta.Name}
		if err := r.GetClient().Get(ctx, ns, cm); err != nil {
			logger.Error(err, "failed to reload countermeasure")
			return err
		}

		meta.SetStatusCondition(&cm.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.TypeMonitoring,
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: objectMeta.Generation,
			Status:             metav1.ConditionTrue,
			Reason:             v1alpha1.ReasonSucceeded,
		})

		cm.Status.LastStatus = v1alpha1.Monitoring
		cm.Status.LastStatusChangeTime = &metav1.Time{Time: time.Now()}

		return r.GetClient().Status().Update(ctx, cm)
	})

	if err != nil {
		if errors.IsConflict(err) {
			logger.Info("409 conflict - failed to update countermeasure status, reconcile re-queued.")
		} else {
			logger.Error(err, "failed to update countermeasure status")
		}
	}

	return ctrl.Result{}, err
}

func (r *CounterMeasureReconciler) HandleErrorAndRequeue(ctx context.Context, objectMeta metav1.ObjectMeta, err error, requeueAfter time.Duration) (ctrl.Result, error) {

	logger := log.FromContext(ctx)
	retryErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		cm := &v1alpha1.CounterMeasure{}
		// Let's re-fetch the countermeasure Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster, and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		ns := types.NamespacedName{Namespace: objectMeta.Namespace, Name: objectMeta.Name}
		if err := r.GetClient().Get(ctx, ns, cm); err != nil {
			logger.Error(err, "failed to reload countermeasure")
			return err
		}

		meta.SetStatusCondition(&cm.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.TypeMonitoring,
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: objectMeta.Generation,
			Status:             metav1.ConditionTrue,
			Reason:             v1alpha1.ReasonSucceeded,
			Message:            err.Error(),
		})

		cm.Status.LastStatus = v1alpha1.Error
		cm.Status.LastStatusChangeTime = &metav1.Time{Time: time.Now()}

		err = r.GetClient().Status().Update(ctx, cm)
		if err != nil {
			r.GetRecorder().Event(cm, "Warning", "ProcessingError", err.Error())
		}
		return err
	})

	if retryErr != nil {
		logger.Error(retryErr, "failed to update countermeasure status")
		return ctrl.Result{}, retryErr
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// MarkInitializing mark this countermeasure with the transient initializing state
func (r *CounterMeasureReconciler) MarkInitializing(ctx context.Context, cm *v1alpha1.CounterMeasure) error {
	logger := log.FromContext(ctx)
	var err error

	if cm.Status.Conditions == nil || len(cm.Status.Conditions) == 0 {

		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			// Let's re-fetch the countermeasure Custom Resource after update the status
			// so that we have the latest state of the resource on the cluster, and we will avoid
			// raise the issue "the object has been modified, please apply
			// your changes to the latest version and try again" which would re-trigger the reconciliation
			// if we try to update it again in the following operations
			ns := types.NamespacedName{Namespace: cm.ObjectMeta.Namespace, Name: cm.ObjectMeta.Name}
			if err := r.GetClient().Get(ctx, ns, cm); err != nil {
				logger.Error(err, "failed to reload countermeasure")
				return err
			}

			meta.SetStatusCondition(&cm.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.TypeMonitoring,
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: cm.Generation,
				Reason:             v1alpha1.ReasonReconciling,
				Message:            "Initializing",
			})

			cm.Status.LastStatus = v1alpha1.Unknown
			cm.Status.LastStatusChangeTime = &metav1.Time{Time: time.Now()}
			return r.GetClient().Status().Update(ctx, cm)
		})

		if err != nil {
			logger.Error(err, "failed to update countermeasure status")
		}
	}

	return err
}
