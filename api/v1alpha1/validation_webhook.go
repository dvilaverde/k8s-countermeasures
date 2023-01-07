package v1alpha1

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	util "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var webhookClient client.Client

func ValidateSpec(spec *CounterMeasureSpec) error {

	validationErrors := make([]error, 0)

	if err := ValidatePrometheus(spec.Prometheus); err != nil {
		validationErrors = append(validationErrors, err)
	}

	if len(spec.Actions) != 0 {
		for _, action := range spec.Actions {
			if err := ValidateAction(action); err != nil {
				validationErrors = append(validationErrors, err)
			}
		}
	} else {
		validationErrors = append(validationErrors, fmt.Errorf("one or more actions are required"))
	}

	return util.NewAggregate(validationErrors)
}

func ValidateAction(a Action) error {

	var (
		actionErrors = make([]error, 0)
		count        = 0
	)

	if a.Debug != nil {
		if len(a.Debug.Image) == 0 {
			actionErrors = append(actionErrors,
				fmt.Errorf("debug config for action '%s' requires a image name", a.Name))
		}
	}

	// checks to see that the action only contains 1 type of action
	tt := reflect.ValueOf(a)
	for i := 0; i < tt.NumField(); i++ {
		f := tt.Field(i)
		if f.Type().Kind() == reflect.Pointer {
			// we're only counting the pointers to the action type structs
			if !f.IsNil() {
				count++
			}
		}
	}

	if count == 0 {
		actionErrors = append(actionErrors, fmt.Errorf("action '%s' must define one action type", a.Name))
	}

	if count > 1 {
		actionErrors = append(actionErrors, fmt.Errorf("each action should only have 1 defined action type"))
	}

	return util.NewAggregate(actionErrors)
}

func ValidatePrometheus(p *PrometheusSpec) error {
	if p == nil {
		return fmt.Errorf("prometheus event source is required")
	}

	// check the service exists if we have a prometheus spec
	if p.Service == nil {
		return fmt.Errorf("prometheus service spec is required")
	}

	svc := &corev1.Service{}
	if err := webhookClient.Get(context.Background(), p.Service.GetNamespacedName(), svc); err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("service '%s' is not found in namespace '%s'", p.Service.Name, p.Service.Namespace)
		}
		return err
	}

	if p.Auth != nil {
		secretRef := p.Auth.SecretReference
		secret := &corev1.Secret{}
		secretName := types.NamespacedName{Namespace: secretRef.Namespace, Name: secretRef.Name}
		if err := webhookClient.Get(context.Background(), secretName, secret); err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("secret '%s' is not found in namespace '%s'", secretRef.Name, secretRef.Namespace)
			}
			return err
		}
	}

	if p.Alert == nil {
		return fmt.Errorf("prometheus alert spec is required")
	}

	return nil
}
