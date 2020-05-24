/*
Copyright 2020 The Kubernetes Authors.

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

package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	genericfeatures "k8s.io/apiserver/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	apiservertesting "k8s.io/kubernetes/cmd/kube-apiserver/app/testing"
	"k8s.io/kubernetes/test/integration/etcd"
	"k8s.io/kubernetes/test/integration/framework"
	"sigs.k8s.io/yaml"
)

// namespace used for all tests, do not change this
const resetFieldsNamespace = "reset-fields-namespace"

// TestResetFields makes sure that fieldManager does not own fields reset by the storage strategy.
func TestApplyResetFields(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, genericfeatures.ServerSideApply, true)()
	server, err := apiservertesting.StartTestServer(t, apiservertesting.NewDefaultTestServerOptions(), []string{"--disable-admission-plugins", "ServiceAccount,TaintNodesByCondition"}, framework.SharedEtcd())
	if err != nil {
		t.Fatal(err)
	}
	defer server.TearDownFn()

	client, err := kubernetes.NewForConfig(server.ClientConfig)
	if err != nil {
		t.Fatal(err)
	}
	dynamicClient, err := dynamic.NewForConfig(server.ClientConfig)
	if err != nil {
		t.Fatal(err)
	}

	// create CRDs so we can make sure that custom resources do not get lost
	etcd.CreateTestCRDs(t, apiextensionsclientset.NewForConfigOrDie(server.ClientConfig), false, etcd.GetCustomResourceDefinitionData()...)

	if _, err := client.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: resetFieldsNamespace}}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	createData := etcd.GetEtcdStorageDataForNamespace(resetFieldsNamespace)

	// gather resources to test
	_, resourceLists, err := client.Discovery().ServerGroupsAndResources()
	if err != nil {
		t.Fatalf("Failed to get ServerGroupsAndResources with error: %+v", err)
	}

	// for _, resourreList := range resourceLists {
	// 	for _, resource := range resourceList.APIResources {

	// 	}
	// }

	for _, resourceList := range resourceLists {
		for _, resource := range resourceList.APIResources {
			fmt.Println(resourceList.GroupVersion, resource.Group, resource.Version, resource.Name)
			if !strings.HasSuffix(resource.Name, "/status") {
				continue
			}
			mapping, err := createMapping(resourceList.GroupVersion, resource)
			if err != nil {
				t.Fatal(err)
			}
			t.Run(mapping.Resource.String(), func(t *testing.T) {
				if _, ok := ignoreList[mapping.Resource]; ok {
					t.Skip()
				}

				status, ok := statusData[mapping.Resource]
				if !ok {
					status = statusDefault
				}
				newResource, ok := createData[mapping.Resource]
				if !ok {
					t.Fatalf("no test data for %s.  Please add a test for your new type to etcd.GetEtcdStorageData().", mapping.Resource)
				}

				newObj := unstructured.Unstructured{}
				if err := json.Unmarshal([]byte(newResource.Stub), &newObj.Object); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(status), &newObj.Object); err != nil {
					t.Fatal(err)
				}

				namespace := resetFieldsNamespace
				if mapping.Scope == meta.RESTScopeRoot {
					namespace = ""
				}
				name := newObj.GetName()
				rsc := dynamicClient.Resource(mapping.Resource).Namespace(namespace)
				_, err := rsc.Create(context.TODO(), &newObj, metav1.CreateOptions{FieldManager: "create_test"})
				if err != nil {
					t.Fatal(err)
				}

				statusObj := unstructured.Unstructured{}
				if err := json.Unmarshal([]byte(status), &statusObj.Object); err != nil {
					t.Fatal(err)
				}
				statusObj.SetAPIVersion(mapping.GroupVersionKind.GroupVersion().String())
				statusObj.SetKind(mapping.GroupVersionKind.Kind)
				statusObj.SetName(name)
				statusYAML, err := yaml.Marshal(statusObj.Object)
				if err != nil {
					t.Fatal(err)
				}

				True := true
				obj, err := dynamicClient.
					Resource(mapping.Resource).
					Namespace(namespace).
					Patch(context.TODO(), name, types.ApplyPatchType, statusYAML, metav1.PatchOptions{FieldManager: "apply_status_test", Force: &True}, "status")
				if err != nil {
					t.Fatalf("Failed to apply: %v", err)
				}

				accessor, err := meta.Accessor(obj)
				if err != nil {
					t.Fatalf("Failed to get meta accessor: %v:\n%v", err, obj)
				}

				managedFields := accessor.GetManagedFields()
				if managedFields == nil {
					t.Fatal("Empty managed fields")
				}

				createFields, err := getManagedFieldsFor(managedFields, "create_test")
				if err != nil {
					t.Fatal(err)
				}
				statusFields, err := getManagedFieldsFor(managedFields, "apply_status_test")
				if err != nil {
					t.Fatal(err)
				}

				for field := range createFields {
					if _, exists := statusFields[field]; exists {
						t.Errorf("found overlapping field ownership: %v", field)
					}
				}

				if err := rsc.Delete(context.TODO(), name, *metav1.NewDeleteOptions(0)); err != nil {
					t.Fatalf("deleting final object failed: %v", err)
				}
			})
		}
	}
}

func getManagedFieldsFor(managedFields []metav1.ManagedFieldsEntry, manager string) (map[string]interface{}, error) {
	for _, entry := range managedFields {
		if entry.Manager == manager {

			fields := make(map[string]interface{})
			if err := json.Unmarshal(entry.FieldsV1.Raw, &fields); err != nil {
				return nil, err
			}
			return fields, nil
		}
	}
	return nil, fmt.Errorf("manager not found: %s", manager)
}
