package countermeasure

import (
	"strings"

	v1alpha1 "github.com/dvilaverde/k8s-countermeasures/apis/countermeasure/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func ServiceToKey(svcRef *v1alpha1.ServiceReference) string {
	if svcRef.Namespace == "" {
		return svcRef.Name
	}

	return svcRef.Namespace + "/" + svcRef.Name
}

func ToNamespaceName(objectMeta *metav1.ObjectMeta) types.NamespacedName {
	return types.NamespacedName{
		Namespace: objectMeta.Namespace,
		Name:      objectMeta.Name,
	}
}

// ToKey will return a name from a ObjectMeta in the form of Namespace/Name. If
// no namespace is present then just Name.
func ToKey(objectMeta *metav1.ObjectMeta) string {
	if objectMeta.Namespace == "" {
		return objectMeta.Name
	}

	return objectMeta.Namespace + "/" + objectMeta.Name
}

// SplitKey will split the Key into a NamespaceName
func SplitKey(key string) types.NamespacedName {
	if strings.Contains(key, "/") {
		split := strings.Split(key, "/")
		return types.NamespacedName{
			Namespace: split[0],
			Name:      split[1],
		}
	}
	return types.NamespacedName{Name: key}
}
