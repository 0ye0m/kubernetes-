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

package node

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/kubelet/util/format"
	"k8s.io/kubernetes/pkg/types"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/version"

	"github.com/golang/glog"
)

const (
	// Number of Nodes that needs to be in the cluster for it to be treated as "large"
	LargeClusterThreshold = 20
)

// cleanupOrphanedPods deletes pods that are bound to nodes that don't
// exist.
func cleanupOrphanedPods(pods []*api.Pod, nodeStore cache.Store, forcefulDeletePodFunc func(*api.Pod) error) {
	for _, pod := range pods {
		if pod.Spec.NodeName == "" {
			continue
		}
		if _, exists, _ := nodeStore.GetByKey(pod.Spec.NodeName); exists {
			continue
		}
		if err := forcefulDeletePodFunc(pod); err != nil {
			utilruntime.HandleError(err)
		}
	}
}

func tolerationsToleratesTaints(tolerations []api.Toleration, taints []api.Taint) bool {
	// If the taint list is nil/empty, it is tolerated by all tolerations by default.
	if len(taints) == 0 {
		return true
	}

	// The taint list isn't nil/empty, a nil/empty toleration list can't tolerate them.
	if len(tolerations) == 0 {
		return false
	}

	for i := range taints {
		taint := &taints[i]
		// skip taints that have effect PreferNoSchedule, since it is for priorities
		if taint.Effect == api.TaintEffectPreferNoSchedule {
			continue
		}

		if !api.TaintToleratedByTolerations(taint, tolerations) {
			return false
		}
	}

	return true
}

func deletePod(kubeClient clientset.Interface, pod *api.Pod, recorder record.EventRecorder, daemonStore cache.StoreToDaemonSetLister) (bool, error) {
	// if the pod has already been marked for deletion, we still return true that there are remaining pods.
	if pod.DeletionGracePeriodSeconds != nil {
		return true, nil
	}
	// if the pod is managed by a daemonset, ignore it
	_, err := daemonStore.GetPodDaemonSets(pod)
	if err == nil { // No error means at least one daemonset was found
		return false, nil
	}

	glog.V(2).Infof("Starting deletion of pod %v", pod.Name)
	recorder.Eventf(pod, api.EventTypeNormal, "NodeControllerEviction", "Marking for deletion Pod %s from Node %s", pod.Name, pod.Spec.NodeName)
	if err := kubeClient.Core().Pods(pod.Namespace).Delete(pod.Name, nil); err != nil {
		return false, err
	}
	return false, nil
}

func getPodsForANode(kubeClient clientset.Interface, nodeName string) (*api.PodList, error) {
	selector := fields.OneTermEqualSelector(api.PodHostField, nodeName)
	options := api.ListOptions{FieldSelector: selector}
	pods, err := kubeClient.Core().Pods(api.NamespaceAll).List(options)
	if err != nil {
		return nil, err
	}
	return pods, nil
}

func forcefullyDeletePod(c clientset.Interface, pod *api.Pod) error {
	var zero int64
	err := c.Core().Pods(pod.Namespace).Delete(pod.Name, &api.DeleteOptions{GracePeriodSeconds: &zero})
	if err == nil {
		glog.V(4).Infof("forceful deletion of %s succeeded", pod.Name)
	}
	return err
}

// forcefullyDeleteNode immediately deletes all pods on the node, and then
// deletes the node itself.
func forcefullyDeleteNode(kubeClient clientset.Interface, nodeName string, forcefulDeletePodFunc func(*api.Pod) error) error {
	selector := fields.OneTermEqualSelector(api.PodHostField, nodeName)
	options := api.ListOptions{FieldSelector: selector}
	pods, err := kubeClient.Core().Pods(api.NamespaceAll).List(options)
	if err != nil {
		return fmt.Errorf("unable to list pods on node %q: %v", nodeName, err)
	}
	for _, pod := range pods.Items {
		if pod.Spec.NodeName != nodeName {
			continue
		}
		if err := forcefulDeletePodFunc(&pod); err != nil {
			return fmt.Errorf("unable to delete pod %q on node %q: %v", pod.Name, nodeName, err)
		}
	}
	if err := kubeClient.Core().Nodes().Delete(nodeName, nil); err != nil {
		return fmt.Errorf("unable to delete node %q: %v", nodeName, err)
	}
	return nil
}

