#!/usr/bin/env bash
# Void visual-provider checks for Void GTK/icon/cursor providers.
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
expected_branch="${INIR_EXPECTED_BRANCH:-feat/void-pr52-visual-providers}"
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
for package in curl unzip; do
  check xbps-query -p pkgver "$package"
done
for needle in \
  'ADW_GTK3_VERSION="6.5"' \
  'ADW_GTK3_SHA256="a81780fadfc432be0fc3d89c4ebb41aa28e4f032d42c36f9789c57dd10cfa41c"' \
  'WHITESUR_ICON_VERSION="2026-09-10"' \
  'WHITESUR_ICON_SHA256="406c9cd59705583f1754b0eaca96cc48bafda042b88ef143f16d8ae1820ecd95"' \
  'CAPITAINE_VERSION="r5"' \
  'CAPITAINE_SHA256="60114cf857902a9907780bdcfa995d600618cf14b37f90776565c9de7e5add6c"' \
  'install_void_visual_providers'; do
  if grep -Fq "$needle" "$deps"; then
    printf 'PASS: provider contains: %s\n' "$needle"
  else
    printf 'FAIL: provider missing: %s\n' "$needle" >&2
    failures=$((failures + 1))
  fi
done

data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
theme_dir="$data_home/themes"
icon_dir="$data_home/icons"
marker_dir="$data_home/inir/providers"

check test -d "$theme_dir/adw-gtk3"
check test -d "$theme_dir/adw-gtk3-dark"
check test -f "$theme_dir/adw-gtk3-dark/gtk-3.0/gtk.css"
check test -d "$icon_dir/WhiteSur-dark"
check test -f "$icon_dir/WhiteSur-dark/index.theme"
check test -d "$icon_dir/Capitaine Cursors"
check test -d "$icon_dir/Capitaine Cursors - White"
check test -L "$icon_dir/capitaine-cursors"
check test -L "$icon_dir/capitaine-cursors-light"
check test "$(readlink "$icon_dir/capitaine-cursors")" = 'Capitaine Cursors'
check test "$(readlink "$icon_dir/capitaine-cursors-light")" = 'Capitaine Cursors - White'
check test "$(cat "$marker_dir/adw-gtk3")" = '6.5:a81780fadfc432be0fc3d89c4ebb41aa28e4f032d42c36f9789c57dd10cfa41c'
check test "$(cat "$marker_dir/whitesur-icons")" = '2026-09-10:406c9cd59705583f1754b0eaca96cc48bafda042b88ef143f16d8ae1820ecd95'
check test "$(cat "$marker_dir/capitaine-cursors")" = 'r5:60114cf857902a9907780bdcfa995d600618cf14b37f90776565c9de7e5add6c'

if [[ "${INIR_VERIFY_IDEMPOTENCY:-false}" == true ]]; then
  before="$(mktemp)"
  after="$(mktemp)"
  provider_snapshot() {
    sha256sum \
      "$marker_dir/adw-gtk3" \
      "$marker_dir/whitesur-icons" \
      "$marker_dir/capitaine-cursors" \
      "$theme_dir/adw-gtk3-dark/gtk-3.0/gtk.css" \
      "$icon_dir/WhiteSur-dark/index.theme" \
      "$icon_dir/Capitaine Cursors - White/index.theme"
    readlink "$icon_dir/capitaine-cursors"
    readlink "$icon_dir/capitaine-cursors-light"
  }
  provider_snapshot > "$before"
  check env ONLY_MISSING_DEPS='adw-gtk3 whitesur-icon-theme capitaine-cursors' \
    "$repo_root/setup" install -y --skip-sysupdate --skip-setups --skip-files
  provider_snapshot > "$after"
  check cmp -s "$before" "$after"
  rm -f "$before" "$after"
else
  printf 'INFO: set INIR_VERIFY_IDEMPOTENCY=true to run the second-install snapshot check\n'
fi

if ((failures > 0)); then
  printf '%d Void visual-provider check(s) failed\n' "$failures" >&2
  exit 1
fi
printf 'All Void visual-provider checks passed\n'
