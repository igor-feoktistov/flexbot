# Flexbot Configuration Examples

Here you can find the examples that may help you to jumpstart with `flexbot` provider for different use cases.
Make sure to update respective `.tfvars` files with your own infrastructure configuration settings.

## Examples

* [simple](./simple.tf) Simple configuration with a lot of comments.
* [multi-host](./multi-host.tf) Provisions multiple servers the same configuration in one shot.
* [rancher.tf](./rancher.tf) Provisions Rancher workload cluster.

### Note
You can easily adapt the examples with IPAM provider via Terraform.
In `flexbot` provider confguration use the following `ipam` definition to disable built-in provider:
```
  ipam {
    provider = "Internal"
  }
```
Then you need to supply `ip` and `fqdn` in resource network for `node` and `iscsi-initiator`.
