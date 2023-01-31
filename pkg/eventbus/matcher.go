package eventbus

import (
	"strings"

	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
)

// SubscriberMatch match all the subscribers to the topic name
func SubscriberMatch(subs Subscribers, topic string) ([]chan events.Event, bool) {
	var chs []chan events.Event

	// always search for the exact match on the topic name, this will
	// cover both the Fully Qualified Name and Non-Qualified
	if r, ok := subs[topic]; ok {
		chs = append(chs, r...)
	}

	topicSlice := strings.Split(topic, ":")
	if len(topicSlice) > 1 {
		t := strings.Join(topicSlice[:len(topicSlice)-1], ":")
		if r, ok := SubscriberMatch(subs, t); ok {
			chs = append(chs, r...)
		}
	}

	return chs, len(chs) > 0
}
