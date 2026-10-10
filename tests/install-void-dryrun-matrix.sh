#!/usr/bin/env bash
set -euo pipefail

repo=${RYOKU_PATH:-$(cd "$(dirname "$0")/.." && pwd)}
backend=$repo/void/iso/backend/ryoku-install
builder=$repo/void/iso/build.sh

fail() {
  printf 'void installer dry-run matrix: %s\n' "$*" >&2
  exit 1
}

mapfile -t sourced_shared < <(
  sed -n 's|^[[:space:]]*source "\$SHARED_LIB_DIR/\([A-Za-z0-9_-]*\)\.sh"$|\1|p' \
    "$backend"
)
mapfile -t staged_shared < <(
  awk '
    /^[[:space:]]*for library in / {
      for (i = 4; i <= NF; i++) {
        gsub(/;/, "", $i)
        if ($i == "do") exit
        print $i
      }
      exit
    }
  ' "$builder"
)
((${#sourced_shared[@]})) || fail 'no shared backend imports found'
((${#staged_shared[@]})) || fail 'no staged shared backend libraries found'

declare -A staged_set=()
for library in "${staged_shared[@]}"; do
  staged_set[$library]=1
  [[ -f $repo/installation/backend/lib/$library.sh ]] \
    || fail "staged shared library has no source: $library"
done
for library in "${sourced_shared[@]}"; do
  [[ -n ${staged_set[$library]:-} ]] \
    || fail "shared backend library is not staged: $library"
done
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
run_case() {
  local name=$1 strategy=$2 profile=$3 encrypt=$4 firmware=$5
  local esp_mode=${6:-shared} online=${7:-0} snapshots=${8:-1}
  local out=$tmp/$name.out
  local -a envs=(
    RYOKU_DRYRUN=1
    RYOKU_DISK=/dev/vda
    RYOKU_PASSWORD_HASH=x
    RYOKU_DISK_STRATEGY="$strategy"
    RYOKU_PROFILE="$profile"
    RYOKU_REPO="$repo"
    RYOKU_COMPOSITOR=niri
    RYOKU_COMPOSITOR_CONFIG_DIR=niri
    RYOKU_BROWSER=firefox
    RYOKU_LOGIN_SHELL=fish
    RYOKU_KEYMAP=de
    RYOKU_ONLINE="$online"
    RYOKU_XKB_LAYOUT=de
    RYOKU_FIRMWARE_MODE="$firmware"
    RYOKU_ESP_MODE="$esp_mode"
    RYOKU_SUBVOL_SNAPSHOTS="$snapshots"
  )
  if [[ $encrypt == yes ]]; then
    envs+=(RYOKU_ENCRYPT=1 RYOKU_LUKS_PASSPHRASE=secret)
  fi
  env "${envs[@]}" "$backend" >"$out"

  mapfile -t stages < <(grep '^@@RYOKU_STEP ' "$out" | awk '{print $2}')
  [[ ${stages[*]} == 'partition filesystems mount pacstrap configure bootloader' ]] \
    || { printf '%s: bad stages: %s\n' "$name" "${stages[*]}" >&2; exit 1; }
  [[ $(grep -c '^@@RYOKU_DONE$' "$out") == 1 ]]
  grep -q 'xbps-install -r /mnt -R /run/initramfs/live/ryoku/repo -i -Sy' "$out"
  grep -q 'ryoku-desktop-niri' "$out"
  grep -q 'ryoku-host session ensure --system' "$out"
  grep -q 'ryoku-host session ensure --user' "$out"
  grep -q 'desktop.input.kbLayout=de' "$out"
  grep -q '/.local/state/ryoku/provisioned' "$out"
  grep -q '/.local/state/ryoku/default-rice-pending' "$out"
  grep -q '/Pictures/Wallpapers' "$out"
  grep -q 'ryoku materialize' "$out"
  ! grep -q 'XBPS_REPOSITORY=' "$out"
  ! grep -q 'Snapshots are not available on Void Linux' "$out"
  if [[ $snapshots == 1 ]]; then
    grep -q 'btrfs subvolume create /mnt/@snapshots' "$out"
    grep -q 'mount -o compress=zstd:1,noatime,subvol=@snapshots .* /mnt/.snapshots' "$out"
    grep -q $'/.snapshots\tbtrfs\tcompress=zstd:1,noatime,subvol=@snapshots' "$out"
    grep -q 'write /mnt/etc/snapper/configs/root' "$out"
    grep -q 'write /mnt/etc/conf.d/snapper' "$out"
    grep -q 'NUMBER_LIMIT="10"' "$out"
    grep -q 'SNAPPER_CONFIGS="root"' "$out"
    grep -q 'write /mnt/etc/default/limine' "$out"
    grep -q 'TARGET_OS_NAME="Ryoku Linux"' "$out"
    grep -q 'ESP_PATH="/boot"' "$out"
    grep -q 'MAX_SNAPSHOT_ENTRIES=10' "$out"
    grep -q 'SNAPSHOT_FORMAT_CHOICE=5' "$out"
    grep -q 'ln -sfn /etc/sv/snapper-cleanup /mnt/etc/runit/runsvdir/default/snapper-cleanup' "$out"
    grep -q 'ln -sfn /etc/sv/limine-snapper-sync /mnt/etc/runit/runsvdir/default/limine-snapper-sync' "$out"
    grep -qE 'xbps-install .* snapper( |$)' "$out"
    grep -qE 'xbps-install .* limine-snapper-sync( |$)' "$out"
    local snap_config_line limine_hook_line
    snap_config_line=$(grep -n -m1 'write /mnt/etc/snapper/configs/root' "$out" | cut -d: -f1)
    limine_hook_line=$(grep -n -m1 '/etc/kernel.d/post-install/50-ryoku-limine' "$out" | cut -d: -f1)
    (( snap_config_line < limine_hook_line )) \
      || { printf '%s: snapshot config was written after the Limine hook\n' "$name" >&2; exit 1; }
    ! grep -q '/mnt/etc/ryoku/snapshots-disabled' "$out"
  else
    ! grep -q 'subvol=@snapshots' "$out"
    grep -q 'write /mnt/etc/ryoku/snapshots-disabled' "$out"
    grep -q 'Snapshots were declined at install (RYOKU_SUBVOL_SNAPSHOTS=0)' "$out"
    ! grep -q 'write /mnt/etc/snapper/configs/root' "$out"
    ! grep -q 'write /mnt/etc/default/limine' "$out"
  fi

  local package_line keymap_line materialize_line ledger_line marker_line
  local qylock_line user_services_line xdg_dirs_line recordings_line rashin_wire_line
  local -a chown_lines=()
  package_line=$(grep -n -m1 'installing Ryoku desktop' "$out" | cut -d: -f1)
  keymap_line=$(grep -n -m1 'desktop.input.kbLayout=de' "$out" | cut -d: -f1)
  mapfile -t chown_lines < <(grep -n 'fixing ownership of /home/' "$out" | cut -d: -f1)
  materialize_line=$(grep -n -m1 'ryoku materialize' "$out" | cut -d: -f1)
  ledger_line=$(grep -n -m1 '/.local/state/ryoku/provisioned' "$out" | cut -d: -f1)
  marker_line=$(grep -n -m1 '/.local/state/ryoku/default-rice-pending' "$out" | cut -d: -f1)
  qylock_line=$(grep -n -m1 'deploying qylock bundle' "$out" | cut -d: -f1)
  user_services_line=$(grep -n -m1 'ryoku-host session ensure --user' "$out" | cut -d: -f1)
  xdg_dirs_line=$(grep -n -m1 'xdg-user-dirs-update' "$out" | cut -d: -f1)
  recordings_line=$(grep -n -m1 'config=.*recording.json' "$out" | cut -d: -f1)
  rashin_wire_line=$(grep -n -m1 'exec ryoku-rashin wire' "$out" | cut -d: -f1)
  [[ ${#chown_lines[@]} == 2 ]] \
    || { printf '%s: expected two ownership passes\n' "$name" >&2; exit 1; }
  (( package_line < keymap_line &&
     keymap_line < chown_lines[0] &&
     chown_lines[0] < materialize_line &&
     materialize_line < ledger_line &&
     ledger_line < marker_line &&
     marker_line < chown_lines[1] &&
     chown_lines[1] < xdg_dirs_line &&
     xdg_dirs_line < recordings_line &&
     recordings_line < rashin_wire_line &&
     rashin_wire_line < qylock_line &&
     qylock_line < user_services_line )) \
    || { printf '%s: desktop provisioning order regressed\n' "$name" >&2; exit 1; }
  if [[ $online == 0 ]]; then
    local mask_line drivers_line unmask_line final_repo_line
    mask_line=$(grep -n -m1 \
      'install -Dm644 /dev/null /mnt/etc/xbps.d/00-repository-main.conf' "$out" \
      | cut -d: -f1)
    drivers_line=$(grep -n -m1 'installing drivers for the detected hardware' "$out" \
      | cut -d: -f1)
    unmask_line=$(grep -n -m1 \
      'rm -f /mnt/etc/xbps.d/00-repository-main.conf' "$out" \
      | cut -d: -f1)
    final_repo_line=$(grep -n -m1 \
      'write /mnt/etc/xbps.d/20-ryoku.conf' "$out" \
      | cut -d: -f1)
    (( mask_line < drivers_line &&
       drivers_line < unmask_line &&
       unmask_line < final_repo_line )) \
      || { printf '%s: offline XBPS mask order regressed\n' "$name" >&2; exit 1; }
  else
    ! grep -q 'masking target XBPS remotes' "$out"
    ! grep -q 'install -Dm644 /dev/null /mnt/etc/xbps.d/' "$out"
  fi
  if [[ $encrypt == yes ]]; then
    grep -q 'cryptsetup luksFormat' "$out"
    grep -q 'rd.luks.uuid=' "$out"
  fi
  if [[ $firmware == bios ]]; then
    grep -q 'bios_grub on' "$out"
    grep -q 'limine bios-install /dev/vda 2' "$out"
  else
    grep -q 'EFI/BOOT/BOOTX64.EFI' "$out"
  fi
}

run_case whole-vm whole vm no uefi
run_case whole-amd-crypt whole amd yes uefi
run_case whole-intel-bios whole intel no bios
run_case whole-online whole vm no uefi shared 1
run_case whole-no-snapshots whole vm no uefi shared 0 0
run_case alongside-shared alongside vm no uefi shared
run_case alongside-dedicated alongside amd-nvidia yes uefi dedicated

mkdir -p "$tmp/bin"
cat >"$tmp/bin/xbps-query" <<'EOF'
#!/usr/bin/env bash
cat <<'PACKAGES'
ii base-system-0.114_1
ii adwaita-icon-theme-47.0_1
ii qt6-5compat-6.9.2_1
uu stale-package-1.0_1
PACKAGES
EOF
chmod +x "$tmp/bin/xbps-query"
# shellcheck source=void/iso/backend/lib/base.sh
source "$repo/void/iso/backend/lib/base.sh"
packages=(base-system adwaita-icon-theme qt6-5compat ryoku-desktop)
PATH="$tmp/bin:$PATH" void_keep_uninstalled_packages packages
[[ ${packages[*]} == ryoku-desktop ]] \
  || { printf 'installed package filter kept: %s\n' "${packages[*]}" >&2; exit 1; }

printf 'void installer dry-run matrix: ok\n'
