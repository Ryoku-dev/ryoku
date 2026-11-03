# Migration: Add the unified snip menu keybind
# Ctrl+Shift+S opens the region selector with the in-overlay toolbar, from which
# the user picks the action (screenshot/edit/OCR/search/record) and scope
# (region/window/fullscreen/color picker). Niri only — on Hyprland the shell
# exposes a "regionMenu" global shortcut the user binds themselves.

MIGRATION_ID="030-unified-snip-keybind"
MIGRATION_TITLE="Unified snip menu keybind"
MIGRATION_DESCRIPTION="Binds Ctrl+Shift+S to the unified snip menu (one entry point for
  screenshot, edit, OCR, visual search, recording, fullscreen and color picker).
  Skipped if Ctrl+Shift+S is already bound."
MIGRATION_TARGET_FILE="~/.config/niri/config.d/70-binds.kdl"
MIGRATION_REQUIRED=false

migration_check() {
  local binds_file="${XDG_CONFIG_HOME:-$HOME/.config}/niri/config.d/70-binds.kdl"
  [[ -f "$binds_file" ]] || return 1

  # Apply only if this is an iNiR-managed binds file (has the region binds),
  # the unified menu bind isn't present yet, and Ctrl+Shift+S is still free
  # (niri rejects duplicate keybinds, which would break the whole config).
  grep -q 'region" "screenshot"' "$binds_file" 2>/dev/null || return 1
  grep -q 'region" "menu"' "$binds_file" 2>/dev/null && return 1
  grep -q 'Ctrl+Shift+S' "$binds_file" 2>/dev/null && return 1
  return 0
}

migration_preview() {
  echo -e "In 70-binds.kdl:"
  echo -e "${STY_GREEN}+ Ctrl+Shift+S { spawn \"inir\" \"region\" \"menu\"; }${STY_RST}"
}

migration_apply() {
  migration_check || return 0
  local binds_file="${XDG_CONFIG_HOME:-$HOME/.config}/niri/config.d/70-binds.kdl"

  # Insert right after the region "search" bind, falling back to "screenshot".
  if grep -q 'region" "search"' "$binds_file"; then
    sed -i '/region" "search";/a\    Ctrl+Shift+S { spawn "inir" "region" "menu"; }' "$binds_file"
  else
    sed -i '/region" "screenshot";/a\    Ctrl+Shift+S { spawn "inir" "region" "menu"; }' "$binds_file"
  fi
}
