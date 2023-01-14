package sources

import (
	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/operator/events"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type EventPublisher interface {
	Publish(events.Event) error
}
type EventPublisherFunc func(events.Event) error

func (pub EventPublisherFunc) Publish(event events.Event) error {
	return pub(event)
}

type ObjectKey struct {
	types.NamespacedName
	Generation int64
}
type ActiveEventSources map[ObjectKey]EventSource
type ActiveCounterMeasures map[ObjectKey]struct{}

type EventManager interface {
	RemoveSource(types.NamespacedName) error

	// Exists Must check for the correct Generation vs existing, if
	// the Generations don't match then will return false.
	SourceExists(metav1.ObjectMeta) bool

	AddSource(EventSource) error

	AddCounterMeasure(*v1alpha1.CounterMeasure) error
	RemoveCounterMeasure(types.NamespacedName) error
	CounterMeasureExists(metav1.ObjectMeta) bool
}

type EventSource interface {
	Key() ObjectKey
	Start(<-chan struct{}) error
	Subscribe(EventPublisher) error
}

func (k ObjectKey) GetName() string {
	return k.Namespace + "/" + k.Name
}
