package source

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type PrometheusSource struct {
	p8sApi        v1.API
	cachedAlerts  []v1.Alert
	lastQueryTime time.Time
}

func NewPrometheusSource(api v1.API) *PrometheusSource {

	return &PrometheusSource{
		p8sApi: api,
	}
}

func NewPrometheusClient(address string) (*PrometheusSource, error) {
	client, err := api.NewClient(api.Config{
		Address: address,
	})

	if err != nil {
		return nil, fmt.Errorf("error creating client, %w", err)
	}

	return NewPrometheusSource(v1.NewAPI(client)), nil
}

func (ps *PrometheusSource) IsAlertActive(alertName string, includePending bool) (bool, error) {

	if ps.lastQueryTime.IsZero() || ps.lastQueryTime.Add(10*time.Second).After(time.Now()) {
		alerts, err := ps.getActiveAlerts()
		if err != nil {
			return false, err
		}

		ps.cachedAlerts = alerts
	}

	var foundAlert *v1.Alert
	for _, alert := range ps.cachedAlerts {
		if alert.State == v1.AlertStateFiring || (includePending && alert.State == v1.AlertStatePending) {
			if alert.Labels["alertname"] == model.LabelValue(alertName) {
				foundAlert = &alert
				break
			}
		}
	}

	return foundAlert != nil, nil
}

func (ps *PrometheusSource) getActiveAlerts() ([]v1.Alert, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	alerts, err := ps.p8sApi.Alerts(ctx)
	if err != nil {
		return nil, fmt.Errorf("error querying prometheus: %w", err)
	}

	return alerts.Alerts, nil
}
