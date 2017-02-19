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

package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	vsphere "k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere"
	"k8s.io/kubernetes/test/e2e/framework"
	"os"
	"strconv"
)

var _ = framework.KubeDescribe("Volume Placement [Feature:Volume]", func() {
	f := framework.NewDefaultFramework("volume-placement")
	var (
		c                  clientset.Interface
		ns                 string
		node1Name          string
		node1KeyValueLabel map[string]string
		node2Name          string
		node2KeyValueLabel map[string]string
		testSeup           bool
	)
	BeforeEach(func() {
		framework.SkipUnlessProviderIs("vsphere")
		c = f.ClientSet
		ns = f.Namespace.Name
		framework.ExpectNoError(framework.WaitForAllNodesSchedulable(c, framework.TestContext.NodeSchedulableTimeout))
		nodeList := framework.GetReadySchedulableNodesOrDie(f.ClientSet)
		node1Name, node1KeyValueLabel, node2Name, node2KeyValueLabel = testSetupVolumePlacement(c, ns, nodeList)
		testSeup = true
	})
	/*
		Steps
		1. Remove labels assigned to node 1 and node 2
		2. Delete VMDK volume
	*/
	AddCleanupAction(func() {
		if len(node1KeyValueLabel) > 0 {
			framework.RemoveLabelOffNode(c, node1Name, "vsphere_e2e_label")
		}
		if len(node2KeyValueLabel) > 0 {
			framework.RemoveLabelOffNode(c, node2Name, "vsphere_e2e_label")
		}
	})

	framework.KubeDescribe("provision pod on node with matching labels", func() {
		var (
			vsp         *vsphere.VSphere
			volumePaths []string
			err         error
		)
		/*
			Steps
			1. Create VMDK volume
			2. Find two nodes with the status available and ready for scheduling.
			3. Add labels to the both nodes. - (vsphere_e2e_label: Random UUID)

		*/
		BeforeEach(func() {
			if !testSeup {
				c = f.ClientSet
				ns = f.Namespace.Name
				framework.ExpectNoError(framework.WaitForAllNodesSchedulable(c, framework.TestContext.NodeSchedulableTimeout))
				nodeList := framework.GetReadySchedulableNodesOrDie(f.ClientSet)
				node1Name, node1KeyValueLabel, node2Name, node2KeyValueLabel = testSetupVolumePlacement(c, ns, nodeList)
				testSeup = true
			}
			By("creating vmdk")
			vsp, err = vsphere.GetVSphere()
			Expect(err).NotTo(HaveOccurred())
			volumePath, err := createVSphereVolume(vsp, nil)
			Expect(err).NotTo(HaveOccurred())
			volumePaths = append(volumePaths, volumePath)
		})

		AfterEach(func() {
			if len(volumePaths) > 0 {
				vsp, err := vsphere.GetVSphere()
				Expect(err).NotTo(HaveOccurred())
				for _, volumePath := range volumePaths {
					vsp.DeleteVolume(volumePath)
				}
			}
			volumePaths = nil
		})
		/*
			Steps

			1. Create pod Spec with volume path of the vmdk and NodeSelector set to label assigned to node1.
			2. Create pod and wait for pod to become ready.
			3. Verify volume is attached to the node1.
			4. Create empty file on the volume to verify volume is writable.
			5. Verify newly created file and previously created files exist on the volume.
			6. Delete pod.
			7. Wait for volume to be detached from the node1.
			8. Repeat Step 1 to 7 and make sure back to back pod creation on same worker node with the same volume is working as expected.

		*/

		It("should create and delete pod with the same volume source on the same worker node", func() {
			var volumeFiles []string
			pod := createPodWithVolumeAndNodeSelector(c, ns, vsp, node1Name, node1KeyValueLabel, volumePaths)

			// Create empty files on the mounted volumes on the pod to verify volume is writable
			// Verify newly and previously created files present on the volume mounted on the pod
			newEmptyFileName := "/mnt/volume1/" + ns + "_1" + ".txt"
			volumeFiles = append(volumeFiles, newEmptyFileName)
			createAndVerifyFilesOnVolume(ns, pod.Name, []string{newEmptyFileName}, volumeFiles)
			deletePodAndWaitForVolumeToDetach(c, ns, vsp, node1Name, pod, volumePaths)

			By("Creating pod on the same node: " + node1Name)
			pod = createPodWithVolumeAndNodeSelector(c, ns, vsp, node1Name, node1KeyValueLabel, volumePaths)

			// Create empty files on the mounted volumes on the pod to verify volume is writable
			// Verify newly and previously created files present on the volume mounted on the pod
			newEmptyFileName = "/mnt/volume1/" + ns + "_2" + ".txt"
			volumeFiles = append(volumeFiles, newEmptyFileName)
			createAndVerifyFilesOnVolume(ns, pod.Name, []string{newEmptyFileName}, volumeFiles)
			deletePodAndWaitForVolumeToDetach(c, ns, vsp, node1Name, pod, volumePaths)
		})

		/*
			Steps

			1. Create pod Spec with volume path of the vmdk1 and NodeSelector set to node1's label.
			2. Create pod and wait for POD to become ready.
			3. Verify volume is attached to the node1.
			4. Create empty file on the volume to verify volume is writable.
			5. Verify newly created file and previously created files exist on the volume.
			6. Delete pod.
			7. Wait for volume to be detached from the node1.
			8. Create pod Spec with volume path of the vmdk1 and NodeSelector set to node2's label.
			9. Create pod and wait for pod to become ready.
			10. Verify volume is attached to the node2.
			11. Create empty file on the volume to verify volume is writable.
			12. Verify newly created file and previously created files exist on the volume.
			13. Delete pod.
		*/

		It("should create and delete pod with the same volume source attach/detach to different worker nodes", func() {
			var volumeFiles []string

			pod := createPodWithVolumeAndNodeSelector(c, ns, vsp, node1Name, node1KeyValueLabel, volumePaths)
			// Create empty files on the mounted volumes on the pod to verify volume is writable
			// Verify newly and previously created files present on the volume mounted on the pod
			newEmptyFileName := "/mnt/volume1/" + ns + "_1" + ".txt"
			volumeFiles = append(volumeFiles, newEmptyFileName)
			createAndVerifyFilesOnVolume(ns, pod.Name, []string{newEmptyFileName}, volumeFiles)
			deletePodAndWaitForVolumeToDetach(c, ns, vsp, node1Name, pod, volumePaths)

			By("Creating pod on the another node: " + node2Name)
			pod = createPodWithVolumeAndNodeSelector(c, ns, vsp, node2Name, node2KeyValueLabel, volumePaths)

			newEmptyFileName = "/mnt/volume1/" + ns + "_2" + ".txt"
			volumeFiles = append(volumeFiles, newEmptyFileName)
			// Create empty files on the mounted volumes on the pod to verify volume is writable
			// Verify newly and previously created files present on the volume mounted on the pod
			createAndVerifyFilesOnVolume(ns, pod.Name, []string{newEmptyFileName}, volumeFiles)

			deletePodAndWaitForVolumeToDetach(c, ns, vsp, node2Name, pod, volumePaths)
		})

		/*
			Test multiple volumes from same datastore within the same pod
			1. Create volumes - vmdk2
			2. Create pod Spec with volume path of vmdk1 (vmdk1 is created in test setup) and vmdk2.
			3. Create pod using spec created in step-2 and wait for pod to become ready.
			4. Verify both volumes are attached to the node on which pod are created. Write some data to make sure volume are accessible.
			5. Delete pod.
			6. Wait for vmdk1 and vmdk2 to be detached from node.
			7. Create pod using spec created in step-2 and wait for pod to become ready.
			8. Verify both volumes are attached to the node on which PODs are created. Verify volume contents are matching with the content written in step 4.
			9. Delete POD.
			10. Wait for vmdk1 and vmdk2 to be detached from node.
		*/

		It("should create and delete pod with multiple volumes from same datastore", func() {
			var (
				volumeFiles []string
			)
			By("creating another vmdk")
			volumePath, err := createVSphereVolume(vsp, nil)
			Expect(err).NotTo(HaveOccurred())
			volumePaths = append(volumePaths, volumePath)

			By("Creating pod on the node: " + node1Name + " with volume :" + volumePaths[0] + " and volume:" + volumePaths[1])
			pod := createPodWithVolumeAndNodeSelector(c, ns, vsp, node1Name, node1KeyValueLabel, volumePaths)

			// Create empty files on the mounted volumes on the pod to verify volume is writable
			// Verify newly and previously created files present on the volume mounted on the pod
			newEmptyFileNameVolume1 := "/mnt/volume1/" + ns + "_1" + ".txt"
			newEmptyFileNameVolume2 := "/mnt/volume2/" + ns + "_1" + ".txt"
			volumeFiles = append(volumeFiles, newEmptyFileNameVolume1)
			volumeFiles = append(volumeFiles, newEmptyFileNameVolume2)
			createAndVerifyFilesOnVolume(ns, pod.Name, []string{newEmptyFileNameVolume1, newEmptyFileNameVolume2}, volumeFiles)

			deletePodAndWaitForVolumeToDetach(c, ns, vsp, node1Name, pod, volumePaths)

			By("Creating pod on the node: " + node1Name + " with volume :" + volumePaths[0] + " and volume:" + volumePaths[1])
			pod = createPodWithVolumeAndNodeSelector(c, ns, vsp, node1Name, node1KeyValueLabel, volumePaths)
			// Create empty files on the mounted volumes on the pod to verify volume is writable
			// Verify newly and previously created files present on the volume mounted on the pod
			newEmptyFileNameVolume1 = "/mnt/volume1/" + ns + "_2" + ".txt"
			newEmptyFileNameVolume2 = "/mnt/volume2/" + ns + "_2" + ".txt"
			volumeFiles = append(volumeFiles, newEmptyFileNameVolume1)
			volumeFiles = append(volumeFiles, newEmptyFileNameVolume2)
			createAndVerifyFilesOnVolume(ns, pod.Name, []string{newEmptyFileNameVolume1, newEmptyFileNameVolume2}, volumeFiles)
		})

		/*
			Test multiple volumes from different datastore within the same pod
			1. Create volumes - vmdk2 on non default shared datastore.
			2. Create pod Spec with volume path of vmdk1 (vmdk1 is created in test setup on default datastore) and vmdk2.
			3. Create pod using spec created in step-2 and wait for pod to become ready.
			4. Verify both volumes are attached to the node on which pod are created. Write some data to make sure volume are accessible.
			5. Delete pod.
			6. Wait for vmdk1 and vmdk2 to be detached from node.
			7. Create pod using spec created in step-2 and wait for pod to become ready.
			8. Verify both volumes are attached to the node on which PODs are created. Verify volume contents are matching with the content written in step 4.
			9. Delete POD.
			10. Wait for vmdk1 and vmdk2 to be detached from node.
		*/
		It("should create and delete pod with multiple volumes from different datastore", func() {
			var (
				volumeFiles []string
			)
			By("creating another vmdk on non default shared datastore")
			/* TODO :
			 	Refactor the code once https://github.com/kubernetes/kubernetes/pull/41113 is merged to master.
				Use volumeOptions.Datastore once PR is merged.
			*/
			cfg := vsphere.GetVSphereConfig()
			cfg.Global.Datastore = os.Getenv("VSPHERE_SECOND_SHARED_DATASTORE")
			vsp2, err := vsphere.GetVSphereWithCustomConfig(cfg)
			Expect(err).NotTo(HaveOccurred())
			volumePath, err := createVSphereVolume(vsp2, nil)
			Expect(err).NotTo(HaveOccurred())
			volumePaths = append(volumePaths, volumePath)

			By("Creating pod on the node: " + node1Name + " with volume :" + volumePaths[0] + " and volume:" + volumePaths[1])
			pod := createPodWithVolumeAndNodeSelector(c, ns, vsp, node1Name, node1KeyValueLabel, volumePaths)

			// Create empty files on the mounted volumes on the pod to verify volume is writable
			// Verify newly and previously created files present on the volume mounted on the pod
			newEmptyFileNameVolume1 := "/mnt/volume1/" + ns + "_1" + ".txt"
			newEmptyFileNameVolume2 := "/mnt/volume2/" + ns + "_1" + ".txt"
			volumeFiles = append(volumeFiles, newEmptyFileNameVolume1)
			volumeFiles = append(volumeFiles, newEmptyFileNameVolume2)
			createAndVerifyFilesOnVolume(ns, pod.Name, []string{newEmptyFileNameVolume1, newEmptyFileNameVolume2}, volumeFiles)

			deletePodAndWaitForVolumeToDetach(c, ns, vsp, node1Name, pod, volumePaths)

			By("Creating pod on the node: " + node1Name + " with volume :" + volumePaths[0] + " and volume:" + volumePaths[1])
			pod = createPodWithVolumeAndNodeSelector(c, ns, vsp, node1Name, node1KeyValueLabel, volumePaths)
			// Create empty files on the mounted volumes on the pod to verify volume is writable
			// Verify newly and previously created files present on the volume mounted on the pod
			newEmptyFileNameVolume1 = "/mnt/volume1/" + ns + "_2" + ".txt"
			newEmptyFileNameVolume2 = "/mnt/volume2/" + ns + "_2" + ".txt"
			volumeFiles = append(volumeFiles, newEmptyFileNameVolume1)
			volumeFiles = append(volumeFiles, newEmptyFileNameVolume2)
			createAndVerifyFilesOnVolume(ns, pod.Name, []string{newEmptyFileNameVolume1, newEmptyFileNameVolume2}, volumeFiles)
			deletePodAndWaitForVolumeToDetach(c, ns, vsp, node1Name, pod, volumePaths)
		})

		/*
			Test Back-to-back pod creation/deletion with different volume sources on the same worker node
			    1. Create volumes - vmdk2
			    2. Create pod Spec - pod-SpecA with volume path of vmdk1 and NodeSelector set to label assigned to node1.
			    3. Create pod Spec - pod-SpecB with volume path of vmdk2 and NodeSelector set to label assigned to node1.
			    4. Create pod-A using pod-SpecA and wait for pod to become ready.
			    5. Create pod-B using pod-SpecB and wait for POD to become ready.
			    6. Verify volumes are attached to the node.
			    7. Create empty file on the volume to make sure volume is accessible. (Perform this step on pod-A and pod-B)
			    8. Verify file created in step 5 is present on the volume. (perform this step on pod-A and pod-B)
			    9. Delete pod-A and pod-B
			    10. Repeatedly (5 times) perform step 4 to 9 and verify associated volume's content is matching.
			    11. Wait for vmdk1 and vmdk2 to be detached from node.
		*/
		It("test back to back pod creation and deletion with different volume sources on the same worker node", func() {
			var (
				podA                *v1.Pod
				podB                *v1.Pod
				testvolumePathsPodA []string
				testvolumePathsPodB []string
				podAFiles           []string
				podBFiles           []string
			)

			defer func() {
				By("clean up undeleted pods")
				if podA != nil {
					_ = c.CoreV1().Pods(ns).Delete(podA.Name, nil)
				}
				if podB != nil {
					_ = c.CoreV1().Pods(ns).Delete(podB.Name, nil)
				}
				By("wait for volumes to be detached from the node: " + node1Name)
				for _, volumePath := range volumePaths {
					waitForVSphereDiskToDetach(vsp, volumePath, types.NodeName(node1Name))
				}
			}()

			testvolumePathsPodA = append(testvolumePathsPodA, volumePaths[0])

			// Create another VMDK Volume
			By("creating another vmdk")
			volumePath, err := createVSphereVolume(vsp, nil)
			Expect(err).NotTo(HaveOccurred())
			volumePaths = append(volumePaths, volumePath)
			testvolumePathsPodB = append(testvolumePathsPodA, volumePath)

			for index := 0; index < 5; index++ {
				By("Creating pod-A on the node: " + node1Name + " with volume:" + testvolumePathsPodA[0])
				podA = createPodWithVolumeAndNodeSelector(c, ns, vsp, node1Name, node1KeyValueLabel, testvolumePathsPodA)
				By("Creating pod-B on the node: " + node1Name + " with volume:" + testvolumePathsPodB[0])
				podB = createPodWithVolumeAndNodeSelector(c, ns, vsp, node1Name, node1KeyValueLabel, testvolumePathsPodB)

				podAFileName := "/mnt/volume1/" + "podA_" + ns + "-" + strconv.Itoa(index+1) + ".txt"
				podBFileName := "/mnt/volume1/" + "podB_" + ns + "_" + strconv.Itoa(index+1) + ".txt"

				podAFiles = append(podAFiles, podAFileName)
				podBFiles = append(podBFiles, podBFileName)

				// Create empty files on the mounted volumes on the pod to verify volume is writable
				By("Creating empty file on volume mounted on pod-A")
				createEmptyFileOnVSphereVolume(ns, podA.Name, podAFileName)

				By("Creating empty file volume mounted on pod-B")
				createEmptyFileOnVSphereVolume(ns, podB.Name, podBFileName)

				// Verify newly and previously created files present on the volume mounted on the pod
				By("Verify newly Created file and previously created files present on volume mounted on pod-A")
				verifyFilesExistOnVSphereVolume(ns, podA.Name, podAFiles)
				By("Verify newly Created file and previously created files present on volume mounted on pod-B")
				verifyFilesExistOnVSphereVolume(ns, podB.Name, podBFiles)

				By("Deleting pod-A")
				err = c.CoreV1().Pods(ns).Delete(podA.Name, nil)
				Expect(err).NotTo(HaveOccurred())
				podA = nil

				By("Deleting pod-B")
				err = c.CoreV1().Pods(ns).Delete(podB.Name, nil)
				Expect(err).NotTo(HaveOccurred())
				podB = nil
			}
		})
	})
})

