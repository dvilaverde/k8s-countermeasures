package prometheus

import (
	"context"
	"testing"
	"time"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/prometheus"
	"github.com/kiali/kiali/prometheus/prometheustest"
	prom_v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type PromAPIMock struct {
	prometheustest.PromAPIMock
}

func (o *PromAPIMock) Query(ctx context.Context,
	query string,
	ts time.Time,
	opts ...prom_v1.Option) (model.Value, prom_v1.Warnings, error) {

	args := o.Called(ctx, query, ts)
	return args.Get(0).(model.Value), nil, nil
}

func (o *PromAPIMock) QueryRange(ctx context.Context, query string, r prom_v1.Range, opts ...prom_v1.Option) (model.Value, prom_v1.Warnings, error) {
	args := o.Called(ctx, query, r)
	return args.Get(0).(model.Value), nil, nil
}

func (o *PromAPIMock) WalReplay(ctx context.Context) (prom_v1.WalReplayStatus, error) {
	return prom_v1.WalReplayStatus{
		Min:     0,
		Max:     0,
		Current: 0,
	}, nil
}

func setupMocked() (*prometheus.Client, *PromAPIMock, error) {
	config.Set(config.NewConfig())
	api := new(PromAPIMock)

	client, err := prometheus.NewClient()
	if err != nil {
		return nil, nil, err
	}
	client.Inject(api)
	return client, api, nil
}

func TestGetAlerts(t *testing.T) {
	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}

	alertTime := time.Date(2017, 01, 15, 0, 0, 0, 0, time.UTC)

	alerts := make([]prom_v1.Alert, 2)
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
	pendingAlert := prom_v1.Alert{
		ActiveAt: alertTime,
		Annotations: model.LabelSet{
			"managed-by": "helm",
		},
		Labels: model.LabelSet{
			"label":     "value",
			"alertname": "pending-alert",
			"pod":       "app-pod-xyxsl",
		},
		State: prom_v1.AlertStatePending,
		Value: "1",
	}
	alerts[0] = activeAlert
	alerts[1] = pendingAlert

	api.On("Alerts", mock.AnythingOfType("*context.timerCtx")).Return(prom_v1.AlertsResult{
		Alerts: alerts,
	})

	p := NewPrometheusService(client.API())
	activeAlerts, err := p.GetActiveAlerts()
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, 2, len(activeAlerts.alerts))

	events, err := activeAlerts.ToEvents(false)
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, 1, len(events))
	data := *events[0].Data
	assert.Equal(t, 3, len(data))
	assert.Equal(t, "app-pod-xyxsl", data["pod"])
	assert.Equal(t, "active-alert", data["alertname"])

	events, err = activeAlerts.ToEvents(true)
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, 2, len(events))
}
