package countermeasure

import (
	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/actions"
	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/detect"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	log = ctrl.Log.WithName("monitor")
)

type counterMeasureHandle struct {
	cancelFunc detect.CancelFunc
	generation int64
}

type CounterMeasureMonitor struct {
	detectors []detect.Detector
	mgr       manager.Manager

	monitored map[string]counterMeasureHandle
}

func NewMonitor(detectors []detect.Detector, mgr manager.Manager) *CounterMeasureMonitor {
	return &CounterMeasureMonitor{
		detectors: detectors,
		mgr:       mgr,
		monitored: make(map[string]counterMeasureHandle),
	}
}

func (c *CounterMeasureMonitor) IsAlreadyMonitored(cm *v1alpha1.CounterMeasure) bool {
	nsName := ToNamespaceName(&cm.ObjectMeta)
	// if the generation hasn't changed from what we're monitoring then short return
	if handle, ok := c.monitored[nsName.String()]; ok {
		if handle.generation == cm.Generation {
			return true
		}
	}

	return false
}

// StartMonitoring will start monitoring a resource for events that require action
func (c *CounterMeasureMonitor) StartMonitoring(countermeasure *v1alpha1.CounterMeasure) error {
	// if the generation hasn't changed from what we're monitoring then short return
	if c.IsAlreadyMonitored(countermeasure) {
		return nil
	}

	found := false
	nsName := ToNamespaceName(&countermeasure.ObjectMeta)
	for _, detect := range c.detectors {
		if detect.Supports(&countermeasure.Spec) {
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
			c.monitored[nsName.String()] = counterMeasureHandle{
				cancelFunc: cancel,
				generation: countermeasure.Generation,
			}

			break
		}
	}

	if !found {
		log.Error(nil, "could not find a supported detector")
	}

	return nil
}

func (c *CounterMeasureMonitor) StopMonitoring(key types.NamespacedName) error {

	if handle, ok := c.monitored[key.String()]; ok {
		handle.cancelFunc()
		// delete the key from this monitored map
		delete(c.monitored, key.String())

		log.Info("stopped monitoring countermeasure", "name", key.Name, "namespace", key.Namespace)
	}

	return nil
}
