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
	util "github.com/dvilaverde/k8s-countermeasures/operator"
	"github.com/dvilaverde/k8s-countermeasures/operator/actions"
	"github.com/dvilaverde/k8s-countermeasures/operator/reconciler"
	"github.com/dvilaverde/k8s-countermeasures/operator/sources"
	"k8s.io/apimachinery/pkg/api/meta"
)

type counterMeasureHandle struct {
	cancelFunc sources.CancelFunc
	generation int64
}

// CounterMeasureReconciler reconciles a CounterMeasure object
type CounterMeasureReconciler struct {
	reconciler.ReconcilerBase
	EventSources   []sources.Source
	actionRegistry actions.Registry
	monitored      map[string]counterMeasureHandle
	Log            logr.Logger
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
			r.stopMonitoring(req.NamespacedName)
			return ctrl.Result{}, nil
		}

		// could not read the CounterMeasure resource, throw error, so it can be re-queued.
		logger.Error(err, "Error getting CounterMeasure resource object")
		return ctrl.Result{}, err
	}

	if r.isAlreadyMonitored(counterMeasureCR) {
		return r.HandleSuccess(ctx, counterMeasureCR)
	}

	logger.Info("Reconciling CounterMeasure", "name", req.Name, "namespace", req.Namespace)

	// Let's just set the status as Unknown when no status are available
	err = r.MarkInitializing(ctx, counterMeasureCR)
	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO: if a Resource is created, make sure this is called so there will be an owner relationship
	// ctrl.SetControllerReference(operatorCR, newResource, r.Scheme)
	logger.Info("Validating counter measure spec", "name", req.Name, "namespace", req.Namespace)

	if ok, err := r.isValid(ctx, counterMeasureCR); !ok {
		return r.HandleError(ctx, counterMeasureCR, err)
	}

	err = r.startMonitoring(counterMeasureCR)
	if err != nil {
		return r.HandleError(ctx, counterMeasureCR, err)
	}

	return r.HandleSuccess(ctx, counterMeasureCR)
}

func (r *CounterMeasureReconciler) isValid(ctx context.Context, cm *v1alpha1.CounterMeasure) (bool, error) {
	err := v1alpha1.ValidateSpec(&cm.Spec)
	return err == nil, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *CounterMeasureReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.monitored = make(map[string]counterMeasureHandle)
	r.actionRegistry = actions.Registry{}
	r.actionRegistry.Initialize()

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.CounterMeasure{}).
		Complete(r)
}

func (r *CounterMeasureReconciler) isAlreadyMonitored(cm *v1alpha1.CounterMeasure) bool {

	nsName := util.ToNamespaceName(&cm.ObjectMeta)
	// if the generation hasn't changed from what we're monitoring then short return
	if handle, ok := r.monitored[nsName.String()]; ok {
		if handle.generation == cm.Generation {
			return true
		}
	}

	return false
}

// StartMonitoring will start monitoring a resource for events that require action
func (r *CounterMeasureReconciler) startMonitoring(countermeasure *v1alpha1.CounterMeasure) error {
	found := false
	nsName := util.ToNamespaceName(&countermeasure.ObjectMeta)

	for _, source := range r.EventSources {
		if source.Supports(&countermeasure.Spec) {

			handler, err := r.actionRegistry.ConvertToHandler(countermeasure, r)
			if err != nil {
				return err
			}

			cancel, err := source.NotifyOn(*countermeasure, handler)
			if err != nil {
				return err
			}

			found = true
			r.monitored[nsName.String()] = counterMeasureHandle{
				cancelFunc: cancel,
				generation: countermeasure.Generation,
			}

			break
		}
	}

	if !found {
		r.Log.Error(nil, "could not find a supported countermeasure event source")
	}

	return nil
}

func (r *CounterMeasureReconciler) stopMonitoring(key types.NamespacedName) error {

	if handle, ok := r.monitored[key.String()]; ok {
		handle.cancelFunc()
		// delete the key from this monitored map
		delete(r.monitored, key.String())

		r.Log.Info("stopped monitoring countermeasure", "name", key.Name, "namespace", key.Namespace)
	}

	return nil
}

func (r *CounterMeasureReconciler) HandleSuccess(ctx context.Context, cm *v1alpha1.CounterMeasure) (ctrl.Result, error) {

	logger := log.FromContext(ctx)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
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
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: cm.Generation,
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

func (r *CounterMeasureReconciler) HandleError(ctx context.Context, cm *v1alpha1.CounterMeasure, err error) (ctrl.Result, error) {
	return r.HandleErrorAndRequeue(ctx, cm, err, 0)
}

func (r *CounterMeasureReconciler) HandleErrorAndRequeue(ctx context.Context, cm *v1alpha1.CounterMeasure, err error, requeueAfter time.Duration) (ctrl.Result, error) {

	r.GetRecorder().Event(cm, "Warning", "ProcessingError", err.Error())

	logger := log.FromContext(ctx)
	retryErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
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
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: cm.Generation,
			Status:             metav1.ConditionTrue,
			Reason:             v1alpha1.ReasonSucceeded,
			Message:            err.Error(),
		})

		cm.Status.LastStatus = v1alpha1.Error
		cm.Status.LastStatusChangeTime = &metav1.Time{Time: time.Now()}
		return r.GetClient().Status().Update(ctx, cm)
	})

	if retryErr != nil {
		logger.Error(retryErr, "failed to update countermeasure status")
		return ctrl.Result{}, retryErr
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *CounterMeasureReconciler) HandleOutcome(ctx context.Context, cm *v1alpha1.CounterMeasure, err error) (ctrl.Result, error) {
	if err != nil {
		return r.HandleError(ctx, cm, err)
	}

	return r.HandleSuccess(ctx, cm)
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
