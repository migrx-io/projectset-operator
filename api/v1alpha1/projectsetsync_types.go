/*
Copyright 2024.

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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ProjectSetSyncSpec defines the desired state of ProjectSetSync
type ProjectSetSyncSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Git repo URL
	GitRepo string `json:"gitRepo"`

	// Env name for cluster
	EnvName string `json:"envName"`

	// Get branch name, default main
	// +kubebuilder:default=main
	GitBranch string `json:"gitBranch,omitempty"`

	// Sync interval in sec, default 10
	// +kubebuilder:default=10
	SyncSecInterval int `json:"syncSecInterval,omitempty"`

	// Path to config file default projectsets.yaml
	// +kubebuilder:default=projectsets.yaml
	ConfFile string `json:"confFile,omitempty"`
}

// ProjectSetSyncStatus defines the observed state of ProjectSetSync
type ProjectSetSyncStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Conditions store the status conditions of the instances
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:singular=projectsetsync
//+kubebuilder:resource:scope=Cluster,shortName=pss;prjssync
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[-1:].type",description="The status"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ProjectSetSync is the Schema for the projectsetsyncs API
type ProjectSetSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectSetSyncSpec   `json:"spec,omitempty"`
	Status ProjectSetSyncStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ProjectSetSyncList contains a list of ProjectSetSync
type ProjectSetSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectSetSync `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProjectSetSync{}, &ProjectSetSyncList{})
}
