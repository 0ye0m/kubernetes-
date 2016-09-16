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

package e2e

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	release_1_4 "k8s.io/client-go/1.5/kubernetes"
	apiapi "k8s.io/client-go/1.5/pkg/api"
	"k8s.io/client-go/1.5/pkg/api/unversioned"
	api "k8s.io/client-go/1.5/pkg/api/v1"
	extensions "k8s.io/client-go/1.5/pkg/apis/extensions/v1beta1"
	policy "k8s.io/client-go/1.5/pkg/apis/policy/v1alpha1"
	"k8s.io/client-go/1.5/pkg/util/intstr"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
)

// timeout is used for most polling/waiting activities
const timeout = 60 * time.Second

// schedulingTimeout is longer specifically because sometimes we need to wait
// awhile to guarantee that we've been patient waiting for something ordinary
// to happen: a pod to get scheduled and move into Ready
const schedulingTimeout = 5 * time.Minute

var _ = framework.KubeDescribe("DisruptionController", func() {
	f := framework.NewDefaultFramework("disruption")
	var ns string
	var cs *release_1_4.Clientset

	BeforeEach(func() {
		// skip on GKE since alpha features are disabled
		framework.SkipIfProviderIs("gke")

		cs = f.StagingClient
		ns = f.Namespace.Name
	})

	It("should create a PodDisruptionBudget", func() {
		createPodDisruptionBudgetOrDie(cs, ns, intstr.FromString("1%"))
	})

	It("should update PodDisruptionBudget status", func() {
		createPodDisruptionBudgetOrDie(cs, ns, intstr.FromInt(2))

		createPodsOrDie(cs, ns, 2)

		// Since disruptionAllowed starts out false, if we see it ever become true,
		// that means the controller is working.
		err := wait.PollImmediate(framework.Poll, timeout, func() (bool, error) {
			pdb, err := cs.Policy().PodDisruptionBudgets(ns).Get("foo")
			if err != nil {
				return false, err
			}
			return pdb.Status.PodDisruptionAllowed, nil
		})
		Expect(err).NotTo(HaveOccurred())

	})

	evictionCases := []struct {
		description    string
		minAvailable   intstr.IntOrString
		podCount       int
		replicaSetSize int32
		shouldDeny     bool
		exclusive      bool
	}{
		{
			description:  "no PDB",
			minAvailable: intstr.FromString(""),
			podCount:     1,
			shouldDeny:   false,
		}, {
			description:  "too few pods, absolute",
			minAvailable: intstr.FromInt(2),
			podCount:     1,
			shouldDeny:   true,
		}, {
			description:  "enough pods, absolute",
			minAvailable: intstr.FromInt(2),
			podCount:     2,
			shouldDeny:   false,
		}, {
			description:    "enough pods, replicaSet, percentage",
			minAvailable:   intstr.FromString("100%"),
			replicaSetSize: 10,
			exclusive:      false,
			shouldDeny:     false,
		}, {
			description:    "too few pods, replicaSet, percentage",
			minAvailable:   intstr.FromString("100%"),
			replicaSetSize: 10,
			exclusive:      true,
			shouldDeny:     true,
		},
	}
	for _, c := range evictionCases {
		expectation := "should allow an eviction"
		if c.shouldDeny {
			expectation = "should not allow an eviction"
		}
		It(fmt.Sprintf("evictions: %s => %s", c.description, expectation), func() {
			createPodsOrDie(cs, ns, c.podCount)
			if c.replicaSetSize > 0 {
				createReplicaSetOrDie(cs, ns, c.replicaSetSize, c.exclusive)
			}

			if c.minAvailable.String() != "" {
				createPodDisruptionBudgetOrDie(cs, ns, c.minAvailable)
			}

			// Locate a running pod.
			var pod api.Pod
			err := wait.PollImmediate(framework.Poll, schedulingTimeout, func() (bool, error) {
				podList, err := cs.Pods(ns).List(apiapi.ListOptions{})
				if err != nil {
					return false, err
				}

				for i := range podList.Items {
					if podList.Items[i].Status.Phase == api.PodRunning {
						pod = podList.Items[i]
						return true, nil
					}
				}

				return false, nil
			})
			Expect(err).NotTo(HaveOccurred())

			e := &policy.Eviction{
				ObjectMeta: api.ObjectMeta{
					Name:      pod.Name,
					Namespace: ns,
				},
			}

			if c.shouldDeny {
				// Since disruptionAllowed starts out false, wait at least 60s hoping that
				// this gives the controller enough time to have truly set the status.
				time.Sleep(timeout)

				err = cs.Pods(ns).Evict(e)
				Expect(err).Should(MatchError("Cannot evict pod as it would violate the pod's disruption budget."))
			} else {
				// Since disruptionAllowed starts out false, if an eviction is ever allowed,
				// that means the controller is working.
				err = wait.PollImmediate(framework.Poll, timeout, func() (bool, error) {
					err = cs.Pods(ns).Evict(e)
					if err != nil {
						return false, nil
					} else {
						return true, nil
					}
				})
				Expect(err).NotTo(HaveOccurred())
			}
		})
	}

})

func createPodDisruptionBudgetOrDie(cs *release_1_4.Clientset, ns string, minAvailable intstr.IntOrString) {
	pdb := policy.PodDisruptionBudget{
		ObjectMeta: api.ObjectMeta{
			Name:      "foo",
			Namespace: ns,
		},
		Spec: policy.PodDisruptionBudgetSpec{
			Selector:     &unversioned.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
			MinAvailable: minAvailable,
		},
	}
	_, err := cs.Policy().PodDisruptionBudgets(ns).Create(&pdb)
	Expect(err).NotTo(HaveOccurred())
}

func createPodsOrDie(cs *release_1_4.Clientset, ns string, n int) {
	for i := 0; i < n; i++ {
		pod := &api.Pod{
			ObjectMeta: api.ObjectMeta{
				Name:      fmt.Sprintf("pod-%d", i),
				Namespace: ns,
				Labels:    map[string]string{"foo": "bar"},
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					{
						Name:  "busybox",
						Image: "gcr.io/google_containers/echoserver:1.4",
					},
				},
				RestartPolicy: api.RestartPolicyAlways,
			},
		}

		_, err := cs.Pods(ns).Create(pod)
		framework.ExpectNoError(err, "Creating pod %q in namespace %q", pod.Name, ns)
	}
}

func createReplicaSetOrDie(cs *release_1_4.Clientset, ns string, size int32, exclusive bool) {
	container := api.Container{
		Name:  "busybox",
		Image: "gcr.io/google_containers/echoserver:1.4",
	}
	if exclusive {
		container.Ports = []api.ContainerPort{
			{HostPort: 5555, ContainerPort: 5555},
		}
	}

	rs := &extensions.ReplicaSet{
		ObjectMeta: api.ObjectMeta{
			Name:      "rs",
			Namespace: ns,
		},
		Spec: extensions.ReplicaSetSpec{
			Replicas: &size,
			Selector: &extensions.LabelSelector{
				MatchLabels: map[string]string{"foo": "bar"},
			},
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: map[string]string{"foo": "bar"},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{container},
				},
			},
		},
	}

	_, err := cs.Extensions().ReplicaSets(ns).Create(rs)
	framework.ExpectNoError(err, "Creating replica set %q in namespace %q", rs.Name, ns)
}
