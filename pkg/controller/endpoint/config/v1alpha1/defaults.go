/*
Copyright 2019 The Kubernetes Authors.

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
	kubectrlmgrconfigv1alpha1 "k8s.io/kube-controller-manager/config/v1alpha1"
)

// RecommendedDefaultEndpointControllerConfiguration defaults a pointer to a
// EndpointControllerConfiguration struct. This will set the recommended default
// values, but they may be subject to change between API versions. This function
// is intentionally not registered in the scheme as a "normal" `SetDefaults_Foo`
// function to allow consumers of this type to set whatever defaults for their
// embedded configs. Forcing consumers to use these defaults would be problematic
// as defaulting in the scheme is done as part of the conversion, and there would
// be no easy way to opt-out. Instead, if you want to use this defaulting method
// run it in your wrapper struct of this type in its `SetDefaults_` method.
func RecommendedDefaultEndpointControllerConfiguration(obj *kubectrlmgrconfigv1alpha1.EndpointControllerConfiguration) {
	if obj.ConcurrentEndpointSyncs == 0 {
		obj.ConcurrentEndpointSyncs = 5
	}

	if obj.EndpointUpdatesQPS == 0.0 {
		// Default 50.0 value matches default kube-api-qps limit for endpoint-controller.
		obj.EndpointUpdatesQPS = 50.0
	}

	if obj.EndpointUpdatesBurst == 0 {
		obj.EndpointUpdatesBurst = 1
	}
}
