package actions

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"text/template"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/controllers/countermeasure/trigger"
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
	Perform(context.Context, ActionData) error
	GetName() string
	GetTargetObjectName(ActionData) string
}

type ActionData struct {
	Labels map[string]string
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
func (b *BaseAction) createObjectName(kind, namespace, name string, data ActionData) string {
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

// ConvertToHandler converts a countermeasure and all actions within into a handler for trigger events.
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
func ObjectKeyFromTemplate(namespaceTemplate, nameTemplate string, data ActionData) client.ObjectKey {
	return client.ObjectKey{
		Namespace: evaluateTemplate(namespaceTemplate, data),
		Name:      evaluateTemplate(nameTemplate, data),
	}
}

func evaluateTemplate(templateString string, data ActionData) string {
	tmpl := template.Must(template.New("").Parse(templateString))
	var buf bytes.Buffer
	tmpl.Execute(&buf, data)
	return buf.String()
}

// OnDetection called when an alert condition is detected.
func (seq *ActionHandlerSequence) OnDetection(ns types.NamespacedName, labels []trigger.InstanceLabels) {

	seq.mutex.Lock()
	defer seq.mutex.Unlock()

	cm := seq.countermeasure
	for _, instLabels := range labels {
		// create a struct that will be used as data for the templates in the custom resource
		actionData := ActionData{
			Labels: instLabels,
		}

		ctx := context.Background()
		for _, action := range seq.actions {
			err := action.Perform(ctx, actionData)

			// TODO: introduce some retrying logic here
			if err != nil {
				seq.recorder.Event(cm, "Warning", "ActionError", err.Error())
				log.Error(err, "action execution error", "name", ns.Name, "namespace", ns.Namespace)
				break
			}

			msg := fmt.Sprintf("Alert detected, action '%s' taken on %s",
				action.GetName(),
				action.GetTargetObjectName(actionData))
			if cm.Spec.DryRun {
				msg = fmt.Sprintf("%s. DryRun=true", msg)
			}

			seq.recorder.Event(cm, "Normal", "AlertFired", msg)
		}
	}
}
