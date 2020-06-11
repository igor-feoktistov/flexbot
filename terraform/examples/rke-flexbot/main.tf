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
  count = length(var.nodes.hosts)
  # UCS compute
  compute {
    hostname = var.nodes.hosts[count.index]
    sp_org = var.node_compute_config.sp_org
    sp_template = var.node_compute_config.sp_template
    blade_spec {
      dn = var.nodes.compute_blade_spec_dn[count.index]
      model = var.nodes.compute_blade_spec_model
      total_memory = var.nodes.compute_blade_spec_total_memory
    }
    safe_removal = false
    wait_for_ssh_timeout = 1800
  }
  # cDOT storage
  storage {
    boot_lun {
      size = var.nodes.boot_lun_size
      os_image = var.nodes.os_image
    }
    seed_lun {
      seed_template = var.nodes.seed_template
    }
    data_lun {
      size = var.nodes.data_lun_size
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
      "curl https://releases.rancher.com/install-docker/19.03.sh | sh > /dev/null 2>&1",
    ]
  }

}

resource rke_cluster "cluster" {
  dynamic "nodes" {
    for_each = [for instance in flexbot_server.host: {
      ip = instance.network[0].node[0].ip
      fqdn = instance.network[0].node[0].fqdn
    }]
    content {
      address = nodes.value.ip
      hostname_override = nodes.value.fqdn
      internal_address = nodes.value.ip
      user    = "cloud-user"
      role    = ["controlplane", "worker", "etcd"]
      ssh_key = file("~/.ssh/id_rsa")
    }
  }
  services {
    etcd {
      backup_config {
        interval_hours = 12
        retention = 6
      }
    }
    kube_api {
      service_cluster_ip_range = "172.20.0.0/16"
      service_node_port_range = "30000-32767"
      pod_security_policy = false
    }
    kube_controller {
      cluster_cidr = "172.30.0.0/16"
      service_cluster_ip_range = "172.20.0.0/16"
    }
    kubelet {
      cluster_domain = "cluster.local"
      cluster_dns_server = "172.20.0.10"
    }
  }
  ingress {
    provider = "nginx"
  }
  addons_include = var.cluster_config.addons_include
}
