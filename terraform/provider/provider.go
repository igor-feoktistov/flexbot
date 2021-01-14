package flexbot

import (
	"fmt"
	"sync"
	"strings"
	"encoding/base64"

	"flexbot/pkg/util/crypt"
	"flexbot/pkg/rancher"
	nodeConfig "flexbot/pkg/config"
	"github.com/denisbrodbeck/machineid"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
        "github.com/hashicorp/terraform-plugin-sdk/terraform"
	rancherManagementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
)

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"pass_phrase": {
				Type: schema.TypeString,
				Optional:  true,
				Sensitive: true,
				Default:   "",
			},
			"synchronized_updates": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"ipam": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"provider": {
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
								v := val.(string)
								if !(v == "Infoblox" || v == "Internal") {
									errs = append(errs, fmt.Errorf("unsupported %q=%s, allowed values are \"Internal\" and \"Infoblox\"", key, v))
								}
								return
							},
						},
						"credentials": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"host": {
										Type:     schema.TypeString,
										Required: true,
									},
									"user": {
										Type:     schema.TypeString,
										Required: true,
									},
									"password": {
										Type:     schema.TypeString,
										Required: true,
										Sensitive: true,
									},
									"wapi_version": {
										Type:     schema.TypeString,
										Required: true,
									},
									"dns_view": {
										Type:     schema.TypeString,
										Required: true,
									},
									"network_view": {
										Type:     schema.TypeString,
										Required: true,
									},
								},
							},
						},
						"dns_zone": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"compute": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"credentials": {
							Type:     schema.TypeList,
							Required: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"host": {
										Type:     schema.TypeString,
										Required: true,
									},
									"user": {
										Type:     schema.TypeString,
										Required: true,
									},
									"password": {
										Type:     schema.TypeString,
										Required: true,
										Sensitive: true,
									},
								},
							},
						},
					},
				},
			},
			"storage": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"credentials": {
							Type:     schema.TypeList,
							Required: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"host": {
										Type:     schema.TypeString,
										Required: true,
									},
									"user": {
										Type:     schema.TypeString,
										Required: true,
									},
									"password": {
										Type:     schema.TypeString,
										Required: true,
										Sensitive: true,
									},
									"zapi_version": {
										Type:     schema.TypeString,
										Optional: true,
									},
								},
							},
						},
					},
				},
			},
			"rancher_api": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"api_url": {
							Type:     schema.TypeString,
							Required: true,
						},
						"token_key": {
							Type:     schema.TypeString,
							Required: true,
						},
						"insecure": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"cluster_id": {
							Type:     schema.TypeString,
							Required: true,
						},
						"wait_for_node_timeout": {
							Optional: true,
							Type:     schema.TypeInt,
							Default:  0,
						},
						"node_grace_timeout": {
							Optional: true,
							Type:     schema.TypeInt,
							Default:  0,
						},
						"drain_input": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"delete_local_data": {
										Type:     schema.TypeBool,
										Optional: true,
										Default:  false,
									},
									"force": {
										Type:     schema.TypeBool,
										Optional: true,
										Default:  false,
									},
									"grace_period": {
										Type:     schema.TypeInt,
										Optional: true,
										Default:  60,
									},
									"ignore_daemon_sets": {
										Type:     schema.TypeBool,
										Optional: true,
										Default:  true,
									},
									"timeout": {
										Type:     schema.TypeInt,
										Optional: true,
										Default:  1800,
									},
								},
							},
						},
					},
				},
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"flexbot_server": resourceFlexbotServer(),
			"flexbot_repo": resourceFlexbotRepo(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"flexbot_crypt": dataSourceFelxbotCrypt(),
		},
		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	var err error
	var config *FlexbotConfig
	if len(d.Get("rancher_api").([]interface{})) > 0 {
		rancher_api := d.Get("rancher_api").([]interface{})[0].(map[string]interface{})
    		ignoreDaemonSets := rancher_api["drain_input"].([]interface{})[0].(map[string]interface{})["ignore_daemon_sets"].(bool)
		nodeDrainInput := &rancherManagementClient.NodeDrainInput{
			Force: rancher_api["drain_input"].([]interface{})[0].(map[string]interface{})["force"].(bool),
			IgnoreDaemonSets: &ignoreDaemonSets,
			DeleteLocalData: rancher_api["drain_input"].([]interface{})[0].(map[string]interface{})["delete_local_data"].(bool),
			GracePeriod: int64(rancher_api["drain_input"].([]interface{})[0].(map[string]interface{})["grace_period"].(int)),
			Timeout: int64(rancher_api["drain_input"].([]interface{})[0].(map[string]interface{})["timeout"].(int)),
		}
		tokenKey := rancher_api["token_key"].(string)
		if strings.HasPrefix(tokenKey, "base64:") {
			var b, b64 []byte
			passPhrase := d.Get("pass_phrase").(string)
			if len(passPhrase) == 0 {
				if passPhrase, err = machineid.ID(); err != nil {
					err = fmt.Errorf("providerConfigure(): machineid.ID() failure: %s", err)
					return nil, err
				}
			}
			if b64, err = base64.StdEncoding.DecodeString(tokenKey[7:]); err != nil {
				err = fmt.Errorf("providerConfigure(): base64.StdEncoding.DecodeString() failure: %s", err)
				return nil, err
			}
			if b, err = crypt.Decrypt(b64, passPhrase); err != nil {
				err = fmt.Errorf("providerConfigure(): Decrypt() failure: %s", err)
				return nil, err
			}
			tokenKey = string(b)
		}
		rancherConfig := &rancher.Config{
			URL: rancher_api["api_url"].(string),
			TokenKey: tokenKey,
			Insecure: rancher_api["insecure"].(bool),
			NodeDrainInput: nodeDrainInput,
			Retries: 3,
    		}
		config = &FlexbotConfig{
			Sync: sync.Mutex{},
			FlexbotProvider: d,
			RancherApiEnabled: rancher_api["enabled"].(bool),
			RancherConfig: rancherConfig,
			WaitForNodeTimeout: rancher_api["wait_for_node_timeout"].(int),
			NodeGraceTimeout: rancher_api["node_grace_timeout"].(int),
			NodeConfig: make(map[string]*nodeConfig.NodeConfig),
		}
		if rancher_api["enabled"].(bool) {
			if err = rancherConfig.ManagementClient(); err != nil {
				err = fmt.Errorf("providerConfigure(): rancherConfig.ManagementClient() error: \"%s\"", err)
			}
		}
	} else {
		config = &FlexbotConfig{
			Sync: sync.Mutex{},
			FlexbotProvider: d,
			RancherApiEnabled: false,
			NodeConfig: make(map[string]*nodeConfig.NodeConfig),
		}
	}
	return config, err
}
