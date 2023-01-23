package events

import (
	"encoding/hex"
	"hash/fnv"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"
)

type EventListener interface {
	OnEvent(Event) error
}

type OnEventFunc func(Event) error

func (pub OnEventFunc) OnEvent(event Event) error {
	return pub(event)
}

type SourceName types.NamespacedName

type EventData map[string]string

func (d *EventData) Get(key string) string {
	return (*d)[key]
}

type Event struct {
	Name       string
	ActiveTime time.Time
	// Data is a pointer ref so these events can be added
	// into the workqueue of the Dispatcher
	Data   *EventData
	Source SourceName
}

// Key hash the EventData into a key that can be used to de-duplicate events.
func (e Event) Key() string {
	if len(*e.Data) == 0 && len(e.Name) == 0 {
		return ""
	}

	keys := make([]string, 0, len(*e.Data))
	for k := range *e.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString(e.Name)
	for _, k := range keys {
		sb.WriteString((*e.Data)[k])
	}

	h := fnv.New32a()
	h.Write([]byte(sb.String()))
	es := hex.EncodeToString(h.Sum(nil))
	return es
}
