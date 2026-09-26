#!/usr/bin/env bash
# preflight: refuse to start unless we're root, in UEFI mode with Secure Boot
# off, and pointed at a big-enough WHOLE disk. under dry-run the checks just
# narrate and never abort, so the flow can be exercised on a dev box with no
# real target disk.

# min target disk: 32 GiB.
RYOKU_MIN_DISK_BYTES=34359738368

# ryoku_secureboot_enabled: true when firmware Secure Boot is currently ON. the
# SecureBoot efivar payload is a 4-byte attribute prefix + a 1-byte value; the
# last byte is the state (1 = enabled). an absent var reads as not enabled.
# RYOKU_SB_VAR overrides the efivar path (tests only).
ryoku_secureboot_enabled() {
  local var=${RYOKU_SB_VAR:-/sys/firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c}
  [[ -e $var ]] || return 1
  local last
  last=$(tail -c1 "$var" 2>/dev/null | od -An -tu1 2>/dev/null | tr -d '[:space:]' || true)
  [[ $last == 1 ]]
}

ryoku_preflight() {
  # dry-run never touches the machine, so narrate and return; we'd just probe
  # hardware that might not be on the dev box.
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log 'preflight: would require root, UEFI (/sys/firmware/efi) with Secure Boot off (override RYOKU_ALLOW_SECUREBOOT=1), and %s a whole disk >= 32 GiB' "$RYOKU_DISK"
    log "preflight: would log the disk's logical sector size (blockdev --getss)"
    log 'preflight: would require the repo payload at %s and a working DNS resolver before any disk write' "$RYOKU_REPO"
    if [[ ${RYOKU_DISK_STRATEGY:-} == alongside ]]; then
      if [[ -n ${RYOKU_RESIZE_PART:-} ]]; then
        log 'preflight: would also require GPT + a usable ESP to identify the existing OS, the shrink tool for %s'\''s filesystem, and RYOKU_RESIZE_TAKE_MIB >= %s; auto mode uses a dedicated Ryoku ESP when the existing ESP has < 8 MiB free' "${RYOKU_RESIZE_PART}" "$(( (2 + $(ryoku_min_root_gib)) * 1024 ))"
      else
        log 'preflight: would also require GPT, a usable EF00 ESP to identify the existing OS, a free region >= %sGiB, and warn on any BitLocker neighbor; auto mode shares the existing ESP with >= 8 MiB free and otherwise creates a dedicated Ryoku ESP' "$(( 2 + $(ryoku_min_root_gib) ))"
      fi
    fi
    log 'preflight: ok (profile=%s, strategy=%s, encrypt=%s)' "$RYOKU_PROFILE" "$RYOKU_DISK_STRATEGY" "${RYOKU_ENCRYPT:-0}"
    return 0
  fi

  # root: partitioning, mkfs, pacstrap, arch-chroot all need it.
  [[ $EUID -eq 0 ]] || die "must run as root"

  # UEFI: boot chain is Limine + an ESP.
  [[ -d /sys/firmware/efi ]] || die "not booted in UEFI mode (/sys/firmware/efi missing)"

  # Secure Boot: Limine ships unsigned, so a machine enforcing Secure Boot
  # refuses to run it. Fail HERE with firmware guidance instead of installing a
  # system that then dies at a security violation on first boot.
  # RYOKU_ALLOW_SECUREBOOT=1 overrides (e.g. the user enrolled their own keys).
  if [[ ${RYOKU_ALLOW_SECUREBOOT:-} != 1 ]] && ryoku_secureboot_enabled; then
    die "Secure Boot is enabled and Limine is unsigned, so the installed system will not boot. Disable Secure Boot in your firmware (UEFI) setup screen, then retry. Set RYOKU_ALLOW_SECUREBOOT=1 only if you have enrolled your own keys."
  fi

  # target disk has to exist and be a block device.
  [[ -b $RYOKU_DISK ]] || die 'target %s is not a block device' "$RYOKU_DISK"

  # target must be a WHOLE disk, not a partition: repartitioning a partition
  # device is nonsense and 'whole' would wipe its parent's table. lsblk TYPE
  # separates a disk from a part/lvm/crypt node.
  local dtype
  dtype=$(lsblk -dno TYPE "$RYOKU_DISK" 2>/dev/null || true)
  [[ $dtype == disk ]] || die 'target %s is a '\''%s'\'', not a whole disk. Pass a disk (e.g. /dev/nvme0n1 or /dev/sda), not a partition.' "$RYOKU_DISK" "${dtype:-unknown}"

  # target disk has to be >= 32 GiB.
  local size
  size=$(blockdev --getsize64 "$RYOKU_DISK")
  (( size >= RYOKU_MIN_DISK_BYTES )) || \
    die '%s is %s GiB; need at least 32 GiB' "$RYOKU_DISK" "$(( (size + 536870912) / 1073741824 ))"

  # the pacstrap set and the whole desktop payload live under $RYOKU_REPO; without
  # it pacstrap dies at "missing package list". check HERE, before the disk is
  # wiped, so a missing or mispointed payload aborts with the disk intact instead
  # of after the wipe.
  local base_list="$RYOKU_REPO/system/packages/base.packages"
  [[ -f $base_list ]] || die 'repo payload missing: %s not found (RYOKU_REPO=%s). The installer image is incomplete or RYOKU_REPO is wrong; the disk has not been touched.' "$base_list" "$RYOKU_REPO"

  log 'preflight: %s is %s GiB, %s-byte logical sectors' "$RYOKU_DISK" "$(( size / 1024 / 1024 / 1024 ))" "$(blockdev --getss "$RYOKU_DISK")"
  if [[ ${RYOKU_DISK_STRATEGY:-} == alongside ]]; then
    if [[ -n ${RYOKU_RESIZE_PART:-} ]]; then ryoku_preflight_resize; else ryoku_preflight_alongside; fi
  fi
  log 'preflight: ok (profile=%s, strategy=%s, encrypt=%s)' "$RYOKU_PROFILE" "$RYOKU_DISK_STRATEGY" "${RYOKU_ENCRYPT:-0}"
}

