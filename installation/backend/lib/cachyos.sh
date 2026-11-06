#!/usr/bin/env bash
# shellcheck shell=bash
# Make the installed target a CachyOS system the way CachyOS's own installer
# does: the pacman.conf CachyOS ships (CachyOS-Live-ISO pacman-more.conf), the
# CPU's optimized repositories above [cachyos] (cachyos-calamares
# detect-architecture: znver4, else v4, else v3), and the CachyOS key trusted
# from the installed cachyos-keyring (offline, no keyserver).
#
# The install ITSELF pulls every package (the CachyOS kernel, CachyOS's pacman)
# from the baked [offline] repo (lib/offline.sh), which holds generic x86-64
# builds so it installs on any CPU, as CachyOS's own offline install does. This
# step wires future `pacman -Syu` to CachyOS so the box stays a CachyOS box: a
# CPU with a tier moves onto that tier's builds on the first -Syu, since every
# tier build carries a higher pkgrel than the generic one.

# ryoku_cachyos_replaced: the base.packages names a CachyOS install never
# carries (system/packages/cachyos.replaces).
ryoku_cachyos_replaced() {
  local file="$RYOKU_REPO/system/packages/cachyos.replaces"
  [[ -f $file ]] || return 0
  grep -vE '^[[:space:]]*(#|$)' "$file"
}

# ryoku_cachyos_drops leaves replaced names out like deselected apps.
# RYOKU_DROP_PACKAGES keeps them out of pacstrap, then users seed-self records
# them in the provisioning ledger so `ryoku update` does not deliver them later.
ryoku_cachyos_drops() {
  local p
  while IFS= read -r p; do
    _ryoku_append_drop "$p"
  done < <(ryoku_cachyos_replaced)
  export RYOKU_DROP_PACKAGES
}

# ryoku_cachyos_is_zen45 CPUINFO: a Zen 4 or Zen 5 CPU with AVX-512, the CPUs
# CachyOS gives its znver4 builds. Same family/model table as
# detect-architecture.
ryoku_cachyos_is_zen45() {
  local cpuinfo=$1 family model
  grep -qw avx512f "$cpuinfo" 2>/dev/null || return 1
  family=$(awk -F: '/^cpu family/ { gsub(/[[:space:]]/, "", $2); print $2; exit }' "$cpuinfo")
  model=$(awk -F: '/^model[[:space:]]*:/ { gsub(/[[:space:]]/, "", $2); print $2; exit }' "$cpuinfo")
  [[ $family =~ ^[0-9]+$ && $model =~ ^[0-9]+$ ]] || return 1
  case $family in
    25) (( (model >= 0x10 && model <= 0x1f) || (model >= 0x60 && model <= 0xaf) )) ;;
    26) return 0 ;;
    *) return 1 ;;
  esac
}

# ryoku_cachyos_cpu_level: the CachyOS repository tier this CPU runs (znver4,
# v4, v3), or nothing for a CPU below x86-64-v3.
ryoku_cachyos_cpu_level() {
  local isa
  isa=$(LC_ALL=C /lib/ld-linux-x86-64.so.2 --help 2>/dev/null) || isa=""
  if ryoku_cachyos_is_zen45 /proc/cpuinfo; then
    printf 'znver4\n'
  elif grep -qF 'x86-64-v4 (supported' <<<"$isa"; then
    printf 'v4\n'
  elif grep -qF 'x86-64-v3 (supported' <<<"$isa"; then
    printf 'v3\n'
  fi
}

# ryoku_cachyos_tier_repos LEVEL: the repository stanzas for a CPU tier, in
# detect-architecture's order. Nothing for an unknown or empty tier.
ryoku_cachyos_tier_repos() {
  local level=$1 list repo
  local -a repos=()
  case $level in
    znver4) list=cachyos-v4-mirrorlist; repos=(cachyos-znver4 cachyos-core-znver4 cachyos-extra-znver4) ;;
    v4) list=cachyos-v4-mirrorlist; repos=(cachyos-v4 cachyos-extra-v4 cachyos-core-v4) ;;
    v3) list=cachyos-v3-mirrorlist; repos=(cachyos-v3 cachyos-extra-v3 cachyos-core-v3) ;;
    *) return 0 ;;
  esac
  for repo in "${repos[@]}"; do
    printf '[%s]\nInclude = /etc/pacman.d/%s\n\n' "$repo" "$list"
  done
}

