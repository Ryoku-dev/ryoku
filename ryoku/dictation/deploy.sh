#!/usr/bin/env bash
set -euo pipefail

reload=1
case "${1:-}" in
  '') ;;
  --no-reload) reload=0 ;;
  *) printf 'usage: deploy.sh [--no-reload]\n' >&2; exit 2 ;;
esac

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
payload="$repo/release/packages/ryoku-dictation/payload.sh"
init="$(ryoku-host init)"
case "$init" in
  systemd|runit) ;;
  *) printf 'dictation: unsupported init system: %s\n' "$init" >&2; exit 1 ;;
esac

if ! pkg-config --exists libpipewire-0.3 wayland-client xkbcommon; then
  case "$(ryoku-host pkgmgr)" in
    pacman)
      sudo pacman -S --needed --noconfirm libpipewire wayland libxkbcommon
      ;;
    xbps)
      sudo xbps-install -Sy pipewire-devel wayland-devel libxkbcommon-devel
      ;;
    dnf)
      sudo ryoku-host pkg install pipewire-devel wayland-devel libxkbcommon-devel
      ;;
    *)
      printf 'dictation: PipeWire, Wayland and xkbcommon development files are required\n' >&2
      exit 1
      ;;
  esac
fi

export RYOKU_SRC="$repo" RYOKU_BUILD_DIR="$here"
"$payload" build
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
"$payload" install "$stage" "$init"

dest="${DESTDIR:-/}"
if [[ $dest == / && $EUID != 0 ]]; then
  sudo cp -a --no-preserve=ownership --remove-destination "$stage/usr/." /usr/
else
  mkdir -p "$dest/usr"
  cp -a --no-preserve=ownership --remove-destination "$stage/usr/." "$dest/usr/"
fi

if [[ $dest == / ]] && (( reload )); then
  ryoku-host svc --user try-restart ryoku-dictate.service
fi
