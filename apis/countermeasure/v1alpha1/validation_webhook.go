package v1alpha1

import (
	"fmt"
	"reflect"

	util "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var webhookClient client.Client

func ValidateSpec(spec *CounterMeasureSpec) error {

	validationErrors := make([]error, 0)

	if err := ValidateOnEvent(spec.OnEvent); err != nil {
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

func ValidateOnEvent(e OnEventSpec) error {
	// check the event name is present with a valid value
	if len(e.EventName) == 0 {
		return fmt.Errorf("event name is required")
	}

	return nil
}
