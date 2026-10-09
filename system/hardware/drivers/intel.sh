#!/usr/bin/env bash
#
# intel.sh: Intel graphics drivers, plus audio DSP firmware, if Intel is present.
#
# intel-media-driver = modern VA-API driver (iHD, Broadwell/Gen8+).
# vpl-gpu-rt        = oneVPL runtime for hw video enc/dec.
# vulkan-intel      = ANV, the Vulkan driver.
# Mesa (OpenGL) rides in from the base desktop set.
#
# idempotent, gated on the GPU, dry-run via RYOKU_DRYRUN.

set -euo pipefail

RYOKU_DRYRUN="${RYOKU_DRYRUN:-0}"
for arg in "$@"; do
  case "$arg" in
    --dry-run) RYOKU_DRYRUN=1 ;;
    -h | --help)
      echo "Usage: intel.sh [--dry-run]"
      echo "Install Intel media/Vulkan drivers if an Intel GPU is present."
      exit 0
      ;;
    *)
      echo "intel.sh: unknown argument: $arg" >&2
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
    echo "intel.sh: already installed: $*"
    return 0
  fi
  echo "intel.sh: installing ${missing[*]}"
  run "${PKG[@]}" install "${missing[@]}"
}

has_intel_gpu() {
  if command -v lspci >/dev/null 2>&1 \
    && lspci 2>/dev/null | grep -iE 'vga|3d|display' | grep -qi 'intel'; then
    return 0
  fi
  grep -Eqs '^DRIVER=(i915|xe)$' /sys/class/drm/card*/device/uevent 2>/dev/null
}

if ! has_intel_gpu; then
  echo "intel.sh: no Intel GPU detected, nothing to do."
  exit 0
fi

install_pkgs intel-media-driver vpl-gpu-rt vulkan-intel

# sof-firmware: Sound Open Firmware. Recent Intel laptops (Tiger Lake and newer)
# route onboard audio through a DSP that has no sound at all without it.
install_pkgs sof-firmware
