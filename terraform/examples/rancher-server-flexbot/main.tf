locals {
  output_path = var.output_path == "" ? "output" : var.output_path
}

provider "flexbot" {
  alias = "crypt"
  pass_phrase = var.pass_phrase
}

data "flexbot_crypt" "aws_access_key_id" {
  provider = flexbot.crypt
  name = "aws_access_key_id"
  encrypted = var.rke_config.s3_backup.access_key_id
}

data "flexbot_crypt" "aws_secret_access_key" {
  provider = flexbot.crypt
  name = "aws_secret_access_key"
  encrypted = var.rke_config.s3_backup.secret_access_key
}

provider "flexbot" {
  alias = "server"
  pass_phrase = var.pass_phrase
  synchronized_updates = true
  ipam {
    provider = "Infoblox"
    credentials {
      host = var.flexbot_credentials.infoblox.host
      user = var.flexbot_credentials.infoblox.user
      password = var.flexbot_credentials.infoblox.password
      wapi_version = var.node_config.infoblox.wapi_version
      dns_view = var.node_config.infoblox.dns_view
      network_view = var.node_config.infoblox.network_view
    }
    dns_zone = var.node_config.infoblox.dns_zone
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
      zapi_version = var.node_config.storage.zapi_version
    }
  }
}

# RKE nodes
resource "flexbot_server" "node" {
  provider = flexbot.server
  for_each = var.nodes
  # UCS compute
  compute {
    hostname = each.key
    sp_org = var.node_config.compute.sp_org
    sp_template = var.node_config.compute.sp_template
    blade_spec {
      dn = each.value.blade_spec.dn
      model = each.value.blade_spec.model
      total_memory = each.value.blade_spec.total_memory
    }
    safe_removal = false
    wait_for_ssh_timeout = 1800
    ssh_user = var.node_config.compute.ssh_user
    ssh_private_key = file(var.node_config.compute.ssh_private_key_path)
    ssh_node_init_commands = [
      "sudo cloud-init status --wait || true",
      "curl https://releases.rancher.com/install-docker/${var.rke_config.docker_version}.sh | sh",
    ]
    ssh_node_bootdisk_resize_commands = ["sudo /usr/sbin/growbootdisk"]
    ssh_node_datadisk_resize_commands = ["sudo /usr/sbin/growdatadisk"]
  }
  # cDOT storage
  storage {
    auto_snapshot_on_update = true
    boot_lun {
      os_image = each.value.os_image
      size = each.value.boot_lun_size
    }
    seed_lun {
      seed_template = each.value.seed_template
    }
    data_lun {
      size = each.value.data_lun_size
    }
  }
  # Compute network
  network {
    # General use interfaces (list)
    dynamic "node" {
      for_each = [for node in var.node_config.network.node: {
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
      for_each = [for iscsi_initiator in var.node_config.network.iscsi_initiator: {
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
    for_each = [for snapshot in each.value.snapshots: {
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
    cloud_user = var.node_config.compute.ssh_user
    ssh_pub_key = file(var.node_config.compute.ssh_public_key_path)
  }
  # Restore from snapshot
  restore {
    restore = each.value.restore.restore
    snapshot_name = each.value.restore.snapshot_name
  }
}

resource rke_cluster "cluster" {
  kubernetes_version = var.rke_config.rke_version
  dynamic "nodes" {
    for_each = [for instance in flexbot_server.node: {
      ip = instance.network[0].node[0].ip
      fqdn = instance.network[0].node[0].fqdn
    } if var.nodes[instance.compute[0].hostname].rke_member]
    content {
      address = nodes.value.ip
      hostname_override = nodes.value.fqdn
      internal_address = nodes.value.ip
      user = var.node_config.compute.ssh_user
      role = ["controlplane", "worker", "etcd"]
      ssh_key = file(var.node_config.compute.ssh_private_key_path)
    }
  }
  network {
    plugin = "canal"
  }
  services {
    etcd {
      backup_config {
        enabled = true
        interval_hours = 1
        retention = 168
        s3_backup_config {
          region = var.rke_config.s3_backup.region
          endpoint = var.rke_config.s3_backup.endpoint
          access_key = data.flexbot_crypt.aws_access_key_id.decrypted
          secret_key = data.flexbot_crypt.aws_secret_access_key.decrypted
          bucket_name = var.rke_config.s3_backup.bucket_name
          folder = var.rke_config.s3_backup.folder
        }
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
      extra_args = {
        max-pods = "128"
      }
    }
  }
  ingress {
    provider = "nginx"
  }
  upgrade_strategy {
    drain = false
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
  addons_include = [var.rke_config.tls_secret_manifest]
}

resource "local_file" "kubeconfig" {
  directory_permission = "0755"
  file_permission = "0644"
  filename = format("${local.output_path}/kubeconfig")
  content = rke_cluster.cluster.kube_config_yaml
}

resource "local_file" "rkeconfig" {
  directory_permission = "0755"
  file_permission = "0644"
  filename = format("${local.output_path}/rkeconfig.yaml")
  content = rke_cluster.cluster.rke_cluster_yaml
}

provider "helm" {
  kubernetes {
    config_path = format("${local.output_path}/kubeconfig")
  }
}

resource "helm_release" "rancher" {
  depends_on = [rke_cluster.cluster]
  name = "rancher"
  repository = var.rancher_config.rancher_helm_repo
  chart = "rancher"
  version = var.rancher_config.rancher_version
  namespace = "cattle-system"
  create_namespace = "true"
  wait = "true"

  set {
    name = "namespace"
    value = "cattle-system"
  }

  set {
    name = "hostname"
    value = var.rancher_config.rancher_server_url
  }

  set {
    name = "ingress.tls.source"
    value = "secret"
  }

  set {
    name = "useBundledSystemChart"
    value = "true"
  }
}
