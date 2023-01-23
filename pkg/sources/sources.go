package sources

import (
	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/dvilaverde/k8s-countermeasures/pkg/manager"
)

type EventSource interface {
	// Key unique identifier for this object.
	Key() manager.ObjectKey
	// Start called when the event starts is started, the done channel
	// will close when it's time to shutdown.
	Start(<-chan struct{}) error
	// Subscribe listeners can be registred with the event source to receive events.
	Subscribe(events.EventListener) error
}
