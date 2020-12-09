package flexbot

import (
	"flexbot/pkg/rancher"
        "github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	rancherManagementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
)

const (
        WAIT4CLUSTER_STATE_TIMEOUT = 1800
        WAIT4NODE_STATE_TIMEOUT = 600
)

type RancherNode struct {
	RancherClient    *rancher.Client
	NodeDrainInput   *rancherManagementClient.NodeDrainInput
	ClusterID        string
	NodeID           string
	NodeControlPlane bool
	NodeEtcd         bool
	NodeWorker       bool
}

func rancherApiInitialize(d *schema.ResourceData, meta interface{}) (*RancherNode, error) {
	var err error
	node := &RancherNode{
		NodeControlPlane: false,
		NodeEtcd:         false,
		NodeWorker:       false,
	}
	p := meta.(*FlexbotConfig).FlexbotProvider
        network := d.Get("network").([]interface{})[0].(map[string]interface{})
	if meta.(*FlexbotConfig).RancherApiEnabled == false || meta.(*FlexbotConfig).RancherConfig == nil {
		return node, err
	}
	node.RancherClient = &(meta.(*FlexbotConfig).RancherConfig.Client)
	node.NodeDrainInput = meta.(*FlexbotConfig).RancherConfig.NodeDrainInput
	node.ClusterID = p.Get("rancher_api").([]interface{})[0].(map[string]interface{})["cluster_id"].(string)
	if node.NodeID, err = node.RancherClient.GetNode(node.ClusterID, network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err == nil {
		if len(node.NodeID) > 0 {
			node.NodeControlPlane, node.NodeEtcd, node.NodeWorker, err = node.RancherClient.GetNodeRole(node.NodeID)
		}
	}
	return node, err
}

func rancherApiClusterWaitForState(node *RancherNode, state string, timeout int) (err error) {
	if node.RancherClient != nil {
		err = node.RancherClient.ClusterWaitForState(node.ClusterID, state, timeout)
	}
	return
}

func rancherApiNodeCordon(node *RancherNode) (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		if err = node.RancherClient.ClusterWaitForState(node.ClusterID, "active", WAIT4CLUSTER_STATE_TIMEOUT); err == nil {
			err = node.RancherClient.NodeCordonDrain(node.NodeID, node.NodeDrainInput)
		}
	}
	return
}

func rancherApiNodeUncordon(node *RancherNode) (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		if err = node.RancherClient.ClusterWaitForState(node.ClusterID, "active", WAIT4CLUSTER_STATE_TIMEOUT); err == nil {
			if err = node.RancherClient.NodeWaitForState(node.NodeID, "active,drained,cordoned", WAIT4NODE_STATE_TIMEOUT); err == nil {
				if err = node.RancherClient.NodeUncordon(node.NodeID); err == nil {
					err = node.RancherClient.NodeWaitForState(node.NodeID, "active", WAIT4NODE_STATE_TIMEOUT)
				}
			}
		}
	}
	return
}

func rancherApiNodeDelete(node *RancherNode) (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		err = node.RancherClient.DeleteNode(node.NodeID)
	}
	return
}
