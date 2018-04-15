/*
Copyright (c) 2017 GigaSpaces Technologies Ltd. All rights reserved

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

package cloudifyprovider

import (
	"fmt"
	cloudify "github.com/cloudify-incubator/cloudify-rest-go-client/cloudify"
	"github.com/golang/glog"
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"errors"
)

// Instances - struct with connection settings
type Instances struct {
	deployment string
	client     *cloudify.Client
}

func (r *Instances) GetDeploymentNodeInfo() (map[string] string, error) {
	deploymentInfo := make(map[string]string)

	data, err := cloudify.ParseDeploymentFile(r.deployment)
	if err != nil {
		fmt.Errorf("Error While trying to parse deployment file")
		return nil, err
	}

	for _, deployment :=  range data.Deployments {
		dep := deployment.(map[string]interface{})

		if dep["deployment_type"] == "node" {
			deploymentInfo["id"] = dep["id"].(string)
			deploymentInfo["node_data_data_type"] = dep["node_data_data_type"].(string)
			return deploymentInfo, nil
		}
	}

	return deploymentInfo, nil
}


func (r *Instances) GetDeploymentNodeID() (map[string]string, error) {
	deploymentInfo, err := r.GetDeploymentNodeInfo()
	if err != nil {
		glog.Errorf("Error: %+v", err)
		return nil, err
	}

	if deploymentInfo == nil {
		errorMessage := "cloudify deployment info is empty"
		glog.Errorf(errorMessage)
		return nil, errors.New(errorMessage)
	}

	return deploymentInfo, nil
}


// NodeAddresses returns the addresses of the specified instance.
// This implementation only returns the address of the calling instance. This is ok
// because the gce implementation makes that assumption and the comment for the interface
// states it as a todo to clarify that it is only for the current host
// Get by name in kubernetes
func (r *Instances) NodeAddresses(nodeName types.NodeName) ([]api.NodeAddress, error) {
	name := string(nodeName)
	glog.V(4).Infof("NodeAddresses [%s]", name)

	var params = map[string]string{}
	deploymentInfo, err := r.GetDeploymentNodeID()
	if err != nil {
		return nil, err
	}

	if deploymentInfo == nil {
		return nil, errors.New("cannot find the deployment info")
	}

	params["deployment_id"] = deploymentInfo["id"]
	nodeInstances, err := r.client.GetAliveNodeInstancesWithType(
		params, deploymentInfo["node_data_data_type"])
	if err != nil {
		glog.Infof("Not found instances: %+v", err)
		return nil, err
	}

	addresses := []api.NodeAddress{}

	for _, nodeInstance := range nodeInstances.Items {

		hostName := nodeInstance.GetStringProperty("hostname")
		if hostName != name {
			// node with different name
			continue
		}

		if hostName != "" {
			addresses = append(addresses, api.NodeAddress{
				Type:    api.NodeHostName,
				Address: hostName,
			})
		}

		hostPrivateIP := nodeInstance.GetStringProperty("ip")
		if hostPrivateIP != "" {
			addresses = append(addresses, api.NodeAddress{
				Type:    api.NodeInternalIP,
				Address: hostPrivateIP,
			})
		}

		hostPublicIP := nodeInstance.GetStringProperty("public_ip")
		if hostPublicIP != "" {
			addresses = append(addresses, api.NodeAddress{
				Type:    api.NodeExternalIP,
				Address: hostPublicIP,
			})
		}
	}

	if len(addresses) == 0 {
		glog.Infof("NodeAddresses: InstanceNotFound: %+v", name)
		return nil, cloudprovider.InstanceNotFound
	}

	glog.Infof("NodeAddresses: %+v", addresses)
	return addresses, nil
}

// NodeAddressesByProviderID returns the node addresses of an instances with the specified unique providerID
// This method will not be called from the node that is requesting this ID. i.e. metadata service
// and other local methods cannot be used here
// Get by ID in infrastructure
func (r *Instances) NodeAddressesByProviderID(providerID string) ([]api.NodeAddress, error) {
	glog.V(4).Infof("NodeAddressesByProviderID for [%s]", providerID)
	addresses := []api.NodeAddress{}

	nodeName, err := r.CurrentNodeName(providerID)
	if err != nil {
		return addresses, err
	}
	return r.NodeAddresses(nodeName)
}

// AddSSHKeyToAllInstances adds an SSH public key as a legal identity for all instances
// expected format for the key is standard ssh-keygen format: <protocol> <blob>
func (r *Instances) AddSSHKeyToAllInstances(user string, keyData []byte) error {
	glog.Errorf("?AddSSHKeyToAllInstances [%s]", user)
	return fmt.Errorf("Not implemented:AddSSHKeyToAllInstances")
}

// CurrentNodeName returns the name of the node we are currently running on
// Convert Hostname to Cloud Id
func (r *Instances) CurrentNodeName(hostname string) (types.NodeName, error) {
	glog.V(4).Infof("CurrentNodeName [%s]", hostname)
	return types.NodeName(hostname), nil
}

// ExternalID returns the cloud provider ID of the specified instance (deprecated).
func (r *Instances) ExternalID(nodeName types.NodeName) (string, error) {
	name := string(nodeName)
	glog.Errorf("?ExternalID [%s]", name)
	return r.InstanceID(nodeName)
}

// InstanceID returns the cloud provider ID of the specified instance.
func (r *Instances) InstanceID(nodeName types.NodeName) (string, error) {
	name := string(nodeName)
	glog.V(4).Infof("InstanceID [%s]", name)

	var params = map[string]string{}
	deploymentInfo, err := r.GetDeploymentNodeID()
	if err != nil {
		return "", err
	}

	if deploymentInfo == nil {
		return "", errors.New("cannot find the deployment info")
	}
	params["deployment_id"] = deploymentInfo["id"]

	nodeInstances, err := r.client.GetAliveNodeInstancesWithType(
		params, deploymentInfo["node_data_data_type"])
	if err != nil {
		glog.Infof("Not found instances: %+v", err)
		return "", err
	}

	for _, nodeInstance := range nodeInstances.Items {
		// check runtime properties
		if nodeInstance.GetStringProperty("hostname") != name {
			// node with different name
			continue
		}

		glog.Infof("Node is alive: %+v", name)
		return name, nil
	}

	glog.Infof("Node died: %+v", name)

	return "", cloudprovider.InstanceNotFound
}

// InstanceType returns the type of the specified instance.
// Note that if the instance does not exist or is no longer running, we must return ("", cloudprovider.InstanceNotFound)
func (r *Instances) InstanceType(nodeName types.NodeName) (string, error) {
	glog.V(4).Infof("InstanceID [%s]", nodeName)
	_, err := r.InstanceID(nodeName)
	if err != nil {
		return "", err
	}
	return providerName, nil
}

// InstanceTypeByProviderID returns the cloudprovider instance type of the node with the specified unique providerID
// This method will not be called from the node that is requesting this ID. i.e. metadata service
// and other local methods cannot be used here
func (r *Instances) InstanceTypeByProviderID(providerID string) (string, error) {
	glog.V(4).Infof("InstanceTypeByProviderID [%s]", providerID)

	nodeName, err := r.CurrentNodeName(providerID)
	if err != nil {
		return "", err
	}
	return r.InstanceType(nodeName)
}

// InstanceExistsByProviderID returns true if the instance with the given provider id still exists and is running.
// If false is returned with no error, the instance will be immediately deleted by the cloud controller manager.
func (r *Instances) InstanceExistsByProviderID(providerID string) (bool, error) {
	glog.V(4).Infof("InstanceExistsByProviderID [%s]", providerID)
	providerValue, err := r.InstanceTypeByProviderID(providerID)
	if err != nil {
		return false, err
	}
	return providerName == providerValue, nil
}

// NewInstances - create instance with support kubernetes intances interface.
func NewInstances(client *cloudify.Client, deployment string) *Instances {
	return &Instances{
		client:     client,
		deployment: deployment,
	}
}
