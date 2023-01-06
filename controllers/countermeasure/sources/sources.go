package sources

import (
	"crypto/sha1"
	"encoding/hex"
	"sort"
	"strings"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

type Handler interface {
	OnDetection(types.NamespacedName, []Event)
}

type HandlerFunc func(types.NamespacedName, []Event)

type EventData map[string]string
type Event struct {
	Name string
	Data EventData
}

// Key hash the EventData into a key that can be used to de-duplicate events.
func (e Event) Key() string {
	if len(e.Data) == 0 && len(e.Name) == 0 {
		return ""
	}

	keys := make([]string, 0, len(e.Data))
	for k := range e.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString(e.Name)
	for _, k := range keys {
		sb.WriteString(e.Data[k])
	}

	h := sha1.New()
	h.Write([]byte(sb.String()))
	es := hex.EncodeToString(h.Sum(nil))
	return es
}

func (handler HandlerFunc) OnDetection(name types.NamespacedName, event []Event) {
	handler(name, event)
}

type CancelFunc func()

type Source interface {
	NotifyOn(countermeasure v1alpha1.CounterMeasure, callback Handler) (CancelFunc, error)

	Supports(countermeasure *v1alpha1.CounterMeasureSpec) bool
}