// maybeDeleteTerminatingPod non-gracefully deletes pods that are terminating
// that should not be gracefully terminated.
func (nc *NodeController) maybeDeleteTerminatingPod(obj interface{}) {
	pod, ok := obj.(*api.Pod)
	if !ok {
		return
	}

	// consider only terminating pods
	if pod.DeletionTimestamp == nil {
		return
	}

	// delete terminating pods that have not yet been scheduled
	if len(pod.Spec.NodeName) == 0 {
		utilruntime.HandleError(nc.forcefullyDeletePod(pod))
		return
	}

	nodeObj, found, err := nc.nodeStore.Store.GetByKey(pod.Spec.NodeName)
	if err != nil {
		// this can only happen if the Store.KeyFunc has a problem creating
		// a key for the pod. If it happens once, it will happen again so
		// don't bother requeuing the pod.
		utilruntime.HandleError(err)
		return
	}

	// delete terminating pods that have been scheduled on
	// nonexistent nodes
	if !found {
		glog.Warningf("Unable to find Node: %v, deleting all assigned Pods.", pod.Spec.NodeName)
		utilruntime.HandleError(nc.forcefullyDeletePod(pod))
		return
	}

	// delete terminating pods that have been scheduled on
	// nodes that do not support graceful termination
	// TODO(mikedanese): this can be removed when we no longer
	// guarantee backwards compatibility of master API to kubelets with
	// versions less than 1.1.0
	node := nodeObj.(*api.Node)
	v, err := version.Parse(node.Status.NodeInfo.KubeletVersion)
	if err != nil {
		glog.V(0).Infof("couldn't parse verions %q of minion: %v", node.Status.NodeInfo.KubeletVersion, err)
		utilruntime.HandleError(nc.forcefullyDeletePod(pod))
		return
	}
	if gracefulDeletionVersion.GT(v) {
		utilruntime.HandleError(nc.forcefullyDeletePod(pod))
		return
	}
}

// update ready status of all pods running on given node from master
// return true if success
func markAllPodsNotReady(kubeClient clientset.Interface, node *api.Node) error {
	// Don't set pods to NotReady if the kubelet is running a version that
	// doesn't understand how to correct readiness.
	// TODO: Remove this check when we no longer guarantee backward compatibility
	// with node versions < 1.2.0.
	if nodeRunningOutdatedKubelet(node) {
		return nil
	}
	nodeName := node.Name
	glog.V(2).Infof("Update ready status of pods on node [%v]", nodeName)
	opts := api.ListOptions{FieldSelector: fields.OneTermEqualSelector(api.PodHostField, nodeName)}
	pods, err := kubeClient.Core().Pods(api.NamespaceAll).List(opts)
	if err != nil {
		return err
	}

	errMsg := []string{}
	for _, pod := range pods.Items {
		// Defensive check, also needed for tests.
		if pod.Spec.NodeName != nodeName {
			continue
		}

		for i, cond := range pod.Status.Conditions {
			if cond.Type == api.PodReady {
				pod.Status.Conditions[i].Status = api.ConditionFalse
				glog.V(2).Infof("Updating ready status of pod %v to false", pod.Name)
				_, err := kubeClient.Core().Pods(pod.Namespace).UpdateStatus(&pod)
				if err != nil {
					glog.Warningf("Failed to update status for pod %q: %v", format.Pod(&pod), err)
					errMsg = append(errMsg, fmt.Sprintf("%v", err))
				}
				break
			}
		}
	}
	if len(errMsg) == 0 {
		return nil
	}
	return fmt.Errorf("%v", strings.Join(errMsg, "; "))
}

// nodeRunningOutdatedKubelet returns true if the kubeletVersion reported
// in the nodeInfo of the given node is "outdated", meaning < 1.2.0.
// Older versions were inflexible and modifying pod.Status directly through
// the apiserver would result in unexpected outcomes.
func nodeRunningOutdatedKubelet(node *api.Node) bool {
	v, err := version.Parse(node.Status.NodeInfo.KubeletVersion)
	if err != nil {
		glog.Errorf("couldn't parse version %q of node %v", node.Status.NodeInfo.KubeletVersion, err)
		return true
	}
	if podStatusReconciliationVersion.GT(v) {
		glog.Infof("Node %v running kubelet at (%v) which is less than the minimum version that allows nodecontroller to mark pods NotReady (%v).", node.Name, v, podStatusReconciliationVersion)
		return true
	}
	return false
}

