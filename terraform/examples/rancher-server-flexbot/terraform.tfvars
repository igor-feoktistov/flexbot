nodes = {
  hosts = ["node-k8s01","node-k8s02","node-k8s03"]
  #compute_blade_spec_dn = ["sys/chassis-4/blade-2","sys/chassis-5/blade-3","sys/chassis-6/blade-4"]
  compute_blade_spec_dn = ["","",""]
  compute_blade_spec_model = "UCSB-B200-M5"
  compute_blade_spec_total_memory = "65536-262144"
  os_image = "ubuntu-18.04.05.01-iboot"
  seed_template = "ubuntu-18.04-cloud-init.template"
  boot_lun_size = 32
  data_lun_size = 64
}

flexbot_credentials = {
  infoblox = {
    host = "ib.example.com"
    user = "admin"
    password = "base64:jqdbcMI8dI5Dq<...skip...>yoskcRz9UUP+gN4v0Eo="
  }
  ucsm = {
    host = "ucsm.example.com"
    user = "admin"
    password = "base64:kEqDbvk/DwABc<...skip...>orS6UIjo21DpA6QTFDOc="
  }
  cdot = {
    host = "vserver.example.com"
    user = "vsadmin"
    password = "base64:qiZIN5H04oK15<...skip...>7k4uoBIIg/boi2n3+4kQ="
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
  sp_template = "org-root/org-Kubernetes/ls-K8S-01-SPD-0"
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

rke_kubernetes_version = "v1.19.3-rancher1-2"

rancher_helm_repo = "https://releases.rancher.com/server-charts/latest"

tls_secret_manifest = "manifests/tls-rancher-ingress.yaml"

docker_version = "19.03"

rancher_version = "2.5.2"

rancher_server_url = "rancher.example.com"

rancher_api_enabled = false

output_path = "output"

pass_phrase = "secret"

token_key = "token-jzzwn:f06lm59d27q5sdhd<...skip...>erhxgz4kdbvflvn06rsvq284h"
