#!/usr/bin/bash
#
# dracut module: boots a snapper snapshot from the "Ryoku snapshots" GRUB
# menu, read-only or restored. It runs after dracut has unlocked LUKS, so an
# encrypted disk restores the same way as a plain one. Only the images
# `ryoku boot-menu sync` builds for the snapshot entries carry it (dracut
# --add ryoku-snapshot); Fedora's own initramfs images stay as they are.

check() {
  return 255
}

depends() {
  echo btrfs
}

installkernel() {
  hostonly='' instmods overlay
}

# shellcheck disable=SC2154 # dracut sets moddir
install() {
  inst_multiple mkdir mount umount mv date ln
  inst_hook pre-mount 90 "$moddir/ryoku-snapshot-restore.sh"
  inst_hook pre-pivot 10 "$moddir/ryoku-snapshot-preview.sh"
}
