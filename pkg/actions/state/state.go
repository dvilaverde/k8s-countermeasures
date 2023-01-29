package state

import (
	"sync"
	"time"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/dvilaverde/k8s-countermeasures/pkg/manager"
	"k8s.io/apimachinery/pkg/types"
)

type ActiveCounterMeasures map[manager.ObjectKey]*Entry
type RunningSet map[manager.ObjectKey]struct{}

type Entry struct {
	sync.Mutex

	Name           string
	Countermeasure *v1alpha1.CounterMeasure

	running  bool
	lastRuns map[string]time.Time
}

type ActionState struct {
	measuresMux sync.RWMutex
	// map of deployed countermeasures, keyed by ObjectKey to the CounterMeasure spec
	counterMeasures ActiveCounterMeasures
}

func NewState() *ActionState {
	return &ActionState{
		measuresMux:     sync.RWMutex{},
		counterMeasures: make(ActiveCounterMeasures, 0),
	}
}

func (s *ActionState) IsRunning(key manager.ObjectKey) bool {
	s.measuresMux.RLock()
	defer s.measuresMux.RUnlock()

	entry, ok := s.counterMeasures[key]
	if !ok {
		return false
	}

	return entry.running
}

// CounterMeasureStart record the start of a countermeasure for an event.
func (s *ActionState) CounterMeasureStart(event events.Event, key manager.ObjectKey) {
	s.measuresMux.Lock()
	defer s.measuresMux.Unlock()

	entry := s.counterMeasures[key]
	entry.Lock()
	entry.running = true

	// if there is a suppression policy then calculate the time until future events
	// with the same key are suppressed.
	policy := entry.Countermeasure.Spec.OnEvent.SuppressionPolicy
	if policy != nil && policy.Duration != nil {
		entry.lastRuns[event.Key()] = time.Now().Add(policy.Duration.Duration)
	}
	entry.Unlock()
}

// CounterMeasureEnd record that a Countermeasure has ended.
func (s *ActionState) CounterMeasureEnd(event events.Event, key manager.ObjectKey) {
	s.measuresMux.Lock()
	defer s.measuresMux.Unlock()
	entry := s.counterMeasures[key]

	entry.Lock()
	entry.running = false
	entry.Unlock()
}

// IsDeployed is the countermeasure with this ObjectKey deployed, this will check the Generation as well
// for the exact version.
func (s *ActionState) IsDeployed(key manager.ObjectKey) bool {
	s.measuresMux.Lock()
	defer s.measuresMux.Unlock()

	_, ok := s.counterMeasures[key]
	return ok
}

// Add adds a new CounterMeasure entry
func (s *ActionState) Add(countermeasure *v1alpha1.CounterMeasure) error {

	key := manager.ToKey(countermeasure.ObjectMeta)

	// In case we already have something registered here, lets remove and re-add
	s.Remove(key.NamespacedName)

	onEvent := countermeasure.Spec.OnEvent
	entry := &Entry{
		Name:           onEvent.EventName,
		Countermeasure: countermeasure,
		running:        false,
		lastRuns:       make(map[string]time.Time),
	}

	s.measuresMux.Lock()
	defer s.measuresMux.Unlock()

	s.counterMeasures[key] = entry

	return nil
}

// Remove delete a CounterMeasure using the NamespaceName
func (s *ActionState) Remove(name types.NamespacedName) error {
	s.measuresMux.Lock()
	defer s.measuresMux.Unlock()

	for k := range s.counterMeasures {
		if k.NamespacedName == name {
			delete(s.counterMeasures, k)
		}
	}

	return nil
}

// GetCounterMeasures return all CounterMeasure entries that subscribe to the named event.
func (s *ActionState) GetCounterMeasure(key manager.ObjectKey) *Entry {
	s.measuresMux.Lock()
	defer s.measuresMux.Unlock()

	entry, ok := s.counterMeasures[key]
	if !ok {
		return nil
	}

	// do some housekeeping when we're looking for CounterMeasure entries
	// so we can free up any memory used from previous events.
	entry.clearExpiredLastRuns()

	return entry
}

func (e *Entry) IsSuppressed(event events.Event) bool {
	e.Lock()
	defer e.Unlock()

	var (
		suppressed = false
	)

	suppressedUntil, ok := e.lastRuns[event.Key()]
	if ok {
		// we have a suppression time so check if it's expired
		suppressed = suppressedUntil.After(time.Now())
	}

	return e.running || suppressed
}

func (e *Entry) clearExpiredLastRuns() {
	e.Lock()
	defer e.Unlock()

	currentTime := time.Now()
	for k, suppressedUntil := range e.lastRuns {
		if suppressedUntil.Before(currentTime) {
			delete(e.lastRuns, k)
		}
	}
}
