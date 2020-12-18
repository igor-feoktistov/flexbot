package flexbot

import (
        "github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func schemaFlexbotRepo() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"images": {
			Type:     schema.TypeList,
			Optional: true,
			Computed: true,
			Elem:     &schema.Schema{Type: schema.TypeString},
		},
		"templates": {
			Type:     schema.TypeList,
			Optional: true,
			Computed: true,
			Elem:     &schema.Schema{Type: schema.TypeString},
		},
		"image_repo": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"name": {
						Type:     schema.TypeString,
						Required: true,
					},
					"location": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
		},
		"template_repo": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"name": {
						Type:     schema.TypeString,
						Required: true,
					},
					"location": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
		},
	}
}
