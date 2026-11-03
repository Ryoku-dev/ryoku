#!/usr/bin/env bash
# Void closure final closure checks for Void doctor/versioning/predicate behavior.
set -u

failures=0
temp_root=""
cleanup() {
  [[ -z "$temp_root" ]] || rm -rf "$temp_root"
}
trap cleanup EXIT

check() {
  if "$@"; then
    printf 'PASS: %s\n' "$*"
  else
    printf 'FAIL: %s\n' "$*" >&2
    failures=$((failures + 1))
  fi
}

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
expected_branch="${INIR_EXPECTED_BRANCH:-prerelease}"
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

launcher="$repo_root/scripts/inir"
doctor="$repo_root/sdata/lib/doctor.sh"
versioning="$repo_root/sdata/lib/versioning.sh"
dolphin_migration="$repo_root/sdata/migrations/005-dolphin-xdg-menu.sh"
qt_migration="$repo_root/sdata/migrations/012-plasma-integration-qt-theming.sh"
browser_migration="$repo_root/sdata/migrations/029-plasma-browser-integration.sh"
orbit_audit="$repo_root/scripts/orbit-visual-audit.sh"
void_deps="$repo_root/sdata/dist-void/install-deps.sh"
deps_map="$repo_root/sdata/lib/deps-map.sh"
uninstall_lib="$repo_root/sdata/lib/uninstall.sh"
package_search="$repo_root/services/deferred/PackageSearch.qml"
tools_view="$repo_root/modules/sidebarLeft/ToolsView.qml"
waffle_updates="$repo_root/modules/waffle/bar/UpdatesButton.qml"
software_view="$repo_root/modules/sidebarLeft/SoftwareView.qml"
default_config="$repo_root/defaults/config.json"
config_qml="$repo_root/modules/common/Config.qml"
switchwall="$repo_root/scripts/colors/switchwall.sh"
sddm_installer="$repo_root/scripts/sddm/install-pixel-sddm.sh"
conflicts_lib="$repo_root/sdata/lib/conflicts.sh"
package_installers="$repo_root/sdata/lib/package-installers.sh"
setup_cli="$repo_root/setup"

array_has_package() {
  local array_name="$1" package="$2"
  sed -n "/^${array_name}=(/,/^)/p" "$void_deps" | grep -Eq "^[[:space:]]+${package}$"
}

for needle in \
  'void-xbps:quickshell' \
  'void-xbps)' \
  'sudo xbps-install -Sf'; do
  check grep -Fq "$needle" "$launcher"
done
# Doctor hands the Quickshell rebuild to the launcher, which owns it for every distro.
check grep -Fq 'doctor --fix-abi && _doctor_abi_detect' "$doctor"
for needle in \
  'xbps) echo "sudo xbps-install -Su"' \
  'elif command -v xbps-install &>/dev/null' \
  'echo "xbps"'; do
  check grep -Fq "$needle" "$versioning"
done
check grep -Fq 'has_usable_systemd_user_manager' "$dolphin_migration"
check grep -Fq 'systemd/private' "$orbit_audit"
check grep -Fq 'timeout 3s systemctl --user show-environment' "$orbit_audit"
for package in curl wget git ripgrep bc xdg-utils xdg-user-dirs libnotify xwayland-satellite xdg-desktop-portal-gnome gnome-keyring libsecret nautilus kitty kf6-kirigami kdialog breeze-icons qt6ct power-profiles-daemon qt6-webengine layer-shell-qt sddm xorg-minimal; do
  check array_has_package VOID_BASE_PACKAGES "$package"
done
check grep -Fq 'command -v qml6 || command -v qml' "$repo_root/services/WebWallpaper.qml"
check grep -Fq '/usr/lib/qt6/bin/qml' "$repo_root/services/WebWallpaper.qml"
if grep -Fq 'import Quickshell' "$repo_root/modules/background/WebWallpaperHost.qml" \
    || grep -Fq 'Quickshell.env(' "$repo_root/modules/background/WebWallpaperHost.qml"; then
  printf 'FAIL: Web Wallpaper host still depends on the Quickshell runner\n' >&2
  failures=$((failures + 1))
else
  printf 'PASS: Web Wallpaper host is pure Qt QML\n'
