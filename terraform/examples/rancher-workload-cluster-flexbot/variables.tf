variable "nodes" {
  type = object({
    masters = map(object({
      blade_spec_dn = string
      blade_spec_model = string
      blade_spec_total_memory = string
      os_image = string
      seed_template = string
      boot_lun_size = number
      data_lun_size = number
      restore = object({
        restore = bool
        snapshot_name = string
      })
      snapshots = list(object({
        name = string
        fsfreeze = bool
      }))
    }))
    workers = map(object({
      blade_spec_dn = string
      blade_spec_model = string
      blade_spec_total_memory = string
      os_image = string
      seed_template = string
      boot_lun_size = number
      data_lun_size = number
      restore = object({
        restore = bool
        snapshot_name = string
      })
      snapshots = list(object({
        name = string
        fsfreeze = bool
      }))
    }))
  })
}

variable "flexbot_credentials" {
  type = map(object({
    host = string
    user = string
    password = string
  }))
}

variable "node_config" {
  type = object({
    infoblox = object({
      wapi_version = string
      dns_view = string
      network_view = string
      dns_zone = string
    })
    compute = object({
      sp_org = string
      sp_template = string
      ssh_user = string
      ssh_public_key_path = string
      ssh_private_key_path = string
    })
    network = map(list(object({
      name = string
      subnet = string
      gateway = string
      dns_server1 = string
      dns_server2 = string
      dns_domain = string
    })))
    storage = object({
      zapi_version = string
    })
  })
}

variable "rancher_config" {
  type = object({
    api_url = string
    cluster_name = string
    kubernetes_version = string
  })
}

variable "pass_phrase" {
  type = string
}

variable "token_key" {
  type = string
}

variable "output_path" {
  type = string
  description = "Path to output directory"
}
