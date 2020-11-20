terraform {
  required_providers {
    flexbot = {
      source  = "igor-feoktistov/flexbot"
      version = "1.4.1"
    }
    rancher2 = {
      source = "rancher/rancher2"
      version = ">= 1.10.5"
    }
    rke = {
      source = "rancher/rke"
      version = ">= 1.1.5"
    }
  }
  required_version = ">= 0.13"
}
