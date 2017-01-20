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

package restclient

import (
	"path"
	"testing"

	"k8s.io/kubernetes/pkg/api"
)

func TestValidatesHostParameter(t *testing.T) {
	testCases := []struct {
		Host    string
		APIPath string

		URL string
		Err bool
	}{
		{"127.0.0.1", "", "http://127.0.0.1/" + api.Registry.GroupOrDie(api.GroupName).GroupVersion.Version, false},
		{"127.0.0.1:8080", "", "http://127.0.0.1:8080/" + api.Registry.GroupOrDie(api.GroupName).GroupVersion.Version, false},
		{"foo.bar.com", "", "http://foo.bar.com/" + api.Registry.GroupOrDie(api.GroupName).GroupVersion.Version, false},
		{"http://host/prefix", "", "http://host/prefix/" + api.Registry.GroupOrDie(api.GroupName).GroupVersion.Version, false},
		{"http://host", "", "http://host/" + api.Registry.GroupOrDie(api.GroupName).GroupVersion.Version, false},
		{"http://host", "/", "http://host/" + api.Registry.GroupOrDie(api.GroupName).GroupVersion.Version, false},
		{"http://host", "/other", "http://host/other/" + api.Registry.GroupOrDie(api.GroupName).GroupVersion.Version, false},
		{"host/server", "", "", true},
	}
	for i, testCase := range testCases {
		hosts, versionedAPIPath, err := DefaultServerURLsForHosts([]string{testCase.Host}, testCase.APIPath, api.Registry.GroupOrDie(api.GroupName).GroupVersion, false)
		switch {
		case err == nil && testCase.Err:
			t.Errorf("expected error but was nil")
			continue
		case err != nil && !testCase.Err:
			t.Errorf("unexpected error %v", err)
			continue
		case err != nil:
			continue
		case len(hosts) != 1:
			t.Errorf("unexpected length of returned hosts %v", hosts)
			continue
		}
		u := hosts[0]
		u.Path = path.Join(u.Path, versionedAPIPath)
		if e, a := testCase.URL, u.String(); e != a {
			t.Errorf("%d: expected host %s, got %s", i, e, a)
			continue
		}
	}
}
