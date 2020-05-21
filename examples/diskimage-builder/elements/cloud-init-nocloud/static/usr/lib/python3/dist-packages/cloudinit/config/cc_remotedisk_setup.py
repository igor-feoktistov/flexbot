# vi: ts=4 expandtab
#
#    Author: Igor Feoktistov <ifeoktistov@yahoo.com>
#
#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.
############################################################################
"""
The module remotedisk_setup provides a simple and uniform way
to handle remote disks such as:
	- iSCSI LUN's:
	    - configures Open iSCSI initiator;
	    - configures device multipath;
	    - enables necessary services;
	    - attaches iSCSI LUN;
	    - discovers multipath device;
	    - creates logical volume;
	    - creates filesystem;
	    - mounts filesystem;
	    - configures /etc/fstab
	- Hypervisor disks (OpenStack Cinder volumes, AWS EBS, etc):
	    - creates logical volume;
	    - creates filesystem;
	    - mounts filesystem;
	    - configures /etc/fstab
	- NFS shares:
	    - mounts NFS share;
	    - configures /etc/fstab
##########################################################
# Example configuration to handle iSCSI LUN:
##########################################################
remotedisk_setup:
   - device: 'iscsi:192.168.1.1:6:3260:1:iqn.1992-08.com.netapp:sn.62546b567fbf11e4811590e2ba6cc3b4:vs.10'
     lvm_group: 'vg_data1'
     lvm_volume: 'lv_data1'
     fs_type: 'xfs'
     mount_point: '/apps/data1'
##########################################################
# Parameters:
#    mandatory:
#       device: 'iscsi:<iSCSI target host/LIF>:<transport protocol>:<port>:<LUN ID>:<iSCSI target name>'
#       fs_type: '<filesystem type>'
#       mount_point: '<mount point dir path>'
#    optional:
#       initiator_name: '<iSCSI initiator name, default is iqn.2005-02.com.open-iscsi:<hostname>>'
#       mount_opts: '<filesystem mount options, default is "defaults,_netdev">'
#       lvm_group: '<LVM logical group name>'
#       lvm_volume: '<LVM logical volume name>'
#       fs_opts: '<filesystem create options specific to mkfs.fs_type>'
#       fs_freq: '<fstab fs freq, default is "1">'
#       fs_passno: '<fstab fs passno, default is "2">'
#    notes:
#       missing lvm_group and lvm_volume will cause filesystem creation on top of multipath device
##########################################################
# Example configuration to handle OpenStack Cinder volume:
##########################################################
remotedisk_setup:
   - device: 'ebs0'
     lvm_group: 'vg_data1'
     lvm_volume: 'lv_data1'
     fs_type: 'ext4'
     mount_point: '/apps/data1'
##########################################################
# Parameters:
#    mandatory:
#       device: 'ebs<0-9> or block device path /dev/vd<b-z>'
#       fs_type: '<filesystem type>'
#       mount_point: '<mount point dir path>'
#    optional:
#       mount_opts: '<filesystem mount options, default is "defaults">'
#       lvm_group: '<LVM logical group name>'
#       lvm_volume: '<LVM logical volume name>'
#       fs_opts: '<filesystem create options specific to mkfs.fs_type>'
#       fs_freq: '<fstab fs freq, default is "1">'
#       fs_passno: '<fstab fs passno, default is "2">'
#    notes:
#       missing lvm_group and lvm_volume will cause filesystem creation on top of block device
##########################################################
# Example configuration to handle NFS shares:
##########################################################
remotedisk_setup:
   - device: 'nfs:192.168.1.1:/myshare'
     mount_point: '/apps/data1'
     mount_opts: 'tcp,rw,rsize=65536,wsize=65536'
##########################################################
# Parameters:
#    mandatory:
#       device: 'nfs:<NFS host>:<NFS share path>'
#       mount_point: '<mount point dir path>'
#    optional:
#       mount_opts: '<NFS share mount options, default is "defaults">'
#       fs_type: 'nfs'
#       fs_freq: '<fstab fs freq, default is "0">'
#       fs_passno: '<fstab fs passno, default is "0">'
##########################################################
"""

import logging
import os
import time
import shlex
import fnmatch
import subprocess
import re
from string import whitespace

