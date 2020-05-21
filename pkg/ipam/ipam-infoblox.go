package ipam

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"flexbot/pkg/config"
	ibclient "github.com/infobloxopen/infoblox-go-client"
)

type IpPool struct {
	Ips []string `json:"ips,omitempty"`
}

type HostIpAddress struct {
	IpAddress string   `json:"ip_address,omitempty"`
	Names     []string `json:"names,omitempty"`
}

type HostRecord struct {
	Ipv4Addrs []struct {
		Ipv4Addr string `json:"ipv4addr,omitempty"`
	} `json:"ipv4addrs,omitempty"`
	Name string `json:"names,omitempty"`
}

type InfobloxProvider struct {
	HostConfig  ibclient.HostConfig
	DnsView     string
	NetworkView string
	DnsZone     string
}

func getAvailableIPs(conn *ibclient.Connector, network_view string, cidr string, numIPs int) (ippool IpPool, err error) {
	objMgr := ibclient.NewObjectManager(conn, "flexbot", "admin")
	var network *ibclient.Network
	if network, err = objMgr.GetNetwork(network_view, cidr, nil); err != nil {
		return
	}
	if network == nil {
		err = fmt.Errorf("network %s not found", cidr)
		return
	}
	r := regexp.MustCompile(`(network/\w+):\d+\.\d+\.\d+\.\d+/\d+/.+`)
	m := r.FindStringSubmatch(network.Ref)
	if m == nil {
		err = fmt.Errorf("missing network ref in response")
		return
	}
	path := []string{"wapi", "v" + conn.HostConfig.Version, m[1]}
	u := url.URL{
		Scheme:   "https",
		Host:     conn.HostConfig.Host + ":" + conn.HostConfig.Port,
		Path:     strings.Join(path, "/"),
		RawQuery: "_function=next_available_ip&num=" + strconv.Itoa(numIPs),
	}
	var req *http.Request
	if req, err = http.NewRequest(http.MethodPost, u.String(), nil); err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(conn.HostConfig.Username, conn.HostConfig.Password)
	var res []byte
	if res, err = conn.Requestor.SendRequest(req); err != nil {
		return
	}
	if err = json.Unmarshal(res, &ippool); err != nil {
		err = fmt.Errorf("Unmarshal(): %s", string(res))
	}
	return
}

func getHostByIp(c *ibclient.Connector, network_view string, ipaddr string) (fqdn string, err error) {
	path := []string{"wapi", "v" + c.HostConfig.Version, "ipv4address"}
	u := url.URL{
		Scheme:   "https",
		Host:     c.HostConfig.Host + ":" + c.HostConfig.Port,
		Path:     strings.Join(path, "/"),
		RawQuery: "ip_address=" + ipaddr + "&network_view=" + network_view,
	}
	var req *http.Request
	if req, err = http.NewRequest(http.MethodGet, u.String(), nil); err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.HostConfig.Username, c.HostConfig.Password)
	var res []byte
	if res, err = c.Requestor.SendRequest(req); err != nil {
		return
	}
	var hostaddr []HostIpAddress
	if err = json.Unmarshal(res, &hostaddr); err != nil {
		err = fmt.Errorf("%s", string(res))
		return
	}
	if len(hostaddr) > 0 && len(hostaddr[0].Names) > 0 {
		fqdn = hostaddr[0].Names[0]
	}
	return
}

func getIpByHost(c *ibclient.Connector, network_view string, fqdn string) (ipaddr string, err error) {
	path := []string{"wapi", "v" + c.HostConfig.Version, "record:host"}
	u := url.URL{
		Scheme:   "https",
		Host:     c.HostConfig.Host + ":" + c.HostConfig.Port,
		Path:     strings.Join(path, "/"),
		RawQuery: "name=" + fqdn + "&network_view=" + network_view,
	}
	var req *http.Request
	req, err = http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.HostConfig.Username, c.HostConfig.Password)
	var res []byte
	res, err = c.Requestor.SendRequest(req)
	if err != nil {
		return
	}
	var host []HostRecord
	if err = json.Unmarshal(res, &host); err != nil {
		err = fmt.Errorf("%s: %s", err, string(res))
	} else {
		if len(host) > 0 && len(host[0].Ipv4Addrs) > 0 {
			ipaddr = host[0].Ipv4Addrs[0].Ipv4Addr
		}
	}
	return
}

