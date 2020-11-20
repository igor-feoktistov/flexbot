package main

import (
	"flexbot/terraform/provider"
	"github.com/hashicorp/terraform/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: flexbot.Provider,
	})
}