from cloudinit.settings import PER_INSTANCE
from cloudinit import type_utils
from cloudinit import util
from cloudinit import templater

frequency = PER_INSTANCE

WAIT_4_BLOCKDEV_MAPPING_ITER = 60
WAIT_4_BLOCKDEV_MAPPING_SLEEP = 5
WAIT_4_BLOCKDEV_DEVICE_ITER = 12
WAIT_4_BLOCKDEV_DEVICE_SLEEP = 5

LVM_CMD = util.which("lvm")
ISCSIADM_CMD = util.which("iscsiadm")
MULTIPATH_CMD = util.which("multipath")
SYSTEMCTL_CMD = util.which("systemctl")
FSTAB_PATH = "/etc/fstab"
ISCSI_INITIATOR_PATH = "/etc/iscsi/initiatorname.iscsi"


def handle(_name, cfg, cloud, log, _args):
    if "remotedisk_setup" not in cfg:
        log.debug("Skipping module named %s, no configuration found" % _name)
        return
    remotedisk_setup = cfg.get("remotedisk_setup")
    log.debug("setting up remote disk: %s", str(remotedisk_setup))
    dev_entry_iscsi = 0
    for definition in remotedisk_setup:
        try:
            device = definition.get("device")
            if device:
                if device.startswith("iscsi"):
                    handle_iscsi(cfg, cloud, log, definition, dev_entry_iscsi)
                    dev_entry_iscsi += 1
                elif device.startswith("nfs"):
                    handle_nfs(cfg, cloud, log, definition)
                elif device.startswith("ebs"):
                    handle_ebs(cfg, cloud, log, definition)
                elif device.startswith("ephemeral"):
                    handle_ebs(cfg, cloud, log, definition)
                else:
                    if "fs_type" in definition:
                        fs_type = definition.get("fs_type")
                        if fs_type == "nfs":
                            handle_nfs(cfg, cloud, log, definition)
                        else:
                            handle_ebs(cfg, cloud, log, definition)
                    else:
                        util.logexc(log, "Expexted \"fs_type\" parameter")
            else:
                util.logexc(log, "Expexted \"device\" parameter")
        except Exception as e:
            util.logexc(log, "Failed during remote disk operation\n"
                             "Exception: %s" % e)


def handle_iscsi(cfg, cloud, log, definition, dev_entry_iscsi):
    # Handle iSCSI LUN
    device = definition.get("device")
    try:
        (iscsi_host,
         iscsi_proto,
         iscsi_port,
         iscsi_lun,
         iscsi_target) = device.split(":", 5)[1:]
    except Exception as e:
        util.logexc(log,
                    "handle_iscsi: "
                    "expected \"device\" attribute in the format: "
                    "\"iscsi:<iSCSI host>:<protocol>:<port>:<LUN>:"
                    "<iSCSI target name>\": %s" % e)
        return
    if dev_entry_iscsi == 0:
        (hostname, fqdn) = util.get_hostname_fqdn(cfg, cloud)
        if "initiator_name" in definition:
            initiator_name = definition.get("initiator_name")
        else:
            initiator_name = "iqn.2005-02.com.open-iscsi:%s" % hostname
        util.write_file(ISCSI_INITIATOR_PATH, "InitiatorName=%s" % initiator_name)
        multipath_tmpl_fn = cloud.get_template_filename("multipath.conf")
        if multipath_tmpl_fn:
            templater.render_to_file(multipath_tmpl_fn, "/etc/multipath.conf", {})
        else:
            log.warn("handle_iscsi: template multipath.conf not found")
        if cloud.distro.osfamily == "redhat":
            iscsi_services = ["iscsi", "iscsid"]
            multipath_services = ["multipathd"]
        elif cloud.distro.osfamily == 'debian':
            iscsi_services = ["open-iscsi", "iscsid"]
            multipath_services = ["multipathd"]
        else:
            util.logexc(log,
                        "handle_iscsi: "
                        "unsupported osfamily \"%s\"" % cloud.distro.osfamily)
            return
        for service in iscsi_services:
            _service_wrapper(cloud, log, service, "enable")
            _service_wrapper(cloud, log, service, "restart")
        for service in multipath_services:
            _service_wrapper(cloud, log, service, "enable")
            _service_wrapper(cloud, log, service, "restart")
    blockdev = _iscsi_lun_discover(log,
                                   iscsi_host,
                                   iscsi_port,
                                   iscsi_lun,
                                   iscsi_target)
    if blockdev:
        lvm_group = definition.get("lvm_group")
        lvm_volume = definition.get("lvm_volume")
        fs_type = definition.get("fs_type")
        fs_opts = definition.get("fs_opts")
        mount_point = definition.get("mount_point")
        mount_opts = definition.get("mount_opts")
        if not mount_opts:
            mount_opts = 'defaults,_netdev'
        else:
            if mount_opts.find("_netdev") == -1:
                mount_opts = "%s,_netdev" % (mount_opts)
        fs_freq = definition.get("fs_freq")
        if not fs_freq:
            fs_freq = "1"
        fs_passno = definition.get("fs_passno")
        if not fs_passno:
            fs_passno = "2"
        if lvm_group and lvm_volume:
            for vg_name in _list_vg_names():
                if vg_name == lvm_group:
                    util.logexc(log,
                                "handle_iscsi: "
                                "logical volume group '%s' exists already"
                                % lvm_group)
                    return
            for lv_name in _list_lv_names():
                if lv_name == lvm_volume:
                    util.logexc(log,
                                "handle_iscsi: "
                                "logical volume '%s' exists already"
                                % lvm_volume)
                    return
            blockdev = _create_lv(log, blockdev, lvm_group, lvm_volume)
        if blockdev:
            if mount_point and fs_type:
                _create_fs(log, blockdev, fs_type, fs_opts)
                _add_fstab_entry(log,
                                 blockdev,
                                 mount_point,
                                 fs_type,
                                 mount_opts,
                                 fs_freq,
                                 fs_passno)
                _mount_fs(log, mount_point)
            else:
                util.logexc(log,
                            "handle_iscsi: "
                            "expexted \"mount_point\" "
                            "and \"fs_type\" parameters")


