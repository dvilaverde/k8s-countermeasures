package prometheus

import (
	"context"
	"fmt"
	"time"

	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/events"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
)

type AlertNotFiring struct {
	msg string
}

func (nf *AlertNotFiring) Error() string {
	if len(nf.msg) == 0 {
		return "alert not firing"
	}
	return nf.msg
}

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

// ToEvents get the Events for the alert name.
func (r *AlertQueryResult) ToEvents(alertName string, includePending bool) ([]events.Event, error) {
	foundAlerts := r.findActiveAlert(alertName, includePending)
	if len(foundAlerts) == 0 {
		return nil, &AlertNotFiring{msg: fmt.Sprintf("alert %v is not firing (or pending)", alertName)}
	}

	eventsArr := make([]events.Event, len(foundAlerts))
	for idx, alert := range foundAlerts {

		labels := make(map[string]string, len(alert.Labels))
		for label, value := range alert.Labels {
			labels[string(label)] = string(value)
		}

		event := events.Event{
			Name:       string(alert.Labels["alertname"]),
			ActiveTime: alert.ActiveAt,
			Data:       labels,
		}

		eventsArr[idx] = event
	}

	return eventsArr, nil
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
