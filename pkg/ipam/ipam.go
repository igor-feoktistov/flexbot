package ipam

import (
	"fmt"

	"flexbot/pkg/config"
)

type IpamProvider interface {
	AllocateIp(cidr string, fqdn string) (string, error)
	ReleaseIp(fqdn string) (string, error)
	Allocate(nodeConfig *config.NodeConfig) error
	AllocatePreflight(nodeConfig *config.NodeConfig) error
	Discover(nodeConfig *config.NodeConfig) error
	Release(nodeConfig *config.NodeConfig) error
}

func NewProvider(ipam *config.Ipam) (provider IpamProvider, err error) {
	switch ipam.Provider {
        case "Infoblox":
                provider = NewInfobloxProvider(ipam)
        case "Internal":
                provider = NewInternalProvider(ipam)
        default:
                err = fmt.Errorf("NewProvider(): IPAM provider %s is not implemented", ipam.Provider)
        }
	return
}
