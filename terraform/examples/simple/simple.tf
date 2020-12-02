provider "flexbot" {

  # Optional - password phrase to decrypt passwords in credentials (if encrypted).
  # Machine ID is used by default
  # Use 'flexbot --op=encryptString [--passphrase=<password phrase>]' CLI
  # to generate encrypted passwords values
  pass_phrase = "secret"

  # Required - IPAM is implemented via pluggable providers.
  # Only "Infoblox" and "Internal" providers are supported at this time.
  # "Internal" provider expects you to supply "ip" and "fqdn" in network configuration.
  # Define 'provider = "Internal"' if you manage IPAM via terraform provider.
  ipam {
    provider = "Infoblox"
    # Required - Credentials for Infoblox master
    credentials {
      host = "ib.example.com"
      user = "admin"
      password = "secret"
      wapi_version = "2.5"
      dns_view = "Internal"
      network_view = "default"
    }
    # Required - Compute node FQDN is <hostname>.<dns_zone>
    dns_zone = "example.com"
  }

  # Required - UCS compute
  compute {
    # Required - Credentials for UCSM
    credentials {
      host = "ucsm.example.com"
      user = "admin"
      password = "secret"
    }
  }

  # Required - cDOT storage
  storage {
    # Required - Credentials either for cDOT cluster or SVM
    # SVM (storage virtual machine) is highly recommended.
    credentials {
      host = "svm.example.com"
      user = "vsadmin"
      password = "secret"
      # Optional - ZAPI version to handle older OnTap
      zapi_version = "1.160"
    }
  }

  # Optional - Rancher API
  # Rancher API helps with node management in Rancher cluster:
  #  - graceful node removal (cordon/drain);
  #  - graceful node blade specs updates (cordon/drain/uncordon);
  #  - graceful node image/cloud-init updates (cordon/drain/uncordon).
  rancher_api {
    # Optional (default is false)
    enabled = true
    api_url = "https://rancher.example.com"
    token_key = "token-xxx"
    insecure = true
    cluster_id = rancher2_cluster.cluster.id
    # Optional - Grace timeout after each node update in changing
    #            blade_spec or os_image/seed_template.
    node_grace_timeout = 60
    drain_input {
      force = true
      delete_local_data = true
      grace_period = 60
      ignore_daemon_sets = true
      timeout = 1800
    }
  }

  # Optional - Synchronized nodes updates.
  # It is highly suggested to enable it when Rancher API is enabled.
  # Affects the following tasks when enabled:
  #  - node blade specs updates;
  #  - node image/cloud-init updates.
  # Enforces sequential nodes updates.
  # Failure in any node update task would stop updates for all others nodes.
  synchronized_updates = true

}

