variable "hosts" {
  type = list(string)
}

variable "flexbot_credentials" {
  type = map(object({
    host = string
    user = string
    password = string
  }))
}

variable "infoblox_config" {
  type = map
}

variable "node_compute_config" {
  type = map
}

variable "node_network_config" {
  type = map(object({
    if_name = string
    subnet = string
    gateway = string
    dns_server1 = string
    dns_server2 = string
    dns_domain = string
  }))
}

provider "flexbot" {
  ipam {
    provider = "Infoblox"
    credentials {
      host = var.flexbot_credentials.infoblox.host
      user = var.flexbot_credentials.infoblox.user
      password = var.flexbot_credentials.infoblox.password
      wapi_version = var.infoblox_config.wapi_version
      dns_view = var.infoblox_config.dns_view
      network_view = var.infoblox_config.network_view
    }
    dns_zone = var.infoblox_config.dns_zone
  }
  compute {
    credentials {
      host = var.flexbot_credentials.ucsm.host
      user = var.flexbot_credentials.ucsm.user
      password = var.flexbot_credentials.ucsm.password
    }
  }
  storage {
    credentials {
      host = var.flexbot_credentials.cdot.host
      user = var.flexbot_credentials.cdot.user
      password = var.flexbot_credentials.cdot.password
    }
  }
}

# Flexbot hosts
resource "flexbot_server" "host" {
  count = length(var.hosts)
  # UCS compute
  compute {
    hostname = var.hosts[count.index]
    sp_org = var.node_compute_config.sp_org
    sp_template = var.node_compute_config.sp_template
    blade_spec {
      model = var.node_compute_config.blade_spec_model
      total_memory = var.node_compute_config.blade_spec_total_memory
    }
    safe_removal = false
    wait_for_ssh_timeout = 1800
  }
  # cDOT storage
  storage {
    boot_lun {
      size = var.node_compute_config.boot_lun_size
      os_image = var.node_compute_config.os_image
    }
    seed_lun {
      seed_template = var.node_compute_config.seed_template
    }
    data_lun {
      size = var.node_compute_config.data_lun_size
    }
  }
  # Compute network
  network {
    # General use interfaces (list)
    node {
      name = var.node_network_config.node1.if_name
      subnet = var.node_network_config.node1.subnet
      gateway = var.node_network_config.node1.gateway
      dns_server1 = var.node_network_config.node1.dns_server1
      dns_domain = var.node_network_config.node1.dns_domain
    }
    # iSCSI initiator networks (list)
    iscsi_initiator {
      name = var.node_network_config.iscsi1.if_name
      subnet = var.node_network_config.iscsi1.subnet
    }
    iscsi_initiator {
      name = var.node_network_config.iscsi2.if_name
      subnet = var.node_network_config.iscsi2.subnet
    }
  }
  # Arguments for cloud-init template
  cloud_args = {
    cloud_user = "cloud-user"
    ssh_pub_key = file(var.node_compute_config.ssh_public_key_path)
  }
}
