Terraform Flexbot Provider
==========================

- Website: https://www.terraform.io
- [![Gitter chat](https://badges.gitter.im/hashicorp-terraform/Lobby.png)](https://gitter.im/hashicorp-terraform/Lobby)
- Mailing list: [Google Groups](http://groups.google.com/group/terraform-tool)

<img src="https://cdn.rawgit.com/hashicorp/terraform-website/master/content/source/assets/images/logo-hashicorp.svg" width="600px">

[Flexbot](https://github.com/igor-feoktistov/flexbot) is a tool to build bare-metal Linux on [FlexPod](https://flexpod.com) (Cisco UCS and NetApp cDOT).

Compared to other bare-metal tools it does not require any additional infrastructure like PXE/DHCP servers.

Requirements
------------

- [Terraform](https://www.terraform.io/downloads.html) 0.13.x
- [Go](https://golang.org/doc/install) 1.14 (to build flexbot CLI and the provider plugin)

Building the provider
---------------------

Clone [flexbot project repository](https://github.com/igor-feoktistov/flexbot).

Build `flexbot` CLI following the instructions in the project README to make sure all dependencies are resolved.

Enter `terraform`  directory and run `make` to build the provider.


Using the provider
------------------
If you want to use the pre-built binaries published on [registry.terraform.io](https://registry.terraform.io/providers/igor-feoktistov/flexbot), just run `terraform init`.
For the provider built from a source code, follow the instructions to [install it as a plugin.](https://www.terraform.io/docs/plugins/basics.html#installing-plugins).
After placing it into your plugins directory, run `terraform init` to initialize it. Please see the examples in `examples` directory.

Community
---------

Join the Flexbot Users [Slack channel](https://join.slack.com/t/flexbot-users/shared_invite/zt-jyhu7gom-K0I4eFvwk2WrUwNrQFc_CA)
