#!/usr/bin/env bash
# Shared by the Fedora driver installers; sourced before hardware detection.
set -euo pipefail

RYOKU_DRYRUN=${RYOKU_DRYRUN:-0}
PRIV=()
(( EUID == 0 )) || PRIV=(sudo -n)

for arg in "$@"; do
  case "$arg" in
    --dry-run) RYOKU_DRYRUN=1 ;;
    -h | --help) echo "Usage: ${0##*/} [--dry-run]"; exit 0 ;;
    *) echo "${0##*/}: unknown argument: $arg" >&2; exit 2 ;;
  esac
done

run() {
  if [[ $RYOKU_DRYRUN == 1 ]]; then
    printf 'DRYRUN: %s\n' "$*"
    return 0
  fi
  "$@"
}

install_pkgs() {
  command -v dnf >/dev/null 2>&1 || {
    echo "${0##*/}: Fedora DNF is required" >&2
    return 1
  }
  run "${PRIV[@]}" dnf -y --exclude='*.i686' install -- "$@"
}

has_gpu_vendor() {
  local dev vendor class
  for dev in /sys/bus/pci/devices/*; do
    [[ -r $dev/vendor && -r $dev/class ]] || continue
    read -r vendor <"$dev/vendor"
    read -r class <"$dev/class"
    [[ $vendor == "$1" && $class == 0x03* ]] && return 0
  done
  return 1
}
