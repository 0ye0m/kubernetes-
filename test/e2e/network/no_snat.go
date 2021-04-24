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

package network

import (
	"context"
	"net"
	"sync"

	"github.com/onsi/ginkgo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/e2e/network/common"
	imageutils "k8s.io/kubernetes/test/utils/image"
)

const (
	// try to use a no common port so it doesn't conflict using hostNetwork
	testPodPort    = "8085"
	noSNATTestName = "no-snat-test"
)

var (
	testPod = v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: noSNATTestName,
			Labels: map[string]string{
				noSNATTestName: "",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  noSNATTestName,
					Image: imageutils.GetE2EImage(imageutils.Agnhost),
					Args:  []string{"netexec", "--http-port", testPodPort},
				},
			},
		},
	}
)

// This test verifies that a Pod on each node in a cluster can talk to Pods on every other node without SNAT.
// Kubernetes imposes the following fundamental requirements on any networking implementation
// (barring any intentional network segmentation policies):
//
// pods on a node can communicate with all pods on all nodes without NAT
// agents on a node (e.g. system daemons, kubelet) can communicate with all pods on that node
// Note: For those platforms that support Pods running in the host network (e.g. Linux):
// pods in the host network of a node can communicate with all pods on all nodes without NAT
// xref: https://kubernetes.io/docs/concepts/cluster-administration/networking/
var _ = common.SIGDescribe("NoSNAT [Slow]", func() {
	f := framework.NewDefaultFramework("no-snat-test")
	ginkgo.It("Should be able to send traffic between Pods without SNAT", func() {
		cs := f.ClientSet
		pc := cs.CoreV1().Pods(f.Namespace.Name)

		ginkgo.By("creating a test pod on each Node")
		nodes, err := e2enode.GetReadySchedulableNodes(cs)
		framework.ExpectNoError(err)
		framework.ExpectNotEqual(len(nodes.Items), 0, "no Nodes in the cluster")

		var wg sync.WaitGroup
		for i, node := range nodes.Items {
			// limit the number of nodes to avoid duration issues on large clusters
			if i == 10 {
				break
			}
			// target Pod at Node
			ginkgo.By("creating pod on node " + node.Name)
			nodeSelection := e2epod.NodeSelection{Name: node.Name}
			e2epod.SetNodeSelection(&testPod.Spec, nodeSelection)
			wg.Add(1)
			go func() {
				defer ginkgo.GinkgoRecover()
				f.PodClient().CreateSync(&testPod)
				wg.Done()
			}()
		}
		wg.Wait()

		ginkgo.By("sending traffic from each pod to the others and checking that SNAT does not occur")
		pods, err := pc.List(context.TODO(), metav1.ListOptions{LabelSelector: noSNATTestName})
		framework.ExpectNoError(err)

		// hit the /clientip endpoint on every other Pods to check if source ip is preserved
		for _, sourcePod := range pods.Items {
			for _, targetPod := range pods.Items {
				if targetPod.Name == sourcePod.Name {
					continue
				}
				targetAddr := net.JoinHostPort(targetPod.Status.PodIP, testPodPort)
				ginkgo.By("testing from pod " + sourcePod.Name + " to pod " + targetPod.Name)
				wg.Add(1)
				go func() {
					defer ginkgo.GinkgoRecover()
					sourceIP, execPodIP := execSourceIPTest(sourcePod, targetAddr)
					ginkgo.By("Verifying the preserved source ip")
					framework.ExpectEqual(sourceIP, execPodIP)
					wg.Done()
				}()
			}
		}
		wg.Wait()
	})

	ginkgo.It("Should be able to send traffic between Pods and an agent on that Node without SNAT", func() {
		cs := f.ClientSet
		pc := cs.CoreV1().Pods(f.Namespace.Name)

		ginkgo.By("creating a test pod on one Node")
		nodes, err := e2enode.GetReadySchedulableNodes(cs)
		framework.ExpectNoError(err)
		framework.ExpectNotEqual(len(nodes.Items), 0, "no Nodes in the cluster")

		// use one node
		node := nodes.Items[0]
		// target Pod at Node
		nodeSelection := e2epod.NodeSelection{Name: node.Name}
		e2epod.SetNodeSelection(&testPod.Spec, nodeSelection)
		f.PodClient().CreateSync(&testPod)
		ginkgo.By("creating a hostnetwork test pod on the same Node")
		testPod.Spec.HostNetwork = true
		f.PodClient().CreateSync(&testPod)
		ginkgo.By("sending traffic from each pod to the others and checking that SNAT does not occur")
		pods, err := pc.List(context.TODO(), metav1.ListOptions{LabelSelector: noSNATTestName})
		framework.ExpectNoError(err)

		// hit the /clientip endpoint on every other Pods to check if source ip is preserved
		for _, sourcePod := range pods.Items {
			for _, targetPod := range pods.Items {
				if targetPod.Name == sourcePod.Name {
					continue
				}
				targetAddr := net.JoinHostPort(targetPod.Status.PodIP, testPodPort)
				ginkgo.By("testing from pod " + sourcePod.Name + " to pod " + targetPod.Name)

				sourceIP, execPodIP := execSourceIPTest(sourcePod, targetAddr)
				ginkgo.By("Verifying the preserved source ip")
				framework.ExpectEqual(sourceIP, execPodIP)
			}
		}

	})

	ginkgo.It("Should be able to send traffic between Pods and HostNetwork pods without SNAT [LinuxOnly]", func() {
		cs := f.ClientSet
		pc := cs.CoreV1().Pods(f.Namespace.Name)

		ginkgo.By("creating a test pod on each Node")
		nodes, err := e2enode.GetReadySchedulableNodes(cs)
		framework.ExpectNoError(err)
		framework.ExpectNotEqual(len(nodes.Items), 0, "no Nodes in the cluster")

		var wg sync.WaitGroup
		// limit the number of nodes to avoid duration issues on large clusters
		maxNodes := len(nodes.Items)
		if maxNodes > 10 {
			maxNodes = 10
		}
		for i, node := range nodes.Items {
			// create the last pod with hostNetwork true
			if i == maxNodes-1 {
				testPod.Spec.HostNetwork = true
			}
			if i == maxNodes {
				break
			}
			// target Pod at Node
			ginkgo.By("creating pod on node " + node.Name)
			nodeSelection := e2epod.NodeSelection{Name: node.Name}
			e2epod.SetNodeSelection(&testPod.Spec, nodeSelection)
			wg.Add(1)
			go func() {
				defer ginkgo.GinkgoRecover()
				f.PodClient().CreateSync(&testPod)
				wg.Done()
			}()
		}
		wg.Wait()

		ginkgo.By("sending traffic from each pod to the others and checking that SNAT does not occur")
		pods, err := pc.List(context.TODO(), metav1.ListOptions{LabelSelector: noSNATTestName})
		framework.ExpectNoError(err)

		// hit the /clientip endpoint on every other Pods to check if source ip is preserved
		for _, sourcePod := range pods.Items {
			for _, targetPod := range pods.Items {
				if targetPod.Name == sourcePod.Name {
					continue
				}
				targetAddr := net.JoinHostPort(targetPod.Status.PodIP, testPodPort)
				ginkgo.By("testing from pod " + sourcePod.Name + " to pod " + targetPod.Name)
				wg.Add(1)
				go func() {
					defer ginkgo.GinkgoRecover()
					sourceIP, execPodIP := execSourceIPTest(sourcePod, targetAddr)
					ginkgo.By("Verifying the preserved source ip")
					framework.ExpectEqual(sourceIP, execPodIP)
					wg.Done()
				}()
			}
		}
		wg.Wait()
	})

})