fi
check grep -Fq 'args.indexOf("--probe")' "$repo_root/modules/background/WebWallpaperHost.qml"
for package in plasma-browser-integration lsp-plugins-lv2 libdbusmenu-gtk3 alsa-pipewire; do
  check array_has_package VOID_AUDIO_PACKAGES "$package"
done
check array_has_package VOID_TOOLKIT_PACKAGES ImageMagick
check array_has_package VOID_FONTS_PACKAGES kde-cli-tools
for package in tesseract-ocr tesseract-ocr-eng tesseract-ocr-spa tesseract-ocr-rus tesseract-ocr-jpn tesseract-ocr-chi_sim tesseract-ocr-chi_tra; do
  check array_has_package VOID_OCR_PACKAGES "$package"
done
check grep -Fq 'INSTALL_TOOLKIT:-true} || ${INSTALL_SCREENCAPTURE:-true}' "$void_deps"
if grep -Fq 'python3-ytmusicapi' "$void_deps"; then
  printf 'FAIL: Void still installs stale distro ytmusicapi alongside the managed runtime\n' >&2
  failures=$((failures + 1))
else
  printf 'PASS: Void YT Music uses only the managed Python runtime\n'
fi
for needle in \
  '[awww-daemon]="awww"' \
  '[flock]="util-linux"' \
  '[kwriteconfig6]="kf6-kconfig"' \
  '[trans]="translate-shell"' \
  '[qt-webengine]="qt6-webengine"' \
  '[layer-shell-qt]="layer-shell-qt"' \
  '[notify-send]="libnotify"' \
  '[xdg-settings]="xdg-utils"' \
  '[secret-tool]="libsecret"' \
  '[gnome-keyring-daemon]="gnome-keyring"' \
  '[powerprofilesctl]="power-profiles-daemon"'; do
  check grep -Fq "$needle" "$void_deps"
done
check grep -Fq 'missing_cmds+=("qt-webengine")' "$doctor"
check grep -Fq 'missing_cmds+=("layer-shell-qt")' "$doctor"
check grep -Fq 'service/inir-super-overview' "$repo_root/sdata/subcmd-install/2.setups.sh"
check grep -Fq 'exec chpst -e "$TURNSTILE_ENV_DIR"' "$repo_root/sdata/subcmd-install/2.setups.sh"
check grep -Fq 'service/inir"]="iNiR runit user service"' "$uninstall_lib"
check grep -Fq 'service/inir-super-overview"]="iNiR Super-tap runit service"' "$uninstall_lib"
check grep -Fq 'sv down "$user_service_root/$service_dir"' "$uninstall_lib"
check grep -Fq 'command -v xbps-query' "$conflicts_lib"
check grep -Fq 'xbps-query -p pkgver "$pkg"' "$conflicts_lib"
check grep -Fq 'pkg_sudo xbps-remove -R -- "$pkg"' "$conflicts_lib"
check grep -Fq 'xbps-query -p pkgver "$pkg"' "$doctor"
if grep -Fq 'Conflicting shells (not Arch, skipped)' "$doctor"; then
  printf 'FAIL: Doctor still skips conflicting shell packages on Void\n' >&2
  failures=$((failures + 1))
else
  printf 'PASS: Doctor inspects conflicting shell packages on Void\n'
fi
check grep -Fq '"powerprofilesctl:power-profiles-daemon"' "$doctor"
check grep -Fq 'ln -sfn /etc/sv/power-profiles-daemon /var/service/power-profiles-daemon' "$repo_root/sdata/subcmd-install/2.setups.sh"
check grep -Fq 'void:nerd-fonts-ttf' "$deps_map"
if grep -Fq 'void:font-jetbrains-mono-nerd' "$deps_map"; then
  printf 'FAIL: stale nonexistent Void Nerd Font package remains in deps map\n' >&2
  failures=$((failures + 1))
else
  printf 'PASS: Void Nerd Font mapping uses the validated provider\n'
fi
for migration in "$qt_migration" "$browser_migration"; do
  check grep -Fq 'command -v xbps-install' "$migration"
  check grep -Fq 'xbps-install -S -y' "$migration"
