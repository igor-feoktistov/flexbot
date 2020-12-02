locals {
  output_path = var.output_path == "" ? "output" : var.output_path
}


provider "rancher2" {
  api_url = var.rancher_config.api_url
  token_key = var.token_key
  insecure = true
}

data "rancher2_setting" "docker_install_url" {
  name = "engine-install-url"
}

resource "rancher2_cluster" "cluster" {
  name = var.rancher_config.cluster_name
  rke_config {
    kubernetes_version = var.rancher_config.kubernetes_version
    network {
      plugin = "canal"
    }
    services {
      etcd {
        creation = "6h"
        retention = "24h"
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
        extra_args = {
          max-pods = "500"
        }
      }
    }
    ingress {
      provider = "nginx"
    }
    upgrade_strategy {
      drain = true
      drain_input {
        force = true
        delete_local_data = true
        grace_period = 60
        ignore_daemon_sets = true
        timeout = 1800
      }
      max_unavailable_controlplane = "1"
      max_unavailable_worker = "1"
    }
    ssh_key_path = var.node_compute_config.ssh_private_key_path
  }
}

provider "flexbot" {
  pass_phrase = var.pass_phrase
  synchronized_updates = true
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
      zapi_version = var.zapi_version
    }
  }
  rancher_api {
    enabled = true
    api_url = var.rancher_config.api_url
    token_key = var.token_key
    insecure = true
    cluster_id = rancher2_cluster.cluster.id
    drain_input {
      force = true
      delete_local_data = true
      grace_period = 60
      ignore_daemon_sets = true
      timeout = 1800
    }
  }
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
      dn = var.nodes.masters.compute_blade_spec_dn[count.index]
      model = var.nodes.masters.compute_blade_spec_model
      total_memory = var.nodes.masters.compute_blade_spec_total_memory
    }
    safe_removal = false
    wait_for_ssh_timeout = 1800
    ssh_user = var.node_compute_config.ssh_user
    ssh_private_key = file(var.node_compute_config.ssh_private_key_path)
    ssh_node_init_commands = [
      "sudo cloud-init status --wait || true",
      "curl ${data.rancher2_setting.docker_install_url.value} | sh",
      "${rancher2_cluster.cluster.cluster_registration_token[0].node_command} --etcd --controlplane",
    ]
    ssh_node_bootdisk_resize_commands = ["sudo /usr/sbin/growbootdisk"]
    ssh_node_datadisk_resize_commands = ["sudo /usr/sbin/growdatadisk"]
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
    dynamic "node" {
      for_each = [for node in var.node_network_config.node: {
        name = node.name
        subnet = node.subnet
        gateway = node.gateway
        dns_server1 = node.dns_server1
        dns_server2 = node.dns_server2
        dns_domain = node.dns_domain
      }]
      content {
        name = node.value.name
        subnet = node.value.subnet
        gateway = node.value.gateway
        dns_server1 = node.value.dns_server1
        dns_server2 = node.value.dns_server2
        dns_domain = node.value.dns_domain
      }
    }
    # iSCSI initiator networks (list)
    dynamic "iscsi_initiator" {
      for_each = [for iscsi_initiator in var.node_network_config.iscsi_initiator: {
        name = iscsi_initiator.name
        subnet = iscsi_initiator.subnet
      }]
      content {
        name = iscsi_initiator.value.name
        subnet = iscsi_initiator.value.subnet
      }
    }
  }
  # Storage snapshots
  dynamic "snapshot" {
    for_each = [for snapshot in var.snapshots: {
      name = snapshot.name
      fsfreeze = snapshot.fsfreeze
    }]
    content {
      name = snapshot.value.name
      fsfreeze = snapshot.value.fsfreeze
    }
  }
  # Arguments for cloud-init template
  cloud_args = {
    cloud_user = var.node_compute_config.ssh_user
    ssh_pub_key = file(var.node_compute_config.ssh_public_key_path)
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
      dn = var.nodes.workers.compute_blade_spec_dn[count.index]
      model = var.nodes.workers.compute_blade_spec_model
      total_memory = var.nodes.workers.compute_blade_spec_total_memory
    }
    safe_removal = false
    wait_for_ssh_timeout = 1800
    ssh_user = var.node_compute_config.ssh_user
    ssh_private_key = file(var.node_compute_config.ssh_private_key_path)
    ssh_node_init_commands = [
      "sudo cloud-init status --wait || true",
      "curl ${data.rancher2_setting.docker_install_url.value} | sh",
      "${rancher2_cluster.cluster.cluster_registration_token[0].node_command} --worker",
    ]
    ssh_node_bootdisk_resize_commands = ["sudo /usr/sbin/growbootdisk"]
    ssh_node_datadisk_resize_commands = ["sudo /usr/sbin/growdatadisk"]
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
    dynamic "node" {
      for_each = [for node in var.node_network_config.node: {
        name = node.name
        subnet = node.subnet
        gateway = node.gateway
        dns_server1 = node.dns_server1
        dns_server2 = node.dns_server2
        dns_domain = node.dns_domain
      }]
      content {
        name = node.value.name
        subnet = node.value.subnet
        gateway = node.value.gateway
        dns_server1 = node.value.dns_server1
        dns_server2 = node.value.dns_server2
        dns_domain = node.value.dns_domain
      }
    }
    # iSCSI initiator networks (list)
    dynamic "iscsi_initiator" {
      for_each = [for iscsi_initiator in var.node_network_config.iscsi_initiator: {
        name = iscsi_initiator.name
        subnet = iscsi_initiator.subnet
      }]
      content {
        name = iscsi_initiator.value.name
        subnet = iscsi_initiator.value.subnet
      }
    }
  }
  # Storage snapshots
  dynamic "snapshot" {
    for_each = [for snapshot in var.snapshots: {
      name = snapshot.name
      fsfreeze = snapshot.fsfreeze
    }]
    content {
      name = snapshot.value.name
      fsfreeze = snapshot.value.fsfreeze
    }
  }
  # Arguments for cloud-init template
  cloud_args = {
    cloud_user = var.node_compute_config.ssh_user
    ssh_pub_key = file(var.node_compute_config.ssh_public_key_path)
  }
}

resource "local_file" "kubeconfig" {
  directory_permission = "0755"
  file_permission = "0644"
  filename = format("${local.output_path}/kubeconfig")
  content  = rancher2_cluster.cluster.kube_config
}
