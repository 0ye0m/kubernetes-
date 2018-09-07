/*
Copyright 2018 The Kubernetes Authors.

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

package integration

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/test/integration/fixtures"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInternalVersionIsHandlerVersion(t *testing.T) {
	tearDown, apiExtensionClient, dynamicClient, err := fixtures.StartDefaultServerWithClients(t)
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown()

	noxuDefinition := fixtures.NewMultipleVersionNoxuCRD(apiextensionsv1beta1.NamespaceScoped)

	assert.Equal(t, "v1beta1", noxuDefinition.Spec.Versions[0].Name)
	assert.Equal(t, "v1beta2", noxuDefinition.Spec.Versions[1].Name)
	assert.True(t, noxuDefinition.Spec.Versions[1].Storage)

	noxuDefinition, err = fixtures.CreateNewCustomResourceDefinition(noxuDefinition, apiExtensionClient, dynamicClient)
	if err != nil {
		t.Fatal(err)
	}

	ns := "not-the-default"

	noxuNamespacedResourceClient := newNamespacedCustomResourceVersionedClient(ns, dynamicClient, noxuDefinition, "v1beta1") // use the non-storage version v1beta1

	t.Logf("Creating foo")
	noxuInstanceToCreate := fixtures.NewNoxuInstance(ns, "foo")
	_, err = noxuNamespacedResourceClient.Create(noxuInstanceToCreate, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// update validation via update because the cache priming in CreateNewCustomResourceDefinition will fail otherwise
	t.Logf("Updating CRD to check apiVersion")
	noxuDefinition, err = updateCustomResourceDefinitionWithRetry(apiExtensionClient, noxuDefinition.Name, func(crd *apiextensionsv1beta1.CustomResourceDefinition) {
		crd.Spec.Validation = &apiextensionsv1beta1.CustomResourceValidation{
			OpenAPIV3Schema: &apiextensionsv1beta1.JSONSchemaProps{
				Properties: map[string]apiextensionsv1beta1.JSONSchemaProps{
					"apiVersion": {
						Pattern: "^v1beta2$", // this means we can only patch v1beta2 version, the storage version
					},
				},
				Required: []string{"apiVersion"},
			},
		}
	})
	assert.NoError(t, err)

	// patch until the apiVersion validation fails, i.e. the patch against v1beta1 is not valid.
	// Note: if there is no conversion going on on etcd read, it's always the storage version we apply the patch against.
	//       This is wrong of course and the following loop would time out because there is never an Invalid error returned.
	t.Logf("Waiting for patch of non-storage version to fail")
	i := 0
	err = wait.PollImmediate(time.Millisecond*100, wait.ForeverTestTimeout, func() (bool, error) {
		patch := []byte(fmt.Sprintf(`{"i": %d}`, i))
		i++

		_, err := noxuNamespacedResourceClient.Patch("foo", types.MergePatchType, patch, metav1.UpdateOptions{})
		if err == nil {
			return false, nil
		}

		// apiVersion validation fails.
		if errors.IsInvalid(err) && strings.Contains(err.Error(), "apiVersion in body should match '^v1beta2$'") {
			return true, nil
		}

		// work around "grpc: the client connection is closing" error
		// TODO: fix the grpc error
		if err, ok := err.(*errors.StatusError); ok && err.Status().Code == http.StatusInternalServerError {
			return false, nil
		}

		return false, err
	})
	assert.NoError(t, err)
}

func TestVersionedNamspacedScopedCRD(t *testing.T) {
	tearDown, apiExtensionClient, dynamicClient, err := fixtures.StartDefaultServerWithClients(t)
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown()

	noxuDefinition := fixtures.NewMultipleVersionNoxuCRD(apiextensionsv1beta1.NamespaceScoped)
	noxuDefinition, err = fixtures.CreateNewCustomResourceDefinition(noxuDefinition, apiExtensionClient, dynamicClient)
	if err != nil {
		t.Fatal(err)
	}

	ns := "not-the-default"
	testSimpleCRUD(t, ns, noxuDefinition, dynamicClient)
}

func TestVersionedClusterScopedCRD(t *testing.T) {
	tearDown, apiExtensionClient, dynamicClient, err := fixtures.StartDefaultServerWithClients(t)
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown()

	noxuDefinition := fixtures.NewMultipleVersionNoxuCRD(apiextensionsv1beta1.ClusterScoped)
	noxuDefinition, err = fixtures.CreateNewCustomResourceDefinition(noxuDefinition, apiExtensionClient, dynamicClient)
	if err != nil {
		t.Fatal(err)
	}

	ns := ""
	testSimpleCRUD(t, ns, noxuDefinition, dynamicClient)
}

func TestStoragedVersionInNamespacedCRDStatus(t *testing.T) {
	noxuDefinition := fixtures.NewMultipleVersionNoxuCRD(apiextensionsv1beta1.NamespaceScoped)
	ns := "not-the-default"
	testStoragedVersionInCRDStatus(t, ns, noxuDefinition)
}

func TestStoragedVersionInClusterScopedCRDStatus(t *testing.T) {
	noxuDefinition := fixtures.NewMultipleVersionNoxuCRD(apiextensionsv1beta1.ClusterScoped)
	ns := ""
	testStoragedVersionInCRDStatus(t, ns, noxuDefinition)
}

func testStoragedVersionInCRDStatus(t *testing.T, ns string, noxuDefinition *apiextensionsv1beta1.CustomResourceDefinition) {
	versionsV1Beta1Storage := []apiextensionsv1beta1.CustomResourceDefinitionVersion{
		{
			Name:    "v1beta1",
			Served:  true,
			Storage: true,
		},
		{
			Name:    "v1beta2",
			Served:  true,
			Storage: false,
		},
	}
	versionsV1Beta2Storage := []apiextensionsv1beta1.CustomResourceDefinitionVersion{
		{
			Name:    "v1beta1",
			Served:  true,
			Storage: false,
		},
		{
			Name:    "v1beta2",
			Served:  true,
			Storage: true,
		},
	}
	tearDown, apiExtensionClient, dynamicClient, err := fixtures.StartDefaultServerWithClients(t)
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown()

	noxuDefinition.Spec.Versions = versionsV1Beta1Storage
	noxuDefinition, err = fixtures.CreateNewCustomResourceDefinition(noxuDefinition, apiExtensionClient, dynamicClient)
	if err != nil {
		t.Fatal(err)
	}

	// The storage version list should be initilized to storage version
	crd, err := apiExtensionClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(noxuDefinition.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if e, a := []string{"v1beta1"}, crd.Status.StoredVersions; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, got %v", e, a)
	}

	// Changing CRD storage version should be reflected immediately
	crd.Spec.Versions = versionsV1Beta2Storage
	_, err = apiExtensionClient.ApiextensionsV1beta1().CustomResourceDefinitions().Update(crd)
	if err != nil {
		t.Fatal(err)
	}
	crd, err = apiExtensionClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(noxuDefinition.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if e, a := []string{"v1beta1", "v1beta2"}, crd.Status.StoredVersions; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, got %v", e, a)
	}

	err = fixtures.DeleteCustomResourceDefinition(crd, apiExtensionClient)
	if err != nil {
		t.Fatal(err)
	}
}