func NewInfobloxProvider(ipam *config.Ipam) (provider *InfobloxProvider) {
	provider = &InfobloxProvider{
		HostConfig: ibclient.HostConfig{
			Host:     ipam.IbCredentials.Host,
			Version:  ipam.IbCredentials.WapiVersion,
			Port:     "443",
			Username: ipam.IbCredentials.User,
			Password: ipam.IbCredentials.Password,
		},
		DnsView:     ipam.IbCredentials.DnsView,
		NetworkView: ipam.IbCredentials.NetworkView,
		DnsZone:     ipam.DnsZone,
	}
	return
}

func (p *InfobloxProvider) AllocateIp(cidr string, fqdn string) (ipaddr string, err error) {
	transportConfig := ibclient.NewTransportConfig("false", 20, 10)
	requestBuilder := &ibclient.WapiRequestBuilder{}
	requestor := &ibclient.WapiHttpRequestor{}
	var conn *ibclient.Connector
	if conn, err = ibclient.NewConnector(p.HostConfig, transportConfig, requestBuilder, requestor); err != nil {
		err = fmt.Errorf("AllocateIP: NewConnector(): %s", err)
		return
	}
	defer conn.Logout()
	if fqdn == "" {
		var ipPool IpPool
		if ipPool, err = getAvailableIPs(conn, p.NetworkView, cidr, 1); err != nil {
			err = fmt.Errorf("AllocateIP: getAvailableIPs(): %s", err)
		} else {
			if len(ipPool.Ips) > 0 {
				ipaddr = ipPool.Ips[0]
			} else {
				err = fmt.Errorf("AllocateIP: getAvailableIPs(): no IPs available in network %s", cidr)
			}
		}
	} else {
		recordHostIpAddr := ibclient.NewHostRecordIpv4Addr(ibclient.HostRecordIpv4Addr{})
		recordHostIpAddr.Ipv4Addr = fmt.Sprintf("func:nextavailableip:%s,%s", cidr, p.NetworkView)
		recordHostIpAddrSlice := []ibclient.HostRecordIpv4Addr{*recordHostIpAddr}
		enableDNS := new(bool)
		*enableDNS = true
		host := ibclient.NewHostRecord(ibclient.HostRecord{
			Name:        fqdn,
			EnableDns:   enableDNS,
			NetworkView: p.NetworkView,
			View:        p.DnsView,
			Ipv4Addrs:   recordHostIpAddrSlice,
		})
		var ref string
		if ref, err = conn.CreateObject(host); err != nil {
			err = fmt.Errorf("AllocateIp: CreateObject(): %s", err)
			return
		}
		host.Ref = ref
		if err = conn.GetObject(host, ref, &host); err != nil {
			err = fmt.Errorf("AllocateIp: GetObject(): %s", err)
		} else {
			ipaddr = host.Ipv4Addrs[0].Ipv4Addr
		}
	}
	return
}

func (p *InfobloxProvider) ReleaseIp(fqdn string) (ipaddr string, err error) {
	transportConfig := ibclient.NewTransportConfig("false", 20, 10)
	requestBuilder := &ibclient.WapiRequestBuilder{}
	requestor := &ibclient.WapiHttpRequestor{}
	var conn *ibclient.Connector
	if conn, err = ibclient.NewConnector(p.HostConfig, transportConfig, requestBuilder, requestor); err != nil {
		err = fmt.Errorf("ReleaseIP: NewConnector(): %s", err)
		return
	}
	defer conn.Logout()
	objMgr := ibclient.NewObjectManager(conn, "flexbot", "admin")
	var host *ibclient.HostRecord
	if host, err = objMgr.GetHostRecord(fqdn, p.NetworkView, "", ""); err != nil {
		err = fmt.Errorf("ReleaseIP: GetHostRecord(): %s", err)
		return
	}
	if host != nil {
		ipaddr = host.Ipv4Addrs[0].Ipv4Addr
		if _, err := objMgr.DeleteHostRecord(host.Ref); err != nil {
			err = fmt.Errorf("ReleaseIP: DeleteHostRecord(): %s", err)
		}
	}
	return
}

func (p *InfobloxProvider) Allocate(nodeConfig *config.NodeConfig) (err error) {
	var ipaddr string
	var hostSuffix string = ""
	for i, _ := range nodeConfig.Network.Node {
		if ipaddr, err = p.AllocateIp(nodeConfig.Network.Node[i].Subnet, nodeConfig.Compute.HostName+hostSuffix+"."+p.DnsZone); err != nil {
			return
		}
		nodeConfig.Network.Node[i].Ip = ipaddr
		nodeConfig.Network.Node[i].Fqdn = nodeConfig.Compute.HostName + hostSuffix + "." + p.DnsZone
		hostSuffix = "-n" + strconv.Itoa(i+1)
	}
	for i, _ := range nodeConfig.Network.IscsiInitiator {
		hostSuffix = "-i" + strconv.Itoa(i+1)
		if ipaddr, err = p.AllocateIp(nodeConfig.Network.IscsiInitiator[i].Subnet, nodeConfig.Compute.HostName+hostSuffix+"."+p.DnsZone); err != nil {
			return
		}
		nodeConfig.Network.IscsiInitiator[i].Ip = ipaddr
		nodeConfig.Network.IscsiInitiator[i].Fqdn = nodeConfig.Compute.HostName + hostSuffix + "." + p.DnsZone
	}
	return
}

