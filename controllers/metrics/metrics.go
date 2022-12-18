package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	JobsCreatedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "countermeasures_jobs_created_total",
			Help: "Number of total jobs the controller has created",
		},
	)
)

func init() {
	metrics.Registry.MustRegister(JobsCreatedTotal)
}
