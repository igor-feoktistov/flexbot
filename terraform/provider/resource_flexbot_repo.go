package flexbot

import (
	"fmt"
	"log"
	"time"

	"flexbot/pkg/config"
	"flexbot/pkg/ontap"
	"github.com/denisbrodbeck/machineid"
        "github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func resourceFlexbotRepo() *schema.Resource {
	return &schema.Resource{
		Schema: schemaFlexbotRepo(),
		Create: resourceCreateRepo,
		Read:   resourceReadRepo,
		Update: resourceUpdateRepo,
		Delete: resourceDeleteRepo,
		Importer: &schema.ResourceImporter{
			State: resourceImportRepo,
		},
                Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(7200 * time.Second),
			Update: schema.DefaultTimeout(7200 * time.Second),
			Delete: schema.DefaultTimeout(600 * time.Second),
                },
	}
}

func resourceCreateRepo(d *schema.ResourceData, meta interface{}) (err error) {
	var nodeConfig *config.NodeConfig
	log.Printf("[INFO] Creating Image Repository")
	if nodeConfig, err = setRepoInput(d, meta); err != nil {
		return
	}
	if err = createRepo(d, nodeConfig, "image_repo"); err != nil {
		return
	}
	if err = createRepo(d, nodeConfig, "template_repo"); err != nil {
		return
	}
	if err = resourceReadRepo(d, meta); err != nil {
		return
	}
	d.SetId(nodeConfig.Storage.CdotCredentials.Host + ":/repo")
	return
}

func createRepo(d *schema.ResourceData, nodeConfig *config.NodeConfig, repo string) (err error) {
	var repoStorage []string
	if repo == "image_repo" {
		repoStorage, err = ontap.GetRepoImages(nodeConfig)
	}
	if repo == "template_repo" {
		repoStorage, err = ontap.GetRepoTemplates(nodeConfig)
	}
	for _, repoItem := range d.Get(repo).([]interface{}) {
		if err == nil && len(repoItem.(map[string]interface{})["location"].(string)) > 0 {
			if repo == "image_repo" {
				if stringSliceElementExists(repoStorage, repoItem.(map[string]interface{})["name"].(string)) {
					err = ontap.DeleteRepoImage(nodeConfig, repoItem.(map[string]interface{})["name"].(string))
					time.Sleep(5 * time.Second)
				}
				if err == nil {
					err = ontap.CreateRepoImage(nodeConfig, repoItem.(map[string]interface{})["name"].(string), repoItem.(map[string]interface{})["location"].(string))
				}
			}
			if repo == "template_repo" {
				if stringSliceElementExists(repoStorage, repoItem.(map[string]interface{})["name"].(string)) {
					err = ontap.DeleteRepoTemplate(nodeConfig, repoItem.(map[string]interface{})["name"].(string))
					time.Sleep(5 * time.Second)
				}
				if err == nil {
					err = ontap.CreateRepoTemplate(nodeConfig, repoItem.(map[string]interface{})["name"].(string), repoItem.(map[string]interface{})["location"].(string))
				}
			}
		}
	}
	if err != nil {
		err = fmt.Errorf("resourceCreateRepo(%s): %s", repo, err)
	}
	return
}

func resourceReadRepo(d *schema.ResourceData, meta interface{}) (err error) {
	var nodeConfig *config.NodeConfig
	log.Printf("[INFO] Reading Image Repository")
	if nodeConfig, err = setRepoInput(d, meta); err == nil {
		err = setRepoOutput(d, meta, nodeConfig)
	}
	if err != nil {
		err = fmt.Errorf("resourceReadRepo(): %s", err)
	}
	return
}

func resourceUpdateRepo(d *schema.ResourceData, meta interface{}) (err error) {
	log.Printf("[INFO] Updating Image Repository")
	if d.HasChange("image_repo") && !d.IsNewResource() {
		err = updateRepo(d, meta, "image_repo")
	}
	if err == nil && d.HasChange("template_repo") && !d.IsNewResource() {
		err = updateRepo(d, meta, "template_repo")
	}
	if err == nil {
		err = resourceReadRepo(d, meta)
	}
	return
}

