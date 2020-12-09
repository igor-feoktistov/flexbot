terraform {
  required_providers {
    flexbot = {
      source  = "igor-feoktistov/flexbot"
      version = ">=1.5.3"
    }
    rke = {
      source = "rancher/rke"
      version = ">= 1.1.5"
    }
  }
  required_version = ">= 0.13"
}
