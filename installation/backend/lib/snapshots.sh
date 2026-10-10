#!/usr/bin/env bash
# shellcheck shell=bash
# btrfs snapshots in the target = snapper "root" config for /, snap-pac
# (auto pre/post around pacman), the cleanup timer, and limine-snapper-sync
# (snapshots -> Limine entries). runs after the aur step so the aur piece
# (limine-snapper-sync) is there. all via the dry-run wrappers. no
# @snapshots subvol = no-op.

ryoku_snapshots() {
  if [[ ${RYOKU_SUBVOL_SNAPSHOTS:-1} != 1 ]]; then
    ryoku_snapshots_disabled
    return 0
  fi
  log "configuring snapper (root), snap-pac, and limine-snapper-sync"
  ryoku_snap_config
  ryoku_snap_services
  ryoku_snap_updatedb
}

ryoku_snapshots_disabled() {
  log "snapshots: @snapshots subvolume disabled, skipping snapper setup"
  # record the explicit opt-out so `ryoku doctor` (which otherwise converges
  # every btrfs root onto the canonical snapper layout) respects the choice
  # instead of silently re-enabling snapshots on the first update. deleting
  # the marker and running `ryoku doctor` enables them later.
  run mkdir -p /mnt/etc/ryoku
  write_file /mnt/etc/ryoku/snapshots-disabled <<'EOF'
# Snapshots were declined at install (RYOKU_SUBVOL_SNAPSHOTS=0). `ryoku doctor`
# leaves the snapper layout alone while this file exists; delete it and run
# `ryoku doctor` to enable snapshots.
EOF
}

# drop the snapper "root" config by hand. `snapper create-config` wants
# snapperd over D-Bus and there's no snapperd in a chroot. @snapshots is
# already mounted at /.snapshots (filesystem.sh), so all we need is:
# the file, /etc/conf.d/snapper registration, 750 root:root on the dir.
# retention = ~10 numbered (snap-pac + update pairs), no timeline (desktop).
ryoku_snap_config() {
  run mkdir -p /mnt/etc/snapper/configs
  write_file /mnt/etc/snapper/configs/root <<'EOF'
# Ryoku snapper config for the root filesystem. Written by the installer instead
# of `snapper create-config` (which needs the snapperd D-Bus daemon, absent in the
# chroot). Keys not listed here fall back to snapper's built-in defaults.
SUBVOLUME="/"
FSTYPE="btrfs"
QGROUP=""
SPACE_LIMIT="0.5"
FREE_LIMIT="0.2"
ALLOW_USERS=""
ALLOW_GROUPS=""
SYNC_ACL="no"
BACKGROUND_COMPARISON="yes"
NUMBER_CLEANUP="yes"
NUMBER_MIN_AGE="1800"
NUMBER_LIMIT="10"
NUMBER_LIMIT_IMPORTANT="10"
TIMELINE_CREATE="no"
TIMELINE_CLEANUP="yes"
TIMELINE_MIN_AGE="1800"
TIMELINE_LIMIT_HOURLY="10"
TIMELINE_LIMIT_DAILY="7"
TIMELINE_LIMIT_WEEKLY="0"
TIMELINE_LIMIT_MONTHLY="0"
TIMELINE_LIMIT_YEARLY="0"
EMPTY_PRE_POST_CLEANUP="yes"
EMPTY_PRE_POST_MIN_AGE="1800"
EOF

  # register "root" so timers + snap-pac pick it up.
  write_file /mnt/etc/conf.d/snapper <<'EOF'
## Path: System/Snapper
## Type: string
## Default: ""
# Snapper configs the systemd units and pacman hooks operate on.
SNAPPER_CONFIGS="root"
EOF

  # /.snapshots wants 750 root:root, snapper insists.
  run chmod 750 /mnt/.snapshots
  run chown root:root /mnt/.snapshots
}

# enable cleanup timer (prunes per the retention above) + limine-snapper-sync
# (snapshots -> Limine entries). limine-snapper-sync is aur, offline installs
# can be missing it, so the enable is best-effort.
ryoku_snap_services() {
  log "enabling snapper-cleanup.timer"
  run arch-chroot /mnt systemctl enable snapper-cleanup.timer

  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log "DRYRUN: arch-chroot /mnt systemctl enable limine-snapper-sync.service"
    return 0
  fi
  if [[ -f /mnt/usr/lib/systemd/system/limine-snapper-sync.service ]]; then
    log "enabling limine-snapper-sync.service"
    arch-chroot /mnt systemctl enable limine-snapper-sync.service \
      || log "snapshots: warning, could not enable limine-snapper-sync.service"
  else
    log "snapshots: limine-snapper-sync not installed (offline AUR?); boot-entry sync not enabled"
  fi
}

# prune /.snapshots from updatedb so plocate indexes the live system, not
# hundreds of frozen copies. mirrors the ryoku doctor reconciler; no conf = no-op.
ryoku_snap_updatedb() {
  local conf=/mnt/etc/updatedb.conf
  if [[ ! -f $conf ]]; then
    log "snapshots: no updatedb.conf in target, skipping locate prune"
    return 0
  fi
  if grep -q '/\.snapshots' "$conf"; then
    return 0
  fi
  log "snapshots: pruning /.snapshots from updatedb"
  if grep -q '^PRUNEPATHS=' "$conf"; then
    run sed -i 's|^PRUNEPATHS="|PRUNEPATHS="/.snapshots |' "$conf"
  else
    printf 'PRUNEPATHS="/.snapshots"\n' | run tee -a "$conf" >/dev/null
  fi
}
