#!/usr/bin/env bash
# shellcheck shell=bash
# Carry the live session's network setup into the installed system so wifi
# survives first boot. three parts, all required:
#
#   1) pin NetworkManager's wifi backend to iwd in the target. base set ships
#      iwd (not wpa_supplicant), and the live ISO already runs NM over iwd
#      (installation/iso/airootfs/etc/NetworkManager/conf.d/wifi-backend.conf).
#      that drop-in lives only on the ISO. without an equivalent in the target,
#      NM falls back to wpa_supplicant, finds nothing, wifi can't associate.
#
#   2) copy the saved connection profiles NM wrote while the user joined a
#      network during install (/etc/NetworkManager/system-connections/
#      *.nmconnection) into the target, preserving the 600 root:root perms NM
#      requires. otherwise the credentials evaporate on reboot.
#
#   3) pin the Wi-Fi regulatory domain (the country) in the target. On world
#      domain 00 the kernel disables or no-IRs most 5 GHz channels, so a dual-band
#      SSID is only ever seen on 2.4 GHz -- the "can only connect to 2.4 GHz" bug.
#      It lives here rather than with the other chroot config because it drives
#      ryoku-wifi-regdom, which reaches the target with the desktop set that
#      ryoku_deploy installs one call earlier. Best-effort: an ISO whose baked
#      desktop set predates the helper skips this, and `ryoku doctor` pins it.
#
# ryoku_network runs in the "configure" stage; ryoku_ensure_dns runs at
# preflight, before the disk is touched. everything routes through the dry-run
# wrappers.

: "${RYOKU_CHROOT_CMD:=arch-chroot}"

ryoku_network() {
  log "persisting NetworkManager configuration into the target"
  ryoku_network_backend
  ryoku_network_connections
  ryoku_network_regdom
}

# ryoku_network_backend carries the live session's NM Wi-Fi backend into the
# target. iwd is the Ryoku default, but the installer falls back to
# wpa_supplicant when iwd cannot associate (WPA3-SAE / phone hotspots), so pin
# whichever backend is live here; otherwise a machine that needed wpa_supplicant
# to get online would drop offline on first boot. Defaults to iwd when the live
# session left no drop-in.
ryoku_network_backend() {
  local backend=iwd
  local live=/etc/NetworkManager/conf.d/wifi-backend.conf
  if [[ -r $live ]] && grep -qiE '^[[:space:]]*wifi\.backend[[:space:]]*=[[:space:]]*wpa_supplicant' "$live"; then
    backend=wpa_supplicant
  fi
  log 'pinning NetworkManager Wi-Fi backend to %s (carried from the live session)' "$backend"
  run mkdir -p /mnt/etc/NetworkManager/conf.d
  write_file /mnt/etc/NetworkManager/conf.d/wifi-backend.conf <<EOF
# Ryoku pins NetworkManager's Wi-Fi backend. iwd is the default; wpa_supplicant
# is carried over when the install needed it (WPA3-SAE / phone hotspots iwd can
# not associate with). Switch it in Settings -> Connections (ryoku-wifi-backend).
[device]
wifi.backend=$backend
EOF
}

