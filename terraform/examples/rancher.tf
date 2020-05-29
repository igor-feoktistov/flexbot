variable "nodes" {
  type = map(object({
    hosts = list(string)
    compute_blade_spec_model = string
    compute_blade_spec_total_memory = string
    os_image = string
    seed_template = string
    boot_lun_size = number
    data_lun_size = number
  }))
}

variable "rancher_config" {
  type = map
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

provider "rancher2" {
  api_url = var.rancher_config.api_url
  token_key = var.rancher_config.token_key
  insecure = true
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

data "rancher2_cluster_template" "template" {
  name = var.rancher_config.rke_template
}

data "rancher2_setting" "docker_install_url" {
  name = "engine-install-url"
}

resource "rancher2_cluster" "cluster" {
  name = "flexbot"
  cluster_template_id = data.rancher2_cluster_template.template.id
  cluster_template_revision_id = data.rancher2_cluster_template.template.default_revision_id
}

# Master nodes
resource "flexbot_server" "master" {
  count = length(var.nodes.masters.hosts)
  # UCS compute
  compute {
    hostname = var.nodes.masters.hosts[count.index]
    sp_org = var.node_compute_config.sp_org
    sp_template = var.node_compute_config.sp_template
    blade_spec {
      model = var.nodes.masters.compute_blade_spec_model
      total_memory = var.nodes.masters.compute_blade_spec_total_memory
    }
    safe_removal = false
    wait_for_ssh_timeout = 1800
  }
  # cDOT storage
  storage {
    boot_lun {
      size = var.nodes.masters.boot_lun_size
      os_image = var.nodes.masters.os_image
    }
    seed_lun {
      seed_template = var.nodes.masters.seed_template
    }
    data_lun {
      size = var.nodes.masters.data_lun_size
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
  # Connection info for provisioners
  connection {
    type = "ssh"
    host = self.network[0].node[0].ip
    user = "cloud-user"
    private_key = file(var.node_compute_config.ssh_private_key_path)
    timeout = "10m"
  }
  # Provisioner to install docker
  provisioner "remote-exec" {
    inline = [
      "curl ${data.rancher2_setting.docker_install_url.value} | sh > /dev/null 2>&1",
    ]
  }
  # Provisioner to register to the cluster
  provisioner "remote-exec" {
    inline = [
      "${rancher2_cluster.cluster.cluster_registration_token[0].node_command} --etcd --controlplane > /dev/null 2>&1",
    ]
  }
}

# Worker nodes
resource "flexbot_server" "worker" {
  count = length(var.nodes.workers.hosts)
  # UCS compute
  compute {
    hostname = var.nodes.workers.hosts[count.index]
    sp_org = var.node_compute_config.sp_org
    sp_template = var.node_compute_config.sp_template
    blade_spec {
      model = var.nodes.workers.compute_blade_spec_model
      total_memory = var.nodes.workers.compute_blade_spec_total_memory
    }
    safe_removal = false
    wait_for_ssh_timeout = 1800
  }
  # cDOT storage
  storage {
    boot_lun {
      size = var.nodes.workers.boot_lun_size
      os_image = var.nodes.workers.os_image
    }
    seed_lun {
      seed_template = var.nodes.workers.seed_template
    }
    data_lun {
      size = var.nodes.workers.data_lun_size
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
  # Connection info for provisioners
  connection {
    type = "ssh"
    host = self.network[0].node[0].ip
    user = "cloud-user"
    private_key = file(var.node_compute_config.ssh_private_key_path)
    timeout = "10m"
  }
  # Provisioner to install docker
  provisioner "remote-exec" {
    inline = [
      "curl ${data.rancher2_setting.docker_install_url.value} | sh > /dev/null 2>&1",
    ]
  }
  # Provisioner to register to the cluster
  provisioner "remote-exec" {
    inline = [
      "${rancher2_cluster.cluster.cluster_registration_token[0].node_command} --worker > /dev/null 2>&1",
    ]
  }
}
