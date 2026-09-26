#!/usr/bin/env bash
# shellcheck shell=bash
# Limine: install + brand, build the initramfs, enable the services the
# desktop needs. branding + templates come from system/boot/ (owned by the
# boot engineer); this step deploys them and fills in the dynamic bits
# (root cmdline, encrypt/nvidia toggles) only known here.
#
# layout contract (matches limine-entry-tool, the stack behind
# limine-mkinitcpio-hook and limine-snapper-sync):
#   /boot/limine.conf              THE config. branding globals + entries.
#                                  the tool regenerates entries here and
#                                  limine-snapper-sync adds the Snapshots
#                                  submenu here, preserving our globals.
#   EFI/limine/limine_x64.efi      the booted binary. limine-install (the
#                                  tool's pacman hook) refreshes this exact
#                                  path on every limine upgrade, so the
#                                  firmware never boots a stale bootloader.
#   EFI/BOOT/BOOTX64.EFI           removable-path fallback, same refresh.
# any limine.conf in another search location (/boot/limine/, EFI/limine/,
# ...) shadows the entries file -- Limine stops at the first match -- so
# those candidates are actively removed.

ryoku_bootloader() {
  CMDLINE=$(ryoku_cmdline)
  log 'kernel cmdline: %s quiet splash' "$CMDLINE"

  ryoku_boot_plymouth
  ryoku_boot_default_limine

  # Intel VMD carry-over MUST land before the initramfs is built (both paths
  # below), or the installed system can't find its own NVMe at boot.
  ryoku_boot_vmd

  if [[ ${RYOKU_DISK_STRATEGY:-} == alongside && ${RYOKU_RESOLVED_ESP_MODE:-${RYOKU_ESP_MODE:-shared}} != dedicated ]]; then
    ryoku_bootloader_alongside
  else
    ryoku_bootloader_own_esp
  fi

  log "enabling services: sddm, NetworkManager, bluetooth, rtkit, power-profiles-daemon"
  run arch-chroot /mnt systemctl enable sddm.service NetworkManager.service bluetooth.service rtkit-daemon.service power-profiles-daemon.service
}

ryoku_bootloader_own_esp() {
  if chroot_has limine-mkinitcpio; then
    log "building UKI via limine-mkinitcpio"
    ryoku_boot_limine_conf branding_only
    run arch-chroot /mnt limine-mkinitcpio
    chroot_has limine-update && run arch-chroot /mnt limine-update
  else
    log "building initramfs via mkinitcpio -P"
    ryoku_boot_limine_conf with_entry
    run arch-chroot /mnt /usr/bin/mkinitcpio -P
  fi
  ryoku_windows_entry
  ryoku_dedicated_existing_entry
  ryoku_boot_install_efi
}

# finalize: runs after the AUR step, when the limine hooks may have landed.
# WHOLE DISK: the hooks rebuilt /boot/limine.conf -- older limine-entry-tool
# writes a standalone /+Ryoku UKI tree (our flat placeholder is then clutter),
# 1.37+ adopts the placeholder as the tree root and nests the "//<kernel>"
# entries under it (nothing to drop). either way entry 1 becomes a directory,
# which can't autoboot, so default_entry moves onto the newest UKI. offline (no
# hook) keeps the flat entry + default_entry 1. ALONGSIDE is handled separately
# (ryoku_bootloader_finalize_alongside): the same menu lives on the XBOOTLDR
# /boot, plus the limine-entry-tool key pins + an in-chroot limine-update rerun.
ryoku_bootloader_finalize() {
  if [[ ${RYOKU_DISK_STRATEGY:-} == alongside && ${RYOKU_RESOLVED_ESP_MODE:-${RYOKU_ESP_MODE:-shared}} != dedicated ]]; then
    ryoku_bootloader_finalize_alongside
    return 0
  fi
  local conf=/mnt/boot/limine.conf
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log 'DRYRUN: promote %s to the tool-managed menu (when the generated tree exists)' "$conf"
    return 0
  fi
  [[ -f $conf ]] || return 0
  if grep -q '^/+' "$conf"; then
    log "limine-mkinitcpio-hook owns the menu: dropping the flat placeholder entry"
    ryoku_boot_limine_promote "$conf"
  elif grep -Eq '^[[:space:]]*//[^/]' "$conf"; then
    log "limine-mkinitcpio-hook adopted the placeholder as the boot tree: repointing the default"
    ryoku_boot_limine_repoint "$conf"
  else
    return 0
  fi
  # the hook's rewrite re-serialized the file; make sure Windows is still there.
  ryoku_windows_entry
  ryoku_dedicated_existing_entry
}

