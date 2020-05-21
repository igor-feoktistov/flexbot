Terraform Flexbot Provider
==========================

Requirements
------------

- [Terraform](https://www.terraform.io/downloads.html) 0.12.x
- [Go](https://golang.org/doc/install) 1.14 (to build flexbot CLI and the provider plugin)

Building the provider
---------------------

Clone `flexbot` project repository.

Build `flexbot` CLI following the instructions in the project README to make sure all dependencies are resolved.

Enter `terraform`  directory and run `make` to build the provider.


Using the provider
------------------
Once you built the provider, follow the instructions to [install it as a plugin.](https://www.terraform.io/docs/plugins/basics.html#installing-plugins).
After placing it into your plugins directory, run `terraform init` to initialize it.
Please see the examples in `examples` directory.
