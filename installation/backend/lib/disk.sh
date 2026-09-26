#!/usr/bin/env bash
# Whole disk creates an ESP + root. Alongside keeps existing partitions and
# creates a 2 GiB FAT boot partition + root in free space; auto mode makes that
# boot partition an ESP when the existing ESP is too full, otherwise XBOOTLDR.
#
# The root floor covers the base closure plus swap and build/snapshot headroom.
ryoku_min_root_gib() { echo $(( 20 + ${RYOKU_SWAP_GIB:-0} )); }

# Both alongside modes use this distinct FAT label for kernel discovery.
RYOKU_ALONGSIDE_BOOT_MIB=2048
RYOKU_ALONGSIDE_BOOT_LABEL=RYOKUBOOT

# ryoku_release_previous_attempt: the failure EXIT trap deliberately LEAVES /mnt
# mounted so a failed install's partial tree + log can be inspected. but the TUI
# "retry" re-runs this backend in the SAME live session, so /mnt (and the
# swapfile the mount stage swapped on) is still held from the prior attempt. a
# held /mnt pins ROOT_DEV, so the partition/wipe below dies "Device or resource
# busy" and ryoku_reclaim_leftovers skips its still-mounted partitions -- the
# retry wedges. tear our own prior attempt down first: swapoff the installer's
# swapfile (a swapfile pins its fs, blocking the umount), recursively unmount
# /mnt, then close the prior attempt's /dev/mapper/root UNCONDITIONALLY -- an
# encrypted attempt can leave the mapper open even with /mnt already unmounted
# (failure between luks open and mount), and a mapper-held partition makes the
# alongside reclaim's partprobe die "unable to inform the kernel". best-effort +
# idempotent; a fresh run (nothing at /mnt, no mapper) no-ops.
ryoku_release_previous_attempt() {
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log "DRYRUN: if /mnt is a leftover mount from a prior attempt, would swapoff /mnt/swap/swapfile, umount -R /mnt, and close a stale /dev/mapper/root"
    return 0
  fi
  if ! mountpoint -q /mnt; then
    ryoku_free_mapper root
    return 0
  fi
  log "releasing /mnt left mounted by a previous install attempt (swapoff + umount -R)"
  swapoff /mnt/swap/swapfile 2>/dev/null || true
  umount -R /mnt 2>/dev/null || umount -l /mnt 2>/dev/null || true
  ryoku_free_mapper root
  udevadm settle 2>/dev/null || true
}

ryoku_partition() {
  # a TUI retry re-runs this backend with /mnt still mounted from the failed
  # attempt; free it before touching the disk (see ryoku_release_previous_attempt).
  ryoku_release_previous_attempt
  case ${RYOKU_DISK_STRATEGY:-} in
    whole)     ryoku_partition_whole ;;
    alongside) ryoku_partition_alongside ;;
    '')        die "RYOKU_DISK_STRATEGY is required (use 'whole' or 'alongside'); refusing to wipe disk on empty strategy." ;;
    *)         die 'disk strategy '\''%s'\'' not supported (use '\''whole'\'' or '\''alongside'\'')' "$RYOKU_DISK_STRATEGY" ;;
  esac
}

# ryoku_free_mapper closes a device-mapper node by name. ryoku_release_disk only
# reaches mappers lsblk ties to the disk; one an earlier run orphaned still owns
# the name and fails the later `cryptsetup open ... root` with "Device root
# already exists". no such node is a no-op.
ryoku_free_mapper() {
  local name=$1 node="/dev/mapper/$1" mp
  [[ -n $name ]] || return 0
  dmsetup info -- "$name" >/dev/null 2>&1 || return 0
  log 'freeing stale mapper /dev/mapper/%s held by a previous run' "$name"
  # unmount and swapoff first (a swapfile inside the fs pins the mapper), then close.
  while IFS= read -r mp; do
    [[ -n $mp ]] || continue
    run_sh "umount -R -- '$mp' 2>/dev/null || umount -l -- '$mp' 2>/dev/null || true"
  done < <(lsblk -nrpo MOUNTPOINT "$node" 2>/dev/null | awk 'NF' | sort -r)
  run_sh 'swapoff -a 2>/dev/null || true'
  run_sh "cryptsetup close -- '$name' 2>/dev/null || dmsetup remove --force -- '$name' 2>/dev/null || true"
  run_sh 'udevadm settle 2>/dev/null || true'
}

