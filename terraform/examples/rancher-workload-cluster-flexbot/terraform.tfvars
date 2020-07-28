nodes = {
  masters = {
    hosts = ["node-k8s01","node-k8s02","node-k8s03"]
    #compute_blade_spec_dn = ["sys/chassis-4/blade-7","sys/chassis-5/blade-1","sys/chassis-9/blade-3"]
    compute_blade_spec_model = "UCSB-B200-M5"
    compute_blade_spec_total_memory = "65536-262144"
    os_image = "ubuntu-18.04.01-iboot"
    seed_template = "ubuntu-18.04-cloud-init.template"
    boot_lun_size = 32
    data_lun_size = 64
  }
  workers = {
    hosts = ["node-k8s04","node-k8s05","node-k8s06"]
    #compute_blade_spec_dn = ["sys/chassis-4/blade-8","sys/chassis-5/blade-2","sys/chassis-9/blade-5"]
    compute_blade_spec_model = "UCSB-B200-M5"
    compute_blade_spec_total_memory = "65536-262144"
    os_image = "ubuntu-18.04.01-iboot"
    seed_template = "ubuntu-18.04-cloud-init.template"
    boot_lun_size = 32
    data_lun_size = 128
  }
}

rancher_config = {
  api_url = "https://rancher.example.com"
  rke_template = "rke-flexpod"
  cluster_name = "flexpod-example"
}

flexbot_credentials = {
  infoblox = {
    host = "ib.example.com"
    user = "admin"
    password = "base64:jqdbcMI8dI<...skip...>5DqZg8+cKiZeudV1js2BEAgN4v0Eo="
  }
  ucsm = {
    host = "ucsm.example.com"
    user = "admin"
    password = "base64:kEqDbvk/Dw<...skip...>ABcCHetWJVorS6UIj5H6pA6QTFDOc="
  }
  cdot = {
    host = "vserver.example.com"
    user = "vsadmin"
    password = "base64:qiZIN5H04o<...skip...>K157k4C3ysTuoBIIg/boHi2n3+4kQ="
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
  sp_template = "org-root/org-Kubernetes/ls-K8S-02-SPD-01"
  ssh_user = "cloud-user"
  ssh_public_key_path = "~/.ssh/id_rsa.pub"
  ssh_private_key_path = "~/.ssh/id_rsa"
}

node_network_config = {
  node = [
    {
      name = "eth2"
      subnet = "192.168.1.0/24"
      gateway = "192.168.1.1"
      dns_server1 = "192.168.1.10"
      dns_server2 = ""
      dns_domain = "example.com"
    },
    {
      name = "eth3"
      subnet = "192.168.4.0/24"
      gateway = ""
      dns_server1 = ""
      dns_server2 = ""
      dns_domain = ""
    }
  ]
  iscsi_initiator = [
    {
      name = "iscsi0"
      subnet = "192.168.2.0/24"
      gateway = ""
      dns_server1 = ""
      dns_server2 = ""
      dns_domain = ""
    },
    {
      name = "iscsi1"
      subnet = "192.168.3.0/24"
      gateway = ""
      dns_server1 = ""
      dns_server2 = ""
      dns_domain = ""
    }
  ]
}

snapshots = []

output_path = "output"
token_key = "token-947h6:d7m2s<...skip...>jmjjlw9zgvd9f7slwvnnqw62l4rg"
pass_phrase = "secret"
