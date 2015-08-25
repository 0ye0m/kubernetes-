/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package master

import (
	"testing"

	"k8s.io/kubernetes/pkg/api"
)

func TestFindExternalAddress(t *testing.T) {
	expectedIP := "172.0.0.1"

	nodes := []*api.Node{new(api.Node), new(api.Node), new(api.Node)}
	nodes[0].Status.Addresses = []api.NodeAddress{{"ExternalIP", expectedIP}}
	nodes[1].Status.Addresses = []api.NodeAddress{{"LegacyHostIP", expectedIP}}
	nodes[2].Status.Addresses = []api.NodeAddress{{"ExternalIP", expectedIP}, {"LegacyHostIP", "172.0.0.2"}}

	for _, node := range nodes {
		ip, err := findExternalAddress(node)
		if err != nil {
			t.Errorf("error getting node external address: %s", err)
		}
		if ip != expectedIP {
			t.Errorf("expected ip to be %s, but was %s", expectedIP, ip)
		}
	}

	_, err := findExternalAddress(new(api.Node))
	if err == nil {
		t.Errorf("expected findExternalAddress to fail on a node with missing ip information")
	}
}
