#!/usr/bin/env bash

vesktop_render_palette() (
  set -e
  local template="$1" carrier="$2" output="$3" temporary
  [[ -f "$carrier" ]] || {
    printf 'Current Ryoku palette not found: %s. Apply a wallpaper or theme first.\n' "$carrier" >&2
    exit 1
  }
  command -v matugen >/dev/null || { printf 'matugen is required for --vesktop\n' >&2; exit 1; }
  temporary=$(mktemp -d)
  trap 'rm -rf "$temporary"' EXIT
  jq -rn --arg input "$template" --arg output "$temporary/palette.css" \
    '"[config]\n[templates.vesktop]\ninput_path = \($input | tojson)\noutput_path = \($output | tojson)\n"' \
    > "$temporary/config.toml"
  # Render only Vesktop, without rerunning other app hooks.
  matugen json "$carrier" --config "$temporary/config.toml" --quiet
  [[ -s "$temporary/palette.css" ]] || { printf 'Matugen did not render the Vesktop palette.\n' >&2; exit 1; }
  cat "$temporary/palette.css" > "$output"
)

vesktop_strip_palette() {
  local css="$1" output="$2" strict="${3:-0}" legacy_digest
  if [[ ! -f "$css" ]]; then
    : > "$output"
    return 0
  fi
  # Keep incomplete blocks intact; setup rejects them.
  awk -v strict="$strict" '
    !inside && ($0 == "/* ryoku-palette-bridge:begin */" ||
                $0 == "/* ryoku-palette-bridge:vesktop-enabled-begin */") {
      inside=1; buffer=$0 ORS
      endmark=($0 == "/* ryoku-palette-bridge:begin */" ?
        "/* ryoku-palette-bridge:end */" : "/* ryoku-palette-bridge:vesktop-enabled-end */")
      next
    }
    inside {
      buffer=buffer $0 ORS
      if ($0 == endmark) { inside=0; buffer="" }
      next
    }
    { print }
    END { if (inside) { printf "%s", buffer; if (strict) exit 1 } }
  ' "$css" > "$output" || {
    printf 'Incomplete Palette Bridge block in %s; repair the markers before setup.\n' "$css" >&2
    return 1
  }
  # Match legacy output exactly so user CSS survives.
  legacy_digest=$(sed -n '1,/^}$/p' "$output" |
    sed -E 's/#[[:xdigit:]]{6}/#000000/g' | sha256sum)
  if [[ "${legacy_digest%% *}" == 77dc96171961a855d7a08ac36ab306a061155f724f747d5fed76f0b9c35056a2 ]]; then
    sed '1,/^}$/d' "$output" > "$output.legacy"
    mv "$output.legacy" "$output"
  fi
}

vesktop_write_palette() (
  set -e
  local css rendered="${2:-}" temporary strict=0
  css=$(realpath -m -- "$1")
  [[ -n "$rendered" ]] && strict=1
  [[ -f "$css" || -n "$rendered" ]] || exit 0
  temporary=$(mktemp -d "$(dirname "$css")/.ryoku-quickcss.XXXXXX")
  trap 'rm -rf "$temporary"' EXIT
  vesktop_strip_palette "$css" "$temporary/custom.css" "$strict"
  if [[ -n "$rendered" ]]; then
    # Keep any user @import rules ahead of the generated declarations.
    cat "$temporary/custom.css" "$rendered" > "$temporary/quickCss.css"
  else
    cat "$temporary/custom.css" > "$temporary/quickCss.css"
  fi
  if [[ -f "$css" ]]; then
    chmod --reference="$css" "$temporary/quickCss.css"
  else
    chmod 0644 "$temporary/quickCss.css"
  fi
  mv -f "$temporary/quickCss.css" "$css"
)
