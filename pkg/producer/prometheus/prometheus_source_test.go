package prometheus

import (
	"reflect"
	"testing"
	"time"

	"github.com/dvilaverde/k8s-countermeasures/apis/eventsource/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/dvilaverde/k8s-countermeasures/pkg/manager"
	"github.com/dvilaverde/k8s-countermeasures/pkg/producer"
	prom_v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestEventSource_Key(t *testing.T) {
	type fields struct {
		key manager.ObjectKey
	}
	tests := []struct {
		name   string
		fields fields
		want   manager.ObjectKey
	}{
		{
			name: "nskey",
			fields: fields{
				key: manager.ObjectKey{
					NamespacedName: types.NamespacedName{
						Namespace: "ns",
						Name:      "name",
					},
					Generation: 1,
				},
			},
			want: manager.ObjectKey{
				NamespacedName: types.NamespacedName{
					Namespace: "ns",
					Name:      "name",
				},
				Generation: 1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &EventProducer{
				config: PrometheusConfig{
					Key: tt.fields.key,
				},
			}
			if got := d.config.Key; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EventSource.Key() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEventSource_poll(t *testing.T) {
	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}

	alertTime := time.Date(2017, 01, 15, 0, 0, 0, 0, time.UTC)
	alerts := make([]prom_v1.Alert, 1)
	activeAlert := prom_v1.Alert{
		ActiveAt: alertTime,
		Annotations: model.LabelSet{
			"managed-by": "helm",
		},
		Labels: model.LabelSet{
			"label":     "value",
			"alertname": "active-alert",
			"pod":       "app-pod-xyxsl",
		},
		State: prom_v1.AlertStateFiring,
		Value: "1",
	}
	alerts[0] = activeAlert

	api.On("Alerts", mock.AnythingOfType("*context.timerCtx")).Return(prom_v1.AlertsResult{
		Alerts: alerts,
	})

	p := NewPrometheusService(client.API())

	promCR := &v1alpha1.Prometheus{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "prom1",
			Namespace:  "ns1",
			Generation: 1,
		},
		Spec: v1alpha1.PrometheusSpec{
			IncludePending: false,
			PollingInterval: metav1.Duration{
				Duration: 15 * time.Millisecond,
			},
		},
	}

	config := PrometheusConfig{
		Key:    manager.ToKey(promCR.ObjectMeta),
		Client: p,
	}
	var eventProducer producer.EventProducer // TODO nees impl
	eventsource := NewEventProducer(config, eventProducer)

	assert.Equal(t, "ns1/prom1", eventsource.Key().GetName())

	done := make(chan struct{})
	go eventsource.Start(done)

	publishCh := make(chan events.Event)

	// TODO: subscribe to the event bus
	eventsource.Subscribe(events.OnEventFunc(func(e events.Event) error {
		publishCh <- e
		return nil
	}))

	select {
	case event := <-publishCh:
		assert.Equal(t, "active-alert", event.Name)
		assert.Equal(t, "app-pod-xyxsl", event.Data.Get("pod"))
	case <-time.After(time.Second * 5):
		t.Fatal("event never arrived")
	}

	close(done)
}
