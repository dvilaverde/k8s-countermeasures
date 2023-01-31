package events

import (
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"
)

type EventData map[string]string

func (d *EventData) Get(key string) string {
	return (*d)[key]
}

type Event struct {
	Name       string    `json:"name,omitempty"`
	ActiveTime time.Time `json:"activeTime,omitempty"`
	// Data is a pointer ref so these events can be added into the workqueue of the Dispatcher
	Data *EventData `json:"data,omitempty"`
}

// Key hash the EventData into a key that can be used to de-duplicate events.
func (e Event) Key() string {
	if (e.Data == nil || len(*e.Data) == 0) && len(e.Name) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(e.Name)

	if e.Data != nil {
		keys := make([]string, 0, len(*e.Data))
		for k := range *e.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString((*e.Data)[k])
		}
	}

	h := fnv.New32a()
	h.Write([]byte(sb.String()))
	es := hex.EncodeToString(h.Sum(nil))
	return es
}

// CreateFullyQualifiedTopicName creates a FQTN from the topic name and event source
// but if the source is empty then the topic will be returned as is.
func CreateFullyQualifiedTopicName(topic string, source types.NamespacedName) string {

	fqtn := topic
	if (source != types.NamespacedName{}) {
		fqtn = fmt.Sprintf("%s:%s:%s", topic, source.Namespace, source.Name)
	}

	return fqtn
}