# ryoku_cachyos_pacman_conf CONF LEVEL: write CachyOS's pacman.conf with the
# tier's repositories above [cachyos]. deploy.sh appends [ryoku] after it, and
# the installer's temporary [offline] stanza comes and goes in between
# (lib/offline.sh).
ryoku_cachyos_pacman_conf() {
  local conf=$1 level=${2:-}
  write_file "$conf" <<EOF
# /etc/pacman.conf, as CachyOS installs it. See pacman.conf(5).

[options]
HoldPkg     = pacman glibc
Architecture = auto
Color
ILoveCandy
VerbosePkgLists
DisableDownloadTimeout
ParallelDownloads = 10
DownloadUser = alpm
SigLevel    = Required DatabaseOptional
LocalFileSigLevel = Optional
NoExtract = etc/skel/.config/autostart/cachyos-hello.desktop

$(ryoku_cachyos_tier_repos "$level")
[cachyos]
Include = /etc/pacman.d/cachyos-mirrorlist

[core]
Include = /etc/pacman.d/mirrorlist

[extra]
Include = /etc/pacman.d/mirrorlist

[multilib]
Include = /etc/pacman.d/mirrorlist
EOF
}

# ryoku_cachyos_arch_check CONF LEVEL: `Architecture = auto` takes in the
# x86_64_v3/v4 packages only under CachyOS's pacman. A target that ended up
# with Arch's pacman (a networked install pacstrapped from Arch mirrors)
# resolves it to x86_64 alone and would reject its own tier, so name it there.
ryoku_cachyos_arch_check() {
  local conf=$1 level=$2 want arches
  case $level in
    v3) want=x86_64_v3 ;;
    v4|znver4) want='x86_64_v3 x86_64_v4' ;;
    *) return 0 ;;
  esac
  arches=$(arch-chroot /mnt pacman-conf --config /etc/pacman.conf Architecture 2>/dev/null) || arches=""
  grep -qx "${want##* }" <<<"$arches" && return 0
  log 'cachyos: the target pacman does not resolve Architecture = auto to %s; naming it explicitly' "$want"
  run sed -i "s|^Architecture = auto\$|Architecture = auto $want|" "$conf"
}

ryoku_cachyos_repo() {
  local conf=/mnt/etc/pacman.conf level
  level=$(ryoku_cachyos_cpu_level)
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log "DRYRUN: write CachyOS's pacman.conf with the ${level:-baseline} repositories above [cachyos], populate the cachyos keyring in the target"
    ryoku_cachyos_identity
    return 0
  fi
  log "cachyos: writing CachyOS's pacman.conf (${level:-baseline} repositories) into the target"
  ryoku_cachyos_pacman_conf "$conf" "$level"
  ryoku_cachyos_arch_check "$conf" "$level"
  ryoku_cachyos_keyring
  ryoku_cachyos_identity
}

# trust the CachyOS key from the INSTALLED cachyos-keyring (no keyserver, so it
# works offline). the recv/lsign path is the online fallback for a box without
# the package.
ryoku_cachyos_keyring() {
  if arch-chroot /mnt pacman-key --populate cachyos >/dev/null 2>&1; then
    log "cachyos: trusted the CachyOS signing key via the installed keyring"
    return 0
  fi
  log "cachyos: keyring populate failed (cachyos-keyring absent?); a later -Syu can 'pacman-key --recv-keys F3B607488DB35A47 --keyserver keyserver.ubuntu.com'"
}

# The package's hook cannot trigger the transaction that installs it. Run its
# script once now, and keep CachyOS Welcome in the launcher without letting its
# skel entry compete with Ryoku's first-login welcome. Accounts already exist by
# the time ryoku_cachyos_repo runs, so clean both skel and their copied entries.
ryoku_cachyos_identity() {
  local hello=etc/skel/.config/autostart/cachyos-hello.desktop
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log "DRYRUN: remove $hello from the target skel and created home directories"
    log 'DRYRUN: run /usr/share/libalpm/scripts/cachyos-branding in the target'
    return 0
  fi
  log 'cachyos: suppressing CachyOS Welcome autostart'
  run rm -f "/mnt/$hello"
  if [[ -d /mnt/home ]]; then
    run find /mnt/home -mindepth 4 -maxdepth 4 \
      -path '*/.config/autostart/cachyos-hello.desktop' -delete
  fi
  log 'cachyos: applying CachyOS system identity'
  run arch-chroot /mnt /usr/share/libalpm/scripts/cachyos-branding
}

