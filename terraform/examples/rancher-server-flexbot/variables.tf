variable "nodes" {
  type = object({
    hosts = list(string)
    compute_blade_spec_dn = list(string)
    compute_blade_spec_model = string
    compute_blade_spec_total_memory = string
    os_image = string
    seed_template = string
    boot_lun_size = number
    data_lun_size = number
  })
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
  type = map(list(object({
    name = string
    subnet = string
    gateway = string
    dns_server1 = string
    dns_server2 = string
    dns_domain = string
  })))
}

variable "snapshots" {
  type = list(object({
    name = string
    fsfreeze = bool
  }))
  default = []
}

variable "zapi_version" {
  type = string
  description = "cDOT ZAPI version"
  default = ""
}

variable "rke_kubernetes_version" {
  type = string
  description = "RKE Kubernetes version"
  default = "v1.18.9-rancher1-1"
}

variable "rancher_helm_repo" {
  type = string
  description = "Rancher Management Server Helm repository URL"
  default = ""
}

variable "rancher_version" {
  type = string
  description = "Rancher Management Server version"
  default = ""
}

variable "docker_version" {
  type = string
  description = "Docker version"
  default = ""
}

variable "tls_secret_manifest" {
  type = string
  description = "Rancher TLS Ingress Secret manifest"
  default = ""
}


variable "pass_phrase" {
  type = string
}

variable "token_key" {
  type = string
}

variable "rancher_api_enabled" {
  type = bool
}

variable "output_path" {
  type = string
  description = "Path to output directory"
}

variable "rancher_server_url" {
  type = string
  description = "Rancher server-url"
}
