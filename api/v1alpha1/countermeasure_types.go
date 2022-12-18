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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PrometheusMonitor definition of a monitor for a prometheus service in the K8s cluster
type PrometheusMonitor struct {
	Expression string                 `json:"expr"`
	Service    corev1.ObjectReference `json:"service"`
	Interval   metav1.Duration        `json:"interval,omitempty"`
	// TODO: support auth (basic and TLS) using secret ref
	// TODO: need a way to get the instance from the result of the expression,
	// 			maybe a resourceLabel property
}

// Operation a RFC 6902 patch operation
type Operation struct {
	Type  string          `json:"op"`
	Path  string          `json:"path"`
	From  string          `json:"from,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}

// Patch defines a patch operation on an existing Custom Resource
type Patch struct {
	Target    corev1.ObjectReference `json:"target"`
	Operation []Operation            `json:"operations"`
}

// Action defines an action to be taken when the monitor detects a condition that needs attention.
type Action struct {
	Name    string          `json:"name"`
	Command []string        `json:"command,omitempty"`
	JobSpec batchv1.JobSpec `json:"job,omitempty"`
	Patch   Patch           `json:"patch,omitempty"`
}

// CounterMeasureSpec defines the desired state of CounterMeasure
type CounterMeasureSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Prometheus PrometheusMonitor `json:"prometheus,omitempty"`
	Actions    []Action          `json:"actions"`
}

// CounterMeasureStatus defines the observed state of CounterMeasure
type CounterMeasureStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Conditions []metav1.Condition `json:"conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CounterMeasure is the Schema for the countermeasures API
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
