provider "flexbot" {

  # Password phrase to decrypt passwords in credentials (if encrypted)
  # Machine ID is used by default
  # Use 'flexbot --op=encryptString [--passphrase=<password phrase>]' CLI to generate encrypted passwords values
  pass_phrase = "secret"

  # IPAM is implemented via pluggable providers (required).
  # Only "Infoblox" and "Internal" providers are supported at this time.
  # "Internal" provider expects you to supply "ip" and "fqdn" in network configurations.
  # Define 'provider = "Internal"' if you manage IPAM via terraform provider.
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

  # UCS compute (required)
  compute {
    # Credentials for UCSM
    credentials {
      host = "ucsm.example.com"
      user = "admin"
      password = "secret"
    }
  }

  # cDOT storage (required)
  storage {
    # Credentials either for cDOT cluster or SVM
    # SVM (storage virtual machine) is highly recommended
    # ZAPI version is optional to handle older OnTap
    credentials {
      host = "svm.example.com"
      user = "vsadmin"
      password = "secret"
      zapi_version = "1.160"
    }
  }

  # Rancher API (optional)
  # Rancher API helps with node management in Rancher cluster:
  #  - graceful node removal (cordon/drain);
  #  - graceful node blade specs updates (cordon/drain/uncordon);
  #  - graceful node image/cloud-init updates (cordon/drain/uncordon).
  rancher_api {
    api_url = "https://rancher.example.com"
    token_key = "token-xxx"
    insecure = true
    cluster_id = rancher2_cluster.cluster.id
    drain_input {
      force = true
      delete_local_data = true
      grace_period = 60
      ignore_daemon_sets = true
      timeout = 1800
    }
  }
  
  # Synchronized nodes updates (optional).
  # Highly suggested to enabled it when Rancher API is enabled.
  # Affects the following tasks when enabled:
  #  - node blade specs updates;
  #  - node image/cloud-init updates.
  # If enabled, above nodes updates happen one node at time.
  # Failure in any node update task would stop updates for all others nodes.
  synchronized_updates = true

}

resource "flexbot_server" "k8s-node1" {

  # UCS compute
  compute {
    hostname = "k8s-node1"
    # UCS Service Profile (server) is to be created here
    sp_org = "org-root/org-Kubernetes"
    # Reference to Service Profile Template (SPT)
    sp_template = "org-root/org-Kubernetes/ls-K8S-SubProd-01"
    # Blade spec to find blade (all specs are optional)
    blade_spec {
      # Blade Dn, supports regexp
      #dn = "sys/chassis-4/blade-3"
      #dn = "sys/chassis-9/blade-[0-9]+"
      # Blade model, supports regexp
      model = "UCSB-B200-M3"
      #model = "UCSB-B200-M[45]"
      # Number of CPUs, supports range
      #num_of_cpus = "2"
      # Number of cores, support range
      #num_of_cores = "36"
      # Total memory in MB, supports range
      total_memory = "65536-262144"
    }
    # By default "destroy" will fail if server has power state "on"
    safe_removal = false
    # Wait for SSH accessible (seconds), default is 0 (no wait)
    wait_for_ssh_timeout = 1200
    # SSH user name, required only for consistent snapshosts
    ssh_user = "cloud-user"
    # SSH private key, required only for consistent snapshosts
    ssh_private_key = file("~/.ssh/id_rsa")
  }

  # cDOT storage
  storage {
    # Boot LUN
    boot_lun {
      # Boot LUN size, GB
      size = 20
      # OS image name
      os_image = "rhel-7.7.01-iboot"
    }
    # Seed LUN for cloud-init
    seed_lun {
      # cloud-init template name (see examples/cloud-init in this project)
      # remove directory path if uploaded to template repo
      seed_template = "../../../examples/cloud-init/rhel7-cloud-init.template"
    }
    # Data LUN is optional
    data_lun {
      # Data LUN size, GB
      size = 50
    }
  }

  # Compute network
  network {
    # Generic network (multiple nodes are allowed)
    node {
      # Name should match respective vNIC name in SPT
      name = "eth2"
      # Supply IP here only for Internal provider
      #ip = "192.168.1.25"
      # Supply FQDN here only for Internal provider
      #fqdn = "k8s-node1.example.com"
      # IPAM allocates IP for node interface
      subnet = "192.168.1.0/24"
      gateway = "192.168.1.1"
      # Arguments for node resolver configuration
      dns_server1 = "192.168.1.10"
      dns_server2 = "192.168.4.10"
      dns_domain = "example.com"
    }
    # iSCSI initiator network #1
    iscsi_initiator {
      # Name should match respective iSCSI vNIC name in SPT
      name = "iscsi0"
      # Supply IP here only for Internal provider
      #ip = "192.168.2.25"
      # Supply FQDN here only for Internal provider
      #fqdn = "k8s-node1-i1.example.com"
      # IPAM allocates IP for iSCSI interface
      subnet = "192.168.2.0/24"
    }
    # iSCSI initiator network #2
    iscsi_initiator {
      # Name should match respective iSCSI vNIC name in SPT
      name = "iscsi1"
      # Supply IP here only for Internal provider
      #ip = "192.168.3.25"
      # Supply FQDN here only for Internal provider
      #fqdn = "k8s-node1-i2.example.com"
      # IPAM allocates IP for iSCSI interface
      subnet = "192.168.3.0/24"
    }
  }

  # cDOT storage snapshots to take while server lifecycle management
  # You may want to keep the list empty until server is built
  snapshot: [
    {
      # Snapshot name
      name: "terraform-2020-07-24-17:15",
      # Ensure "fsfreeze" for every filesystem on iSCSI LUN's before taking snapshot
      # Requires ssh_user and ssh_private_key parameters in "compute"
      fsfreeze: true
    }
  ]

  # Cloud Arguments are optional user defined key/value pairs to resolve in cloud-init template
  cloud_args = {
    cloud_user = "cloud-user"
    ssh_pub_key = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAxxxxxxxxxxxxxxxxxxxxxxx"
  }

  # Connection info for provisioners
  connection {
    type = "ssh"
    host = self.network[0].node[0].ip
    user = "cloud-user"
    private_key = file("~/.ssh/id_rsa")
    timeout = "10m"
  }

  # Provisioner to install docker
  provisioner "remote-exec" {
    inline = [
      "curl https://releases.rancher.com/install-docker/19.03.sh | sh",
    ]
  }
}

# Show server IP address
output "ip_address" {
  value = flexbot_server.k8s-node1.network[0].node[0].ip
}

# Show server FQDN
output "fqdn" {
  value = flexbot_server.k8s-node1.network[0].node[0].fqdn
}
