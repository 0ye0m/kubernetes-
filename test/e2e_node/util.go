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

package e2e_node

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	v1alpha1 "k8s.io/kubernetes/pkg/apis/componentconfig/v1alpha1"
	"k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/stats"
	"k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/gomega"
)

// TODO(random-liu): Get this automatically from kubelet flag.
var kubeletAddress = flag.String("kubelet-address", "http://127.0.0.1:10255", "Host and port of the kubelet")

var startServices = flag.Bool("start-services", true, "If true, start local node services")
var stopServices = flag.Bool("stop-services", true, "If true, stop local node services after running tests")

func getNodeSummary() (*stats.Summary, error) {
	req, err := http.NewRequest("GET", *kubeletAddress+"/stats/summary", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build http request: %v", err)
	}
	req.Header.Add("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get /stats/summary: %v", err)
	}

	defer resp.Body.Close()
	contentsBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read /stats/summary: %+v", resp)
	}

	decoder := json.NewDecoder(strings.NewReader(string(contentsBytes)))
	summary := stats.Summary{}
	err = decoder.Decode(&summary)
	if err != nil {
		return nil, fmt.Errorf("failed to parse /stats/summary to go struct: %+v", resp)
	}
	return &summary, nil
}

// Returns the current KubeletConfiguration
func getCurrentKubeletConfig() (*componentconfig.KubeletConfiguration, error) {
	resp := pollConfigz(5*time.Minute, 5*time.Second)
	kubeCfg, err := decodeConfigz(resp)
	if err != nil {
		return nil, err
	}
	return kubeCfg, nil
}

func getCurrentKubeletConfigMap(f *framework.Framework) (*api.ConfigMap, error) {
	return f.ClientSet.Core().ConfigMaps("kube-system").Get(fmt.Sprintf("kubelet-%s", framework.TestContext.NodeName))
}

// Creates or updates the configmap for KubeletConfiguration, waits for the Kubelet to restart
// with the new configuration. Returns an error if the configuration after waiting 40 seconds
// doesn't match what you attempted to set.
func setKubeletConfiguration(f *framework.Framework, kubeCfg *componentconfig.KubeletConfiguration) error {
	const (
		restartGap = 30 * time.Second
	)

	// Check whether a configmap for KubeletConfiguration already exists
	_, err := getCurrentKubeletConfigMap(f)
	if fmt.Sprintf("%v", err) == fmt.Sprintf("configmaps \"kubelet-%s\" not found", framework.TestContext.NodeName) {
		_, err := createConfigMap(f, kubeCfg)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		// The configmap exists, update it instead of creating it.
		_, err := updateConfigMap(f, kubeCfg)
		if err != nil {
			return err
		}
	}

	// Wait for the Kubelet to restart.
	time.Sleep(restartGap)

	// Retrieve the new config and compare it to the one we attempted to set
	newKubeCfg, err := getCurrentKubeletConfig()
	if err != nil {
		return err
	}

	// Return an error if the desired config is not in use by now
	if !reflect.DeepEqual(*kubeCfg, *newKubeCfg) {
		return fmt.Errorf("Either the Kubelet did not restart or it did not present the modified configuration via /configz after restarting.")
	}
	return nil
}

// Causes the test to fail, or returns a status 200 response from the /configz endpoint
func pollConfigz(timeout time.Duration, pollInterval time.Duration) *http.Response {
	endpoint := fmt.Sprintf("http://127.0.0.1:8080/api/v1/proxy/nodes/%s/configz", framework.TestContext.NodeName)
	client := &http.Client{}
	req, err := http.NewRequest("GET", endpoint, nil)
	framework.ExpectNoError(err)
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	Eventually(func() bool {
		resp, err = client.Do(req)
		if err != nil {
			glog.Errorf("Failed to get /configz, retrying. Error: %v", err)
			return false
		}
		if resp.StatusCode != 200 {
			glog.Errorf("/configz response status not 200, retrying. Response was: %+v", resp)
			return false
		}
		return true
	}, timeout, pollInterval).Should(Equal(true))
	return resp
}

// Decodes the http response  from /configz and returns a componentconfig.KubeletConfiguration (internal type).
func decodeConfigz(resp *http.Response) (*componentconfig.KubeletConfiguration, error) {
	// This hack because /configz reports the following structure:
	// {"componentconfig": {the JSON representation of v1alpha1.KubeletConfiguration}}
	type configzWrapper struct {
		ComponentConfig v1alpha1.KubeletConfiguration `json:"componentconfig"`
	}

	configz := configzWrapper{}
	kubeCfg := componentconfig.KubeletConfiguration{}

	contentsBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(contentsBytes, &configz)
	if err != nil {
		return nil, err
	}

	err = api.Scheme.Convert(&configz.ComponentConfig, &kubeCfg, nil)
	if err != nil {
		return nil, err
	}

	return &kubeCfg, nil
}

// Uses KubeletConfiguration to create a `kubelet-<node-name>` ConfigMap in the "kube-system" namespace.
func createConfigMap(f *framework.Framework, kubeCfg *componentconfig.KubeletConfiguration) (*api.ConfigMap, error) {
	kubeCfgExt := v1alpha1.KubeletConfiguration{}
	api.Scheme.Convert(kubeCfg, &kubeCfgExt, nil)

	bytes, err := json.Marshal(kubeCfgExt)
	framework.ExpectNoError(err)

	cmap, err := f.ClientSet.Core().ConfigMaps("kube-system").Create(&api.ConfigMap{
		ObjectMeta: api.ObjectMeta{
			Name: fmt.Sprintf("kubelet-%s", framework.TestContext.NodeName),
		},
		Data: map[string]string{
			"kubelet.config": string(bytes),
		},
	})
	if err != nil {
		return nil, err
	}
	return cmap, nil
}

// Similar to createConfigMap, except this updates an existing ConfigMap.
func updateConfigMap(f *framework.Framework, kubeCfg *componentconfig.KubeletConfiguration) (*api.ConfigMap, error) {
	kubeCfgExt := v1alpha1.KubeletConfiguration{}
	api.Scheme.Convert(kubeCfg, &kubeCfgExt, nil)

	bytes, err := json.Marshal(kubeCfgExt)
	framework.ExpectNoError(err)

	cmap, err := f.ClientSet.Core().ConfigMaps("kube-system").Update(&api.ConfigMap{
		ObjectMeta: api.ObjectMeta{
			Name: fmt.Sprintf("kubelet-%s", framework.TestContext.NodeName),
		},
		Data: map[string]string{
			"kubelet.config": string(bytes),
		},
	})
	if err != nil {
		return nil, err
	}
	return cmap, nil
}
