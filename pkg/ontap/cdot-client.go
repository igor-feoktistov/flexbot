package ontap

import (
	"errors"
	"time"

	"flexbot/pkg/config"
	"github.com/igor-feoktistov/go-ontap-sdk/ontap"
)

func CreateCdotClient(nodeConfig *config.NodeConfig) (c *ontap.Client, err error) {
	c = ontap.NewClient(
		"https://"+nodeConfig.Storage.CdotCredentials.Host,
		&ontap.ClientOptions{
			BasicAuthUser:     nodeConfig.Storage.CdotCredentials.User,
			BasicAuthPassword: nodeConfig.Storage.CdotCredentials.Password,
			SSLVerify:         false,
			Debug:             false,
			Timeout:           60 * time.Second,
			Version:           "1.160",
		},
	)
	var vserverOptions *ontap.VserverGetOptions
	if nodeConfig.Storage.SvmName == "" {
		// We don't need vserver name when connected to Vserver LIF
		vserverOptions = &ontap.VserverGetOptions{MaxRecords: 1}
	} else {
		// Name of vserver is required when connected to Cluster LIF
		vserverOptions = &ontap.VserverGetOptions{
			MaxRecords: 1,
			Query: &ontap.VserverInfo{
				VserverName: nodeConfig.Storage.SvmName,
			},
		}
	}
	vserverResponse, _, err := c.VserverGetAPI(vserverOptions)
	if err != nil {
		return
	} else {
		if vserverResponse.Results.NumRecords == 1 {
			nodeConfig.Storage.SvmName = vserverResponse.Results.VserverAttributes[0].VserverName
			c.SetVserver(nodeConfig.Storage.SvmName)
		} else {
			if nodeConfig.Storage.SvmName == "" {
				err = errors.New("CreateCdotClient(): expected svmName in storage configuration")
			} else {
				err = errors.New("CreateCdotClient: vserver not found: " + nodeConfig.Storage.SvmName)
			}
		}
	}
	return
}
