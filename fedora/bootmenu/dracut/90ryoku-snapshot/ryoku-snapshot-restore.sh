#!/usr/bin/bash
#
# pre-mount hook. rd.ryoku.restore=<top-level path of a snapshot> makes that
# snapshot the root subvolume again before it is mounted: the current root is
# renamed to <root>.broken-<time> (kept, so a mistaken restore is itself
# reversible) and a writable snapshot of the chosen one takes its name. The
# root subvolume is the one rootflags=subvol= names. Anything unexpected
# leaves the disk untouched and boots the current root.
#
# dracut sources hooks, so this returns and never exits.

# shellcheck source=/dev/null
type getarg >/dev/null 2>&1 || . /lib/dracut-lib.sh

ryoku_snapshot_restore() {
  local snap rootsub dev top stamp run="${RYOKU_RUN:-/run}"
  snap=$(getarg rd.ryoku.restore=) || return 0
  case "$snap" in
    "" | /* | *..* | *[!A-Za-z0-9._/-]*) warn "ryoku: refusing snapshot path '$snap'"; return 0 ;;
    */snapshot) ;;
    *) warn "ryoku: '$snap' is not a snapper snapshot"; return 0 ;;
  esac
  rootsub=$(getarg rootflags=)
  case "$rootsub" in *subvol=*) ;; *) rootsub="" ;; esac
  rootsub=${rootsub#*subvol=}
  rootsub=${rootsub%%,*}
  rootsub=${rootsub#/}
  case "$rootsub" in
    "" | *..* | */*) warn "ryoku: no root subvolume in rootflags; not restoring"; return 0 ;;
  esac
  dev=$(label_uuid_to_dev "$(getarg root=)")
  [ -e "$dev" ] || { warn "ryoku: root device '$dev' not found; not restoring"; return 0; }

  top="$run/ryoku/restore-top"
  mkdir -p "$top"
  mount -t btrfs -o subvolid=5 "$dev" "$top" || { warn "ryoku: cannot mount the btrfs top level"; return 0; }
  if [ ! -d "$top/$snap" ] || [ ! -d "$top/$rootsub" ]; then
    warn "ryoku: $snap or $rootsub is missing; not restoring"
    umount "$top"
    return 0
  fi
  stamp=$(date -u +%Y%m%d-%H%M%S)
  if ! mv "$top/$rootsub" "$top/$rootsub.broken-$stamp"; then
    warn "ryoku: could not set the current root aside; not restoring"
    umount "$top"
    return 0
  fi
  if ! btrfs subvolume snapshot "$top/$snap" "$top/$rootsub" >/dev/null; then
    warn "ryoku: snapshotting $snap failed; putting the current root back"
    mv "$top/$rootsub.broken-$stamp" "$top/$rootsub"
    umount "$top"
    return 0
  fi
  umount "$top"
  info "ryoku: restored $snap; the previous root is $rootsub.broken-$stamp"
  mkdir -p "$run/ryoku"
  echo "$snap $rootsub.broken-$stamp" >"$run/ryoku/restored"
}

ryoku_snapshot_restore
