nodes = {
  hosts = ["nodek8s01", "nodek8s02", "nodek8s03"]
  compute_blade_spec_dn = ["", "", ""]
  compute_blade_spec_model = "UCSB-B200-M5"
  compute_blade_spec_total_memory = "65536-262144"
  os_image = "rhel-7.8.01-iboot"
  seed_template = "cloud-init/rhel7-cloud-init.template"
  boot_lun_size = 24
  data_lun_size = 128
}

flexbot_credentials = {
  infoblox = {
    host = "ib.example.com"
    user = "admin"
    password = "base64:hROs<...trimmed...>asdvfwerferf="
  }
  ucsm = {
    host = "ucsm.example.com"
    user = "admin"
    password = "base64:SdfU<...trimmed...>zidfvdgbgKhg="
  }
  cdot = {
    host = "svm.example.com"
    user = "vsadmin"
    password = "base64:SFgi<...trimmed...>dgajGeKjsYGb="
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
  ssh_public_key_path = "~/.ssh/id_rsa.pub"
  ssh_private_key_path = "~/.ssh/id_rsa"
}

node_network_config = {
  node1 = {
    if_name = "eth2"
    subnet = "192.168.1.0/24"
    gateway = "192.168.1.1"
    dns_server1 = "192.168.5.10"
    dns_server2 = ""
    dns_domain = "example.com"
  }
  iscsi1 = {
    if_name = "iscsi0"
    subnet = "192.168.2.0/24"
    gateway = ""
    dns_server1 = ""
    dns_server2 = ""
    dns_domain = ""
  }
  iscsi2 = {
    if_name = "iscsi1"
    subnet = "192.168.3.0/24"
    gateway = ""
    dns_server1 = ""
    dns_server2 = ""
    dns_domain = ""
  }
}
