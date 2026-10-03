#!/usr/bin/env bash
# shellcheck shell=bash
# Fully-offline install. The ISO bakes the entire package closure (base + all
# hardware + dev + CachyOS + the Ryoku desktop set) into a single [offline]
# file:// repo (installation/iso/offline-repo.sh), so pacstrap and the in-chroot
# driver step install from local disk with no network at all. Active only when
# RYOKU_ONLINE != 1 AND RYOKU_OFFLINE_REPO points at a baked repo that has a
# synced db; otherwise the online path in pacstrap.sh / deploy.sh runs unchanged.
#
# The base lays first; the desktop set, the GPU drivers and the AUR toolset then
# install as separate in-chroot transactions from the same repo, so one bad
# desktop package cannot take the base system down with it.

: "${RYOKU_OFFLINE_REPO:=}"
RYOKU_OFFLINE_REPO_NAME=offline

# ryoku_offline_active: true when this is an offline install backed by a baked
# repo (the db glob proves the repo is real, not just an empty dir).
ryoku_offline_active() {
  [[ ${RYOKU_ONLINE:-1} != 1 ]] || return 1
  [[ -n $RYOKU_OFFLINE_REPO && -d $RYOKU_OFFLINE_REPO ]] || return 1
  compgen -G "$RYOKU_OFFLINE_REPO/$RYOKU_OFFLINE_REPO_NAME.db*" >/dev/null 2>&1
}

# ryoku_offline_verify: an offline install must have a repo it can actually
# install from. RYOKU_ONLINE=0 with no usable baked repo used to sail through
# preflight and only fail at pacstrap -- after the disk was already wiped --
# because every network step politely skips itself when offline and pacstrap then
# reached for a mirror that is not there. Fail here instead, before the disk is
# touched, and say which half is missing.
ryoku_offline_verify() {
  [[ ${RYOKU_ONLINE:-1} != 1 ]] || return 0
  ryoku_offline_active && return 0
  if [[ -z $RYOKU_OFFLINE_REPO ]]; then
    die "this is an offline install (RYOKU_ONLINE=0) but RYOKU_OFFLINE_REPO is unset, so there is nothing to install from. Boot an ISO that bakes the offline repo, or connect to a network and install online."
  fi
  if [[ ! -d $RYOKU_OFFLINE_REPO ]]; then
    die 'offline install: the baked package repo is missing at %s. This image is incomplete; connect to a network to install online, or re-download the ISO.' "$RYOKU_OFFLINE_REPO"
  fi
  die 'offline install: %s has no %s.db, so pacman cannot resolve anything from it (a bake that stopped before repo-add). This image is incomplete; connect to a network to install online, or re-download the ISO.' "$RYOKU_OFFLINE_REPO" "$RYOKU_OFFLINE_REPO_NAME"
}

# ryoku_offline_prepare: write the pacstrap-time pacman.conf pointing [offline]
# at the baked repo and export RYOKU_PACMAN_CONF so lib/pacstrap.sh passes
# `pacstrap -C`. TrustAll: the packages were vetted over TLS at ISO-build time
# and re-verified against real keyrings on the target's first online -Syu.
ryoku_offline_prepare() {
  ryoku_offline_active || return 0
  local conf=${RYOKU_PACMAN_CONF:-/tmp/ryoku-offline-pacman.conf}
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log 'offline: would write %s ([offline] -> file://%s, TrustAll) and pacstrap from it' "$conf" "$RYOKU_OFFLINE_REPO"
    export RYOKU_PACMAN_CONF=$conf
    return 0
  fi
  cat >"$conf" <<EOF
[options]
Architecture = x86_64 x86_64_v3
HoldPkg = pacman glibc
ParallelDownloads = 5
SigLevel = Never
LocalFileSigLevel = Never

[$RYOKU_OFFLINE_REPO_NAME]
SigLevel = Never
Server = file://$RYOKU_OFFLINE_REPO
EOF
  export RYOKU_PACMAN_CONF=$conf
  log 'offline: pacstrap installs from the baked [offline] repo at %s' "$RYOKU_OFFLINE_REPO"
}

# ryoku_offline_pacstrap_extra: a no-op hook, kept so lib/pacstrap.sh's call site
# needs no change. The desktop set used to be folded into the offline pacstrap
# here, but that put the whole desktop (its umbrella pulls ~everything, including
# large packages) into the base transaction: one corrupt or
# conflicting desktop package then aborted the entire pacstrap as "could not lay
# the base system", bricking the install. The desktop now installs as a SEPARATE
# chroot transaction from the same [offline] repo (lib/deploy.sh), so the base
# always lays and a desktop-package failure is isolated and recoverable. omarchy
# stages its install the same way: a base set first, then the rest.
ryoku_offline_pacstrap_extra() {
  return 0
}

# The config every in-chroot offline transaction runs with, path inside the
# target. It registers [offline] and nothing else: pacman fails a transaction
# outright ("could not find database") when ANY registered repo has no synced db,
# and in a fresh target core/extra/multilib, the CachyOS repos and [ryoku] are all
# registered and unsynced.
RYOKU_OFFLINE_CHROOT_CONF=/etc/pacman.d/ryoku-offline.conf

# ryoku_offline_pacman ARGS...: pacman in the target, restricted to the baked
# repo. Every offline in-chroot install goes through this. stdin closed, like
# run(): nothing here may wait on an answer.
ryoku_offline_pacman() {
  arch-chroot /mnt pacman --config "$RYOKU_OFFLINE_CHROOT_CONF" "$@" </dev/null
}

