package reconciler

import (
	"reflect"
	"testing"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/eventsource/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestServiceToKey(t *testing.T) {
	svc := v1alpha1.ServiceReference{
		Namespace: "ns",
		Name:      "svc",
	}

	assert.Equal(t, "ns/svc", ServiceToKey(svc))
}

func TestToNamespaceName(t *testing.T) {
	objectMeta := &metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "object",
	}

	assert.Equal(t, "namespace/object", ToNamespaceName(objectMeta).String())
}

func TestToKey(t *testing.T) {
	type args struct {
		objectMeta *metav1.ObjectMeta
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "has-namespace",
			args: args{
				objectMeta: &metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "object",
				},
			},
			want: "namespace/object",
		},
		{
			name: "no-namespace",
			args: args{
				objectMeta: &metav1.ObjectMeta{
					Name: "object",
				},
			},
			want: "object",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToKey(tt.args.objectMeta); got != tt.want {
				t.Errorf("ToKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSplitKey(t *testing.T) {
	type args struct {
		key string
	}
	tests := []struct {
		name string
		args args
		want types.NamespacedName
	}{
		{
			name: "no-namespace",
			args: args{key: "object"},
			want: types.NamespacedName{Name: "object"},
		},
		{
			name: "has-namespace",
			args: args{key: "namespace/object"},
			want: types.NamespacedName{Name: "object", Namespace: "namespace"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SplitKey(tt.args.key); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
