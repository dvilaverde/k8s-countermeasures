package actions

import (
	"fmt"
	"testing"
	"time"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	esV1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/eventsource/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/pkg/actions/state"
	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestManager_OnEvent(t *testing.T) {
	manager := Deploy(t)

	e := events.Event{
		Name: "event2",
		Source: events.SourceName{
			Namespace: "ns",
			Name:      "prometheus",
		},
		ActiveTime: time.Now(),
		Data: &events.EventData{
			"prop1": "value1",
		},
	}

	manager.OnEvent(e)
	assert.Eventually(t, func() bool {

		e := manager.state.GetCounterMeasures("event2")[0]
		println(fmt.Sprintf("Running: %v", e.Running))

		return !e.Running
	}, time.Second*5, time.Millisecond*500, "expected the action to complete")
}

func TestManager_Add(t *testing.T) {

	manager := Deploy(t)

	assert.True(t, manager.Exists(CreateObjectMeta("all-events")))
	entry := manager.state.GetCounterMeasures("event1")[0]
	assert.Equal(t, 0, len(entry.Sources))

	assert.True(t, manager.Exists(CreateObjectMeta("selected-events")))
	entry = manager.state.GetCounterMeasures("event2")[0]
	assert.Equal(t, 1, len(entry.Sources))
}

func TestManager_Remove(t *testing.T) {
	manager := Deploy(t)
	assert.True(t, manager.Exists(CreateObjectMeta("all-events")))
	err := manager.Remove(types.NamespacedName{Namespace: "ns", Name: "all-events"})
	assert.NoError(t, err)
	assert.False(t, manager.Exists(CreateObjectMeta("all-events")))
}

func TestManager_Exists(t *testing.T) {
	manager := Deploy(t)
	assert.True(t, manager.Exists(CreateObjectMeta("all-events")))
}

func Deploy(t *testing.T) *Manager {
	actionRegistry := Registry{}
	actionRegistry.Initialize()

	es := &esV1alpha1.Prometheus{
		TypeMeta: v1.TypeMeta{
			APIVersion: esV1alpha1.GroupVersion.Group + "/" + esV1alpha1.GroupVersion.Version,
			Kind:       "Prometheus",
		},
		ObjectMeta: CreateObjectMeta("prometheus"),
	}
	objs := []runtime.Object{es}

	// Create a fake client to mock API calls.
	s, _ := esV1alpha1.SchemeBuilder.Build()
	k8sClient := clientfake.NewClientBuilder().
		WithScheme(s).
		WithRuntimeObjects(objs...).
		Build()

	manager := &Manager{
		client:         k8sClient,
		restConfig:     nil,
		recorder:       nil,
		state:          *state.NewState(),
		ActionRegistry: actionRegistry,
	}
	managerLog = testr.New(t)

	// add without a SourceSelector
	cm := &v1alpha1.CounterMeasure{
		TypeMeta: v1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.Group + "/" + v1alpha1.GroupVersion.Version,
			Kind:       "CounterMeasure",
		},
		ObjectMeta: CreateObjectMeta("all-events"),
		Spec: v1alpha1.CounterMeasureSpec{
			OnEvent: v1alpha1.OnEventSpec{
				EventName: "event1",
			},
		},
	}
	manager.Add(cm)

	// add with a SourceSelector
	cm = &v1alpha1.CounterMeasure{
		TypeMeta: v1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.Group + "/" + v1alpha1.GroupVersion.Version,
			Kind:       "CounterMeasure",
		},
		ObjectMeta: CreateObjectMeta("selected-events"),
		Spec: v1alpha1.CounterMeasureSpec{
			OnEvent: v1alpha1.OnEventSpec{
				EventName: "event2",
				SourceSelector: &v1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
			},
		},
	}
	manager.Add(cm)

	return manager
}

func CreateObjectMeta(name string) v1.ObjectMeta {
	meta := v1.ObjectMeta{
		Namespace:  "ns",
		Name:       name,
		Generation: 1,
		Labels:     map[string]string{"app": "test"},
	}

	return meta
}
