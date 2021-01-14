repo = {
  images = [
    {
      name = "rhel-7.8.01-iboot"
      location = "/diskimage-builder/images/rhel-7.8.01-iboot.raw"
    },
    {
      name = "ubuntu-18.04.05.01-iboot"
      location = "/diskimage-builder/images/ubuntu-18.04.05.01-iboot.raw"
    }
  ]
  templates = [
    {
      name = "rhel7.8.01-cloud-init.template"
      location = "/diskimage-builder/templates/rhel7.8.01-cloud-init.template"
    },
    {
      name = "ubuntu-18.04.05.01-cloud-init.template"
      location = "/diskimage-builder/templates/ubuntu-18.04.05.01-cloud-init.template"
    }
  ]
}

storage_credentials = {
  host = "vserver.example.com"
  user = "vsadmin"
  password = "base64:qiZIN5H04oK15<...skip...>7k4uoBIIg/boi2n3+4kQ="
  zapi_version = ""
}
