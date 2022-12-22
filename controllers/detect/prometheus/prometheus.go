package prometheus

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type Builder func(string) (*PrometheusService, error)

type PrometheusService struct {
	p8sApi v1.API
}

type AlertQueryResult struct {
	alerts []v1.Alert
}

func NewPrometheusService(api v1.API) *PrometheusService {

	return &PrometheusService{
		p8sApi: api,
	}
}

func NewPrometheusClient(address string) (*PrometheusService, error) {
	client, err := api.NewClient(api.Config{
		Address: address,
	})

	if err != nil {
		return nil, fmt.Errorf("error creating client, %w", err)
	}

	return NewPrometheusService(v1.NewAPI(client)), nil
}

// IsAlertActive returns true if the named alert is currently 'firing', or 'pending' if enabled.
func (r *AlertQueryResult) IsAlertActive(alertName string, includePending bool) bool {
	foundAlert := r.findActiveAlert(alertName, includePending)
	return foundAlert != nil
}

func (r *AlertQueryResult) GetActiveAlertLabels(alertName string, includePending bool) (map[string]string, error) {
	foundAlert := r.findActiveAlert(alertName, includePending)
	if foundAlert == nil {
		return nil, fmt.Errorf("alert %v is not firing (or pending)", alertName)
	}

	labels := make(map[string]string, len(foundAlert.Labels))
	for label, value := range foundAlert.Labels {
		labels[string(label)] = string(value)
	}

	return labels, nil
}

// findActiveAlert returns an alert if it is firing (or pending), but not inactive
func (r *AlertQueryResult) findActiveAlert(alertName string, includePending bool) *v1.Alert {
	var foundAlert *v1.Alert
	for _, alert := range r.alerts {
		if alert.State == v1.AlertStateFiring || (includePending && alert.State == v1.AlertStatePending) {
			if alert.Labels["alertname"] == model.LabelValue(alertName) {
				foundAlert = &alert
				break
			}
		}
	}

	return foundAlert
}

func (ps *PrometheusService) GetActiveAlerts() (AlertQueryResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	alerts, err := ps.p8sApi.Alerts(ctx)
	if err != nil {
		return AlertQueryResult{}, fmt.Errorf("error querying prometheus: %w", err)
	}

	return AlertQueryResult{alerts: alerts.Alerts}, nil
}
