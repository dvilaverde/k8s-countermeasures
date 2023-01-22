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
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var prometheuslog = logf.Log.WithName("prometheus-resource")

var webhookClient client.Client

func (r *Prometheus) SetupWebhookWithManager(mgr ctrl.Manager) error {
	if webhookClient == nil {
		webhookClient = mgr.GetClient()
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-eventsource-vilaverde-rocks-v1alpha1-prometheus,mutating=true,failurePolicy=fail,sideEffects=None,groups=eventsource.vilaverde.rocks,resources=prometheuses,verbs=create;update,versions=v1alpha1,name=mprometheus.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Prometheus{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Prometheus) Default() {
	prometheuslog.Info("default", "name", r.Name)
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-eventsource-vilaverde-rocks-v1alpha1-prometheus,mutating=false,failurePolicy=fail,sideEffects=None,groups=eventsource.vilaverde.rocks,resources=prometheuses,verbs=create;update,versions=v1alpha1,name=vprometheus.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Prometheus{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Prometheus) ValidateCreate() error {
	prometheuslog.Info("validate create", "name", r.Name)
	return ValidatePrometheus(r)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Prometheus) ValidateUpdate(old runtime.Object) error {
	prometheuslog.Info("validate update", "name", r.Name)
	return ValidatePrometheus(r)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Prometheus) ValidateDelete() error {
	return nil
}

func ValidatePrometheus(r *Prometheus) error {
	p := r.Spec
	if (p.Service == ServiceReference{}) {
		return fmt.Errorf("service reference is required")
	}

	svc := &corev1.Service{}
	if err := webhookClient.Get(context.Background(), p.Service.GetNamespacedName(), svc); err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("prometheus service '%s' is not found in namespace '%s'", p.Service.Name, p.Service.Namespace)
		}
		return err
	}

	if p.Auth != nil {
		secretRef := p.Auth.SecretReference
		secret := &corev1.Secret{}

		// use the namespace of the event p8s source when none provided for the secret.
		namespace := secretRef.Namespace
		if len(namespace) == 0 {
			namespace = r.Namespace
		}

		secretName := types.NamespacedName{Namespace: namespace, Name: secretRef.Name}
		if err := webhookClient.Get(context.Background(), secretName, secret); err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("secret '%s' is not found in namespace '%s'", secretRef.Name, secretRef.Namespace)
			}
			return err
		}
	}

	return nil
}
