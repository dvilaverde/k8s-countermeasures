package sources

import (
	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/dvilaverde/k8s-countermeasures/pkg/manager"
)

type EventSource interface {
	Key() manager.ObjectKey
	Start(<-chan struct{}) error
	Subscribe(events.EventPublisher) error
}
