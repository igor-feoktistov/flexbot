# Flexbot to build and manage RKE cluster

This example demonstrates how to use `flexbot` and `rancher/rke` Terraform providers
to build and manage RKE cluster with bare-metal nodes on FlexPOD.

See [crypt](../crypt) example on how to generate encrypted strings for passwords and tokens.

To unlock encrypted passwords you can input `pass_phrase` value either via Terraform prompt
or other means like `.auto.tfvars` or ENV variable.
