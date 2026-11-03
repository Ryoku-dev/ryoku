#!/usr/bin/env bash
# Void font-provider checks for Void font providers.
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
expected_branch="${INIR_EXPECTED_BRANCH:-feat/void-pr53-font-providers}"
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
for package in fontconfig nerd-fonts-ttf noto-fonts-emoji; do
  check xbps-query -p pkgver "$package"
done
for needle in \
  'MATERIAL_SYMBOLS_COMMIT="40a7a292a79d9394157e1ea24f83d52d5e17c556"' \
  'MATERIAL_SYMBOLS_SHA256="f1472f172c0fc4a922be22972e4752ccc54fe795ed82564ab6f6b097782f2dbc"' \
  'ROBOTO_FLEX_VERSION="3.200"' \
  'ROBOTO_FLEX_SHA256="6b2b14e11308c7d3e8388b623cf740c46b872e7519198e0cff8062e52b75239b"' \
  'GOOGLE_FONTS_COMMIT="a54f7446f84a1125ef6bf08baa46f3639e8905e0"' \
  'GABARITO_SHA256="8650e2bd7747f7d74619fd7aecbcb0309e6f37b7964024f3fb15ae4833b67ca5"' \
  'OXANIUM_SHA256="2ce01d946e1e1ffc8d7eecfffbda8623bedd63eaf811a20488c4b69af45babb0"' \
  'install_void_font_providers'; do
  if grep -Fq "$needle" "$deps"; then
    printf 'PASS: provider contains: %s\n' "$needle"
  else
    printf 'FAIL: provider missing: %s\n' "$needle" >&2
    failures=$((failures + 1))
  fi
done

data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
font_dir="$data_home/fonts"
check test -s "$font_dir/MaterialSymbolsRounded.ttf"
check test -s "$font_dir/RobotoFlex.ttf"
check test -s "$font_dir/Gabarito.ttf"
check test -s "$font_dir/Oxanium.ttf"
check bash -c 'printf "%s  %s\n" "$1" "$2" | sha256sum -c - >/dev/null' _ \
  f1472f172c0fc4a922be22972e4752ccc54fe795ed82564ab6f6b097782f2dbc "$font_dir/MaterialSymbolsRounded.ttf"
check bash -c 'printf "%s  %s\n" "$1" "$2" | sha256sum -c - >/dev/null' _ \
  a55c1e67f6dcf27f2bb71dc3e4c03d3abcbc5054411aac943b2f94985195825e "$font_dir/RobotoFlex.ttf"
check bash -c 'printf "%s  %s\n" "$1" "$2" | sha256sum -c - >/dev/null' _ \
  8650e2bd7747f7d74619fd7aecbcb0309e6f37b7964024f3fb15ae4833b67ca5 "$font_dir/Gabarito.ttf"
check bash -c 'printf "%s  %s\n" "$1" "$2" | sha256sum -c - >/dev/null' _ \
  2ce01d946e1e1ffc8d7eecfffbda8623bedd63eaf811a20488c4b69af45babb0 "$font_dir/Oxanium.ttf"

font_matches() {
  fc-match -f '%{family}\n' "$1" 2>/dev/null | grep -Fq "$2"
}
check font_matches 'Material Symbols Rounded' 'Material Symbols Rounded'
check font_matches 'JetBrainsMono Nerd Font' 'JetBrainsMono Nerd Font'
check font_matches 'Roboto Flex' 'Roboto Flex'
check font_matches 'Gabarito' 'Gabarito'
check font_matches 'Oxanium' 'Oxanium'
check font_matches 'Noto Color Emoji' 'Noto Color Emoji'

if [[ "${INIR_VERIFY_IDEMPOTENCY:-false}" == true ]]; then
  before="$(mktemp)"
  after="$(mktemp)"
  provider_snapshot() {
    xbps-query -p pkgver nerd-fonts-ttf
    sha256sum \
      "$font_dir/MaterialSymbolsRounded.ttf" \
      "$font_dir/RobotoFlex.ttf" \
      "$font_dir/Gabarito.ttf" \
      "$font_dir/Oxanium.ttf"
    fc-match -f '%{family}\n' 'JetBrainsMono Nerd Font'
  }
  provider_snapshot > "$before"
  check env ONLY_MISSING_DEPS='font-jetbrains-mono-nerd font-providers' \
    "$repo_root/setup" install -y --skip-sysupdate --skip-setups --skip-files
  provider_snapshot > "$after"
  check cmp -s "$before" "$after"
  rm -f "$before" "$after"
else
  printf 'INFO: set INIR_VERIFY_IDEMPOTENCY=true to run the second-install snapshot check\n'
fi

if ((failures > 0)); then
  printf '%d Void font-provider check(s) failed\n' "$failures" >&2
  exit 1
fi
printf 'All Void font-provider checks passed\n'