done
check grep -Fq 'xbps-query -X' "$uninstall_lib"
check grep -Fq 'sudo xbps-remove -R' "$uninstall_lib"
check grep -Fq 'function cleanPackageCache()' "$package_search"
check grep -Fq 'sudo xbps-remove -O' "$package_search"
check grep -Fq 'PackageSearch.updateSystem()' "$tools_view"
check grep -Fq 'PackageSearch.cleanPackageCache()' "$tools_view"
if grep -Fq 'yay -Syu' "$tools_view" || grep -Fq 'paccache -rk1' "$tools_view"; then
  printf 'FAIL: Tools view still contains direct Arch package actions\n' >&2
  failures=$((failures + 1))
else
  printf 'PASS: Tools view delegates package actions to PackageSearch\n'
fi
check grep -Fq 'PackageSearch.runConfiguredUpdate()' "$waffle_updates"
check grep -Fq 'root.updateSystem()' "$repo_root/services/deferred/PackageSearch.qml"
if grep -Fq '"update": "kitty -e arch-update"' "$default_config" \
    || grep -Fq 'property string update: "kitty -e sudo pacman -Syu"' "$config_qml"; then
  printf 'FAIL: fresh config still persists an Arch-only update command\n' >&2
  failures=$((failures + 1))
else
  printf 'PASS: fresh update command delegates to the detected backend\n'
fi
check grep -Fq 'Translation.tr("Install pacman, apt, or dnf") + " / xbps"' "$software_view"
check grep -Fq 'sudo xbps-install -S ffmpeg' "$switchwall"
check grep -Fq 'sudo xbps-install -S sddm xorg-minimal qt6-declarative qt6-qt5compat' "$sddm_installer"
check grep -Fq 'sudo ln -s /etc/sv/sddm /var/service/' "$sddm_installer"
check grep -Fq 'configure_void_sddm_service' "$repo_root/sdata/lib/functions.sh"
check grep -Fq 'configure_void_sddm_service' "$setup_cli"
check grep -Fq 'Competing display manager detected' "$repo_root/sdata/lib/functions.sh"
check grep -Fq 'Niri display-manager session entry is missing or invalid' "$repo_root/sdata/lib/functions.sh"
check grep -Fq 'INIR_DBUS_SYSTEM_SOCKET' "$repo_root/sdata/lib/functions.sh"
check grep -Fq 'xbps-query -p pkgver quickshell' "$setup_cli"
check grep -Fq 'xbps-query -p repository quickshell' "$setup_cli"
check grep -Fq 'xbps-install -S sudo' "$package_installers"

check bash -n "$launcher" "$doctor" "$dolphin_migration" "$qt_migration" "$browser_migration" "$orbit_audit" "$uninstall_lib" "$switchwall" "$sddm_installer" "$conflicts_lib" "$package_installers" "$setup_cli"

predicate_fixture() (
  set -e
  local root
  root="$(mktemp -d)"
  trap 'rm -rf "$root"' EXIT
  mkdir -p "$root/config/niri" "$root/home"
  cat > "$root/config/niri/config.kdl" <<'KDL'
environment {
    XDG_CURRENT_DESKTOP "niri"
}
spawn-at-startup "true"
spawn-at-startup "bash" "-c" "systemctl --user import-environment XDG_MENU_PREFIX && kbuildsycoca6"
KDL
  HOME="$root/home" REPO_ROOT="$repo_root" XDG_CONFIG_HOME="$root/config" \
    XDG_RUNTIME_DIR="$root/runtime" bash -c '
      source "$REPO_ROOT/sdata/migrations/005-dolphin-xdg-menu.sh"
      migration_apply
      grep -Fq '\''XDG_MENU_PREFIX "plasma-"'\'' "$XDG_CONFIG_HOME/niri/config.kdl"
      ! grep -Fq '\''systemctl --user import-environment'\'' "$XDG_CONFIG_HOME/niri/config.kdl"
    '
)
check predicate_fixture

