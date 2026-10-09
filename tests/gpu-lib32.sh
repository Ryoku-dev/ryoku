#!/usr/bin/env bash
# Hermetic behavior checks for ryoku-gpu-lib32. A fake DRM tree selects the
# vendors and a fake host seam records the packages the helper requests.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
cmd="$here/../system/hardware/gpu/ryoku-gpu-lib32"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fail() { echo "FAIL: $1" >&2; exit 1; }

conf="$tmp/pacman.conf"
printf '[multilib]\nInclude = /dev/null\n' >"$conf"
fakebin="$tmp/bin"
mkdir -p "$fakebin"
capture="$tmp/packages"

cat >"$fakebin/ryoku-host" <<'EOF'
#!/bin/sh
case "${1:-}" in
  pkgmgr)
    printf '%s\n' "${RYOKU_TEST_PKGMGR:-pacman}"
    ;;
  pkg)
    shift
    [ "${1:-}" = install ] || exit 2
    shift
    while [ "$#" -gt 0 ] && [ "${1#--}" != "$1" ]; do shift; done
    printf '%s\n' "$@" >"$RYOKU_HOST_CAPTURE"
    exit "${RYOKU_HOST_INSTALL_EXIT:-0}"
    ;;
  *)
    exit 2
    ;;
esac
EOF
chmod +x "$fakebin/ryoku-host"

cat >"$fakebin/sudo" <<'EOF'
#!/bin/sh
exec "$@"
EOF
chmod +x "$fakebin/sudo"

mk_card() { # <drm-root> <cardN> <driver>
  local d="$1/$2/device"
  mkdir -p "$d"
  printf 'DRIVER=%s\nPCI_SLOT_NAME=0000:0%s:00.0\n' "$3" "${2#card}" >"$d/uevent"
}

packages_for() { # <drm-root>
  : >"$capture"
  RYOKU_PACMAN_CONF="$conf" RYOKU_GPU_DRM_ROOT="$1" \
    RYOKU_HOST_CAPTURE="$capture" PATH="$fakebin:/usr/bin:/bin" \
    bash "$cmd" >/dev/null || return
  cat "$capture"
}

root="$tmp/amd"; mk_card "$root" card0 amdgpu
out="$(packages_for "$root")"
grep -qx 'lib32-mesa' <<<"$out" || fail "amd: missing lib32-mesa baseline"
grep -qx 'lib32-vulkan-icd-loader' <<<"$out" || fail "amd: missing loader baseline"
grep -qx 'lib32-vulkan-radeon' <<<"$out" || fail "amd: missing Radeon ICD"
grep -qEx 'lib32-vulkan-(intel|nouveau)|lib32-nvidia-utils' <<<"$out" \
  && fail "amd: wrong-vendor package requested"

root="$tmp/intel"; mk_card "$root" card0 i915
grep -qx 'lib32-vulkan-intel' <<<"$(packages_for "$root")" \
  || fail "intel: missing Intel ICD"

root="$tmp/nvidia"; mk_card "$root" card0 nvidia
grep -qx 'lib32-nvidia-utils' <<<"$(packages_for "$root")" \
  || fail "nvidia: missing NVIDIA userspace"

root="$tmp/hybrid"; mk_card "$root" card0 i915; mk_card "$root" card1 nvidia
out="$(packages_for "$root")"
grep -qx 'lib32-vulkan-intel' <<<"$out" || fail "hybrid: missing Intel ICD"
grep -qx 'lib32-nvidia-utils' <<<"$out" || fail "hybrid: missing NVIDIA userspace"
[[ $(grep -cx 'lib32-mesa' <<<"$out") -eq 1 ]] || fail "hybrid: lib32-mesa not deduped"

root="$tmp/none"; mkdir -p "$root"
out="$(packages_for "$root")"
grep -qx 'lib32-mesa' <<<"$out" || fail "none: missing baseline Mesa"
grep -qE '^lib32-vulkan-(radeon|intel|nouveau)$|^lib32-nvidia-utils$' <<<"$out" \
  && fail "none: a vendor package leaked in"

offconf="$tmp/pacman-noml.conf"
printf '[core]\nInclude = /dev/null\n' >"$offconf"
printf '#!/bin/sh\nexit 0\n' >"$fakebin/ryoku-pkg-multilib"
chmod +x "$fakebin/ryoku-pkg-multilib"
if RYOKU_PACMAN_CONF="$offconf" RYOKU_GPU_DRM_ROOT="$root" \
    RYOKU_HOST_CAPTURE="$capture" PATH="$fakebin:/usr/bin:/bin" \
    bash "$cmd" >/dev/null 2>&1; then
  fail "multilib off: should have exited non-zero"
fi

root="$tmp/void"; mk_card "$root" card0 amdgpu
if ! out="$(RYOKU_TEST_PKGMGR=xbps RYOKU_HOST_INSTALL_EXIT=5 \
    RYOKU_GPU_DRM_ROOT="$root" RYOKU_HOST_CAPTURE="$capture" \
    PATH="$fakebin:/usr/bin:/bin" bash "$cmd" 2>&1)"; then
  fail "unavailable Void libraries should be non-fatal"
fi
grep -Fq '32-bit GPU libraries are not available on this distribution' <<<"$out" \
  || fail "unavailable Void libraries: missing clear message"

echo "gpu-lib32: all checks passed"
