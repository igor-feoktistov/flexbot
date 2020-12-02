## 1.4.3 (December 2, 2020)

FEATURES:

* **New Argument:** `rancher_api/enabled` - (Optional) Gives a flexibility to define `rancher_api` which is not functional yet (spin-up Rancher Management Server as an example).

ENHANCEMENTS:

* Migrated to new Hashicorp terraform-plugin-sdk

BUG FIXES:

* Fixed provider crash issue in resourceDelete routine in case of `rancher_api` is not defined.

## 1.4.2 (November 24, 2020)

FEATURES:

* **New Argument:** `ssh_node_bootdisk_resize_commands` - (Optional) To support boot disk resize on host.
* **New Argument:** `ssh_node_datadisk_resize_commands` - (Optional) To support data disk resize on host.
* **New Argument:** `node_grace_timeout` - (Optional) Grace timeout after each node update in changing blade_spec or os_image/seed_template.

ENHANCEMENTS:

* Resource update routine now supports re-sizing for boot_lun and data_lun.

BUG FIXES:

## 1.4.1 (November 20, 2020)

FEATURES:

* **New Argument:** `rancher_api` - (Optional) Integration with Rancher API helps with node management of Rancher custom clusters.
* **New Argument:** `synchronized_updates` - (Optional) Forces sequential order for node updates.
* **New Argument:** `ssh_node_init_commands` - (Optional) Brings `provisioner` functionality inside `flexbot_server` resource for better error management and node updates functionality.

ENHANCEMENTS:

* Support for Rancher API which helps with graceful node management (cordon/drain/uncordon) in Rancher custom clusters.
* Support for synchronized node updates. Highly recommended for Rancher cluster node management.
* Improved node update routine.
* Added support for image and cloud-init seed templates updates

BUG FIXES:

* Fixed the bug with storage cleanup while cloud-init seed template updates
