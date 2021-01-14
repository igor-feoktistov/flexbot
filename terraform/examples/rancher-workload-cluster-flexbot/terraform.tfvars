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

node_config = {
  infoblox = {
    wapi_version = "2.5"
    dns_view = "Internal"
    network_view = "default"
    dns_zone = "example.com"
  }
  compute = {
    sp_org = "org-root/org-Kubernetes"
    sp_template = "org-root/org-Kubernetes/ls-K8S-01-PROD-01"
    ssh_user = "cloud-user"
    ssh_public_key_path = "~/.ssh/id_rsa.pub"
    ssh_private_key_path = "~/.ssh/id_rsa"
  }
  network = {
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
        subnet = "192.168.2.0/24"
        gateway = ""
        dns_server1 = ""
        dns_server2 = ""
        dns_domain = ""
      }
    ]
    iscsi_initiator = [
      {
        name = "iscsi0"
        subnet = "192.168.3.0/24"
        gateway = ""
        dns_server1 = ""
        dns_server2 = ""
        dns_domain = ""
      },
      {
        name = "iscsi1"
        subnet = "192.168.4.0/24"
        gateway = ""
        dns_server1 = ""
        dns_server2 = ""
        dns_domain = ""
      }
    ]
  }
  storage = {
    zapi_version = ""
  }
}

rancher_config = {
  api_url = "https://rancher.example.com"
  cluster_name = "op-us-west-01-01"
  token_key = "base64:wKIPlAQ5rwsKlvqJjvtiWHeabSOQP<...skip...>MoSLErt2L4JeJdpztfA=="
  kubernetes_version = "v1.19.6-rancher1-1"
  s3_backup = {
    region = "us-east-1"
    endpoint = "s3-accesspoint.us-east-1.amazonaws.com"
    access_key_id = "base64:ZoNJRxrA/lm1Wme5W<...skip...>c9LMgwDgoYYT26fxzySnod9VjBQs"
    secret_access_key = "base64:Gk3HYIMSdFP/k<...skip...>sFgBji3sS+ggQCcUfErFGlCJFiw="
    bucket_name = "op-us-west-01-01-backups"
    folder = "rancher"
  }
}

output_path = "output"
