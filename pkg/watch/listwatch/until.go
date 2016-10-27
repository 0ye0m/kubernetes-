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

package listwatch

import (
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"
)

// ListerWatcher is any object that knows how to perform an initial list and start a watch on a resource.
type ListerWatcher interface {
	// List should return a list type object; the Items field will be extracted, and the
	// ResourceVersion field will be used to start the watch in the right place.
	List(options api.ListOptions) (runtime.Object, error)
	// Watch should begin a watch at the specified version.
	Watch(options api.ListOptions) (watch.Interface, error)
}

// TODO: check for watch expired error and retry watch from latest point?  Same issue exists for Until.
func Until(timeout time.Duration, lw ListerWatcher, conditions ...watch.ConditionFunc) (*watch.Event, error) {
	if len(conditions) == 0 {
		return nil, nil
	}

	list, err := lw.List(api.ListOptions{})
	if err != nil {
		return nil, err
	}
	initialItems, err := meta.ExtractList(list)
	if err != nil {
		return nil, err
	}

	// use the initial items as simulated "adds"
	var lastEvent *watch.Event
	currIndex := 0
	passedConditions := 0
	for _, condition := range conditions {
		// check the next condition against the previous event and short circuit waiting for the next watch
		if lastEvent != nil {
			done, err := condition(*lastEvent)
			if err != nil {
				return lastEvent, err
			}
			if done {
				passedConditions = passedConditions + 1
				continue
			}
		}

	ConditionSucceeded:
		for currIndex < len(initialItems) {
			lastEvent = &watch.Event{Type: watch.Added, Object: initialItems[currIndex]}
			currIndex++

			done, err := condition(*lastEvent)
			if err != nil {
				return lastEvent, err
			}
			if done {
				passedConditions = passedConditions + 1
				break ConditionSucceeded
			}
		}
	}
	if passedConditions == len(conditions) {
		return lastEvent, nil
	}
	remainingConditions := conditions[passedConditions:]

	metaObj, err := meta.ListAccessor(list)
	if err != nil {
		return nil, err
	}
	currResourceVersion := metaObj.GetResourceVersion()

	watchInterface, err := lw.Watch(api.ListOptions{ResourceVersion: currResourceVersion})
	if err != nil {
		return nil, err
	}

	return watch.Until(timeout, watchInterface, remainingConditions...)
}
