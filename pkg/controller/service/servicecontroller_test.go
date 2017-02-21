/*
Copyright 2015 The Kubernetes Authors.

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
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset/fake"
	informers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	fakecloud "k8s.io/kubernetes/pkg/cloudprovider/providers/fake"
	"k8s.io/kubernetes/pkg/controller"
)

const region = "us-central"

func newService(name string, uid types.UID, serviceType v1.ServiceType) *v1.Service {
	return &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "namespace", UID: uid, SelfLink: testapi.Default.SelfLink("services", name)}, Spec: v1.ServiceSpec{Type: serviceType}}
}

func alwaysReady() bool { return true }

func newController() (*ServiceController, *fakecloud.FakeCloud, *fake.Clientset) {
	cloud := &fakecloud.FakeCloud{}
	cloud.Region = region

	client := fake.NewSimpleClientset()

	informerFactory := informers.NewSharedInformerFactory(client, controller.NoResyncPeriodFunc())
	serviceInformer := informerFactory.Core().V1().Services()
	nodeInformer := informerFactory.Core().V1().Nodes()

	controller, _ := New(cloud, client, serviceInformer, nodeInformer, "test-cluster")
	controller.nodeListerSynced = alwaysReady
	controller.serviceListerSynced = alwaysReady
	controller.eventRecorder = record.NewFakeRecorder(100)

	controller.init()
	cloud.Calls = nil     // ignore any cloud calls made in init()
	client.ClearActions() // ignore any client calls made in init()

	return controller, cloud, client
}

func defaultExternalService() *v1.Service {

	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "external-balancer",
			Namespace: "default",
		},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeLoadBalancer,
		},
	}

}

func TestCreateExternalLoadBalancer(t *testing.T) {
	table := []struct {
		service             *v1.Service
		expectErr           bool
		expectCreateAttempt bool
	}{
		{
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-external-balancer",
					Namespace: "default",
				},
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeClusterIP,
				},
			},
			expectErr:           false,
			expectCreateAttempt: false,
		},
		{
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "udp-service",
					Namespace: "default",
					SelfLink:  testapi.Default.SelfLink("services", "udp-service"),
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{{
						Port:     80,
						Protocol: v1.ProtocolUDP,
					}},
					Type: v1.ServiceTypeLoadBalancer,
				},
			},
			expectErr:           false,
			expectCreateAttempt: true,
		},
		{
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-service1",
					Namespace: "default",
					SelfLink:  testapi.Default.SelfLink("services", "basic-service1"),
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{{
						Port:     80,
						Protocol: v1.ProtocolTCP,
					}},
					Type: v1.ServiceTypeLoadBalancer,
				},
			},
			expectErr:           false,
			expectCreateAttempt: true,
		},
	}

	for _, item := range table {
		controller, cloud, client := newController()
		err, _ := controller.createLoadBalancerIfNeeded("foo/bar", item.service)
		if !item.expectErr && err != nil {
			t.Errorf("unexpected error: %v", err)
		} else if item.expectErr && err == nil {
			t.Errorf("expected error creating %v, got nil", item.service)
		}
		actions := client.Actions()
		if !item.expectCreateAttempt {
			if len(cloud.Calls) > 0 {
				t.Errorf("unexpected cloud provider calls: %v", cloud.Calls)
			}
			if len(actions) > 0 {
				t.Errorf("unexpected client actions: %v", actions)
			}
		} else {
			var balancer *fakecloud.FakeBalancer
			for k := range cloud.Balancers {
				if balancer == nil {
					b := cloud.Balancers[k]
					balancer = &b
				} else {
					t.Errorf("expected one load balancer to be created, got %v", cloud.Balancers)
					break
				}
			}
			if balancer == nil {
				t.Errorf("expected one load balancer to be created, got none")
			} else if balancer.Name != controller.loadBalancerName(item.service) ||
				balancer.Region != region ||
				balancer.Ports[0].Port != item.service.Spec.Ports[0].Port {
				t.Errorf("created load balancer has incorrect parameters: %v", balancer)
			}
			actionFound := false
			for _, action := range actions {
				if action.GetVerb() == "update" && action.GetResource().Resource == "services" {
					actionFound = true
				}
			}
			if !actionFound {
				t.Errorf("expected updated service to be sent to client, got these actions instead: %v", actions)
			}
		}
	}
}

// TODO: Finish converting and update comments
func TestUpdateNodesInExternalLoadBalancer(t *testing.T) {
	nodes := []*v1.Node{
		{ObjectMeta: metav1.ObjectMeta{Name: "node0"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "node73"}},
	}
	table := []struct {
		services            []*v1.Service
		expectedUpdateCalls []fakecloud.FakeUpdateBalancerCall
	}{
		{
			// No services present: no calls should be made.
			services:            []*v1.Service{},
			expectedUpdateCalls: nil,
		},
		{
			// Services do not have external load balancers: no calls should be made.
			services: []*v1.Service{
				newService("s0", "111", v1.ServiceTypeClusterIP),
				newService("s1", "222", v1.ServiceTypeNodePort),
			},
			expectedUpdateCalls: nil,
		},
		{
			// Services does have an external load balancer: one call should be made.
			services: []*v1.Service{
				newService("s0", "333", v1.ServiceTypeLoadBalancer),
			},
			expectedUpdateCalls: []fakecloud.FakeUpdateBalancerCall{
				{newService("s0", "333", v1.ServiceTypeLoadBalancer), nodes},
			},
		},
		{
			// Three services have an external load balancer: three calls.
			services: []*v1.Service{
				newService("s0", "444", v1.ServiceTypeLoadBalancer),
				newService("s1", "555", v1.ServiceTypeLoadBalancer),
				newService("s2", "666", v1.ServiceTypeLoadBalancer),
			},
			expectedUpdateCalls: []fakecloud.FakeUpdateBalancerCall{
				{newService("s0", "444", v1.ServiceTypeLoadBalancer), nodes},
				{newService("s1", "555", v1.ServiceTypeLoadBalancer), nodes},
				{newService("s2", "666", v1.ServiceTypeLoadBalancer), nodes},
			},
		},
		{
			// Two services have an external load balancer and two don't: two calls.
			services: []*v1.Service{
				newService("s0", "777", v1.ServiceTypeNodePort),
				newService("s1", "888", v1.ServiceTypeLoadBalancer),
				newService("s3", "999", v1.ServiceTypeLoadBalancer),
				newService("s4", "123", v1.ServiceTypeClusterIP),
			},
			expectedUpdateCalls: []fakecloud.FakeUpdateBalancerCall{
				{newService("s1", "888", v1.ServiceTypeLoadBalancer), nodes},
				{newService("s3", "999", v1.ServiceTypeLoadBalancer), nodes},
			},
		},
		{
			// One service has an external load balancer and one is nil: one call.
			services: []*v1.Service{
				newService("s0", "234", v1.ServiceTypeLoadBalancer),
				nil,
			},
			expectedUpdateCalls: []fakecloud.FakeUpdateBalancerCall{
				{newService("s0", "234", v1.ServiceTypeLoadBalancer), nodes},
			},
		},
	}
	for _, item := range table {
		controller, cloud, _ := newController()

		var services []*v1.Service
		for _, service := range item.services {
			services = append(services, service)
		}
		if err := controller.updateLoadBalancerHosts(services, nodes); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(item.expectedUpdateCalls, cloud.UpdateCalls) {
			t.Errorf("expected update calls mismatch, expected %+v, got %+v", item.expectedUpdateCalls, cloud.UpdateCalls)
		}
	}
}

func TestGetNodeConditionPredicate(t *testing.T) {
	tests := []struct {
		node         v1.Node
		expectAccept bool
		name         string
	}{
		{
			node:         v1.Node{},
			expectAccept: false,
			name:         "empty",
		},
		{
			node: v1.Node{
				Status: v1.NodeStatus{
					Conditions: []v1.NodeCondition{
						{Type: v1.NodeReady, Status: v1.ConditionTrue},
					},
				},
			},
			expectAccept: true,
			name:         "basic",
		},
		{
			node: v1.Node{
				Spec: v1.NodeSpec{Unschedulable: true},
				Status: v1.NodeStatus{
					Conditions: []v1.NodeCondition{
						{Type: v1.NodeReady, Status: v1.ConditionTrue},
					},
				},
			},
			expectAccept: false,
			name:         "unschedulable",
		},
	}
	pred := getNodeConditionPredicate()
	for _, test := range tests {
		accept := pred(&test.node)
		if accept != test.expectAccept {
			t.Errorf("Test failed for %s, expected %v, saw %v", test.name, test.expectAccept, accept)
		}
	}
}

func TestUpdateService(t *testing.T) {

	var controller *ServiceController
	var cloud *fakecloud.FakeCloud

	testCases := []struct {
		testName   string
		key        string
		updateFn   func() //Manupulate the structure
		srv        *v1.Service
		expectedFn func(error, time.Duration) error //Error comparision function
	}{
		{
			testName: "If updating a valid service",
			key:      "validKey",
			srv:      defaultExternalService(),
			updateFn: func() {

				controller, cloud, _ = newController()
				controller.cache.getOrCreate("validKey")

			},
			expectedFn: func(err error, retryDuration time.Duration) error {

				if err != nil {
					return err
				}
				if retryDuration != doNotRetry {
					return fmt.Errorf("retryDuration Expected=%v Obtained=%v", doNotRetry, retryDuration)
				}
				return nil
			},
		},
	}

	for _, tst := range testCases {
		tst.updateFn()
		srvCache := controller.cache.getOrCreate(tst.key)

		obtErr, retryDuration := controller.processServiceUpdate(srvCache, tst.srv, tst.key)
		if err := tst.expectedFn(obtErr, retryDuration); err != nil {
			t.Errorf("%v processServiceUpdate() %v", tst.testName, err)
		}
	}

}

func TestSyncService(t *testing.T) {

	var controller *ServiceController
	var cloud *fakecloud.FakeCloud

	testCases := []struct {
		testName    string
		key         string
		updateFn    func() //Function to manipulate the controller element to simulate error
		expectedErr error  //syncService() only returns error
	}{
		{
			testName: "if an invalid service name is synced",
			key:      "invalid/key/string",
			updateFn: func() {
				controller, cloud, _ = newController()

			},
			expectedErr: fmt.Errorf("unexpected key format: %q", "invalid/key/string"),
		},
		//TODO: see if we can add a test for valid but error throwing service, its difficult right now because synCService() currently runtime.HandleError
		{
			testName: "if valid service",
			key:      "external-balancer",
			updateFn: func() {
				controller, cloud, _ = newController()
				srv := controller.cache.getOrCreate("external-balancer")
				srv.state = defaultExternalService()
			},
			expectedErr: nil,
		},
	}

	for _, tst := range testCases {
		var printError bool
		tst.updateFn()
		obtainedErr := controller.syncService(tst.key)
		if obtainedErr != tst.expectedErr {
			printError = true
			if obtainedErr != nil && tst.expectedErr != nil {
				if obtainedErr.Error() == tst.expectedErr.Error() {
					//Check if the actual error matches, if yes then test PASSed
					printError = false
				}
			}
		}
		if printError {
			t.Errorf("%v Expected=%v Obtained=%v", tst.testName, tst.expectedErr, obtainedErr)
		}
	}
}

func TestDeleteService(t *testing.T) {

	var controller *ServiceController
	var cloud *fakecloud.FakeCloud

	testCases := []struct {
		testName        string
		updateFn        func()                                                //Update function used to manupulate srv and controller values
		checkExpectedFn func(srvErr error, retryDuration time.Duration) error //Function to check if the returned value is expected
	}{
		{
			testName: "If an invalid service is deleted",
			updateFn: func() {
				controller, cloud, _ = newController()
			},
			checkExpectedFn: func(srvErr error, retryDuration time.Duration) error {

				expectedError := "Service external-balancer not in cache even though the watcher thought it was. Ignoring the deletion."
				if srvErr == nil || srvErr.Error() != expectedError {
					//cannot be nil or Wrong error message
					return fmt.Errorf("Expected=%v Obtained=%v", expectedError, srvErr)
				}

				if retryDuration != doNotRetry {
					//Retry duration should match
					return fmt.Errorf("RetryDuration Expected=%v Obtained=%v", doNotRetry, retryDuration)
				}

				return nil
			},
		},
		{
			testName: "If cloudprovided failed to delete the service",
			updateFn: func() {
				controller, cloud, _ = newController()
				srv := controller.cache.getOrCreate("external-balancer")
				srv.state = defaultExternalService()
				cloud.Err = fmt.Errorf("Error Deleting the Loadbalancer")

			},
			checkExpectedFn: func(srvErr error, retryDuration time.Duration) error {

				expectedError := "Error Deleting the Loadbalancer"

				if srvErr == nil || srvErr.Error() != expectedError {
					return fmt.Errorf("Expected=%v Obtained=%v", expectedError, srvErr)
				}

				if retryDuration != minRetryDelay {
					return fmt.Errorf("RetryDuration Expected=%v Obtained=%v", minRetryDelay, retryDuration)
				}
				return nil
			},
		},
		{
			testName: "If delete was successful",
			updateFn: func() {

				controller, cloud, _ = newController()
				srv := controller.cache.getOrCreate("external-balancer")
				srv.state = defaultExternalService()

			},
			checkExpectedFn: func(srvErr error, retryDuration time.Duration) error {

				if srvErr != nil {
					return fmt.Errorf("Expected=nil Obtained=%v", srvErr)
				}

				if retryDuration != doNotRetry {
					//Retry duration should match
					return fmt.Errorf("RetryDuration Expected=%v Obtained=%v", doNotRetry, retryDuration)
				}

				return nil
			},
		},
	}

	for _, tst := range testCases {
		tst.updateFn()
		obtainedErr, retryDuration := controller.processServiceDeletion("external-balancer")
		if err := tst.checkExpectedFn(obtainedErr, retryDuration); err != nil {
			t.Errorf("%v processServiceDeletion() %v", tst.testName, err)
		}
	}

}

// Add unit testing for needsUpdate functions

func TestDoesExternalLoadBalancerNeedsUpdate(t *testing.T) {

	var oldSrv, newSrv *v1.Service

	testCases := []struct {
		testName       string //Name of the test case
		updateFn       func() //Function to update the old and new service varuables
		expectedResult bool   //needsupdate always returns bool

	}{
		{
			testName: "If the service type is different",
			updateFn: func() {
				oldSrv = defaultExternalService()
				newSrv = defaultExternalService()
				newSrv.Spec.Type = v1.ServiceTypeClusterIP
			},
			expectedResult: true,
		},
		{
			testName: "If the Ports are different",
			updateFn: func() {
				oldSrv = defaultExternalService()
				newSrv = defaultExternalService()
				oldSrv.Spec.Ports = []v1.ServicePort{
					{
						Port: 8000,
					},
					{
						Port: 9000,
					},
					{
						Port: 10000,
					},
				}
				newSrv.Spec.Ports = []v1.ServicePort{
					{
						Port: 8001,
					},
					{
						Port: 9001,
					},
					{
						Port: 10001,
					},
				}

			},
			expectedResult: true,
		},
		{
			testName: "If externel ip counts are different",
			updateFn: func() {
				oldSrv = defaultExternalService()
				newSrv = defaultExternalService()
				oldSrv.Spec.ExternalIPs = []string{"old.IP.1"}
				newSrv.Spec.ExternalIPs = []string{"new.IP.1", "new.IP.2"}
			},
			expectedResult: true,
		},
		{
			testName: "If externel ips are different",
			updateFn: func() {
				oldSrv = defaultExternalService()
				newSrv = defaultExternalService()
				oldSrv.Spec.ExternalIPs = []string{"old.IP.1", "old.IP.2"}
				newSrv.Spec.ExternalIPs = []string{"new.IP.1", "new.IP.2"}
			},
			expectedResult: true,
		},
		{
			testName: "If UID is different",
			updateFn: func() {
				oldSrv = defaultExternalService()
				newSrv = defaultExternalService()
				oldSrv.UID = types.UID("UID old")
				newSrv.UID = types.UID("UID new")
			},
			expectedResult: true,
		},
	}

	controller, _, _ := newController()
	for _, tst := range testCases {
		tst.updateFn()
		obtainedResult := controller.needsUpdate(oldSrv, newSrv)
		if obtainedResult != tst.expectedResult {
			t.Errorf("%v needsUpdate() should have returned %v but returned %v", tst.testName, tst.expectedResult, obtainedResult)
		}
	}
}

func TestServiceCache(t *testing.T) {

	sC := &serviceCache{serviceMap: make(map[string]*cachedService)} //ServiceCache

	testCases := []struct {
		testName   string
		runTest    func()
		expectedFn func() error
	}{
		{
			testName: "Add",
			runTest: func() {
				cS := sC.getOrCreate("addTest")
				cS.state = defaultExternalService()
			},
			expectedFn: func() error {
				//There must be exactly one element
				if len(sC.serviceMap) != 1 {
					return fmt.Errorf("Expected=1 Obtained=%d", len(sC.serviceMap))
				}
				return nil
			},
		},
		{
			testName: "Del",
			runTest: func() {
				sC.delete("addTest")

			},
			expectedFn: func() error {
				//Now it should have no element
				if len(sC.serviceMap) != 0 {
					return fmt.Errorf("Expected=0 Obtained=%d", len(sC.serviceMap))
				}
				return nil
			},
		},
		{
			testName: "SetandGet",
			runTest: func() {
				sC.set("addTest", &cachedService{state: defaultExternalService()})
			},
			expectedFn: func() error {
				//Now it should have one element
				Cs, bool := sC.get("addTest")
				if !bool {
					return fmt.Errorf("is Available Expected=true Obtained=%v", bool)
				}
				if Cs == nil {
					return fmt.Errorf("CachedService expected:non-nil Obtained=nil")
				}
				return nil
			},
		},
		{
			testName: "ListKeys",
			runTest: func() {
				//Add one more entry here
				sC.set("addTest1", &cachedService{state: defaultExternalService()})
			},
			expectedFn: func() error {
				//It should have two elements
				keys := sC.ListKeys()
				if len(keys) != 2 {
					return fmt.Errorf("Elementes Expected=2 Obtained=%v", len(keys))
				}
				if keys[0] != "addTest" || keys[1] != "addTest1" {
					return fmt.Errorf("Keys do not match")
				}
				return nil
			},
		},
		{
			testName: "GetbyKeys",
			runTest: func() {
				//Do nothing
			},
			expectedFn: func() error {
				//It should have two elements
				srv, isKey, err := sC.GetByKey("addTest")
				if srv == nil || isKey == false || err != nil {
					return fmt.Errorf("Expected(non-nil, true, nil) Obtained(%v,%v,%v)", srv, isKey, err)
				}
				return nil
			},
		},
		{
			testName: "allServices",
			runTest: func() {
				//Do nothing
			},
			expectedFn: func() error {
				//It should return two elements
				srvArray := sC.allServices()
				if len(srvArray) != 2 {
					return fmt.Errorf("Expected(2) Obtained(%v)", len(srvArray))
				}
				return nil
			},
		},
	}

	for _, tst := range testCases {
		tst.runTest()
		if err := tst.expectedFn(); err != nil {
			t.Errorf("%v returned %v", tst.testName, err)
		}
	}
}

//Test a utility functions as its not easy to unit test nodeSyncLoop directly
func TestUtilityFunctions_nodeSlicesEqualForLB(t *testing.T) {
	numNodes := 10
	nArray := make([]*v1.Node, 10)

	for i := 0; i < numNodes; i++ {
		nArray[i] = &v1.Node{}
		nArray[i].Name = fmt.Sprintf("node1")
	}
	if !nodeSlicesEqualForLB(nArray, nArray) {
		t.Errorf("Expected=true Obtained=false")
	}
}