resource "flexbot_server" "k8s-node1" {

  # Required - UCS compute
  compute {
    # Required - node name
    hostname = "k8s-node1"
    # Required - UCS Service Profile (server) is to be created here
    sp_org = "org-root/org-Kubernetes"
    # Required - Reference to Service Profile Template (SPT)
    sp_template = "org-root/org-Kubernetes/ls-K8S-SubProd-01"
    # Optional - Blade spec to find blade (all specs are optional)
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
    # Optional - By default "destroy" will fail if server has powerstate "on".
    # It is highly recommended to disable it for Rancher nodes to prevent failures
    # in automated nodes management.
    safe_removal = false
    # Optional - Wait for SSH is accessible (seconds), default is 0 (no wait)
    wait_for_ssh_timeout = 1800
    # Optional - SSH user name.
    # Required for consistent snapshosts and if defined "ssh_node_init_commands".
    # Should match the user defined in cloud-init for image access.
    ssh_user = "cloud-user"
    # Optional - SSH private key. Same as above.
    ssh_private_key = file("~/.ssh/id_rsa")
    # Optional - Brings "provisioner" functionality inside "flexbot_server" resource for better
    # error handling and node updates functionality.
    ssh_node_init_commands = [
      "sudo cloud-init status --wait || true",
      "curl ${data.rancher2_setting.docker_install_url.value} | sh",
      "${rancher2_cluster.cluster.cluster_registration_token[0].node_command} --etcd --controlplane"
    ]
    # Optional - commands to re-size boot disk on host
    ssh_node_bootdisk_resize_commands = ["sudo /usr/sbin/growbootdisk"]
    # Optional - commands to re-size data disk on host
    ssh_node_datadisk_resize_commands = ["sudo /usr/sbin/growdatadisk"]
  }

  # Required - cDOT storage
  storage {
    # Required - Boot LUN
    boot_lun {
      # Required - Boot LUN size, GB
      size = 20
      # Required - OS image name
      os_image = "rhel-7.7.01-iboot"
    }
    # Required - Seed LUN for cloud-init
    seed_lun {
      # Required - cloud-init template name (see examples/cloud-init in this project).
      # Remove directory path if template uploaded to cDOT storage template repository.
      seed_template = "../../../examples/cloud-init/rhel7-cloud-init.template"
    }
    # Optional - Data LUN
    data_lun {
      # Data LUN size, GB
      size = 50
    }
    # Optional - automatically take a snapshot before any image update
    auto_snapshot_on_update = true
  }

  # Required - Compute network
  network {
    # Required - Generic network (multiple nodes are allowed)
    node {
      # Required - Name should match respective vNIC name in Service Profile Template
      name = "eth2"
      # Optional - Supply IP here only for "Internal" provider
      #ip = "192.168.1.25"
      # Optional - Supply FQDN here only for "Internal" provider
      #fqdn = "k8s-node1.example.com"
      # IPAM allocates IP for node interface
      # Required - Subnet in CIDR format for IPAM IP allocation
      subnet = "192.168.1.0/24"
      # Required - default GW IP address
      gateway = "192.168.1.1"
      # Optional - Arguments for node resolver configuration.
      dns_server1 = "192.168.1.10"
      dns_server2 = "192.168.4.10"
      dns_domain = "example.com"
    }
    # Required - iSCSI initiator network #1
    iscsi_initiator {
      # Required - Name should match respective iSCSI vNIC name in Service Profile Template
      name = "iscsi0"
      # Optional - Supply IP here only for "Internal" provider
      #ip = "192.168.2.25"
      # Optional - Supply FQDN here only for "Internal" provider
      #fqdn = "k8s-node1-i1.example.com"
      # IPAM allocates IP for iSCSI interface
      # Required - Subnet in CIDR format for IPAM IP allocation
      subnet = "192.168.2.0/24"
    }
    # Optional, but highly suggested - iSCSI initiator network #2
    iscsi_initiator {
      # required - Name should match respective iSCSI vNIC name in Service Profile Template
      name = "iscsi1"
      # Optionl - Supply IP here only for "Internal" provider
      #ip = "192.168.3.25"
      # Optional - Supply FQDN here only for "Internal" provider
      #fqdn = "k8s-node1-i2.example.com"
      # IPAM allocates IP for iSCSI interface
      # Required - Subnet in CIDR format for IPAM IP allocation
      subnet = "192.168.3.0/24"
    }
  }

  # Optional - cDOT storage snapshots to take while server lifecycle management
  # You may want to keep the list empty until server is built
  snapshot: [
    {
      # Required - Snapshot name
      name: "terraform-2020-07-24-17:15",
      # Optional - Ensures "fsfreeze" for every filesystem on iSCSI LUN's before taking snapshot.
      # Requires ssh_user and ssh_private_key parameters in "compute"
      fsfreeze: true
    }
  ]

  # Optional - Cloud Arguments are user defined key/value pairs to resolve in cloud-init template
  cloud_args = {
    cloud_user = "cloud-user"
    ssh_pub_key = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAxxxxxxxxxxxxxxxxxxxxxxx"
  }

  # Restore from snapshot
  # Optional - restore server LUN's from snapshot.
  restore {
    # Make sure to set "restore=false" once it's completed.
    restore = true
    # Optional - by default it finds latest snapshot created by the provider
    #            if you set auto_snapshot_on_update to true for storage.
    # List of all available snapshots you can find in state (look for snapshosts[]) for the resource.
    snapshot_name = "k8s-node1.snap.1"
  }

  # Optional - Connection info for provisioners
  connection {
    type = "ssh"
    host = self.network[0].node[0].ip
    user = "cloud-user"
    private_key = file("~/.ssh/id_rsa")
    timeout = "10m"
  }

  # Optional - Provisioner to initialize node
  # Same functionality is available via "ssh_node_init_commands" parameter
  # in "compute". For Rancher nodes it is highly suggested to define node
  # initialization via "ssh_node_init_commands" parameter.
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