# finalize, alongside: pin the two limine-entry-tool keys the branded, dedup-free
# tree needs -- TARGET_OS_NAME (so limine-snapper-sync finds our entry to hang the
# Snapshots submenu under) and FIND_BOOTLOADERS=no (so the tool does not re-add a
# duplicate of the existing-OS chainload we hand-wrote as a fallback entry) -- in
# /etc/default/limine, the only config the tool reads. then, when the hooks landed
# in the AUR step, rerun limine-update in the chroot: with the /etc/kernel/cmdline
# we seeded it regenerates the tool-managed tree in /boot/limine.conf, which we
# autoboot + prune off the flat seed exactly like whole disk. offline / no hooks:
# the flat seed stays at default_entry 1, still bootable. the static stage-1 hop
# on the shared ESP is never touched.
ryoku_bootloader_finalize_alongside() {
  local defaults=/mnt/etc/default/limine conf=/mnt/boot/limine.conf
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log 'DRYRUN: pin TARGET_OS_NAME + FIND_BOOTLOADERS=no + EFI_REGISTER=no in %s, then (when limine-mkinitcpio-hook is installed) rerun limine-update in the chroot and repoint /boot/limine.conf off the flat seed; offline keeps the seed at default_entry 1' "$defaults"
    return 0
  fi
  if [[ -f $defaults ]]; then
    ryoku_limine_conf_set "$defaults" TARGET_OS_NAME '"Ryoku Linux"'
    ryoku_limine_conf_set "$defaults" FIND_BOOTLOADERS no
    # EFI_REGISTER=no: limine-install would otherwise register a "Limine" NVRAM
    # entry for ESP_PATH=/boot -- here the XBOOTLDR, not an ESP -- on every limine
    # upgrade, next to the 'Ryoku' entry that points at the real loader.
    ryoku_limine_conf_set "$defaults" EFI_REGISTER no
  fi
  if chroot_has_pkg limine-mkinitcpio-hook; then
    log "limine hooks landed: regenerating the tool-managed menu in /boot/limine.conf"
    run arch-chroot /mnt limine-update
    [[ -f $conf ]] || return 0
    if grep -q '^/+' "$conf"; then
      ryoku_boot_limine_promote "$conf"
    elif grep -Eq '^[[:space:]]*//[^/]' "$conf"; then
      ryoku_boot_limine_repoint "$conf"
    fi
  else
    log "no limine-mkinitcpio-hook in the target (offline or AUR skipped): the flat /Ryoku Linux seed stays bootable at default_entry 1"
  fi
}

# ryoku_limine_conf_set FILE KEY VALUE: set KEY=VALUE in a shell-style config,
# replacing an existing assignment in place or appending one. VALUE is emitted
# verbatim, so quote it in the caller when it must be quoted.
ryoku_limine_conf_set() {
  local file=$1 key=$2 value=$3 tmp
  tmp=$(mktemp) || return 1
  if grep -qE "^[[:space:]]*${key}=" "$file"; then
    sed "s|^[[:space:]]*${key}=.*|${key}=${value}|" "$file" >"$tmp"
  else
    cat "$file" >"$tmp"
    printf '%s=%s\n' "$key" "$value" >>"$tmp"
  fi
  mv "$tmp" "$file"
}

