# Flexbot to build and manage Rancher Management Server

This example demonstrates how to use `flexbot`, `rancher/rke`, and `helm` Terraform providers
to build and manage Rancher Management Server on RKE cluster with bare-metal nodes on FlexPOD.

See [crypt](../crypt) example on how to generate encrypted strings for passwords and tokens.

To unlock encrypted passwords you can input `pass_phrase` value either via Terraform prompt
or other means like `.auto.tfvars` or ENV variable.
