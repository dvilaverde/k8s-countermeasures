package state

import (
	"sync"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/dvilaverde/k8s-countermeasures/pkg/manager"
	"k8s.io/apimachinery/pkg/types"
)

type ActiveCounterMeasures map[manager.ObjectKey]*Entry
type RunningSet map[manager.ObjectKey]struct{}

type Entry struct {
	Name string
	// A set of sources that this counter measure will accept events from
	Sources        map[events.SourceName]struct{}
	Key            manager.ObjectKey
	Countermeasure *v1alpha1.CounterMeasure
	Running        bool
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

func (s *ActionState) SetRunning(key manager.ObjectKey, isRunning bool) {
	s.measuresMux.Lock()
	defer s.measuresMux.Unlock()

	entry := s.counterMeasures[key]
	entry.Running = isRunning
}

func (s *ActionState) IsDeployed(key manager.ObjectKey) bool {
	s.measuresMux.Lock()
	defer s.measuresMux.Unlock()

	_, ok := s.counterMeasures[key]
	return ok
}

func (s *ActionState) Add(countermeasure *v1alpha1.CounterMeasure, sources []manager.ObjectKey) error {

	key := manager.ToKey(countermeasure.ObjectMeta)
	if s.IsDeployed(key) {
		// we already have something registered here, lets remove and re-add
		s.Remove(key.NamespacedName)
	}

	onEvent := countermeasure.Spec.OnEvent
	entry := &Entry{
		Name:           onEvent.EventName,
		Sources:        make(map[events.SourceName]struct{}),
		Key:            key,
		Countermeasure: countermeasure,
		Running:        false,
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

func (s *ActionState) GetCounterMeasures(eventName string) []Entry {
	s.measuresMux.Lock()
	defer s.measuresMux.Unlock()

	objecKeys, ok := s.eventIndex[eventName]
	if !ok {
		return []Entry{}
	}

	entries := make([]Entry, 0, len(objecKeys))
	for _, objectKey := range objecKeys {
		entry, ok := s.counterMeasures[objectKey]
		if !ok {
			// something is wrong here maybe log in the future but for now just panic so
			// we can catch during development
			panic("invalid state")
		}

		entries = append(entries, *entry)
	}

	return entries
}

func (e Entry) Accept(event events.Event) bool {
	var (
		accept = e.Name == event.Name
	)

	if e.Sources != nil && len(e.Sources) > 0 {
		_, accept = e.Sources[event.Source]
	}

	return accept
}
