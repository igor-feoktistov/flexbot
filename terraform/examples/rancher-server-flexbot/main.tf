locals {
  output_path = var.output_path == "" ? "output" : var.output_path
}

provider "flexbot" {
  pass_phrase = var.pass_phrase
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
    ssh_user = var.node_compute_config.ssh_user
    ssh_private_key = file(var.node_compute_config.ssh_private_key)
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
  # Connection info for provisioners
  connection {
    type = "ssh"
    host = self.network[0].node[0].ip
    user = var.node_compute_config.ssh_user
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
      user = var.node_compute_config.ssh_user
      role = ["controlplane", "worker", "etcd"]
      ssh_key = file(var.node_compute_config.ssh_private_key_path)
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
  version = "1.2.2"
  kubernetes {
    config_path = format("${local.output_path}/kubeconfig")
  }
}

resource "helm_release" "cert-manager" {
  depends_on = [local_file.kubeconfig]
  name = "cert-manager"
  chart = "cert-manager"
  repository = "https://charts.jetstack.io"
  namespace = "cert-manager"
  create_namespace = "true"

  set {
    name = "namespace"
    value = "cert-manager"
  }

  set {
    name = "version"
    value = "v0.15.1"
  }

  set {
    name = "installCRDs"
    value = "true"
  }
}

resource "time_sleep" "wait_for_cert_manager" {
  depends_on = [helm_release.cert-manager]
  create_duration = "30s"
}

resource "helm_release" "rancher" {
  depends_on = [helm_release.cert-manager, time_sleep.wait_for_cert_manager]
  name = "rancher"
  chart = "rancher"
  repository = "https://releases.rancher.com/server-charts/stable"
  namespace = "cattle-system"
  create_namespace = "true"

  set {
    name = "namespace"
    value = "cattle-system"
  }

  set {
    name = "hostname"
    value = var.rancher_server_url
  }

  set {
    name = "ingress.extraAnnotations.nginx\\.ingress\\.kubernetes\\.io/server-alias"
    value = join(" ", formatlist("%s.nip.io", [for instance in flexbot_server.host : instance.network[0].node[0].ip]))
  }
}