package flexbot

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"bytes"
	"time"
	"golang.org/x/crypto/ssh"

	"flexbot/pkg/config"
	"flexbot/pkg/ipam"
	"flexbot/pkg/ontap"
	"flexbot/pkg/ucsm"
	"flexbot/pkg/rancher"
	"github.com/denisbrodbeck/machineid"
        "github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func resourceFlexbotServer() *schema.Resource {
	return &schema.Resource{
		Schema: schemaFlexbotServer(),
		Create: resourceCreateServer,
		Read:   resourceReadServer,
		Update: resourceUpdateServer,
		Delete: resourceDeleteServer,
		Importer: &schema.ResourceImporter{
			State: resourceImportServer,
		},
	}
}

func resourceCreateServer(d *schema.ResourceData, meta interface{}) (err error) {
	var nodeConfig *config.NodeConfig
	if nodeConfig, err = setFlexbotInput(d, meta); err != nil {
		return
	}
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	sshUser := compute["ssh_user"].(string)
	sshPrivateKey := compute["ssh_private_key"].(string)
	for _, snapshot := range d.Get("snapshot").([]interface{}) {
		name := snapshot.(map[string]interface{})["name"].(string)
		if snapshot.(map[string]interface{})["fsfreeze"].(bool) {
			if len(sshUser) == 0 || len(sshPrivateKey) == 0 {
				err = fmt.Errorf("resourceCreateServer(): expected compute.ssh_user and compute.ssh_private_key parameters to ensure fsfreeze for snapshot %s", name)
				return
			}
		}
	}
	log.Printf("[INFO] Creating Server %s", nodeConfig.Compute.HostName)
	var serverExists bool
	if serverExists, err = ucsm.DiscoverServer(nodeConfig); err != nil {
		return
	}
	if serverExists {
		err = fmt.Errorf("resourceCreateServer(): serverServer %s already exists", nodeConfig.Compute.HostName)
		return
	}
	var provider ipam.IpamProvider
	switch nodeConfig.Ipam.Provider {
	case "Infoblox":
		provider = ipam.NewInfobloxProvider(&nodeConfig.Ipam)
	case "Internal":
		provider = ipam.NewInternalProvider(&nodeConfig.Ipam)
	default:
		err = fmt.Errorf("resourceCreateServer(): IPAM provider %s is not implemented", nodeConfig.Ipam.Provider)
		return
	}
	var preflightErr error
	preflightErrMsg := []string{}
	preflightErr = provider.AllocatePreflight(nodeConfig)
	if preflightErr != nil {
		preflightErrMsg = append(preflightErrMsg, preflightErr.Error())
	}
	preflightErr = ontap.CreateBootStoragePreflight(nodeConfig)
	if preflightErr != nil {
		preflightErrMsg = append(preflightErrMsg, preflightErr.Error())
	}
	preflightErr = ucsm.CreateServerPreflight(nodeConfig)
	if preflightErr != nil {
		preflightErrMsg = append(preflightErrMsg, preflightErr.Error())
	}
	preflightErr = ontap.CreateSeedStoragePreflight(nodeConfig)
	if preflightErr != nil {
		preflightErrMsg = append(preflightErrMsg, preflightErr.Error())
	}
	if len(preflightErrMsg) > 0 {
		err = fmt.Errorf("resourceCreateServer(): %s", strings.Join(preflightErrMsg, "\n"))
		return
	}
	if err = provider.Allocate(nodeConfig); err != nil {
		err = fmt.Errorf("resourceCreateServer(): %s", err)
		return
	}
	if err = ontap.CreateBootStorage(nodeConfig); err == nil {
		_, err = ucsm.CreateServer(nodeConfig)
	}
	if err == nil {
		err = ontap.CreateSeedStorage(nodeConfig)
	}
	if err == nil {
		err = ucsm.StartServer(nodeConfig)
	}
	d.SetId(nodeConfig.Compute.HostName)
	if err == nil {
		d.SetConnInfo(map[string]string{
			"type": "ssh",
			"host": nodeConfig.Network.Node[0].Ip,
		})
	}
	if compute["wait_for_ssh_timeout"].(int) > 0 && len(sshUser) > 0 && len(sshPrivateKey) > 0 && err == nil {
		if err = waitForSsh(nodeConfig, compute["wait_for_ssh_timeout"].(int), sshUser, sshPrivateKey); err == nil {
			for _, cmd := range compute["ssh_node_init_commands"].([]interface{}) {
				var cmdOutput string
				log.Printf("[INFO] Running SSH command on node %s: %s", nodeConfig.Compute.HostName, cmd.(string))
				if cmdOutput, err = runSshCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey, cmd.(string)); err != nil {
					break
				}
				if len(cmdOutput) > 0 {
					log.Printf("[DEBUG] completed SSH command: exec: %s, output: %s", cmd.(string), cmdOutput)
				}
			}
		}
	}
	if err == nil {
		for _, snapshot := range d.Get("snapshot").([]interface{}) {
			name := snapshot.(map[string]interface{})["name"].(string)
			if snapshot.(map[string]interface{})["fsfreeze"].(bool) {
				err = createSnapshot(nodeConfig, sshUser, sshPrivateKey, name)
			} else {
				err = ontap.CreateSnapshot(nodeConfig, name, "")
			}
			if err != nil {
				break
			}
		}
	}
	if err != nil {
		err = fmt.Errorf("resourceCreateServer(): %s", err)
	}
	setFlexbotOutput(d, meta, nodeConfig)
	return
}

func resourceReadServer(d *schema.ResourceData, meta interface{}) (err error) {
	var nodeConfig *config.NodeConfig
	if nodeConfig, err = setFlexbotInput(d, meta); err != nil {
		return
	}
	log.Printf("[INFO] Refreshing Server %s", nodeConfig.Compute.HostName)
	var serverExists bool
	if serverExists, err = ucsm.DiscoverServer(nodeConfig); err != nil {
		return
	}
	var storageExists bool
	if storageExists, err = ontap.DiscoverBootStorage(nodeConfig); err != nil {
		return
	}
	if serverExists && storageExists {
		var provider ipam.IpamProvider
		switch nodeConfig.Ipam.Provider {
		case "Infoblox":
			provider = ipam.NewInfobloxProvider(&nodeConfig.Ipam)
		case "Internal":
			provider = ipam.NewInternalProvider(&nodeConfig.Ipam)
		default:
			err = fmt.Errorf("resourceReadServer(): IPAM provider %s is not implemented", nodeConfig.Ipam.Provider)
			return
		}
		if err = provider.Discover(nodeConfig); err != nil {
			return
		}
		setFlexbotOutput(d, meta, nodeConfig)
	} else {
		d.SetId("")
	}
	return
}