# ryoku_tree_mountpoints <mounts-file> <dev>...: every mountpoint in <mounts-file>
# (/proc/mounts format) whose source device is one of <dev>..., deepest path
# first. reads /proc/mounts, NOT lsblk MOUNTPOINT (which prints only ONE
# mountpoint per device), so all of a partition's subvol mounts come back;
# decodes the octal escapes /proc/mounts uses (\040 space, \011 tab, \134 \).
ryoku_tree_mountpoints() {
  local mounts=$1; shift
  [[ -r $mounts ]] || return 0
  (( $# )) || return 0
  local devset
  printf -v devset '%s\n' "$@"
  awk -v devs="$devset" '
    BEGIN { n = split(devs, a, "\n"); for (i = 1; i <= n; i++) if (a[i] != "") want[a[i]] = 1 }
    ($1 in want) {
      mp = $2
      gsub(/\\040/, " ", mp); gsub(/\\011/, "\t", mp); gsub(/\\134/, "\\", mp)
      print mp
    }
  ' "$mounts" | sort -ru
}

# ryoku_disk_swapfiles <swaps-file> <mountpoint>...: paths of file-type swap
# entries in <swaps-file> (/proc/swaps format) that sit under one of the given
# mountpoints -- so the installer's own /mnt/swap/swapfile (a FILE on @swap, not
# a swap-typed block device) is freed on a retry. block-device swaps are handled
# by the partition swapoff loop; this covers only the swapFILE case.
ryoku_disk_swapfiles() {
  local swaps=$1; shift
  [[ -r $swaps ]] || return 0
  (( $# )) || return 0
  local mpset
  printf -v mpset '%s\n' "$@"
  awk -v mps="$mpset" '
    BEGIN { cnt = split(mps, mp, "\n") }
    NR == 1 { next }
    $2 == "file" {
      f = $1
      gsub(/\\040/, " ", f); gsub(/\\011/, "\t", f); gsub(/\\134/, "\\", f)
      for (i = 1; i <= cnt; i++) {
        if (mp[i] == "") continue
        if (f == mp[i] || index(f, mp[i] "/") == 1) { print f; break }
      }
    }
  ' "$swaps"
}

# ryoku_release_disk = free the target so the wipe doesn't die "Device or
# resource busy". on the live medium something we didn't put there often holds
# the disk: udisks-automount, a stale /mnt, an open LUKS/dm mapper from a
# previous failed run, auto-enabled swap. kernel refuses to re-read or wipe a
# disk while ANY child is held, so tear it down leaves-first: swapoff, unmount,
# close dm/LUKS, vgchange -an, mdadm --stop, settle. scoped to the disk's own
# tree (lsblk "$disk"), never touches the live medium. best-effort + idempotent.
ryoku_release_disk() {
  local disk=$1
  [[ -b $disk || -n ${RYOKU_DRYRUN:-} ]] || return 0
  log 'releasing %s before wipe (swapoff, unmount, close holders)' "$disk"

  local mounts_src=${RYOKU_PROC_MOUNTS:-/proc/mounts}
  local swaps_src=${RYOKU_PROC_SWAPS:-/proc/swaps}

  # every device in the disk's own tree (the disk, its partitions, any dm/LUKS
  # child). the exact set matched against /proc/mounts + /proc/swaps below, so a
  # sibling disk is never touched. lsblk -p names a mapper child the same way
  # /proc/mounts does (/dev/mapper/<name>), so the string match is direct.
  local name
  local -a tree_devs=()
  while IFS= read -r name; do
    [[ -n $name ]] && tree_devs+=("$name")
  done < <(lsblk -nrpo NAME "$disk" 2>/dev/null)

  # this disk's mountpoints, deepest first (see ryoku_tree_mountpoints for why
  # /proc/mounts and not lsblk).
  local -a mps=()
  local mp
  while IFS= read -r mp; do
    [[ -n $mp ]] && mps+=("$mp")
  done < <(ryoku_tree_mountpoints "$mounts_src" "${tree_devs[@]}")

  # swapoff any swapFILE on one of those mountpoints BEFORE the umount below: a
  # live swapfile pins its fs and fails the unmount. this is the installer's own
  # /mnt/swap/swapfile on a retry -- it is a file, so it never appears in the
  # partition swapoff loop further down.
  local sf
  while IFS= read -r sf; do
    [[ -n $sf ]] || continue
    run_sh "swapoff -- '$sf' 2>/dev/null || true"
  done < <(ryoku_disk_swapfiles "$swaps_src" "${mps[@]}")

  # unmount every mountpoint on the disk, deepest first (a nested mount pins its
  # parent). lazy-unmount fallback so a busy mount still releases the device.
  for mp in "${mps[@]}"; do
    run_sh "umount -R -- '$mp' 2>/dev/null || umount -l -- '$mp' 2>/dev/null || true"
  done

  # swapoff any swap PARTITION on the disk (a swap-typed child device).
  while IFS= read -r name; do
    [[ -n $name ]] || continue
    run_sh "swapoff -- '$name' 2>/dev/null || true"
  done < <(lsblk -nrpo NAME,FSTYPE "$disk" 2>/dev/null | awk '$2=="swap"{print $1}')

  # close dm holders on the disk (LUKS/crypt, LVM, plain dm), leaves first so a
  # stacked setup unwinds. cryptsetup for crypt, dmsetup for the rest.
  while IFS= read -r name; do
    [[ -n $name ]] || continue
    run_sh "cryptsetup close -- '$name' 2>/dev/null || dmsetup remove --force -- '$name' 2>/dev/null || true"
  done < <(lsblk -nrpo NAME,TYPE "$disk" 2>/dev/null | awk '$2=="crypt"||$2=="lvm"||$2=="dm"{print $1}' | tac)

  # deactivate any LVM VG with a PV on the disk or one of its partitions. feed
  # pvs the exact tree devices, NEVER a "${disk}*" glob: /dev/sda* would also
  # match /dev/sdaa on a many-disk box and could kill a VG on another disk.
  if command -v pvs >/dev/null 2>&1 && (( ${#tree_devs[@]} > 0 )); then
    while IFS= read -r name; do
      [[ -n $name ]] || continue
      run_sh "vgchange -an -- '$name' 2>/dev/null || true"
    done < <(pvs --noheadings -o vg_name "${tree_devs[@]}" 2>/dev/null | awk 'NF' | sort -u)
  fi

  # stop any md RAID array with a member on the disk.
  while IFS= read -r name; do
    [[ -n $name ]] || continue
    run_sh "mdadm --stop -- '$name' 2>/dev/null || true"
  done < <(lsblk -nrpo NAME,TYPE "$disk" 2>/dev/null | awk '$2 ~ /raid/{print $1}' | tac)

  # a "root" mapper orphaned off the disk tree escapes the loops above; free it too.
  ryoku_free_mapper root

  run_sh 'udevadm settle 2>/dev/null || true'
}

# ryoku_wipe_signatures clears signatures off a device, retried: right after
# sgdisk zaps the GPT the kernel can briefly report the device busy while it
# drops the stale table, even with no holders left. settle + retry; the final
# attempt routes through run() so a real failure still aborts loudly (and is
# printed, not executed, under dry-run).
ryoku_wipe_signatures() {
  local target=$1
  if [[ -z ${RYOKU_DRYRUN:-} ]]; then
    for _ in 1 2; do
      wipefs --all "$target" 2>/dev/null && return 0
      udevadm settle 2>/dev/null || true
      sleep 1
    done
  fi
  run wipefs --all "$target"
}

ryoku_partition_whole() {
  local disk=$RYOKU_DISK
  local esp_end=$(( 1 + RYOKU_ESP_GIB * 1024 ))   # MiB: 1 MiB align gap + the ESP

  # destructive-wipe guard: refuse to zap a disk that already holds partitions
  # (e.g. a Windows install) unless the caller has explicitly acked. the TUI
  # only sets RYOKU_WIPE_CONFIRMED=1 after the typed "ERASE" ack on the Review
  # screen. a truly blank disk goes through without the token so a fresh install
  # is not gated on a second confirmation.
  # under dry-run the disk may be absent and ryoku_disk_populated fails closed,
  # so narrate the guard instead of probing; the real check stands in real mode.
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log 'DRYRUN: would refuse to wipe %s if it holds partitions and RYOKU_WIPE_CONFIRMED != 1' "$disk"
  elif [[ ${RYOKU_WIPE_CONFIRMED:-} != 1 ]] && ryoku_disk_populated "$disk"; then
    die 'refusing to wipe %s: it already holds partitions and RYOKU_WIPE_CONFIRMED is not set. Pick '\''alongside'\'' to keep them, or set RYOKU_WIPE_CONFIRMED=1 to wipe explicitly.' "$disk"
  fi

  log 'partitioning %s (whole disk, GPT: %sGiB ESP + root)' "$disk" "${RYOKU_ESP_GIB}"

  # free the disk before touching it: on the live medium the target may be held
  # by an auto-mounted partition (udisks), leftover state from a previous run,
  # or active swap. while a child holds the disk, sgdisk/wipefs fail "Device or
  # resource busy".
  ryoku_release_disk "$disk"

  # nuke the partition table, then re-read so the kernel drops the now-stale
  # partitions before wipefs probes the bare disk.
  run sgdisk --zap-all "$disk"
  run partprobe "$disk"
  run_sh 'udevadm settle 2>/dev/null || true'
  ryoku_wipe_signatures "$disk"

  # fresh GPT: partition 1 = ESP (EF00 == GPT 'esp' flag), partition 2 = root.
  run parted --script "$disk" mklabel gpt
  run parted --script "$disk" mkpart ESP fat32 1MiB "${esp_end}MiB"
  run parted --script "$disk" set 1 esp on
  run parted --script "$disk" mkpart root "${esp_end}MiB" 100%

  # let the kernel re-read the new table before touching the partitions.
  run partprobe "$disk"
  run_sh 'udevadm settle || true'

  ESP_DEV=$(part_dev "$disk" 1)
  ROOT_PART=$(part_dev "$disk" 2)

  # a fresh GPT can lay these partitions over an older layout, so a stale sig
  # (old LUKS2 header, previous btrfs) can still sit at the start of each. the
  # whole-disk wipefs above doesn't reach into partition space, so clear the new
  # partitions directly. otherwise blkid reports the old type (e.g. crypto_LUKS)
  # and the later mount fails with "unknown filesystem type".
  run wipefs --all "$ESP_DEV"
  run wipefs --all "$ROOT_PART"
  log 'ESP=%s root partition=%s' "$ESP_DEV" "$ROOT_PART"
}

ryoku_partition_alongside() {
  local disk=$RYOKU_DISK mode=${RYOKU_RESOLVED_ESP_MODE:-${RYOKU_ESP_MODE:-shared}}
  [[ $mode != auto ]] || die "alongside ESP mode is unresolved; preflight must select shared or dedicated."
  if [[ $mode == dedicated ]]; then
    log 'partitioning %s (alongside existing OS: dedicated 2GiB Ryoku ESP + root in free space; existing ESP untouched)' "$disk"
  else
    log 'partitioning %s (alongside existing OS: 2GiB XBOOTLDR + root in free space; existing ESP shared)' "$disk"
  fi

  # under dry-run the disk may not exist; narrate what we'd do and pick
  # plausible device names so the rest of the flow can be exercised.
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    local d_min d_max
    d_min=$(ryoku_min_root_gib)
    if [[ $mode == dedicated ]]; then
      log 'DRYRUN: would create a dedicated 2GiB Ryoku ESP + %sGiB root; the existing ESP stays untouched' "${d_min}"
    else
      log 'DRYRUN: would create a 2GiB XBOOTLDR + %sGiB root and share the existing ESP' "${d_min}"
    fi
    log "DRYRUN: with RYOKU_RECLAIM_LEFTOVERS=1 would reclaim UNMOUNTED leftover partitions labeled exactly ryoku/ryokuboot from a prior failed run; without the ack, existing such partitions abort the install"
    d_max=$(ryoku_max_partnum "$disk" 2>/dev/null || true)
    { [[ $d_max =~ ^[0-9]+$ ]] && (( d_max > 0 )); } || d_max=3
    ESP_DEV=$(part_dev "$disk" "$(( d_max + 1 ))")
    ROOT_PART=$(part_dev "$disk" "$(( d_max + 2 ))")
    log 'DRYRUN: new ryoku-boot=%s (2GiB, mode=%s, label ryokuboot) root=%s (label ryoku)' "$ESP_DEV" "$mode" "$ROOT_PART"
    return 0
  fi

  # UEFI dual-boot needs a GPT label. refuse MBR rather than guess at a remap.
  local pttype
  pttype=$(blkid -o value -s PTTYPE "$disk" 2>/dev/null || true)
  [[ $pttype == gpt ]] || die 'alongside needs a GPT disk; %s has '\''%s'\'' partition table. Use whole-disk, or convert to GPT.' "$disk" "${pttype:-no}"

  # reclaim leftovers of a previous failed run BEFORE measuring free space, so
  # the region they hold is available again and retries don't stack partitions.
  # gated on RYOKU_RECLAIM_LEFTOVERS=1 (the TUI's typed-ERASE ack): without it,
  # existing ryoku/ryokuboot partitions are fatal, not silently deleted.
  ryoku_reclaim_leftovers "$disk"

  # limine finds our kernels by the boot partition's FAT label, so that label
  # MUST be unique across the machine. if anything already carries it (a foreign
  # partition, a hand-built layout), refuse rather than create a second one and
  # let limine chainload the wrong volume. our own not-yet-created partition
  # can't match; a reclaimed leftover was already deleted above.
  local clash
  clash=$(lsblk -rpno NAME,LABEL 2>/dev/null | awk -v l="$RYOKU_ALONGSIDE_BOOT_LABEL" '$2==l{print $1}' | head -n1)
  [[ -z $clash ]] || die 'a partition (%s) already has the FAT label '\''%s'\'' that alongside needs for its boot partition; refusing to create a colliding label. Relabel or remove %s, then retry.' "$clash" "$RYOKU_ALONGSIDE_BOOT_LABEL" "$clash"

  # pick the free region. the TUI ran the same sfdisk probe and passes the chosen
  # region's exact sectors; a direct backend/test call falls back to the largest.
  # either way we RE-READ the live table and refuse a range that is not still
  # free -- the disk runs this once and must never write over Windows.
  local ss spm region_start region_end min_root need_gib region_mib
  ss=$(blockdev --getss "$disk"); spm=$(( 1048576 / ss ))
  min_root=$(ryoku_min_root_gib)
  need_gib=$(( 2 + min_root ))
  if [[ -n ${RYOKU_REGION_START:-} && -n ${RYOKU_REGION_END:-} ]]; then
    region_start=$RYOKU_REGION_START; region_end=$RYOKU_REGION_END
    ryoku_region_is_free "$disk" "$region_start" "$region_end" \
      || die 'requested region %s-%s is not inside a free area of %s (did the disk change since it was probed?); refusing to partition.' "${region_start}" "${region_end}" "$disk"
  else
    read -r region_start region_end _ < <(ryoku_free_regions "$disk" | sort -k3,3 -nr | head -n1) || true
    [[ -n ${region_start:-} && -n ${region_end:-} ]] \
      || die 'no unallocated region >= %sGiB on %s; shrink a Windows partition first, then retry.' "${need_gib}" "$disk"
  fi
  region_mib=$(( (region_end - region_start + 1) / spm ))
  (( region_mib >= need_gib * 1024 )) || die 'not enough free space on %s: %sGiB in the chosen region, need >= %sGiB (2GiB boot + %sGiB root). Shrink the Windows partition first, then retry.' "$disk" "$(( region_mib / 1024 ))" "${need_gib}" "${min_root}"

  # boot fills the region head; root fills the remainder of THE SAME region.
  # region_start/end are already 1 MiB-aligned by ryoku_free_regions, so every
  # boundary below stays aligned.
  local boot_start boot_end root_start
  boot_start=$region_start
  boot_end=$(( boot_start + RYOKU_ALONGSIDE_BOOT_MIB * spm - 1 ))
  root_start=$(( boot_end + 1 ))
  log 'alongside region: sectors %s-%s (%sGiB); boot %s-%s, root %s-%s' "${region_start}" "${region_end}" "$(( region_mib / 1024 ))" "${boot_start}" "${boot_end}" "${root_start}" "${region_end}"

  # snapshot the pre-existing partition set so we can prove (after sgdisk) that
  # BOTH new partitions landed in free space without overwriting an existing one.
  local -a pre_parts=()
  local p
  while IFS= read -r p; do
    [[ -n $p ]] && pre_parts+=("$p")
  done < <(ryoku_partitions "$disk")

  # Create the boot partition and root at explicit ranges. Shared mode uses an
  # XBOOTLDR (EA00); dedicated mode makes the same 2 GiB FAT partition an ESP
  # (EF00), so a full Windows ESP is never touched.
  local boot_type=ea00
  [[ $mode == dedicated ]] && boot_type=ef00
  run sgdisk \
    -n "0:${boot_start}:${boot_end}" -t "0:${boot_type}" -c 0:ryokuboot \
    -n "0:${root_start}:${region_end}" -t 0:8300 -c 0:ryoku \
    "$disk"
  run partprobe "$disk"
  run_sh 'udevadm settle || true'

  # the by-partlabel nodes can lag partprobe on a busy or slow bus (USB, Ventoy);
  # wait for our two freshly-labeled partitions to appear before mapping them, so
  # a slow udev never spuriously aborts a valid alongside layout. a wait only: if
  # they never show, the mapping below still fails loudly rather than guessing.
  for _ in 1 2 3 4 5; do
    [[ -e /dev/disk/by-partlabel/ryoku && -e /dev/disk/by-partlabel/ryokuboot ]] && break
    udevadm settle 2>/dev/null || true
    sleep 1
  done

  # the new partitions = current set minus the pre-existing set; must be exactly
  # two (the ESP + the root).
  local -a new_parts=()
  local seen q
  while IFS= read -r p; do
    [[ -n $p ]] || continue
    seen=0
    for q in "${pre_parts[@]}"; do [[ $q == "$p" ]] && { seen=1; break; }; done
    (( seen )) || new_parts+=("$p")
  done < <(ryoku_partitions "$disk")
  (( ${#new_parts[@]} == 2 )) || die 'alongside expected to create 2 new partitions (ESP + root) but sees %d (%s); refusing to continue.' "${#new_parts[@]}" "${new_parts[*]:-none}"

  # map the two new partitions to ESP/root by our exact GPT partlabels.
  ESP_DEV=""; ROOT_PART=""
  local lbl
  for p in "${new_parts[@]}"; do
    lbl=$(lsblk -dno PARTLABEL "$p" 2>/dev/null || true)
    case $lbl in
      ryokuboot) ESP_DEV=$p ;;
      ryoku)     ROOT_PART=$p ;;
    esac
  done
  [[ -n $ESP_DEV ]]   || die "alongside could not find the new ryoku-boot partition (partlabel ryokuboot) after sgdisk; refusing to continue."
  [[ -n $ROOT_PART ]] || die "alongside could not find the new Ryoku root (partlabel ryoku) after sgdisk; refusing to continue."
  [[ $ESP_DEV != "$ROOT_PART" ]] || die 'alongside boot and root resolved to the same device %s; refusing to continue.' "$ESP_DEV"

  # hard safety, applied to BOTH new partitions: each must be a real NEW block
  # device, must not be the disk itself, must not have existed before sgdisk, and
  # its parent must be the target disk. any failure = we're about to touch an
  # existing OS partition, so abort before any wipefs/mkfs.
  local dev parent disk_base=${disk##*/}
  for dev in "$ESP_DEV" "$ROOT_PART"; do
    [[ $dev != "$disk" ]] || die 'alongside partition resolves to disk %s; refusing to format.' "$disk"
    [[ -b $dev ]] || die 'alongside created a partition but %s is not a block device.' "$dev"
    for p in "${pre_parts[@]}"; do
      [[ $p != "$dev" ]] || die 'alongside partition %s existed before sgdisk; refusing to format an existing partition.' "$dev"
    done
    parent=$(lsblk -no PKNAME "$dev" 2>/dev/null | head -n1)
    [[ $parent == "$disk_base" ]] || die 'alongside partition %s parent='\''%s'\'' does not match disk '\''%s'\''; refusing to format.' "$dev" "$parent" "$disk_base"
  done

  # clear any stale sig in the two NEW partitions only (never the disk or any
  # existing partition), so a leftover LUKS/btrfs header at these offsets can't
  # fail the later mkfs/mount.
  run wipefs --all "$ESP_DEV"
  run wipefs --all "$ROOT_PART"
  log 'boot=%s (new ryoku-boot, mode=%s, /boot) root partition=%s' "$ESP_DEV" "$mode" "$ROOT_PART"
}

# ryoku_reclaim_leftovers deletes partitions that are VERIFIED failed-alongside
# debris (ryoku_is_leftover: GPT partlabel exactly ryoku/ryokuboot AND not a
# living system) and not mounted: leftovers of a previous failed run that would
# otherwise eat the free region and stack up on every retry. GATED: reclaim (a
# destructive delete) runs ONLY when RYOKU_RECLAIM_LEFTOVERS=1 -- the TUI sets it
# after the typed ERASE ack on the Review screen. without the ack, finding such
# debris is fatal: die listing it and the two ways forward rather than delete
# anything. a LIVING install (an @/etc btrfs, a kernel-bearing boot vfat) is NEVER
# reclaimed -- not even with the ack -- and a partition we cannot read is kept
# too (fail-closed). only our own exact labels, only verified debris, only when
# unmounted; anything else is untouched.
ryoku_reclaim_leftovers() {
  local disk=$1 p lbl mnt num info list
  local -a dnums=() dinfo=()
  # first pass: identify leftovers while the table is still stable (nothing
  # deleted yet). collect their numbers; do NOT delete mid-scan -- sgdisk
  # re-reads the table after each -d, which races the kernel's view of the
  # sibling partitions we still have to inspect.
  while IFS= read -r p; do
    [[ -n $p ]] || continue
    lbl=$(lsblk -dno PARTLABEL "$p" 2>/dev/null || true)
    [[ $lbl == ryoku || $lbl == ryokuboot ]] || continue
    mnt=$(lsblk -nrpo MOUNTPOINT "$p" 2>/dev/null | awk 'NF' | head -n1)
    if [[ -n $mnt ]]; then
      log 'leaving %s alone: labeled '\''%s'\'' but mounted at %s (not a leftover)' "$p" "$lbl" "$mnt"
      continue
    fi
    # living-install guard (defense in depth, same test the probe applies): a
    # real existing OS carrying our label is NOT debris, even with the ack.
    if ! ryoku_is_leftover "$p"; then
      log 'leaving %s alone: labeled '\''%s'\'' but it holds a living install (or could not be inspected); never reclaimed' "$p" "$lbl"
      continue
    fi
    num=$(part_num "$p")
    [[ $num =~ ^[0-9]+$ ]] || continue
    dnums+=("$num")
    dinfo+=("$p (GPT label '$lbl', partition $num)")
  done < <(ryoku_partitions "$disk")

  (( ${#dnums[@]} )) || return 0

  # safety gate: without the explicit ack these might be a healthy completed
  # Ryoku install, not a failed-run leftover. refuse to delete; list them and
  # the two ways forward.
  if [[ ${RYOKU_RECLAIM_LEFTOVERS:-} != 1 ]]; then
    list=""
    for info in "${dinfo[@]}"; do list+="  $info"$'\n'; done
    die 'existing Ryoku-labeled partition(s) on %s (a previous Ryoku install or a failed run):
%salongside will NOT delete these automatically -- they may be a working Ryoku install. To proceed, either:
  1) restart the installer so it rescans the disk, then confirm reclaim on the Review screen (the typed ERASE ack, which sets RYOKU_RECLAIM_LEFTOVERS=1; a mid-session retry keeps the pre-failure scan and never arms the ack), or
  2) delete or keep them yourself with another tool, then retry.' "$disk" "$list"
  fi

  for info in "${dinfo[@]}"; do
    log 'reclaiming leftover %s from a previous failed run' "$info"
  done
  # delete them all in ONE sgdisk call: a single table re-read at the end, so
  # removing one partition can't disturb the kernel's node for another.
  local -a delargs=()
  for num in "${dnums[@]}"; do delargs+=(-d "$num"); done
  run sgdisk "${delargs[@]}" "$disk"
  run partprobe "$disk"
  run_sh 'udevadm settle || true'
}

# ryoku_partitions: partition device paths on a disk, in table order.
ryoku_partitions() {
  lsblk -lnpo NAME,TYPE "$1" 2>/dev/null | awk '$2=="part"{print $1}'
}

# ryoku_disk_populated: 0 (true) when $1 has at least one visible partition, 1
# only when the disk IS visible AND has zero partitions. if the disk can't be
# read (missing device, broken GPT, no lsblk) we return 0 so the wipe guard
# fails closed: better to abort than wipe a disk we didn't fully introspect.
ryoku_disk_populated() {
  local disk=$1
  lsblk -dno NAME "$disk" >/dev/null 2>&1 || return 0
  local n
  n=$(lsblk -lnpo NAME,TYPE "$disk" 2>/dev/null | awk '$2=="part"' | wc -l)
  (( n > 0 ))
}

# ryoku_max_partnum: highest partition number on the disk (0 if none).
ryoku_max_partnum() {
  sgdisk -p "$1" 2>/dev/null | awk '/^[[:space:]]+[0-9]+[[:space:]]/{n=$1} END{print n+0}'
}

# ryoku_free_regions <disk>: one aligned free region per line as
#   START_SECTOR END_SECTOR SIZE_MIB
# for every gap >= 1024 MiB, read structurally from `sfdisk --json`. I stopped
# trusting parted here: dirty NTFS makes it lie, and archinstall crashes on the
# same disks (KPMcore reads the table with sfdisk for exactly this reason). start
# aligns UP to 1 MiB, end DOWN, in the disk's real sector size (512 and 4096 both
# correct). sfdisk emits firstlba/lastlba only for GPT (lastlba already excludes
# the backup GPT); a dos/MBR table has neither, so fall back to 1 MiB in and the
# whole-disk last sector, else free space on an MBR data disk is never listed.
ryoku_free_regions() {
  local disk=$1 json ss first last
  json=$(sfdisk --json "$disk" 2>/dev/null) || return 0
  [[ -n $json ]] || return 0
  read -r ss first last < <(printf '%s' "$json" | jq -r \
    '.partitiontable | "\(.sectorsize // 512) \(.firstlba // "-") \(.lastlba // "-")"')
  [[ $first == - ]] && first=$(( 1048576 / ss ))                                   # dos: 1 MiB in
  [[ $last == - ]] && last=$(( $(blockdev --getsize64 "$disk" 2>/dev/null || echo 0) / ss - 1 ))
  printf '%s\n' "$json" | jq -r --arg ss "$ss" --arg first "$first" --arg last "$last" '
    "meta \($ss) \($first) \($last)",
    (.partitiontable.partitions[]? | "part \(.start) \(.size)")
  ' | awk '
    function emit(gs, ge,   as, ae, mib) {
      as = int((gs + spm - 1) / spm) * spm         # align start up to 1 MiB
      ae = int((ge + 1) / spm) * spm - 1            # align end down to 1 MiB
      if (ae <= as) return
      mib = (ae - as + 1) / spm
      if (mib >= 1024) printf "%d %d %d\n", as, ae, mib
    }
    $1 == "meta" { ss = $2; first = $3; last = $4; spm = 1048576 / ss; next }
    $1 == "part" { n++; ps[n] = $2; pe[n] = $2 + $3 - 1; next }
    END {
      for (i = 2; i <= n; i++) {                    # insertion sort by start; n is tiny
        a = ps[i]; b = pe[i]; j = i - 1
        while (j >= 1 && ps[j] > a) { ps[j+1] = ps[j]; pe[j+1] = pe[j]; j-- }
        ps[j+1] = a; pe[j+1] = b
      }
      cur = first
      for (i = 1; i <= n; i++) {
        if (ps[i] > cur) emit(cur, ps[i] - 1)
        if (pe[i] + 1 > cur) cur = pe[i] + 1
      }
      emit(cur, last)
    }
  '
}

# ryoku_region_is_free <disk> <start> <end>: 0 when [start,end] sits fully inside
# one free region sfdisk reports RIGHT NOW. the TUI passes sectors it computed
# from the same probe, but the disk is precious: re-read and refuse a range that
# is no longer free (a partition changed under us since the probe).
ryoku_region_is_free() {
  local disk=$1 s=$2 e=$3
  ryoku_free_regions "$disk" | awk -v s="$s" -v e="$e" '
    (s + 0 >= $1 + 0 && e + 0 <= $2 + 0) { found = 1; exit }
    END { exit(found ? 0 : 1) }
  '
}

# ryoku_windows_esp <disk>: print the EF00 partition containing /EFI/Microsoft.
# Probes candidate ESPs read-only and returns non-zero when none matches.
ryoku_windows_esp() {
  local disk=$1 p typ tmpd found="" hit
  tmpd=$(mktemp -d) || return 1
  while IFS= read -r p; do
    [[ -n $p ]] || continue
    typ=$(lsblk -dno PARTTYPE "$p" 2>/dev/null || true)
    [[ ${typ,,} == c12a7328-f81f-11d2-ba4b-00a0c93ec93b ]] || continue
    if mount -o ro "$p" "$tmpd" 2>/dev/null; then
      hit=$(find "$tmpd" -maxdepth 2 -type d -ipath '*/efi/microsoft' -print -quit 2>/dev/null || true)
      [[ -n $hit ]] && found=$p
      umount "$tmpd" 2>/dev/null || true
    fi
    [[ -n $found ]] && break
  done < <(ryoku_partitions "$disk")
  rmdir "$tmpd" 2>/dev/null || true
  [[ -n $found ]] || return 1
  printf '%s\n' "$found"
}

# ryoku_btrfs_has_system <partdev>: does the btrfs on <partdev> carry a living
# system -- an @ subvolume that actually holds /etc? mounts the TOP volume
# (subvolid=5) read-only so @ is visible even when the fs default subvol is @
# itself; never writes; always unmounts. exit: 0 living, 1 verified empty (no
# @/etc), 2 could-not-inspect. callers treat 2 as "keep" (fail-closed).
ryoku_btrfs_has_system() {
  local dev=$1 tmpd rc=2
  tmpd=$(mktemp -d) || return 2
  if mount -o ro,subvolid=5 "$dev" "$tmpd" 2>/dev/null; then
    if [[ -d $tmpd/@/etc ]]; then rc=0; else rc=1; fi
    umount "$tmpd" 2>/dev/null || true
  fi
  rmdir "$tmpd" 2>/dev/null || true
  return $rc
}

# ryoku_vfat_has_boot <partdev>: does the vfat on <partdev> hold anything that
# makes it a live boot volume -- a kernel (vmlinuz*) or an EFI tree? read-only
# mount, always unmounted. exit: 0 has boot content, 1 verified empty/kernel-less,
# 2 could-not-inspect (treated as "keep").
ryoku_vfat_has_boot() {
  local dev=$1 tmpd rc=2 hit
  tmpd=$(mktemp -d) || return 2
  if mount -o ro "$dev" "$tmpd" 2>/dev/null; then
    hit=$(find "$tmpd" -maxdepth 2 \( -iname 'vmlinuz*' -o -iname 'initramfs*' -o -ipath '*/efi/*' \) -print -quit 2>/dev/null || true)
    if [[ -n $hit ]]; then rc=0; else rc=1; fi
    umount "$tmpd" 2>/dev/null || true
  fi
  rmdir "$tmpd" 2>/dev/null || true
  return $rc
}

# ryoku_is_leftover <partdev>: 0 only when <partdev> is VERIFIED failed-alongside
# debris we may reclaim -- GPT partlabel exactly ryoku/ryokuboot AND not a living
# system: ryokuboot with no fs or an empty/kernel-less vfat; ryoku with no fs or a
# btrfs lacking an @/etc install. a living install (or any partition we cannot
# read) is NEVER a leftover -> return 1 (keep). this is the single test the probe
# and the reclaim step both apply, so the disk is judged identically in both.
ryoku_is_leftover() {
  local dev=$1 lbl fstype rc
  lbl=$(lsblk -dno PARTLABEL "$dev" 2>/dev/null || true)
  [[ $lbl == ryoku || $lbl == ryokuboot ]] || return 1
  fstype=$(blkid -o value -s TYPE "$dev" 2>/dev/null || true)
  if [[ $lbl == ryokuboot ]]; then
    [[ -z $fstype ]] && return 0
    [[ $fstype == vfat ]] || return 1
    ryoku_vfat_has_boot "$dev"; rc=$?
    [[ $rc -eq 1 ]] && return 0
    return 1
  fi
  [[ -z $fstype ]] && return 0
  [[ $fstype == btrfs ]] || return 1
  ryoku_btrfs_has_system "$dev"; rc=$?
  [[ $rc -eq 1 ]] && return 0
  return 1
}

# ryoku_disk_os_kind <disk>: name the existing OS by its living root filesystem
# when the ESP itself is ambiguous. a btrfs @/etc install labeled 'ryoku' is
# ryoku; any other living root (labeled btrfs, or ext with /etc) is linux; a disk
# with no recognizable install prints nothing and returns 1.
ryoku_disk_os_kind() {
  local disk=$1 p fstype lbl rc tmpd linux_found=""
  while IFS= read -r p; do
    [[ -n $p ]] || continue
    fstype=$(blkid -o value -s TYPE "$p" 2>/dev/null || true)
    case $fstype in
      btrfs)
        ryoku_btrfs_has_system "$p"; rc=$?
        if [[ $rc -eq 0 ]]; then
          lbl=$(blkid -o value -s LABEL "$p" 2>/dev/null || true)
          [[ -n $lbl ]] || lbl=$(lsblk -dno PARTLABEL "$p" 2>/dev/null || true)
          if [[ ${lbl,,} == ryoku ]]; then printf 'ryoku\n'; return 0; fi
          linux_found=1
        fi
        ;;
      ext4|ext3|ext2)
        tmpd=$(mktemp -d) || continue
        if mount -o ro "$p" "$tmpd" 2>/dev/null; then
          [[ -d $tmpd/etc ]] && linux_found=1
          umount "$tmpd" 2>/dev/null || true
        fi
        rmdir "$tmpd" 2>/dev/null || true
        ;;
    esac
  done < <(ryoku_partitions "$disk")
  [[ -n $linux_found ]] && { printf 'linux\n'; return 0; }
  return 1
}

# ryoku_esp_scan <disk>: classify the selected existing EF00 ESP and name its
# EFI binary for chainloading. Prefers a Windows ESP if any; otherwise
# the ESP's own bootloader markers decide (limine/ryoku, then a foreign vendor
# dir), and an empty/ambiguous ESP is classified by the disk's living root fs.
# prints "<dev> <windows|ryoku|linux> <existing_boot|->" (existing_boot '-' means
# nothing bootable to chainload); returns 1 when the disk has no EF00 ESP.
ryoku_esp_scan() {
  local disk=$1 p typ tmpd kind boot v out="" firstout=""
  tmpd=$(mktemp -d) || return 1
  while IFS= read -r p; do
    [[ -n $p ]] || continue
    typ=$(lsblk -dno PARTTYPE "$p" 2>/dev/null || true)
    [[ ${typ,,} == c12a7328-f81f-11d2-ba4b-00a0c93ec93b ]] || continue
    mount -o ro "$p" "$tmpd" 2>/dev/null || continue
    kind=""; boot="-"
    if [[ -d $tmpd/EFI/Microsoft ]]; then
      kind=windows
      [[ -f $tmpd/EFI/Microsoft/Boot/bootmgfw.efi ]] && boot=/EFI/Microsoft/Boot/bootmgfw.efi
    elif [[ -f $tmpd/limine.conf || -f $tmpd/EFI/limine/limine_x64.efi ]]; then
      kind=ryoku
      if [[ -f $tmpd/EFI/limine/limine_x64.efi ]]; then boot=/EFI/limine/limine_x64.efi
      elif [[ -f $tmpd/EFI/BOOT/BOOTX64.EFI ]]; then boot=/EFI/BOOT/BOOTX64.EFI; fi
    else
      v=$(find "$tmpd/EFI" -mindepth 2 -maxdepth 2 -iname '*.efi' \
            ! -ipath '*/EFI/Microsoft/*' ! -ipath '*/EFI/BOOT/*' -printf '/EFI/%P\n' 2>/dev/null | sort | head -n1 || true)
      if [[ -n $v ]]; then
        kind=linux; boot=$v
      else
        kind=$(ryoku_disk_os_kind "$disk" || true)
        [[ -n $kind ]] || kind=linux
        [[ -f $tmpd/EFI/BOOT/BOOTX64.EFI ]] && boot=/EFI/BOOT/BOOTX64.EFI
      fi
    fi
    umount "$tmpd" 2>/dev/null || true
    if [[ $kind == windows ]]; then out="$p $kind $boot"; break; fi
    [[ -n $firstout ]] || firstout="$p $kind $boot"
  done < <(ryoku_partitions "$disk")
  rmdir "$tmpd" 2>/dev/null || true
  [[ -n $out ]] || out=$firstout
  [[ -n $out ]] || return 1
  printf '%s\n' "$out"
}

ryoku_esp_free_kib() {
  local dev=$1 tmpd avail=0
  tmpd=$(mktemp -d) || { printf '0\n'; return; }
  if mount -o ro "$dev" "$tmpd" 2>/dev/null; then
    avail=$(df -k --output=avail "$tmpd" 2>/dev/null | tail -1 | tr -d ' ')
    umount "$tmpd" 2>/dev/null || true
  fi
  rmdir "$tmpd" 2>/dev/null || true
  [[ $avail =~ ^[0-9]+$ ]] || avail=0
  printf '%s\n' "$avail"
}

# ryoku_emit_leftovers <disk>: one `leftover <dev> <partlabel> <sizeMiB>` line per
# VERIFIED failed-alongside debris partition (ryoku_is_leftover). read-only.
ryoku_emit_leftovers() {
  local disk=$1 p lbl sz
  while IFS= read -r p; do
    [[ -n $p ]] || continue
    lbl=$(lsblk -dno PARTLABEL "$p" 2>/dev/null || true)
    [[ $lbl == ryoku || $lbl == ryokuboot ]] || continue
    ryoku_is_leftover "$p" || continue
    sz=$(( $(blockdev --getsize64 "$p" 2>/dev/null || echo 0) / 1048576 ))
    printf 'leftover %s %s %s\n' "$p" "$lbl" "$sz"
  done < <(ryoku_partitions "$disk")
}

# ryoku_any_shrinkable <disk>: 0 when at least one partition can give up space
# (ryoku_part_shrink_info reports 'yes'). used by the alongside verdict: a fully
# allocated disk still qualifies when a partition is carveable.
ryoku_any_shrinkable() {
  local disk=$1 p fstype size_mib info
  while IFS= read -r p; do
    [[ -n $p ]] || continue
    fstype=$(blkid -o value -s TYPE "$p" 2>/dev/null || true)
    size_mib=$(( $(blockdev --getsize64 "$p" 2>/dev/null || echo 0) / 1048576 ))
    info=$(ryoku_part_shrink_info "$p" "$fstype" "$size_mib")
    [[ $(awk '{print $3}' <<<"$info") == yes ]] && return 0
  done < <(ryoku_partitions "$disk")
  return 1
}

# ryoku_probe_alongside <disk>: read-only report the TUI renders. machine lines:
#   sectorsize <bytes>
#   esp_kind windows|ryoku|linux    kind of the selected existing ESP
#   existing_boot <path>|none       EFI binary of the existing system to chainload
#   esp <device>                    selected existing ESP
#   region <start> <end> <mib>      zero or more, largest first
#   leftover <dev> <label> <mib>    zero or more, VERIFIED failed-run debris only
#   verdict ok|none|no-gpt|no-esp|error|create-esp
#   message <text>                  present on every non-ok verdict
# lines are ADDED, never reordered, so the TUI's keyword parser stays stable.
# sectorsize + region are reported for ANY readable disk (even non-GPT/ESP-less)
# so a prepared secondary disk isn't shown as blank; the verdict still gates it.
# ok = a usable existing ESP + a free region or shrinkable partition. create-esp =
# GPT, free space, but no ESP: Ryoku makes a dedicated one in the free space.
ryoku_probe_alongside() {
  local disk=$1 pttype ss regions espinfo esp esp_kind esp_boot shrinkable=no esp_found=no readable=no
  [[ -b $disk ]] || { printf 'verdict error\nmessage %s is not a block device\n' "$disk"; return 0; }
  pttype=$(blkid -o value -s PTTYPE "$disk" 2>/dev/null || true)
  ss=$(blockdev --getss "$disk" 2>/dev/null || echo 512)
  printf 'sectorsize %s\n' "$ss"
  # Report free space for every readable disk before the gate below, so a prepared
  # secondary disk isn't blank; the verdict (unchanged) still keeps it non-target.
  sfdisk --json "$disk" >/dev/null 2>&1 && readable=yes
  [[ $readable == yes ]] && regions=$(ryoku_free_regions "$disk" | sort -k3,3 -nr)
  # esp_scan reads via lsblk+mount, so it holds even when sfdisk can't parse.
  if [[ $pttype == gpt ]] && espinfo=$(ryoku_esp_scan "$disk"); then
    esp_found=yes
    read -r esp esp_kind esp_boot <<<"$espinfo"
    printf 'esp_kind %s\n' "$esp_kind"
    [[ $esp_boot == - ]] && esp_boot=none
    printf 'existing_boot %s\n' "$esp_boot"
    printf 'esp %s\n' "$esp"
    printf 'esp_count %s\n' "$(sgdisk -p "$disk" 2>/dev/null | awk '$6=="EF00"' | wc -l | tr -d ' ')"
    printf 'esp_free_kib %s\n' "$(ryoku_esp_free_kib "$esp")"
  fi
  [[ -n $regions ]] && printf '%s\n' "$regions" | while read -r s e m; do printf 'region %s %s %s\n' "$s" "$e" "$m"; done
  # Leftovers are matched by partlabel, so they are found with or without an ESP:
  # a create-esp disk can carry debris from a failed run too.
  [[ $readable == yes ]] && ryoku_emit_leftovers "$disk"
  # Verdict gate. alongside runs on a GPT disk with somewhere to place us and an
  # ESP: an existing one to share/identify, or -- on a disk with free space but no
  # ESP of its own -- a dedicated one Ryoku creates in that free space (verdict
  # create-esp; nothing existing is touched). Size sufficiency is gated downstream.
  if [[ $pttype != gpt ]]; then
    printf 'verdict no-gpt\nmessage %s has a '\''%s'\'' partition table; alongside needs GPT, so its free space is listed but cannot be used as an install target here. Use whole-disk, or convert this disk to GPT.\n' "$disk" "${pttype:-none}"
    return 0
  fi
  if [[ $readable != yes ]]; then
    printf 'verdict error\nmessage could not read the partition table on %s (sfdisk failed); the disk may be unreadable or lack a usable GPT. This is not a free-space problem.\n' "$disk"
    return 0
  fi
  if [[ $esp_found != yes ]]; then
    if [[ -n $regions ]]; then
      printf 'verdict create-esp\nmessage no existing EFI System Partition on %s; Ryoku will create a dedicated 2 GiB ESP plus its root in the free space and leave the existing partitions untouched.\n' "$disk"
    else
      printf 'verdict no-esp\nmessage no EFI System Partition on %s and no free space to create one; free space by shrinking a partition, or use whole-disk.\n' "$disk"
    fi
    return 0
  fi
  [[ -z $regions ]] && ryoku_any_shrinkable "$disk" && shrinkable=yes
  if [[ -n $regions || $shrinkable == yes ]]; then
    printf 'verdict ok\n'
  else
    printf 'verdict none\nmessage no unallocated region and no shrinkable partition on %s; free space by shrinking an existing partition, then retry.\n' "$disk"
  fi
}

# ryoku_human_to_mib <value>: convert a btrfs-style size ("144.00MiB", "1.17GiB",
# "160.00KiB", or a bare byte count) to whole MiB, rounding up. used by the btrfs
# branch of the resize probe, which reads `btrfs filesystem show` unmounted.
ryoku_human_to_mib() {
  local v=$1
  [[ -n $v ]] || { printf '0'; return 0; }
  awk -v v="$v" 'BEGIN{
    n=v; sub(/[KMGTPE]iB$/,"",n)
    u=v; sub(/^[0-9.]+/,"",u)
    m=(u=="KiB")?n/1024:(u=="MiB")?n:(u=="GiB")?n*1024:(u=="TiB")?n*1048576:(u=="PiB")?n*1073741824:n/1048576
    if (m<0) m=0
    printf "%d", m + 0.999999
  }'
}

# ryoku_part_label <partdev>: a single whitespace-free label token for the probe
# line. fs LABEL first, then GPT PARTLABEL, else "-"; internal whitespace becomes
# underscores so a "Basic data partition" name can't shift the positional fields.
ryoku_part_label() {
  local dev=$1 lbl
  lbl=$(blkid -o value -s LABEL "$dev" 2>/dev/null || true)
  [[ -n $lbl ]] || lbl=$(lsblk -dno PARTLABEL "$dev" 2>/dev/null || true)
  lbl=$(printf '%s' "$lbl" | tr -s '[:space:]' '_')
  lbl=${lbl#_}; lbl=${lbl%_}
  [[ -n $lbl ]] || lbl='-'
  printf '%s' "$lbl"
}

# ryoku_ntfs_info_state <ntfsresize --info output>: clean, dirty, or unknown.
# dirty wins over a resize line on purpose: a volume that is hibernated or
# scheduled for a check must never be shrunk or repartitioned around, whatever
# else the probe happens to print.
ryoku_ntfs_info_state() {
  if grep -qiE 'scheduled for check|volume is dirty|unsafe state|hibernat' <<<"$1"; then
    printf 'dirty'
  elif grep -q 'You might resize at' <<<"$1"; then
    printf 'clean'
  else
    printf 'unknown'
  fi
}

# ryoku_dirty_ntfs_on <disk>: every NTFS partition on the disk whose volume is
# hibernated or dirty, one device per line, read-only. Windows Fast Startup and
# hybrid shutdown leave a volume in this state, and writing a partition table
# under one is what invites Windows Startup Repair to "fix" the disk on its next
# boot -- a repair that has deleted fresh Linux partitions outright.
ryoku_dirty_ntfs_on() {
  local disk=$1 p fstype info
  command -v ntfsresize >/dev/null 2>&1 || return 0
  while IFS= read -r p; do
    [[ -n $p ]] || continue
    fstype=$(blkid -o value -s TYPE "$p" 2>/dev/null || true)
    [[ $fstype == ntfs ]] || continue
    info=$(ntfsresize --info "$p" 2>&1 || true)
    [[ $(ryoku_ntfs_info_state "$info") == dirty ]] && printf '%s\n' "$p"
  done < <(ryoku_partitions "$disk")
  return 0
}

# ryoku_part_shrink_info <partdev> <fstype> <sizeMiB>: judge ONE partition's
# shrinkability, strictly read-only, and print "<usedMiB> <minMiB> <shrinkable> <reason...>".
# usedMiB/minMiB are -1 when unknown or not shrinkable. the resize probe and the
# carve re-verify both call this, so a partition is judged the same in both places.
ryoku_part_shrink_info() {
  local dev=$1 fstype=$2 size_mib=$3
  local used=-1 min=-1 shrink=no reason
  case $fstype in
    ntfs)
      local info state min_bytes margin
      info=$(ntfsresize --info "$dev" 2>&1 || true)
      state=$(ryoku_ntfs_info_state "$info")
      case $state in
        clean)
          min_bytes=$(grep -o 'You might resize at [0-9]\+ bytes' <<<"$info" | grep -o '[0-9]\+' | head -n1)
          [[ -n $min_bytes ]] || min_bytes=0
          used=$(( (min_bytes + 1048575) / 1048576 ))
          margin=$(( size_mib / 10 )); (( margin < 1024 )) && margin=1024
          min=$(( used + margin ))
          if (( min < size_mib )); then shrink=yes; reason="clean NTFS"; else reason="NTFS already at its minimum size"; fi
          ;;
        dirty)
          reason="NTFS is dirty (hibernated or Fast Startup): boot Windows, disable Fast Startup, full shutdown"
          ;;
        *)
          if command -v ntfsresize >/dev/null 2>&1; then
            reason="NTFS could not be probed (ntfsresize --info failed)"
          else
            reason="ntfsresize (ntfsprogs) not installed"
          fi
          ;;
      esac
      ;;
    ext4|ext3|ext2)
      local state bs blkcount freeblk mblocks
      state=$(dumpe2fs -h "$dev" 2>/dev/null | sed -n 's/^Filesystem state:[[:space:]]*//p' | head -n1)
      if [[ $state != clean ]]; then
        reason="${fstype} is not clean (${state:-unknown}): run e2fsck -f first"
      else
        bs=$(dumpe2fs -h "$dev" 2>/dev/null | sed -n 's/^Block size:[[:space:]]*//p' | head -n1)
        blkcount=$(dumpe2fs -h "$dev" 2>/dev/null | sed -n 's/^Block count:[[:space:]]*//p' | head -n1)
        freeblk=$(dumpe2fs -h "$dev" 2>/dev/null | sed -n 's/^Free blocks:[[:space:]]*//p' | head -n1)
        mblocks=$(resize2fs -P "$dev" 2>/dev/null | sed -n 's/.*:[[:space:]]*//p' | head -n1)
        [[ $blkcount =~ ^[0-9]+$ && $freeblk =~ ^[0-9]+$ && $bs =~ ^[0-9]+$ ]] \
          && used=$(( (blkcount - freeblk) * bs / 1048576 ))
        if [[ $bs =~ ^[0-9]+$ && $mblocks =~ ^[0-9]+$ ]]; then
          min=$(( (mblocks * bs + 1048575) / 1048576 ))
          min=$(( min + (min + 9) / 10 ))
          if (( min < size_mib )); then shrink=yes; reason="${fstype}"; else reason="${fstype} already near its minimum size"; fi
        elif (( used >= 0 )); then
          # resize2fs -P refuses any fs without a FRESH fsck stamp, which is
          # every root that has been mounted since its last check - so estimate
          # the floor from used blocks with a fatter margin. The carve itself
          # runs e2fsck -fy first and the real resize2fs still enforces the
          # true minimum, so this coarser probe number only shapes the slider.
          min=$(( used + (used + 4) / 5 + 512 ))
          if (( min < size_mib )); then shrink=yes; reason="${fstype} (estimated; verified before any change)"; else reason="${fstype} already near its minimum size"; fi
        else
          reason="${fstype} minimum size could not be estimated"
        fi
      fi
      ;;
    btrfs)
      local show total used_h alloc_h used_m alloc_m
      show=$(btrfs filesystem show "$dev" 2>/dev/null || true)
      total=$(grep -o 'Total devices [0-9]\+' <<<"$show" | grep -o '[0-9]\+' | head -n1)
      if [[ -z $show ]]; then
        reason="btrfs could not be probed"
      elif [[ ${total:-1} != 1 ]]; then
        reason="multi-device btrfs: shrink unsupported"
      else
        used_h=$(grep -o 'FS bytes used [0-9.]\+[KMGTPE]iB' <<<"$show" | awk '{print $NF}' | head -n1)
        alloc_h=$(sed -n 's/.*devid.* used \([0-9.]\+[KMGTPE]iB\) .*/\1/p' <<<"$show" | head -n1)
        used_m=$(ryoku_human_to_mib "$used_h")
        alloc_m=$(ryoku_human_to_mib "$alloc_h")
        used=$used_m
        # btrfs cannot shrink below the highest allocated chunk without a balance,
        # so the honest floor is max(used + 15%, currently allocated), not used+15%.
        min=$(( used_m + (used_m * 15 + 99) / 100 ))
        (( min < alloc_m )) && min=$alloc_m
        if (( min < size_mib )); then shrink=yes; reason="single-device btrfs"; else reason="btrfs already near its minimum size"; fi
      fi
      ;;
    swap)
      used=0; min=8; shrink=yes; reason="swap (recreated smaller, UUID kept)"
      ;;
    crypto_LUKS)
      reason="LUKS: shrinking encrypted volumes is a documented follow-up (decrypt, or use whole-disk)"
      ;;
    BitLocker)
      reason="BitLocker: decrypt in Windows first"
      ;;
    vfat)
      reason="boot partition (ESP): kept as is"
      ;;
    '')
      reason="no filesystem"
      ;;
    *)
      reason="${fstype}: cannot be shrunk"
      ;;
  esac
  printf '%s %s %s %s\n' "$used" "$min" "$shrink" "$reason"
}

