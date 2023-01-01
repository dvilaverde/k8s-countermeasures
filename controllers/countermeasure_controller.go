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

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	monv1 "github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure"
)

// CounterMeasureReconciler reconciles a CounterMeasure object
type CounterMeasureReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Monitor *monv1.CounterMeasureMonitor
	Log     logr.Logger
}

// NewCounterMeasureReconciler create a new reconciler
func NewCounterMeasureReconciler(monitor *monv1.CounterMeasureMonitor,
	client client.Client,
	scheme *runtime.Scheme,
	log logr.Logger) *CounterMeasureReconciler {
	return &CounterMeasureReconciler{
		Client:  client,
		Scheme:  scheme,
		Monitor: monitor,
		Log:     log,
	}
}

// Refer to the following URL for the K8s API groups:
// https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.21/#-strong-api-groups-strong-
//
//+kubebuilder:rbac:groups=operator.vilaverde.rocks,resources=countermeasures,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.vilaverde.rocks,resources=countermeasures/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.vilaverde.rocks,resources=countermeasures/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list

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
	err := r.Get(ctx, req.NamespacedName, counterMeasureCR)
	if err != nil {
		if errors.IsNotFound(err) {
			// stop reconciliation since the Operator Custom Resource was not found
			logger.Info("CounterMeasure resource not found", "name", req.Name, "namespace", req.Namespace)
			// Notify the monitoring service to stop monitoring the NamespaceName
			r.Monitor.StopMonitoring(req.NamespacedName)
			return ctrl.Result{}, nil
		}

		// could not read the CounterMeasure resource, throw error so it can be requeued.
		logger.Error(err, "Error getting CounterMeasure resource object")
		return ctrl.Result{}, err
	}

	if r.Monitor.IsAlreadyMonitored(counterMeasureCR) {
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling CounterMeasure", "name", req.Name, "namespace", req.Namespace)

	// Let's just set the status as Unknown when no status are available
	if counterMeasureCR.Status.Conditions == nil || len(counterMeasureCR.Status.Conditions) == 0 {
		meta.SetStatusCondition(&counterMeasureCR.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.TypeMonitoring,
			Status:  metav1.ConditionUnknown,
			Reason:  v1alpha1.ReasonReconciling,
			Message: "Starting reconciliation",
		})

		if err = r.Status().Update(ctx, counterMeasureCR); err != nil {
			logger.Error(err, "failed to update countermeasure status")
			return ctrl.Result{}, err
		}

		// Let's re-fetch the countermeasure Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		if err := r.Get(ctx, req.NamespacedName, counterMeasureCR); err != nil {
			logger.Error(err, "failed to reload countermeasure")
			return ctrl.Result{}, err
		}
	}

	if counterMeasureCR.GetDeletionTimestamp() != nil {
		// TODO: handle deletion
		return ctrl.Result{}, nil
	} else {
		// TODO: if a Resource is created, make sure this is called so there will be an owner relationship
		// ctrl.SetControllerReference(operatorCR, newResource, r.Scheme)
		logger.Info("Validating counter measure spec", "name", req.Name, "namespace", req.Namespace)

		valid, err := r.isValid(ctx, counterMeasureCR)
		if err == nil && valid {
			err = r.Monitor.StartMonitoring(counterMeasureCR)
			if err == nil {

				// TODO move this all to a re-usable method
				meta.SetStatusCondition(&counterMeasureCR.Status.Conditions, metav1.Condition{
					Type:    v1alpha1.TypeMonitoring,
					Status:  metav1.ConditionTrue,
					Reason:  v1alpha1.ReasonSucceeded,
					Message: "Monitoring alerts",
				})

				counterMeasureCR.Status.LastStatus = v1alpha1.Monitoring
				counterMeasureCR.Status.LastStatusChangeTime = &metav1.Time{Time: time.Now()}

				if err = r.Status().Update(context.TODO(), counterMeasureCR); err != nil {
					logger.Error(err, "failed to update countermeasure status")
					return ctrl.Result{}, err
				}

				if err := r.Get(ctx, req.NamespacedName, counterMeasureCR); err != nil {
					logger.Error(err, "failed to reload countermeasure")
					return ctrl.Result{}, err
				}
			}
		}

		return ctrl.Result{}, err
	}
}

func (r *CounterMeasureReconciler) isValid(ctx context.Context,
	counterMeasure *v1alpha1.CounterMeasure) (bool, error) {
	logger := log.FromContext(ctx)
	promSvc := counterMeasure.Spec.Prometheus.Service
	serviceObject := &corev1.Service{}
	var err error
	valid := true
	if err = r.Get(ctx, promSvc.GetNamespacedName(), serviceObject); err != nil {
		if errors.IsNotFound(err) {
			meta.SetStatusCondition(&counterMeasure.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.TypeMonitoring,
				Status:             metav1.ConditionFalse,
				Reason:             v1alpha1.ReasonResourceNotAvailable,
				LastTransitionTime: metav1.NewTime(time.Now()),
				Message:            fmt.Sprintf("service %v:%v not found", promSvc.Namespace, promSvc.Name),
			})

			// if the update happens the the err value will be reset to nil which is what we
			// want so that we don't retry the reconcile
			if err = r.Status().Update(ctx, counterMeasure); err != nil {
				logger.Error(err, "failed to update countermeasure status")
			}
		} else {
			logger.Error(err, "error getting prometheus target resource", "name", promSvc.Name, "namespace", promSvc.Namespace)
		}

		valid = false
	}
	return (valid && len(ValidateSpec(&counterMeasure.Spec)) == 0), err
}

// SetupWithManager sets up the controller with the Manager.
func (r *CounterMeasureReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.CounterMeasure{}).
		Complete(r)
}
