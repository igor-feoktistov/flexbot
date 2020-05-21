#!/bin/sh

export DIB_DEV_USER_USERNAME=devuser
export DIB_DEV_USER_PWDLESS_SUDO=Yes
export DIB_DEV_USER_PASSWORD=secret
export DIB_BOOTLOADER_SERIAL_CONSOLE=tty0
export DIB_BLOCK_DEVICE_CONFIG='
  - local_loop:
      name: image0
      size: 3GB
  - partitioning:
      base: image0
      label: mbr
      partitions:
        - name: root
          flags: [ boot, primary ]
          size: 100%
  - mkfs:
      name: root_fs
      base: root
      label: rootfs
      type: xfs
      mount:
        mount_point: /
        fstab:
          options: "defaults"
          fsck-passno: 1
'
disk-image-create vm block-device-mbr centos cloud-init-nocloud devuser dracut-regenerate iscsi-boot install-static -p lvm2 -p device-mapper -p device-mapper-multipath -p device-mapper-libs -t raw -o centos-8.1.01-iboot
