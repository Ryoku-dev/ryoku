# Migration: Add XDG_MENU_PREFIX for Dolphin file associations
# Fixes "Open With" menu in Dolphin

MIGRATION_ID="005-dolphin-xdg-menu"
MIGRATION_TITLE="Dolphin File Associations"
MIGRATION_DESCRIPTION="Adds XDG_MENU_PREFIX environment variable for Dolphin.
  This fixes the 'Open With' menu showing applications correctly."
MIGRATION_TARGET_FILE="~/.config/niri/config.kdl"
MIGRATION_REQUIRED=false

_migration_repo_root="${REPO_ROOT:-$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)}"
# shellcheck source=/dev/null
source "${_migration_repo_root}/sdata/lib/functions.sh" 2>/dev/null || true

migration_check() {
  local config="${XDG_CONFIG_HOME:-$HOME/.config}/niri/config.kdl"
  [[ -f "$config" ]] || return 1
  if ! grep -q 'XDG_MENU_PREFIX "plasma-"' "$config"; then
    return 0
  fi
  if ! has_usable_systemd_user_manager \
      && grep -q 'systemctl --user import-environment XDG_MENU_PREFIX' "$config"; then
    return 0
  fi
  return 1
}

migration_preview() {
  echo -e "${STY_GREEN}+ XDG_MENU_PREFIX \"plasma-\"${STY_RST} (in environment block)"
  if has_usable_systemd_user_manager; then
    echo -e "${STY_GREEN}+ spawn-at-startup for systemctl import-environment${STY_RST}"
  else
    echo -e "${STY_RED}- stale systemctl user startup (non-systemd session)${STY_RST}"
  fi
}

migration_diff() {
  cat << 'DIFF'
Will add to environment block:
  XDG_MENU_PREFIX "plasma-"

Systemd-user sessions also add:
  spawn-at-startup "bash" "-c" "systemctl --user import-environment XDG_MENU_PREFIX && kbuildsycoca6"

Non-systemd sessions remove that legacy startup line if present.
DIFF
}

migration_apply() {
  local config="${XDG_CONFIG_HOME}/niri/config.kdl"

  if ! migration_check; then
    return 0
  fi

  # Add XDG_MENU_PREFIX after XDG_CURRENT_DESKTOP.
  if ! grep -q 'XDG_MENU_PREFIX "plasma-"' "$config" \
      && grep -q 'XDG_CURRENT_DESKTOP' "$config"; then
    sed -i '/XDG_CURRENT_DESKTOP/a\    XDG_MENU_PREFIX "plasma-"  // Required for Dolphin file associations' "$config"
  fi

  # Import into systemd only when its user manager is usable. Non-systemd
  # sessions inherit the Niri environment and must not keep this startup line.
  if has_usable_systemd_user_manager; then
    if ! grep -q 'import-environment XDG_MENU_PREFIX' "$config" \
        && grep -q 'spawn-at-startup' "$config"; then
      sed -i '0,/spawn-at-startup/s//spawn-at-startup "bash" "-c" "systemctl --user import-environment XDG_MENU_PREFIX \&\& kbuildsycoca6"\n\nspawn-at-startup/' "$config"
    fi
  else
    sed -i '/spawn-at-startup "bash" "-c" "systemctl --user import-environment XDG_MENU_PREFIX && kbuildsycoca6"/d' "$config"
  fi
}