ryoku_resolve_esp_mode() {
  local avail_kib=${1:-0}
  case ${RYOKU_ESP_MODE:-auto} in
    auto)
      if (( avail_kib >= 8192 )); then printf 'shared\n'; else printf 'dedicated\n'; fi
      ;;
    shared)
      (( avail_kib >= 8192 )) || die 'the shared ESP has %s KiB free; shared mode needs >= 8 MiB. Use dedicated mode or free space on the ESP.' "${avail_kib}"
      printf 'shared\n'
      ;;
    dedicated) printf 'dedicated\n' ;;
    *) die 'RYOKU_ESP_MODE must be auto, shared, or dedicated (got '\''%s'\'')' "$RYOKU_ESP_MODE" ;;
  esac
}

# Alongside needs a GPT disk and an ESP: an existing one to identify the current
# OS (shared mode also needs 8 MiB free there), or -- in dedicated mode -- one
# Ryoku creates in the free space, so a disk with no ESP of its own still works.
ryoku_require_existing_esp() {
  local disk=$RYOKU_DISK pttype ef_count espinfo esp kind boot avail_kib=0
  pttype=$(blkid -o value -s PTTYPE "$disk" 2>/dev/null || true)
  [[ $pttype == gpt ]] || die 'alongside needs a GPT disk; %s has '\''%s'\'' partition table. Use whole-disk, or convert to GPT.' "$disk" "${pttype:-no}"

  # Scan every ESP to identify the existing system. Windows is preferred for
  # chainload metadata, but dedicated mode never writes the selected ESP.
  ef_count=$(sgdisk -p "$disk" 2>/dev/null | awk '$6=="EF00"' | wc -l)
  if ! espinfo=$(ryoku_esp_scan "$disk"); then
    # No existing ESP. dedicated mode creates its own in the free space, so there
    # is nothing to identify or share; any other mode still needs one to boot.
    [[ ${RYOKU_ESP_MODE:-auto} == dedicated ]] \
      || die 'no usable EFI System Partition on %s to identify the existing OS. Use whole-disk, or create an ESP first.' "$disk"
    RYOKU_RESOLVED_ESP_MODE=dedicated
    RYOKU_PF_ESP=""; RYOKU_PF_ESP_KIND=none; RYOKU_PF_ESP_BOOT=none
    export RYOKU_RESOLVED_ESP_MODE RYOKU_PF_ESP RYOKU_PF_ESP_KIND RYOKU_PF_ESP_BOOT
    log 'alongside boot mode: dedicated Ryoku ESP created in free space (no existing ESP on %s)' "$disk"
    return 0
  fi
  read -r esp kind boot <<<"$espinfo"

  avail_kib=$(ryoku_esp_free_kib "$esp")
  [[ $avail_kib =~ ^[0-9]+$ ]] || avail_kib=0
  RYOKU_RESOLVED_ESP_MODE=$(ryoku_resolve_esp_mode "$avail_kib")
  export RYOKU_RESOLVED_ESP_MODE
  if [[ $RYOKU_RESOLVED_ESP_MODE == shared ]]; then
    log 'alongside boot mode: shared ESP (%s, %s KiB free)' "$esp" "${avail_kib}"
    (( ef_count > 1 )) && log 'WARNING: %s has %s EFI System Partitions; Ryoku shares the %s ESP (%s).' "$disk" "$ef_count" "$kind" "$esp"
  else
    log 'alongside boot mode: dedicated Ryoku ESP (existing %s ESP has %s KiB free and stays untouched)' "$kind" "${avail_kib}"
  fi
  RYOKU_PF_ESP=$esp
  RYOKU_PF_ESP_KIND=$kind
  RYOKU_PF_ESP_BOOT=$boot
  export RYOKU_PF_ESP RYOKU_PF_ESP_KIND RYOKU_PF_ESP_BOOT
}

