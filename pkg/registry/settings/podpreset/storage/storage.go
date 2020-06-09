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

package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	settingsapi "k8s.io/kubernetes/pkg/apis/settings"
	"k8s.io/kubernetes/pkg/registry/settings/podpreset"
)

// REST implements a RESTStorage for pod presets against etcd.
type REST struct {
	*genericregistry.Store
}

// NewREST returns a RESTStorage object that will work against pod presets.
func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, error) {
	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &settingsapi.PodPreset{} },
		NewListFunc:              func() runtime.Object { return &settingsapi.PodPresetList{} },
		DefaultQualifiedResource: settingsapi.Resource("podpresets"),

		CreateStrategy: podpreset.Strategy,
		UpdateStrategy: podpreset.Strategy,
		DeleteStrategy: podpreset.Strategy,

		// TODO: define table converter that exposes more than name/creation timestamp
		TableConvertor: rest.NewDefaultTableConvertor(settingsapi.Resource("podpresets")),
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