func testSetupVolumePlacement(client clientset.Interface, namespace string, nodes *v1.NodeList) (node1Name string, node1KeyValueLabel map[string]string, node2Name string, node2KeyValueLabel map[string]string) {
	if len(nodes.Items) != 0 {
		node1Name = nodes.Items[0].Name
		node2Name = nodes.Items[1].Name
	} else {
		framework.Failf("Unable to find ready and schedulable Node")
	}
	node1LabelValue := "vsphere_e2e_" + string(uuid.NewUUID())
	node1KeyValueLabel = make(map[string]string)
	node1KeyValueLabel["vsphere_e2e_label"] = node1LabelValue
	framework.AddOrUpdateLabelOnNode(client, node1Name, "vsphere_e2e_label", node1LabelValue)

	node2LabelValue := "vsphere_e2e_" + string(uuid.NewUUID())
	node2KeyValueLabel = make(map[string]string)
	node2KeyValueLabel["vsphere_e2e_label"] = node2LabelValue
	framework.AddOrUpdateLabelOnNode(client, node2Name, "vsphere_e2e_label", node2LabelValue)
	return node1Name, node1KeyValueLabel, node2Name, node2KeyValueLabel
}

func createPodWithVolumeAndNodeSelector(client clientset.Interface, namespace string, vsp *vsphere.VSphere, nodeName string, nodeKeyValueLabel map[string]string, volumePaths []string) *v1.Pod {
	var pod *v1.Pod
	var err error
	By("Creating pod on the node: " + nodeName)
	podspec := getVSpherePodSpecWithVolumePaths(volumePaths, nodeKeyValueLabel, nil)

	pod, err = client.CoreV1().Pods(namespace).Create(podspec)
	Expect(err).NotTo(HaveOccurred())
	By("Waiting for pod to be ready")
	Expect(framework.WaitForPodNameRunningInNamespace(client, pod.Name, namespace)).To(Succeed())

	By("Verify volume is attached to the node: " + nodeName)
	for _, volumePath := range volumePaths {
		isAttached, err := verifyVSphereDiskAttached(vsp, volumePath, types.NodeName(nodeName))
		Expect(err).NotTo(HaveOccurred())
		Expect(isAttached).To(BeTrue(), "disk:"+volumePath+" is not attached with the node")
	}
	return pod
}

func createAndVerifyFilesOnVolume(namespace string, podname string, newEmptyfilesToCreate []string, filesToCheck []string) {
	// Create empty files on the mounted volumes on the pod to verify volume is writable
	By("Creating empty file on volume mounted on:" + podname)
	createEmptyFilesOnVSphereVolume(namespace, podname, newEmptyfilesToCreate)

	// Verify newly and previously created files present on the volume mounted on the pod
	By("Verify newly Created file and previously created files present on volume mounted on:" + podname)
	verifyFilesExistOnVSphereVolume(namespace, podname, filesToCheck)
}

func deletePodAndWaitForVolumeToDetach(client clientset.Interface, namespace string, vsp *vsphere.VSphere, nodeName string, pod *v1.Pod, volumePaths []string) {
	var err error
	By("Deleting pod")
	err = client.CoreV1().Pods(namespace).Delete(pod.Name, nil)
	Expect(err).NotTo(HaveOccurred())

	By("Waiting for volume to be detached from the node")
	for _, volumePath := range volumePaths {
		waitForVSphereDiskToDetach(vsp, volumePath, types.NodeName(nodeName))
	}
}