# ryoku_bitlocker_warn <disk>: a BitLocker neighbour is not blocking (the user may
# hold the key), but boot may later prompt for the recovery key.
ryoku_bitlocker_warn() {
  if lsblk -rno FSTYPE "$1" 2>/dev/null | grep -qi bitlocker; then
    log 'WARNING: a BitLocker-encrypted partition is present on %s. Booting Windows via Ryoku may prompt for the BitLocker recovery key; have it ready. (Recorded, not blocking.)' "$1"
  fi
}

# ryoku_windows_faststartup_gate <disk>: refuse an alongside install onto a disk
# whose Windows volume is hibernated or dirty. The carve path re-judges its one
# partition at shrink time, but a plain alongside install never touches the NTFS
# volume -- and that is exactly the case that bites: the partition table changes
# under a hibernated volume, then Windows Startup Repair rewrites the table on
# its next boot and takes the fresh Ryoku partitions with it. RYOKU_ALLOW_DIRTY_NTFS=1
# is the documented override for a user who accepts that risk.
ryoku_windows_faststartup_gate() {
  local disk=$1 dirty
  [[ ${RYOKU_ALLOW_DIRTY_NTFS:-} == 1 ]] && return 0
  dirty=$(ryoku_dirty_ntfs_on "$disk")
  [[ -n $dirty ]] || return 0
  die 'Windows on %s is hibernated or was left by Fast Startup (%s). Installing beside a dirty NTFS volume lets Windows Startup Repair rewrite the partition table on its next boot, which can delete the partitions Ryoku is about to create. Boot Windows, turn off Fast Startup (Power Options > Choose what the power buttons do), shut down fully, then retry. RYOKU_ALLOW_DIRTY_NTFS=1 installs anyway, at your risk.' "$disk" "$(printf '%s' "$dirty" | tr '\n' ' ')"
}

# Alongside preflight resolves the boot mode and validates the free region.
ryoku_preflight_alongside() {
  local disk=$RYOKU_DISK need_gib region_mib
  ryoku_require_existing_esp
  need_gib=$(( 2 + $(ryoku_min_root_gib) ))
  region_mib=$(ryoku_free_regions "$disk" | sort -k3,3 -nr | awk 'NR==1{print $3+0}')
  (( region_mib >= need_gib * 1024 )) || die 'no unallocated region >= %sGiB on %s (largest is %sGiB). Shrink a partition first, then retry.' "${need_gib}" "$disk" "$(( region_mib / 1024 ))"
  ryoku_bitlocker_warn "$disk"
  ryoku_windows_faststartup_gate "$disk"
  log 'preflight alongside: GPT ok, boot mode %s, existing ESP %s (%s), free region %sGiB >= %sGiB' "$RYOKU_RESOLVED_ESP_MODE" "$RYOKU_PF_ESP" "$RYOKU_PF_ESP_KIND" "$(( region_mib / 1024 ))" "${need_gib}"
}

# Carve uses the same boot-mode gate, then validates the shrink target and size.
ryoku_preflight_resize() {
  local disk=$RYOKU_DISK part=$RYOKU_RESIZE_PART take=${RYOKU_RESIZE_TAKE_MIB:-0} fstype tool need_gib
  ryoku_require_existing_esp
  [[ -b $part ]] || die 'carve target RYOKU_RESIZE_PART=%s is not a block device.' "$part"
  local parent; parent=$(lsblk -no PKNAME "$part" 2>/dev/null | head -n1)
  [[ /dev/$parent == "$disk" ]] || die 'carve target %s is not a partition of %s (parent is %s).' "$part" "$disk" "${parent:-unknown}"
  fstype=$(blkid -o value -s TYPE "$part" 2>/dev/null || true)
  case $fstype in
    ntfs)           tool=ntfsresize ;;
    ext4|ext3|ext2) tool=resize2fs ;;
    btrfs)          tool=btrfs ;;
    swap)           tool=mkswap ;;
    *)              die 'carve does not support filesystem '\''%s'\'' on %s (only ntfs, ext4, btrfs, swap).' "${fstype:-none}" "$part" ;;
  esac
  command -v "$tool" >/dev/null 2>&1 || die 'carve of %s needs '\''%s'\'', which is not on the live image (ntfsresize ships in ntfsprogs).' "$fstype" "$tool"
  need_gib=$(( 2 + $(ryoku_min_root_gib) ))
  { [[ $take =~ ^[0-9]+$ ]] && (( take >= need_gib * 1024 )); } || die 'RYOKU_RESIZE_TAKE_MIB='\''%s'\'' must free at least %s GiB (2 GiB boot + %s GiB root) for Ryoku.' "${take}" "${need_gib}" "$(ryoku_min_root_gib)"
  ryoku_bitlocker_warn "$disk"
  ryoku_windows_faststartup_gate "$disk"
  log 'preflight carve: GPT ok, boot mode %s, will carve %s MiB out of %s (%s) with %s' "$RYOKU_RESOLVED_ESP_MODE" "${take}" "$part" "$fstype" "$tool"
}
