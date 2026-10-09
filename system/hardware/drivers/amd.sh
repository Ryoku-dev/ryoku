#!/usr/bin/env bash
#
# amd.sh: AMD graphics drivers if an AMD GPU is present.
#
# pure open Mesa stack: mesa (OpenGL + amdgpu userspace), vulkan-radeon
# (RADV). no proprietary blob.
#
# idempotent, gated on the GPU, dry-run via RYOKU_DRYRUN.

set -euo pipefail

RYOKU_DRYRUN="${RYOKU_DRYRUN:-0}"
for arg in "$@"; do
  case "$arg" in
    --dry-run) RYOKU_DRYRUN=1 ;;
    -h | --help)
      echo "Usage: amd.sh [--dry-run]"
      echo "Install AMD Mesa/Vulkan drivers if an AMD GPU is present."
      exit 0
      ;;
    *)
      echo "amd.sh: unknown argument: $arg" >&2
      exit 1
      ;;
  esac
done

run() {
  if [[ $RYOKU_DRYRUN == 1 ]]; then
    printf 'DRYRUN: %s\n' "$*"
    return 0
  fi
  "$@"
}

PKG=(ryoku-host pkg)
(( EUID == 0 )) || PKG=(sudo -n ryoku-host pkg)

pkg_installed() { ryoku-host pkg installed "$1" >/dev/null 2>&1; }

install_pkgs() {
  local missing=() p
  for p in "$@"; do pkg_installed "$p" || missing+=("$p"); done
  if (( ${#missing[@]} == 0 )); then
    echo "amd.sh: already installed: $*"
    return 0
  fi
  echo "amd.sh: installing ${missing[*]}"
  run "${PKG[@]}" install "${missing[@]}"
}

has_amd_gpu() {
  if command -v lspci >/dev/null 2>&1 \
    && lspci 2>/dev/null | grep -iE 'vga|3d|display' | grep -qiE 'amd|ati|radeon|advanced micro devices'; then
    return 0
  fi
  grep -Eqs '^DRIVER=(amdgpu|radeon)$' /sys/class/drm/card*/device/uevent 2>/dev/null
}

if ! has_amd_gpu; then
  echo "amd.sh: no AMD GPU detected, nothing to do."
  exit 0
fi

install_pkgs mesa vulkan-radeon