func nodeExistsInCloudProvider(cloud cloudprovider.Interface, nodeName string) (bool, error) {
	instances, ok := cloud.Instances()
	if !ok {
		return false, fmt.Errorf("%v", ErrCloudInstance)
	}
	if _, err := instances.ExternalID(nodeName); err != nil {
		if err == cloudprovider.InstanceNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func recordNodeEvent(recorder record.EventRecorder, nodeName, nodeUID, eventtype, reason, event string) {
	ref := &api.ObjectReference{
		Kind:      "Node",
		Name:      nodeName,
		UID:       types.UID(nodeUID),
		Namespace: "",
	}
	glog.V(2).Infof("Recording %s event message for node %s", event, nodeName)
	recorder.Eventf(ref, eventtype, reason, "Node %s event: %s", nodeName, event)
}

func recordNodeStatusChange(recorder record.EventRecorder, node *api.Node, new_status string) {
	ref := &api.ObjectReference{
		Kind:      "Node",
		Name:      node.Name,
		UID:       node.UID,
		Namespace: "",
	}
	glog.V(2).Infof("Recording status change %s event message for node %s", new_status, node.Name)
	// TODO: This requires a transaction, either both node status is updated
	// and event is recorded or neither should happen, see issue #6055.
	recorder.Eventf(ref, api.EventTypeNormal, new_status, "Node %s status is now: %s", node.Name, new_status)
}

func terminatePod(kubeClient clientset.Interface, recorder record.EventRecorder, nodeName string, nodeUID string, pod *api.Pod, since time.Time, maxGracePeriod time.Duration) (bool, time.Duration, error) {
	// the time before we should try again
	complete := true
	nextAttempt := time.Duration(0)
	now := time.Now()
	elapsed := now.Sub(since)

	if pod.Spec.NodeName != nodeName {
		return complete, nextAttempt, nil
	}
	// only clean terminated pods
	if pod.DeletionGracePeriodSeconds == nil {
		return complete, nextAttempt, nil
	}

	// the user's requested grace period
	grace := time.Duration(*pod.DeletionGracePeriodSeconds) * time.Second
	if grace > maxGracePeriod {
		grace = maxGracePeriod
	}

	// the time remaining before the pod should have been deleted
	remaining := grace - elapsed
	if remaining < 0 {
		remaining = 0
		glog.V(2).Infof("Removing pod %v after %s grace period", pod.Name, grace)
		recordNodeEvent(recorder, nodeName, nodeUID, api.EventTypeNormal, "TerminatingEvictedPod", fmt.Sprintf("Pod %s has exceeded the grace period for deletion after being evicted from Node %q and is being force killed", pod.Name, nodeName))
		if err := kubeClient.Core().Pods(pod.Namespace).Delete(pod.Name, api.NewDeleteOptions(0)); err != nil {
			glog.Errorf("Error completing deletion of pod %s: %v", pod.Name, err)
			complete = false
		}
	} else {
		glog.V(2).Infof("Pod %v still terminating, requested grace period %s, %s remaining", pod.Name, grace, remaining)
		complete = false
	}

	if nextAttempt < remaining {
		nextAttempt = remaining
	}
	return complete, nextAttempt, nil
}

func addNodeOutageTaint(kubeClient clientset.Interface, node *api.Node) error {
	taintNodeOutage := api.Taint{Key: "operator", Value: "node-outage", Effect: api.TaintEffectNoSchedule}
	annotations := node.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	taints, err := api.GetTaintsFromNodeAnnotations(annotations)
	if err != nil {
		return err
	}
	for _, taint := range taints {
		if taint.Key == taintNodeOutage.Key && taint.Effect == taintNodeOutage.Effect {
			glog.V(2).Infof("operator taint already exists with value %v", taint.Effect)
			return nil
		}
	}
	taints = append(taints, taintNodeOutage)
	taintsData, err := json.Marshal(taints)
	if err != nil {
		return err
	}
	annotations[api.TaintsAnnotationKey] = string(taintsData)
	node.SetAnnotations(annotations)
	_, err = kubeClient.Core().Nodes().Update(node)
	return err
}

func removeNodeOutageTaint(kubeClient clientset.Interface, node *api.Node) error {
	taintNodeOutage := api.Taint{Key: "operator", Value: "node-outage", Effect: api.TaintEffectNoSchedule}
	annotations := node.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	taints, err := api.GetTaintsFromNodeAnnotations(annotations)
	if len(taints) == 0 {
		return nil
	}
	if err != nil {
		return err
	}
	var newTaints []api.Taint
	for _, taint := range taints {
		if !(taint.Key == taintNodeOutage.Key && taint.Effect == taintNodeOutage.Effect && taint.Value == taintNodeOutage.Value) {
			newTaints = append(newTaints, taint)
		}
	}
	taintsData, err := json.Marshal(newTaints)
	if err != nil {
		return err
	}
	annotations[api.TaintsAnnotationKey] = string(taintsData)
	node.SetAnnotations(annotations)
	_, err = kubeClient.Core().Nodes().Update(node)
	return err
}

type evictionMessage struct {
	podName      string
	podNamespace string
	nodeUID      types.UID
}
