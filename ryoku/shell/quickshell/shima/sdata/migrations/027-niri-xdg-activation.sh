#!/usr/bin/env bash
# Migration: Honor xdg-activation tokens with invalid serials
# Apps like VS Code, Firefox, etc. generate activation tokens with invalid
# serials when they're not focused. Niri drops these by default, so opening
# a file from Nautilus in VS Code (for example) won't steal focus.
# niri v25.05 added a debug flag to honor them anyway.

MIGRATION_ID="027-niri-xdg-activation"
MIGRATION_TITLE="Honor xdg-activation with invalid serial (niri)"
MIGRATION_DESCRIPTION="Adds debug { honor-xdg-activation-with-invalid-serial } to niri config so apps can steal focus when activated by another app (e.g. Nautilus open-with → VS Code)."
MIGRATION_REQUIRED=true

_niri_config_dir() {
  echo "${XDG_CONFIG_HOME:-$HOME/.config}/niri"
}

migration_check() {
  local niri_dir
  niri_dir="$(_niri_config_dir)"
  [[ -d "$niri_dir" ]] || return 1

  # Already present anywhere in niri config → skip
  if grep -rq 'honor-xdg-activation-with-invalid-serial' "$niri_dir" 2>/dev/null; then
    return 1
  fi
  return 0
}

migration_apply() {
  local niri_dir config_file
  niri_dir="$(_niri_config_dir)"
  config_file="$niri_dir/config.kdl"
  [[ -f "$config_file" ]] || return 0

  # Idempotency: re-check before applying
  if grep -rq 'honor-xdg-activation-with-invalid-serial' "$niri_dir" 2>/dev/null; then
    return 0
  fi

  # Case 1: existing debug { } block — inject the flag inside it
  if grep -q '^debug {' "$config_file" 2>/dev/null; then
    sed -i '/^debug {$/a\    honor-xdg-activation-with-invalid-serial' "$config_file"
    return 0
  fi

  # Case 2: no debug block — insert via awk (handles multi-line safely)
  local tmp_file="${config_file}.mig027"
  local inserted=0

  # Try after hotkey-overlay closing brace
  if grep -q '^hotkey-overlay {' "$config_file" 2>/dev/null; then
    awk '
      /^hotkey-overlay \{/ { in_hotkey=1 }
      in_hotkey && /^\}/ {
        print
        print ""
        print "// Honor activation tokens even with invalid serials — lets apps like VS Code,"
        print "// Firefox, etc. steal focus when another app requests it (e.g. Nautilus open-with)."
        print "debug {"
        print "    honor-xdg-activation-with-invalid-serial"
        print "}"
        in_hotkey=0; done=1; next
      }
      { print }
    ' "$config_file" > "$tmp_file"
    mv "$tmp_file" "$config_file"
    inserted=1
  fi

  # Fallback: before first include directive
  if [[ "$inserted" -eq 0 ]] && grep -q '^include ' "$config_file" 2>/dev/null; then
    awk '
      !done && /^include / {
        print "// Honor activation tokens even with invalid serials — lets apps like VS Code,"
        print "// Firefox, etc. steal focus when another app requests it (e.g. Nautilus open-with)."
        print "debug {"
        print "    honor-xdg-activation-with-invalid-serial"
        print "}"
        print ""
        done=1
      }
      { print }
    ' "$config_file" > "$tmp_file"
    mv "$tmp_file" "$config_file"
    inserted=1
  fi

  # Last resort: append at end
  if [[ "$inserted" -eq 0 ]]; then
    printf '\n// Honor activation tokens even with invalid serials\ndebug {\n    honor-xdg-activation-with-invalid-serial\n}\n' >> "$config_file"
  fi
}

migration_preview() {
  echo "In niri config.kdl, add:"
  echo "  debug {"
  echo "      honor-xdg-activation-with-invalid-serial"
  echo "  }"
  echo ""
  echo "This lets apps steal focus across workspaces when activated by"
  echo "another app (e.g. Nautilus open-with → VS Code). Requires niri ≥ v25.05."
}
