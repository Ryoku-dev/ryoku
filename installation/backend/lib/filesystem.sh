#!/usr/bin/env bash
# shellcheck shell=bash
# format the partitions, lay out the Btrfs subvolumes, mount it all under
# /mnt. two entrypoints: ryoku_filesystems (mkfs + subvol create) in the
# "filesystems" stage, ryoku_mount (mounts + swapfile) in the "mount" stage.
# ESP_DEV / ROOT_DEV come from disk + luks.
#
# subvolume layout. always: @ -> /, @log -> /var/log, @pkg -> the package
# cache path selected by RYOKU_PACKAGE_CACHE_DIR (defaults to pacman's cache).
# toggle-gated: @home, @snapshots, @backups (RYOKU_SUBVOL_*). @swap iff
# RYOKU_SWAP_GIB > 0, so the swapfile stays out of @:
#   @ @home @log @pkg @snapshots @backups @swap

# zstd:1 (fastest level): the install writes ~13 GiB, so level 1 cuts the
# compress-on-write CPU markedly vs the default (3) while keeping a solid ratio.
# genfstab carries this into the target fstab, so runtime writes stay fast too.
RYOKU_BTRFS_OPTS="compress=zstd:1,noatime"

: "${RYOKU_PACKAGE_CACHE_DIR:=/var/cache/pacman/pkg}"

ryoku_filesystems() {
  # Alongside uses a distinct label in both XBOOTLDR and dedicated-ESP modes.
  local boot_label=BOOT
  [[ ${RYOKU_DISK_STRATEGY:-} == alongside ]] && boot_label=${RYOKU_ALONGSIDE_BOOT_LABEL:-RYOKUBOOT}
  log 'formatting boot (%s, vfat, label %s) and root (%s, btrfs)' "$ESP_DEV" "$boot_label" "$ROOT_DEV"
  run mkfs.vfat -F32 -n "$boot_label" "$ESP_DEV"
  run mkfs.btrfs -f -L ryoku "$ROOT_DEV"

  log "creating subvolumes"
  run mount "$ROOT_DEV" /mnt
  run btrfs subvolume create /mnt/@
  [[ ${RYOKU_SUBVOL_HOME:-1} == 1 ]] && run btrfs subvolume create /mnt/@home
  run btrfs subvolume create /mnt/@log
  run btrfs subvolume create /mnt/@pkg
  [[ ${RYOKU_SUBVOL_SNAPSHOTS:-1} == 1 ]] && run btrfs subvolume create /mnt/@snapshots
  [[ ${RYOKU_SUBVOL_BACKUPS:-0} == 1 ]] && run btrfs subvolume create /mnt/@backups
  # swapfile can't live in @ (or any snapshotted subvol): btrfs refuses to
  # snapshot one with an active swapfile, which breaks snapper on every
  # pacman transaction. its own @swap subvol.
  (( ${RYOKU_SWAP_GIB:-0} > 0 )) && run btrfs subvolume create /mnt/@swap
  run umount /mnt
}

ryoku_mount() {
  local o=$RYOKU_BTRFS_OPTS
  log 'mounting subvolumes under /mnt (%s)' "$o"
  run mount -o "$o,subvol=@" "$ROOT_DEV" /mnt

  # dirs must exist on @ before their subvols mount over them.
  run mkdir -p /mnt/var/log "/mnt$RYOKU_PACKAGE_CACHE_DIR" /mnt/boot
  [[ ${RYOKU_SUBVOL_HOME:-1} == 1 ]] && run mkdir -p /mnt/home
  [[ ${RYOKU_SUBVOL_SNAPSHOTS:-1} == 1 ]] && run mkdir -p /mnt/.snapshots
  [[ ${RYOKU_SUBVOL_BACKUPS:-0} == 1 ]] && run mkdir -p /mnt/.backups

  [[ ${RYOKU_SUBVOL_HOME:-1} == 1 ]] && run mount -o "$o,subvol=@home" "$ROOT_DEV" /mnt/home
  run mount -o "$o,subvol=@log" "$ROOT_DEV" /mnt/var/log
  run mount -o "$o,subvol=@pkg" "$ROOT_DEV" "/mnt$RYOKU_PACKAGE_CACHE_DIR"
  [[ ${RYOKU_SUBVOL_SNAPSHOTS:-1} == 1 ]] && run mount -o "$o,subvol=@snapshots" "$ROOT_DEV" /mnt/.snapshots
  [[ ${RYOKU_SUBVOL_BACKUPS:-0} == 1 ]] && run mount -o "$o,subvol=@backups" "$ROOT_DEV" /mnt/.backups

  run mount "$ESP_DEV" /mnt/boot

  # ESP capacity guard, HERE (right after the ESP mounts) and NOT deep in the
  # bootloader step: the Limine binaries, the kernel, and both initramfs images
  # have to fit on /mnt/boot, but pacstrap + mkinitcpio fill /boot long before
  # the bootloader runs, so a too-small ESP used to fail cryptically mid-install.
  # The generated layouts use at least 1 GiB (whole) or 2 GiB (alongside);
  # this catches only hand-built/reused undersized boot partitions.
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log "dry-run: would require >= 64 MiB free on /boot"
  else
    local esp_avail_kib
    esp_avail_kib=$(df -k --output=avail /mnt/boot 2>/dev/null | tail -1 | tr -d ' ')
    [[ $esp_avail_kib =~ ^[0-9]+$ ]] || esp_avail_kib=0
    (( esp_avail_kib >= 65536 )) || die '/boot has %s KiB free; need >= 64 MiB for the bootloader + kernel + initramfs.' "$esp_avail_kib"
  fi

  ryoku_swapfile
}

ryoku_swapfile() {
  if (( ${RYOKU_SWAP_GIB:-0} <= 0 )); then
    log "swap: none"
    return 0
  fi
  # @swap is mounted separately so the swapfile never lands inside a
  # snapshotted subvol (see above). mkswapfile sets NOCOW + the right perms;
  # genfstab picks up both the mount and the swap entry later.
  log 'swap: %sGiB swapfile in @swap (kept out of snapshots)' "$RYOKU_SWAP_GIB"
  run mkdir -p /mnt/swap
  run mount -o noatime,subvol=@swap "$ROOT_DEV" /mnt/swap
  run btrfs filesystem mkswapfile --size "${RYOKU_SWAP_GIB}g" /mnt/swap/swapfile
  run swapon /mnt/swap/swapfile
}
