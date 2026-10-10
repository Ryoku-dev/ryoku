#!/usr/bin/env bash
# Void-specific helpers layered on the neutral installer libraries.

VOID_XBPS_OFFLINE_MASKS=()
VOID_XBPS_MASK_MANIFEST=/mnt/etc/xbps.d/.ryoku-offline-masks

void_offline_verify() {
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log 'offline XBPS repository: would require %s/x86_64-repodata and package archives' "$RYOKU_OFFLINE_REPO"
    return 0
  fi
  [[ -r $RYOKU_OFFLINE_REPO/x86_64-repodata ]] \
    || die 'offline XBPS repository is missing %s/x86_64-repodata; the disk has not been touched' "$RYOKU_OFFLINE_REPO"
  local packages=("$RYOKU_OFFLINE_REPO"/*.xbps)
  [[ -e ${packages[0]} ]] \
    || die 'offline XBPS repository %s contains no package archives; the disk has not been touched' "$RYOKU_OFFLINE_REPO"
  [[ -x $RYOKU_RESOLVER ]] \
    || die 'Void package resolver is missing or not executable: %s' "$RYOKU_RESOLVER"
  log 'offline XBPS repository ready: %s' "$RYOKU_OFFLINE_REPO"
}

void_seed_keys() {
  log "seeding XBPS trust keys before the first target transaction"
  run mkdir -p /mnt/var/db/xbps/keys
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    printf 'DRYRUN: copy XBPS key plists from the live system and %s into /mnt/var/db/xbps/keys\n' "$RYOKU_XBPS_KEYRING_DIR"
    return 0
  fi
  local keys=() source
  for source in /var/db/xbps/keys "$RYOKU_XBPS_KEYRING_DIR"; do
    [[ -d $source ]] || continue
    shopt -s nullglob
    keys=("$source"/*.plist)
    shopt -u nullglob
    ((${#keys[@]})) && cp -f "${keys[@]}" /mnt/var/db/xbps/keys/
  done
  shopt -s nullglob
  keys=(/mnt/var/db/xbps/keys/*.plist)
  shopt -u nullglob
  ((${#keys[@]})) || die "no XBPS trust keys were available for the offline repository"
}

void_chroot_mounts_on() {
  run mkdir -p /mnt/dev /mnt/proc /mnt/sys /mnt/run
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    printf 'DRYRUN: bind /dev /proc /sys /run into /mnt and mark them rslave\n'
    return 0
  fi
  local tree
  for tree in dev proc sys run; do
    mountpoint -q "/mnt/$tree" || mount --rbind "/$tree" "/mnt/$tree"
    mount --make-rslave "/mnt/$tree"
  done
}

void_chroot() {
  run chroot /mnt "$@"
}

void_offline_target_on() {
  run mkdir -p /mnt/etc/xbps.d
  write_file /mnt/etc/xbps.d/00-ryoku-offline.conf <<EOF
repository=$RYOKU_OFFLINE_REPO
EOF
  [[ ${RYOKU_ONLINE:-0} != 1 ]] || return 0

  # These files can arrive after the base transaction through Void's repository
  # packages or ryoku-keyring, so include them even when they are not present yet.
  local -a candidates=(
    00-repository-main.conf
    10-repository-nonfree.conf
    10-repository-multilib.conf
    10-repository-multilib-nonfree.conf
    20-ryoku.conf
  )
  local config name
  shopt -s nullglob
  for config in /mnt/usr/share/xbps.d/*.conf; do
    if grep -qE '^[[:space:]]*repository[[:space:]]*=' "$config"; then
      candidates+=("${config##*/}")
    fi
  done
  shopt -u nullglob

  local -A seen=()
  VOID_XBPS_OFFLINE_MASKS=()
  for name in "${candidates[@]}"; do
    [[ -n $name && $name == "${name##*/}" && $name == *.conf ]] \
      || die 'invalid XBPS repository mask name: %s' "$name"
    [[ -z ${seen[$name]:-} ]] || continue
    seen[$name]=1
    VOID_XBPS_OFFLINE_MASKS+=("$name")
  done

  log 'masking target XBPS remotes for the offline install'
  for name in "${VOID_XBPS_OFFLINE_MASKS[@]}"; do
    run install -Dm644 /dev/null "/mnt/etc/xbps.d/$name"
  done
  printf '%s\n' "${VOID_XBPS_OFFLINE_MASKS[@]}" \
    | write_file "$VOID_XBPS_MASK_MANIFEST"
}

void_offline_target_off() {
  local -a masks=()
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    masks=("${VOID_XBPS_OFFLINE_MASKS[@]}")
  elif [[ -f $VOID_XBPS_MASK_MANIFEST ]]; then
    mapfile -t masks <"$VOID_XBPS_MASK_MANIFEST"
  fi

  local name
  for name in "${masks[@]}"; do
    [[ -n $name && $name == "${name##*/}" && $name == *.conf ]] \
      || die 'invalid recorded XBPS repository mask name: %s' "$name"
    run rm -f "/mnt/etc/xbps.d/$name"
  done
  run rm -f "$VOID_XBPS_MASK_MANIFEST"
}

void_remote_repo_url() {
  if [[ -n ${RYOKU_XBPS_REPO:-} ]]; then
    printf '%s' "$RYOKU_XBPS_REPO"
  elif [[ ${RYOKU_REF:-} == unstable-dev ]]; then
    printf '%s' 'https://repo.ryoku.dev/stable/void/channels/testing/x86_64'
  else
    printf '%s' 'https://repo.ryoku.dev/stable/void/x86_64'
  fi
}

void_repo_finalize() {
  local remote
  remote=$(void_remote_repo_url)
  log 'switching the installed system to Void mirrors and Ryoku repository %s' "$remote"
  if [[ ${RYOKU_ONLINE:-0} != 1 || -f $VOID_XBPS_MASK_MANIFEST ]]; then
    void_offline_target_off
  fi
  run rm -f /mnt/etc/xbps.d/00-ryoku-offline.conf
  write_file /mnt/etc/xbps.d/20-ryoku.conf <<EOF
repository=$remote
EOF
  # Repository indexes are disposable. Drop the local-medium cache so the next
  # update must sync the configured remote mirrors rather than retain a path
  # below /run/initramfs/live.
  run_sh "rm -f /mnt/var/db/xbps/*-repodata /mnt/var/db/xbps/*-repodata.sig2 2>/dev/null || true"
  if [[ -z ${RYOKU_DRYRUN:-} ]]; then
    if grep -R -F "$RYOKU_OFFLINE_REPO" /mnt/etc/xbps.d /mnt/usr/share/xbps.d >/dev/null 2>&1; then
      die "the installed repository configuration still references the live medium"
    fi
  fi
}

void_enable_service() {
  local name=$1
  if [[ -n ${RYOKU_DRYRUN:-} || -d /mnt/etc/sv/$name ]]; then
    run mkdir -p /mnt/etc/runit/runsvdir/default
    run ln -sfn "/etc/sv/$name" "/mnt/etc/runit/runsvdir/default/$name"
  fi
}

void_resolve() {
  "$RYOKU_RESOLVER" "$@"
}
