terraform {
  required_providers {
    flexbot = {
      source  = "igor-feoktistov/flexbot"
      version = ">=1.6.2"
    }
    rke = {
      source = "rancher/rke"
      version = ">= 1.1.5"
    }
  }
  required_version = ">= 0.13"
}
