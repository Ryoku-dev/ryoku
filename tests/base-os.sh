#!/usr/bin/env bash
set -euo pipefail

repo=${RYOKU_PATH:-$(cd "$(dirname "$0")/.." && pwd)}
helper=$repo/ryoku/shell/scripts/ryoku-base-os
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fail() { printf 'base-os: %s\n' "$*" >&2; exit 1; }

check() {
  local name=$1 expected=$2 release=$3 file="$tmp/$1"
  printf '%s' "$release" > "$file"
  actual=$(RYOKU_OS_RELEASE="$file" "$helper")
  [[ $actual == "$expected" ]] ||
    fail "$name resolved to $actual, want $expected"
}

check arch Arch $'ID=arch\nNAME="Arch Linux"\n'
check cachyos Arch $'ID=cachyos\nID_LIKE="linux arch"\nNAME=CachyOS\n'
check void Void $'ID=void\nNAME="Void"\n'
check missing-name Linux $'ID=unknown\n'
