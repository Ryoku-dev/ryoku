#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
source "$script_dir/release-source.sh"

sources=${RYOKU_FETCH_SOURCES:?RYOKU_FETCH_SOURCES is required}

die() {
  printf 'fetch/ryotunes.sh: %s\n' "$*" >&2
  exit 1
}

for tool in tar gzip pnpm; do
  command -v "$tool" >/dev/null 2>&1 || die "$tool is required"
done

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

fetch_github_release_source \
  Ryoku-dev/ryotunes ryotunes "$work" "$sources" \
  '^v[0-9]+\.[0-9]+\.[0-9]+$'

version=$RYOKU_FETCH_VERSION
tar -xzf "$RYOKU_FETCH_ARCHIVE_PATH" -C "$work"
source_dir="$work/ryotunes-$version"
[[ -f $source_dir/Cargo.lock && -f $source_dir/ui/pnpm-lock.yaml ]] \
  || die "$RYOKU_FETCH_ARCHIVE does not contain the expected source tree"
grep -Eq "^version = \"$version\"$" "$source_dir/Cargo.toml" \
  || die "$RYOKU_FETCH_ARCHIVE Cargo workspace version does not match $version"

(
  cd "$source_dir/ui"
  pnpm install --frozen-lockfile \
    --virtual-store-dir=node_modules/.pnpm \
    --config.prefer-symlinked-executables=true
  rm -f node_modules/.modules.yaml node_modules/.pnpm-workspace-state-v1.json
  rm -rf .svelte-kit build
  tar --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner \
    --format=pax \
    --pax-option=exthdr.name=%d/PaxHeaders/%f,delete=atime,delete=ctime \
    -cf - node_modules |
    gzip -n > "$sources/ryotunes-$version-node_modules.tar.gz"
)
