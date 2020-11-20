package rancher

import (
    "fmt"
    "sort"
    "sync"
    "time"
    "strings"
    "io/ioutil"
    "crypto/tls"
    "crypto/x509"
    "net/http"
    "net/url"
    "encoding/base64"
    "github.com/hashicorp/go-version"
    "github.com/rancher/norman/clientbase"
    "github.com/rancher/norman/types"
    clusterClient "github.com/rancher/rancher/pkg/client/generated/cluster/v3"
    managementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
)

const (
	rancher2ClientAPIVersion          = "/v3"
	rancher2ReadyAnswer               = "pong"
        rancher2RetriesWait               = 5
        rancher2RKEK8sSystemImageVersion  = "2.3.0"
        maxHTTPRedirect                   = 5
)

type Client struct {
	Management *managementClient.Client
	CatalogV2  map[string]*clientbase.APIBaseClient
	Cluster    map[string]*clusterClient.Client
}

type Config struct {
	TokenKey             string `json:"tokenKey"`
	URL                  string `json:"url"`
	CACerts              string `json:"cacert"`
	Insecure             bool   `json:"insecure"`
	Bootstrap            bool   `json:"bootstrap"`
	ClusterID            string `json:"clusterId"`
	ProjectID            string `json:"projectId"`
	Retries              int
	RancherVersion       string
	K8SDefaultVersion    string
	K8SSupportedVersions []string
	Sync                 sync.Mutex
	NodeDrainInput       *managementClient.NodeDrainInput
	Client               Client
}

func NormalizeURL(input string) string {
	if input == "" {
		return ""
	}
	u, err := url.Parse(input)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	u.Path = ""
	return u.String()
}

func RootURL(url string) string {
	NormalizeURL(url)
	url = strings.TrimSuffix(url, "/v3")
	return url
}

func NewListOpts(filters map[string]interface{}) *types.ListOpts {
	listOpts := clientbase.NewListOpts()
	if filters != nil {
		listOpts.Filters = filters
	}
	return listOpts
}

func DoGet(url, username, password, token, cacert string, insecure bool) ([]byte, error) {
	if url == "" {
		return nil, fmt.Errorf("Doing get: URL is nil")
	}
	client := &http.Client{
		Timeout: time.Duration(60 * time.Second),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxHTTPRedirect {
				return fmt.Errorf("Stopped after %d redirects", maxHTTPRedirect)
			}
			if len(token) > 0 {
				req.Header.Add("Authorization", "Bearer "+token)
			} else if len(username) > 0 && len(password) > 0 {
				s := username + ":" + password
				req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(s)))
			}
			return nil
		},
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
		Proxy:           http.ProxyFromEnvironment,
	}
	if cacert != "" {
		rootCAs, _ := x509.SystemCertPool()
		if rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}
		rootCAs.AppendCertsFromPEM([]byte(cacert))
		transport.TLSClientConfig.RootCAs = rootCAs
	}
	client.Transport = transport
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("Doing get: %v", err)
	}
	if len(token) > 0 {
		req.Header.Add("Authorization", "Bearer "+token)
	} else if len(username) > 0 && len(password) > 0 {
		s := username + ":" + password
		req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(s)))
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Doing get: %v", err)
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func IsVersionLessThan(ver1, ver2 string) (bool, error) {
	v1, err := version.NewVersion(ver1)
	if err != nil {
		return false, err
	}
	v2, err := version.NewVersion(ver2)
	if err != nil {
		return false, err
	}
	return v1.LessThan(v2), nil
}

func (c *Config) GetRancherVersion() (string, error) {
	if len(c.RancherVersion) > 0 {
		return c.RancherVersion, nil
	}
	if c.Client.Management == nil {
		err := c.ManagementClient()
		if err != nil {
			return "", err
		}
	}
	version, err := c.Client.Management.Setting.ByID("server-version")
	if err != nil {
		return "", fmt.Errorf("[ERROR] Getting Rancher version: %s", err)
	}
	c.RancherVersion = version.Value
	return c.RancherVersion, nil
}

func (c *Config) isRancherReady() error {
	var err error
	var resp []byte
	url := RootURL(c.URL) + "/ping"
	for i := 0; i <= c.Retries; i++ {
		resp, err = DoGet(url, "", "", "", c.CACerts, c.Insecure)
		if err == nil && rancher2ReadyAnswer == string(resp) {
			return nil
		}
		time.Sleep(rancher2RetriesWait * time.Second)
	}
	return fmt.Errorf("Rancher is not ready: %v", err)
}

func (c *Config) getK8SDefaultVersion() (string, error) {
	if len(c.K8SDefaultVersion) > 0 {
		return c.K8SDefaultVersion, nil
	}

	if c.Client.Management == nil {
		err := c.ManagementClient()
		if err != nil {
			return "", err
		}
	}
	k8sVer, err := c.Client.Management.Setting.ByID("k8s-version")
	if err != nil {
		return "", err
	}
	c.K8SDefaultVersion = k8sVer.Value
	return c.K8SDefaultVersion, nil
}

