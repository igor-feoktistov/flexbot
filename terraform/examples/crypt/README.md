# Flexbot crypt datasource example

This example demonstrates how to generate encrypted strings for passwords, token keys, access keys, and others.
Terraform state file wil have encrypted strings in attributes with name `encrypted`.

Crypt package uses AES encryption with 256-bit keys generated via SHA256 sum.
