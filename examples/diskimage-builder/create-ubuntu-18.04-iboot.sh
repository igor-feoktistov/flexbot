#!/bin/sh

export DIB_RELEASE=bionic
export DIB_DEV_USER_USERNAME=devuser
export DIB_DEV_USER_PWDLESS_SUDO=Yes
export DIB_DEV_USER_PASSWORD=secret
export DIB_BOOTLOADER_SERIAL_CONSOLE=tty0
export DIB_BLOCK_DEVICE_CONFIG='
  - local_loop:
      name: image0
      size: 3GB
      mkfs:
        name: root_fs
        label: rootfs
        type: ext4
        mount:
          mount_point: /
          fstab:
            options: "defaults"
            fsck-passno: 1'
disk-image-create vm block-device-mbr ubuntu cloud-init-nocloud devuser iscsi-boot bootloader grub2 install-static -p multipath-tools -p multipath-tools-boot -p kpartx-boot -t raw -o ubuntu-18.04.05.01-iboot
