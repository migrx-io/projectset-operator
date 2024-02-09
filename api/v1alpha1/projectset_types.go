/*
Copyright 2024 Anatolii Makarov.

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
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ProjectSetSpec defines the desired state of ProjectSet
type ProjectSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Namespace name
	Namespace string `json:"namespace"`

	// ProjectSet templates
	Templates []string `json:"templates,omitempty"`

	// Custom namespace labels
	Labels map[string]string `json:"labels,omitempty"`

	// Custom namespace annotations
	Annotations map[string]string `json:"annotations,omitempty"`

	// ResourceQuota specification
	ResourceQuota corev1.ResourceQuotaSpec `json:"resourceQuota,omitempty"`

	// LimitRange specification
	LimitRange v1.LimitRangeSpec `json:"limitRange,omitempty"`

	// RBAC Role Rules
	RoleRules map[string][]rbacv1.PolicyRule `json:"roleRules,omitempty"`

	// User permissions
	GroupPermissions map[string][]rbacv1.Subject `json:"groupPermissions,omitempty"`

	// Network Policy specitifation
	PolicySpec map[string]networkingv1.NetworkPolicySpec `json:"policySpec,omitempty"`
}

// ProjectSetStatus defines the observed state of ProjectSet
type ProjectSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Conditions store the status conditions of the instances
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:singular=projectset
//+kubebuilder:resource:scope=Cluster,shortName=ps;prjs
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[-1:].type",description="The status"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ProjectSet is the Schema for the projectsets API
type ProjectSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectSetSpec   `json:"spec,omitempty"`
	Status ProjectSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ProjectSetList contains a list of ProjectSet
type ProjectSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProjectSet{}, &ProjectSetList{})
}
