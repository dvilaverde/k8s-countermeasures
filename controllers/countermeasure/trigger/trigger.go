package trigger

import (
	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

type Handler interface {
	OnDetection(types.NamespacedName, map[string]string)
}

type HandlerFunc func(types.NamespacedName, map[string]string)

func (handler HandlerFunc) OnDetection(name types.NamespacedName, labels map[string]string) {
	handler(name, labels)
}

type CancelFunc func()

type Trigger interface {
	NotifyOn(countermeasure v1alpha1.CounterMeasure, callback Handler) (CancelFunc, error)

	Supports(countermeasure *v1alpha1.CounterMeasureSpec) bool
}
