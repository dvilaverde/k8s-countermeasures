package manager

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestObjectKey_GetName(t *testing.T) {
	type fields struct {
		NamespacedName types.NamespacedName
		Generation     int64
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "test1",
			fields: fields{
				NamespacedName: types.NamespacedName{Namespace: "ns", Name: "name"},
				Generation:     1,
			},
			want: "ns/name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := ObjectKey{
				NamespacedName: tt.fields.NamespacedName,
				Generation:     tt.fields.Generation,
			}
			if got := k.GetName(); got != tt.want {
				t.Errorf("ObjectKey.GetName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToKey(t *testing.T) {
	type args struct {
		meta metav1.ObjectMeta
	}
	tests := []struct {
		name string
		args args
		want ObjectKey
	}{
		{
			name: "testkey",
			args: args{
				meta: metav1.ObjectMeta{
					Name:       "name",
					Namespace:  "ns",
					Generation: 1,
				},
			},
			want: ObjectKey{
				NamespacedName: types.NamespacedName{Namespace: "ns", Name: "name"},
				Generation:     1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToKey(tt.args.meta); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ToKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
