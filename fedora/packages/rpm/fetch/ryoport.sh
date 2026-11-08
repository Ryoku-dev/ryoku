#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
source "$script_dir/release-source.sh"

sources=${RYOKU_FETCH_SOURCES:?RYOKU_FETCH_SOURCES is required}

command -v tar >/dev/null 2>&1 || {
  printf 'fetch/ryoport.sh: tar is required\n' >&2
  exit 1
}

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

fetch_github_release_source \
  Ryoku-dev/ryoport ryoport "$work" "$sources" \
  '^v1\.[0-9]+\.[0-9]$'

tar -xzf "$RYOKU_FETCH_ARCHIVE_PATH" -C "$work"
source_dir="$work/ryoport-$RYOKU_FETCH_VERSION"
[[ -f $source_dir/go.mod && -f $source_dir/vendor/modules.txt && \
   -f $source_dir/cmd/ryoport/main.go ]] || {
  printf 'fetch/ryoport.sh: %s does not contain the expected vendored source tree\n' \
    "$RYOKU_FETCH_ARCHIVE" >&2
  exit 1
}
