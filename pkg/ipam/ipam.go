package ipam

import (
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
