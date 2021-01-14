terraform {
  required_providers {
    flexbot = {
      source  = "igor-feoktistov/flexbot"
      version = ">=1.6.2"
    }
    rancher2 = {
      source = "rancher/rancher2"
      version = ">= 1.11.0"
    }
  }
  required_version = ">= 0.13"
}