versioning_fixture() (
  set -e
  local root
  root="$(mktemp -d)"
  trap 'rm -rf "$root"' EXIT
  export HOME="$root/home"
  export XDG_CONFIG_HOME="$root/home/.config"
  export XDG_CONFIG_HOME_RESOLVED="$root/home/.config"
  export XDG_CACHE_HOME="$root/home/.cache"
  export XDG_RUNTIME_DIR="$root/runtime-dir"
  export REPO_ROOT="$repo_root"
  export INIR_INSTALL_MODE=package-managed
  export INIR_UPDATE_STRATEGY=package-manager
  export INIR_PACKAGE_MANAGER=xbps
  export INIR_PACKAGE_NAME=inir
  mkdir -p "$XDG_CONFIG_HOME_RESOLVED/inir" "$XDG_RUNTIME_DIR" "$root/runtime"
  source "$repo_root/sdata/lib/versioning.sh"
  write_version_info_json "$VERSION_FILE_LOCAL" "1.2.3" "abc123" "package"
  [[ "$(get_installed_install_mode)" == package-managed ]]
  [[ "$(get_installed_update_strategy)" == package-manager ]]
  [[ "$(get_installed_package_manager)" == xbps ]]
  [[ "$(get_installed_package_update_hint)" == "sudo xbps-install -Su" ]]
  touch "$root/runtime/shell.qml"
  cp "$VERSION_FILE_LOCAL" "$root/runtime/version.json"
  INIR_RUNTIME_DIR="$root/runtime" "$repo_root/scripts/inir" version --json \
    | jq -e '.installMode == "package-managed" and .packageManager == "xbps"' >/dev/null
)
check versioning_fixture

if [[ "${INIR_STATIC_ONLY:-false}" == true ]]; then
  if ((failures > 0)); then
    printf '%d Void closure static check(s) failed\n' "$failures" >&2
    exit 1
  fi
  printf 'All Void closure static checks passed\n'
  exit 0
fi

check command -v xbps-query
check command -v xbps-install
check command -v xbps-uhelper
check command -v qs
check xbps-query -p pkgver quickshell
for package in curl wget git ripgrep bc xdg-utils xdg-user-dirs libnotify xwayland-satellite xdg-desktop-portal-gnome gnome-keyring libsecret nautilus kitty kf6-kirigami kdialog breeze-icons qt6ct power-profiles-daemon qt6-webengine layer-shell-qt sddm xorg-minimal plasma-browser-integration lsp-plugins-lv2 libdbusmenu-gtk3 alsa-pipewire ImageMagick kde-cli-tools tesseract-ocr tesseract-ocr-eng tesseract-ocr-spa tesseract-ocr-rus tesseract-ocr-jpn tesseract-ocr-chi_sim tesseract-ocr-chi_tra; do
  check xbps-query -R -p pkgver "$package"
done
for package in awww util-linux kf6-kconfig translate-shell; do
  check xbps-query -R -p pkgver "$package"
done
power_profiles_files="$(xbps-query -R -f power-profiles-daemon 2>/dev/null || true)"
check grep -Fq '/etc/sv/power-profiles-daemon/run' <<<"$power_profiles_files"
check grep -Fq '/usr/share/dbus-1/system-services/org.freedesktop.UPower.PowerProfiles.service' <<<"$power_profiles_files"
check grep -Fq '/usr/share/polkit-1/actions/power-profiles-daemon.policy' <<<"$power_profiles_files"
if xbps-query -p pkgver power-profiles-daemon >/dev/null 2>&1; then
  printf 'PASS: power-profiles-daemon is already installed\n'
else
  power_profiles_plan="$(xbps-install -n power-profiles-daemon 2>&1 || true)"
  check grep -Fq 'power-profiles-daemon-' <<<"$power_profiles_plan"
fi

quickshell_reinstall_dry_run() {
  xbps-install -Sfn quickshell 2>/dev/null | grep -Eq '^quickshell-[^[:space:]]+[[:space:]]+reinstall[[:space:]]'
}
check quickshell_reinstall_dry_run

abi_noop() {
  local output rc
  output="$("$launcher" doctor --fix-abi --noninteractive 2>&1)"
  rc=$?
  printf '%s\n' "$output"
  [[ $rc -eq 0 ]] && grep -Fq 'matches the installed Qt' <<<"$output"
}
check abi_noop

runtime_dir="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
if [[ -S "$runtime_dir/systemd/private" ]] \
    && command -v timeout >/dev/null 2>&1 \
    && timeout 3s systemctl --user show-environment >/dev/null 2>&1; then
  printf 'INFO: usable systemd user manager is active; non-systemd runtime assertion skipped\n'
else
  printf 'PASS: Void VM has no usable systemd user manager\n'
fi

if ((failures > 0)); then
  printf '%d Void closure check(s) failed\n' "$failures" >&2
  exit 1
fi
printf 'All Void closure checks passed\n'
