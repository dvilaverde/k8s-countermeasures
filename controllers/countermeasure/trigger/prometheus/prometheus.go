package prometheus

import (
	"context"
	"fmt"
	"time"

	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/trigger"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
)

type Builder func(string, string, string) (*PrometheusService, error)

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

func NewPrometheusClient(address, username, password string) (*PrometheusService, error) {
	clientConfig := api.Config{
		Address: address,
	}

	if len(username) > 0 {
		clientConfig.RoundTripper = config.NewBasicAuthRoundTripper(username,
			config.Secret(password), "",
			api.DefaultRoundTripper)
	}

	client, err := api.NewClient(clientConfig)

	if err != nil {
		return nil, fmt.Errorf("error creating client, %w", err)
	}

	return NewPrometheusService(v1.NewAPI(client)), nil
}

// IsAlertActive returns true if the named alert is currently 'firing', or 'pending' if enabled.
func (r *AlertQueryResult) IsAlertActive(alertName string, includePending bool) bool {
	foundAlerts := r.findActiveAlert(alertName, includePending)
	return len(foundAlerts) > 0
}

func (r *AlertQueryResult) GetActiveAlertLabels(alertName string, includePending bool) ([]trigger.InstanceLabels, error) {
	foundAlerts := r.findActiveAlert(alertName, includePending)
	if len(foundAlerts) == 0 {
		return nil, fmt.Errorf("alert %v is not firing (or pending)", alertName)
	}

	instances := make([]trigger.InstanceLabels, len(foundAlerts))
	for idx, alert := range foundAlerts {
		labels := make(map[string]string, len(alert.Labels))
		for label, value := range alert.Labels {
			labels[string(label)] = string(value)
		}
		instances[idx] = labels
	}

	return instances, nil
}

// findActiveAlert returns an alert if it is firing (or pending), but not inactive
func (r *AlertQueryResult) findActiveAlert(alertName string, includePending bool) []v1.Alert {
	foundAlerts := make([]v1.Alert, 0)
	for _, alert := range r.alerts {
		if alert.State == v1.AlertStateFiring || (includePending && alert.State == v1.AlertStatePending) {
			if alert.Labels["alertname"] == model.LabelValue(alertName) {
				foundAlerts = append(foundAlerts, alert)
			}
		}
	}

	return foundAlerts
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
