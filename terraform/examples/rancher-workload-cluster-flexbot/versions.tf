terraform {
  required_providers {
    flexbot = {
      source  = "igor-feoktistov/flexbot"
      version = ">=1.5.4"
    }
    rancher2 = {
      source = "rancher/rancher2"
      version = ">= 1.10.6"
    }
  }
  required_version = ">= 0.13"
}
