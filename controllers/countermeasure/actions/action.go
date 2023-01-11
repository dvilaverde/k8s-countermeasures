package actions

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"text/template"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/events"
	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var log = ctrl.Log.WithName("actions")

type Registry struct {
	builders map[reflect.Type]ActionBuilder
}

type Action interface {
	Perform(context.Context, events.Event) (bool, error)
	GetName() string
	GetType() string
	GetTargetObjectName(events.Event) string
}

type ActionHandlerSequence struct {
	actions        []Action
	recorder       record.EventRecorder
	countermeasure *v1alpha1.CounterMeasure
	mutex          sync.Mutex
}

type BaseAction struct {
	DryRun bool
	Name   string
	client client.Client
}

type Builder interface {
	GetClient() client.Client
	GetRestConfig() *rest.Config
	GetRecorder() record.EventRecorder
}
type ActionBuilder func(v1alpha1.Action, Builder, bool) Action

func (b *BaseAction) GetName() string {
	return b.Name
}

// createObjectName evaluate the template (if any) in name and namespace to produce an object name.
func (b *BaseAction) createObjectName(kind, namespace, name string, data events.Event) string {
	return fmt.Sprintf("%s: '%s/%s'", strings.ToLower(kind),
		evaluateTemplate(namespace, data),
		evaluateTemplate(name, data))
}

// Initialize registers all the known actions with the registry
func (r *Registry) Initialize() {
	r.RegisterAction(v1alpha1.DeleteSpec{}, func(spec v1alpha1.Action, b Builder, dryRun bool) Action {
		return NewDeleteFromBase(BaseAction{client: b.GetClient(), DryRun: dryRun, Name: spec.Name}, *spec.Delete)
	})

	r.RegisterAction(v1alpha1.DebugSpec{}, func(spec v1alpha1.Action, b Builder, dryRun bool) Action {
		cs, _ := kubernetes.NewForConfig(b.GetRestConfig())
		// TODO handle ignored error
		return NewDebugFromBase(BaseAction{client: b.GetClient(), DryRun: dryRun, Name: spec.Name}, cs.CoreV1(), *spec.Debug)
	})

	r.RegisterAction(v1alpha1.PatchSpec{}, func(spec v1alpha1.Action, b Builder, dryRun bool) Action {
		return NewPatchFromBase(BaseAction{client: b.GetClient(), DryRun: dryRun, Name: spec.Name}, *spec.Patch)
	})

	r.RegisterAction(v1alpha1.RestartSpec{}, func(spec v1alpha1.Action, b Builder, dryRun bool) Action {
		return NewRestartFromBase(BaseAction{client: b.GetClient(), DryRun: dryRun, Name: spec.Name}, *spec.Restart)
	})
}

// RegisterAction register a new action with the registry
func (r *Registry) RegisterAction(prototype interface{}, builder ActionBuilder) {

	if r.builders == nil {
		r.builders = make(map[reflect.Type]ActionBuilder)
	}

	r.builders[reflect.TypeOf(prototype)] = builder
}

// Create creates an implementation of an action defined in the Action spec
func (r *Registry) Create(builderArgs Builder, action v1alpha1.Action, dryRun bool) (Action, error) {
	reflectType := reflect.ValueOf(action)
	for i := 0; i < reflectType.NumField(); i++ {
		valueField := reflectType.Field(i)
		typeField := reflectType.Type().Field(i)
		if valueField.Kind() == reflect.Pointer && !valueField.IsNil() {
			if builder, ok := r.builders[typeField.Type.Elem()]; ok {
				return builder(action, builderArgs, dryRun), nil
			}
		}
	}

	return nil, fmt.Errorf("action '%s' is either mis-configured or using an unknown action type", action.Name)
}

// ConvertToHandler converts a countermeasure and all actions within into a handler for source events.
func (r *Registry) ConvertToHandler(countermeasure *v1alpha1.CounterMeasure, builder Builder) (*ActionHandlerSequence, error) {
	seq := &ActionHandlerSequence{
		actions:        make([]Action, 0),
		recorder:       builder.GetRecorder(),
		countermeasure: countermeasure,
	}

	for _, action := range countermeasure.Spec.Actions {
		actionImpl, err := r.Create(builder, action, countermeasure.Spec.DryRun)
		if err != nil {
			return seq, err
		}

		seq.actions = append(seq.actions, actionImpl)
	}

	return seq, nil
}

// ObjectKeyFromTemplate create a client.ObjectKey from a namespace and name template.
func ObjectKeyFromTemplate(namespaceTemplate, nameTemplate string, event events.Event) client.ObjectKey {
	return client.ObjectKey{
		Namespace: evaluateTemplate(namespaceTemplate, event),
		Name:      evaluateTemplate(nameTemplate, event),
	}
}

func evaluateTemplate(templateString string, event events.Event) string {
	tmpl := template.Must(template.New("").Parse(templateString))
	var buf bytes.Buffer
	tmpl.Execute(&buf, event)
	return buf.String()
}

// OnDetection called when an alert condition is detected.
func (seq *ActionHandlerSequence) OnDetection(ns types.NamespacedName, events []events.Event, eventDone chan<- string) {

	seq.mutex.Lock()
	defer seq.mutex.Unlock()

	// in the event of errors close the done channel to avoid goroutine leak
	defer func() {
		close(eventDone)
	}()

	cm := seq.countermeasure
	for _, event := range events {
		// create a struct that will be used as data for the templates in the custom resource

		ctx := context.Background()
		for _, action := range seq.actions {
			labels := prometheus.Labels{"namespace": ns.Namespace, "type": action.GetType()}
			ok, err := action.Perform(ctx, event)

			// TODO: introduce some retrying logic here
			if err != nil {
				metrics.ActionErrors.With(labels).Add(1)
				seq.recorder.Event(cm, "Warning", "ActionError", err.Error())
				log.Error(err, "action execution error", "name", ns.Name, "namespace", ns.Namespace)
				break
			}

			if ok {
				metrics.ActionsTaken.With(labels).Add(1)
				msg := fmt.Sprintf("Alert detected, action '%s' taken on %s",
					action.GetName(),
					action.GetTargetObjectName(event))
				if cm.Spec.DryRun {
					msg = fmt.Sprintf("%s. DryRun=true", msg)
				}

				seq.recorder.Event(cm, "Normal", "AlertFired", msg)
			}
		}

		eventDone <- event.Key()
	}
}
