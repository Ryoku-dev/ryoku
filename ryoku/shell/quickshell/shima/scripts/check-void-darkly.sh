#!/usr/bin/env bash
# Void Darkly checks for the Void Darkly Qt6 provider.
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
expected_branch="${INIR_EXPECTED_BRANCH:-feat/void-pr54-darkly}"
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
deps_map="$repo_root/sdata/lib/deps-map.sh"
for package in \
    cmake extra-cmake-modules qt6-base-devel qt6-declarative-devel \
    kf6-kcoreaddons-devel kf6-kcmutils-devel kf6-kcolorscheme-devel \
    kf6-kconfig-devel kf6-kguiaddons-devel kf6-ki18n-devel \
    kf6-kiconthemes-devel kf6-kwindowsystem-devel kf6-kirigami-devel \
    kf6-frameworkintegration-devel kf6-kdecoration-devel; do
  check xbps-query -p pkgver "$package"
done
for needle in \
  'DARKLY_VERSION="0.5.39"' \
  'DARKLY_SOURCE_SHA256="5fed786f78ac3a6153e99920e722c981348c01fc781fb511371f6bfedee0f0c2"' \
  'install_void_darkly' \
  'void_darkly_kcm_path' \
  '-DBUILD_QT5=OFF' \
  '-DBUILD_QT6=ON' \
  '-DWITH_DECORATIONS=ON'; do
  if grep -Fq -- "$needle" "$deps"; then
    printf 'PASS: provider contains: %s\n' "$needle"
  else
    printf 'FAIL: provider missing: %s\n' "$needle" >&2
    failures=$((failures + 1))
  fi
done
check grep -Fq 'void:COMPILE:https://github.com/Bali10050/Darkly' "$deps_map"

qt_plugin_dirs=()
if command -v qtpaths6 >/dev/null 2>&1; then
  qt_plugin_dirs+=("$(qtpaths6 --plugin-dir 2>/dev/null || true)")
elif command -v qtpaths >/dev/null 2>&1; then
  qt_plugin_dirs+=("$(qtpaths --plugin-dir 2>/dev/null || true)")
fi
qt_plugin_dirs+=(/usr/lib64/qt6/plugins /usr/lib/qt6/plugins /usr/lib/x86_64-linux-gnu/qt6/plugins)
plugin=""
kcm=""
platform_theme=""
for plugin_dir in "${qt_plugin_dirs[@]}"; do
  [[ -n "$plugin_dir" && -d "$plugin_dir" ]] || continue
  if [[ -z "$plugin" ]]; then
    plugin="$(find "$plugin_dir/styles" -maxdepth 1 -type f -iname '*darkly*.so' -print -quit 2>/dev/null || true)"
  fi
  if [[ -z "$kcm" ]]; then
    kcm="$(find "$plugin_dir/org.kde.kdecoration3.kcm" -maxdepth 1 -type f -name 'kcm_darklydecoration.so' -print -quit 2>/dev/null || true)"
  fi
  if [[ -z "$platform_theme" && -f "$plugin_dir/platformthemes/KDEPlasmaPlatformTheme6.so" ]]; then
    platform_theme="$plugin_dir/platformthemes/KDEPlasmaPlatformTheme6.so"
  fi
done
check test -n "$plugin"
if [[ -n "$plugin" ]]; then
  check test -z "$(ldd "$plugin" 2>/dev/null | grep 'not found' || true)"
  check test -s "$plugin"
fi
check test -n "$kcm"
if [[ -n "$kcm" ]]; then
  check test -z "$(ldd "$kcm" 2>/dev/null | grep 'not found' || true)"
  check test -s "$kcm"
fi
marker="${XDG_DATA_HOME:-$HOME/.local/share}/inir/providers/darkly"
check test "$(cat "$marker" 2>/dev/null || true)" = '0.5.39:5fed786f78ac3a6153e99920e722c981348c01fc781fb511371f6bfedee0f0c2'

qt_output="$(timeout 3 env QT_QPA_PLATFORM=offscreen QT_STYLE_OVERRIDE=Darkly qt6ct 2>&1 || true)"
if grep -Fq "invalid style override 'Darkly'" <<<"$qt_output"; then
  printf 'FAIL: Qt6 rejected QT_STYLE_OVERRIDE=Darkly\n' >&2
  failures=$((failures + 1))
else
  printf 'PASS: Qt6 accepts QT_STYLE_OVERRIDE=Darkly\n'
fi
check test -n "$platform_theme"

if [[ "${INIR_VERIFY_IDEMPOTENCY:-false}" == true && -n "$plugin" ]]; then
  before="$(mktemp)"
  after="$(mktemp)"
  provider_snapshot() {
    sha256sum "$plugin" "$kcm" "$marker"
  }
  provider_snapshot > "$before"
  check env ONLY_MISSING_DEPS=darkly \
    "$repo_root/setup" install -y --skip-sysupdate --skip-setups --skip-files
  provider_snapshot > "$after"
  check cmp -s "$before" "$after"
  rm -f "$before" "$after"
else
  printf 'INFO: set INIR_VERIFY_IDEMPOTENCY=true after the Darkly plugin exists\n'
fi

if ((failures > 0)); then
  printf '%d Void Darkly check(s) failed\n' "$failures" >&2
  exit 1
fi
printf 'All Void Darkly checks passed\n'
