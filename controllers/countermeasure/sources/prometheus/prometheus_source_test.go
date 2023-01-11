package prometheus

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/events"
	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/sources"
	prom_v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type OperatorSDKClientMock struct {
	mock.Mock
	client.Client
}

func (m *OperatorSDKClientMock) Get(ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	opts ...client.GetOption) error {

	args := m.Called(ctx, key, nil, nil)
	return args.Error(0)
}

func Test_callbackSuppressExpired(t *testing.T) {

	data := make(events.EventData)
	event := events.Event{
		Name:       "Alert1",
		ActiveTime: time.Now().Add(-30 * time.Second),
		Data:       &data,
	}

	cb := callback{
		name:             types.NamespacedName{Namespace: "ns", Name: "name"},
		suppressedAlerts: make(map[string]time.Time),
		alertSpec: &v1alpha1.PrometheusAlertSpec{
			SuppressionPolicy: &v1alpha1.SuppressionPolicySpec{
				Duration: &metav1.Duration{
					Duration: 15 * time.Second,
				},
			},
		},
	}

	cb.suppressedAlerts[event.Key()] = event.ActiveTime

	cb.removeExpiredSuppressions()

	assert.Equal(t, 0, len(cb.suppressedAlerts), "suppressed alert was not deleted")
}

func Test_Notify(t *testing.T) {

	ctx := context.TODO()

	mockClient := new(OperatorSDKClientMock)
	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	p8Client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}

	alertTime := time.Date(2017, 01, 15, 0, 0, 0, 0, time.UTC)
	alerts := make([]prom_v1.Alert, 1)
	alerts[0] = prom_v1.Alert{
		ActiveAt: alertTime,
		Labels: model.LabelSet{
			"label":     "value",
			"alertname": "custom-alert",
			"pod":       "app-pod-xyxsl",
		},
		State: prom_v1.AlertStateFiring,
		Value: "1",
	}

	api.On("Alerts", mock.AnythingOfType("*context.timerCtx")).Return(prom_v1.AlertsResult{
		Alerts: alerts,
	})

	builder := func(string, string, string) (*PrometheusService, error) {
		return NewPrometheusService(p8Client.API()), nil
	}

	source := NewEventSource(builder, 1*time.Second)
	source.InjectClient(mockClient)
	if err := source.Start(ctx); err != nil {
		t.Error(err)
		return
	}

	cm := v1alpha1.CounterMeasure{
		TypeMeta:   metav1.TypeMeta{Kind: "CounterMeasure", APIVersion: "countermeasure.vilaverde.rocks/v1alpha1"},
		ObjectMeta: metav1.ObjectMeta{Name: "cm1", Namespace: "ns1"},
		Spec: v1alpha1.CounterMeasureSpec{
			Prometheus: &v1alpha1.PrometheusSpec{
				Service: &v1alpha1.ServiceReference{
					Namespace: "ns-mon",
					Name:      "prom-svc",
				},
				Alert: &v1alpha1.PrometheusAlertSpec{
					AlertName:      "custom-alert",
					IncludePending: false,
				},
			},
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)

	assert.True(t, source.Supports(&cm.Spec))
	source.NotifyOn(cm, sources.HandlerFunc(func(nn types.NamespacedName, e []events.Event, done chan<- string) {
		assert.Equal(t, 3, len(*e[0].Data))
		wg.Done()
		close(done)
	}))

	if err != nil {
		t.Error(err)
		return
	}

	wg.Wait()
}

func Test_findNamedPort(t *testing.T) {
	type args struct {
		service   *corev1.Service
		namedPort string
	}
	tests := []struct {
		name  string
		args  args
		want  corev1.ServicePort
		want1 bool
	}{
		{
			name: "two ports",
			args: args{
				service: &corev1.Service{
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name: "foo",
								Port: 8080,
							},
							{
								Name: "web",
								Port: 8081,
							},
						},
					},
				},
				namedPort: "web",
			},
			want: corev1.ServicePort{
				Name: "web",
				Port: 8081,
			},
			want1: true,
		},
		{
			name: "one port",
			args: args{
				service: &corev1.Service{
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name: "foo",
								Port: 8080,
							},
						},
					},
				},
				namedPort: "web",
			},
			want: corev1.ServicePort{
				Name: "foo",
				Port: 8080,
			},
			want1: true,
		},
		{
			name: "zero ports",
			args: args{
				service: &corev1.Service{
					Spec: corev1.ServiceSpec{},
				},
				namedPort: "web",
			},
			want:  corev1.ServicePort{},
			want1: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := findNamedPort(tt.args.service, tt.args.namedPort)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findNamedPort() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("findNamedPort() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
