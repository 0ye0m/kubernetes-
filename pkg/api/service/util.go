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

package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	netsets "k8s.io/kubernetes/pkg/util/net/sets"
)

const (
	defaultLoadBalancerSourceRanges = "0.0.0.0/0"
)

// IsAllowAll checks whether the netsets.IPNet allows traffic from 0.0.0.0/0
func IsAllowAll(ipnets netsets.IPNet) bool {
	for _, s := range ipnets.StringSlice() {
		if s == "0.0.0.0/0" {
			return true
		}
	}
	return false
}

// GetLoadBalancerSourceRanges first try to parse and verify LoadBalancerSourceRanges field from a service.
// If the field is not specified, turn to parse and verify the AnnotationLoadBalancerSourceRangesKey annotation from a service,
// extracting the source ranges to allow, and if not present returns a default (allow-all) value.
func GetLoadBalancerSourceRanges(service *api.Service) (netsets.IPNet, error) {
	var ipnets netsets.IPNet
	var err error
	// if SourceRange field is specified, ignore sourceRange annotation
	if len(service.Spec.LoadBalancerSourceRanges) > 0 {
		specs := service.Spec.LoadBalancerSourceRanges
		ipnets, err = netsets.ParseIPNets(specs...)

		if err != nil {
			return nil, fmt.Errorf("service.Spec.LoadBalancerSourceRanges: %v is not valid. Expecting a list of IP ranges. For example, 10.0.0.0/24. Error msg: %v", specs, err)
		}
	} else {
		val := service.Annotations[AnnotationLoadBalancerSourceRangesKey]
		val = strings.TrimSpace(val)
		if val == "" {
			val = defaultLoadBalancerSourceRanges
		}
		specs := strings.Split(val, ",")
		ipnets, err = netsets.ParseIPNets(specs...)
		if err != nil {
			return nil, fmt.Errorf("%s: %s is not valid. Expecting a comma-separated list of source IP ranges. For example, 10.0.0.0/24,192.168.2.0/24", AnnotationLoadBalancerSourceRangesKey, val)
		}
	}
	return ipnets, nil
}

// RequestsOnlyLocalTraffic chekcs if service requests OnlyLocal traffic.
func RequestsOnlyLocalTraffic(service *api.Service) bool {
	// First check the beta annotation and then the first class field. This is so existing
	// Services continue to work till the user decides to transition to the first class field.
	// If they transition to the first class field, there's no way to go back to beta without
	// rolling back the cluster.
	if l, ok := service.Annotations[BetaAnnotationExternalTraffic]; ok {
		if l == string(api.ServiceExternalTrafficTypeOnlyLocal) {
			return true
		} else if l == string(api.ServiceExternalTrafficTypeGlobal) {
			return false
		} else {
			glog.Errorf("Invalid value for annotation %v: %v", BetaAnnotationExternalTraffic, l)
		}
	}
	return service.Spec.ExternalTraffic == api.ServiceExternalTrafficTypeOnlyLocal
}

// NeedsHealthCheck Check if service needs health check.
func NeedsHealthCheck(service *api.Service) bool {
	if service.Spec.Type != api.ServiceTypeLoadBalancer {
		return false
	}
	return RequestsOnlyLocalTraffic(service)
}

// GetServiceHealthCheckNodePort Return health check node port for service, if one exists
func GetServiceHealthCheckNodePort(service *api.Service) int32 {
	if !NeedsHealthCheck(service) {
		return 0
	}
	// First check the beta annotation and then the first class field. This is so existing
	// Services continue to work till the user decides to transition to the first class field.
	// If they transition to the first class field, there's no way to go back to beta without
	// rolling back the cluster.
	if l, ok := service.Annotations[BetaAnnotationHealthCheckNodePort]; ok {
		p, err := strconv.Atoi(l)
		if err != nil {
			glog.Errorf("Failed to parse annotation %v: %v", BetaAnnotationHealthCheckNodePort, err)
		} else {
			return int32(p)
		}
	}
	return service.Spec.HealthCheckNodePort
}
