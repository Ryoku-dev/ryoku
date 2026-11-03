#!/usr/bin/env bash

MIGRATION_ID="022-service-compositor-wants"
MIGRATION_TITLE="Wire inir.service to compositor instead of graphical-session.target"
MIGRATION_DESCRIPTION="Moves the inir.service wants link from graphical-session.target.wants/ to the detected compositor service (e.g. niri.service.wants/). Prevents inir from starting under KDE, GNOME, or other DEs."
MIGRATION_TARGET_FILE="~/.config/systemd/user/*.wants/inir.service"
MIGRATION_REQUIRED=true

# Source the predicate helper when migrations run outside the installer.
_migration_repo_root="${REPO_ROOT:-$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)}"
# shellcheck source=/dev/null
source "${_migration_repo_root}/sdata/lib/functions.sh" 2>/dev/null || true

_systemd_user_dir="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"

_detect_compositor_for_migration() {
  # Same logic as detect_compositor_service in scripts/inir.
  # Never falls back to graphical-session.target — that's the whole
  # point of this migration.
  if command -v systemctl >/dev/null 2>&1; then
    if systemctl --user cat niri.service &>/dev/null; then
      printf 'niri.service\n'
      return 0
    fi
    if systemctl --user cat 'wayland-wm@Hyprland.service' &>/dev/null; then
      printf 'wayland-wm@Hyprland.service\n'
      return 0
    fi
  fi
  return 1
}

migration_check() {
  # No-op if usable systemd user manager is not available (ADR-0002).
  # Without the predicate, systemctl --user can block for 10-30s.
  has_usable_systemd_user_manager || return 1

  # Needs migration if a graphical-session.target.wants link exists.
  local old_link="${_systemd_user_dir}/graphical-session.target.wants/inir.service"

  # No old link — nothing to migrate
  [[ -e "$old_link" || -L "$old_link" ]] || return 1

  # Old link exists — needs migration (remove it regardless of compositor detection)
  return 0
}

migration_preview() {
  local target
  if target="$(_detect_compositor_for_migration)"; then
    echo -e "${STY_RED}- graphical-session.target.wants/inir.service${STY_RST}"
    echo -e "${STY_GREEN}+ ${target}.wants/inir.service${STY_RST}"
  else
    echo -e "${STY_RED}- graphical-session.target.wants/inir.service${STY_RST}"
    echo -e "${STY_YELLOW}  (no compositor detected — link will be removed but not rewired)${STY_RST}"
  fi
  echo ""
  echo "This prevents iNiR from starting under KDE, GNOME, or other desktop environments."
}

migration_apply() {
  # No-op if usable systemd user manager is not available (ADR-0002).
  # Without the predicate, systemctl --user can block for 10-30s.
  has_usable_systemd_user_manager || return 0

  local old_link="${_systemd_user_dir}/graphical-session.target.wants/inir.service"
  local service_file="${_systemd_user_dir}/inir.service"

  # Always remove the stale graphical-session.target link
  rm -f "$old_link"

  # Create a compositor-specific link if we can detect one
  local target
  if target="$(_detect_compositor_for_migration)" && [[ -f "$service_file" ]]; then
    local new_wants_dir="${_systemd_user_dir}/${target}.wants"
    mkdir -p "$new_wants_dir"
    ln -sf "$service_file" "$new_wants_dir/inir.service"
  fi

  systemctl --user daemon-reload >/dev/null 2>&1 || true
}
