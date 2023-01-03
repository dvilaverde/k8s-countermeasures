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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

const (
	ReasonSucceeded            = "Succeeded"
	ReasonReconciling          = "Reconciling"
	ReasonResourceNotAvailable = "ResourceNotAvailable"

	TypeMonitoring = "Monitoring"
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

type ObjectReference struct {
	// `namespace` is the namespace of the object.
	Namespace string `json:"namespace"`
	// `name` is the name of the object.
	Name string `json:"name"`
	// `kind` is the type of object
	Kind string `json:"kind"`
	// `apiVersion` is the version of the object
	ApiVersion string `json:"apiVersion"`
}

type PodReference struct {
	// `namespace` is the namespace of the pod.
	Namespace string `json:"namespace"`
	// `name` is the name of the pod.
	Name string `json:"name"`
	// `container` is the name a container in a pod.
	Container string `json:"container,omitempty"`
}

type DeploymentReference struct {
	// `namespace` is the namespace of the deployment.
	Namespace string `json:"namespace"`
	// `name` is the name of the deployment.
	Name string `json:"name"`
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
	Service *ServiceReference `json:"service"`
	// TODO: support auth (basic and TLS) using secret ref

	Alert *PrometheusAlertSpec `json:"alert,omitempty"`
}

// PrometheusAlertSpec definition of a monitored prometheus alert
type PrometheusAlertSpec struct {
	AlertName      string `json:"name"`
	IncludePending bool   `json:"includePending,omitempty"`
}

// DebugSpec Patches a pod with an ephemeral container that can be used to troubleshoot
type DebugSpec struct {
	Name    string       `json:"name,omitempty"`
	Command []string     `json:"command,omitempty"`
	Args    []string     `json:"args,omitempty"`
	Image   string       `json:"image"`
	PodRef  PodReference `json:"podRef"`
	StdIn   bool         `json:"stdin,omitempty"`
	TTY     bool         `json:"tty,omitempty"`
}

// PatchSpec defines a patch operation on an existing Custom Resource
type PatchSpec struct {
	TargetObjectRef ObjectReference `json:"targetObjectRef"`
	PatchType       types.PatchType `json:"patchType"`
	YAMLTemplate    string          `json:"yamlTemplate"`
}

type DeleteSpec struct {
	TargetObjectRef ObjectReference `json:"targetObjectRef"`
}

type RestartSpec struct {
	DeploymentRef DeploymentReference `json:"deploymentRef"`
}

func (o ObjectReference) ToGroupVersionKind() (schema.GroupVersionKind, error) {
	gv, err := schema.ParseGroupVersion(o.ApiVersion)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}

	return gv.WithKind(o.Kind), nil
}

// Action defines an action to be taken when the monitor detects a condition that needs attention.
type Action struct {
	Name string `json:"name"`

	// TODO: Add the following low-level operations:
	// Create *CreateSpec `json:"create,omitempty"`

	// +kubebuilder:validation:Optional
	Delete *DeleteSpec `json:"delete,omitempty"`
	// +kubebuilder:validation:Optional
	Patch *PatchSpec `json:"patch,omitempty"`

	// The following specs are high level operations for convienence.
	//
	// +kubebuilder:validation:Optional
	Debug *DebugSpec `json:"debug,omitempty"`
	// +kubebuilder:validation:Optional
	Restart *RestartSpec `json:"restart,omitempty"`
}

// CounterMeasureSpec defines the desired state of CounterMeasure
type CounterMeasureSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Prometheus *PrometheusSpec `json:"prometheus,omitempty"`
	Actions    []Action        `json:"actions"`
	// +kubebuilder:default=false
	DryRun bool `json:"dryRun,omitempty"`
}

// CounterMeasureStatus defines the observed state of CounterMeasure
type CounterMeasureStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	LastStatus           StatusType   `json:"lastStatus,omitempty"`
	LastStatusChangeTime *metav1.Time `json:"lastStatusChangeTime,omitempty"`

	Conditions []metav1.Condition `json:"conditions"`
}

type StatusType string

const (
	Monitoring StatusType = "Monitoring"
	Applying   StatusType = "Applying"
	Error      StatusType = "Error"
	Unknown    StatusType = "Unknown"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CounterMeasure is the Schema for the countermeasures API
// +kubebuilder:printcolumn:name="Dry Run",type=boolean,JSONPath=`.spec.dryRun`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.lastStatus`
// +kubebuilder:printcolumn:name="Status Last Changed",type=string,JSONPath=`.status.lastStatusChangeTime`
// +kubebuilder:resource:shortName=ctm
// +kubebuilder:singular=countermeasure
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
