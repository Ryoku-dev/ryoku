#!/usr/bin/env bash
# Void desktop-parity closure checks for Void desktop/default parity.
set -u

failures=0
check() {
  if "$@"; then
    printf 'PASS: %s\n' "$*"
  else
    printf 'FAIL: %s\n' "$*" >&2
    failures=$((failures + 1))
  fi
}

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
expected_branch="${INIR_EXPECTED_BRANCH:-feat/void-pr55-desktop-closure}"
expected_commit="${INIR_EXPECTED_COMMIT:-}"
actual_branch="$(git -C "$repo_root" branch --show-current 2>/dev/null || true)"
actual_commit="$(git -C "$repo_root" rev-parse HEAD 2>/dev/null || true)"

check test "$actual_branch" = "$expected_branch"
if [[ -n "$expected_commit" ]]; then
  check test "$actual_commit" = "$expected_commit"
else
  printf 'INFO: commit=%s (not pinned; set INIR_EXPECTED_COMMIT to pin it)\n' "$actual_commit"
fi
if [[ "${INIR_ALLOW_DIRTY:-false}" != true ]]; then
  check test -z "$(git -C "$repo_root" status --porcelain 2>/dev/null)"
fi

deps="$repo_root/sdata/dist-void/install-deps.sh"
files_step="$repo_root/sdata/subcmd-install/3.files.sh"

for needle in \
  'RUBIK_SHA256="1b3a7437ba2af80e465e773ed60c5036d1ba6ace492d89046dbcf18fb31e4e88"' \
  'RUBIK_ITALIC_SHA256="08c6c4018a5ada8b517407b46897e46cf6ebb106853fbd3e89addb51d3b59c62"' \
  '60-inir-void-font-aliases.conf' \
  '<family>Google Sans Flex</family>' \
  '<family>Roboto Flex</family>'; do
  if grep -Fq "$needle" "$deps"; then
    printf 'PASS: font closure contains: %s\n' "$needle"
  else
    printf 'FAIL: font closure missing: %s\n' "$needle" >&2
    failures=$((failures + 1))
  fi
done

check xbps-query -p pkgver plasma-integration

font_matches() {
  fc-match -f '%{family}\n' "$1" 2>/dev/null | head -1 | grep -Fq "$2"
}
check font_matches 'Rubik' 'Rubik'
check font_matches 'Google Sans Flex' 'Roboto Flex'
check font_matches 'Roboto Flex' 'Roboto Flex'
check font_matches 'Gabarito' 'Gabarito'
check font_matches 'Oxanium' 'Oxanium'
check font_matches 'JetBrains Mono NF' 'JetBrainsMono Nerd Font'
check font_matches 'Material Symbols Rounded' 'Material Symbols Rounded'

alias_file="${XDG_CONFIG_HOME:-$HOME/.config}/fontconfig/conf.d/60-inir-void-font-aliases.conf"
check test -s "$alias_file"
check grep -Fq '<family>Google Sans Flex</family>' "$alias_file"
check grep -Fq '<family>Roboto Flex</family>' "$alias_file"

check grep -Fq 'xbps-query -p pkgver plasma-integration' "$files_step"
check grep -Fq 's/QT_QPA_PLATFORMTHEME "qt6ct"/QT_QPA_PLATFORMTHEME "kde"/' "$files_step"

niri_env="${XDG_CONFIG_HOME:-$HOME/.config}/niri/config.d/40-environment.kdl"
if [[ -f "$niri_env" ]]; then
  check grep -Fq 'QT_QPA_PLATFORMTHEME "kde"' "$niri_env"
  if grep -Fq 'QT_QPA_PLATFORMTHEME "qt6ct"' "$niri_env"; then
    printf 'FAIL: Niri environment still forces qt6ct despite plasma-integration\n' >&2
    failures=$((failures + 1))
  else
    printf 'PASS: Niri environment does not force qt6ct\n'
  fi
else
  printf 'INFO: Niri split environment file unavailable; runtime file check skipped\n'
fi

if [[ "${INIR_VERIFY_IDEMPOTENCY:-false}" == true ]]; then
  before="$(mktemp)"
  after="$(mktemp)"
  provider_snapshot() {
    sha256sum \
      "${XDG_DATA_HOME:-$HOME/.local/share}/fonts/Rubik.ttf" \
      "${XDG_DATA_HOME:-$HOME/.local/share}/fonts/Rubik-Italic.ttf" \
      "$alias_file"
    fc-match -f '%{family}\n' 'Google Sans Flex'
  }
  provider_snapshot > "$before"
  check env ONLY_MISSING_DEPS=font-providers \
    "$repo_root/setup" install -y --skip-sysupdate --skip-setups --skip-files
  provider_snapshot > "$after"
  check cmp -s "$before" "$after"
  rm -f "$before" "$after"
else
  printf 'INFO: set INIR_VERIFY_IDEMPOTENCY=true to run the closure snapshot check\n'
fi

if ((failures > 0)); then
  printf '%d Void desktop-parity check(s) failed\n' "$failures" >&2
  exit 1
fi
printf 'All Void desktop-parity checks passed\n'
