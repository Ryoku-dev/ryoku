#!/usr/bin/env bash
# Migration: Set malloc optimization env vars for Quickshell memory management
#
# Qt decodes wallpaper images in background threads. glibc malloc creates
# per-thread arenas that retain freed memory. After several wallpaper switches,
# hundreds of MB accumulate. These env vars fix the problem.

MIGRATION_ID="014-malloc-arena-optimization"
MIGRATION_TITLE="Set malloc optimization for Quickshell memory management"
MIGRATION_DESCRIPTION="Adds MALLOC_ARENA_MAX and MALLOC_MMAP_THRESHOLD_ to environment.d
  to prevent glibc malloc arenas from retaining freed wallpaper textures.
  Reduces memory growth from ~100MB/wallpaper-switch to near-zero."
MIGRATION_TARGET_FILE="~/.config/environment.d/quickshell-mem.conf"
MIGRATION_REQUIRED=true

migration_check() {
  local conf="${XDG_CONFIG_HOME:-$HOME/.config}/environment.d/quickshell-mem.conf"
  # Needs migration if the file doesn't exist or doesn't have both vars
  if [[ ! -f "$conf" ]]; then
    return 0
  fi
  ! grep -q 'MALLOC_ARENA_MAX' "$conf" || ! grep -q 'MALLOC_MMAP_THRESHOLD_' "$conf"
}

migration_preview() {
  echo "Will create ~/.config/environment.d/quickshell-mem.conf with:"
  echo ""
  echo -e "${STY_GREEN}  MALLOC_ARENA_MAX=2${STY_RST}"
  echo -e "${STY_GREEN}  MALLOC_MMAP_THRESHOLD_=131072${STY_RST}"
  echo ""
  echo "This limits glibc malloc arenas and forces large allocations (decoded"
  echo "wallpaper textures) to use mmap, so memory is returned to the OS when freed."
  echo ""
  echo "Takes effect on next login/session restart."
}

migration_diff() {
  local conf="${XDG_CONFIG_HOME:-$HOME/.config}/environment.d/quickshell-mem.conf"
  if [[ -f "$conf" ]]; then
    echo "Current file:"
    cat "$conf"
  else
    echo "File does not exist yet."
  fi
  echo ""
  echo "After migration:"
  echo "  MALLOC_ARENA_MAX=2"
  echo "  MALLOC_MMAP_THRESHOLD_=131072"
}

migration_apply() {
  local conf_dir="${XDG_CONFIG_HOME:-$HOME/.config}/environment.d"
  local conf="${conf_dir}/quickshell-mem.conf"

  mkdir -p "$conf_dir"

  cat > "$conf" << 'ENVEOF'
# Quickshell/iNiR memory optimization
# Prevents glibc malloc arenas from retaining freed wallpaper textures.
# See: scripts/quickshell-env.sh for details.
MALLOC_ARENA_MAX=2
MALLOC_MMAP_THRESHOLD_=131072
ENVEOF

  echo "  Created $conf"
  echo "  Takes effect on next login/session restart."
}
