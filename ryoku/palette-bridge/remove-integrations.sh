#!/usr/bin/env bash
set -euo pipefail

config_root="${XDG_CONFIG_HOME:-$HOME/.config}"
state_root="${XDG_STATE_HOME:-$HOME/.local/state}/ryoku/palette-bridge"
ownership_file="$state_root/owned-files.tsv"
overlay_apps="$config_root/ryoku/user_edits/matugen/apps.toml"
want=

usage() { printf 'Usage: %s --spotify | --vesktop | --zen\n' "${0##*/}"; }

case "${1:-}" in
  --spotify) want=spotify ;;
  --vesktop) want=vesktop ;;
  --zen) want=zen ;;
  -h|--help) usage; exit 0 ;;
  *) usage >&2; exit 2 ;;
esac

remove_owned_file() {
  local path="$1"
  [[ -f "$path" ]] && rm -f -- "$path"
}

remove_matugen_section() {
  local section="$1" temporary
  [[ -f "$overlay_apps" ]] || return 0
  temporary=$(mktemp)
  awk -v target="[$section]" '
    $0 == target { skip=1; next }
    /^\[/ && skip { skip=0 }
    !skip { print }
  ' "$overlay_apps" > "$temporary"
  install -m 0644 "$temporary" "$overlay_apps"
  rm -f "$temporary"
}

case "$want" in
  spotify)
    if command -v spicetify >/dev/null; then
      spicetify config extensions ryoku-wallpaper-colors.js- || true
      spicetify apply || true
    fi
    ;;
  vesktop)
    remove_matugen_section templates.vesktop
    settings="$config_root/vesktop/settings/settings.json"
    if [[ -f "$settings" ]] && command -v jq >/dev/null; then
      temporary=$(mktemp)
      jq '.enabledThemes = ((.enabledThemes // []) | map(select(. != "midnight-ryoku.theme.css")))' "$settings" > "$temporary"
      install -m 0644 "$temporary" "$settings"
      rm -f "$temporary"
    fi
    ;;
  zen)
    remove_matugen_section templates.zen
    ;;
esac

if [[ -f "$ownership_file" ]]; then
  while IFS=$'\t' read -r integration path; do
    [[ "$integration" == "$want" ]] || continue
    case "$want:$path" in
      spotify:"$config_root"/spicetify/Extensions/ryoku-wallpaper-colors.js|\
      vesktop:"$config_root"/ryoku/user_edits/matugen/templates/vesktop-colors.css|\
      vesktop:"$config_root"/vesktop/themes/midnight-ryoku.theme.css|\
      zen:"$config_root"/ryoku/user_edits/matugen/templates/zen.css)
        remove_owned_file "$path"
        ;;
      zen:*/chrome/userChrome.css)
        temporary=$(mktemp)
        grep -Fvx '@import "ryoku-colors.css";' "$path" > "$temporary" || true
        install -m 0644 "$temporary" "$path"
        rm -f "$temporary"
        ;;
      zen:*/user.js)
        temporary=$(mktemp)
        grep -vE 'user_pref\("(toolkit\.legacyUserProfileCustomizations\.stylesheets", true|zen\.theme\.disable-lightweight", false)\);' "$path" > "$temporary" || true
        install -m 0644 "$temporary" "$path"
        rm -f "$temporary"
        ;;
    esac
  done < "$ownership_file"
  temporary=$(mktemp)
  awk -F '\t' -v app="$want" '$1 != app' "$ownership_file" > "$temporary"
  install -m 0600 "$temporary" "$ownership_file"
  rm -f "$temporary"
fi

if [[ "$want" == vesktop || "$want" == zen ]]; then
  command -v ryoku >/dev/null && ryoku materialize
  command -v ryogami >/dev/null && ryogami wallpaper repaint
fi
printf 'Removed the %s integration files owned by Ryoku Palette Bridge.\n' "$want"
