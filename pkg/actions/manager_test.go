package actions

import (
	"context"
	"strings"
	"sync"
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
	"k8s.io/client-go/tools/record"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestManager_OnEvent(t *testing.T) {
	eventsMux := sync.Mutex{}
	recordedEvents := make([]string, 0)
	manager, eventsCh := Deploy(t)

	go func() {
		for s := range eventsCh {
			eventsMux.Lock()
			recordedEvents = append(recordedEvents, s)
			eventsMux.Unlock()
		}
	}()

	mockAction := &MockAction{}

	manager.ActionRegistry.RegisterAction(v1alpha1.RestartSpec{}, func(spec v1alpha1.Action, c ActionContext, dryRun bool) Action {
		return mockAction
	})

	e := events.Event{
		Name:       "event2",
		ActiveTime: time.Now(),
		Data: &events.EventData{
			"prop1": "value1",
		},
	}

	manager.OnEvent(e)
	assert.Eventually(t, func() bool {
		e := manager.state.GetCounterMeasures("event2")[0]
		return !e.Running
	}, time.Second*5, time.Millisecond*500, "expected the action to complete")

	eventsMux.Lock()
	assert.Equal(t, 1, len(recordedEvents))
	eventsMux.Unlock()

	// if we fire the event again it should not fire due to the suppression policy
	manager.OnEvent(e)
	assert.Eventually(t, func() bool {
		eventsMux.Lock()
		defer eventsMux.Unlock()
		return strings.Contains(recordedEvents[1], "Skipping")
	}, time.Second*5, time.Millisecond*500, "expected there to be a suppress event")
}

func TestManager_Add(t *testing.T) {

	manager, _ := Deploy(t)

	assert.True(t, manager.Exists(CreateObjectMeta("all-events")))
	entry := manager.state.GetCounterMeasures("event1")[0]
	assert.Equal(t, 0, len(entry.Sources))

	assert.True(t, manager.Exists(CreateObjectMeta("selected-events")))
	entry = manager.state.GetCounterMeasures("event2")[0]
	assert.Equal(t, 1, len(entry.Sources))
}

func TestManager_Remove(t *testing.T) {
	manager, _ := Deploy(t)
	assert.True(t, manager.Exists(CreateObjectMeta("all-events")))
	err := manager.Remove(types.NamespacedName{Namespace: "ns", Name: "all-events"})
	assert.NoError(t, err)
	assert.False(t, manager.Exists(CreateObjectMeta("all-events")))
}

func TestManager_Exists(t *testing.T) {
	manager, _ := Deploy(t)
	assert.True(t, manager.Exists(CreateObjectMeta("all-events")))
}

func Deploy(t *testing.T) (*Manager, <-chan string) {
	actionRegistry := Registry{}
	// purposely not initializing the action registry so that the callsers of Deploy
	// can add mock actions

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

	recorder := record.NewFakeRecorder(100)

	manager := &Manager{
		client:         k8sClient,
		restConfig:     nil,
		recorder:       recorder,
		state:          state.NewState(),
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
				SuppressionPolicy: &v1alpha1.SuppressionPolicySpec{
					Duration: &v1.Duration{
						Duration: time.Second * 10,
					},
				},
				SourceSelector: &v1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
			},
			Actions: []v1alpha1.Action{
				{
					Name:         "test",
					RetryEnabled: false,
					Restart: &v1alpha1.RestartSpec{
						DeploymentRef: v1alpha1.DeploymentReference{
							Namespace: "ns",
							Name:      "name",
						},
					},
				},
			},
		},
	}
	manager.Add(cm)

	return manager, recorder.Events
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

type MockAction struct {
}

func (mock *MockAction) Perform(context.Context, events.Event) error {
	return nil
}

func (mock *MockAction) GetName() string {
	return "mock"
}

func (mock *MockAction) GetType() string {
	return "mock"
}

func (mock *MockAction) GetTargetObjectName(events.Event) string {
	return "mock"
}

func (mock *MockAction) SupportsRetry() bool {
	return false
}
