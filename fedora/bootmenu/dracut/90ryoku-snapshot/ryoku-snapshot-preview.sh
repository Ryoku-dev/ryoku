#!/usr/bin/bash
#
# pre-pivot hook. rd.ryoku.preview=<top-level path of a snapshot> boots that
# read-only snapshot for a look: the snapshot is the lower layer of an overlay
# whose writes go to RAM, so the system runs normally and nothing on disk
# changes. sysroot.mount has already mounted the snapshot read-only (the
# entry's rootflags name it); the overlay is stacked on top of /sysroot, and
# its lower layer is a second mount under /run, which switch-root carries
# into the booted system. fstab's options for / name a btrfs subvolume, which
# the overlay refuses on remount, so systemd-remount-fs is masked for this
# boot only (/run).
#
# dracut sources hooks, so this returns and never exits.

# shellcheck source=/dev/null
type getarg >/dev/null 2>&1 || . /lib/dracut-lib.sh

ryoku_snapshot_preview() {
  local snap dev run="${RYOKU_RUN:-/run}" newroot="${NEWROOT:-/sysroot}"
  local base="$run/ryoku/preview"
  snap=$(getarg rd.ryoku.preview=) || return 0
  case "$snap" in
    "" | /* | *..* | *[!A-Za-z0-9._/-]*) warn "ryoku: refusing snapshot path '$snap'"; return 0 ;;
  esac
  dev=$(label_uuid_to_dev "$(getarg root=)")
  mkdir -p "$base/lower" "$base/rw"
  if ! mount -t btrfs -o "ro,subvol=$snap" "$dev" "$base/lower"; then
    warn "ryoku: cannot mount $snap; booting it read-only as it is"
    return 0
  fi
  mount -t tmpfs -o mode=0755 ryoku-preview "$base/rw"
  mkdir -p "$base/rw/upper" "$base/rw/work"
  if ! mount -t overlay ryoku-preview \
    -o "lowerdir=$base/lower,upperdir=$base/rw/upper,workdir=$base/rw/work" "$newroot"; then
    warn "ryoku: overlay failed; booting $snap read-only as it is"
    umount "$base/rw" "$base/lower"
    return 0
  fi
  mkdir -p "$run/systemd/system"
  ln -sf /dev/null "$run/systemd/system/systemd-remount-fs.service"
  info "ryoku: previewing $snap; changes stay in RAM"
}

ryoku_snapshot_preview
