package flexbot

import (
	"fmt"
	"time"
	"math"
	"encoding/json"

	"flexbot/pkg/rancher"
	"flexbot/pkg/config"
	"github.com/igor-feoktistov/go-ucsm-sdk/util"
        "github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	rancherManagementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
)

const (
        WAIT4CLUSTER_STATE_TIMEOUT = 1800
        WAIT4NODE_STATE_TIMEOUT = 600
)

const (
	NodeAnnotationCompute = "flexpod-compute"
	NodeAnnotationStorage = "flexpod-storage"
)

type RancherNode struct {
	RancherClient    *rancher.Client
	NodeConfig       *config.NodeConfig
	NodeDrainInput   *rancherManagementClient.NodeDrainInput
	ClusterID        string
	NodeID           string
	NodeControlPlane bool
	NodeEtcd         bool
	NodeWorker       bool
}

type ComputeAnnotations struct {
	UcsmHost  string         `yaml:"ucsmHost" json:"ucsmHost"`
	SpDn      string         `yaml:"spDn" json:"spDn"`
	Blade     util.BladeSpec `yaml:"blade" json:"blade"`
}

type StorageAnnotations struct {
	BootImage struct {
		OsImage      string `yaml:"osImage" json:"osImage"`
		SeedTemplate string `yaml:"seedTemplate" json:"seedTemplate"`
	}                           `yaml:"bootImage" json:"bootImage"`
	Svm     string              `yaml:"svm" json:"svm"`
	Volume  string              `yaml:"volume" json:"volume"`
	Igroup  string              `yaml:"igroup" json:"igroup"`
	BootLun string              `yaml:"bootLun" json:"bootLun"`
	DataLun string              `yaml:"dataLun" json:"dataLun"`
	SeedLun string              `yaml:"seedLun" json:"seedLun"`
}

func rancherApiInitialize(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig, waitForNode bool) (node *RancherNode, err error) {
	p := meta.(*FlexbotConfig).FlexbotProvider
        network := d.Get("network").([]interface{})[0].(map[string]interface{})
	node = &RancherNode{
		NodeConfig:       nodeConfig,
		NodeControlPlane: false,
		NodeEtcd:         false,
		NodeWorker:       false,
	}
	if meta.(*FlexbotConfig).RancherConfig == nil || meta.(*FlexbotConfig).RancherApiEnabled == false {
		return
	}
	node.RancherClient = &(meta.(*FlexbotConfig).RancherConfig.Client)
	node.NodeDrainInput = meta.(*FlexbotConfig).RancherConfig.NodeDrainInput
	node.ClusterID = p.Get("rancher_api").([]interface{})[0].(map[string]interface{})["cluster_id"].(string)
	if node.NodeID, err = node.RancherClient.GetNode(node.ClusterID, network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err == nil {
		if len(node.NodeID) > 0 {
			node.NodeControlPlane, node.NodeEtcd, node.NodeWorker, err = node.RancherClient.GetNodeRole(node.NodeID)
		}
	}
	if waitForNode && meta.(*FlexbotConfig).WaitForNodeTimeout > 0 {
		giveupTime := time.Now().Add(time.Second * time.Duration(meta.(*FlexbotConfig).WaitForNodeTimeout))
		if err = node.RancherClient.ClusterWaitForState(node.ClusterID, "active", meta.(*FlexbotConfig).WaitForNodeTimeout); err != nil {
			return
		}
    		for time.Now().Before(giveupTime) {
			if node.NodeID, err = node.RancherClient.GetNode(node.ClusterID, network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err != nil {
				return
			}
			if len(node.NodeID) > 0 {
				if err = node.RancherClient.NodeWaitForState(node.NodeID, "active", int(math.Round(giveupTime.Sub(time.Now()).Seconds()))); err == nil {
					node.NodeControlPlane, node.NodeEtcd, node.NodeWorker, err = node.RancherClient.GetNodeRole(node.NodeID)
				}
				return
			}
			time.Sleep(5 * time.Second)
		}
	}
	return
}

func (node *RancherNode) rancherApiClusterWaitForState(state string, timeout int) (err error) {
	if node.RancherClient != nil {
		err = node.RancherClient.ClusterWaitForState(node.ClusterID, state, timeout)
	}
	return
}

func (node *RancherNode) rancherApiNodeCordon() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		if err = node.RancherClient.ClusterWaitForState(node.ClusterID, "active", WAIT4CLUSTER_STATE_TIMEOUT); err == nil {
			err = node.RancherClient.NodeCordonDrain(node.NodeID, node.NodeDrainInput)
		}
	}
	return
}

func (node *RancherNode) rancherApiNodeUncordon() (err error) {
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

func (node *RancherNode) rancherApiNodeDelete() (err error) {
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		err = node.RancherClient.DeleteNode(node.NodeID)
	}
	return
}

func (node *RancherNode) rancherApiNodeSetAnnotations() (err error) {
	var compute_b, storage_b []byte
	if node.RancherClient != nil && len(node.NodeID) > 0 {
		annotations := make(map[string]string)
		if len(node.NodeConfig.Compute.SpDn) > 0 && len(node.NodeConfig.Compute.BladeAssigned.Dn) > 0 {
			computeAnnotations := ComputeAnnotations{
				UcsmHost: node.NodeConfig.Compute.UcsmCredentials.Host,
				SpDn: node.NodeConfig.Compute.SpDn,
				Blade: node.NodeConfig.Compute.BladeAssigned,
			}
			if compute_b, err = json.Marshal(computeAnnotations); err != nil {
				err = fmt.Errorf("json.Marshal(computeAnnotations): %s", err)
				return
			}
			annotations[NodeAnnotationCompute] = string(compute_b)
		}
		if len(node.NodeConfig.Storage.SvmName) > 0 {
			storageAnnotations := StorageAnnotations{
				Svm: node.NodeConfig.Storage.SvmName,
				Volume: node.NodeConfig.Storage.VolumeName,
				Igroup: node.NodeConfig.Storage.IgroupName,
				BootLun: node.NodeConfig.Storage.BootLun.Name,
				DataLun: node.NodeConfig.Storage.DataLun.Name,
				SeedLun: node.NodeConfig.Storage.SeedLun.Name,
			}
			storageAnnotations.BootImage.OsImage = node.NodeConfig.Storage.BootLun.OsImage.Name
			storageAnnotations.BootImage.SeedTemplate = node.NodeConfig.Storage.SeedLun.SeedTemplate.Name
			if storage_b, err = json.Marshal(storageAnnotations); err != nil {
				err = fmt.Errorf("json.Marshal(storageAnnotations): %s", err)
				return
			}
			annotations[NodeAnnotationStorage] = string(storage_b)
		}
		err = node.RancherClient.NodeSetAnnotations(node.NodeID, annotations)
	}
	return
}
