package actions

import (
	"reflect"
	"testing"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	"github.com/dvilaverde/k8s-countermeasures/pkg/events"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestRegistry_Create(t *testing.T) {
	reg := Registry{}
	reg.Initialize()

	spec := v1alpha1.Action{
		Delete: &v1alpha1.DeleteSpec{
			TargetObjectRef: v1alpha1.ObjectReference{
				Namespace:  "ns",
				Name:       "name",
				Kind:       "Pod",
				ApiVersion: "v1",
			},
		},
	}

	action, err := reg.Create(ActionContext{}, spec, false)
	assert.NotNil(t, action)
	assert.Nil(t, err)
	assert.Equal(t, reflect.TypeOf(&Delete{}), reflect.TypeOf(action))
}

func TestObjectKeyFromTemplate(t *testing.T) {
	type args struct {
		namespaceTemplate string
		nameTemplate      string
		event             events.Event
	}
	tests := []struct {
		name string
		args args
		want client.ObjectKey
	}{
		{
			name: "test",
			args: args{
				namespaceTemplate: "ns",
				nameTemplate:      "name",
				event:             events.Event{},
			},
			want: client.ObjectKey{
				Namespace: "ns",
				Name:      "name",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ObjectKeyFromTemplate(tt.args.namespaceTemplate, tt.args.nameTemplate, tt.args.event); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ObjectKeyFromTemplate() = %v, want %v", got, tt.want)
			}
		})
	}
}
