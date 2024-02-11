//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectSet) DeepCopyInto(out *ProjectSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectSet.
func (in *ProjectSet) DeepCopy() *ProjectSet {
	if in == nil {
		return nil
	}
	out := new(ProjectSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProjectSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectSetList) DeepCopyInto(out *ProjectSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ProjectSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectSetList.
func (in *ProjectSetList) DeepCopy() *ProjectSetList {
	if in == nil {
		return nil
	}
	out := new(ProjectSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProjectSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectSetSpec) DeepCopyInto(out *ProjectSetSpec) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	in.ResourceQuota.DeepCopyInto(&out.ResourceQuota)
	in.LimitRange.DeepCopyInto(&out.LimitRange)
	if in.RoleRules != nil {
		in, out := &in.RoleRules, &out.RoleRules
		*out = make(map[string][]v1.PolicyRule, len(*in))
		for key, val := range *in {
			var outVal []v1.PolicyRule
			if val == nil {
				(*out)[key] = nil
			} else {
				in, out := &val, &outVal
				*out = make([]v1.PolicyRule, len(*in))
				for i := range *in {
					(*in)[i].DeepCopyInto(&(*out)[i])
				}
			}
			(*out)[key] = outVal
		}
	}
	if in.GroupPermissions != nil {
		in, out := &in.GroupPermissions, &out.GroupPermissions
		*out = make(map[string][]v1.Subject, len(*in))
		for key, val := range *in {
			var outVal []v1.Subject
			if val == nil {
				(*out)[key] = nil
			} else {
				in, out := &val, &outVal
				*out = make([]v1.Subject, len(*in))
				copy(*out, *in)
			}
			(*out)[key] = outVal
		}
	}
	if in.PolicySpec != nil {
		in, out := &in.PolicySpec, &out.PolicySpec
		*out = make(map[string]networkingv1.NetworkPolicySpec, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectSetSpec.
func (in *ProjectSetSpec) DeepCopy() *ProjectSetSpec {
	if in == nil {
		return nil
	}
	out := new(ProjectSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectSetStatus) DeepCopyInto(out *ProjectSetStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectSetStatus.
func (in *ProjectSetStatus) DeepCopy() *ProjectSetStatus {
	if in == nil {
		return nil
	}
	out := new(ProjectSetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectSetSync) DeepCopyInto(out *ProjectSetSync) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectSetSync.
func (in *ProjectSetSync) DeepCopy() *ProjectSetSync {
	if in == nil {
		return nil
	}
	out := new(ProjectSetSync)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProjectSetSync) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectSetSyncList) DeepCopyInto(out *ProjectSetSyncList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ProjectSetSync, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectSetSyncList.
func (in *ProjectSetSyncList) DeepCopy() *ProjectSetSyncList {
	if in == nil {
		return nil
	}
	out := new(ProjectSetSyncList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProjectSetSyncList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectSetSyncSpec) DeepCopyInto(out *ProjectSetSyncSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectSetSyncSpec.
func (in *ProjectSetSyncSpec) DeepCopy() *ProjectSetSyncSpec {
	if in == nil {
		return nil
	}
	out := new(ProjectSetSyncSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectSetSyncStatus) DeepCopyInto(out *ProjectSetSyncStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectSetSyncStatus.
func (in *ProjectSetSyncStatus) DeepCopy() *ProjectSetSyncStatus {
	if in == nil {
		return nil
	}
	out := new(ProjectSetSyncStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectSetTemplate) DeepCopyInto(out *ProjectSetTemplate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectSetTemplate.
func (in *ProjectSetTemplate) DeepCopy() *ProjectSetTemplate {
	if in == nil {
		return nil
	}
	out := new(ProjectSetTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProjectSetTemplate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectSetTemplateList) DeepCopyInto(out *ProjectSetTemplateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ProjectSetTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectSetTemplateList.
func (in *ProjectSetTemplateList) DeepCopy() *ProjectSetTemplateList {
	if in == nil {
		return nil
	}
	out := new(ProjectSetTemplateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProjectSetTemplateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectSetTemplateSpec) DeepCopyInto(out *ProjectSetTemplateSpec) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	in.ResourceQuota.DeepCopyInto(&out.ResourceQuota)
	in.LimitRange.DeepCopyInto(&out.LimitRange)
	if in.RoleRules != nil {
		in, out := &in.RoleRules, &out.RoleRules
		*out = make(map[string][]v1.PolicyRule, len(*in))
		for key, val := range *in {
			var outVal []v1.PolicyRule
			if val == nil {
				(*out)[key] = nil
			} else {
				in, out := &val, &outVal
				*out = make([]v1.PolicyRule, len(*in))
				for i := range *in {
					(*in)[i].DeepCopyInto(&(*out)[i])
				}
			}
			(*out)[key] = outVal
		}
	}
	if in.GroupPermissions != nil {
		in, out := &in.GroupPermissions, &out.GroupPermissions
		*out = make(map[string][]v1.Subject, len(*in))
		for key, val := range *in {
			var outVal []v1.Subject
			if val == nil {
				(*out)[key] = nil
			} else {
				in, out := &val, &outVal
				*out = make([]v1.Subject, len(*in))
				copy(*out, *in)
			}
			(*out)[key] = outVal
		}
	}
	if in.PolicySpec != nil {
		in, out := &in.PolicySpec, &out.PolicySpec
		*out = make(map[string]networkingv1.NetworkPolicySpec, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectSetTemplateSpec.
func (in *ProjectSetTemplateSpec) DeepCopy() *ProjectSetTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(ProjectSetTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectSetTemplateStatus) DeepCopyInto(out *ProjectSetTemplateStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectSetTemplateStatus.
func (in *ProjectSetTemplateStatus) DeepCopy() *ProjectSetTemplateStatus {
	if in == nil {
		return nil
	}
	out := new(ProjectSetTemplateStatus)
	in.DeepCopyInto(out)
	return out
}
