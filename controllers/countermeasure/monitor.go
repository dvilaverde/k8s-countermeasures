package countermeasure

import (
	operatorv1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/actions"
	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/detect"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	log = ctrl.Log.WithName("monitor")
)

type CounterMeasureMonitor struct {
	detectors []detect.Detector
	mgr       manager.Manager

	monitored map[string]detect.CancelFunc
}

func NewMonitor(detectors []detect.Detector, mgr manager.Manager) *CounterMeasureMonitor {
	return &CounterMeasureMonitor{
		detectors: detectors,
		mgr:       mgr,
		monitored: make(map[string]detect.CancelFunc),
	}
}

// StartMonitoring will start monitoring a resource for events that require action
func (c *CounterMeasureMonitor) StartMonitoring(countermeasure *operatorv1alpha1.CounterMeasure) error {

	found := false

	for _, detect := range c.detectors {
		if detect.Supports(&countermeasure.Spec) {
			nsName := ToNamespaceName(&countermeasure.ObjectMeta)

			// TODO invert the control here to have the Actions register with the monitor
			// and use the mgr to create the action like mgr.NewAction(countermeasure)
			handler, err := actions.CounterMeasureToActions(countermeasure, c.mgr)
			if err != nil {
				return err
			}

			cancel, err := detect.NotifyOn(*countermeasure, handler)
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
