#!/usr/bin/env bash
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
config_root="${XDG_CONFIG_HOME:-$HOME/.config}"
overlay_root="$config_root/ryoku/user_edits/matugen"
overlay_apps="$overlay_root/apps.toml"
live_apps="$config_root/matugen/apps.toml"
if [[ "$config_root" == "$HOME/.config" ]]; then
  # Literal paths written into apps.toml; Matugen expands the tilde.
  # shellcheck disable=SC2088
  matugen_template_root='~/.config/matugen/templates'
  # shellcheck disable=SC2088
  vesktop_quick_css='~/.config/vesktop/settings/quickCss.css'
else
  matugen_template_root="$config_root/matugen/templates"
  vesktop_quick_css="$config_root/vesktop/settings/quickCss.css"
fi
want_spotify=false
want_vesktop=false
want_zen=false
state_root="${XDG_STATE_HOME:-$HOME/.local/state}/ryoku/palette-bridge"
ownership_file="$state_root/owned-files.tsv"

usage() {
  printf 'Usage: %s --all | [--spotify] [--vesktop] [--zen]\n' "${0##*/}"
}

while (( $# )); do
  case "$1" in
    --all) want_spotify=true; want_vesktop=true; want_zen=true ;;
    --spotify) want_spotify=true ;;
    --vesktop) want_vesktop=true ;;
    --zen) want_zen=true ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; exit 2 ;;
  esac
  shift
done

if ! $want_spotify && ! $want_vesktop && ! $want_zen; then
  usage >&2
  exit 2
fi

prepare_matugen_overlay() {
  install -d "$overlay_root/templates"
  if [[ ! -f "$overlay_apps" ]]; then
    [[ -f "$live_apps" ]] || {
      printf 'Matugen apps file not found: %s\n' "$live_apps" >&2
      exit 1
    }
    install -m 0644 "$live_apps" "$overlay_apps"
  fi
}

record_owned() {
  local integration="$1" path="$2" temporary
  install -d "$state_root"
  temporary=$(mktemp)
  if [[ -f "$ownership_file" ]]; then
    awk -F '\t' -v app="$integration" -v path="$path" '!($1 == app && $2 == path)' "$ownership_file" > "$temporary"
  fi
  printf '%s\t%s\n' "$integration" "$path" >> "$temporary"
  install -m 0600 "$temporary" "$ownership_file"
  rm -f "$temporary"
}

