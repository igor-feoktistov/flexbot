# flexbot is a tool to build bare-metal Linux on FlexPod (Cisco UCS and NetApp cDOT)

Compared to other bare-metal tools it does not require any additional infrastructure like PXE/DHCP servers.

# Documentation

## Usage

 - Provision server:
   ```flexbot --config=<config file path> --op=provisionServer --host=<host node name> --image=<image name> --template=<cloud-init template path>```

 - De-provision server:
   ```flexbot --config=<config file path> --op=deprovisionServer --host=<host node name>```

 - Power Off server:
   ```flexbot --config=<config file path> --op=stopServer --host=<host node name>```

 - Power On server:
   ```flexbot --config=<config file path> --op=startServer --host=<host node name>```

 - Encrypt passwords in configuration:
   ```flexbot --config=<config file path> --op=encryptConfig```

 - Decrypt passwords in configuration:
   ```flexbot --config=<config file path> --op=decryptConfig```

 - Upload image into image repository:
   ```flexbot --config=<config file path> --op=uploadImage --image=<image name> --imagePath=<image path>```

 - Delete image from image repository:
   ```flexbot --config=<config file path> --op=deleteImage --image=<image name>```

 - List images in image repository:
   ```flexbot --config=<config file path> --op=listImages```

## Runtime arguments

  - config: `a path to configuration file, STDIN, or argument value in JSON (default "STDIN")`
  - dumpResult: `file path or STDOUT (default "STDOUT")`
  - encodingFormat: `supported encoding formats: json, yaml (default "yaml")`
  - host: `compute node name`
  - image: `boot image name`
  - imagePath: `a path to boot image (prefix can be either file:// or http(s)://)`
  - op: `provisionServer, deprovisionServer, stopServer, startServer, uploadImage, listImages, encryptConfig, decryptConfig`
  - passphrase: `passphrase to encrypt/decrypt passwords in configuration (default is machineid)`
  - template: `a path to cloud-init template (prefix can be either file:// or http(s)://)`

## Passwords Encryption

Your host `machineid` is a default encryption key if you choose to encrypt passwords.

## Configuration

Configuration can be provided either in YAML or JSON format.

```
# IPAM is implemented via pluggable providers.
# Only Infoblox and Internal providers are supported at this time.
# Internal provider expects you to supply "ip" and "fqdn" in network configuration.
ipam:
    provider: Infoblox
    # Credentials for Infoblox master
    ibCredentials:
        host: 10.253.4.10
        user: admin
        # if you choose to encrypt passwords, should start from "base64:" prefix
        password: secret
        wapiVersion: "2.5"
        dnsView: Internal
        networkView: default
    # Compute node FQDN is <node name>.<dnsZone>
    dnsZone: corp.fslab.ntap
# UCS Service Profile is created from Service Profile Template (SPT)
compute:
    # Credentials for UCSM
    ucsmCredentials:
        host: ucsm99.mgmt.fslab.ntap
        user: admin
        password: secret
    # UCS Service Profile (server) is to be created here
    spOrg: org-root/org-Kubernetes
    # Reference to Service Profile Template (SPT)
    spTemplate: org-root/org-Kubernetes/ls-K8S-M3160-SPD-01
    # Blade search is conducted by applying "AND" rule to all provided specs
    bladeSpec:
        # "dn" is optional, supports regexp
        #dn: sys/chassis-1/blade-2
        #dn: sys/chassis-9/blade-[0-9]+
        # "model" is optional, supports regexp
        model: UCSB-B200-M4
        #model: UCSB-B200-M[45]
        # "numOfCpus" is optional, supports ranges
        numOfCpus: "2"
        # "numOfCores" is optional, support ranges
        numOfCores: "36"
        # "totalMemory" in MB is optional, supports ranges
        totalMemory: "262144"
        #totalMemory: "262144-393216"
storage:
    # Credentials either for cDOT cluster or SVM
    # SVM (storage virtual machine) is highly recommended
    cdotCredentials:
        host: svmdurlabnks03spd.corp.fslab.ntap
        user: vsadmin
        password: secret
    # not required if SVM is in cdotCredentials
    #svmName: svmdurlabnks03spd
    # Boot LUN
    bootLun:
        # boot LUN size in GB
        size: 20
    # Data LUN (optional)
    dataLun:
        # data LUN size in GB
        size: 50
    # Seed LUN (optional)
    seedLun:
        # optionally you can pass seedTemplate location here
        seedTemplate:
          # see "template" runtime argument
          location: templates/ubuntu-18.04-cloud-init.template
network:
    # Node network interfaces (list)
    node:
        # name should match respective vNIC name in SPT
      - name: eth2
        # Supply IP here only for Internal provider
        ip: 10.253.20.52
        # Supply FQDN here only for Internal provider
        fqdn: phyk8s01.corp.fslab.ntap
        # IPAM allocates IP for node interface
        subnet: 10.253.20.0/24
        gateway: 10.253.20.1
        # arguments for node resolver configuration
        dnsServer1: 10.253.4.10
        # "dnsServer2" is optional
        dnsServer2: 10.253.4.11
        dnsDomain: corp.fslab.ntap
    # iSCSI initiator network interfaces (list)
    # Minimum one interface required, two is highly recommended
    iscsiInitiator:
        # name should match respective iSCSI vNIC name in SPT
      - name: iscsi0
        # Supply IP here only for Internal provider
        ip: 10.253.18.80
        # Supply FQDN here only for Internal provider
        fqdn: phyk8s01-i1.corp.fslab.ntap
        # IPAM allocates IP for iSCSI interface
        subnet: 10.253.18.0/24
        # name should match respective iSCSI vNIC name in SPT
      - name: iscsi1
        # Supply IP here only for Internal provider
        ip: 10.253.19.78
        # Supply FQDN here only for Internal provider
        fqdn: phyk8s01-i2.corp.fslab.ntap
        # IPAM allocates IP for iSCSI interface
        subnet: 10.253.19.0/24
cloudArgs:
    # optional user defined key/value pairs to address in cloud-init templates
    cloud_user: cloud-user
    ssh_pub_key: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC9W8<trimmed>"
```

