/*
Copyright 2014 Google Inc. All rights reserved.

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

package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/testapi"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/davecgh/go-spew/spew"
)

func newPodList(nPods int, nPorts int) *api.PodList {
	pods := []api.Pod{}
	for i := 0; i < nPods; i++ {
		p := api.Pod{
			TypeMeta:   api.TypeMeta{APIVersion: testapi.Version()},
			ObjectMeta: api.ObjectMeta{Name: fmt.Sprintf("pod%d", i)},
			Spec: api.PodSpec{
				Containers: []api.Container{{Ports: []api.ContainerPort{}}},
			},
			Status: api.PodStatus{
				PodIP: fmt.Sprintf("1.2.3.%d", 4+i),
				Conditions: []api.PodCondition{
					{
						Type:   api.PodReady,
						Status: api.ConditionFull,
					},
				},
			},
		}
		for j := 0; j < nPorts; j++ {
			p.Spec.Containers[0].Ports = append(p.Spec.Containers[0].Ports, api.ContainerPort{ContainerPort: 8080 + j})
		}
		pods = append(pods, p)
	}
	return &api.PodList{
		TypeMeta: api.TypeMeta{APIVersion: testapi.Version(), Kind: "PodList"},
		Items:    pods,
	}
}

func TestFindPort(t *testing.T) {
	pod := api.Pod{
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Ports: []api.ContainerPort{
						{
							Name:          "foo",
							ContainerPort: 111,
							HostPort:      1111,
						},
						{
							Name:          "bar",
							ContainerPort: 222,
							HostPort:      2222,
						},
						{
							Name:          "default",
							ContainerPort: 333,
							HostPort:      3333,
						},
					},
				},
			},
		},
	}

	emptyPortsPod := api.Pod{
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Ports: []api.ContainerPort{},
				},
			},
		},
	}

	singlePortPod := api.Pod{
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Ports: []api.ContainerPort{
						{
							ContainerPort: 444,
						},
					},
				},
			},
		},
	}

	noDefaultPod := api.Pod{
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Ports: []api.ContainerPort{
						{
							Name:          "foo",
							ContainerPort: 555,
						},
					},
				},
			},
		},
	}

	servicePort := 999

	tests := []struct {
		pod      api.Pod
		portName util.IntOrString

		wport int
		werr  bool
	}{
		{
			pod,
			util.IntOrString{Kind: util.IntstrString, StrVal: "foo"},
			111,
			false,
		},
		{
			pod,
			util.IntOrString{Kind: util.IntstrString, StrVal: "bar"},
			222,
			false,
		},
		{
			pod,
			util.IntOrString{Kind: util.IntstrInt, IntVal: 222},
			222,
			false,
		},
		{
			pod,
			util.IntOrString{Kind: util.IntstrInt, IntVal: 7000},
			7000,
			false,
		},
		{
			pod,
			util.IntOrString{Kind: util.IntstrString, StrVal: "baz"},
			0,
			true,
		},
		{
			emptyPortsPod,
			util.IntOrString{Kind: util.IntstrString, StrVal: "foo"},
			0,
			true,
		},
		{
			emptyPortsPod,
			util.IntOrString{Kind: util.IntstrString, StrVal: ""},
			servicePort,
			false,
		},
		{
			emptyPortsPod,
			util.IntOrString{Kind: util.IntstrInt, IntVal: 0},
			servicePort,
			false,
		},
		{
			singlePortPod,
			util.IntOrString{Kind: util.IntstrString, StrVal: ""},
			444,
			false,
		},
		{
			singlePortPod,
			util.IntOrString{Kind: util.IntstrInt, IntVal: 0},
			444,
			false,
		},
		{
			noDefaultPod,
			util.IntOrString{Kind: util.IntstrString, StrVal: ""},
			555,
			false,
		},
		{
			noDefaultPod,
			util.IntOrString{Kind: util.IntstrInt, IntVal: 0},
			555,
			false,
		},
	}
	for _, test := range tests {
		port, err := findPort(&test.pod, &api.Service{Spec: api.ServiceSpec{Port: servicePort, TargetPort: test.portName}})
		if port != test.wport {
			t.Errorf("Wrong port for %v: expected %d, Got %d", test.portName, test.wport, port)
		}
		if err == nil && test.werr {
			t.Errorf("unexpected non-error")
		}
		if err != nil && test.werr == false {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

type serverResponse struct {
	statusCode int
	obj        interface{}
}

func makeTestServer(t *testing.T, podResponse serverResponse, serviceResponse serverResponse, endpointsResponse serverResponse) (*httptest.Server, *util.FakeHandler) {
	fakePodHandler := util.FakeHandler{
		StatusCode:   podResponse.statusCode,
		ResponseBody: runtime.EncodeOrDie(testapi.Codec(), podResponse.obj.(runtime.Object)),
	}
	fakeServiceHandler := util.FakeHandler{
		StatusCode:   serviceResponse.statusCode,
		ResponseBody: runtime.EncodeOrDie(testapi.Codec(), serviceResponse.obj.(runtime.Object)),
	}
	fakeEndpointsHandler := util.FakeHandler{
		StatusCode:   endpointsResponse.statusCode,
		ResponseBody: runtime.EncodeOrDie(testapi.Codec(), endpointsResponse.obj.(runtime.Object)),
	}
	mux := http.NewServeMux()
	mux.Handle("/api/"+testapi.Version()+"/pods", &fakePodHandler)
	mux.Handle("/api/"+testapi.Version()+"/services", &fakeServiceHandler)
	mux.Handle("/api/"+testapi.Version()+"/endpoints", &fakeEndpointsHandler)
	mux.Handle("/api/"+testapi.Version()+"/endpoints/", &fakeEndpointsHandler)
	mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		t.Errorf("unexpected request: %v", req.RequestURI)
		res.WriteHeader(http.StatusNotFound)
	})
	return httptest.NewServer(mux), &fakeEndpointsHandler
}

func TestSyncEndpointsEmpty(t *testing.T) {
	testServer, _ := makeTestServer(t,
		serverResponse{http.StatusOK, newPodList(0, 0)},
		serverResponse{http.StatusOK, &api.ServiceList{}},
		serverResponse{http.StatusOK, &api.Endpoints{}})
	defer testServer.Close()
	client := client.NewOrDie(&client.Config{Host: testServer.URL, Version: testapi.Version()})
	endpoints := NewEndpointController(client)
	if err := endpoints.SyncServiceEndpoints(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSyncEndpointsError(t *testing.T) {
	testServer, _ := makeTestServer(t,
		serverResponse{http.StatusOK, newPodList(0, 0)},
		serverResponse{http.StatusInternalServerError, &api.ServiceList{}},
		serverResponse{http.StatusOK, &api.Endpoints{}})
	defer testServer.Close()
	client := client.NewOrDie(&client.Config{Host: testServer.URL, Version: testapi.Version()})
	endpoints := NewEndpointController(client)
	if err := endpoints.SyncServiceEndpoints(); err == nil {
		t.Errorf("unexpected non-error")
	}
}

func TestSyncEndpointsItemsPreserveNoSelector(t *testing.T) {
	serviceList := api.ServiceList{
		Items: []api.Service{
			{
				ObjectMeta: api.ObjectMeta{Name: "foo"},
				Spec:       api.ServiceSpec{},
			},
		},
	}
	testServer, endpointsHandler := makeTestServer(t,
		serverResponse{http.StatusOK, newPodList(0, 0)},
		serverResponse{http.StatusOK, &serviceList},
		serverResponse{http.StatusOK, &api.Endpoints{
			ObjectMeta: api.ObjectMeta{
				Name:            "foo",
				ResourceVersion: "1",
			},
			Subsets: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "6.7.8.9"}},
				Ports:     []api.EndpointPort{{Port: 1000}},
			}},
		}})
	defer testServer.Close()
	client := client.NewOrDie(&client.Config{Host: testServer.URL, Version: testapi.Version()})
	endpoints := NewEndpointController(client)
	if err := endpoints.SyncServiceEndpoints(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	endpointsHandler.ValidateRequestCount(t, 0)
}

func TestSyncEndpointsProtocolTCP(t *testing.T) {
	serviceList := api.ServiceList{
		Items: []api.Service{
			{
				ObjectMeta: api.ObjectMeta{Name: "foo", Namespace: "other"},
				Spec: api.ServiceSpec{
					Selector: map[string]string{},
					Protocol: api.ProtocolTCP,
				},
			},
		},
	}
	testServer, endpointsHandler := makeTestServer(t,
		serverResponse{http.StatusOK, newPodList(0, 0)},
		serverResponse{http.StatusOK, &serviceList},
		serverResponse{http.StatusOK, &api.Endpoints{
			ObjectMeta: api.ObjectMeta{
				Name:            "foo",
				ResourceVersion: "1",
			},
			Subsets: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "6.7.8.9"}},
				Ports:     []api.EndpointPort{{Port: 1000, Protocol: "TCP"}},
			}},
		}})
	defer testServer.Close()
	client := client.NewOrDie(&client.Config{Host: testServer.URL, Version: testapi.Version()})
	endpoints := NewEndpointController(client)
	if err := endpoints.SyncServiceEndpoints(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	endpointsHandler.ValidateRequestCount(t, 0)
}

func TestSyncEndpointsProtocolUDP(t *testing.T) {
	serviceList := api.ServiceList{
		Items: []api.Service{
			{
				ObjectMeta: api.ObjectMeta{Name: "foo", Namespace: "other"},
				Spec: api.ServiceSpec{
					Selector: map[string]string{},
					Protocol: api.ProtocolUDP,
				},
			},
		},
	}
	testServer, endpointsHandler := makeTestServer(t,
		serverResponse{http.StatusOK, newPodList(0, 0)},
		serverResponse{http.StatusOK, &serviceList},
		serverResponse{http.StatusOK, &api.Endpoints{
			ObjectMeta: api.ObjectMeta{
				Name:            "foo",
				ResourceVersion: "1",
			},
			Subsets: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "6.7.8.9"}},
				Ports:     []api.EndpointPort{{Port: 1000, Protocol: "UDP"}},
			}},
		}})
	defer testServer.Close()
	client := client.NewOrDie(&client.Config{Host: testServer.URL, Version: testapi.Version()})
	endpoints := NewEndpointController(client)
	if err := endpoints.SyncServiceEndpoints(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	endpointsHandler.ValidateRequestCount(t, 0)
}

func TestSyncEndpointsItemsEmptySelectorSelectsAll(t *testing.T) {
	serviceList := api.ServiceList{
		Items: []api.Service{
			{
				ObjectMeta: api.ObjectMeta{Name: "foo", Namespace: "other"},
				Spec: api.ServiceSpec{
					Selector: map[string]string{},
				},
			},
		},
	}
	testServer, endpointsHandler := makeTestServer(t,
		serverResponse{http.StatusOK, newPodList(1, 1)},
		serverResponse{http.StatusOK, &serviceList},
		serverResponse{http.StatusOK, &api.Endpoints{
			ObjectMeta: api.ObjectMeta{
				Name:            "foo",
				ResourceVersion: "1",
			},
			Subsets: []api.EndpointSubset{},
		}})
	defer testServer.Close()
	client := client.NewOrDie(&client.Config{Host: testServer.URL, Version: testapi.Version()})
	endpoints := NewEndpointController(client)
	if err := endpoints.SyncServiceEndpoints(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	data := runtime.EncodeOrDie(testapi.Codec(), &api.Endpoints{
		ObjectMeta: api.ObjectMeta{
			Name:            "foo",
			ResourceVersion: "1",
		},
		Subsets: []api.EndpointSubset{{
			Addresses: []api.EndpointAddress{{IP: "1.2.3.4", TargetRef: &api.ObjectReference{Kind: "Pod", Name: "pod0"}}},
			Ports:     []api.EndpointPort{{Port: 8080, Protocol: "TCP"}},
		}},
	})
	endpointsHandler.ValidateRequest(t, "/api/"+testapi.Version()+"/endpoints/foo?namespace=other", "PUT", &data)
}

func TestSyncEndpointsItemsPreexisting(t *testing.T) {
	serviceList := api.ServiceList{
		Items: []api.Service{
			{
				ObjectMeta: api.ObjectMeta{Name: "foo", Namespace: "bar"},
				Spec: api.ServiceSpec{
					Selector: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
	}
	testServer, endpointsHandler := makeTestServer(t,
		serverResponse{http.StatusOK, newPodList(1, 1)},
		serverResponse{http.StatusOK, &serviceList},
		serverResponse{http.StatusOK, &api.Endpoints{
			ObjectMeta: api.ObjectMeta{
				Name:            "foo",
				ResourceVersion: "1",
			},
			Subsets: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "6.7.8.9"}},
				Ports:     []api.EndpointPort{{Port: 1000}},
			}},
		}})
	defer testServer.Close()
	client := client.NewOrDie(&client.Config{Host: testServer.URL, Version: testapi.Version()})
	endpoints := NewEndpointController(client)
	if err := endpoints.SyncServiceEndpoints(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	data := runtime.EncodeOrDie(testapi.Codec(), &api.Endpoints{
		ObjectMeta: api.ObjectMeta{
			Name:            "foo",
			ResourceVersion: "1",
		},
		Subsets: []api.EndpointSubset{{
			Addresses: []api.EndpointAddress{{IP: "1.2.3.4", TargetRef: &api.ObjectReference{Kind: "Pod", Name: "pod0"}}},
			Ports:     []api.EndpointPort{{Port: 8080, Protocol: "TCP"}},
		}},
	})
	endpointsHandler.ValidateRequest(t, "/api/"+testapi.Version()+"/endpoints/foo?namespace=bar", "PUT", &data)
}

func TestSyncEndpointsItemsPreexistingIdentical(t *testing.T) {
	serviceList := api.ServiceList{
		Items: []api.Service{
			{
				ObjectMeta: api.ObjectMeta{Name: "foo", Namespace: api.NamespaceDefault},
				Spec: api.ServiceSpec{
					Selector: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
	}
	testServer, endpointsHandler := makeTestServer(t,
		serverResponse{http.StatusOK, newPodList(1, 1)},
		serverResponse{http.StatusOK, &serviceList},
		serverResponse{http.StatusOK, &api.Endpoints{
			ObjectMeta: api.ObjectMeta{
				ResourceVersion: "1",
			},
			Subsets: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "1.2.3.4", TargetRef: &api.ObjectReference{Kind: "Pod", Name: "pod0"}}},
				Ports:     []api.EndpointPort{{Port: 8080, Protocol: "TCP"}},
			}},
		}})
	defer testServer.Close()
	client := client.NewOrDie(&client.Config{Host: testServer.URL, Version: testapi.Version()})
	endpoints := NewEndpointController(client)
	if err := endpoints.SyncServiceEndpoints(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	endpointsHandler.ValidateRequest(t, "/api/"+testapi.Version()+"/endpoints/foo?namespace=default", "GET", nil)
}

func TestSyncEndpointsItems(t *testing.T) {
	serviceList := api.ServiceList{
		Items: []api.Service{
			{
				ObjectMeta: api.ObjectMeta{Name: "foo", Namespace: "other"},
				Spec: api.ServiceSpec{
					Selector: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
	}
	testServer, endpointsHandler := makeTestServer(t,
		serverResponse{http.StatusOK, newPodList(3, 2)},
		serverResponse{http.StatusOK, &serviceList},
		serverResponse{http.StatusOK, &api.Endpoints{}})
	defer testServer.Close()
	client := client.NewOrDie(&client.Config{Host: testServer.URL, Version: testapi.Version()})
	endpoints := NewEndpointController(client)
	if err := endpoints.SyncServiceEndpoints(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	expectedSubsets := []api.EndpointSubset{{
		Addresses: []api.EndpointAddress{
			{IP: "1.2.3.4", TargetRef: &api.ObjectReference{Kind: "Pod", Name: "pod0"}},
			{IP: "1.2.3.5", TargetRef: &api.ObjectReference{Kind: "Pod", Name: "pod1"}},
			{IP: "1.2.3.6", TargetRef: &api.ObjectReference{Kind: "Pod", Name: "pod2"}},
		},
		Ports: []api.EndpointPort{
			{Port: 8080, Protocol: "TCP"},
		},
	}}
	data := runtime.EncodeOrDie(testapi.Codec(), &api.Endpoints{
		ObjectMeta: api.ObjectMeta{
			ResourceVersion: "",
		},
		Subsets: sortSubsets(expectedSubsets),
	})
	endpointsHandler.ValidateRequest(t, "/api/"+testapi.Version()+"/endpoints?namespace=other", "POST", &data)
}

func TestSyncEndpointsPodError(t *testing.T) {
	serviceList := api.ServiceList{
		Items: []api.Service{
			{
				Spec: api.ServiceSpec{
					Selector: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
	}
	testServer, _ := makeTestServer(t,
		serverResponse{http.StatusInternalServerError, &api.PodList{}},
		serverResponse{http.StatusOK, &serviceList},
		serverResponse{http.StatusOK, &api.Endpoints{}})
	defer testServer.Close()
	client := client.NewOrDie(&client.Config{Host: testServer.URL, Version: testapi.Version()})
	endpoints := NewEndpointController(client)
	if err := endpoints.SyncServiceEndpoints(); err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestPackSubsets(t *testing.T) {
	testCases := []struct {
		given  []api.EndpointSubset
		expect []api.EndpointSubset
	}{
		{
			given:  []api.EndpointSubset{{Addresses: []api.EndpointAddress{}, Ports: []api.EndpointPort{}}},
			expect: []api.EndpointSubset{},
		}, {
			given: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "1.2.3.4"}},
				Ports:     []api.EndpointPort{{Port: 111}},
			}},
			expect: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "1.2.3.4"}},
				Ports:     []api.EndpointPort{{Port: 111}},
			}},
		}, {
			given: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "1.2.3.4"}},
				Ports:     []api.EndpointPort{{Port: 111}},
			}, {
				Addresses: []api.EndpointAddress{{IP: "1.2.3.4"}},
				Ports:     []api.EndpointPort{{Port: 222}},
			}},
			expect: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "1.2.3.4"}},
				Ports:     []api.EndpointPort{{Port: 111}, {Port: 222}},
			}},
		}, {
			given: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "1.2.3.4"}},
				Ports:     []api.EndpointPort{{Port: 111}},
			}, {
				Addresses: []api.EndpointAddress{{IP: "1.2.3.5"}},
				Ports:     []api.EndpointPort{{Port: 111}},
			}},
			expect: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "1.2.3.4"}, {IP: "1.2.3.5"}},
				Ports:     []api.EndpointPort{{Port: 111}},
			}},
		}, {
			given: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "1.2.3.4"}},
				Ports:     []api.EndpointPort{{Port: 111}},
			}, {
				Addresses: []api.EndpointAddress{{IP: "1.2.3.5"}},
				Ports:     []api.EndpointPort{{Port: 222}},
			}},
			expect: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "1.2.3.4"}},
				Ports:     []api.EndpointPort{{Port: 111}},
			}, {
				Addresses: []api.EndpointAddress{{IP: "1.2.3.5"}},
				Ports:     []api.EndpointPort{{Port: 222}},
			}},
		}, {
			given: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "1.2.3.4"}},
				Ports:     []api.EndpointPort{{Port: 111}},
			}, {
				Addresses: []api.EndpointAddress{{IP: "1.2.3.5"}},
				Ports:     []api.EndpointPort{{Port: 222}},
			}, {
				Addresses: []api.EndpointAddress{{IP: "1.2.3.6"}},
				Ports:     []api.EndpointPort{{Port: 111}},
			}, {
				Addresses: []api.EndpointAddress{{IP: "1.2.3.5"}},
				Ports:     []api.EndpointPort{{Port: 333}},
			}},
			expect: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "1.2.3.4"}, {IP: "1.2.3.6"}},
				Ports:     []api.EndpointPort{{Port: 111}},
			}, {
				Addresses: []api.EndpointAddress{{IP: "1.2.3.5"}},
				Ports:     []api.EndpointPort{{Port: 222}, {Port: 333}},
			}},
		},
	}

	for i, tc := range testCases {
		result := packSubsets(tc.given)
		if !reflect.DeepEqual(result, sortSubsets(tc.expect)) {
			t.Errorf("case[%d]: expected %s, got %s", i, spew.Sprintf("%v", sortSubsets(tc.expect)), spew.Sprintf("%v", result))
		}
	}
}