func updateRepo(d *schema.ResourceData, meta interface{}, repo string) (err error) {
	var oldRepoState, newRepoState, repoStateInter, repoStorage []string
	var nodeConfig *config.NodeConfig
	if nodeConfig, err = setRepoInput(d, meta); err != nil {
		return
	}
	oldRepo, newRepo := d.GetChange(repo)
	for _, repoItem := range oldRepo.([]interface{}) {
		oldRepoState = append(oldRepoState, repoItem.(map[string]interface{})["name"].(string))
	}
	for _, repoItem := range newRepo.([]interface{}) {
		newRepoState = append(newRepoState, repoItem.(map[string]interface{})["name"].(string))
	}
	repoStateInter = stringSliceIntersection(oldRepoState, newRepoState)
	if repo == "image_repo" {
		repoStorage, err = ontap.GetRepoImages(nodeConfig)
	}
	if repo == "template_repo" {
		repoStorage, err = ontap.GetRepoTemplates(nodeConfig)
	}
	for _, name := range oldRepoState {
		if err == nil {
			if stringSliceElementExists(repoStorage, name) && !stringSliceElementExists(repoStateInter, name) {
				if repo == "image_repo" {
					err = ontap.DeleteRepoImage(nodeConfig, name)
				}
				if repo == "template_repo" {
					err = ontap.DeleteRepoTemplate(nodeConfig, name)
				}
			}
		}
	}
	if err == nil {
		if repo == "image_repo" {
			repoStorage, err = ontap.GetRepoImages(nodeConfig)
		}
		if repo == "template_repo" {
			repoStorage, err = ontap.GetRepoTemplates(nodeConfig)
		}
	}
	for _, newRepoItem := range newRepo.([]interface{}) {
		if err == nil {
			if len(newRepoItem.(map[string]interface{})["location"].(string)) > 0 {
				locationChanged := true
				for _, oldRepoItem := range oldRepo.([]interface{}) {
					if newRepoItem.(map[string]interface{})["name"].(string) == oldRepoItem.(map[string]interface{})["name"].(string) && newRepoItem.(map[string]interface{})["location"].(string) == oldRepoItem.(map[string]interface{})["location"].(string) {
						locationChanged = false
					}
				}
				if repo == "image_repo" {
					if stringSliceElementExists(repoStorage, newRepoItem.(map[string]interface{})["name"].(string)) && locationChanged {
						err = ontap.DeleteRepoImage(nodeConfig, newRepoItem.(map[string]interface{})["name"].(string))
						time.Sleep(5 * time.Second)
					}
					if err == nil && (!stringSliceElementExists(repoStateInter, newRepoItem.(map[string]interface{})["name"].(string)) || locationChanged) {
						err = ontap.CreateRepoImage(nodeConfig, newRepoItem.(map[string]interface{})["name"].(string), newRepoItem.(map[string]interface{})["location"].(string))
					}
				}
				if repo == "template_repo" {
					if stringSliceElementExists(repoStorage, newRepoItem.(map[string]interface{})["name"].(string)) && locationChanged {
						err = ontap.DeleteRepoTemplate(nodeConfig, newRepoItem.(map[string]interface{})["name"].(string))
						time.Sleep(5 * time.Second)
					}
					if err == nil && (!stringSliceElementExists(repoStateInter, newRepoItem.(map[string]interface{})["name"].(string)) || locationChanged) {
						err = ontap.CreateRepoTemplate(nodeConfig, newRepoItem.(map[string]interface{})["name"].(string), newRepoItem.(map[string]interface{})["location"].(string))
					}
				}
			}
		}
	}
	if err != nil {
		err = fmt.Errorf("resourceUpdateRepo(%s): %s", repo, err)
	}
	return
}

func resourceDeleteRepo(d *schema.ResourceData, meta interface{}) (err error) {
	var nodeConfig *config.NodeConfig
	log.Printf("[INFO] Deleting Image Repository")
	nodeConfig, err = setRepoInput(d, meta)
	for _, repoItem := range d.Get("image_repo").([]interface{}) {
		if err == nil {
			err = ontap.DeleteRepoImage(nodeConfig, repoItem.(map[string]interface{})["name"].(string))
		}
	}
	for _, repoItem := range d.Get("template_repo").([]interface{}) {
		if err == nil {
			err = ontap.DeleteRepoTemplate(nodeConfig, repoItem.(map[string]interface{})["name"].(string))
		}
	}
	if err != nil {
		err = fmt.Errorf("resourceDeleteRepo(): %s", err)
	} else {
		d.SetId("")
	}
	return
}

func resourceImportRepo(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	err := resourceReadRepo(d, meta)
	if err != nil {
		return []*schema.ResourceData{}, err
	}
	return []*schema.ResourceData{d}, nil
}

func setRepoInput(d *schema.ResourceData, meta interface{}) (nodeConfig *config.NodeConfig, err error) {
	meta.(*FlexbotConfig).Sync.Lock()
	defer meta.(*FlexbotConfig).Sync.Unlock()
	nodeConfig = &config.NodeConfig{}
	p := meta.(*FlexbotConfig).FlexbotProvider
	p_storage := p.Get("storage").([]interface{})[0].(map[string]interface{})
	cdotCredentials := p_storage["credentials"].([]interface{})[0].(map[string]interface{})
	nodeConfig.Storage.CdotCredentials.Host = cdotCredentials["host"].(string)
	nodeConfig.Storage.CdotCredentials.User = cdotCredentials["user"].(string)
	nodeConfig.Storage.CdotCredentials.Password = cdotCredentials["password"].(string)
	nodeConfig.Storage.CdotCredentials.ZapiVersion = cdotCredentials["zapi_version"].(string)
	passPhrase := p.Get("pass_phrase").(string)
	if passPhrase == "" {
		if passPhrase, err = machineid.ID(); err != nil {
			return
		}
	}
	if err = config.SetDefaults(nodeConfig, "", "", "", passPhrase); err != nil {
		err = fmt.Errorf("SetDefaults(): failure: %s", err)
	}
	return
}

func setRepoOutput(d *schema.ResourceData, meta interface{}, nodeConfig *config.NodeConfig) (err error) {
	var images, templates []string
	meta.(*FlexbotConfig).Sync.Lock()
	defer meta.(*FlexbotConfig).Sync.Unlock()
	if images, err = ontap.GetRepoImages(nodeConfig); err == nil {
		d.Set("images", images)
		if templates, err = ontap.GetRepoTemplates(nodeConfig); err == nil {
			d.Set("templates", templates)
		}
	}
        return
}
