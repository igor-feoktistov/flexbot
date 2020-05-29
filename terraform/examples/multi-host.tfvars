hosts = ["node01","node02"]

flexbot_credentials = {
  infoblox = {
    host = "ib.example.com"
    user = "admin"
    password = "secret"
  }
  ucsm = {
    host = "ucsm.example.com"
    user = "admin"
    password = "secret"
  }
  cdot = {
    host = "svm.example.com"
    user = "vsadmin"
    password = "secret"
  }
}

infoblox_config = {
  wapi_version = "2.5"
  dns_view = "Internal"
  network_view = "default"
  dns_zone = "example.com"
}

node_compute_config = {
  sp_org = "org-root/org-Kubernetes"
  sp_template = "org-root/org-Kubernetes/ls-K8S-SubProd-01"
  blade_spec_model = "UCSB-B200-M5"
  blade_spec_total_memory = "65536-262144"
  os_image = "ubuntu-18.04-iboot"
  seed_template = "cloud-init/ubuntu-18.04-cloud-init.template"
  boot_lun_size = 20
  data_lun_size = 50
  ssh_public_key_path = "~/.ssh/id_rsa.pub"
  ssh_private_key_path = "~/.ssh/id_rsa"
}

node_network_config = {
  node1 = {
    if_name = "eth2"
    subnet = "192.168.2.0/24"
    gateway = "192.168.2.1"
    dns_server1 = "192.168.2.10"
    dns_server2 = "192.168.5.10"
    dns_domain = "example.com"
  }
  iscsi1 = {
    if_name = "iscsi0"
    subnet = "192.168.3.0/24"
    gateway = ""
    dns_server1 = ""
    dns_server2 = ""
    dns_domain = ""
  }
  iscsi2 = {
    if_name = "iscsi1"
    subnet = "192.168.4.0/24"
    gateway = ""
    dns_server1 = ""
    dns_server2 = ""
    dns_domain = ""
  }
}
