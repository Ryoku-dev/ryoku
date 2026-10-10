#!/usr/bin/env bash

void_profile_lanes() {
  case $RYOKU_PROFILE in
    amd) printf '%s\n' hardware:amd ;;
    intel) printf '%s\n' hardware:intel ;;
    amd-nvidia) printf '%s\n' hardware:amd hardware:intel hardware:nvidia ;;
    vm) printf '%s\n' hardware:vm ;;
    *) die 'unknown RYOKU_PROFILE: %s (want amd-nvidia|amd|intel|vm)' "$RYOKU_PROFILE" ;;
  esac
}

void_resolver_drop_args() {
  local package
  local -a packages=()
  IFS=',' read -ra packages <<<"${RYOKU_DROP_PACKAGES:-}"
  for package in "${packages[@]}"; do
    if [[ -n $package ]]; then
      printf '%s\n' --drop "$package"
    fi
  done
}

void_keep_uninstalled_packages() {
  local array_name=$1
  local -n requested=$array_name
  [[ -z ${RYOKU_DRYRUN:-} ]] || return 0

  local listing
  listing=$(env XBPS_ARCH=x86_64 xbps-query -r /mnt -l) \
    || die 'could not read the installed XBPS package set'
  local -a installed=() pending=()
  mapfile -t installed < <(awk '$1 == "ii" { print $2 }' <<<"$listing")

  local package pkgver found
  for package in "${requested[@]}"; do
    found=0
    for pkgver in "${installed[@]}"; do
      if [[ $pkgver == "$package"-[0-9]* ]]; then
        found=1
        break
      fi
    done
    if ((found == 0)); then
      pending+=("$package")
    fi
  done
  requested=("${pending[@]}")
}

void_base_install() {
  void_seed_keys
  local -a args=(--lane system --session)
  local -a drop_args=()
  mapfile -t drop_args < <(void_resolver_drop_args)
  args+=("${drop_args[@]}")
  local -a packages=(base-system)
  mapfile -t -O "${#packages[@]}" packages < <(void_resolve "${args[@]}")
  mapfile -t packages < <(printf '%s\n' "${packages[@]}" | awk 'NF && !seen[$0]++')

  log 'installing Void base from the offline repository (%d packages)' "${#packages[@]}"
  run env XBPS_ARCH=x86_64 xbps-install -r /mnt -R "$RYOKU_OFFLINE_REPO" -i -Sy "${packages[@]}"
  void_offline_target_on
  void_write_fstab
}

void_write_fstab() {
  local root_uuid esp_uuid opts=${RYOKU_BTRFS_OPTS:-compress=zstd:1,noatime}
  root_uuid=$(dev_uuid "$ROOT_DEV") || die 'fstab: could not read root UUID from %s' "$ROOT_DEV"
  esp_uuid=$(dev_uuid "$ESP_DEV") || die 'fstab: could not read boot UUID from %s' "$ESP_DEV"
  log "writing UUID-based /etc/fstab"
  run mkdir -p /mnt/etc
  {
    printf 'UUID=%s\t/\tbtrfs\t%s,subvol=@\t0 0\n' "$root_uuid" "$opts"
    [[ ${RYOKU_SUBVOL_HOME:-1} == 1 ]] && printf 'UUID=%s\t/home\tbtrfs\t%s,subvol=@home\t0 0\n' "$root_uuid" "$opts"
    printf 'UUID=%s\t/var/log\tbtrfs\t%s,subvol=@log\t0 0\n' "$root_uuid" "$opts"
    printf 'UUID=%s\t%s\tbtrfs\t%s,subvol=@pkg\t0 0\n' "$root_uuid" "$RYOKU_PACKAGE_CACHE_DIR" "$opts"
    [[ ${RYOKU_SUBVOL_SNAPSHOTS:-1} == 1 ]] && printf 'UUID=%s\t/.snapshots\tbtrfs\t%s,subvol=@snapshots\t0 0\n' "$root_uuid" "$opts"
    [[ ${RYOKU_SUBVOL_BACKUPS:-0} == 1 ]] && printf 'UUID=%s\t/.backups\tbtrfs\t%s,subvol=@backups\t0 0\n' "$root_uuid" "$opts"
    if (( ${RYOKU_SWAP_GIB:-0} > 0 )); then
      printf 'UUID=%s\t/swap\tbtrfs\tnoatime,subvol=@swap\t0 0\n' "$root_uuid"
      printf '/swap/swapfile\tnone\tswap\tdefaults\t0 0\n'
    fi
    printf 'UUID=%s\t/boot\tvfat\tdefaults,noatime\t0 2\n' "$esp_uuid"
    printf 'tmpfs\t/tmp\ttmpfs\tdefaults,nosuid,nodev\t0 0\n'
  } | write_file /mnt/etc/fstab
}