set_matugen_section() {
  local section="$1" input_path="$2" output_path="$3" temporary
  temporary=$(mktemp)
  awk -v target="[$section]" '
    $0 == target { skip=1; next }
    /^\[/ && skip { skip=0 }
    !skip { print }
  ' "$overlay_apps" > "$temporary"
  {
    printf '\n[%s]\n' "$section"
    printf 'input_path = "%s"\n' "$input_path"
    printf 'output_path = "%s"\n' "$output_path"
  } >> "$temporary"
  install -m 0644 "$temporary" "$overlay_apps"
  rm -f "$temporary"
}

if $want_spotify; then
  command -v spicetify >/dev/null || { printf 'spicetify is required for --spotify\n' >&2; exit 1; }
  install -d "$config_root/spicetify/Extensions"
  install -m 0644 "$project_root/spicetify/ryoku-wallpaper-colors.js" \
    "$config_root/spicetify/Extensions/ryoku-wallpaper-colors.js"
  record_owned spotify "$config_root/spicetify/Extensions/ryoku-wallpaper-colors.js"
  if ! spicetify config extensions | grep -Fxq 'ryoku-wallpaper-colors.js'; then
    spicetify config extensions ryoku-wallpaper-colors.js
  fi
  spicetify apply
  printf 'Installed the Spotify live-palette extension.\n'
fi

if $want_vesktop; then
  command -v jq >/dev/null || { printf 'jq is required for --vesktop\n' >&2; exit 1; }
  settings="$config_root/vesktop/settings/settings.json"
  [[ -f "$settings" ]] || { printf 'Vesktop settings not found: %s\n' "$settings" >&2; exit 1; }
  prepare_matugen_overlay
  install -m 0644 "$project_root/templates/vesktop-colors.css" "$overlay_root/templates/vesktop-colors.css"
  record_owned vesktop "$overlay_root/templates/vesktop-colors.css"
  set_matugen_section templates.vesktop \
    "$matugen_template_root/vesktop-colors.css" \
    "$vesktop_quick_css"
  install -d "$config_root/vesktop/themes"
  install -m 0644 "$project_root/vesktop/midnight-ryoku.theme.css" \
    "$config_root/vesktop/themes/midnight-ryoku.theme.css"
  record_owned vesktop "$config_root/vesktop/themes/midnight-ryoku.theme.css"
  temporary=$(mktemp)
  jq '.useQuickCss = true | .enabledThemes = ((.enabledThemes // []) | if index("midnight-ryoku.theme.css") then . else . + ["midnight-ryoku.theme.css"] end)' \
    "$settings" > "$temporary"
  install -m 0644 "$temporary" "$settings"
  rm -f "$temporary"
  printf 'Installed the Vesktop no-flash Midnight integration.\n'
fi

if $want_zen; then
  zen_root="$config_root/zen"
  if [[ -n "${ZEN_PROFILE_ROOT:-}" ]]; then
    profile_root="$ZEN_PROFILE_ROOT"
  else
    profile_root=$(find "$zen_root" -mindepth 2 -maxdepth 2 -name .parentlock -printf '%h\n' -quit 2>/dev/null || true)
    if [[ -z "$profile_root" && -f "$zen_root/profiles.ini" ]]; then
      profile_path=$(awk -F= '
        /^\[Profile/ { active=1; path=""; preferred=0; next }
        /^\[/ { if (active && preferred && path != "") { print path; exit }; active=0 }
        active && $1 == "Path" { path=$2 }
        active && $1 == "Default" && $2 == "1" { preferred=1 }
        END { if (active && preferred && path != "") print path }
      ' "$zen_root/profiles.ini" | head -n1)
      [[ -n "$profile_path" ]] && profile_root="$zen_root/$profile_path"
    fi
  fi
  [[ -n "${profile_root:-}" && -d "$profile_root" ]] || {
    printf 'Could not identify a Zen profile; set ZEN_PROFILE_ROOT and retry.\n' >&2
    exit 1
  }
  prepare_matugen_overlay
  install -m 0644 "$project_root/templates/zen.css" "$overlay_root/templates/zen.css"
  record_owned zen "$overlay_root/templates/zen.css"
  if [[ "$profile_root" == "$HOME/"* ]]; then
    profile_output="~${profile_root#"$HOME"}/chrome/ryoku-colors.css"
  else
    profile_output="$profile_root/chrome/ryoku-colors.css"
  fi
  set_matugen_section templates.zen "$matugen_template_root/zen.css" "$profile_output"
  install -d "$profile_root/chrome"
  user_chrome="$profile_root/chrome/userChrome.css"
  touch "$user_chrome"
  if ! grep -Fqx '@import "ryoku-colors.css";' "$user_chrome"; then
    temporary=$(mktemp)
    printf '@import "ryoku-colors.css";\n' > "$temporary"
    sed -n '1,$p' "$user_chrome" >> "$temporary"
    install -m 0644 "$temporary" "$user_chrome"
    rm -f "$temporary"
  fi
  record_owned zen "$user_chrome"
  user_js="$profile_root/user.js"
  touch "$user_js"
  temporary=$(mktemp)
  grep -vE 'user_pref\("(toolkit\.legacyUserProfileCustomizations\.stylesheets|zen\.theme\.disable-lightweight)"' "$user_js" > "$temporary" || true
  printf 'user_pref("toolkit.legacyUserProfileCustomizations.stylesheets", true);\n' >> "$temporary"
  printf 'user_pref("zen.theme.disable-lightweight", false);\n' >> "$temporary"
  install -m 0644 "$temporary" "$user_js"
  record_owned zen "$user_js"
  rm -f "$temporary"
  printf 'Installed Zen profile wiring. Sign/install zen-extension separately, then restart Zen once.\n'
fi

if $want_vesktop || $want_zen; then
  command -v ryoku >/dev/null || { printf 'ryoku is required to materialize Matugen overlays\n' >&2; exit 1; }
  ryoku materialize
  if command -v ryogami >/dev/null; then
    ryogami wallpaper repaint
  fi
fi