# ryoku_offline_chroot_on: make the baked repo resolvable INSIDE the target chroot
# so the desktop set, the GPU driver step and the AUR toolset can install offline.
# bind the repo into /mnt at the same path (so file:// resolves), write the
# [offline]-only config, and add an [offline] stanza to the target pacman.conf too
# (for anything that shells out to plain pacman). best-effort: a bind failure must
# not abort an install whose drivers are already present (e.g. a VM).
ryoku_offline_chroot_on() {
  ryoku_offline_active || return 0
  local conf=/mnt/etc/pacman.conf
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log 'offline: would bind %s into the chroot, write the [offline]-only %s, and add [offline] to %s' "$RYOKU_OFFLINE_REPO" "$RYOKU_OFFLINE_CHROOT_CONF" "$conf"
    return 0
  fi
  run mkdir -p "/mnt$RYOKU_OFFLINE_REPO"
  if ! mountpoint -q "/mnt$RYOKU_OFFLINE_REPO" 2>/dev/null; then
    mount --bind "$RYOKU_OFFLINE_REPO" "/mnt$RYOKU_OFFLINE_REPO" \
      || log "offline: warning, could not bind the offline repo into the chroot (driver installs will be skipped)"
  fi
  ryoku_offline_chroot_conf
  grep -q '^\[offline\]' "$conf" 2>/dev/null && return 0
  append_file "$conf" <<EOF

[offline]
SigLevel = Never
Server = file://$RYOKU_OFFLINE_REPO
EOF
}

# chroot_conf: DBPath and CacheDir stay at their defaults, so this shares the db
# pacstrap synced and the cache the packages landed in. x86_64_v3 is for the
# CachyOS variant's v3 builds.
ryoku_offline_chroot_conf() {
  write_file "/mnt$RYOKU_OFFLINE_CHROOT_CONF" <<EOF
# Ryoku installer, offline install window only. Removed when it closes.
[options]
Architecture = x86_64 x86_64_v3
HoldPkg = pacman glibc
ParallelDownloads = 5
SigLevel = Never
LocalFileSigLevel = Never

[$RYOKU_OFFLINE_REPO_NAME]
SigLevel = Never
Server = file://$RYOKU_OFFLINE_REPO
EOF
}

# ryoku_offline_chroot_off: undo ryoku_offline_chroot_on before the install ends,
# so the installed system's pacman.conf carries only the real (http) repos and
# no dangling file:// [offline] that a later `pacman -Syu` would choke on. drop
# the synced offline db too. idempotent; safe from the failure trap.
ryoku_offline_chroot_off() {
  ryoku_offline_active || return 0
  [[ -n ${RYOKU_DRYRUN:-} ]] && { log 'offline: would strip [offline] from the target pacman.conf, drop %s, and unmount the bound repo' "$RYOKU_OFFLINE_CHROOT_CONF"; return 0; }
  rm -f "/mnt$RYOKU_OFFLINE_CHROOT_CONF" 2>/dev/null || true
  local conf=/mnt/etc/pacman.conf
  if [[ -f $conf ]]; then
    # delete the [offline] stanza: the header line and its two directive lines.
    sed -i '/^\[offline\]$/,/^Server = file:\/\//d' "$conf" 2>/dev/null || true
    # tidy a stray blank line the delete can leave at EOF.
    sed -i -e :a -e '/^\n*$/{$d;N;ba}' "$conf" 2>/dev/null || true
  fi
  rm -f /mnt/var/lib/pacman/sync/"$RYOKU_OFFLINE_REPO_NAME".db* 2>/dev/null || true
  if mountpoint -q "/mnt$RYOKU_OFFLINE_REPO" 2>/dev/null; then
    umount "/mnt$RYOKU_OFFLINE_REPO" 2>/dev/null || umount -l "/mnt$RYOKU_OFFLINE_REPO" 2>/dev/null || true
  fi
  rmdir "/mnt$RYOKU_OFFLINE_REPO" 2>/dev/null || true
}

# ryoku_offline_aur: install the AUR toolset bundled into the baked [offline] repo
# (offline-repo.sh bake_aur_set). every ISO install is offline, so this is the only
# place the AUR set an online install builds actually lands. runs inside the
# [offline] window (repo bound + db synced by pacstrap), best-effort: a tool that
# did not bake is filtered out (one unknown name would abort the whole -S batch),
# and a failed install (e.g. a DKMS module) warns instead of aborting the install.
ryoku_offline_aur() {
  ryoku_offline_active || return 0
  local aur_file="$RYOKU_REPO/system/packages/aur.packages"
  [[ -f $aur_file ]] || return 0
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log "DRYRUN: install the bundled AUR toolset from the baked [offline] repo (best-effort)"
    return 0
  fi
  local -a want=() have=()
  mapfile -t want < <(ryoku_drop_from < <(grep -vE '^[[:space:]]*(#|$)' "$aur_file"))
  (( ${#want[@]} )) || return 0
  local p
  for p in "${want[@]}"; do
    ryoku_offline_pacman -Sp "$p" >/dev/null 2>&1 && have+=("$p")
  done
  (( ${#have[@]} )) || { log "AUR: no bundled tools found in the offline repo (built best-effort at ISO time); skipping"; return 0; }
  log 'AUR: installing %d bundled tool(s) from the baked [offline] repo' "${#have[@]}"
  ryoku_offline_pacman -S --noconfirm --needed "${have[@]}" \
    || log "AUR: warning, some bundled tools did not install (e.g. a DKMS module); continuing"
  return 0
}
