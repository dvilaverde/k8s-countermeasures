package countermeasure

import (
	operatorv1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/actions"
	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/detect"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	log = ctrl.Log.WithName("monitor")
)

type CounterMeasureMonitor struct {
	detectors []detect.Detector
	client    client.Client

	monitored map[string]detect.CancelFunc
}

func NewMonitor(detectors []detect.Detector, client client.Client) *CounterMeasureMonitor {
	return &CounterMeasureMonitor{
		detectors: detectors,
		client:    client,
		monitored: make(map[string]detect.CancelFunc),
	}
}

// StartMonitoring will start monitoring a resource for events that require action
func (c *CounterMeasureMonitor) StartMonitoring(countermeasure *operatorv1alpha1.CounterMeasure) error {

	found := false

	for _, detect := range c.detectors {
		if detect.Supports(&countermeasure.Spec) {
			nsName := ToNamespaceName(&countermeasure.ObjectMeta)

			cancel, err := detect.NotifyOn(*countermeasure, actions.NewDeleteAction(c.client))
			if err != nil {
				return err
			}

			found = true
			c.monitored[nsName.String()] = cancel
			break
		}
	}

	if !found {
		log.Error(nil, "could not find a supported detector")
	}

	return nil
}

func (c *CounterMeasureMonitor) StopMonitoring(key types.NamespacedName) error {

	if cancel, ok := c.monitored[key.String()]; ok {
		cancel()
	}

	return nil
}