# ryoku_network_connections: copy the saved .nmconnection keyfiles from the
# live session into the target with the ownership/perms NM demands.
ryoku_network_connections() {
  local srcdir=/etc/NetworkManager/system-connections
  local dst=/mnt/etc/NetworkManager/system-connections
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    printf 'DRYRUN: copy saved network profiles %s/*.nmconnection -> %s/ (chmod 600, chown root:root)\n' \
      "$srcdir" "$dst"
    return 0
  fi
  [[ -d $srcdir ]] || { log 'skip: %s not present' "$srcdir"; return 0; }
  local files=()
  shopt -s nullglob
  files=("$srcdir"/*.nmconnection)
  shopt -u nullglob
  (( ${#files[@]} )) || { log "no saved network profiles to carry over"; return 0; }
  mkdir -p "$dst"
  cp "${files[@]}" "$dst"/
  # NM refuses keyfiles that are group/world readable or not root-owned, so
  # normalize both regardless of where they came from.
  chmod 600 "$dst"/*.nmconnection
  chown root:root "$dst"/*.nmconnection
  log 'carried over %d saved network profile(s)' "${#files[@]}"
}

# ryoku_network_regdom pins the Wi-Fi regulatory domain (the country) in the
# target. World domain 00 disables or no-IRs most 5 GHz channels, so without a
# country the installed machine only ever sees the 2.4 GHz half of a dual-band
# SSID. Resolve one -- explicit override, else IP geolocation when online, else
# the locale's country -- and let ryoku-wifi-regdom (the single source of truth,
# shipped by the desktop set installed just above) persist it. When nothing
# resolves, leave the domain alone rather than guess.
ryoku_network_regdom() {
  local cc=""
  if [[ -n ${RYOKU_REGDOM:-} ]]; then
    cc=${RYOKU_REGDOM}
  elif [[ ${RYOKU_ONLINE:-1} == 1 ]]; then
    if [[ -n ${RYOKU_DRYRUN:-} ]]; then
      printf 'DRYRUN: curl -fsSL https://ipinfo.io/country\n'
      cc='<auto-country>'
    else
      # configure runs with the network up, so geolocation resolves even when the
      # user never joined Wi-Fi on the installer's own screens. two providers for
      # resilience; the first two-letter answer wins.
      local url
      for url in "https://ipinfo.io/country" "http://ip-api.com/line?fields=countryCode"; do
        cc=$(curl -fsSL --max-time 10 "$url" 2>/dev/null | tr -d '[:space:]') || cc=""
        [[ $cc =~ ^[A-Za-z]{2}$ ]] && break
        cc=""
      done
    fi
  fi
  # no override and no geolocation (offline, or both providers blocked): the
  # locale names a country too, and a plausible domain beats world 00.
  if [[ -z $cc && ${RYOKU_LOCALE:-} == *_* ]]; then
    cc=${RYOKU_LOCALE#*_}
    cc=${cc%%.*}
  fi
  if [[ $cc != '<auto-country>' && ! $cc =~ ^[A-Za-z]{2}$ ]]; then
    log "warn: could not resolve a Wi-Fi country (regdom stays world 00, 5 GHz limited); set RYOKU_REGDOM to override"
    return 0
  fi
  # The helper ships with the desktop set, and an ISO's baked set can predate it
  # (the bake pulls the published ryoku-desktop, not this checkout), so it is not
  # guaranteed to be in the target. Skip rather than die: `ryoku doctor` pins the
  # domain on the installed box.
  if [[ -z ${RYOKU_DRYRUN:-} && ! -x /mnt/usr/bin/ryoku-wifi-regdom ]]; then
    log 'Wi-Fi regulatory domain: %s not applied yet (ryoku-wifi-regdom is not in this target; '\''ryoku doctor'\'' sets it after first boot)' "$cc"
    return 0
  fi
  log 'Wi-Fi regulatory domain: %s' "$cc"
  run "$RYOKU_CHROOT_CMD" /mnt ryoku-wifi-regdom set "$cc" \
    || log 'warn: could not pin the Wi-Fi regulatory domain to %s (continuing; '\''ryoku doctor'\'' retries it)' "$cc"
}

# ryoku_ensure_dns: pacstrap and the desktop set resolve mirror hostnames, but a
# live box can have a route (so the TUI's reachability check passes) and still no
# working resolver -- NetworkManager over iwd may not populate /etc/resolv.conf,
# and the install then dies at pacstrap with "Could not resolve host" AFTER the
# disk is wiped. Verify name resolution; if it is broken, drop in public
# resolvers and re-test; on a genuinely offline box abort HERE, before the disk
# is touched, instead of stranding a wiped disk. Runs at preflight, before
# partitioning. RYOKU_RESOLV_CONF overrides the resolver file (tests only).
ryoku_ensure_dns() {
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log "dns: would verify name resolution and write fallback resolvers (1.1.1.1, 9.9.9.9, 8.8.8.8) if it fails"
    return 0
  fi
  [[ ${RYOKU_ONLINE:-1} == 1 ]] || { log "dns: offline install, skipping the resolver check"; return 0; }

  if ryoku_dns_works; then
    log "dns: name resolution works"
    return 0
  fi

  local resolv=${RYOKU_RESOLV_CONF:-/etc/resolv.conf}
  log 'dns: cannot resolve mirror hostnames; writing fallback resolvers to %s' "$resolv"
  rm -f -- "$resolv" 2>/dev/null || true
  printf 'nameserver 1.1.1.1\nnameserver 9.9.9.9\nnameserver 8.8.8.8\n' >"$resolv"

  if ryoku_dns_works; then
    log "dns: name resolution restored with the fallback resolvers"
    return 0
  fi
  die "no working DNS: cannot resolve package mirrors even with public resolvers. Connect to a working network (the Wi-Fi step, or plug in Ethernet) and retry. The disk has not been touched yet."
}

# ryoku_dns_works: can the system resolver turn a hostname into an address? uses
# the same nsswitch path pacman and curl take, so it reflects what pacstrap will
# see. tries more than one host so a single dead domain does not read as a dead
# resolver. RYOKU_DNS_PROBE_HOSTS overrides the probe set (tests only).
ryoku_dns_works() {
  local host
  for host in ${RYOKU_DNS_PROBE_HOSTS:-archlinux.org geo.mirror.pkgbuild.com}; do
    getent hosts "$host" >/dev/null 2>&1 && return 0
  done
  return 1
}

# ryoku_ensure_mirrors: DNS can work while HTTP does not (captive portal, half-up
# wifi, blocked egress). A flaky link then fails DEEP in pacstrap: a partial db
# sync resolves core but not extra and pacman reports "target not found: go"
# (the first dev package) on an already-wiped disk. Probe the actual bytes the
# install needs, both Arch's geo mirror and the [ryoku] repo, before any disk
# write. RYOKU_MIRROR_PROBE_URLS overrides the probe set (tests only).
ryoku_ensure_mirrors() {
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log "mirrors: would verify HTTP reach of the Arch geo mirror and repo.ryoku.dev before touching the disk"
    log "mirrors: on a TLS/reach failure would read the server clock (curl -kI Date header) and, if the system clock is off by > 24h, set it (date -s) and re-probe once"
    return 0
  fi
  [[ ${RYOKU_ONLINE:-1} == 1 ]] || { log "mirrors: offline install, skipping the reachability check"; return 0; }

  local url healed=0
  for url in ${RYOKU_MIRROR_PROBE_URLS:-\
https://geo.mirror.pkgbuild.com/core/os/x86_64/core.db \
https://repo.ryoku.dev/stable/x86_64/ryoku.db}; do
    curl -fsSI --retry 2 --max-time 20 -o /dev/null "$url" && continue
    # a dead-CMOS box has a badly wrong clock: TLS then rejects every cert (not
    # yet valid / expired) and pacman signatures fail too. once, read the
    # server's own clock over an unverified connection and correct ours, then
    # re-probe. best-effort; a genuine reach failure still aborts below.
    if (( healed == 0 )) && ryoku_fix_clock_skew "$url"; then
      healed=1
      curl -fsSI --retry 2 --max-time 20 -o /dev/null "$url" && continue
    fi
    die 'cannot reach %s over HTTP. The install downloads everything (base system, desktop, toolchains), so it needs a solid connection: reconnect Wi-Fi or plug in Ethernet, then retry. The disk has not been touched yet.' "$url"
  done
  log "mirrors: Arch geo mirror and repo.ryoku.dev reachable"
}

# ryoku_fix_clock_skew: a dead CMOS battery leaves the clock years off, which
# makes TLS reject every certificate (not yet valid / expired) and breaks pacman
# signature checks. read the server's own clock over an UNVERIFIED connection
# (curl -k, so the skewed cert can't block us) and, if ours is off by more than a
# day, set it from the Date header. returns 0 only when it adjusted the clock, so
# the caller re-probes; any other outcome returns non-zero (best-effort, never
# aborts the install on its own).
ryoku_fix_clock_skew() {
  local url=$1 date_hdr server_epoch now_epoch delta
  date_hdr=$(curl -ksI --max-time 20 "$url" 2>/dev/null \
    | awk -F': ' 'tolower($1)=="date"{sub(/\r$/,"",$2); print $2; exit}')
  [[ -n $date_hdr ]] || return 1
  server_epoch=$(date -u -d "$date_hdr" +%s 2>/dev/null) || return 1
  [[ $server_epoch =~ ^[0-9]+$ ]] || return 1
  now_epoch=$(date -u +%s)
  delta=$(( server_epoch - now_epoch )); (( delta < 0 )) && delta=$(( -delta ))
  (( delta > 86400 )) || return 1
  log 'clock skew: system clock is off by %ss from %s; setting it from the server Date header (%s)' "${delta}" "$url" "$date_hdr"
  run date -s "$date_hdr" || true
  return 0
}
