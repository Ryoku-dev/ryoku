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
host="$(command -v ryoku-host)"
init="$("$host" init)"
case "$init" in
  systemd|runit) ;;
  *) printf 'dictation: unsupported init system: %s\n' "$init" >&2; exit 1 ;;
esac

if ! command -v cmake >/dev/null 2>&1 || ! command -v ninja >/dev/null 2>&1 ||
   ! command -v c++ >/dev/null 2>&1 || ! command -v glslc >/dev/null 2>&1 ||
   ! pkg-config --exists libpipewire-0.3 wayland-client xkbcommon vulkan; then
  case "$("$host" pkgmgr)" in
    pacman)
      sudo pacman -S --needed --noconfirm cmake ninja gcc pkgconf libpipewire wayland libxkbcommon shaderc vulkan-headers vulkan-icd-loader
      ;;
    xbps)
      sudo xbps-install -Sy cmake ninja gcc pkgconf pipewire-devel wayland-devel libxkbcommon-devel shaderc Vulkan-Headers vulkan-loader-devel
      ;;
    dnf)
      sudo "$host" pkg install cmake ninja-build gcc-c++ pkgconf-pkg-config pipewire-devel wayland-devel libxkbcommon-devel glslc vulkan-headers vulkan-loader-devel
      ;;
    *)
      printf 'dictation: the native compiler, PipeWire, Wayland, xkbcommon and Vulkan development tools are required\n' >&2
      exit 1
      ;;
  esac
fi

export RYOKU_SRC="$repo" RYOKU_BUILD_DIR="$here"
"$payload" build
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
"$payload" install "$stage" "$init"
install -Dm644 "$repo/system/services/manifest.tsv" "$stage/usr/share/ryoku/services.tsv"
if [[ $init == runit ]]; then
  install -d -m755 "$stage/usr/lib/ryoku/runit/user/ryoku-dictate"
  cp -a "$repo/void/init/user/ryoku-dictate/." "$stage/usr/lib/ryoku/runit/user/ryoku-dictate/"
fi

dest="${DESTDIR:-/}"
if [[ $dest == / && $EUID != 0 ]]; then
  sudo cp -a --no-preserve=ownership --remove-destination "$stage/usr/." /usr/
else
  mkdir -p "$dest/usr"
  cp -a --no-preserve=ownership --remove-destination "$stage/usr/." "$dest/usr/"
fi

if [[ $dest == / ]]; then
  if [[ $init == systemd ]]; then
    "$host" svc --user daemon-reload
  fi
  if (( reload )) && "$host" svc --user is-active ryoku-dictate.service >/dev/null 2>&1; then
    "$host" svc --user restart ryoku-dictate.service
  fi
fi
