#!/usr/bin/env bash
set -euo pipefail

: "${RYOKU_FETCH_TREE:?}"
package=imgborders
dest="$RYOKU_FETCH_TREE/.rpm-$package/$package"
bundle="$RYOKU_FETCH_TREE/release/packages/$package/$package.bundle"
patch="$RYOKU_FETCH_TREE/fedora/packages/rpm/fetch/imgborders-hyprland-0.57.patch"
commit=08be22236144d3c91607bcfa955ed0d457f4f50b
rm -rf "$dest"
mkdir -p "${dest%/*}"
git clone -q "$bundle" "$dest"

git -C "$dest" checkout -q --detach "$commit"
[[ $(git -C "$dest" rev-parse HEAD) == "$commit" ]]
git -C "$dest" apply "$patch"
printf '%s\n' "$commit" > "$dest/.ryoku-source-commit"
rm -rf "$dest/.git"
