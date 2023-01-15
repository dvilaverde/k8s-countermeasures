package manager

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type ObjectKey struct {
	types.NamespacedName
	Generation int64
}

type Manager[T any] interface {
	Add(T) error
	Remove(types.NamespacedName) error
	Exists(metav1.ObjectMeta) bool
}

func (k ObjectKey) GetName() string {
	return k.Namespace + "/" + k.Name
}
