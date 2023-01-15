package events

import (
	"sync"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"

	"github.com/dvilaverde/k8s-countermeasures/pkg/manager"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ manager.Manager[*v1alpha1.CounterMeasure] = &Manager{}
var _ EventListener = &Manager{}

type ActiveCounterMeasures map[manager.ObjectKey]struct{}

type Manager struct {
	measuresMux sync.Mutex
	measures    ActiveCounterMeasures
}

// OnEvent called by the dispatcher when an event is received.
func (m *Manager) OnEvent(event Event) error {
	// TODO: needs implementation
	return nil
}

// Add install a countermeasure to route events to
func (m *Manager) Add(cm *v1alpha1.CounterMeasure) error {
	// TODO: needs implementation
	return nil
}

// Measure uninstall a countermeasure from the event subscription
func (m *Manager) Remove(name types.NamespacedName) error {
	m.measuresMux.Lock()
	defer m.measuresMux.Unlock()

	for k := range m.measures {
		if k.NamespacedName == name {
			delete(m.measures, k)
		}
	}

	return nil
}

// Exists uninstall a countermeasure from the event subscription
func (m *Manager) Exists(objectName metav1.ObjectMeta) bool {
	key := manager.ObjectKey{
		NamespacedName: types.NamespacedName{Namespace: objectName.Namespace, Name: objectName.Name},
		Generation:     objectName.Generation,
	}

	m.measuresMux.Lock()
	defer m.measuresMux.Unlock()

	_, ok := m.measures[key]
	return ok
}
