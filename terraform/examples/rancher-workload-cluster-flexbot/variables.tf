variable "nodes" {
  type = object({
    masters = map(object({
      blade_spec = object({
        dn = string
        model = string
        total_memory = string
      })
      powerstate = string
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
      blade_spec = object({
        dn = string
        model = string
        total_memory = string
      })
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
  type = object({
    infoblox = object({
      host = string
      user = string
      password = string
    })
    ucsm = object({
      host = string
      user = string
      password = string
    })
    cdot = object({
      host = string
      user = string
      password = string
    })
  })
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
    token_key = string
    cluster_name = string
    kubernetes_version = string
    s3_backup = object({
      region = string
      endpoint = string
      access_key_id = string
      secret_access_key = string
      bucket_name = string
      folder = string
    })
  })
}

variable "pass_phrase" {
  type = string
}

variable "output_path" {
  type = string
  description = "Path to output directory"
}
