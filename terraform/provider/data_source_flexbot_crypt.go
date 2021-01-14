package flexbot

import (
	"fmt"
	"encoding/base64"

	"flexbot/pkg/util/crypt"
	"github.com/denisbrodbeck/machineid"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func dataSourceFelxbotCrypt() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceFlexbotCryptRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Entity string name",
			},
			"encrypted": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Encrypted string encoded in base64 format",
			},
			"decrypted": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Sensitive:   true,
				Description: "Decrypted sting in plain text",
			},
		},
	}
}

func dataSourceFlexbotCryptRead(d *schema.ResourceData, meta interface{}) (err error) {
	var b, b64 []byte
	passPhrase := meta.(*FlexbotConfig).FlexbotProvider.Get("pass_phrase").(string)
	if len(passPhrase) == 0 {
		if passPhrase, err = machineid.ID(); err != nil {
			err = fmt.Errorf("dataSourceFlexbotCryptRead(): machineid.ID() failure: %s", err)
			return
		}
	}
	name := d.Get("name").(string)
	encrypted := d.Get("encrypted").(string)
	decrypted := d.Get("decrypted").(string)
	if len(encrypted) == 0 && len(decrypted) > 0 {
		if b, err = crypt.Encrypt([]byte(decrypted), passPhrase); err != nil {
			err = fmt.Errorf("dataSourceFlexbotCryptRead(): crypt.Encrypt() error: %s", err)
			return
		}
		d.Set("encrypted", "base64:" + base64.StdEncoding.EncodeToString(b))
	}
	if len(decrypted) == 0 && len(encrypted) > 0 {
		if b64, err = base64.StdEncoding.DecodeString(encrypted[7:]); err != nil {
			err = fmt.Errorf("dataSourceFlexbotCryptRead(): base64.StdEncoding.DecodeString() failure: %s", err)
                        return
		}
		if b, err = crypt.Decrypt(b64, passPhrase); err != nil {
			err = fmt.Errorf("dataSourceFlexbotCryptRead(): crypt.Decrypt() error: %s", err)
			return
		}
		d.Set("decrypted", string(b))
	}
	d.SetId(name)
	return
}
