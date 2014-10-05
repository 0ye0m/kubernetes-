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

package api

import (
	stderrs "errors"

	"code.google.com/p/go.net/context"
)

// Context carries values across API boundaries.
type Context interface {
	Value(key interface{}) interface{}
}

// The key type is unexported to prevent collisions
type key int

// namespaceKey is the context key for the request namespace.
const namespaceKey key = 0

// NewContext instantiates a base context object for request flows.
func NewContext() Context {
	return context.TODO()
}

// NewDefaultContext instantiates a base context object for request flows in the default namespace
func NewDefaultContext() Context {
	return WithNamespace(NewContext(), NamespaceDefault)
}

// WithValue returns a copy of parent in which the value associated with key is val.
func WithValue(parent Context, key interface{}, val interface{}) Context {
	internalCtx, ok := parent.(context.Context)
	if !ok {
		panic(stderrs.New("Invalid context type"))
	}
	return context.WithValue(internalCtx, key, val)
}

// WithNamespace returns a copy of parent in which the namespace value is set
func WithNamespace(parent Context, namespace string) Context {
	return WithValue(parent, namespaceKey, namespace)
}

// NamespaceFrom returns the value of the namespace key on the ctx
func NamespaceFrom(ctx Context) (string, bool) {
	namespace, ok := ctx.Value(namespaceKey).(string)
	return namespace, ok
}

// ValidNamespace returns false if the namespace on the context differs from the resource.  If the resource has no namespace, it is set to the value in the context.
func ValidNamespace(ctx Context, metadata *ObjectMeta) bool {
	ns, ok := NamespaceFrom(ctx)
	if len(metadata.Namespace) == 0 {
		metadata.Namespace = ns
	}
	return ns == metadata.Namespace && ok
}
