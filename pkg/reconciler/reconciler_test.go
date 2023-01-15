package reconciler

import (
	"reflect"
	"testing"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/eventsource/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
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

func Test_findNamedPort(t *testing.T) {
	type args struct {
		service   *corev1.Service
		namedPort string
	}
	tests := []struct {
		name  string
		args  args
		want  corev1.ServicePort
		want1 bool
	}{
		{
			name: "two ports",
			args: args{
				service: &corev1.Service{
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name: "foo",
								Port: 8080,
							},
							{
								Name: "web",
								Port: 8081,
							},
						},
					},
				},
				namedPort: "web",
			},
			want: corev1.ServicePort{
				Name: "web",
				Port: 8081,
			},
			want1: true,
		},
		{
			name: "one port",
			args: args{
				service: &corev1.Service{
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name: "foo",
								Port: 8080,
							},
						},
					},
				},
				namedPort: "web",
			},
			want: corev1.ServicePort{
				Name: "foo",
				Port: 8080,
			},
			want1: true,
		},
		{
			name: "zero ports",
			args: args{
				service: &corev1.Service{
					Spec: corev1.ServiceSpec{},
				},
				namedPort: "web",
			},
			want:  corev1.ServicePort{},
			want1: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := FindNamedPort(tt.args.service, tt.args.namedPort)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findNamedPort() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("findNamedPort() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
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