def handle_nfs(cfg, cloud, log, definition):
    # Handle NFS share mounts
    device = definition.get("device")
    if device.startswith("nfs"):
        (proto, share_path) = device.split(":", 1)
    else:
        share_path = device
    fs_type = definition.get("fs_type")
    mount_point = definition.get("mount_point")
    mount_opts = definition.get("mount_opts")
    if not mount_opts:
        mount_opts = "defaults"
    fs_freq = definition.get("fs_freq")
    if not fs_freq:
        fs_freq = "0"
    fs_passno = definition.get("fs_passno")
    if not fs_passno:
        fs_passno = "0"
    if mount_point and fs_type:
        _add_fstab_entry(log,
                         share_path,
                         mount_point,
                         fs_type,
                         mount_opts,
                         fs_freq,
                         fs_passno)
        _mount_fs(log, mount_point)
    else:
        util.logexc(log,
                    "handle_nfs: "
                    "expexted \"mount_point\" and \"fs_type\" parameters")


def handle_ebs(cfg, cloud, log, definition):
    # Handle block device either explicitly provided via device path or
    # via device name mapping (Amazon/OpenStack)
    device = definition.get("device")
    blockdev = _cloud_device_2_os_device(cloud, log, device)
    if blockdev:
        lvm_group = definition.get("lvm_group")
        lvm_volume = definition.get("lvm_volume")
        fs_type = definition.get("fs_type")
        fs_opts = definition.get("fs_opts")
        mount_point = definition.get("mount_point")
        mount_opts = definition.get("mount_opts")
        if not mount_opts:
            mount_opts = "defaults"
        fs_freq = definition.get("fs_freq")
        if not fs_freq:
            fs_freq = "1"
        fs_passno = definition.get("fs_passno")
        if not fs_passno:
            fs_passno = "2"
        if lvm_group and lvm_volume:
            for vg_name in _list_vg_names():
                if vg_name == lvm_group:
                    util.logexc(log,
                                "handle_ebs: "
                                "logical volume group '%s' exists already"
                                % lvm_group)
                    return
            for lv_name in _list_lv_names():
                if lv_name == lvm_volume:
                    util.logexc(log,
                                "handle_ebs: "
                                "logical volume '%s' exists already"
                                % lvm_volume)
                    return
            blockdev = _create_lv(log, blockdev, lvm_group, lvm_volume)
        if blockdev:
            if mount_point and fs_type:
                _create_fs(log, blockdev, fs_type, fs_opts)
                _add_fstab_entry(log,
                                 blockdev,
                                 mount_point,
                                 fs_type,
                                 mount_opts,
                                 fs_freq,
                                 fs_passno)
                _mount_fs(log, mount_point)
            else:
                util.logexc(log,
                            "handle_ebs: "
                            "expexted \"mount_point\" and "
                            "\"fs_type\" parameters")


