# Flexbot to build and manage Rancher Custom Cluster

This example demonstrates how to use `flexbot` and `rancher/rancher2` Terraform providers
to build and manage Rancher Custom Cluster with bare-metal nodes on FlexPOD

See [crypt](../crypt) example on how to generate encrypted strings for passwords and tokens.

To unlock encrypted passwords you can input `pass_phrase` value either via Terraform prompt
or other means like `.auto.tfvars` or ENV variable.
