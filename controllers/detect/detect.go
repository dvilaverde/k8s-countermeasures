package detect

import (
	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

type DetectedFunc func(types.NamespacedName, map[string]string)

type Detector interface {
	NotifyOn(countermeasure v1alpha1.CounterMeasure, callback DetectedFunc) error

	Supports(countermeasure *v1alpha1.CounterMeasureSpec) bool
}
