provider "flexbot" {

  # IPAM is implemented via pluggable providers.
  # Only Infoblox and Internal providers are supported at this time.
  # Internal provider expects you to supply "ip" and "fqdn" in network configurations.
  ipam {
    provider = "Infoblox"
    # Credentials for Infoblox master
    credentials {
      host = "ib.example.com"
      user = "admin"
      password = "secret"
      wapi_version = "2.5"
      dns_view = "Internal"
      network_view = "default"
    }
    # Compute node FQDN is <hostname>.<dns_zone>
    dns_zone = "example.com"
  }

  # UCS Service Profile is created from Service Profile Template (SPT)
  compute {
    # Credentials for UCSM
    credentials {
      host = "ucsm.example.com"
      user = "admin"
      password = "secret"
    }
  }

  # cDOT storage
  storage {
    # Credentials either for cDOT cluster or SVM
    # SVM (storage virtual machine) is highly recommended
    credentials {
      host = "svm.example.com"
      user = "vsadmin"
      password = "secret"
    }
  }

}
