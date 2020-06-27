// +build !dockerless

/*
Copyright 2016 The Kubernetes Authors.

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

package dockershim

import (
	"fmt"

	dockercontainer "github.com/docker/docker/api/types/container"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

// modifyHostOptionsForContainer applies NetworkMode/UTSMode to container's dockercontainer.HostConfig.
func modifyHostOptionsForContainer(nsOpts *runtimeapi.NamespaceOption, podSandboxID string, hc *dockercontainer.HostConfig) {
	sandboxNSMode := fmt.Sprintf("container:%v", podSandboxID)
	hc.NetworkMode = dockercontainer.NetworkMode(sandboxNSMode)
	hc.IpcMode = dockercontainer.IpcMode(sandboxNSMode)
	hc.UTSMode = ""

	if nsOpts.GetNetwork() == runtimeapi.NamespaceMode_NODE {
		hc.UTSMode = namespaceModeHost
	}
}