# ryoku_probe_resize <disk>: the read-only report the TUI's carve view renders.
# a superset of `probe alongside`: the same sectorsize/esp/region lines, PLUS one
# `part` line per partition describing what it can give up. frozen format lives in
# .superpowers/sdd/resize-probe-format.txt. a fully allocated disk (no region
# lines) is still verdict ok -- that is exactly the disk carve exists for.
ryoku_probe_resize() {
  local disk=$1 pttype ss esp regions p idx fstype label size_mib info
  [[ -b $disk ]] || { printf 'verdict error\nmessage %s is not a block device\n' "$disk"; return 0; }
  pttype=$(blkid -o value -s PTTYPE "$disk" 2>/dev/null || true)
  if [[ $pttype != gpt ]]; then
    printf 'verdict no-gpt\nmessage %s has a '\''%s'\'' partition table; carve needs GPT. Use whole-disk, or convert to GPT.\n' "$disk" "${pttype:-none}"
    return 0
  fi
  if ! sfdisk --json "$disk" >/dev/null 2>&1; then
    printf 'verdict error\nmessage could not read the partition table on %s (sfdisk failed); the disk may be unreadable or lack a usable GPT.\n' "$disk"
    return 0
  fi
  ss=$(blockdev --getss "$disk" 2>/dev/null || echo 512)
  printf 'sectorsize %s\n' "$ss"
  if esp=$(ryoku_windows_esp "$disk"); then printf 'esp %s\n' "$esp"; fi
  regions=$(ryoku_free_regions "$disk" | sort -k3,3 -nr)
  [[ -n $regions ]] && printf '%s\n' "$regions" | while read -r s e m; do printf 'region %s %s %s\n' "$s" "$e" "$m"; done
  while IFS= read -r p; do
    [[ -n $p ]] || continue
    idx=$(part_num "$p")
    fstype=$(blkid -o value -s TYPE "$p" 2>/dev/null || true)
    label=$(ryoku_part_label "$p")
    size_mib=$(( $(blockdev --getsize64 "$p" 2>/dev/null || echo 0) / 1048576 ))
    info=$(ryoku_part_shrink_info "$p" "$fstype" "$size_mib")
    printf 'part %s %s %s %s %s %s\n' "$p" "$idx" "${fstype:--}" "$label" "$size_mib" "$info"
  done < <(ryoku_partitions "$disk")
  printf 'verdict ok\n'
}