# Optional CachyOS gaming layer for the plain Arch variant. Arch repositories
# remain authoritative while the lower CachyOS repositories provide names Arch
# does not ship.
ryoku_cachyos_layer_packages() {
  local here manifest packages
  here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
  for manifest in \
    "${RYOKU_INSTALLER_COMPONENTS_FILE:-}" \
    "${RYOKU_REPO:-/usr/share/ryoku}/installation/installerplan/components.tsv" \
    "$here/../../installerplan/components.tsv"; do
    [[ -n $manifest && -f $manifest ]] || continue
    packages=$(awk -F '\t' '
      $1 == "cachyos-layer" {
        if ($5 != "" && $5 != "-") { print $5; found = 1 }
        exit
      }
      END { if (!found) exit 1 }
    ' "$manifest") || return 1
    printf '%s\n' "$packages"
    return 0
  done
  return 1
}

ryoku_cachyos_kernel_has_nvidia() {
  local pkgbase version
  for pkgbase in /mnt/usr/lib/modules/*/pkgbase; do
    [[ -r $pkgbase ]] || continue
    [[ $(<"$pkgbase") == linux-cachyos ]] || continue
    version=${pkgbase%/pkgbase}
    version=${version##*/}
    arch-chroot /mnt modinfo -k "$version" nvidia >/dev/null 2>&1 && return 0
  done
  return 1
}

ryoku_cachyos_target_uses_nvidia_dkms() {
  local package
  for package in nvidia-open-dkms nvidia-dkms nvidia-580xx-dkms nvidia-470xx-dkms; do
    arch-chroot /mnt pacman -Q "$package" >/dev/null 2>&1 && return 0
  done
  return 1
}

