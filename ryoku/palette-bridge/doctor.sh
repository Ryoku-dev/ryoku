#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
set -uo pipefail

palette=${RYOKU_PALETTE:-"${XDG_CACHE_HOME:-$HOME/.cache}/ryoku/colors.json"}
base_url=${RYOKU_PALETTE_BRIDGE_URL:-http://127.0.0.1:47616}
failed=0

ok() {
  printf 'ok   %s\n' "$1"
}

fail() {
  printf 'FAIL %s\n' "$1" >&2
  failed=1
}

for command_name in curl systemctl; do
  if ! command -v "$command_name" >/dev/null; then
    fail "required command is missing: $command_name"
  fi
done
((failed == 0)) || exit "$failed"

have_jq=true
if ! command -v jq >/dev/null; then
  have_jq=false
  printf 'WARN jq is missing; the palette-diff check will be skipped\n'
fi

if $have_jq; then
  if jq -e '
      type == "object" and
      (.primary | test("^#[0-9a-fA-F]{6}$")) and
      (.surface | test("^#[0-9a-fA-F]{6}$")) and
      (.onSurface | test("^#[0-9a-fA-F]{6}$")) and
      all(.[]; type == "string" and test("^#[0-9a-fA-F]{6}$"))
    ' "$palette" >/dev/null 2>&1; then
    ok "palette is valid: $palette"
  else
    fail "palette is missing or invalid: $palette"
  fi
else
  printf 'WARN jq is missing; palette validation skipped\n'
fi

if systemctl --user is-active --quiet ryoku-palette-bridge.service; then
  ok "canonical service is active"
elif systemctl --user is-active --quiet ryoku-spicetify-palette.service; then
  printf 'WARN legacy service is active; rerun install.sh to migrate it\n'
else
  fail "palette bridge service is inactive"
fi

if [[ $(curl --connect-timeout 2 --max-time 5 -fsS "$base_url/healthz" 2>/dev/null) == ok ]]; then
  ok "health endpoint responds"
else
  fail "health endpoint is unavailable: $base_url/healthz"
fi

if $have_jq; then
  published=$(curl --connect-timeout 2 --max-time 5 -fsS "$base_url/v1/palette" 2>/dev/null || true)
  local_normalized=$(jq -cS . "$palette" 2>/dev/null || true)
  published_normalized=$(jq -cS . <<< "$published" 2>/dev/null || true)
  if [[ -n $local_normalized && $published_normalized == "$local_normalized" ]]; then
    ok "published palette matches"
  else
    fail "published palette differs from the active palette"
  fi
else
  printf 'WARN jq is missing; the published-palette check was skipped\n'
fi

if ((failed == 0)); then
  printf 'Ryoku Palette Bridge is healthy.\n'
fi
exit "$failed"
