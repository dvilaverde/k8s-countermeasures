package eventbus

import (
	"reflect"
	"testing"

	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
)

func TestApplyTopicMatcher(t *testing.T) {

	s := make(Subscribers)
	ch := make(chan events.Event)
	s["topic1"] = []chan events.Event{ch}
	s["topic1:namespace"] = []chan events.Event{ch}
	s["topic1:namespace:name"] = []chan events.Event{ch}
	s["topic2"] = []chan events.Event{ch}
	s["topic2:namespace"] = []chan events.Event{ch}
	s["topic2:namespace:name"] = []chan events.Event{ch}
	s["topic3"] = []chan events.Event{ch}
	s["topic4:namespace:name"] = []chan events.Event{ch}

	type args struct {
		subs  Subscribers
		topic string
	}
	tests := []struct {
		name  string
		args  args
		want  int
		want1 bool
	}{
		{
			name:  "fq_topic",
			args:  args{topic: "topic1:namespace:name", subs: s},
			want:  3,
			want1: true,
		},
		{
			name:  "empty",
			args:  args{topic: "topic1:namespace:name", subs: make(Subscribers)},
			want:  0,
			want1: false,
		},
		{
			name:  "topic4",
			args:  args{topic: "topic4:namespace:name", subs: s},
			want:  1,
			want1: true,
		},
		{
			name:  "topic3",
			args:  args{topic: "topic3:namespace:name", subs: s},
			want:  1,
			want1: true,
		},
		{
			name:  "no_subs",
			args:  args{topic: "alert5:my-ns:ds", subs: s},
			want:  0,
			want1: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := SubscriberMatch(tt.args.subs, tt.args.topic)
			if !reflect.DeepEqual(len(got), tt.want) {
				t.Errorf("SubscriberMatch() got = %v, want %v", len(got), tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("SubscriberMatch() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
