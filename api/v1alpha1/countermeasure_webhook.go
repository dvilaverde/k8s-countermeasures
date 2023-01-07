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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var (
	countermeasurelog = logf.Log.WithName("countermeasure-resource")
)

func (r *CounterMeasure) SetupWebhookWithManager(mgr ctrl.Manager) error {

	if webhookClient == nil {
		webhookClient = mgr.GetClient()
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-operator-vilaverde-rocks-v1alpha1-countermeasure,mutating=true,failurePolicy=fail,sideEffects=None,groups=operator.vilaverde.rocks,resources=countermeasures,verbs=create;update,versions=v1alpha1,name=mcountermeasure.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &CounterMeasure{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *CounterMeasure) Default() {
	countermeasurelog.Info("default", "name", r.Name)

	// TODO: fill in your defaulting logic.
}

//+kubebuilder:webhook:path=/validate-operator-vilaverde-rocks-v1alpha1-countermeasure,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.vilaverde.rocks,resources=countermeasures,verbs=create;update,versions=v1alpha1,name=vcountermeasure.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &CounterMeasure{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *CounterMeasure) ValidateCreate() error {
	countermeasurelog.Info("validate create", "name", r.Name)
	return ValidateSpec(&r.Spec)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *CounterMeasure) ValidateUpdate(old runtime.Object) error {
	countermeasurelog.Info("validate update", "name", r.Name)
	return ValidateSpec(&r.Spec)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *CounterMeasure) ValidateDelete() error {
	return nil
}