# ryoku_limine_autoboot CONF: point default_entry at the Limine entry-path
# ("<dir>/<kernel>") of the kernel this install boots -- the one RYOKU_VARIANT
# chose, which is also what /etc/ryoku/default-kernel records -- else the first
# kernel nested under the top-level OS directory. No kernel is preferred by
# name here: the variant is known at install time, so nothing has to be guessed
# from the menu text.
#
# Limine's numeric default_entry counts TOP-LEVEL entries only, so on the hook's
# collapsed-directory layout a bare index lands on the sibling "/EFI fallback",
# which chainloads Limine and loops the countdown; an entry path (CONFIG.md)
# autoboots the kernel leaf directly, and remember_last_entry autoboots the last
# kernel used. A flat menu (no directory) keeps default_entry: 1, its bootable
# placeholder. Mirrors limineDefaultKernelPath/reconcileLimineAutoboot so a
# doctored box matches a fresh install.
ryoku_limine_autoboot() {
  local conf=$1 path tmp
  local kver=linux; [[ ${RYOKU_VARIANT:-plain} == cachyos ]] && kver=linux-cachyos
  path=$(awk -v want="$kver" '
    { t = $0; sub(/^[[:space:]]+/, "", t) }
    t ~ /^\/[^\/]/                 { dir = t; sub(/^\/\+?/, "", dir); next }
    t ~ /^\/\/[^\/]/ && dir != "" {
      k = t; sub(/^\/\//, "", k)
      if (k == "Snapshots") next
      p = dir "/" k
      if (first == "") first = p
      if (chosen == "" && k == want) chosen = p
    }
    END { print (chosen != "" ? chosen : first) }
  ' "$conf")
  [[ -n $path ]] || path=1
  tmp=$(mktemp) || return 1
  awk -v p="$path" '
    /^default_entry:/       { print "default_entry: " p; next }
    /^remember_last_entry:/ { print "remember_last_entry: yes"; next }
    { print }
  ' "$conf" >"$tmp"
  grep -q '^default_entry:' "$tmp"       || sed -i "/^timeout:/a default_entry: $path" "$tmp"
  grep -q '^remember_last_entry:' "$tmp" || sed -i "/^default_entry:/a remember_last_entry: yes" "$tmp"
  mv "$tmp" "$conf"
}

# promote CONF: drop the flat "/Ryoku Linux" placeholder (entry line + its
# indented options) and point default_entry at the first UKI inside the
# /+Ryoku tree. pure file surgery, atomic, no chroot -- unit-tested by
# tests/limine-bootloader.sh.
ryoku_boot_limine_promote() {
  local conf=$1 tmp
  tmp=$(mktemp) || return 1
  awk '
    $0 == "/Ryoku Linux" { skip = 1; next }
    skip && /^[[:space:]]+[^[:space:]]/ { next }
    { skip = 0; print }
  ' "$conf" >"$tmp"
  mv "$tmp" "$conf"
  ryoku_limine_autoboot "$conf"
}

# repoint CONF for the adopted layout: limine-entry-tool 1.37+ keeps the flat
# "/Ryoku Linux" placeholder and nests the "//<kernel>" UKIs under it, turning
# it into the menu directory -- but leaves the placeholder's boot stanza
# (protocol/kernel_path/cmdline/module_path) wedged between the title and the
# first sub-entry, where Limine's grammar allows only a `comment`. a directory
# that is also a boot entry cannot autoboot: the timeout resolves nothing and
# the countdown restarts forever. strip that stanza (keep the title, comments,
# and every sub-entry) and move the default off the directory onto its first
# UKI. atomic, idempotent (re-running finds a clean directory + default 2).
ryoku_boot_limine_repoint() {
  local conf=$1 tmp
  tmp=$(mktemp) || return 1
  awk '
    $0 == "/Ryoku Linux" { print; head = 1; n = 0; next }
    head && $0 ~ /^[[:space:]]*\/\// {
      for (i = 0; i < n; i++)
        if (buf[i] !~ /^[[:space:]]+(protocol|kernel_path|module_path|path|cmdline):/ && buf[i] !~ /^[[:space:]]*$/)
          print buf[i]
      head = 0; print; next
    }
    head && $0 ~ /^\/[^\/]/ {
      for (i = 0; i < n; i++) print buf[i]
      head = 0; print; next
    }
    head { buf[n++] = $0; next }
    { print }
    END { if (head) for (i = 0; i < n; i++) print buf[i] }
  ' "$conf" >"$tmp"
  mv "$tmp" "$conf"
  ryoku_limine_autoboot "$conf"
}

# chroot_has: does $1 exist inside the target? dry-run = false, so the flow
# takes the plain mkinitcpio path (no AUR hook in the base).
chroot_has() {
  [[ -n ${RYOKU_DRYRUN:-} ]] && return 1
  arch-chroot /mnt command -v "$1" >/dev/null 2>&1
}

# chroot_has_pkg: is pacman package $1 installed in the target? dry-run = false,
# so an offline/base install takes the no-hooks path.
chroot_has_pkg() {
  [[ -n ${RYOKU_DRYRUN:-} ]] && return 1
  arch-chroot /mnt pacman -Q "$1" >/dev/null 2>&1
}

# cmdline (without "quiet splash"; default.conf appends it): UUID root for
# plain installs, cryptdevice + mapper for LUKS, plus the hibernation resume=
# pair when a swapfile exists. NOTE: stdout of this function IS the cmdline
# (captured via $(...)), so every human-facing note goes to stderr.
ryoku_cmdline() {
  local cmdline
  if [[ ${RYOKU_ENCRYPT:-} == 1 ]]; then
    local luks_uuid
    luks_uuid=$(dev_uuid "$LUKS_PART") || die 'could not read the LUKS UUID of %s (blkid returned nothing); refusing to write a cryptdevice= cmdline that would not boot.' "$LUKS_PART"
    cmdline="root=/dev/mapper/root rootflags=subvol=@ rw cryptdevice=UUID=${luks_uuid}:root"
  else
    local root_uuid
    root_uuid=$(dev_uuid "$ROOT_DEV") || die 'could not read the root UUID of %s (blkid returned nothing); refusing to write a root=UUID= cmdline that would not boot.' "$ROOT_DEV"
    cmdline="root=UUID=${root_uuid} rootflags=subvol=@ rw"
  fi
  [[ $RYOKU_PROFILE == amd-nvidia ]] && cmdline+=" nvidia_drm.modeset=1"

  # hibernation: the 'resume' initramfs hook needs the swap-backing device and
  # the swapfile's physical offset within the btrfs. only meaningful with a
  # swapfile (@swap subvol, created in the mount stage before us). the device
  # ref mirrors root=: /dev/mapper/root under LUKS, else UUID= of the root fs.
  # 'map-swapfile -r' (prints just the offset) needs btrfs-progs >= 5.16 -- the
  # same release that added the mkswapfile we build with -- so this normally
  # succeeds; on an older toolchain we skip cleanly (no hibernate, still boots).
  if (( ${RYOKU_SWAP_GIB:-0} > 0 )); then
    local resume_ref off
    if [[ ${RYOKU_ENCRYPT:-} == 1 ]]; then
      resume_ref=/dev/mapper/root
    else
      resume_ref=UUID=${root_uuid}
    fi
    if [[ -n ${RYOKU_DRYRUN:-} ]]; then
      log "dry-run: hibernation resume=$resume_ref resume_offset=<btrfs map-swapfile /mnt/swap/swapfile>" >&2
      cmdline+=" resume=$resume_ref resume_offset=<OFFSET>"
    elif [[ -f /mnt/swap/swapfile ]] && off=$(btrfs inspect-internal map-swapfile -r /mnt/swap/swapfile 2>/dev/null) && [[ -n $off ]]; then
      cmdline+=" resume=$resume_ref resume_offset=$off"
    else
      log "hibernation: no swapfile or btrfs-progs too old for map-swapfile; skipping resume= (system boots, hibernate disabled)" >&2
    fi
  fi
  printf '%s' "$cmdline"
}

# vmd: Intel Volume Management Device (a.k.a. Intel RST "VMD" mode) hides the
# NVMe behind the vmd controller. if the LIVE installer kernel had to load the
# vmd module to see the disk, the INSTALLED initramfs needs it too -- otherwise
# the target can't find its own root at boot (a classic Intel-laptop install
# that boots the ISO fine then drops to an emergency shell). detected on the
# live system (/sys/module/vmd) and written as a MODULES+= drop-in BEFORE the
# initramfs build, so both the limine-mkinitcpio (UKI) and mkinitcpio -P paths
# bake it in. '+=' so it stacks with nvidia.sh's MODULES=() drop-in.
ryoku_boot_vmd() {
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log "dry-run: if the live kernel has VMD loaded (/sys/module/vmd), would write /mnt/etc/mkinitcpio.conf.d/ryoku-vmd.conf with MODULES+=(vmd)"
    return 0
  fi
  [[ -d /sys/module/vmd ]] || return 0
  log "Intel VMD active on the live system: adding 'vmd' to the target initramfs so it finds the NVMe"
  run mkdir -p /mnt/etc/mkinitcpio.conf.d
  write_file /mnt/etc/mkinitcpio.conf.d/ryoku-vmd.conf <<'EOF'
MODULES+=(vmd)
EOF
}

ryoku_boot_plymouth() {
  # The ryoku-desktop package (installed in the configure stage, before this step)
  # owns /usr/share/plymouth/themes/ryoku. Never lay a second, unowned copy here:
  # pacman would then abort every later `-Syu` on "exists in filesystem" once the
  # package owns that path. Just make it the default so the initramfs built next
  # embeds it; ryoku-boot-apply re-asserts it on every update.
  if [[ -n ${RYOKU_DRYRUN:-} || -d /mnt/usr/share/plymouth/themes/ryoku ]]; then
    log "setting Plymouth default theme 'ryoku'"
    run arch-chroot /mnt plymouth-set-default-theme ryoku
  else
    log "skip: Plymouth theme absent (desktop set not installed); ryoku-boot-apply sets it on the first update"
  fi
}

# default_limine: write /etc/default/limine, swap @@CMDLINE@@ for the real
# root cmdline.
ryoku_boot_default_limine() {
  log "deploying /etc/default/limine"
  local src="$RYOKU_REPO/system/boot/limine/default.conf"
  local content
  if [[ -f $src ]]; then
    content=$(<"$src")
  else
    content=$(ryoku_builtin_default_limine)
  fi
  # substitute only on the KERNEL_CMDLINE directive, so any @@CMDLINE@@
  # sitting in the surrounding comments stays.
  content=$(printf '%s\n' "$content" | sed "/^KERNEL_CMDLINE\[default\]/ s|@@CMDLINE@@|$CMDLINE|")
  run mkdir -p /mnt/etc/default
  write_file /mnt/etc/default/limine <<<"$content"
}

# limine_conf: write /boot/limine.conf (the ESP root -- the one location
# limine-entry-tool manages, so the hook's UKI entries and the snapshot
# submenu land in the file the firmware actually reads). branding header from
# the repo (trailing placeholder entry stripped) or a built-in fallback.
# with_entry appends a plain linux-protocol entry for the mkinitcpio -P path
# and points default_entry at it (the flat menu has no tree directory to
# skip); branding_only keeps default_entry: 2 and leaves entries to the
# limine hook. shadowing candidates are removed either way.
ryoku_boot_limine_conf() {
  local mode=$1
  # variant kernel: cachyos boots linux-cachyos (stock linux stays installed and
  # the limine hook lists it as fallback); plain boots stock linux.
  local kver=linux; [[ ${RYOKU_VARIANT:-plain} == cachyos ]] && kver=linux-cachyos
  # record it: on a live box a kernel package the user added later looks exactly
  # like the one the install was built around, so `ryoku doctor` has nothing to
  # point default_entry at without this (doctor.defaultKernelFile).
  run mkdir -p /mnt/etc/ryoku
  write_file /mnt/etc/ryoku/default-kernel <<<"$kver"
  local src="$RYOKU_REPO/system/boot/limine/limine.conf"
  local branding
  if [[ -f $src ]]; then
    branding=$(sed '/^\/Ryoku Linux/,$d' "$src")
  else
    branding=$(ryoku_builtin_limine_branding)
  fi
  if [[ $mode == with_entry ]]; then
    branding=$(printf '%s\n' "$branding" | sed 's/^default_entry: 2$/default_entry: 1/')
  fi

  ryoku_boot_limine_conflicts
  {
    printf '%s\n' "$branding"
    if [[ $mode == with_entry ]]; then
      cat <<EOF

/Ryoku Linux
    protocol: linux
    kernel_path: boot():/vmlinuz-$kver
    cmdline: $CMDLINE quiet splash
    module_path: boot():/initramfs-$kver.img
EOF
    fi
  } | write_file /mnt/boot/limine.conf
}

# conflicts: remove every limine.conf candidate that would shadow the ESP-root
# config (Limine stops at its first match; limine-install warns about exactly
# this list). matters on re-runs of this installer and on an ESP reused from
# another distro. the empty /boot/limine dir is dropped too.
ryoku_boot_limine_conflicts() {
  run rm -f \
    /mnt/boot/EFI/limine/limine.conf \
    /mnt/boot/EFI/BOOT/limine.conf \
    /mnt/boot/boot/limine/limine.conf \
    /mnt/boot/boot/limine.conf \
    /mnt/boot/limine/limine.conf
  run_sh "rmdir /mnt/boot/limine 2>/dev/null || true"
}

# windows_entry: find an installed Windows on ANY drive (not only the reused
# ESP) and write a uuid()-addressed Limine chainload entry. dual-boot stays
# bootable after Ryoku takes the boot order. boot():/ only reaches Limine's
# own ESP, so a cross-drive Windows has to be referenced by its partition
# GUID; the shared system/boot helper does the scan. runs AFTER the menu is
# generated so the entry isn't regenerated away, and the shipped post.d hook
# re-asserts it on later kernel updates. dry-run skips (probe mounts).
ryoku_windows_entry() {
  local helper="$RYOKU_REPO/system/boot/limine/ryoku-windows-entry"
  local conf=/mnt/boot/limine.conf
  [[ -x $helper && -f $conf ]] || return 0
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log "dry-run: skipping Windows boot-entry detection"
    return 0
  fi
  if "$helper" sync "$conf" >/dev/null 2>&1 && grep -q '^/Windows$' "$conf" 2>/dev/null; then
    log "Windows detected: added a chainload entry to the Limine menu"
  fi
}

# Add the selected non-Windows ESP loader to a dedicated-ESP install. Windows is
# handled separately by ryoku_windows_entry.
# $1/$2 are optional (production runs argless with the /mnt defaults; the limine
# bootloader test injects temp paths), so SC2120's "arguments never passed" is
# expected here.
# shellcheck disable=SC2120
ryoku_dedicated_existing_entry() {
  local mode=${RYOKU_RESOLVED_ESP_MODE:-${RYOKU_ESP_MODE:-shared}}
  local conf=${1:-/mnt/boot/limine.conf} state=${2:-/mnt/etc/ryoku/limine-existing-esp}
  local esp kind boot partuuid title
  [[ ${RYOKU_DISK_STRATEGY:-} == alongside && $mode == dedicated && -f $conf ]] || return 0
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log 'dry-run: would persist and add the existing non-Windows OS to %s by ESP PARTUUID when a loader is found' "$conf"
    return 0
  fi
  esp=${RYOKU_PF_ESP:-}
  kind=${RYOKU_PF_ESP_KIND:-}
  boot=${RYOKU_PF_ESP_BOOT:--}
  [[ $kind == none ]] && return 0   # create-esp: no existing OS to chainload
  [[ -n $esp && -n $kind ]] \
    || die "dedicated bootloader: preflight did not preserve the existing ESP metadata."
  [[ $kind != windows ]] || return 0
  if [[ -z $boot || $boot == - || $boot == none ]]; then
    log 'note: the existing %s system has no chainloadable EFI binary; it stays available through the firmware menu.' "$kind"
    return 0
  fi
  partuuid=$(blkid -o value -s PARTUUID "$esp" 2>/dev/null || true)
  [[ -n $partuuid ]] \
    || die 'dedicated bootloader: blkid found no PARTUUID for the existing ESP %s.' "$esp"
  run mkdir -p "$(dirname "$state")"
  printf '%s\t%s\t%s\n' "$kind" "$partuuid" "$boot" | write_file "$state"
  [[ $kind == ryoku ]] && title="Ryoku (existing)" || title="Linux (existing)"
  grep -qxF "/$title" "$conf" && return 0
  ryoku_alongside_existing_entry "$kind" "$boot" "$partuuid" >>"$conf"
  log 'existing %s install detected: added a GUID-addressed chainload entry' "$kind"
}

# install_efi: drop the Limine EFI binary on the ESP and register a boot
# entry. paths match limine-install (limine-entry-tool) exactly --
# EFI/limine/limine_x64.efi + the EFI/BOOT fallback -- so the tool's pacman
# hook keeps refreshing the very binary the firmware boots on every limine
# package upgrade, and its NVRAM dedup (partition uuid + loader path)
# recognizes our entry instead of adding a second one.
ryoku_boot_install_efi() {
  log "installing Limine EFI binary + boot entry"
  run mkdir -p /mnt/boot/EFI/BOOT /mnt/boot/EFI/limine
  run cp /mnt/usr/share/limine/BOOTX64.EFI /mnt/boot/EFI/limine/limine_x64.efi
  # Only whole-disk and dedicated-alongside modes reach this path, so /boot is
  # Ryoku's own ESP and the fallback cannot overwrite another OS.
  run cp /mnt/usr/share/limine/BOOTX64.EFI /mnt/boot/EFI/BOOT/BOOTX64.EFI

  local esp_partnum
  esp_partnum=$(part_num "$ESP_DEV")
  [[ -n $esp_partnum ]] || die 'could not derive the ESP partition number from %s; refusing to register a boot entry against a guessed partition.' "$ESP_DEV"
  # efibootmgr writes firmware NVRAM, which some machines expose readonly or
  # report full (HP / Insyde-class firmware). that MUST NOT abort the install
  # (set -e): the removable-path EFI/BOOT/BOOTX64.EFI copy above still boots the
  # system. so both --create and --bootnext are best-effort, with a loud warning
  # naming the fallback. loader path stays byte-identical (\EFI\limine\limine_x64.efi)
  # so limine-install's pacman-hook NVRAM dedup (partition uuid + loader path)
  # still recognizes this entry instead of adding a second one on upgrades.
  if ! run arch-chroot /mnt efibootmgr --create --disk "$RYOKU_DISK" --part "$esp_partnum" \
    --label Ryoku --loader '\EFI\limine\limine_x64.efi' --unicode; then
    log "WARNING: efibootmgr could not register the 'Ryoku' NVRAM boot entry (readonly or full firmware NVRAM, e.g. HP/Insyde). The system still boots via the UEFI removable-path fallback EFI/BOOT/BOOTX64.EFI on our ESP; if the firmware does not pick it up automatically, select it once from the firmware boot menu."
    return 0
  fi
  # boot the installed system on the next reboot even if the USB installer
  # is still in (firmware tends to prefer removable media otherwise). also
  # best-effort: a firmware that rejected --create may reject --bootnext too.
  if [[ -z "${RYOKU_DRYRUN:-}" ]]; then
    local num
    num=$(efibootmgr 2>/dev/null | sed -n 's/^Boot\([0-9A-Fa-f]\{4\}\)\*\? Ryoku\b.*/\1/p' | head -1)
    if [[ -n $num ]]; then
      run efibootmgr --bootnext "$num" || log "WARNING: could not set BootNext; pick 'Ryoku' from the firmware boot menu on the first reboot (or it falls back to EFI/BOOT/BOOTX64.EFI)."
    fi
  fi
}

# bootloader, alongside branch: TWO-STAGE limine. the kernels live on our OWN
# XBOOTLDR /boot (FAT label RYOKUBOOT); the existing OS keeps the SHARED ESP.
# limine reads FAT only and resolves its config + boot()-relative paths ONLY on
# the volume it was loaded from, so a tool-managed menu on the shared ESP panics
# on boot()-relative kernel paths (proven). the working topology:
#   stage 1 (shared ESP, /EFI/ryoku/* ONLY): a STATIC hop -- one efi_chainload
#           entry to the second-stage limine, timeout 0. never regenerated.
#   stage 2 (our XBOOTLDR /boot): the second-stage limine binary
#           (/boot/ryoku-limine.efi) beside /boot/limine.conf, the ONE file
#           limine-entry-tool manages here (branded menu, kernels, snapshots).
# because stage 2 lives on the XBOOTLDR, boot() there is the XBOOTLDR, NEVER the
# shared ESP: every cross-volume path to the existing OS is guid() explicit.
# we write ONLY /EFI/ryoku/* on the shared ESP (never a foreign vendor dir) and
# tar the whole ESP first so a mistake stays recoverable. this topology is
# VM-proven; evidence at .superpowers/sdd/twostage-report.md.
ryoku_bootloader_alongside() {
  log "building initramfs via mkinitcpio -P (kernels on the XBOOTLDR /boot)"
  # /usr/bin/mkinitcpio: see the wipe branch. The wrapper's prompt would stall the
  # install right here, with the kernels not yet on the boot partition.
  run arch-chroot /mnt /usr/bin/mkinitcpio -P

  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log "DRYRUN: install the second-stage limine at /boot/ryoku-limine.efi + seed /boot/limine.conf (branding, fslabel(RYOKUBOOT) kernels, guid() existing-OS chainload), mount the existing OS's ESP at /mnt/efi, tar it to /var/backups/ryoku/, drop the STATIC stage-1 hop at /efi/EFI/ryoku/{BOOTX64.EFI,limine.conf} (timeout 0, efi_chainload -> fslabel(RYOKUBOOT):/ryoku-limine.efi), seed /etc/kernel/cmdline + a /efi fstab line + the ryoku-limine-stage.hook that refreshes both stage loaders on limine upgrades, and register 'Ryoku' first in BootOrder; other vendors' dirs (e.g. /EFI/Microsoft) untouched"
    return 0
  fi

  # re-probe the shared ESP as the source of truth (no env from the TUI): its
  # device, kind, and the existing system's EFI binary to chainload.
  local espinfo esp esp_kind esp_boot esp_partuuid
  espinfo=$(ryoku_esp_scan "$RYOKU_DISK") \
    || die 'alongside bootloader: no EFI System Partition on %s to share; refusing to install a bootloader with nowhere to land.' "$RYOKU_DISK"
  read -r esp esp_kind esp_boot <<<"$espinfo"
  [[ $esp_boot == - ]] && esp_boot=none

  # the shared ESP's GPT partition GUID: every stage-2 cross-volume path is
  # guid(<this>):/... . an empty guid would silently un-boot the existing OS, so
  # ASSERT it -- the VM run lost a boot to an empty identifier.
  esp_partuuid=$(blkid -o value -s PARTUUID "$esp" 2>/dev/null || true)
  [[ -n $esp_partuuid ]] || die 'alongside bootloader: blkid found no PARTUUID for the shared ESP %s; refusing to write guid()-addressed entries against an empty identifier.' "$esp"

  # STAGE 2 on our XBOOTLDR /boot (already mounted): the second-stage limine +
  # the tool-managed menu. touches only our own volume, so it precedes any write
  # to the shared ESP.
  log "installing the second-stage limine at /boot/ryoku-limine.efi + seeding /boot/limine.conf"
  run cp /mnt/usr/share/limine/BOOTX64.EFI /mnt/boot/ryoku-limine.efi
  ryoku_alongside_conf_text "$esp_kind" "$esp_boot" "$esp_partuuid" | write_file /mnt/boot/limine.conf
  if [[ $esp_kind != windows && $esp_boot == none ]]; then
    log 'note: the existing %s system on the shared ESP has no chainloadable EFI binary; it stays bootable via the firmware boot menu only.' "$esp_kind"
  fi

  # The shared ESP is a FAT volume another OS writes. Windows Fast Startup and
  # hard power-offs leave it dirty, and laying Ryoku's loader into a dirty FAT
  # is how a shared boot partition gets corrupted: fsck.fat -a repairs the
  # countable errors without prompting, and anything it cannot fix stops the
  # install here, before one byte of the existing OS's ESP is touched.
  if command -v fsck.fat >/dev/null 2>&1; then
    local fsrc=0
    fsck.fat -a "$esp" >/dev/null 2>&1 || fsrc=$?
    (( fsrc <= 1 )) \
      || die 'the shared ESP %s has filesystem errors fsck.fat could not repair (rc=%s); Ryoku refuses to write into it. Boot the OS that owns the ESP and repair the volume, or install with RYOKU_ESP_MODE=dedicated so Ryoku keeps to its own ESP.' "$esp" "$fsrc"
  fi
  run mkdir -p /mnt/efi
  run mount "$esp" /mnt/efi

  # BEFORE any write to the shared ESP: back it up to the new root, so a botched
  # write to the existing OS's ESP is recoverable from the installed system.
  local ts bak
  ts=$(date +%Y%m%d-%H%M%S)
  case $esp_kind in
    windows) bak="windows-esp-${ts}.tar" ;;   # historical name, kept for Windows
    *)       bak="esp-${esp_kind}-${ts}.tar" ;;
  esac
  run mkdir -p /mnt/var/backups/ryoku
  run_sh "tar -C /mnt/efi -cf /mnt/var/backups/ryoku/${bak} ."
  log 'backed up the shared ESP (%s, kind=%s) to /var/backups/ryoku/%s' "$esp" "$esp_kind" "${bak}"

  # STAGE 1 on the shared ESP: our loader + the static hop in our OWN /EFI/ryoku
  # dir; NEVER a foreign vendor's dir.
  run mkdir -p /mnt/efi/EFI/ryoku
  run cp /mnt/usr/share/limine/BOOTX64.EFI /mnt/efi/EFI/ryoku/BOOTX64.EFI
  ryoku_stage1_hop_text | write_file /mnt/efi/EFI/ryoku/limine.conf

  # removable-path fallback only if there isn't one already: the existing OS's
  # own /EFI/BOOT/BOOTX64.EFI (if present) must survive untouched.
  if [[ -e /mnt/efi/EFI/BOOT/BOOTX64.EFI ]]; then
    log "leaving the existing /EFI/BOOT/BOOTX64.EFI in place (not ours to overwrite)"
  else
    run mkdir -p /mnt/efi/EFI/BOOT
    run cp /mnt/usr/share/limine/BOOTX64.EFI /mnt/efi/EFI/BOOT/BOOTX64.EFI
  fi

  # target plumbing the VM run proved necessary. /etc/default/limine already
  # pins ESP_PATH=/boot (ryoku_boot_default_limine deployed it before this
  # branch), so limine-entry-tool never autodetects the shared ESP once both
  # FATs are in fstab; here we add the kernel cmdline file limine-update reads
  # and the /efi mount.
  ryoku_alongside_kernel_cmdline "$CMDLINE quiet splash"
  ryoku_alongside_fstab_efi "$esp"
  ryoku_alongside_stage_hook

  # register 'Ryoku' first in BootOrder; the existing entry is left as-is. best
  # effort: firmware that rejects NVRAM writes still boots via the fallback above.
  local esp_partnum
  esp_partnum=$(part_num "$esp")
  [[ -n $esp_partnum ]] || die 'could not derive the shared ESP partition number from %s; refusing to register a boot entry against a guessed partition.' "$esp"
  if ! run arch-chroot /mnt efibootmgr --create --disk "$RYOKU_DISK" --part "$esp_partnum" \
    --label Ryoku --loader '\EFI\ryoku\BOOTX64.EFI' --unicode; then
    log "WARNING: efibootmgr could not register the 'Ryoku' NVRAM boot entry (readonly or full firmware NVRAM). The system still boots via /EFI/BOOT/BOOTX64.EFI on the shared ESP; if the firmware ignores it, pick it once from the firmware boot menu."
  fi

  run umount /mnt/efi
  run_sh 'rmdir /mnt/efi 2>/dev/null || true'
}

