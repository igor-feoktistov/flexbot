package ipam

import (
	"fmt"

	"flexbot/pkg/config"
)

type InternalProvider struct{}

func NewInternalProvider(ipam *config.Ipam) (provider *InternalProvider) {
	provider = &InternalProvider{}
	return
}

func (p *InternalProvider) AllocateIp(cidr string, fqdn string) (ipaddr string, err error) {
	return
}

func (p *InternalProvider) ReleaseIp(fqdn string) (ipaddr string, err error) {
	return
}

func (p *InternalProvider) Allocate(nodeConfig *config.NodeConfig) (err error) {
	for i, _ := range nodeConfig.Network.Node {
		if nodeConfig.Network.Node[i].Ip == "" {
			err = fmt.Errorf("Allocate: expected network.node[%d].ip in configuration", i)
			return
		}
		if nodeConfig.Network.Node[i].Fqdn == "" {
			err = fmt.Errorf("Allocate: expected network.node[%d].fqdn in configuration", i)
			return
		}
	}
	for i, _ := range nodeConfig.Network.IscsiInitiator {
		if nodeConfig.Network.IscsiInitiator[i].Ip == "" {
			err = fmt.Errorf("Allocate: expected network.iscsiinitiator[%d].ip in configuration", i)
			return
		}
		if nodeConfig.Network.IscsiInitiator[i].Fqdn == "" {
			err = fmt.Errorf("Allocate: expected network.iscsiinitiator[%d].fqdn in configuration", i)
			return
		}
	}
	return
}

func (p *InternalProvider) Discover(nodeConfig *config.NodeConfig) (err error) {
	err = p.Allocate(nodeConfig)
	return
}

func (p *InternalProvider) AllocatePreflight(nodeConfig *config.NodeConfig) (err error) {
	err = p.Allocate(nodeConfig)
	return
}

func (p *InternalProvider) Release(nodeConfig *config.NodeConfig) (err error) {
	return
}
