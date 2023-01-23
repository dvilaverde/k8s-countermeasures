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

	Name string
	// A set of sources that this counter measure will accept events from
	Sources        map[events.SourceName]struct{}
	Key            manager.ObjectKey
	Countermeasure *v1alpha1.CounterMeasure
	Running        bool
	lastRuns       map[string]time.Time
}

type ActionState struct {
	measuresMux sync.RWMutex
	// map of deployed countermeasures, keyed by ObjectKey to the CounterMeasure spec
	counterMeasures ActiveCounterMeasures
	// eventname to ObjectKey of the countermeasure
	eventIndex map[string][]manager.ObjectKey
}

func NewState() *ActionState {
	return &ActionState{
		measuresMux:     sync.RWMutex{},
		counterMeasures: make(ActiveCounterMeasures, 0),
		eventIndex:      make(map[string][]manager.ObjectKey),
	}
}

func (s *ActionState) IsRunning(key manager.ObjectKey) bool {
	s.measuresMux.RLock()
	defer s.measuresMux.RUnlock()

	entry, ok := s.counterMeasures[key]
	if !ok {
		return false
	}

	return entry.Running
}

// CounterMeasureStart record the start of a countermeasure for an event.
func (s *ActionState) CounterMeasureStart(event events.Event, key manager.ObjectKey) {
	s.measuresMux.Lock()
	defer s.measuresMux.Unlock()

	entry := s.counterMeasures[key]
	entry.Lock()
	entry.Running = true

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
	entry.Running = false
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
func (s *ActionState) Add(countermeasure *v1alpha1.CounterMeasure, sources []manager.ObjectKey) error {

	key := manager.ToKey(countermeasure.ObjectMeta)

	// In case we already have something registered here, lets remove and re-add
	s.Remove(key.NamespacedName)

	onEvent := countermeasure.Spec.OnEvent
	entry := &Entry{
		Name:           onEvent.EventName,
		Sources:        make(map[events.SourceName]struct{}),
		Key:            key,
		Countermeasure: countermeasure,
		Running:        false,
		lastRuns:       make(map[string]time.Time),
	}

	for _, source := range sources {
		key := source.NamespacedName
		entry.Sources[events.SourceName(key)] = struct{}{}
	}

	s.measuresMux.Lock()
	defer s.measuresMux.Unlock()

	s.counterMeasures[key] = entry
	s.eventIndex[onEvent.EventName] = append(s.eventIndex[onEvent.EventName], key)

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
func (s *ActionState) GetCounterMeasures(eventName string) []*Entry {
	s.measuresMux.Lock()
	defer s.measuresMux.Unlock()

	objecKeys, ok := s.eventIndex[eventName]
	if !ok {
		return nil
	}

	var rebuildIndex = false
	entries := make([]*Entry, 0, len(objecKeys))
	for _, objectKey := range objecKeys {
		entry, ok := s.counterMeasures[objectKey]
		if !ok {
			// A countermeasure was removed but the index wasn't updated
			rebuildIndex = true
			continue
		}

		// do some housekeeping when we're looking for CounterMeasure entries
		// so we can free up any memory used from previous events.
		entry.clearExpiredLastRuns()
		entries = append(entries, entry)
	}

	// rebuild the out of date index
	if rebuildIndex {
		s.eventIndex[eventName] = []manager.ObjectKey{}
		for _, e := range entries {
			s.eventIndex[eventName] = append(s.eventIndex[eventName], e.Key)
		}
	}

	return entries
}

// Accept returns true if the event should trigger a CounterMeasure
func (e *Entry) Accept(event events.Event) bool {
	var (
		accept = e.Name == event.Name
	)

	if accept && e.Sources != nil && len(e.Sources) > 0 {
		_, accept = e.Sources[event.Source]
	}

	return accept
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

	return e.Running || suppressed
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