def _cloud_device_2_os_device(cloud, log, name):
    # Translate cloud device (ebs# and ephemaral#) to OS block device path
    blockdev = None
    for i in range(WAIT_4_BLOCKDEV_MAPPING_ITER):
        if (cloud.datasource.metadata and
                "block-device-mapping" in cloud.datasource.metadata):
            metadata = cloud.datasource.metadata
        else:
            if (cloud.datasource.ec2_metadata and
                    "block-device-mapping" in cloud.datasource.ec2_metadata):
                metadata = cloud.datasource.ec2_metadata
            else:
                util.logexc(log,
                            "_cloud_device_2_os_device: "
                            "metadata item block-device-mapping not found")
                return None
        blockdev_items = metadata["block-device-mapping"].iteritems()
        for (map_name, device) in blockdev_items:
            if map_name == name:
                blockdev = device
                break
        if blockdev is None:
            cloud.datasource.get_data()
            time.sleep(WAIT_4_BLOCKDEV_MAPPING_SLEEP)
            continue
    if blockdev is None:
        util.logexc(log,
                    "_cloud_device_2_os_device: "
                    "unable to convert %s to a device" % name)
        return None
    if not blockdev.startswith("/"):
        blockdev_path = "/dev/%s" % blockdev
    else:
        blockdev_path = blockdev
    for i in range(WAIT_4_BLOCKDEV_DEVICE_ITER):
        if os.path.exists(blockdev_path):
            return blockdev_path
        time.sleep(WAIT_4_BLOCKDEV_DEVICE_SLEEP)
    util.logexc(log,
                "_cloud_device_2_os_device: "
                "device %s does not exist" % blockdev_path)
    return None


