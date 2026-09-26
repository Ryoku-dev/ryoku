#!/usr/bin/env bash
# The alongside Fast Startup gate. A hibernated or dirty Windows volume must
# stop an alongside install before the partition table changes: Windows Startup
# Repair rewrites that table on its next boot, and that repair has deleted fresh
# Linux partitions outright. The volume state comes from `ntfsresize --info`, so
# the fixtures stub ntfsresize (dirty vs clean) against a REAL ntfs partition on
# a loop device -- blkid, lsblk and sgdisk all run for real, only the volume
# probe is faked, and in both directions, so a gate that fires on a clean
# volume fails here too.
#
# needs root + loop devices + mkntfs; prints a skip and exits 0 otherwise, so a
# runner without them stays green. run: sudo bash "$0".
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$here/.."
fail() { echo "FAIL: $1" >&2; exit 1; }
skip() { echo "install-windows-faststartup: SKIP ($1)"; exit 0; }
[[ $EUID -eq 0 ]] || skip "not root; needs losetup/mkntfs (run: sudo bash $0)"
for t in losetup sgdisk partprobe udevadm truncate mkntfs ntfsresize blkid; do
  command -v "$t" >/dev/null 2>&1 || skip "missing $t"
done

img=""; loop=""; stub=""
cleanup() {
  [[ -n $loop ]] && losetup -d "$loop" 2>/dev/null || true
  [[ -n $img ]] && rm -f "$img" 2>/dev/null || true
  [[ -n $stub ]] && rm -rf "$stub" 2>/dev/null || true
}
trap cleanup EXIT

img="$(mktemp --suffix=.faststartup-test.img)"
truncate -s 512M "$img" || { rm -f "$img"; skip "cannot create a sparse file"; }
loop="$(losetup -f --show -P "$img" 2>/dev/null || true)"
[[ -n $loop ]] || { rm -f "$img"; skip "loop devices unavailable"; }
sgdisk -n 1:0:+100M -t 1:ef00 -c 1:"EFI system partition" \
       -n 2:0:+300M -t 2:0700 -c 2:"Basic data partition" "$loop" >/dev/null
partprobe "$loop"; udevadm settle
for _ in 1 2 3 4 5; do [[ -b ${loop}p2 ]] && break; sleep 0.3; udevadm settle; done
[[ -b ${loop}p2 ]] || fail "partition nodes never appeared for $loop"
mkntfs -q -f "${loop}p2" >/dev/null || fail "mkntfs could not format ${loop}p2"
[[ $(blkid -o value -s TYPE "${loop}p2" 2>/dev/null) == ntfs ]] \
  || fail "blkid does not see ntfs on ${loop}p2"

# The one faked tool. --info prints a dirty or a clean volume report; a dirty
# volume still prints its resize line, which is exactly the case the gate must
# not talk itself out of.
stub="$(mktemp -d)"
cat >"$stub/ntfsresize" <<'EOF'
#!/usr/bin/env bash
[[ ${1:-} == --info ]] || exit 0
case ${NTFS_STUB_STATE:-clean} in
  dirty) printf 'ERROR: Volume is scheduled for check.\nYou might resize at 10485760 bytes.\n' ;;
  clean) printf 'You might resize at 10485760 bytes.\n' ;;
  *)     printf 'ntfsresize: cannot read %s\n' "${2:-?}" >&2; exit 1 ;;
esac
EOF
chmod +x "$stub/ntfsresize"
# run_gate: the gate in a clean shell over the real loop disk; the caller's env
# (NTFS_STUB_STATE, RYOKU_ALLOW_DIRTY_NTFS) rides along on the env command.
run_gate() {
  # the inner script expands its own vars; shellcheck must not see them here
  # shellcheck disable=SC2016
  env PATH="$stub:$PATH" ROOT="$root" DISK="$loop" bash -c '
    source "$ROOT/installation/backend/lib/common.sh"
    source "$ROOT/installation/backend/lib/disk.sh"
    source "$ROOT/installation/backend/lib/preflight.sh"
    ryoku_windows_faststartup_gate "$DISK"
  ' 2>&1
}
# run_dirty_list: what ryoku_dirty_ntfs_on reports for the disk.
run_dirty_list() {
  # the inner script expands its own vars; shellcheck must not see them here
  # shellcheck disable=SC2016
  env PATH="$stub:$PATH" ROOT="$root" DISK="$loop" bash -c '
    source "$ROOT/installation/backend/lib/common.sh"
    source "$ROOT/installation/backend/lib/disk.sh"
    ryoku_dirty_ntfs_on "$DISK"
  ' 2>&1
}

out="$(NTFS_STUB_STATE=dirty run_gate)" && rc=0 || rc=$?
(( rc != 0 )) || fail "a dirty Windows volume must stop the alongside install (gate exited 0)"
grep -qi "Fast Startup" <<<"$out" \
  || fail "the refusal must name Fast Startup, got: $out"

list="$(NTFS_STUB_STATE=dirty run_dirty_list)"
[[ $list == "${loop}p2" ]] || fail "ryoku_dirty_ntfs_on listed '$list', want ${loop}p2"

out="$(NTFS_STUB_STATE=dirty RYOKU_ALLOW_DIRTY_NTFS=1 run_gate)" \
  || fail "RYOKU_ALLOW_DIRTY_NTFS=1 must override the gate, got: $out"

out="$(NTFS_STUB_STATE=clean run_gate)" \
  || fail "a clean Windows volume must not stop the install, got: $out"
list="$(NTFS_STUB_STATE=clean run_dirty_list)"
[[ -z $list ]] || fail "a clean volume must not be listed dirty, got: $list"

echo "install-windows-faststartup: PASS (dirty refuses, clean passes, override honoured)"
