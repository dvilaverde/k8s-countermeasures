package events

import (
	"testing"

	"k8s.io/apimachinery/pkg/types"
)

func TestEvent_Key(t *testing.T) {
	type fields struct {
		Name string
		Data EventData
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "has-data",
			fields: fields{
				Name: "Alert1",
				Data: map[string]string{"k1": "v1", "k2": "v2"},
			},
			want: "327f7f85",
		},
		{
			name: "no-data",
			fields: fields{
				Name: "Alert1",
				Data: map[string]string{},
			},
			want: "d6990678",
		},
		{
			name: "empty",
			fields: fields{
				Name: "",
				Data: nil,
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := Event{
				Name: tt.fields.Name,
				Data: &tt.fields.Data,
			}
			if got := e.Key(); got != tt.want {
				t.Errorf("Event.Key() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateFullyQualifiedTopicName(t *testing.T) {
	type args struct {
		topic  string
		source types.NamespacedName
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "has source",
			args: args{
				topic:  "topic1",
				source: types.NamespacedName{Namespace: "ns1", Name: "source"},
			},
			want: "topic1:ns1:source",
		},
		{
			name: "no source",
			args: args{
				topic:  "topic1",
				source: types.NamespacedName{},
			},
			want: "topic1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CreateFullyQualifiedTopicName(tt.args.topic, tt.args.source); got != tt.want {
				t.Errorf("CreateFullyQualifiedTopicName() = %v, want %v", got, tt.want)
			}
		})
	}
}
