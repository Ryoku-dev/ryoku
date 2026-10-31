#!/usr/bin/env bash
set -euo pipefail

: "${RYOKU_FETCH_TREE:?}"
package=hyprland-preview-share-picker
commit=7e11349d3f6ecae75b42be73d6a258503656cc8f
root="$RYOKU_FETCH_TREE/.rpm-$package"
dest="$root/$package"
rm -rf "$root"
mkdir -p "$root"
git clone -q https://github.com/WhySoBad/hyprland-preview-share-picker.git "$dest"
git -C "$dest" checkout -q --detach "$commit"
[[ $(git -C "$dest" rev-parse HEAD) == "$commit" ]]

read -r mode object protocol_commit path < <(git -C "$dest" ls-tree "$commit" lib/hyprland-protocols)
[[ $mode == 160000 && $object == commit && $path == lib/hyprland-protocols ]]
[[ $protocol_commit =~ ^[0-9a-f]{40}$ ]]
git clone -q https://github.com/hyprwm/hyprland-protocols.git "$dest/lib/hyprland-protocols"
git -C "$dest/lib/hyprland-protocols" checkout -q --detach "$protocol_commit"
[[ $(git -C "$dest/lib/hyprland-protocols" rev-parse HEAD) == "$protocol_commit" ]]

printf '%s\n' "$commit" > "$dest/.ryoku-source-commit"
printf '%s\n' "$protocol_commit" > "$dest/lib/hyprland-protocols/.ryoku-source-commit"
rm -rf "$dest/lib/hyprland-protocols/.git"
