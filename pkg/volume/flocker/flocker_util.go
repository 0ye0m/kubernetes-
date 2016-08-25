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

package flocker

import (
	"fmt"
	"time"

	"k8s.io/kubernetes/pkg/util/rand"
	"k8s.io/kubernetes/pkg/volume"

	flockerApi "github.com/ClusterHQ/flocker-go"
	"github.com/golang/glog"
)

type FlockerUtil struct{}

func (util *FlockerUtil) DeleteVolume(d *flockerVolumeDeleter) error {
	return fmt.Errorf("Flocker deleter for %+v not yet implemented", d)
}

func (util *FlockerUtil) CreateVolume(c *flockerVolumeProvisioner) (datasetUUID string, volumeSizeGB int, labels map[string]string, err error) {

	if c.flockerClient == nil {
		c.flockerClient, err = c.plugin.newFlockerClient("")
		if err != nil {
			return
		}
	}

	nodes, err := c.flockerClient.ListNodes()
	if err != nil {
		return
	}
	if len(nodes) < 1 {
		err = fmt.Errorf("No nodes found inside the flocker cluster to provision a dataset")
		return
	}

	// select random node
	rand.Seed(time.Now().UTC().UnixNano())
	node := nodes[rand.Intn(len(nodes))]
	glog.V(2).Infof("Select flocker nodes %s to provision dataset", node.UUID)

	requestBytes := c.options.Capacity.Value()
	volumeSizeGB = int(volume.RoundUpSize(requestBytes, 1024*1024*1024))

	createOptions := &flockerApi.CreateDatasetOptions{
		MaximumSize: requestBytes,
		Metadata: map[string]string{
			"type": "k8s-dynamic-prov",
			"pvc":  c.options.PVCName,
		},
		Primary: node.UUID,
	}

	datasetState, err := c.flockerClient.CreateDataset(createOptions)
	if err != nil {
		return
	}
	datasetUUID = datasetState.DatasetID

	glog.V(2).Infof("Successfully created Flocker dataset %s", datasetUUID)

	return
}