## Command Output

Command output (either in YAML or JSON format) will include submitted configuration plus discovered
while build host and storage entities. This is an example output from successful run:
```
# status is either "success" or "failure"
# "errorMessage" will be added in case of "failure"
status: success
server:
    ipam:
        provider: Infoblox
        dnsZone: corp.fslab.ntap
    compute:
        hostName: phygymk8s01-lab
        spOrg: org-root/org-Kubernetes
        spTemplate: org-root/org-Kubernetes/ls-K8S-M3160-SPD-01
        spDn: org-root/org-Kubernetes/ls-phygymk8s01-lab
        bladeSpec:
            dn: sys/chassis-1/blade-5
            model: UCSB-B200-M4
            numOfCpus: 2
            numOfCores: 36
            totalMemory: 65536
    storage:
        svmName: svmdurlabnks03spd
        imageRepoName: image_repo
        volumeName: phygymk8s01_lab_iboot
        igroupName: phygymk8s01_lab_iboot
        bootLun:
            name: phygymk8s01_lab_iboot
            size: 20
            osImage:
                name: ubuntu-18.04-iboot
        dataLun:
            name: phygymk8s01_lab_data
            id: 1
            size: 50
        seedLun:
            name: phygymk8s01_lab_seed
            id: 2
            seedTemplate:
                location: templates/ubuntu-18.04-cloud-init.template
    network:
        node:
          - name: eth2
            macaddr: 00:25:B5:99:04:BF
            ip: 10.253.20.52
            fqdn: phyk8s01.corp.fslab.ntap
            subnet: 10.253.20.0/24
            netlen: "24"
            gateway: 10.253.20.1
            dnsServer1: 10.253.4.10
            dnsDomain: corp.fslab.ntap
        iscsiInitiator:
          - name: iscsi0
            ip: 10.253.18.80
            fqdn: phyk8s01-i1.corp.fslab.ntap
            subnet: 10.253.18.0/24
            netlen: "24"
            gateway: 0.0.0.0
            dnsServer1: 0.0.0.0
            dnsServer2: 0.0.0.0
            initiatorName: iqn.2005-02.com.open-iscsi:phygymk8s01-lab.1
            iscsiTarget:
                nodeName: iqn.1992-08.com.netapp:sn.cfe29c87000211eabab300a098ae4dc7:vs.32
                interfaces:
                  - 10.253.18.58
                  - 10.253.18.57
          - name: iscsi1
            ip: 10.253.19.78
            fqdn: phyk8s01-i2.corp.fslab.ntap
            subnet: 10.253.19.0/24
            netlen: "24"
            gateway: 0.0.0.0
            dnsServer1: 0.0.0.0
            dnsServer2: 0.0.0.0
            initiatorName: iqn.2005-02.com.open-iscsi:phygymk8s01-lab.2
            iscsiTarget:
                nodeName: iqn.1992-08.com.netapp:sn.cfe29c87000211eabab300a098ae4dc7:vs.32
                interfaces:
                  - 10.253.19.58
                  - 10.253.19.57
```

## cDOT Storage Requirements

####iSCSI LIF's
Minimal one iSCSI LIF is required. Two or four LIF's in two VLAN ID's are highly recommended.
Non-routed iSCSI is highly recommeneded, meaning interfaces for host iSCSI initiator and SVM
iSCSI LIF's should belong to the same VLAN's.

####NAS LIF's
At least one NFS LIF is required for "uploadImage" functionality.
Once all images are uploaded, NAS LIF can be disabled or removed.
Server build functionality does not require this LIF.

## UCS Requirements

UCS Service Profile Template should be configured to support iSCSI boot.

See below screenshots with configuration details.

###iSCSI vNIC's

<p align="center">
    <img src="https://github.com/igor-feoktistov/flexbot/blob/master/docs/images/SPT-details1.png">
</p>

###Boot Order

<p align="center">
    <img src="https://github.com/igor-feoktistov/flexbot/blob/master/docs/images/SPT-details2.png">
</p>

###iSCSI Boot Parameters

<p align="center">
    <img src="https://github.com/igor-feoktistov/flexbot/blob/master/docs/images/SPT-details3.png">
</p>

## Images

There is no requirements on which tool to use to create images.
Though I highly recommend Diskimage-builder from OpenStack project `https://docs.openstack.org/diskimage-builder/latest/`.
See `examples/diskimage-builder/ubuntu-18.04-iboot.sh` on how to build iSCSI booted ubuntu-18.04.
To upload image use `uploadImage` operation. Make sure to run it in local to your FlexPod network.
The tool is using NFS protocol to upload images. There is no requirements for NFS client.
NFS client support in the tool is built-in.

## Cloud-init templates

Cloud-init templates is GoLang templates with passed from `configuration` parameters.
See `examples/cloud-init/ubuntu-18.04-cloud-init.template` as an example for ubuntu-18.04.

## Installation

`flexbot` requires GoLang v1.14 or higher

Before you run `make` command, make sure to initialize and build dependencies:
```
cd $GOPATH/src/flexbot
go mod init flexbot
go mod tidy
```
