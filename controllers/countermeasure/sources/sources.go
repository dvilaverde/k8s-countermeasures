package sources

import (
	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/events"
	"k8s.io/apimachinery/pkg/types"
)

type Handler interface {
	OnDetection(types.NamespacedName, []events.Event, chan<- string)
}

type HandlerFunc func(types.NamespacedName, []events.Event, chan<- string)

func (handler HandlerFunc) OnDetection(name types.NamespacedName, event []events.Event, done chan<- string) {
	handler(name, event, done)
}

type CancelFunc func()

type Source interface {
	NotifyOn(countermeasure v1alpha1.CounterMeasure, callback Handler) (CancelFunc, error)

	Supports(countermeasure *v1alpha1.CounterMeasureSpec) bool
}
