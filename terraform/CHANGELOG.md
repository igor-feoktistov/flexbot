## 1.6.2 (January 14, 2021)

FEATURES:
* **New Parameter:** `wait_for_node_timeout` in `rancher_api` provider argument - (Optional) Ensures Rancher node is "active" before completing.
* **New Parameter:** `num_of_threads` in `blade_spec` compute argument - (Optional) Blade search by range of `num_of_threads` or exact value.
* **New Datasource:** `flexbot_crypt` to help with encrypting user names, passwords, and tokens.

ENHANCEMENTS:
* Record assigned compute details in node annotations (suffix flexpod-compute).
* Record assigned storage details in node annotations (suffix flexpod-storage).
* The above enhancements require `rancher_api` and `wait_for_node_timeout` > 0 in provider settings.
* Package `crypt` is updated to replace md5 with sha256 sum. Make sure to re-generate encrypted strings in credentials.

BUG FIXES:


## 1.6.1 (January 4, 2021)

FEATURES:

ENHANCEMENTS:
* IPAM code optimization to make it more "plugin" friendly.
* Code sync-up with latest changes in kdomanski/iso9660 package.

BUG FIXES:
* Fixed the issue when under certain conditions change in blade_spec would
  trigger blade re-assignment in spite of current blade meets all criteria
  in newly provided blade_spec.


## 1.6.0 (December 18, 2020)

FEATURES:
* **New Resource:** `repo` - Flexbot images and templates repositories management via Terraform

ENHANCEMENTS:
* Added support for Linux on ARM64 platform

BUG FIXES:


## 1.5.4 (December 15, 2020)

FEATURES:
* **New Argument:** `compute/powerstate` - (Optional) Enables powerstate management for UCS blade.

ENHANCEMENTS:

BUG FIXES:


## 1.5.3 (December 8, 2020)

FEATURES:

ENHANCEMENTS:

* Improved Rancher node use case logic for server updates.
* Code optimization for Rancher

BUG FIXES:


## 1.5.1 (December 2, 2020)

FEATURES:

* **New Argument:** `auto_snapshot_on_update` - (Optional) Enables automatic snapshots on image or seed template updates.
* **New Argument:** `restore` - (Optional) Enables restore functionality for server LUN's.
* **New Argument:** `rancher_api/enabled` - (Optional) Gives a flexibility to define `rancher_api` which is not functional yet (spin-up Rancher Management Server as an example).

ENHANCEMENTS:

* Migrated to new Hashicorp Plugin SDK.
* Restore feature allows to restore server LUN's from snapshots.

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