func (c *Config) getK8SVersions() ([]string, error) {
	if len(c.K8SSupportedVersions) > 0 {
		return c.K8SSupportedVersions, nil
	}
	if c.Client.Management == nil {
		err := c.ManagementClient()
		if err != nil {
			return nil, err
		}
	}
	if ok, _ := c.IsRancherVersionLessThan(rancher2RKEK8sSystemImageVersion); ok {
		return nil, nil
	}
	RKEK8sSystemImageCollection, err := c.Client.Management.RkeK8sSystemImage.ListAll(NewListOpts(nil))
	if err != nil {
		return nil, fmt.Errorf("[ERROR] Listing RKE K8s System Images: %s", err)
	}
	versions := make([]*version.Version, 0, len(RKEK8sSystemImageCollection.Data))
	for _, RKEK8sSystem := range RKEK8sSystemImageCollection.Data {
		v, _ := version.NewVersion(RKEK8sSystem.Name)
		versions = append(versions, v)

	}
	sort.Sort(sort.Reverse(version.Collection(versions)))
	for i := range versions {
		c.K8SSupportedVersions = append(c.K8SSupportedVersions, "v"+versions[i].String())
	}
	return c.K8SSupportedVersions, nil
}

func (c *Config) IsRancherVersionLessThan(ver string) (bool, error) {
	if len(ver) == 0 {
		return false, fmt.Errorf("[ERROR] version is nil")
	}
	_, err := c.GetRancherVersion()
	if err != nil {
		return false, fmt.Errorf("[ERROR] getting rancher server version")
	}
	return IsVersionLessThan(c.RancherVersion, ver)
}

func (c *Config) ManagementClient() (error) {
	c.Sync.Lock()
	defer c.Sync.Unlock()
	if c.Client.Management != nil {
		return nil
	}
	err := c.isRancherReady()
	if err != nil {
		return err
	}
	options := c.CreateClientOpts()
	options.URL = options.URL + rancher2ClientAPIVersion
	mClient, err := managementClient.NewClient(options)
	if err != nil {
		return err
	}
	c.Client.Management = mClient
	return err
}

func (c *Config) CreateClientOpts() *clientbase.ClientOpts {
	c.NormalizeURL()
	options := &clientbase.ClientOpts{
		URL:      c.URL,
		TokenKey: c.TokenKey,
		CACerts:  c.CACerts,
		Insecure: c.Insecure,
	}
	return options
}

func (c *Config) NormalizeURL() {
	c.URL = NormalizeURL(c.URL)
}

func (client *Client) GetNode(clusterId string, nodeIpAddr string) (nodeId string, err error) {
        var clusters *managementClient.ClusterCollection
	var nodes *managementClient.NodeCollection

	filters := map[string]interface{} {
                "id": clusterId,
        }
	clusters, err = client.Management.Cluster.List(NewListOpts(filters))
	if err == nil && len(clusters.Data) > 0 {
		filters := map[string]interface{} {
			    "clusterId": clusterId,
			    "ipAddress": nodeIpAddr,
		}
    		nodes, err = client.Management.Node.List(NewListOpts(filters))
    		if err == nil && len(nodes.Data) > 0 {
    			nodeId =  nodes.Data[0].ID
    		}
	}
	if err != nil {
		err = fmt.Errorf("rancher.GetNode() error: %s", err)
	}
	return
}

func (client *Client) NodeWaitForState(nodeId string, state string, timeout int) (err error) {
	var node *managementClient.Node
	var nodeLastState string
	giveupTime := time.Now().Add(time.Second * time.Duration(timeout))
    	for time.Now().Before(giveupTime) {
            	if node, err = client.Management.Node.ByID(nodeId); err != nil {
            		return err
            	}
            	if node.State == state {
            		return
            	}
            	nodeLastState = node.State
            	time.Sleep(5 * time.Second)
        }
    	err = fmt.Errorf("rancher.NodeWaitForState(): wait for node state exceeded timeout=%d: expected state=%s, last state=%s", nodeLastState)
        return
}

func (client *Client) NodeCordonDrain(nodeId string, nodeDrainInput *managementClient.NodeDrainInput) (err error) {
	var node *managementClient.Node
	var ok bool
        if node, err = client.Management.Node.ByID(nodeId); err != nil {
    		return
        }
	_, ok = node.Actions["cordon"]
    	if ok {
		if err = client.Management.Node.ActionCordon(node); err != nil {
			return
		}
	}
	_, ok = node.Actions["drain"]
    	if ok {
		if err = client.Management.Node.ActionDrain(node, nodeDrainInput); err == nil {
			err = client.NodeWaitForState(nodeId, "drained", int(nodeDrainInput.Timeout + nodeDrainInput.GracePeriod))
		}
	}
	if err != nil {
		err = fmt.Errorf("rancher.NodeCordonDrain() error: %s", err)
	}
        return
}

func (client *Client) NodeUncordon(nodeId string) (err error) {
	var node *managementClient.Node
        if node, err = client.Management.Node.ByID(nodeId); err != nil {
    		return
        }
	_, ok := node.Actions["uncordon"]
    	if ok {
    		err = client.Management.Node.ActionUncordon(node)
    	}
	if err != nil {
		err = fmt.Errorf("rancher.NodeUncordon() error: %s", err)
	}
	return
}

func (client *Client) DeleteNode(nodeId string) (err error) {
	var node *managementClient.Node
        if node, err = client.Management.Node.ByID(nodeId); err != nil {
    		return
        }
        err = client.Management.Node.Delete(node)
	if err != nil {
		err = fmt.Errorf("rancher.DeleteNode() error: %s", err)
	}
	return
}
	