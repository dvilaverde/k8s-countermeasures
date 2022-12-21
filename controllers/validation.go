package controllers

import (
	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateSpec(spec *v1alpha1.CounterMeasureSpec) field.ErrorList {
	return field.ErrorList{}
}