func (p *InfobloxProvider) Discover(nodeConfig *config.NodeConfig) (err error) {
	transportConfig := ibclient.NewTransportConfig("false", 20, 10)
	requestBuilder := &ibclient.WapiRequestBuilder{}
	requestor := &ibclient.WapiHttpRequestor{}
	var conn *ibclient.Connector
	if conn, err = ibclient.NewConnector(p.HostConfig, transportConfig, requestBuilder, requestor); err != nil {
		err = fmt.Errorf("Discover: NewConnector(): %s", err)
		return
	}
	defer conn.Logout()
	var hostSuffix string = ""
	for i, _ := range nodeConfig.Network.Node {
		var ipaddr string
		if ipaddr, err = getIpByHost(conn, p.NetworkView, nodeConfig.Compute.HostName+hostSuffix+"."+p.DnsZone); err != nil {
			err = fmt.Errorf("Discover: getIpByHost(): %s", err)
			return
		}
		if ipaddr == "" {
			err = fmt.Errorf("Discover: getIpByHost(): no host record found for FQDN  %s", nodeConfig.Compute.HostName+hostSuffix+"."+p.DnsZone)
			return
		}
		nodeConfig.Network.Node[i].Ip = ipaddr
		nodeConfig.Network.Node[i].Fqdn = nodeConfig.Compute.HostName + hostSuffix + "." + p.DnsZone
		hostSuffix = "-n" + strconv.Itoa(i+1)
	}
	for i, _ := range nodeConfig.Network.IscsiInitiator {
		hostSuffix = "-i" + strconv.Itoa(i+1)
		if nodeConfig.Network.IscsiInitiator[i].Ip != "" {
			var fqdn string
			if fqdn, err = getHostByIp(conn, p.NetworkView, nodeConfig.Network.IscsiInitiator[i].Ip); err != nil {
				err = fmt.Errorf("Discover: getHostByIp(): %s", err)
				return
			}
			if fqdn == "" {
				err = fmt.Errorf("Discover: getHostByIp(): no host record found for IP %s", nodeConfig.Network.IscsiInitiator[i].Ip)
				return
			}
			if fqdn == nodeConfig.Compute.HostName+hostSuffix+"."+p.DnsZone {
				nodeConfig.Network.IscsiInitiator[i].Fqdn = fqdn
			} else {
				err = fmt.Errorf("Discover: expected iSCSI initiator interface FQDN \"%s\", resolved \"%s\"", nodeConfig.Compute.HostName+hostSuffix+"."+p.DnsZone, fqdn)
				return
			}
		}
	}
	return
}

func (p *InfobloxProvider) AllocatePreflight(nodeConfig *config.NodeConfig) (err error) {
	for i, _ := range nodeConfig.Network.Node {
		if _, err = p.AllocateIp(nodeConfig.Network.Node[i].Subnet, ""); err != nil {
			return
		}
	}
	for i, _ := range nodeConfig.Network.IscsiInitiator {
		if _, err = p.AllocateIp(nodeConfig.Network.IscsiInitiator[i].Subnet, ""); err != nil {
			return
		}
	}
	return
}

func (p *InfobloxProvider) Release(nodeConfig *config.NodeConfig) (err error) {
	var ipaddr string
	var hostSuffix string = ""
	for i, _ := range nodeConfig.Network.Node {
		if ipaddr, err = p.ReleaseIp(nodeConfig.Compute.HostName + hostSuffix + "." + p.DnsZone); err != nil {
			return
		}
		nodeConfig.Network.Node[i].Ip = ipaddr
		hostSuffix = "-n" + strconv.Itoa(i+1)
	}
	for i, _ := range nodeConfig.Network.IscsiInitiator {
		hostSuffix = "-i" + strconv.Itoa(i+1)
		if ipaddr, err = p.ReleaseIp(nodeConfig.Compute.HostName + hostSuffix + "." + p.DnsZone); err != nil {
			return
		}
		nodeConfig.Network.IscsiInitiator[i].Ip = ipaddr
	}
	return
}