# ryoku_stage1_hop_text: the STATIC stage-1 hop written beside our BOOTX64.EFI on
# the shared ESP (/EFI/ryoku/limine.conf). limine reads the config next to the
# binary it loaded, so this is the first thing the shared-ESP loader sees. it is
# NEVER regenerated by limine-entry-tool -- that tool owns the stage-2
# /boot/limine.conf on the XBOOTLDR. one job: efi_chainload the second-stage
# limine on our own boot volume (addressed by its FAT label), where the real menu
# lives with correct binary-relative paths.
ryoku_stage1_hop_text() {
  local label=${RYOKU_ALONGSIDE_BOOT_LABEL:-RYOKUBOOT}
  cat <<EOF
# Ryoku stage-1 hop. STATIC -- do not edit: limine-entry-tool never regenerates
# this file (it manages /boot/limine.conf on the $label volume). it only
# chainloads the second-stage limine there, where the branded menu, kernels, and
# Snapshots submenu live with correct binary-relative paths.
timeout: 0
default_entry: 1

/Ryoku
    protocol: efi_chainload
    image_path: fslabel($label):/ryoku-limine.efi
EOF
}

# ryoku_alongside_conf_text <esp_kind> <existing_boot> <esp_partuuid>: the STAGE-2
# /boot/limine.conf body -- pure (stdout only), so it is generator-testable.
# branding globals + our flat linux entry (fslabel(RYOKUBOOT) kernels,
# offline-bootable), then the existing-system chainload entry. this file lives on
# the XBOOTLDR, so boot() here is the XBOOTLDR: every path to the shared ESP is
# guid() explicit, never boot():/EFI/... .
ryoku_alongside_conf_text() {
  local esp_kind=$1 esp_boot=$2 esp_partuuid=$3 branding src="$RYOKU_REPO/system/boot/limine/limine.conf"
  local kver=linux; [[ ${RYOKU_VARIANT:-plain} == cachyos ]] && kver=linux-cachyos
  local label=${RYOKU_ALONGSIDE_BOOT_LABEL:-RYOKUBOOT}
  if [[ -f $src ]]; then
    branding=$(sed '/^\/Ryoku Linux/,$d' "$src")
  else
    branding=$(ryoku_builtin_limine_branding)
  fi
  branding=$(printf '%s\n' "$branding" | sed 's/^default_entry: 2$/default_entry: 1/')
  printf '%s\n' "$branding"
  cat <<EOF

/Ryoku Linux
    protocol: linux
    kernel_path: fslabel($label):/vmlinuz-$kver
    cmdline: $CMDLINE quiet splash
    module_path: fslabel($label):/initramfs-$kver.img
EOF
  ryoku_alongside_existing_entry "$esp_kind" "$esp_boot" "$esp_partuuid"
}

