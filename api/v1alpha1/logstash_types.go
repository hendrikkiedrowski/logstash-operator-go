/*
Copyright 2021.

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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type StorageSpec struct {
	//+kubebuilder:default:=standard
	StorageClassName string `json:"storageClassName,omitempty"`
	//+kubebuilder:default:="1Gi"
	Size resource.Quantity `json:"size,omitempty"`
	//+kubebuilder:default:={ReadWriteOnce}
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
}

// LogstashInputService defines the desired state of LogstashInputService
type LogstashInputService struct {
	Name string `json:"name"`
	Port int    `json:"port"`
	Type string `json:"type"`
}

// LogstashInputSpec defines the desired state of LogstashInput
type LogstashInputSpec struct {
	Service LogstashInputService `json:"service"`
	Data    string               `json:"data"`
}

// LogstashInput is the Schema for the logstashinputs API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type LogstashInput struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec LogstashInputSpec `json:"spec,omitempty"`
}

// LogstashInputList contains a list of LogstashInput
// +kubebuilder:object:root=true
type LogstashInputList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LogstashInput `json:"items"`
}

// LogstashFilterSpec defines the desired state of LogstashFilter
type LogstashFilterSpec struct {
	// +kubebuilder:default:=50
	Order int    `json:"order"`
	Data  string `json:"data"`
}

// LogstashFilter is the Schema for the logstashfilters API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type LogstashFilter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec LogstashFilterSpec `json:"spec,omitempty"`
}

// LogstashFilterList contains a list of LogstashFilter
// +kubebuilder:object:root=true
type LogstashFilterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LogstashFilter `json:"items"`
}

// LogstashOutputSpec defines the desired state of LogstashOutput
type LogstashOutputSpec struct {
	Data string `json:"data"`
}

// LogstashOutput is the Schema for the logstashinputs API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type LogstashOutput struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec LogstashOutputSpec `json:"spec,omitempty"`
}

// LogstashOutputList contains a list of LogstashOutput
// +kubebuilder:object:root=true
type LogstashOutputList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LogstashOutput `json:"items"`
}

// LogstashPipelineSpec defines the desired state of LogstashPipeline
type LogstashPipelineSpec struct {
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// +kubebuilder:default:="input {\n  beats {\n    port => 5044\n  }\n}\noutput {\n  stdout { codec => rubydebug }\n}"
	//Config string `json:"config"`
}

// LogstashPipeline is the Schema for the logstashpipelines API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type LogstashPipeline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec LogstashPipelineSpec `json:"spec,omitempty"`
}

// LogstashPipelineList contains a list of LogstashPipeline
// +kubebuilder:object:root=true
type LogstashPipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LogstashPipeline `json:"items"`
}

// LogstashSpec defines the desired state of Logstash
type LogstashSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ReplicaCount specifies how many replicas we want.
	//+kubebuilder:default:=1
	ReplicaCount int32       `json:"replicaCount,omitempty"`
	Storage      StorageSpec `json:"storage"`
}

// LogstashStatus defines the observed state of Logstash
type LogstashStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Nodes shows the nodes that are used.
	Nodes []string `json:"nodes"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Logstash is the Schema for the logstashes API
type Logstash struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LogstashSpec   `json:"spec,omitempty"`
	Status LogstashStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LogstashList contains a list of Logstash
type LogstashList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Logstash `json:"items"`
}

func init() {
	SchemeBuilder.Register(
		&Logstash{},
		&LogstashList{},
		&LogstashPipeline{},
		&LogstashPipelineList{},
		&LogstashInput{},
		&LogstashInputList{},
		&LogstashOutput{},
		&LogstashOutputList{},
		&LogstashFilter{},
		&LogstashFilterList{},
	)
}
