package events

import (
	"testing"
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