# ryoku_alongside_existing_entry <esp_kind> <existing_boot> <esp_partuuid>: the menu
# entry for the OS that owns the shared ESP, so it stays bootable after Ryoku takes
# the boot order. the shared ESP is a DIFFERENT volume than this stage-2 config, so
# it is addressed by its partition GUID; a boot():/EFI/... here would hit the
# XBOOTLDR, not the shared ESP. Windows keeps its /Windows title; a ryoku/linux
# neighbor gets a "<Kind> (existing)" entry pointing at the binary the probe found.
# no binary (existing_boot none) = no entry (reachable via the firmware menu only).
ryoku_alongside_existing_entry() {
  local esp_kind=$1 esp_boot=$2 esp_partuuid=$3 title
  if [[ $esp_kind == windows ]]; then
    cat <<EOF

/Windows
    protocol: efi_chainload
    image_path: guid($esp_partuuid):/EFI/Microsoft/Boot/bootmgfw.efi
    comment: Windows Boot Manager
EOF
    return 0
  fi
  [[ -n $esp_boot && $esp_boot != none && $esp_boot != - ]] || return 0
  case $esp_kind in
    ryoku) title="Ryoku (existing)" ;;
    *)     title="Linux (existing)" ;;
  esac
  cat <<EOF

/$title
    protocol: efi_chainload
    image_path: guid($esp_partuuid):$esp_boot
    comment: existing $esp_kind install