func resourceUpdateServer(d *schema.ResourceData, meta interface{}) (err error) {
	if d.HasChange("compute") && !d.IsNewResource() {
		if err = resourceUpdateServerCompute(d, meta); err != nil {
			resourceReadServer(d, meta)
			return
		}
		d.SetPartial("compute")
	}
	if d.HasChange("storage") && !d.IsNewResource() {
		if err = resourceUpdateServerStorage(d, meta); err != nil {
			resourceReadServer(d, meta)
			return
		}
		d.SetPartial("storage")
	}
	if d.HasChange("snapshot") && !d.IsNewResource() {
		if err = resourceUpdateServerSnapshot(d, meta); err != nil {
			return
		}
		d.SetPartial("snapshot")
	}
	if d.HasChange("restore") {
		if err = resourceUpdateServerRestore(d, meta); err != nil {
			return
		}
		d.SetPartial("restore")
	}
	d.Partial(false)
	return
}

func resourceUpdateServerCompute(d *schema.ResourceData, meta interface{}) (err error) {
	var powerState, clusterId, nodeId string
	var nodeConfig *config.NodeConfig
	var rancherClient *rancher.Client
	p := meta.(*FlexbotConfig).FlexbotProvider
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	network := d.Get("network").([]interface{})[0].(map[string]interface{})
	sshUser := compute["ssh_user"].(string)
	sshPrivateKey := compute["ssh_private_key"].(string)
	if meta.(*FlexbotConfig).RancherApiEnabled && meta.(*FlexbotConfig).RancherConfig != nil {
		rancherClient = &(meta.(*FlexbotConfig).RancherConfig.Client)
	}
	if nodeConfig, err = setFlexbotInput(d, meta); err != nil {
		return
	}
	log.Printf("[INFO] Updating Server Compute for node %s", nodeConfig.Compute.HostName)
	err = meta.(*FlexbotConfig).UpdateManagerAcquire()
	defer meta.(*FlexbotConfig).UpdateManagerRelease()
	if err != nil {
		err = fmt.Errorf("resourceUpdateServer(compute): last resource instance update returned error: %s", err)
		return
	}
	if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
		meta.(*FlexbotConfig).UpdateManagerSetError(err)
		return
	}
	if rancherClient != nil {
		clusterId = p.Get("rancher_api").([]interface{})[0].(map[string]interface{})["cluster_id"].(string)
		if nodeId, err = rancherClient.GetNode(clusterId, network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err != nil {
			err = fmt.Errorf("resourceUpdateServer(compute): error: %s", err)
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
	}
	oldCompute, newCompute := d.GetChange("compute")
	oldBladeSpec := (oldCompute.([]interface{})[0].(map[string]interface{}))["blade_spec"].([]interface{})[0].(map[string]interface{})
	newBladeSpec := (newCompute.([]interface{})[0].(map[string]interface{}))["blade_spec"].([]interface{})[0].(map[string]interface{})
	bladeSpecChange := false
	for _, specItem := range []string{"dn", "model", "num_of_cpus", "num_of_cores", "total_memory"} {
		if oldBladeSpec[specItem].(string) != newBladeSpec[specItem].(string) {
			bladeSpecChange = true
		}
	}
	if bladeSpecChange {
		nodeConfig.Compute.BladeSpec.Dn = ""
	}
	if oldBladeSpec["dn"].(string) != newBladeSpec["dn"].(string) {
		nodeConfig.Compute.BladeSpec.Dn = newBladeSpec["dn"].(string)
		bladeSpecChange = true
	}
	if bladeSpecChange {
		if powerState == "up" && (newCompute.([]interface{})[0].(map[string]interface{}))["safe_removal"].(bool) {
			err = fmt.Errorf("resourceUpdateServer(compute): server %s has power state up", nodeConfig.Compute.HostName)
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if rancherClient != nil && len(nodeId) > 0 {
			log.Printf("[INFO] Rancher API: cordoning/draining node id=%s", nodeId)
			if err = rancherClient.NodeCordonDrain(nodeId, meta.(*FlexbotConfig).RancherConfig.NodeDrainInput); err != nil {
				err = fmt.Errorf("resourceUpdateServer(compute): error: %s", err)
				meta.(*FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		if (newCompute.([]interface{})[0].(map[string]interface{}))["wait_for_ssh_timeout"].(int) > 0 && len(sshUser) > 0 && len(sshPrivateKey) > 0 {
			// Trying graceful node shutdown
			if _, err = runSshCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey, "sudo shutdown -h 0"); err != nil {
				err = fmt.Errorf("resourceUpdateServer(compute): runSshCommand(shutdown) error: %s", err)
				meta.(*FlexbotConfig).UpdateManagerSetError(err)
				return
			}
			waitForShutdown := time.Now().Add(time.Second * time.Duration(60))
			for time.Now().Before(waitForShutdown) {
				if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
					meta.(*FlexbotConfig).UpdateManagerSetError(err)
					return
				}
				if powerState == "down" {
					break
				}
				time.Sleep(1 * time.Second)
			}
		}
		if powerState == "up" {
			if err = ucsm.StopServer(nodeConfig); err != nil {
				meta.(*FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		if err = ucsm.UpdateServer(nodeConfig); err != nil {
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if err = ucsm.StartServer(nodeConfig); err != nil {
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if (newCompute.([]interface{})[0].(map[string]interface{}))["wait_for_ssh_timeout"].(int) > 0 && len(sshUser) > 0 && len(sshPrivateKey) > 0 {
			if err = waitForSsh(nodeConfig, (newCompute.([]interface{})[0].(map[string]interface{}))["wait_for_ssh_timeout"].(int), sshUser, sshPrivateKey); err != nil {
				meta.(*FlexbotConfig).UpdateManagerSetError(err)
				return
			}
			if rancherClient != nil && len(nodeId) > 0 {
				if err = rancherClient.NodeWaitForState(nodeId, "active,drained,cordoned", 300); err == nil {
					if err = rancherClient.NodeUncordon(nodeId); err == nil {
						err = rancherClient.NodeWaitForState(nodeId, "active", 300)
					}
				}
				if err != nil {
					err = fmt.Errorf("resourceUpdateServer(compute): error: %s", err)
					meta.(*FlexbotConfig).UpdateManagerSetError(err)
					return
				}
				if meta.(*FlexbotConfig).NodeGraceTimeout > 0 {
					time.Sleep(time.Duration(meta.(*FlexbotConfig).NodeGraceTimeout) * time.Second)
				}
			}
		}
		if err = resourceReadServer(d, meta); err != nil {
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
		}
	}
	return
}

func resourceUpdateServerStorage(d *schema.ResourceData, meta interface{}) (err error) {
	var powerState, clusterId, nodeId string
	var nodeConfig *config.NodeConfig
	var rancherClient *rancher.Client
	p := meta.(*FlexbotConfig).FlexbotProvider
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	network := d.Get("network").([]interface{})[0].(map[string]interface{})
	sshUser := compute["ssh_user"].(string)
	sshPrivateKey := compute["ssh_private_key"].(string)
	if meta.(*FlexbotConfig).RancherApiEnabled && meta.(*FlexbotConfig).RancherConfig != nil {
		rancherClient = &(meta.(*FlexbotConfig).RancherConfig.Client)
	}
	if nodeConfig, err = setFlexbotInput(d, meta); err != nil {
		return
	}
	oldStorage, newStorage := d.GetChange("storage")
	oldBootLun := (oldStorage.([]interface{})[0].(map[string]interface{}))["boot_lun"].([]interface{})[0].(map[string]interface{})
	newBootLun := (newStorage.([]interface{})[0].(map[string]interface{}))["boot_lun"].([]interface{})[0].(map[string]interface{})
	oldSeedLun := (oldStorage.([]interface{})[0].(map[string]interface{}))["seed_lun"].([]interface{})[0].(map[string]interface{})
	newSeedLun := (newStorage.([]interface{})[0].(map[string]interface{}))["seed_lun"].([]interface{})[0].(map[string]interface{})
	if oldBootLun["os_image"].(string) != newBootLun["os_image"].(string) || oldSeedLun["seed_template"].(string) != newSeedLun["seed_template"].(string) {
		log.Printf("[INFO] Updating Server Storage image for node %s", nodeConfig.Compute.HostName)
		err = meta.(*FlexbotConfig).UpdateManagerAcquire()
		defer meta.(*FlexbotConfig).UpdateManagerRelease()
		if err != nil {
			err = fmt.Errorf("resourceUpdateServer(storage): last resource instance update returned error: %s", err)
			return
		}
		if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if rancherClient != nil {
			clusterId = p.Get("rancher_api").([]interface{})[0].(map[string]interface{})["cluster_id"].(string)
			if nodeId, err = rancherClient.GetNode(clusterId, network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err != nil {
				err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
				meta.(*FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		nodeConfig.Storage.BootLun.OsImage.Name = newBootLun["os_image"].(string)
		nodeConfig.Storage.SeedLun.SeedTemplate.Location = newSeedLun["seed_template"].(string)
		log.Printf("[INFO] Running boot storage preflight check")
		if err = ontap.CreateBootStoragePreflight(nodeConfig); err != nil {
			err = fmt.Errorf("resourceUpdateServer(storage): boot storage preflight check error: %s", err)
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		log.Printf("[INFO] Running seed storage preflight check")
		if err = ontap.CreateSeedStoragePreflight(nodeConfig); err != nil {
			err = fmt.Errorf("resourceUpdateServer(storage): seed storage preflight check error: %s", err)
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if powerState == "up" && compute["safe_removal"].(bool) {
			err = fmt.Errorf("resourceUpdateServer(storage): server %s has power state up", nodeConfig.Compute.HostName)
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if powerState == "up" {
			if rancherClient != nil && len(nodeId) > 0 {
				log.Printf("[INFO] Rancher API: cordoning/draining node id=%s", nodeId)
				if err = rancherClient.NodeCordonDrain(nodeId, meta.(*FlexbotConfig).RancherConfig.NodeDrainInput); err != nil {
					err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
					meta.(*FlexbotConfig).UpdateManagerSetError(err)
					return
				}
			}
			if err = ucsm.StopServer(nodeConfig); err != nil {
				meta.(*FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		if (newStorage.([]interface{})[0].(map[string]interface{}))["auto_snapshot_on_update"].(bool) {
			t := time.Now()
			snapshotName := fmt.Sprintf("terraform:%s:%s-%s", oldBootLun["os_image"].(string), oldSeedLun["seed_template"].(string), t.Format(time.RFC3339))
			if err = ontap.CreateSnapshot(nodeConfig, snapshotName, ""); err != nil {
				err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
				meta.(*FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		if err = ontap.DeleteBootLUNs(nodeConfig); err != nil {
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if err = ontap.CreateBootStorage(nodeConfig); err != nil {
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if err = ontap.CreateSeedStorage(nodeConfig); err != nil {
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if err = ucsm.StartServer(nodeConfig); err != nil {
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if compute["wait_for_ssh_timeout"].(int) > 0  && len(sshUser) > 0 && len(sshPrivateKey) > 0 && err == nil {
			if err = waitForSsh(nodeConfig, compute["wait_for_ssh_timeout"].(int), sshUser, sshPrivateKey); err == nil {
				for _, cmd := range compute["ssh_node_init_commands"].([]interface{}) {
					var cmdOutput string
					log.Printf("[INFO] Running SSH command on node %s: %s", nodeConfig.Compute.HostName, cmd.(string))
					if cmdOutput, err = runSshCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey, cmd.(string)); err != nil {
						meta.(*FlexbotConfig).UpdateManagerSetError(err)
						return
					}
					if len(cmdOutput) > 0 {
						log.Printf("[DEBUG] completed SSH command: exec: %s, output: %s", cmd.(string), cmdOutput)
					}
				}
			}
		}
		if rancherClient != nil && len(nodeId) > 0 {
			if err = rancherClient.NodeUncordon(nodeId); err == nil {
				err = rancherClient.NodeWaitForState(nodeId, "active", 300)
			}
			if err != nil {
				err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
				meta.(*FlexbotConfig).UpdateManagerSetError(err)
				return
			}
			if meta.(*FlexbotConfig).NodeGraceTimeout > 0 {
				time.Sleep(time.Duration(meta.(*FlexbotConfig).NodeGraceTimeout) * time.Second)
			}
		}
	}
	var resizeBootLun, resizeDataLun bool
	if oldBootLun["size"].(int) != newBootLun["size"].(int) {
		log.Printf("[INFO] Re-sizing Server Storage boot LUN for node %s", nodeConfig.Compute.HostName)
		resizeBootLun = true
	}
	if len((oldStorage.([]interface{})[0].(map[string]interface{}))["data_lun"].([]interface{})) > 0 && len((newStorage.([]interface{})[0].(map[string]interface{}))["data_lun"].([]interface{})) > 0 {
		oldDataLun := (oldStorage.([]interface{})[0].(map[string]interface{}))["data_lun"].([]interface{})[0].(map[string]interface{})
		newDataLun := (newStorage.([]interface{})[0].(map[string]interface{}))["data_lun"].([]interface{})[0].(map[string]interface{})
		if oldDataLun["size"].(int) != newDataLun["size"].(int) {
			log.Printf("[INFO] Re-sizing Server Storage data LUN for node %s", nodeConfig.Compute.HostName)
			resizeDataLun = true
		}
	}
	if resizeBootLun || resizeDataLun {
		if err = ontap.ResizeBootStorage(nodeConfig); err != nil {
			err = fmt.Errorf("resourceUpdateServer(storage): error: %s", err)
			return
		}
	}
	if compute["wait_for_ssh_timeout"].(int) > 0 && len(sshUser) > 0 && len(sshPrivateKey) > 0 {
		if resizeBootLun {
			for _, cmd := range compute["ssh_node_bootdisk_resize_commands"].([]interface{}) {
				var cmdOutput string
				log.Printf("[INFO] Running SSH command on node %s: %s", nodeConfig.Compute.HostName, cmd.(string))
				if cmdOutput, err = runSshCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey, cmd.(string)); err != nil {
					return
				}
				if len(cmdOutput) > 0 {
					log.Printf("[DEBUG] completed SSH command: exec: %s, output: %s", cmd.(string), cmdOutput)
				}
			}
		}
		if resizeDataLun {
			for _, cmd := range compute["ssh_node_datadisk_resize_commands"].([]interface{}) {
				var cmdOutput string
				log.Printf("[INFO] Running SSH command on node %s: %s", nodeConfig.Compute.HostName, cmd.(string))
				if cmdOutput, err = runSshCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey, cmd.(string)); err != nil {
					return
				}
				if len(cmdOutput) > 0 {
					log.Printf("[DEBUG] completed SSH command: exec: %s, output: %s", cmd.(string), cmdOutput)
				}
			}
		}
	}
	return
}

func resourceUpdateServerSnapshot(d *schema.ResourceData, meta interface{}) (err error) {
	var oldSnapState, newSnapState, snapStateInter, snapStorage []string
	var nodeConfig *config.NodeConfig
	if nodeConfig, err = setFlexbotInput(d, meta); err != nil {
		return
	}
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	sshUser := compute["ssh_user"].(string)
	sshPrivateKey := compute["ssh_private_key"].(string)
	oldSnapshot, newSnapshot := d.GetChange("snapshot")
	for _, snapshot := range oldSnapshot.([]interface{}) {
		oldSnapState = append(oldSnapState, snapshot.(map[string]interface{})["name"].(string))
	}
	for _, snapshot := range newSnapshot.([]interface{}) {
		newSnapState = append(newSnapState, snapshot.(map[string]interface{})["name"].(string))
	}
	snapStateInter = stringSliceIntersection(oldSnapState, newSnapState)
	if snapStorage, err = ontap.GetSnapshots(nodeConfig); err != nil {
		err = fmt.Errorf("resourceUpdateServer(): %s", err)
		return
	}
	for _, name := range oldSnapState {
		if stringSliceElementExists(snapStorage, name) && !stringSliceElementExists(snapStateInter, name) {
			if err = ontap.DeleteSnapshot(nodeConfig, name); err != nil {
				err = fmt.Errorf("resourceUpdateServer(): %s", err)
				return
			}
		}
	}
	for _, name := range newSnapState {
		if !stringSliceElementExists(snapStorage, name) && !stringSliceElementExists(snapStateInter, name) {
			for _, snapshot := range newSnapshot.([]interface{}) {
				if snapshot.(map[string]interface{})["name"].(string) == name {
					if snapshot.(map[string]interface{})["fsfreeze"].(bool) {
						if len(sshUser) > 0 && len(sshPrivateKey) > 0 {
							err = createSnapshot(nodeConfig, sshUser, sshPrivateKey, name)
						} else {
							err = fmt.Errorf("expected compute.ssh_user and compute.ssh_private_key parameters to ensure fsfreeze for snapshot %s", name)
						}
					} else {
						err = ontap.CreateSnapshot(nodeConfig, name, "")
					}
					if err != nil {
						err = fmt.Errorf("resourceUpdateServer(): %s", err)
						return
					}
				}
			}
		}
	}
	return
}

func resourceUpdateServerRestore(d *schema.ResourceData, meta interface{}) (err error) {
	var powerState, clusterId, nodeId string
	var nodeConfig *config.NodeConfig
	var rancherClient *rancher.Client
	p := meta.(*FlexbotConfig).FlexbotProvider
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	network := d.Get("network").([]interface{})[0].(map[string]interface{})
	sshUser := compute["ssh_user"].(string)
	sshPrivateKey := compute["ssh_private_key"].(string)
	if meta.(*FlexbotConfig).RancherApiEnabled && meta.(*FlexbotConfig).RancherConfig != nil {
		rancherClient = &(meta.(*FlexbotConfig).RancherConfig.Client)
	}
	if nodeConfig, err = setFlexbotInput(d, meta); err != nil {
		return
	}
	oldRestore, newRestore := d.GetChange("restore")
	if len(newRestore.([]interface{})) == 0 {
		return
	}
	restore := newRestore.([]interface{})[0].(map[string]interface{})
	if !restore["restore"].(bool) {
		return
	}
	log.Printf("[INFO] Restoring Server Storage from snapshot")
	err = meta.(*FlexbotConfig).UpdateManagerAcquire()
	defer meta.(*FlexbotConfig).UpdateManagerRelease()
	if err != nil {
		err = fmt.Errorf("resourceUpdateServer(storage): last resource instance update returned error: %s", err)
		return
	}
	if restore["snapshot_name"] == nil || len(restore["snapshot_name"].(string)) == 0 {
		var lastSnapshot string
		var snapshotList []string
		tRFC3339exp, _ := regexp.Compile(`20[0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]-[0-9][0-9]:[0-9][0-9]$`)
		lastSnapshotCreated := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
		if snapshotList, err =  ontap.GetSnapshots(nodeConfig); err != nil {
			err = fmt.Errorf("resourceUpdateServer(restore): error: %s", err)
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		for _, snapshot := range snapshotList {
    			timeFormatted := tRFC3339exp.FindString(snapshot)
    			if len(timeFormatted) > 0 {
    				if snapshotCreated, err := time.Parse(time.RFC3339, timeFormatted); err == nil {
    					if snapshotCreated.Unix() > lastSnapshotCreated.Unix() {
    						lastSnapshotCreated = snapshotCreated
    						lastSnapshot = snapshot
    					}
    				}
    			}
			if len(lastSnapshot) > 0 {
				restore["snapshot_name"] = lastSnapshot
			}
		}
	}
	if restore["snapshot_name"] == nil || len(restore["snapshot_name"].(string)) == 0 {
		err = fmt.Errorf("resourceUpdateServer(restore): snapshot not found, expected snapshot_name")
		meta.(*FlexbotConfig).UpdateManagerSetError(err)
		return
	}
	if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
		meta.(*FlexbotConfig).UpdateManagerSetError(err)
		return
	}
	if rancherClient != nil {
		clusterId = p.Get("rancher_api").([]interface{})[0].(map[string]interface{})["cluster_id"].(string)
		if nodeId, err = rancherClient.GetNode(clusterId, network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err != nil {
			err = fmt.Errorf("resourceUpdateServer(restore): error: %s", err)
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
	}
	if powerState == "up" && compute["safe_removal"].(bool) {
		err = fmt.Errorf("resourceUpdateServer(restore): server %s has power state up", nodeConfig.Compute.HostName)
		meta.(*FlexbotConfig).UpdateManagerSetError(err)
		return
	}
	if powerState == "up" {
		if rancherClient != nil && len(nodeId) > 0 {
			log.Printf("[INFO] Rancher API: cordoning/draining node id=%s", nodeId)
			if err = rancherClient.NodeCordonDrain(nodeId, meta.(*FlexbotConfig).RancherConfig.NodeDrainInput); err != nil {
				err = fmt.Errorf("resourceUpdateServer(restore): error: %s", err)
				meta.(*FlexbotConfig).UpdateManagerSetError(err)
				return
			}
		}
		if err = ucsm.StopServer(nodeConfig); err != nil {
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
	}
	if err = ontap.RestoreSnapshot(nodeConfig, restore["snapshot_name"].(string)); err != nil {
		err = fmt.Errorf("resourceUpdateServer(restore): error: %s", err)
		meta.(*FlexbotConfig).UpdateManagerSetError(err)
		return
	}
	if err = ontap.LunRestoreMapping(nodeConfig); err != nil {
		err = fmt.Errorf("resourceUpdateServer(restore): error: %s", err)
		meta.(*FlexbotConfig).UpdateManagerSetError(err)
		return
	}
	if err = ucsm.StartServer(nodeConfig); err != nil {
		meta.(*FlexbotConfig).UpdateManagerSetError(err)
		return
	}
	if compute["wait_for_ssh_timeout"].(int) > 0  && len(sshUser) > 0 && len(sshPrivateKey) > 0 {
		if err = waitForSsh(nodeConfig, compute["wait_for_ssh_timeout"].(int), sshUser, sshPrivateKey); err != nil {
			meta.(*FlexbotConfig).UpdateManagerSetError(err)
			return
		}
		if rancherClient != nil && len(nodeId) > 0 {
			if err = rancherClient.NodeWaitForState(nodeId, "active,drained,cordoned", 300); err == nil {
				if err = rancherClient.NodeUncordon(nodeId); err == nil {
					err = rancherClient.NodeWaitForState(nodeId, "active", 300)
				}
			}
			if err != nil {
				err = fmt.Errorf("resourceUpdateServer(restore): error: %s", err)
				meta.(*FlexbotConfig).UpdateManagerSetError(err)
				return
			}
			if meta.(*FlexbotConfig).NodeGraceTimeout > 0 {
				time.Sleep(time.Duration(meta.(*FlexbotConfig).NodeGraceTimeout) * time.Second)
			}
		}
	}
	d.Set("restore", oldRestore)
	return
}

func resourceDeleteServer(d *schema.ResourceData, meta interface{}) (err error) {
	var powerState, clusterId, nodeId string
	var nodeConfig *config.NodeConfig
	var rancherClient *rancher.Client
	p := meta.(*FlexbotConfig).FlexbotProvider
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	network := d.Get("network").([]interface{})[0].(map[string]interface{})
	if nodeConfig, err = setFlexbotInput(d, meta); err != nil {
		return
	}
	if meta.(*FlexbotConfig).RancherApiEnabled && meta.(*FlexbotConfig).RancherConfig != nil {
		rancherClient = &(meta.(*FlexbotConfig).RancherConfig.Client)
	}
	log.Printf("[INFO] Deleting Server %s", nodeConfig.Compute.HostName)
	if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
		return
	}
	if powerState == "up" && compute["safe_removal"].(bool) {
		err = fmt.Errorf("resourceDeleteServer(): server %s has power state up", nodeConfig.Compute.HostName)
		return
	}
	if rancherClient != nil {
		clusterId = p.Get("rancher_api").([]interface{})[0].(map[string]interface{})["cluster_id"].(string)
		if nodeId, err = rancherClient.GetNode(clusterId, network["node"].([]interface{})[0].(map[string]interface{})["ip"].(string)); err != nil {
			err = fmt.Errorf("resourceDeleteServer(): rancherClient.GetNode() error: %s", err)
			return
		}
	}
	if powerState == "up" {
		if rancherClient != nil && len(nodeId) > 0 {
			log.Printf("[INFO] Rancher API: cordoning/draining node id=%s", nodeId)
			if err = rancherClient.NodeCordonDrain(nodeId, meta.(*FlexbotConfig).RancherConfig.NodeDrainInput); err != nil {
				err = fmt.Errorf("resourceDeleteServer(): rancherClient.NodeCordonDrain() error: %s", err)
				return
			}
		}
		if err = ucsm.StopServer(nodeConfig); err != nil {
			return
		}
	}
	if rancherClient != nil && len(nodeId) > 0 {
		log.Printf("[INFO] Rancher API: deleting node id=%s", nodeId)
		if err = rancherClient.DeleteNode(nodeId); err != nil {
			err = fmt.Errorf("resourceDeleteServer(): rancherClient.DeleteNode() error: %s", err)
			return
		}
	}
	var stepErr error
	stepErrMsg := []string{}
	stepErr = ucsm.DeleteServer(nodeConfig)
	if stepErr != nil {
		stepErrMsg = append(stepErrMsg, stepErr.Error())
	}
	stepErr = ontap.DeleteBootStorage(nodeConfig)
	if stepErr != nil {
		stepErrMsg = append(stepErrMsg, stepErr.Error())
	}
	var provider ipam.IpamProvider
	switch nodeConfig.Ipam.Provider {
	case "Infoblox":
		provider = ipam.NewInfobloxProvider(&nodeConfig.Ipam)
	case "Internal":
		provider = ipam.NewInternalProvider(&nodeConfig.Ipam)
	default:
		err = fmt.Errorf("resourceDeleteServer(): IPAM provider %s is not implemented", nodeConfig.Ipam.Provider)
		return
	}
	stepErr = provider.Release(nodeConfig)
	if stepErr != nil {
		stepErrMsg = append(stepErrMsg, stepErr.Error())
	}
	if len(stepErrMsg) > 0 {
		err = fmt.Errorf("resourceDeleteServer(): %s", strings.Join(stepErrMsg, "\n"))
	}
	return
}

func resourceImportServer(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	err := resourceReadServer(d, meta)
	if err != nil {
		return []*schema.ResourceData{}, err
	}
	return []*schema.ResourceData{d}, nil
}

func setFlexbotInput(d *schema.ResourceData, meta interface{}) (*config.NodeConfig, error) {
	var nodeConfig config.NodeConfig
	var err error
	meta.(*FlexbotConfig).Sync.Lock()
	defer meta.(*FlexbotConfig).Sync.Unlock()
	p := meta.(*FlexbotConfig).FlexbotProvider
	p_ipam := p.Get("ipam").([]interface{})[0].(map[string]interface{})
	nodeConfig.Ipam.Provider = p_ipam["provider"].(string)
	nodeConfig.Ipam.DnsZone = p_ipam["dns_zone"].(string)
	ibCredentials := p_ipam["credentials"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Ipam.IbCredentials.Host = ibCredentials["host"].(string)
	nodeConfig.Ipam.IbCredentials.User = ibCredentials["user"].(string)
	nodeConfig.Ipam.IbCredentials.Password = ibCredentials["password"].(string)
	nodeConfig.Ipam.IbCredentials.WapiVersion = ibCredentials["wapi_version"].(string)
	nodeConfig.Ipam.IbCredentials.DnsView = ibCredentials["dns_view"].(string)
	nodeConfig.Ipam.IbCredentials.NetworkView = ibCredentials["network_view"].(string)
	p_compute := p.Get("compute").([]interface{})[0].(map[string]interface{})
	ucsmCredentials := p_compute["credentials"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Compute.UcsmCredentials.Host = ucsmCredentials["host"].(string)
	nodeConfig.Compute.UcsmCredentials.User = ucsmCredentials["user"].(string)
	nodeConfig.Compute.UcsmCredentials.Password = ucsmCredentials["password"].(string)
	p_storage := p.Get("storage").([]interface{})[0].(map[string]interface{})
	cdotCredentials := p_storage["credentials"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Storage.CdotCredentials.Host = cdotCredentials["host"].(string)
	nodeConfig.Storage.CdotCredentials.User = cdotCredentials["user"].(string)
	nodeConfig.Storage.CdotCredentials.Password = cdotCredentials["password"].(string)
	nodeConfig.Storage.CdotCredentials.ZapiVersion = cdotCredentials["zapi_version"].(string)
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	nodeConfig.Compute.SpOrg = compute["sp_org"].(string)
	nodeConfig.Compute.SpTemplate = compute["sp_template"].(string)
	if len(compute["blade_spec"].([]interface{})) > 0 {
		bladeSpec := compute["blade_spec"].([]interface{})[0].(map[string]interface{})
		nodeConfig.Compute.BladeSpec.Dn = bladeSpec["dn"].(string)
		nodeConfig.Compute.BladeSpec.Model = bladeSpec["model"].(string)
		nodeConfig.Compute.BladeSpec.NumOfCpus = bladeSpec["num_of_cpus"].(string)
		nodeConfig.Compute.BladeSpec.NumOfCores = bladeSpec["num_of_cores"].(string)
		nodeConfig.Compute.BladeSpec.TotalMemory = bladeSpec["total_memory"].(string)
	}
	storage := d.Get("storage").([]interface{})[0].(map[string]interface{})
	nodeConfig.Storage.SvmName = storage["svm_name"].(string)
	nodeConfig.Storage.ImageRepoName = storage["image_repo_name"].(string)
	nodeConfig.Storage.VolumeName = storage["volume_name"].(string)
	nodeConfig.Storage.IgroupName = storage["igroup_name"].(string)
	bootLun := storage["boot_lun"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Storage.BootLun.Name = bootLun["name"].(string)
	nodeConfig.Storage.BootLun.Size = bootLun["size"].(int)
	seedLun := storage["seed_lun"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Storage.SeedLun.Name = seedLun["name"].(string)
	if len(storage["data_lun"].([]interface{})) > 0 {
		dataLun := storage["data_lun"].([]interface{})[0].(map[string]interface{})
		nodeConfig.Storage.DataLun.Name = dataLun["name"].(string)
		nodeConfig.Storage.DataLun.Size = dataLun["size"].(int)
	}
	network := d.Get("network").([]interface{})[0].(map[string]interface{})
	for i, _ := range network["node"].([]interface{}) {
		node := network["node"].([]interface{})[i].(map[string]interface{})
		nodeConfig.Network.Node = append(nodeConfig.Network.Node, config.NetworkInterface{})
		nodeConfig.Network.Node[i].Name = node["name"].(string)
		nodeConfig.Network.Node[i].Macaddr = node["macaddr"].(string)
		nodeConfig.Network.Node[i].Ip = node["ip"].(string)
		nodeConfig.Network.Node[i].Fqdn = node["fqdn"].(string)
		nodeConfig.Network.Node[i].Subnet = node["subnet"].(string)
		nodeConfig.Network.Node[i].Gateway = node["gateway"].(string)
		nodeConfig.Network.Node[i].DnsServer1 = node["dns_server1"].(string)
		nodeConfig.Network.Node[i].DnsServer2 = node["dns_server2"].(string)
		nodeConfig.Network.Node[i].DnsDomain = node["dns_domain"].(string)
	}
	for i, _ := range network["iscsi_initiator"].([]interface{}) {
		initiator := network["iscsi_initiator"].([]interface{})[i].(map[string]interface{})
		nodeConfig.Network.IscsiInitiator = append(nodeConfig.Network.IscsiInitiator, config.IscsiInitiator{})
		nodeConfig.Network.IscsiInitiator[i].Name = initiator["name"].(string)
		nodeConfig.Network.IscsiInitiator[i].Ip = initiator["ip"].(string)
		nodeConfig.Network.IscsiInitiator[i].Fqdn = initiator["fqdn"].(string)
		nodeConfig.Network.IscsiInitiator[i].Subnet = initiator["subnet"].(string)
		nodeConfig.Network.IscsiInitiator[i].Gateway = initiator["gateway"].(string)
		nodeConfig.Network.IscsiInitiator[i].DnsServer1 = initiator["dns_server1"].(string)
		nodeConfig.Network.IscsiInitiator[i].DnsServer2 = initiator["dns_server2"].(string)
		nodeConfig.Network.IscsiInitiator[i].InitiatorName = initiator["initiator_name"].(string)
		nodeConfig.Network.IscsiInitiator[i].IscsiTarget = &config.IscsiTarget{}
		if len(initiator["iscsi_target"].([]interface{})) > 0 {
            		nodeConfig.Network.IscsiInitiator[i].IscsiTarget.NodeName = initiator["iscsi_target"].([]interface{})[0].(map[string]interface{})["node_name"].(string)
			for _, target_addr := range initiator["iscsi_target"].([]interface{})[0].(map[string]interface{})["interfaces"].([]interface{}) {
                    		nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces = append(nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces, target_addr.(string))
            		}
		}
	}
	nodeConfig.CloudArgs = make(map[string]string)
	for argKey, argValue := range d.Get("cloud_args").(map[string]interface{}) {
		nodeConfig.CloudArgs[argKey] = argValue.(string)
	}
	passPhrase := p.Get("pass_phrase").(string)
	if passPhrase == "" {
		if passPhrase, err = machineid.ID(); err != nil {
			return nil, err
		}
	}
	if err = config.SetDefaults(&nodeConfig, compute["hostname"].(string), bootLun["os_image"].(string), seedLun["seed_template"].(string), passPhrase); err != nil {
		err = fmt.Errorf("SetDefaults(): failure: %s", err)
	}
	return &nodeConfig, err
}

func setFlexbotOutput(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) {
	meta.(*FlexbotConfig).Sync.Lock()
	defer meta.(*FlexbotConfig).Sync.Unlock()
	compute := d.Get("compute").([]interface{})[0].(map[string]interface{})
	network := d.Get("network").([]interface{})[0].(map[string]interface{})
	storage := d.Get("storage").([]interface{})[0].(map[string]interface{})
	compute["sp_dn"] = nodeConfig.Compute.SpDn
	if len(compute["blade_spec"].([]interface{})) > 0 {
		bladeSpec := compute["blade_spec"].([]interface{})[0].(map[string]interface{})
		bladeSpec["dn"] = nodeConfig.Compute.BladeSpec.Dn
		compute["blade_spec"].([]interface{})[0] = bladeSpec
	}
	storage["svm_name"] = nodeConfig.Storage.SvmName
	storage["image_repo_name"] = nodeConfig.Storage.ImageRepoName
	storage["volume_name"] = nodeConfig.Storage.VolumeName
	storage["igroup_name"] = nodeConfig.Storage.IgroupName
	bootLun := storage["boot_lun"].([]interface{})[0].(map[string]interface{})
	bootLun["name"] = nodeConfig.Storage.BootLun.Name
	bootLun["id"] = nodeConfig.Storage.BootLun.Id
	if nodeConfig.Storage.BootLun.Size > 0 {
		bootLun["size"] = nodeConfig.Storage.BootLun.Size
	}
	if nodeConfig.Storage.BootLun.OsImage.Name != "" {
		bootLun["os_image"] = nodeConfig.Storage.BootLun.OsImage.Name
	}
	storage["boot_lun"].([]interface{})[0] = bootLun
	seedLun := storage["seed_lun"].([]interface{})[0].(map[string]interface{})
	seedLun["name"] = nodeConfig.Storage.SeedLun.Name
	seedLun["id"] = nodeConfig.Storage.SeedLun.Id
	if nodeConfig.Storage.SeedLun.SeedTemplate.Location != "" {
		seedLun["seed_template"] = nodeConfig.Storage.SeedLun.SeedTemplate.Location
	}
	storage["seed_lun"].([]interface{})[0] = seedLun
	if len(storage["data_lun"].([]interface{})) > 0 {
		dataLun := storage["data_lun"].([]interface{})[0].(map[string]interface{})
		dataLun["name"] = nodeConfig.Storage.DataLun.Name
		dataLun["id"] = nodeConfig.Storage.DataLun.Id
		if nodeConfig.Storage.DataLun.Size > 0 {
			dataLun["size"] = nodeConfig.Storage.DataLun.Size
		}
		storage["data_lun"].([]interface{})[0] = dataLun
	}
	storage["snapshots"] = []string{}
	for _, snapshot := range nodeConfig.Storage.Snapshots {
		storage["snapshots"] = append(storage["snapshots"].([]string), snapshot)
	}
	for i, _ := range network["node"].([]interface{}) {
		node := network["node"].([]interface{})[i].(map[string]interface{})
		node["macaddr"] = nodeConfig.Network.Node[i].Macaddr
		node["ip"] = nodeConfig.Network.Node[i].Ip
		node["fqdn"] = nodeConfig.Network.Node[i].Fqdn
		network["node"].([]interface{})[i] = node
	}
	for i, _ := range network["iscsi_initiator"].([]interface{}) {
		initiator := network["iscsi_initiator"].([]interface{})[i].(map[string]interface{})
		initiator["ip"] = nodeConfig.Network.IscsiInitiator[i].Ip
		initiator["initiator_name"] = nodeConfig.Network.IscsiInitiator[i].InitiatorName
		initiator["fqdn"] = nodeConfig.Network.IscsiInitiator[i].Fqdn
		initiator["subnet"] = nodeConfig.Network.IscsiInitiator[i].Subnet
		initiator["gateway"] = nodeConfig.Network.IscsiInitiator[i].Gateway
		initiator["dns_server1"] = nodeConfig.Network.IscsiInitiator[i].DnsServer1
		initiator["dns_server2"] = nodeConfig.Network.IscsiInitiator[i].DnsServer2
		if len(initiator["iscsi_target"].([]interface{})) == 0 {
			if nodeConfig.Network.IscsiInitiator[i].IscsiTarget != nil {
				iscsi_target := make(map[string]interface{})
				iscsi_target["node_name"] = nodeConfig.Network.IscsiInitiator[i].IscsiTarget.NodeName
				iscsi_target["interfaces"] = []string{}
				for _, iface := range nodeConfig.Network.IscsiInitiator[i].IscsiTarget.Interfaces {
					iscsi_target["interfaces"] = append(iscsi_target["interfaces"].([]string), iface)
				}
				initiator["iscsi_target"] = append(initiator["iscsi_target"].([]interface{}), iscsi_target)
			}
		}
		network["iscsi_initiator"].([]interface{})[i] = initiator
	}
	d.Set("compute", []interface{}{compute})
	d.Set("network", []interface{}{network})
	d.Set("storage", []interface{}{storage})
}

func createSnapshot(nodeConfig *config.NodeConfig, sshUser string, sshPrivateKey string, snapshotName string) (err error) {
	var filesystems, cmd, errs []string
	var signer ssh.Signer
	var conn *ssh.Client
	var sess *ssh.Session
	var b_stdout, b_stderr bytes.Buffer
	if signer, err = ssh.ParsePrivateKey([]byte(sshPrivateKey)); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to parse SSH private key: %s", err)
		return
	}
	config := &ssh.ClientConfig {
		User: sshUser,
		Auth: []ssh.AuthMethod {
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	if conn, err = ssh.Dial("tcp", nodeConfig.Network.Node[0].Ip + ":22", config); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to connect to host %s: %s", nodeConfig.Network.Node[0].Ip, err)
		return
	}
	defer conn.Close()
	if sess, err = conn.NewSession(); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to create SSH session: %s", err)
		return
	}
	sess.Stdout = &b_stdout
	sess.Stderr = &b_stderr
	err = sess.Run(`cat /proc/mounts | sed -n 's/^\/dev\/mapper\/[^ ]\+[ ]\+\(\/[^ ]\{1,64\}\).*/\1/p'`)
	sess.Close()
	if err != nil {
		err = fmt.Errorf("createSnapshot(): failed to run command: %s: %s", err, b_stderr.String())
		return
	}
	if b_stdout.Len() > 0 {
		filesystems = strings.Split(strings.Trim(b_stdout.String(), "\n"), "\n")
	}
	for _, fs := range filesystems {
		cmd = append(cmd, "fsfreeze -f " + fs)
	}
	cmd = append(cmd, "fsfreeze -f / && echo -n frozen && sleep 5 && fsfreeze -u /")
	for _, fs := range filesystems {
		cmd = append(cmd, "fsfreeze -u " + fs)
	}
	if sess, err = conn.NewSession(); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to create SSH session: %s", err)
		return
	}
	defer sess.Close()
	b_stdout.Reset()
	b_stderr.Reset()
	sess.Stdout = &b_stdout
	sess.Stderr = &b_stderr
	if err = sess.Start(fmt.Sprintf(`sudo -n sh -c '%s'`, strings.Join(cmd, " && "))); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to start SSH command: %s", err)
		return
	}
	for i := 0; i < 10; i++ {
		if b_stdout.Len() > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if b_stdout.String() == "frozen" {
		if err = ontap.CreateSnapshot(nodeConfig, snapshotName, ""); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if err = sess.Wait(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to run SSH command: %s: %s", err, b_stderr.String()))
	}
	if err != nil {
		err = fmt.Errorf("createSnapshot(): %s", strings.Join(errs, " , "))
	}
	return
}

func waitForSsh(nodeConfig *config.NodeConfig, waitForSshTimeout int, sshUser string, sshPrivateKey string) (err error) {
	giveupTime := time.Now().Add(time.Second * time.Duration(waitForSshTimeout))
	restartTime := time.Now().Add(time.Second * 600)
	for time.Now().Before(giveupTime) {
		if checkSshListen(nodeConfig.Network.Node[0].Ip) {
			if len(sshUser) > 0 && len(sshPrivateKey) > 0 {
				stabilazeTime := time.Now().Add(time.Second * 60)
				for time.Now().Before(stabilazeTime) {
					if err = checkSshCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey); err == nil {
						break
					}
					time.Sleep(1 * time.Second)
				}
			}
			if err == nil {
				break
			}
		}
		time.Sleep(1 * time.Second)
		if time.Now().After(restartTime) {
			ucsm.StopServer(nodeConfig)
			ucsm.StartServer(nodeConfig)
			restartTime = time.Now().Add(time.Second * 600)
		}
	}
	if time.Now().After(giveupTime) && err != nil {
		err = fmt.Errorf("waitForSsh(): exceeded timeout %d: %s", waitForSshTimeout, err)
	}
	return
}
