#!/usr/bin/env bash
# shellcheck shell=bash
# Make the installed target a CachyOS system for future updates: add CPU-compatible
# CachyOS repositories above [core], enable [multilib] for the 32-bit gaming/driver
# set (proton-cachyos, lib32-*), and trust the CachyOS key from the installed
# cachyos-keyring (offline, no keyserver).
#
# The install ITSELF pulls every package (including the CachyOS kernel + proton)
# from the baked [offline] repo (lib/offline.sh); this step only wires future
# `pacman -Syu` to CachyOS so the box stays a CachyOS box.

ryoku_cachyos_repo() {
  local conf=/mnt/etc/pacman.conf v3=0
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log "DRYRUN: select CPU-compatible CachyOS repos, enable [multilib], populate the cachyos keyring in the target"
    return 0
  fi
  [[ -f $conf ]] || { log 'cachyos: no %s in the target yet; skipping repo config' "$conf"; return 0; }
  log "cachyos: configuring the CachyOS repositories in the target"
  if LC_ALL=C /lib/ld-linux-x86-64.so.2 --help 2>/dev/null |
      grep -qF 'x86-64-v3 (supported'; then
    v3=1
    ryoku_cachyos_arch "$conf"
  fi
  ryoku_cachyos_multilib "$conf"
  ryoku_cachyos_repos "$conf" "$v3"
  ryoku_cachyos_keyring
}

# allow x86_64_v3 packages so pacman accepts the v3 repos (stock resolves
# Architecture = auto to x86_64 and would reject them). append, never rewrite a
# commented line (it could sit inside a repo section and break the parse there).
ryoku_cachyos_arch() {
  local conf=$1
  grep -qE '^Architecture[[:space:]]*=.*x86_64_v3' "$conf" && return 0
  if grep -qE '^Architecture[[:space:]]*=' "$conf"; then
    run sed -i 's|^Architecture[[:space:]]*=.*|& x86_64_v3|' "$conf"
  else
    run sed -i 's|^\[options\]$|[options]\nArchitecture = auto x86_64_v3|' "$conf"
  fi
}

# enable [multilib] for the 32-bit gaming/driver packages (proton-cachyos,
# lib32-*). uncomment the stock commented block if present, else append it.
ryoku_cachyos_multilib() {
  local conf=$1
  grep -qE '^\[multilib\]' "$conf" && return 0
  if grep -qE '^#\[multilib\]' "$conf"; then
    run sed -i '/^#\[multilib\]/,/^#Include[[:space:]]*=.*mirrorlist/ s|^#||' "$conf"
  else
    append_file "$conf" <<'EOF'

[multilib]
Include = /etc/pacman.d/mirrorlist
EOF
  fi
  grep -qE '^\[multilib\]' "$conf" || log "cachyos: warning, could not enable [multilib] (lib32 updates will be unavailable)"
}

# add CPU-compatible CachyOS repos above [core]. Baseline x86-64 gets only
# [cachyos]; v3-capable CPUs keep the optimized core/extra rebuilds.
ryoku_cachyos_repos() {
  local conf=$1 v3=${2:-1}
  grep -qE '^\[cachyos' "$conf" && { log "cachyos: repositories already present"; return 0; }
  if [[ $v3 != 1 ]]; then
    if grep -qE '^\[core\]' "$conf"; then
      run sed -i "0,/^\[core\]/ s|^\[core\]|[cachyos]\nInclude = /etc/pacman.d/cachyos-mirrorlist\n\n[core]|" "$conf"
    else
      append_file "$conf" <<'EOF'

[cachyos]
Include = /etc/pacman.d/cachyos-mirrorlist
EOF
    fi
    grep -qE '^\[cachyos\]' "$conf" || log 'cachyos: warning, could not add the CachyOS repo to %s' "$conf"
    return 0
  fi
  grep -qE '^\[core\]' "$conf" || { log 'cachyos: no [core] anchor in %s; appending the CachyOS repos' "$conf"; ryoku_cachyos_repos_append "$conf"; return 0; }
  run sed -i "0,/^\[core\]/ s|^\[core\]|[cachyos-v3]\nInclude = /etc/pacman.d/cachyos-v3-mirrorlist\n\n[cachyos-core-v3]\nInclude = /etc/pacman.d/cachyos-v3-mirrorlist\n\n[cachyos-extra-v3]\nInclude = /etc/pacman.d/cachyos-v3-mirrorlist\n\n[cachyos]\nInclude = /etc/pacman.d/cachyos-mirrorlist\n\n[core]|" "$conf"
  grep -qE '^\[cachyos-v3\]' "$conf" || log 'cachyos: warning, could not add the CachyOS repos to %s' "$conf"
}

# fallback when the conf has no [core] to anchor above (unexpected): append the
# same repos at the end so they are still configured for future updates.
ryoku_cachyos_repos_append() {
  append_file "$1" <<'EOF'

[cachyos-v3]
Include = /etc/pacman.d/cachyos-v3-mirrorlist

[cachyos-core-v3]
Include = /etc/pacman.d/cachyos-v3-mirrorlist

[cachyos-extra-v3]
Include = /etc/pacman.d/cachyos-v3-mirrorlist

[cachyos]
Include = /etc/pacman.d/cachyos-mirrorlist
EOF
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