# Kernel transactions run while pacman's mkinitcpio install hook is masked.
# Replay the packaged hook once with every installed kernel so /boot images and
# presets cannot lag behind the modules that pacman just upgraded.
ryoku_cachyos_refresh_kernel_assets() {
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log 'DRYRUN: feed every installed usr/lib/modules/<ver>/vmlinuz with a pkgbase to /usr/share/libalpm/scripts/mkinitcpio install so /boot kernel images and presets match the upgraded modules'
    return 0
  fi

  local target=${RYOKU_CACHYOS_TARGET_ROOT:-/mnt}
  local target_chroot=${RYOKU_TARGET_CHROOT:-arch-chroot}
  local pkgbase module_dir version source
  local -a targets=()
  for pkgbase in "$target"/usr/lib/modules/*/pkgbase; do
    [[ -r $pkgbase ]] || continue
    module_dir=${pkgbase%/pkgbase}
    version=${module_dir##*/}
    source="$module_dir/vmlinuz"
    if [[ ! -r $source ]]; then
      log 'CachyOS layer: kernel %s has no packaged vmlinuz source at %s' "$(<"$pkgbase")" "$source"
      return 1
    fi
    targets+=("usr/lib/modules/$version/vmlinuz")
  done
  if (( ${#targets[@]} == 0 )); then
    log 'CachyOS layer: no installed kernel pkgbase files were found; kernel boot assets were not refreshed'
    return 1
  fi

  log 'CachyOS layer: refreshing /boot images and presets for every installed kernel'
  printf '%s\n' "${targets[@]}" |
    "$target_chroot" "$target" /usr/share/libalpm/scripts/mkinitcpio install
}

ryoku_cachyos_layer() {
  [[ ${RYOKU_VARIANT:-plain} == plain && ${RYOKU_CACHYOS_LAYER:-0} == 1 ]] || return 0
  RYOKU_CACHYOS_LAYER_ACTIVE=0
  RYOKU_CACHYOS_DEFAULT_KERNEL=0
  export RYOKU_CACHYOS_LAYER_ACTIVE RYOKU_CACHYOS_DEFAULT_KERNEL

  local package_csv
  if ! package_csv=$(ryoku_cachyos_layer_packages); then
    log 'CachyOS layer: the cachyos-layer package row is missing; skipping the layer. Add it later from Ryostore > CachyOS Layer.'
    return 0
  fi
  local -a packages=()
  IFS=, read -ra packages <<<"$package_csv"

  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log 'DRYRUN: check the target CPU for x86-64-v3 and the CachyOS mirror for reachability, enable [cachyos-v3], [cachyos], and multilib with the shipped helpers, then run pacman -Syu for %s' "${packages[*]}"
    log 'DRYRUN: on NVIDIA, add linux-cachyos-nvidia-open for Arch nvidia-open or verify the DKMS module with modinfo; use linux-cachyos as default when its NVIDIA module is usable'
    ryoku_cachyos_refresh_kernel_assets
    RYOKU_CACHYOS_LAYER_ACTIVE=1
    RYOKU_CACHYOS_DEFAULT_KERNEL=1
    export RYOKU_CACHYOS_LAYER_ACTIVE RYOKU_CACHYOS_DEFAULT_KERNEL
    return 0
  fi
  if ! arch-chroot /mnt sh -c \
    "/lib/ld-linux-x86-64.so.2 --help 2>/dev/null | grep -q 'x86-64-v3 (supported'"; then
    log 'CachyOS layer: this CPU does not support x86-64-v3; skipping the layer'
    return 0
  fi
  local mirror_probe=https://mirror.cachyos.org/repo/x86_64_v3/cachyos-v3/cachyos-v3.db
  if ! curl -fsSI --connect-timeout 5 --max-time 10 -o /dev/null "$mirror_probe"; then
    log 'CachyOS layer: the CachyOS mirror is unreachable; skipping the layer. Add it later from Ryostore > CachyOS Layer.'
    return 0
  fi

  if ! ryoku_cachyos_layer_helper ryoku-pkg-cachyos ||
    ! ryoku_cachyos_layer_helper ryoku-pkg-multilib; then
    log 'CachyOS layer: could not add the CachyOS or multilib repository; skipping the layer. Add it later from Ryostore > CachyOS Layer.'
    return 0
  fi

  log 'CachyOS layer: installing %s' "${packages[*]}"
  if ! run arch-chroot /mnt pacman -Syu --needed --noconfirm --noprogressbar "${packages[@]}"; then
    log 'CachyOS layer: its packages did not install; the install goes on without it. Add it later from Ryostore > CachyOS Layer.'
    return 0
  fi
  local nvidia_pci='' nvidia_ready=1
  if command -v lspci >/dev/null 2>&1; then
    nvidia_pci=$(lspci -d 10de: 2>/dev/null || true)
  fi
  if [[ -z $nvidia_pci && ${RYOKU_PROFILE:-} == amd-nvidia ]]; then
    nvidia_pci=profile
  fi
  if [[ -n $nvidia_pci ]]; then
    if [[ $(arch-chroot /mnt pacman -Qq nvidia-open 2>/dev/null) == nvidia-open ]]; then
      log 'CachyOS layer: adding the linux-cachyos module for Arch nvidia-open'
      if ! run arch-chroot /mnt pacman -S --needed --noconfirm --noprogressbar linux-cachyos-nvidia-open; then
        log 'CachyOS layer: linux-cachyos-nvidia-open did not install; keeping stock linux as the default kernel'
        nvidia_ready=0
      fi
    elif ryoku_cachyos_target_uses_nvidia_dkms; then
      log 'CachyOS layer: checking the DKMS NVIDIA module for linux-cachyos'
    else
      log 'CachyOS layer: the NVIDIA GPU has no prebuilt or DKMS driver for linux-cachyos; keeping stock linux as the default kernel'
      nvidia_ready=0
    fi
  fi

  if ! ryoku_cachyos_refresh_kernel_assets; then
    log 'CachyOS layer: kernel boot assets could not be refreshed; keeping stock linux as the only boot entry'
    return 0
  fi
  if [[ ! -f /mnt/boot/vmlinuz-linux-cachyos ]]; then
    log 'CachyOS layer: /boot/vmlinuz-linux-cachyos was not created; keeping stock linux as the only boot entry'
    return 0
  fi
  RYOKU_CACHYOS_LAYER_ACTIVE=1
  export RYOKU_CACHYOS_LAYER_ACTIVE

  if [[ -n $nvidia_pci ]]; then
    (( nvidia_ready == 1 )) || return 0
    if ! ryoku_cachyos_kernel_has_nvidia; then
      log 'CachyOS layer: no usable NVIDIA module was built for linux-cachyos; keeping stock linux as the default kernel'
      return 0
    fi
  fi

  RYOKU_CACHYOS_DEFAULT_KERNEL=1
  export RYOKU_CACHYOS_DEFAULT_KERNEL
}

# The helpers ship with the desktop; the payload copy covers a target that does
# not carry them yet. A failure returns non-zero so the caller can skip.
ryoku_cachyos_layer_helper() {
  local name=$1 source="$RYOKU_REPO/system/extras/$1" target="/root/$1" rc=0
  if [[ -x /mnt/usr/bin/$name ]]; then
    run arch-chroot /mnt env -u RYOKU_PACMAN_CONF XDG_RUNTIME_DIR=/tmp RYOKU_PRIVILEGE_COMMAND= "/usr/bin/$name"
    return
  fi
  if [[ ! -x $source ]]; then
    log 'CachyOS layer: helper %s is missing' "$source"
    return 1
  fi
  if ! run cp "$source" "/mnt$target" || ! run chmod 755 "/mnt$target"; then
    return 1
  fi
  run arch-chroot /mnt env -u RYOKU_PACMAN_CONF XDG_RUNTIME_DIR=/tmp RYOKU_PRIVILEGE_COMMAND= "$target" || rc=$?
  run rm -f "/mnt$target"
  return "$rc"
}
