terraform {
  required_providers {
    flexbot = {
      source  = "igor-feoktistov/flexbot"
      version = ">=1.4.2"
    }
    rke = {
      source = "rancher/rke"
      version = ">= 1.1.5"
    }
    helm = {
      source = "hashicorp/helm"
      version = ">=1.3.2"
    }
  }
  required_version = ">= 0.13"
}
