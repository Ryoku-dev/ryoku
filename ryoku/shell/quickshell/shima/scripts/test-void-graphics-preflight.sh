#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
probe="$repo_root/scripts/check-void-graphics.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

run_probe() {
    local dri_root="$1" sysfs_root="$2" output_file="$3"
    shift 3
    set +e
    INIR_DRI_ROOT="$dri_root" INIR_SYSFS_ROOT="$sysfs_root" bash "$probe" "$@" >"$output_file" 2>&1
    local rc=$?
    set -e
    return "$rc"
}

mkdir -p "$tmp/no-dri" "$tmp/no-sys"
INIR_DRI_ROOT="$tmp/no-dri" INIR_SYSFS_ROOT="$tmp/no-sys" INIR_SKIP_GRAPHICS_PREFLIGHT=1 \
    bash "$probe" >"$tmp/skipped.out" 2>&1
grep -Fq 'Graphics preflight skipped' "$tmp/skipped.out"

if run_probe "$tmp/no-dri" "$tmp/no-sys" "$tmp/no-render.out"; then
    printf 'FAIL: graphics preflight accepted a machine with no render node\n' >&2
    exit 1
else
    rc=$?
fi
[[ $rc -eq 2 ]]
grep -Fq 'No accessible DRM render node' "$tmp/no-render.out"

mkdir -p "$tmp/generic-dri" "$tmp/generic-sys/class/drm/renderD128" "$tmp/generic-sys/devices/gpu0"
: > "$tmp/generic-dri/renderD128"
ln -s "$tmp/generic-sys/devices/gpu0" "$tmp/generic-sys/class/drm/renderD128/device"
run_probe "$tmp/generic-dri" "$tmp/generic-sys" "$tmp/generic.out"
grep -Fq 'DRM render node available' "$tmp/generic.out"

mkdir -p "$tmp/virtio-bad-dri" "$tmp/virtio-bad-sys/class/drm/renderD128" \
    "$tmp/virtio-bad-sys/devices/gpu0/virtio0"
: > "$tmp/virtio-bad-dri/renderD128"
ln -s "$tmp/virtio-bad-sys/devices/gpu0" "$tmp/virtio-bad-sys/class/drm/renderD128/device"
printf '0100000000000000\n' > "$tmp/virtio-bad-sys/devices/gpu0/virtio0/features"
if run_probe "$tmp/virtio-bad-dri" "$tmp/virtio-bad-sys" "$tmp/virtio-bad.out"; then
    printf 'FAIL: graphics preflight accepted VirtIO without VirGL\n' >&2
    exit 1
else
    rc=$?
fi
[[ $rc -eq 3 ]]
grep -Fq 'VirtIO GPU has no VirGL support' "$tmp/virtio-bad.out"

mkdir -p "$tmp/virtio-good-dri" "$tmp/virtio-good-sys/class/drm/renderD128" \
    "$tmp/virtio-good-sys/devices/gpu0/virtio0"
: > "$tmp/virtio-good-dri/renderD128"
ln -s "$tmp/virtio-good-sys/devices/gpu0" "$tmp/virtio-good-sys/class/drm/renderD128/device"
printf '1100100000000000\n' > "$tmp/virtio-good-sys/devices/gpu0/virtio0/features"
run_probe "$tmp/virtio-good-dri" "$tmp/virtio-good-sys" "$tmp/virtio-good.out"
grep -Fq 'VirtIO VirGL render node available' "$tmp/virtio-good.out"

printf 'Void graphics preflight checks passed\n'
