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

package bulk

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	bulkapi "k8s.io/apiserver/pkg/apis/bulk"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/handlers"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/server/mux"
)

// LocalAPIGroupInfo contains services to serve local group through bulk api.
type LocalAPIGroupInfo struct {

	// GroupVersion is uniqute group identifier
	GroupVersion schema.GroupVersion
	Preferred    bool

	Storage    map[string]rest.Storage
	Mapper     meta.RESTMapper
	Linker     runtime.SelfLinker
	Serializer runtime.NegotiatedSerializer

	Authorizer                 authorizer.Authorizer
	AuthroizationCachingPeriod time.Duration
}

// APIManagerFactory constructs instances of APIManager
type APIManagerFactory struct {
	Root                 string
	NegotiatedSerializer runtime.NegotiatedSerializer
	ContextMapper        request.RequestContextMapper
	Delegate             *APIManager
	WSTimeout            time.Duration
}

// APIManager installs web handlers for Bulk API.
type APIManager struct {

	// FIXME: support multiple group versions at the same time
	GroupVersion schema.GroupVersion

	// root is path prefix for installed endpoints
	Root string

	// Available api groups.
	apiGroups map[schema.GroupVersion]*registeredAPIGroup

	// Map api group -> preferred version.
	preferredVersion map[string]string

	negotiatedSerializer runtime.NegotiatedSerializer
	mapper               request.RequestContextMapper
	wsTimeout            time.Duration
}

// registeredAPIGroup is either LocalAPIGroupInfo or ProxiedAPIGroupInfo
type registeredAPIGroup struct {
	Local *LocalAPIGroupInfo
}

// New constructs new instance of *APIManager
func (f APIManagerFactory) New() *APIManager {
	glog.V(7).Infof("Construct new bulk.APIManager from %v", f)
	// TODO: merge negotiatedSerializer & ContextMapper from .Delegate

	// Merge API groups from delegate
	preferredVersion := make(map[string]string)
	groups := make(map[schema.GroupVersion]*registeredAPIGroup)
	if f.Delegate != nil {
		for k, v := range f.Delegate.apiGroups {
			glog.V(8).Infof("Reuse %v from delegated bulk.APIManager", k)
			groups[k] = v
		}
		for k, v := range f.Delegate.preferredVersion {
			preferredVersion[k] = v
		}
	}

	return &APIManager{
		// FIXME: Don't hardcode version
		GroupVersion:         schema.GroupVersion{Version: "v1alpha1", Group: bulkapi.GroupName},
		Root:                 f.Root,
		negotiatedSerializer: f.NegotiatedSerializer,
		mapper:               f.ContextMapper,
		wsTimeout:            f.WSTimeout,
		preferredVersion:     preferredVersion,
		apiGroups:            groups,
	}
}

// Install adds the handlers to the given mux.
func (m *APIManager) Install(c *mux.PathRecorderMux) {
	prefix := fmt.Sprintf("%s/bulk", m.Root)
	c.HandleFunc(prefix+"/watch", watchHTTPHandler{m}.ServeHTTP)
}

// UnregisterGroup unrgisters group from bulk manager.
func (m *APIManager) UnregisterGroup(gv schema.GroupVersion) (found bool) {
	glog.V(7).Infof("Unregister %v at bulk.APIManager", gv)
	if _, found := m.apiGroups[gv]; !found {
		return false
	}
	pversion := m.preferredVersion[gv.Group]
	if pversion == gv.Version {
		delete(m.preferredVersion, gv.Group)
	}
	delete(m.apiGroups, gv)
	return
}

// RegisterLocalGroup enables Bulk API for provided group.
func (m *APIManager) RegisterLocalGroup(agv LocalAPIGroupInfo) error {
	return m.registerAPIGroupCommon(agv.GroupVersion, registeredAPIGroup{Local: &agv}, agv.Preferred)
}

// RegisterAPIGroup enables Bulk API for provided group.
func (m *APIManager) registerAPIGroupCommon(gv schema.GroupVersion, agv registeredAPIGroup, preferredVersion bool) error {
	if _, found := m.apiGroups[gv]; found {
		return fmt.Errorf("group %v already registered", agv)
	}
	if _, found := m.preferredVersion[gv.Group]; preferredVersion && found {
		return fmt.Errorf("group %v already has preferred version", agv)
	}
	glog.V(7).Infof("Register %v at bulk.APIManager", gv)
	m.apiGroups[gv] = &agv
	if preferredVersion {
		m.preferredVersion[gv.Group] = gv.Version
	}
	return nil
}
