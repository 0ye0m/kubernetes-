/*
Copyright 2019 The Kubernetes Authors.

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

package csi

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/volume"
)

var _ volume.MetricsProvider = &metricsCsi{}

// metricsCsi represents a MetricsProvider that calculates the used and
// available Volume space for the Volume path.

type metricsCsi struct {
	// the directory path the volume is mounted to.
	targetPath string
	csiClientGetter
	volumeID string
}

// NewMetricsCsi creates a new metricsCsi with the Volume ID and path.
func NewMetricsCsi(volumeID string, targetPath string) volume.MetricsProvider {
	return &metricsCsi{volumeID: volumeID, targetPath: targetPath}
}

func (mc *metricsCsi) GetMetrics() (*volume.Metrics, error) {
	metrics := &volume.Metrics{Time: metav1.Now()}

	if mc.volumeID == "" {
		return nil, fmt.Errorf("VolumeID is nil")
	}

	if mc.targetPath == "" {
		return nil, fmt.Errorf("targetpath is nil")
	}

	err := mc.getCSIVolInfo(metrics)
	if err != nil {
		return metrics, err
	}

	return metrics, nil
}

func (mc *metricsCsi) getCSIVolInfo(metrics *volume.Metrics) error {

	ctx, cancel := context.WithTimeout(context.Background(), csiTimeout)
	defer cancel()

	csiClient, err := mc.csiClientGetter.Get()
	if err != nil {
		klog.Error(log("metricsCsi.getCSIVolInfo failed to get CSI client: %v", err))
		return err
	}

	// Check whether "GET_VOLUME_STATS" is set
	volumeStatsSet, err := csiClient.NodeSupportsVolumeStats(ctx)
	if err != nil {
		return err
	}
	if !volumeStatsSet {
		return nil
	}

	var resUsageMap map[string]usageCount
	resUsageMap, err = csiClient.NodeGetVolumeStats(ctx, mc.volumeID, mc.targetPath)

	if err != nil {
		return err
	}

	for k, v := range resUsageMap {
		if k == "BYTES" {
			metrics.Used = resource.NewQuantity(v.used, resource.BinarySI)
			metrics.Available = resource.NewQuantity(v.available, resource.BinarySI)
			metrics.Capacity = resource.NewQuantity(v.total, resource.BinarySI)
		} else if k == "INODES" {
			metrics.InodesUsed = resource.NewQuantity(v.used, resource.BinarySI)
			metrics.InodesFree = resource.NewQuantity(v.available, resource.BinarySI)
			metrics.Inodes = resource.NewQuantity(v.total, resource.BinarySI)
		}
	}
	return nil
}
