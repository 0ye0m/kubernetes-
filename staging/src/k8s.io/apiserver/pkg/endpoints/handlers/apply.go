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

package handlers

import (
	"context"
	"fmt"

	"github.com/ghodss/yaml"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apply"
	"k8s.io/apimachinery/pkg/apply/parse"
	"k8s.io/apimachinery/pkg/apply/strategy"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/kube-openapi/pkg/util/proto"
)

type applyPatcher struct {
	*patcher

	model proto.Schema
}

// TODO(apelisse): workflowId needs to be passed as a query
// param/header, and a better defaulting needs to be defined too.
const workflowId = "default"

func (p *applyPatcher) convertCurrentVersion(obj runtime.Object) (map[string]interface{}, error) {
	vo, err := p.unsafeConvertor.ConvertToVersion(obj, p.kind.GroupVersion())
	if err != nil {
		return nil, err
	}
	return runtime.DefaultUnstructuredConverter.ToUnstructured(vo)
}

func (p *applyPatcher) extractLastIntent(obj runtime.Object, workflow string) (map[string]interface{}, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, fmt.Errorf("couldn't get accessor: %v", err)
	}
	last := make(map[string]interface{})
	if accessor.GetLastApplied()[workflow] != "" {
		if err := json.Unmarshal([]byte(accessor.GetLastApplied()[workflow]), &last); err != nil {
			return nil, fmt.Errorf("couldn't unmarshal last applied field: %v", err)
		}
	}
	return last, nil
}

func (p *applyPatcher) getNewIntent() (map[string]interface{}, error) {
	patch := make(map[string]interface{})
	if err := yaml.Unmarshal(p.patchBytes, &patch); err != nil {
		return nil, fmt.Errorf("couldn't unmarshal patch object: %v (patch: %v)", err, string(p.patchBytes))
	}
	return patch, nil
}

func (p *applyPatcher) convertResultToUnversioned(result apply.Result) (runtime.Object, error) {
	voutput, err := p.creater.New(p.kind)
	if err != nil {
		return nil, fmt.Errorf("failed to create empty output object: %v", err)
	}

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(result.MergedResult.(map[string]interface{}), voutput)
	if err != nil {
		return nil, fmt.Errorf("failed to convert merge result back: %v", err)
	}
	p.defaulter.Default(voutput)

	uoutput, err := p.toUnversioned(voutput)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to unversioned: %v", err)
	}

	return uoutput, nil
}

func (p *applyPatcher) applyPatchToCurrentObject(currentObject runtime.Object) (runtime.Object, error) {
	current, err := p.convertCurrentVersion(currentObject)
	if err != nil {
		return nil, fmt.Errorf("failed to convert current object: %v", err)
	}

	lastIntent, err := p.extractLastIntent(currentObject, workflowId)
	if err != nil {
		return nil, fmt.Errorf("failed to extract last intent: %v", err)
	}
	newIntent, err := p.getNewIntent()
	if err != nil {
		return nil, fmt.Errorf("failed to get new intent: %v", err)
	}

	element, err := parse.CreateElement(lastIntent, newIntent, current, p.model)
	if err != nil {
		return nil, fmt.Errorf("failed to parse elements: %v", err)
	}
	result, err := element.Merge(strategy.Create(strategy.Options{}))
	if err != nil {
		return nil, fmt.Errorf("failed to merge elements: %v", err)
	}

	output, err := p.convertResultToUnversioned(result)
	if err != nil {
		return nil, fmt.Errorf("failed to convert merge result: %v", err)
	}

	// TODO(apelisse): Update last applied
	// TODO(apelisse): Also update last-applied on the create path
	// TODO(apelisse): Check for conflicts with other lastApplied
	// and report actionable errors to users.

	return output, nil
}

// applyPatch is called every time GuaranteedUpdate asks for the updated object,
// and is given the currently persisted object as input.
func (p *patcher) applyPatch(_ context.Context, _, currentObject runtime.Object) (runtime.Object, error) {
	// Make sure we actually have a persisted currentObject
	p.trace.Step("About to apply patch")
	if hasUID, err := hasUID(currentObject); err != nil {
		return nil, err
	} else if !hasUID {
		return nil, errors.NewNotFound(p.resource.GroupResource(), p.name)
	}

	objToUpdate, err := p.mechanism.applyPatchToCurrentObject(currentObject)
	if err != nil {
		return nil, err
	}
	if err := checkName(objToUpdate, p.name, p.namespace, p.namer); err != nil {
		return nil, err
	}
	return objToUpdate, nil
}
