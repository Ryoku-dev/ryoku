#!/usr/bin/env bash

_deploy_asset() { # manager src dest mode
  local manager=$1 src=$2 dest=$3 mode=$4
  if [[ $manager == pacman ]] && ! pacman -Qo "$dest" >/dev/null 2>&1; then
    if [[ -f $dest && ! -L $dest ]] && cmp -s "$src" "$dest"; then
      sudo rm -f -- "$dest"
    fi
    return 0
  fi
  cmp -s "$src" "$dest" && return 0
  sudo install -Dm"$mode" "$src" "$dest"
}

_deploy_assets() { # manager src dest
  local manager=$1 src=$2 dest=$3 file
  while IFS= read -r -d '' file; do
    _deploy_asset "$manager" "$file" "$dest/${file#"$src/"}" 644 || return
  done < <(find "$src" -type f -print0)
}
