/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ServiceReference struct {
	// `namespace` is the namespace of the service.
	Namespace string `json:"namespace"`
	// `name` is the name of the service.
	Name string `json:"name"`
	// `path` is an optional URL path which will be sent in any request to
	// this service.
	// +optional
	Path *string `json:"path,omitempty"`
	// `port` should be a valid port number (1-65535, inclusive).
	// +optional
	Port int32 `json:"port,omitempty"`
	// `targetPort` should be a valid name of a port in the target service.
	// +optional
	TargetPort string `json:"targetPort,omitempty"`
	// `useTls` true if the HTTPS endpoint should be used.
	// +optional
	UseTls bool `json:"useTls,omitempty"`
}

// AuthSpec Spec for references to secrets
type AuthSpec struct {
	SecretReference corev1.SecretReference `json:"secretRef"`
}

// PrometheusSpec defines the desired state of Prometheus
type PrometheusSpec struct {
	Service ServiceReference `json:"service"`
	// Defines a Kubernetes secret with a type indicating the authentication scheme
	// for example the type: 'kubernetes.io/basic-auth' indicates basic auth credentials
	// to prometheus.
	Auth *AuthSpec `json:"auth,omitempty"`

	PollingInterval int32 `json:"pollingInterval"`
	IncludePending  bool  `json:"includePending"`
}

// PrometheusStatus defines the observed state of Prometheus
type PrometheusStatus struct {
	State      StateType          `json:"state,omitempty"`
	Conditions []metav1.Condition `json:"conditions"`
}

type StateType string

const (
	Monitoring StateType = "Polling"
	Unknown    StateType = "Unknown"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Polling Interval",type=integer,JSONPath=`.spec.pollingInterval`
// +kubebuilder:printcolumn:name="Include Pending",type=boolean,JSONPath=`.spec.includePending`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.state`
// +kubebuilder:resource:shortName=pes
// Prometheus is the Schema for the prometheuses API
type Prometheus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PrometheusSpec   `json:"spec,omitempty"`
	Status PrometheusStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PrometheusList contains a list of Prometheus
type PrometheusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Prometheus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Prometheus{}, &PrometheusList{})
}

// GetNamespacedName get the NamespacedName of the Service
func (s *ServiceReference) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: s.Namespace,
		Name:      s.Name,
	}
}
