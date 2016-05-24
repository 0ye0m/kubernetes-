// +build !ignore_autogenerated

/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

// This file was autogenerated by deepcopy-gen. Do not edit it manually!

package v1

import (
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	api_v1 "k8s.io/kubernetes/pkg/api/v1"
	conversion "k8s.io/kubernetes/pkg/conversion"
	runtime "k8s.io/kubernetes/pkg/runtime"
)

func init() {
	SchemeBuilder.Register(func(scheme *runtime.Scheme) {
		if err := scheme.AddGeneratedDeepCopyFuncs(
			DeepCopy_v1_Job,
			DeepCopy_v1_JobCondition,
			DeepCopy_v1_JobList,
			DeepCopy_v1_JobSpec,
			DeepCopy_v1_JobStatus,
			DeepCopy_v1_LabelSelector,
			DeepCopy_v1_LabelSelectorRequirement,
		); err != nil {
			// if one of the deep copy functions is malformed, detect it immediately.
			panic(err)
		}
	})
}

func DeepCopy_v1_Job(in Job, out *Job, c *conversion.Cloner) error {
	if err := unversioned.DeepCopy_unversioned_TypeMeta(in.TypeMeta, &out.TypeMeta, c); err != nil {
		return err
	}
	if err := api_v1.DeepCopy_v1_ObjectMeta(in.ObjectMeta, &out.ObjectMeta, c); err != nil {
		return err
	}
	if err := DeepCopy_v1_JobSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := DeepCopy_v1_JobStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func DeepCopy_v1_JobCondition(in JobCondition, out *JobCondition, c *conversion.Cloner) error {
	out.Type = in.Type
	out.Status = in.Status
	if err := unversioned.DeepCopy_unversioned_Time(in.LastProbeTime, &out.LastProbeTime, c); err != nil {
		return err
	}
	if err := unversioned.DeepCopy_unversioned_Time(in.LastTransitionTime, &out.LastTransitionTime, c); err != nil {
		return err
	}
	out.Reason = in.Reason
	out.Message = in.Message
	return nil
}

func DeepCopy_v1_JobList(in JobList, out *JobList, c *conversion.Cloner) error {
	if err := unversioned.DeepCopy_unversioned_TypeMeta(in.TypeMeta, &out.TypeMeta, c); err != nil {
		return err
	}
	if err := unversioned.DeepCopy_unversioned_ListMeta(in.ListMeta, &out.ListMeta, c); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := in.Items, &out.Items
		*out = make([]Job, len(in))
		for i := range in {
			if err := DeepCopy_v1_Job(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func DeepCopy_v1_JobSpec(in JobSpec, out *JobSpec, c *conversion.Cloner) error {
	if in.Parallelism != nil {
		in, out := in.Parallelism, &out.Parallelism
		*out = new(int32)
		**out = *in
	} else {
		out.Parallelism = nil
	}
	if in.Completions != nil {
		in, out := in.Completions, &out.Completions
		*out = new(int32)
		**out = *in
	} else {
		out.Completions = nil
	}
	if in.ActiveDeadlineSeconds != nil {
		in, out := in.ActiveDeadlineSeconds, &out.ActiveDeadlineSeconds
		*out = new(int64)
		**out = *in
	} else {
		out.ActiveDeadlineSeconds = nil
	}
	if in.Selector != nil {
		in, out := in.Selector, &out.Selector
		*out = new(LabelSelector)
		if err := DeepCopy_v1_LabelSelector(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.Selector = nil
	}
	if in.ManualSelector != nil {
		in, out := in.ManualSelector, &out.ManualSelector
		*out = new(bool)
		**out = *in
	} else {
		out.ManualSelector = nil
	}
	if err := api_v1.DeepCopy_v1_PodTemplateSpec(in.Template, &out.Template, c); err != nil {
		return err
	}
	return nil
}

func DeepCopy_v1_JobStatus(in JobStatus, out *JobStatus, c *conversion.Cloner) error {
	if in.Conditions != nil {
		in, out := in.Conditions, &out.Conditions
		*out = make([]JobCondition, len(in))
		for i := range in {
			if err := DeepCopy_v1_JobCondition(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Conditions = nil
	}
	if in.StartTime != nil {
		in, out := in.StartTime, &out.StartTime
		*out = new(unversioned.Time)
		if err := unversioned.DeepCopy_unversioned_Time(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.StartTime = nil
	}
	if in.CompletionTime != nil {
		in, out := in.CompletionTime, &out.CompletionTime
		*out = new(unversioned.Time)
		if err := unversioned.DeepCopy_unversioned_Time(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.CompletionTime = nil
	}
	out.Active = in.Active
	out.Succeeded = in.Succeeded
	out.Failed = in.Failed
	return nil
}

func DeepCopy_v1_LabelSelector(in LabelSelector, out *LabelSelector, c *conversion.Cloner) error {
	if in.MatchLabels != nil {
		in, out := in.MatchLabels, &out.MatchLabels
		*out = make(map[string]string)
		for key, val := range in {
			(*out)[key] = val
		}
	} else {
		out.MatchLabels = nil
	}
	if in.MatchExpressions != nil {
		in, out := in.MatchExpressions, &out.MatchExpressions
		*out = make([]LabelSelectorRequirement, len(in))
		for i := range in {
			if err := DeepCopy_v1_LabelSelectorRequirement(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.MatchExpressions = nil
	}
	return nil
}

func DeepCopy_v1_LabelSelectorRequirement(in LabelSelectorRequirement, out *LabelSelectorRequirement, c *conversion.Cloner) error {
	out.Key = in.Key
	out.Operator = in.Operator
	if in.Values != nil {
		in, out := in.Values, &out.Values
		*out = make([]string, len(in))
		copy(*out, in)
	} else {
		out.Values = nil
	}
	return nil
}