EOF
}

# ryoku_alongside_kernel_cmdline <cmdline> [target_root]: seed /etc/kernel/cmdline
# with the exact cmdline the stage-2 seed entry boots. limine-update (run in
# finalize once the hooks land) reads this to regenerate /boot/limine.conf; the
# in-chroot regen fails without it (proven in the VM run).
ryoku_alongside_kernel_cmdline() {
  local cmdline=$1 root=${2:-/mnt}
  run mkdir -p "$root/etc/kernel"
  write_file "$root/etc/kernel/cmdline" <<<"$cmdline"
}

# ryoku_alongside_fstab_efi <esp_dev> [fstab]: add the shared ESP to the target
# fstab at /efi, so the installed system can reach it and `ryoku doctor` can tell
# this is an alongside box. ASSERT a non-empty fs UUID first: the VM run lost a
# boot to an fstab line with an empty UUID, so an empty read is fatal, never
# silent. nofail keeps a missing/foreign ESP from wedging boot (root + /boot are
# on our own partitions).
ryoku_alongside_fstab_efi() {
  local esp=$1 fstab=${2:-/mnt/etc/fstab} uuid
  uuid=$(blkid -o value -s UUID "$esp" 2>/dev/null || true)
  [[ -n $uuid ]] || die 'alongside bootloader: blkid found no filesystem UUID for the shared ESP %s; refusing to append an empty-UUID /efi fstab line (a boot-losing mistake).' "$esp"
  printf 'UUID=%s\t/efi\tvfat\tdefaults,nofail,noatime\t0 2\n' "$uuid" >>"$fstab"
}

