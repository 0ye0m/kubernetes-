// +build !ignore_autogenerated

/*
Copyright 2017 The Kubernetes Authors.

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

package admission

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
	v1beta1 "k8s.io/client-go/pkg/apis/authentication/v1beta1"
	reflect "reflect"
)

func init() {
	SchemeBuilder.Register(RegisterDeepCopies)
}

// RegisterDeepCopies adds deep-copy functions to the given scheme. Public
// to allow building arbitrary schemes.
func RegisterDeepCopies(scheme *runtime.Scheme) error {
	return scheme.AddGeneratedDeepCopyFuncs(
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_admission_AdmittanceReview, InType: reflect.TypeOf(&AdmittanceReview{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_admission_AdmittanceReviewSpec, InType: reflect.TypeOf(&AdmittanceReviewSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_admission_AdmittanceReviewStatus, InType: reflect.TypeOf(&AdmittanceReviewStatus{})},
	)
}

func DeepCopy_admission_AdmittanceReview(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*AdmittanceReview)
		out := out.(*AdmittanceReview)
		*out = *in
		if newVal, err := c.DeepCopy(&in.ObjectMeta); err != nil {
			return err
		} else {
			out.ObjectMeta = *newVal.(*v1.ObjectMeta)
		}
		if err := DeepCopy_admission_AdmittanceReviewSpec(&in.Spec, &out.Spec, c); err != nil {
			return err
		}
		return nil
	}
}

func DeepCopy_admission_AdmittanceReviewSpec(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*AdmittanceReviewSpec)
		out := out.(*AdmittanceReviewSpec)
		*out = *in
		// in.Object is kind 'Interface'
		if in.Object != nil {
			if newVal, err := c.DeepCopy(&in.Object); err != nil {
				return err
			} else {
				out.Object = *newVal.(*runtime.Object)
			}
		}
		// in.OldObject is kind 'Interface'
		if in.OldObject != nil {
			if newVal, err := c.DeepCopy(&in.OldObject); err != nil {
				return err
			} else {
				out.OldObject = *newVal.(*runtime.Object)
			}
		}
		if newVal, err := c.DeepCopy(&in.UserInfo); err != nil {
			return err
		} else {
			out.UserInfo = *newVal.(*v1beta1.UserInfo)
		}
		return nil
	}
}

func DeepCopy_admission_AdmittanceReviewStatus(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*AdmittanceReviewStatus)
		out := out.(*AdmittanceReviewStatus)
		*out = *in
		return nil
	}
}
