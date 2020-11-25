package flexbot

import (
	"sync"
	
	"flexbot/pkg/rancher"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	rancherManagementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
)

type UpdateManager struct {
	Sync      sync.Mutex
	LastError error
}

type FlexbotConfig struct {
	FlexbotProvider       *schema.ResourceData
	RancherConfig         *rancher.Config
	RancherNodeDrainInput *rancherManagementClient.NodeDrainInput
	NodeGraceTimeout      int
	UpdateManager         UpdateManager
}

func (c *FlexbotConfig) UpdateManagerAcquire() (error) {
	if c.FlexbotProvider.Get("synchronized_updates").(bool) {
		c.UpdateManager.Sync.Lock()
		return c.UpdateManager.LastError
	} else {
		return nil
	}
}

func (c *FlexbotConfig) UpdateManagerSetError(err error) {
	if c.FlexbotProvider.Get("synchronized_updates").(bool) {
		c.UpdateManager.LastError = err
	}
}

func (c *FlexbotConfig) UpdateManagerRelease() {
	if c.FlexbotProvider.Get("synchronized_updates").(bool) {
		c.UpdateManager.Sync.Unlock()
	}
}
