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

package deployment

import (
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/testapi"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/runtime"
)

func TestSelectableFieldLabelConversions(t *testing.T) {
	apitesting.TestSelectableFieldLabelConversionsOfKind(t,
		testapi.Extensions.GroupVersion().String(),
		"Deployment",
		DeploymentToSelectableFields(&extensions.Deployment{}),
		nil,
	)
}

func TestStatusUpdates(t *testing.T) {
	tests := []struct {
		old      runtime.Object
		obj      runtime.Object
		expected runtime.Object
	}{
		{
			old:      newDeployment(map[string]string{"test": "label"}, map[string]string{"test": "annotation"}),
			obj:      newDeployment(map[string]string{"test": "label", "sneaky": "label"}, map[string]string{"test": "annotation"}),
			expected: newDeployment(map[string]string{"test": "label"}, map[string]string{"test": "annotation"}),
		},
		{
			old:      newDeployment(map[string]string{"test": "label"}, map[string]string{"test": "annotation"}),
			obj:      newDeployment(map[string]string{"test": "label"}, map[string]string{"test": "annotation", "sneaky": "annotation"}),
			expected: newDeployment(map[string]string{"test": "label"}, map[string]string{"test": "annotation", "sneaky": "annotation"}),
		},
	}

	for _, test := range tests {
		deploymentStatusStrategy{}.PrepareForUpdate(api.NewContext(), test.obj, test.old)
		if !reflect.DeepEqual(test.expected, test.obj) {
			t.Errorf("Unexpected object mismatch! Expected:\n%#v\ngot:\n%#v", test.expected, test.obj)
		}
	}
}

func TestDeploymentStrategyUpdates(t *testing.T) {
	old := newDeployment(map[string]string{"test": "label"}, map[string]string{"test": "annotation"})
	old.Spec.Strategy = *newDeploymentStrategy(extensions.RollingUpdateDeploymentStrategyType)
	obj := newDeployment(map[string]string{"test": "label"}, map[string]string{"test": "annotation"})
	obj.Spec.Strategy = *newDeploymentStrategy(extensions.RollingUpdateDeploymentStrategyType)
	resetDeploymentStrategyType(obj, extensions.RecreateDeploymentStrategyType)
	expected := newDeployment(map[string]string{"test": "label"}, map[string]string{"test": "annotation"})
	expected.Spec.Strategy = *newDeploymentStrategy(extensions.RecreateDeploymentStrategyType)

	deploymentStrategy{}.PrepareForUpdate(api.NewContext(), obj, old)
	if !reflect.DeepEqual(expected.Spec, obj.Spec) {
		t.Errorf("Unexpected object mismatch! Expected:\n%#v\ngot:\n%#v", expected, obj)
	}
}

func newDeployment(labels, annotations map[string]string) *extensions.Deployment {
	return &extensions.Deployment{
		ObjectMeta: api.ObjectMeta{
			Name:        "test",
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: extensions.DeploymentSpec{
			Replicas: 1,
			Strategy: extensions.DeploymentStrategy{
				Type: extensions.RecreateDeploymentStrategyType,
			},
			Template: api.PodTemplateSpec{
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:  "test",
							Image: "test",
						},
					},
				},
			},
		},
	}
}

func newDeploymentStrategy(strategyType extensions.DeploymentStrategyType) *extensions.DeploymentStrategy {
	strategy := &extensions.DeploymentStrategy{
		Type: strategyType,
	}
	if strategyType == extensions.RollingUpdateDeploymentStrategyType {
		strategy.RollingUpdate = &extensions.RollingUpdateDeployment{}
	}
	return strategy
}

func resetDeploymentStrategyType(deployment *extensions.Deployment, strategyType extensions.DeploymentStrategyType) {
	deployment.Spec.Strategy.Type = strategyType
}
