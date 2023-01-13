package events

import (
	"encoding/hex"
	"hash/fnv"
	"sort"
	"strings"
	"time"
)

type SourceName struct {
	Name      string
	Namespace string
}

type EventData map[string]string

type Event struct {
	Name       string
	ActiveTime time.Time
	Data       *EventData
	Source     SourceName
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
