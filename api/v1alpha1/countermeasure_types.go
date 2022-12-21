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
	json "encoding/json"

	batchv1 "k8s.io/api/batch/v1"
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
}

// GetNamespacedName get the NamespacedName of the Service
func (s *ServiceReference) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: s.Namespace,
		Name:      s.Name,
	}
}

// Prometheus definition of a monitor for a prometheus service in the K8s cluster
type PrometheusSpec struct {
	Service  *ServiceReference `json:"service"`
	Interval metav1.Duration   `json:"interval,omitempty"`
	// TODO: support auth (basic and TLS) using secret ref
	// TODO: need a way to get the instance from the result of the expression,
	// 			maybe a resourceLabel property

	Alert *PrometheusAlertSpec `json:"alert,omitempty"`
}

// PrometheusAlertSpec definition of a monitored prometheus alert
type PrometheusAlertSpec struct {
	AlertName      string `json:"name"`
	IncludePending bool   `json:"includePending,omitempty"`
}

// CommandSpec command and arguments to execute in a container
type CommandSpec []string

// Operation a RFC 6902 patch operation
type Operation struct {
	Type  string          `json:"op"`
	Path  string          `json:"path"`
	From  string          `json:"from,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}

// PatchSpec defines a patch operation on an existing Custom Resource
type PatchSpec struct {
	Target    corev1.ObjectReference `json:"target"`
	Operation []Operation            `json:"operations"`
}

// Action defines an action to be taken when the monitor detects a condition that needs attention.
type Action struct {
	Name string `json:"name"`
	// +kubebuilder:validation:Optional
	Command CommandSpec `json:"command,omitempty"`

	// +kubebuilder:validation:Optional
	JobSpec *batchv1.JobTemplateSpec `json:"job,omitempty"`

	// +kubebuilder:validation:Optional
	Patch *PatchSpec `json:"patch,omitempty"`
}

// CounterMeasureSpec defines the desired state of CounterMeasure
type CounterMeasureSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Prometheus *PrometheusSpec `json:"prometheus,omitempty"`
	Actions    []Action        `json:"actions"`
}

// CounterMeasureStatus defines the observed state of CounterMeasure
type CounterMeasureStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	LastObservation     LastObservationType `json:"lastObservation,omitempty"`
	LastObservationTime *metav1.Time        `json:"lastObservationTime,omitempty"`

	Conditions []metav1.Condition `json:"conditions"`
}

type LastObservationType string

const (
	Monitoring LastObservationType = "Monitoring"
	Applying   LastObservationType = "Applying"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CounterMeasure is the Schema for the countermeasures API
// +kubebuilder:printcolumn:name="Observation",type=string,JSONPath=`.status.lastObservation`
// +kubebuilder:printcolumn:name="Observation Time",type=string,JSONPath=`.status.lastObservationTime`
type CounterMeasure struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CounterMeasureSpec   `json:"spec,omitempty"`
	Status CounterMeasureStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CounterMeasureList contains a list of CounterMeasure
type CounterMeasureList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CounterMeasure `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CounterMeasure{}, &CounterMeasureList{})
}