# ryoku_alongside_stage_hook: keep the two-stage loaders current. limine's own
# deploy hook only refreshes ESP_PATH/EFI/limine/limine_x64.efi, which this
# topology never boots -- our stage-1 (/efi/EFI/ryoku/BOOTX64.EFI) and stage-2
# (/boot/ryoku-limine.efi) would keep booting the version the ISO shipped while
# limine.conf moves on. So refresh both from the package on every limine upgrade.
ryoku_alongside_stage_hook() {
  run mkdir -p /mnt/etc/pacman.d/hooks
  write_file /mnt/etc/pacman.d/hooks/ryoku-limine-stage.hook <<'EOF'
[Trigger]
Operation = Install
Operation = Upgrade
Type = Package
Target = limine

[Action]
Description = Refreshing the Ryoku two-stage Limine loaders...
When = PostTransaction
Exec = /bin/sh -c 'src=/usr/share/limine/BOOTX64.EFI; [ -f "$src" ] || exit 0; [ -f /boot/ryoku-limine.efi ] && cp -f "$src" /boot/ryoku-limine.efi; [ -f /efi/EFI/ryoku/BOOTX64.EFI ] && cp -f "$src" /efi/EFI/ryoku/BOOTX64.EFI; exit 0'
EOF
}

ryoku_builtin_default_limine() {
  cat <<'EOF'
TARGET_OS_NAME="Ryoku"
ESP_PATH="/boot"
ENABLE_UKI=yes
CUSTOM_UKI_NAME="ryoku"
KERNEL_CMDLINE[default]="@@CMDLINE@@"
KERNEL_CMDLINE[default]+=" quiet splash"
EOF
}

ryoku_builtin_limine_branding() {
  cat <<'EOF'
timeout: 3
default_entry: 1
remember_last_entry: yes
interface_branding: Ryoku Bootloader
interface_branding_color: C75D2B
interface_help_color: C75D2B
hash_mismatch_panic: no

term_background: 060607
backdrop: 060607
term_palette: 060607;EAE2D5;C75D2B;3A3630;88A57D;C75D2B;8C857A;EAE2D5
term_palette_bright: 141210;EAE2D5;C75D2B;3A3630;88A57D;C75D2B;8C857A;EAE2D5
term_foreground: EAE2D5
term_foreground_bright: EAE2D5
term_background_bright: 141210
EOF
}