def _list_vg_names():
    # List all LVM volume groups
    p = subprocess.Popen([LVM_CMD, "vgs", "-o", "vg_name"],
                         stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    err = p.wait()
    if err:
        return []
    output = p.communicate()[0]
    output = output.decode().split("\n")
    if not output:
        return []
    header = output[0].strip()
    if header != "VG":
        return []
    names = []
    for name in output[1:]:
        if not name:
            break
        names.append(name.strip())
    return names


def _list_lv_names():
    # List all LVM logical volumes
    p = subprocess.Popen([LVM_CMD, "lvs", "-o", "lv_name"],
                         stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    err = p.wait()
    if err:
        return []
    output = p.communicate()[0]
    output = output.decode().split("\n")
    if not output:
        return []
    header = output[0].strip()
    if header != "LV":
        return []
    names = []
    for name in output[1:]:
        if not name:
            break
        names.append(name.strip())
    return names


def _create_lv(log, device, vg_name, lv_name):
    # Create volume group
    pvcreate_cmd = [LVM_CMD, "pvcreate", device]
    vgcreate_cmd = [LVM_CMD, "vgcreate", vg_name, device]
    lvcreate_cmd = [LVM_CMD,
                    "lvcreate", "-l", "100%FREE", "--name", lv_name, vg_name]
    try:
        util.subp(pvcreate_cmd)
        util.subp(vgcreate_cmd)
        util.subp(lvcreate_cmd)
        return "/dev/mapper/%s-%s" % (vg_name, lv_name)
    except Exception as e:
        util.logexc(log,
                    "_create_lv: "
                    "failed to create LVM volume '%s' for device '%s': %s"
                    % (lv_name, device, e))
        return None


def _create_fs(log, device, fs_type, fs_opts=None):
    # Create filesystem
    mkfs_cmd = util.which("mkfs.%s" % fs_type)
    if not mkfs_cmd:
        mkfs_cmd = util.which("mk%s" % fs_type)
    if not mkfs_cmd:
        util.logexc(log,
                    "_create_fs: "
                    "cannot create filesystem type '%s': "
                    "failed to find mkfs.%s command" % (fs_type, fs_type))
        return
    try:
        if fs_opts:
            util.subp([mkfs_cmd, fs_opts, device])
        else:
            util.subp([mkfs_cmd, device])
    except Exception as e:
        util.logexc(log,
                    "_create_fs: "
                    "failed to create filesystem type '%s': %s" % (fs_type, e))


def _add_fstab_entry(log,
                     device,
                     mount_point,
                     fs_type,
                     mount_opts,
                     fs_freq,
                     fs_passno):
    # Create fstab entry
    fstab_lines = []
    for line in util.load_file(FSTAB_PATH).splitlines():
        try:
            toks = re.compile("[%s]+" % (whitespace)).split(line)
        except:
            pass
        if len(toks) > 0 and toks[0] == device:
            util.logexc(log,
                        "_add_fstab_entry: "
                        "file %s has device %s already" % (FSTAB_PATH, device))
            return
        if len(toks) > 1 and toks[1] == mount_point:
            util.logexc(log,
                        "_add_fstab_entry: "
                        "file %s has mount point %s already"
                        % (FSTAB_PATH, mount_point))
            return
        fstab_lines.append(line)
    fstab_lines.extend(["%s\t%s\t%s\t%s\t%s\t%s" %
                       (device,
                        mount_point,
                        fs_type,
                        mount_opts,
                        fs_freq,
                        fs_passno)])
    contents = "%s\n" % ('\n'.join(fstab_lines))
    util.write_file(FSTAB_PATH, contents)


def _mount_fs(log, mount_point):
    # Mount filesystem according to fstab entry
    try:
        util.ensure_dir(mount_point)
    except Exception as e:
        util.logexc(log,
                    "_mount_fs: "
                    "failed to make '%s' mount point directory: %s"
                    % (mount_point, e))
        return
    try:
        util.subp(["mount", mount_point])
    except Exception as e:
        util.logexc(log,
                    "_mount_fs: "
                    "activating mounts via 'mount %s' failed: %s"
                    % (mount_point, e))


def _service_wrapper(cloud, log, service, command):
    # Wrapper for service related commands
    if cloud.distro.osfamily == "redhat":
        svc_cmd = [SYSTEMCTL_CMD, command, service]
    elif cloud.distro.osfamily == "debian":
        svc_cmd = [SYSTEMCTL_CMD, command, service]
    else:
        util.logexc(log,
                    "_handle_service: "
                    "unsupported osfamily \"%s\"" % cloud.distro.osfamily)
        return
    try:
        util.subp(svc_cmd, capture=False)
    except Exception as e:
        util.logexc(log,
                    "_handle_service: "
                    "failure to \"%s\" \"%s\": %s" % (command, service, e))


def _iscsi_lun_discover(log, iscsi_host, iscsi_port, iscsi_lun, iscsi_target):
    # Discover iSCSI target and map LUN ID to multipath device path
    blockdev = None
    for i in range(WAIT_4_BLOCKDEV_MAPPING_ITER):
        try:
            util.subp([ISCSIADM_CMD,
                       "--mode",
                       "discoverydb",
                       "--type",
                       "sendtargets",
                       "--portal",
                       "%s:%s" % (iscsi_host, iscsi_port),
                       "--discover",
                       "--login",
                       "all"],
                      capture=False)
        except Exception as e:
            pass
        p = subprocess.Popen([ISCSIADM_CMD, "-m", "node"],
                             stdout=subprocess.PIPE,
                             stderr=subprocess.PIPE)
        err = p.wait()
        if err:
            util.logexc(log,
                        "_iscsi_lun_discover: "
                        "failure from \"%s -m node\" command" % ISCSIADM_CMD)
            return None
        output = p.communicate()[0]
        output = output.decode().split("\n")
        if not output:
            util.logexc(log,
                        "_iscsi_lun_discover: "
                        "no iSCSI nodes discovered for target \"%s\""
                        % iscsi_target)
            time.sleep(WAIT_4_BLOCKDEV_MAPPING_SLEEP)
            continue
        for node in output:
            iscsi_portal = node.split(",", 1)[0]
            if iscsi_portal:
                try:
                    util.subp([ISCSIADM_CMD,
                               "-m",
                               "node",
                               "-T",
                               iscsi_target,
                               "-p",
                               iscsi_portal,
                               "--op",
                               "update",
                               "-n",
                               "node.startup",
                               "-v",
                               "automatic"],
                              capture=False)
                except Exception as e:
                    util.logexc(log,
                                "_iscsi_lun_discover: "
                                "failure in attempt to set automatic binding "
                                "for target portal \"%s\": %s"
                                % (iscsi_portal, e))
                    pass
        p = subprocess.Popen([ISCSIADM_CMD, "-m", "session", "-P3"],
                             stdout=subprocess.PIPE,
                             stderr=subprocess.PIPE)
        err = p.wait()
        if err:
            util.logexc(log,
                        "_iscsi_lun_discover: "
                        "failure from \"%s -m session -P3\" command"
                        % ISCSIADM_CMD)
            return None
        output = p.communicate()[0]
        output = output.decode().split("\n")
        if not output:
            util.logexc(log,
                        "_iscsi_lun_discover: "
                        "no iSCSI sessions discovered for target \"%s\""
                        % iscsi_target)
        else:
            current_iscsi_target = None
            current_iscsi_sid = None
            current_iscsi_lun = None
            for line in output:
                m = re.search("^Target: ([a-z0-9\.:-]*)", line)
                if m:
                    current_iscsi_target = m.group(1)
                    continue
                else:
                    if (current_iscsi_target and
                            current_iscsi_target == iscsi_target):
                        m = re.search("SID: ([0-9]*)", line)
                        if m:
                            if current_iscsi_sid and not current_iscsi_lun:
                                try:
                                    util.subp([ISCSIADM_CMD,
                                               "-m",
                                               "session",
                                               "-r",
                                               current_iscsi_sid,
                                               "-u"],
                                              capture=False)
                                except:
                                    pass
                            current_iscsi_sid = m.group(1)
                            current_iscsi_lun = None
                            continue
                        m = re.search("scsi[0-9]* Channel [0-9]* "
                                      "Id [0-9]* Lun: ([0-9]*)", line)
                        if m:
                            current_iscsi_lun = m.group(1)
                            continue
                        if (current_iscsi_lun and
                                current_iscsi_lun == iscsi_lun):
                            m = re.search("Attached scsi disk (sd[a-z]*)",
                                          line)
                            if m:
                                attached_scsi_disk = m.group(1)
                                p = subprocess.Popen(["/lib/udev/scsi_id",
                                                      "-g", "-u", "-d",
                                                      "/dev/%s"
                                                      % attached_scsi_disk],
                                                     stdout=subprocess.PIPE,
                                                     stderr=subprocess.PIPE)
                                err = p.wait()
                                if err:
                                    util.logexc(log,
                                                "_iscsi_lun_discover: "
                                                "failure from "
                                                "\"/lib/udev/scsi_id\" "
                                                "command")
                                    return None
                                output2 = p.communicate()[0]
                                output2 = output2.decode().split('\n')
                                if not output2:
                                    util.logexc(log,
                                                "_iscsi_lun_discover: "
                                                "no wwid returned for device "
                                                "\"/dev/%s\""
                                                % attached_scsi_disk)
                                else:
                                    blockdev = "/dev/mapper/%s" % output2[0]
            if current_iscsi_sid and not current_iscsi_lun:
                try:
                    util.subp([ISCSIADM_CMD,
                               "-m",
                               "session",
                               "-r",
                               current_iscsi_sid,
                               "-u"],
                              capture=False)
                except:
                    pass
        if blockdev:
            break
        else:
            time.sleep(WAIT_4_BLOCKDEV_MAPPING_SLEEP)
    if blockdev:
        for i in range(WAIT_4_BLOCKDEV_DEVICE_ITER):
            if os.path.exists(blockdev):
                return blockdev
            try:
                util.subp([MULTIPATH_CMD], capture=False)
            except Exception as e:
                util.logexc(log,
                            "_iscsi_lun_discover: "
                            "failure to run \"%s\": %s" % (MULTIPATH_CMD, e))
                return None
            time.sleep(WAIT_4_BLOCKDEV_DEVICE_SLEEP)
    else:
        return None
