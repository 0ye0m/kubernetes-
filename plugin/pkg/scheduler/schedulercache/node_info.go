/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package schedulercache

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api"
	clientcache "k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm/priorities/priorityutil"
)

var emptyResource = Resource{}

// NodeInfo is node level aggregated information.
type NodeInfo struct {
	// Total requested resource of all pods (including assumed ones) on this node
	requestedResource *Resource
	pods              []*api.Pod
	nonzeroRequest    *Resource
}

// Resource is a collection of compute resource
type Resource struct {
	MilliCPU int64
	Memory   int64
}

// NewNodeInfo returns a ready to use empty NodeInfo object.
// If any pods are given in arguments, their information will be aggregated in
// the returned object.
func NewNodeInfo(pods ...*api.Pod) *NodeInfo {
	ni := &NodeInfo{
		requestedResource: &Resource{},
		nonzeroRequest:    &Resource{},
	}
	for _, pod := range pods {
		ni.addPod(pod)
	}
	return ni
}

// Pods return all pods scheduled (including assumed to be) on this node
func (n *NodeInfo) Pods() []*api.Pod {
	if n == nil {
		return nil
	}
	return n.pods
}

// RequestedResource returns aggregated resource request of pods on this node
func (n *NodeInfo) RequestedResource() Resource {
	if n == nil {
		return emptyResource
	}
	return *n.requestedResource
}

// NonZeroRequest returns aggregated nonzero resource request of pods on this node
func (n *NodeInfo) NonZeroRequest() Resource {
	if n == nil {
		return emptyResource
	}
	return *n.nonzeroRequest
}

func (n *NodeInfo) Clone() *NodeInfo {
	pods := append([]*api.Pod(nil), n.pods...)
	clone := &NodeInfo{
		requestedResource: &(*n.requestedResource),
		nonzeroRequest:    &(*n.nonzeroRequest),
		pods:              pods,
	}
	return clone
}

// String returns representation of human readable format of this NodeInfo.
func (n *NodeInfo) String() string {
	podKeys := make([]string, len(n.pods))
	for i, pod := range n.pods {
		podKeys[i] = pod.Name
	}
	return fmt.Sprintf("&NodeInfo{Pods:%v, RequestedResource:%#v, NonZeroRequest: %#v}", podKeys, n.requestedResource, n.nonzeroRequest)
}

// AddPod adds pod information to this NodeInfo.
func (n *NodeInfo) addPod(pod *api.Pod) {
	cpu, mem, non0_cpu, non0_mem := calculateResource(pod)
	n.requestedResource.MilliCPU += cpu
	n.requestedResource.Memory += mem
	n.nonzeroRequest.MilliCPU += non0_cpu
	n.nonzeroRequest.Memory += non0_mem
	n.pods = append(n.pods, pod)
}

// removePod subtracts pod information to this NodeInfo.
func (n *NodeInfo) removePod(pod *api.Pod) {
	cpu, mem, non0_cpu, non0_mem := calculateResource(pod)
	n.requestedResource.MilliCPU -= cpu
	n.requestedResource.Memory -= mem
	n.nonzeroRequest.MilliCPU -= non0_cpu
	n.nonzeroRequest.Memory -= non0_mem

	getKey := mustGetPodKey
	for i := range n.pods {
		if getKey(pod) == getKey(n.pods[i]) {
			// delete the element
			n.pods[i] = n.pods[len(n.pods)-1]
			n.pods = n.pods[:len(n.pods)-1]
			break
		}
	}
}

func calculateResource(pod *api.Pod) (cpu int64, mem int64, non0_cpu int64, non0_mem int64) {
	for _, c := range pod.Spec.Containers {
		req := c.Resources.Requests
		cpu += req.Cpu().MilliValue()
		mem += req.Memory().Value()

		non0_cpu_req, non0_mem_req := priorityutil.GetNonzeroRequests(&req)
		non0_cpu += non0_cpu_req
		non0_mem += non0_mem_req
	}
	return
}

// mustGetPodKey returns the string key of a pod.
// A pod is ensured to have accessor. We don't want to check the error everytime.
// TODO: We should consider adding a Key() method to api.Pod
func mustGetPodKey(pod *api.Pod) string {
	key, err := clientcache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		panic("api.Pod should have key func: " + err.Error())
	}
	return key
}
