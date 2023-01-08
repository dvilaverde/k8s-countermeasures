package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	ActionsTaken = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "countermeasures_actions_taken_total",
		Help: "Number of total actions the controller executed as a result of an event",
	}, []string{"namespace", "type"})

	ActionErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "countermeasures_action_errors_total",
		Help: "Number of total errors encountered while the controller attempted to execute an action",
	}, []string{"namespace", "type"})
)

func init() {
	metrics.Registry.MustRegister(ActionsTaken)
}
