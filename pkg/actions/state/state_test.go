package state

import (
	"fmt"
	"testing"
	"time"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/dvilaverde/k8s-countermeasures/pkg/manager"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestActionState_IsRunning(t *testing.T) {
	state := NewState()

	key := Key()
	assert.False(t, state.IsRunning(key))

	cm := &v1alpha1.CounterMeasure{
		ObjectMeta: v1.ObjectMeta{
			Name:       key.Name,
			Namespace:  key.Namespace,
			Generation: key.Generation,
		},
		Spec: v1alpha1.CounterMeasureSpec{
			OnEvent: v1alpha1.OnEventSpec{
				EventName: "event1",
			},
		},
	}

	state.Add(cm)

	event := events.Event{
		Name:       "event1",
		ActiveTime: time.Now(),
		Data:       &events.EventData{},
	}
	state.CounterMeasureStart(event, key)
	assert.True(t, state.IsRunning(key))
}

func TestActionState_IsDeployed(t *testing.T) {
	state := NewState()

	key := Key()
	assert.False(t, state.IsDeployed(key))

	cm := &v1alpha1.CounterMeasure{
		ObjectMeta: v1.ObjectMeta{
			Name:       key.Name,
			Namespace:  key.Namespace,
			Generation: key.Generation,
		},
		Spec: v1alpha1.CounterMeasureSpec{
			OnEvent: v1alpha1.OnEventSpec{
				EventName: "event1",
			},
		},
	}

	state.Add(cm)
	assert.True(t, state.IsDeployed(key))
}

func TestActionState_Add(t *testing.T) {
	state := NewState()

	key := Key()
	assert.False(t, state.IsDeployed(key))

	cm := &v1alpha1.CounterMeasure{
		ObjectMeta: v1.ObjectMeta{
			Name:       key.Name,
			Namespace:  key.Namespace,
			Generation: key.Generation,
		},
		Spec: v1alpha1.CounterMeasureSpec{
			OnEvent: v1alpha1.OnEventSpec{
				EventName: "event1",
			},
		},
	}

	state.Add(cm)

	entry := state.GetCounterMeasure(manager.ToKey(cm.ObjectMeta))
	assert.False(t, entry.running)
	assert.Equal(t, "event1", entry.Name)
}

func TestActionState_Remove(t *testing.T) {
	state := NewState()

	key := Key()
	assert.False(t, state.IsDeployed(key))

	cm := &v1alpha1.CounterMeasure{
		ObjectMeta: v1.ObjectMeta{
			Name:       key.Name,
			Namespace:  key.Namespace,
			Generation: key.Generation,
		},
		Spec: v1alpha1.CounterMeasureSpec{
			OnEvent: v1alpha1.OnEventSpec{
				EventName: "event1",
			},
		},
	}

	state.Add(cm)
	assert.True(t, state.IsDeployed(key))
	state.Remove(key.NamespacedName)
	assert.False(t, state.IsDeployed(key))
}

func TestActionState_GetCounterMeasure(t *testing.T) {

	state := NewState()

	key := Key()
	assert.False(t, state.IsDeployed(key))

	for i := 0; i < 10; i++ {
		cm := &v1alpha1.CounterMeasure{
			ObjectMeta: v1.ObjectMeta{
				Name:       fmt.Sprintf("%s-%d", key.Name, i),
				Namespace:  key.Namespace,
				Generation: key.Generation,
			},
			Spec: v1alpha1.CounterMeasureSpec{
				OnEvent: v1alpha1.OnEventSpec{
					EventName: fmt.Sprintf("event%d", i%2),
				},
			},
		}

		state.Add(cm)
		assert.NotNil(t, state.GetCounterMeasure(manager.ToKey(cm.ObjectMeta)))
	}
}

func TestActionState_CounterMeasureEvents(t *testing.T) {
	state := NewState()

	key := Key()
	assert.False(t, state.IsDeployed(key))

	cm := &v1alpha1.CounterMeasure{
		ObjectMeta: v1.ObjectMeta{
			Name:       key.Name,
			Namespace:  key.Namespace,
			Generation: key.Generation,
		},
		Spec: v1alpha1.CounterMeasureSpec{
			OnEvent: v1alpha1.OnEventSpec{
				EventName: "event1",
				SuppressionPolicy: &v1alpha1.SuppressionPolicySpec{
					Duration: &v1.Duration{
						Duration: time.Second * 10,
					},
				},
			},
		},
	}

	state.Add(cm)
	assert.True(t, state.IsDeployed(key))

	event := events.Event{
		Name:       "event1",
		ActiveTime: time.Now(),
		Data:       &events.EventData{},
	}

	entry := state.GetCounterMeasure(manager.ToKey(cm.ObjectMeta))
	state.CounterMeasureStart(event, key)
	assert.True(t, entry.IsSuppressed(event))
	state.CounterMeasureEnd(event, key)

	// should still be suppressed since we have a 10 second suppression policy
	assert.True(t, entry.IsSuppressed(event))
}

func Key() manager.ObjectKey {
	return manager.ObjectKey{
		NamespacedName: types.NamespacedName{Namespace: "ns", Name: "name"},
		Generation:     1,
	}
}
