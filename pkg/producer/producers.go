package producer

import (
	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/dvilaverde/k8s-countermeasures/pkg/manager"
)

type KeyedEventProducer interface {
	EventProducer

	// Key returns a unique identifier for this key
	Key() manager.ObjectKey

	// Start called when the producer can start producing event, the done channel
	// will close when it's time to shutdown.
	Start(<-chan struct{}) error
}

type EventProducer interface {
	// Publish producers can publish events to a topic on the bus.
	Publish(string, events.Event) error
}
