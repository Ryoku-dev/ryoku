#!/usr/bin/env bash
set -euo pipefail

: "${RYOKU_FETCH_TREE:?}"
package=ryoku-keysounds-profiles
bundle="$RYOKU_FETCH_TREE/release/packages/$package/mechvibes.bundle"
commit=8d18b40792c7ce859a898ce0e840a0c01112888c
dest="$RYOKU_FETCH_TREE/.rpm-$package/mechvibes"
rm -rf "$dest"
mkdir -p "${dest%/*}"
git clone -q "$bundle" "$dest"
git -C "$dest" checkout -q --detach "$commit"
[[ $(git -C "$dest" rev-parse HEAD) == "$commit" ]]
printf '%s\n' "$commit" > "$dest/.ryoku-source-commit"
rm -rf "$dest/.git"
