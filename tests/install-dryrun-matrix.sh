#!/usr/bin/env bash
# End-to-end dry-run coverage for strategy, encryption, swap, ESP mode, and ISO
# variant. Every successful run must emit the canonical stage sentinels and end
# with exactly one @@RYOKU_DONE.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=tests/install-dryrun-harness.sh
source "$here/install-dryrun-harness.sh"
root=$INSTALL_DRYRUN_ROOT
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
fail() { echo "FAIL: $1" >&2; exit 1; }

# The installer refuses to run without a compositor choice and the config dir
# the TUI derives from the seam for it; the matrix asserts the Hyprland tree's
# paths, so that is the one it installs.
export RYOKU_COMPOSITOR="${RYOKU_COMPOSITOR:-hyprland}"
export RYOKU_COMPOSITOR_CONFIG_DIR="${RYOKU_COMPOSITOR_CONFIG_DIR:-hypr}"
export RYOKU_COMPOSITOR_GPU_PIN="${RYOKU_COMPOSITOR_GPU_PIN:-gpu.lua}"

canonical="partition filesystems mount pacstrap configure bootloader"

# run_backend <strategy> <encrypt> <swap> [esp-mode] [variant]
run_backend() {
  local output=$tmp/arch.out
  rc=0
  install_dryrun_run arch "$output" \
    RYOKU_DISK_STRATEGY="$1" RYOKU_ENCRYPT="$2" RYOKU_SWAP_GIB="$3" \
    RYOKU_ESP_MODE="${4:-auto}" RYOKU_VARIANT="${5:-plain}" \
    RYOKU_LUKS_PASSPHRASE=passphrase || rc=$?
  out=$(<"$output")
}

for strategy in whole alongside; do
  for encrypt in 0 1; do
    for swap in 0 8; do
      mode=auto
      [[ $strategy == alongside ]] && mode=shared
      tag="strategy=$strategy encrypt=$encrypt swap=$swap mode=$mode"
      run_backend "$strategy" "$encrypt" "$swap" "$mode"
      [[ $rc -eq 0 ]] || fail "$tag: dry run exited $rc: $out"

      # exactly six @@RYOKU_STEP sentinels, in the canonical order.
      steps="$(grep -oE '@@RYOKU_STEP [a-z]+' <<<"$out" | awk '{print $2}' | tr '\n' ' ')"
      [[ ${steps% } == "$canonical" ]] || fail "$tag: step order is '${steps% }', expected '$canonical'"

      # exactly one @@RYOKU_DONE, and it is the LAST sentinel of any kind.
      done_n="$(grep -cF '@@RYOKU_DONE' <<<"$out")"
      [[ $done_n -eq 1 ]] || fail "$tag: expected exactly 1 @@RYOKU_DONE, got $done_n"
      last="$(grep -E '@@RYOKU_STEP|@@RYOKU_DONE' <<<"$out" | tail -n1)"
      [[ $last == '@@RYOKU_DONE' ]] || fail "$tag: @@RYOKU_DONE is not the last sentinel (last='$last')"

      # secure-boot gate is narrated in every run (the preflight dry-run note).
      grep -qiF 'secure boot' <<<"$out" || fail "$tag: preflight did not narrate the Secure Boot gate"

      # per-strategy narration.
      if [[ $strategy == alongside ]]; then
        grep -qF 'XBOOTLDR' <<<"$out"        || fail "$tag: shared mode missing the XBOOTLDR /boot narration"
        grep -qF '/EFI/Microsoft' <<<"$out"  || fail "$tag: shared mode missing the existing-ESP safety promise"
        grep -qF 'ryokuboot' <<<"$out"       || fail "$tag: alongside missing the ryokuboot partlabel"
      else
        grep -qF '/EFI/Microsoft' <<<"$out" && fail "$tag: whole-disk wrongly narrated Windows' ESP (/EFI/Microsoft)"
      fi

      # per-encrypt narration: the pinned argon2id KDF only when encrypting.
      if [[ $encrypt == 1 ]]; then
        grep -qF 'luksFormat --type luks2 --pbkdf argon2id' <<<"$out" \
          || fail "$tag: encrypt missing the pinned argon2id luksFormat"
      else
        grep -qF 'luksFormat' <<<"$out" && fail "$tag: non-encrypt run emitted a luksFormat"
      fi

      # per-swap narration: the hibernation resume line only when swap > 0.
      if [[ $swap -gt 0 ]]; then
        grep -qF 'hibernation resume=' <<<"$out" || fail "$tag: swap>0 missing the hibernation resume narration"
      else
        grep -qF 'hibernation resume=' <<<"$out" && fail "$tag: swap=0 wrongly narrated a hibernation resume"
      fi
    done
  done
done

# Auto cannot be resolved without probing a real ESP, so an alongside dry-run
# must choose the path it wants narrated.
run_backend alongside 0 0 auto
[[ $rc -ne 0 ]] || fail "alongside auto dry-run silently selected an ESP mode"
grep -qF 'set RYOKU_ESP_MODE=shared or dedicated' <<<"$out" \
  || fail "alongside auto dry-run did not explain how to select a mode"

# Dedicated mode must stay entirely on the new ESP for both ISO variants.
for variant in plain cachyos; do
  run_backend alongside 0 0 dedicated "$variant"
  [[ $rc -eq 0 ]] || fail "$variant dedicated-ESP dry run exited $rc: $out"
  grep -qF 'mode=dedicated' <<<"$out" \
    || fail "$variant dedicated-ESP dry run lost the partition mode"
  grep -qF '/mnt/boot/EFI/limine/limine_x64.efi' <<<"$out" \
    || fail "$variant dedicated-ESP dry run did not install Limine on the new ESP"
  grep -qF '/mnt/efi/EFI/ryoku' <<<"$out" \
    && fail "$variant dedicated-ESP dry run touched the existing ESP path"
  grep -qF '@@RYOKU_DONE' <<<"$out" \
    || fail "$variant dedicated-ESP dry run did not complete"
done

# RYOKU_GPU_MODE wiring: the value is now CONSUMED. under dry-run the mapped
# `ryoku-gpu mode` call is narrated against the user's gpu.lua; sync->performance.
out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
  RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole RYOKU_GPU_MODE=sync \
  bash "$root/installation/backend/ryoku-install" 2>&1)" || fail "gpu-mode dry run exited nonzero: $out"
grep -qF 'ryoku-gpu mode performance' <<<"$out" || fail "RYOKU_GPU_MODE=sync did not narrate 'ryoku-gpu mode performance'"
grep -qF '/home/ryoku/.config/hypr/gpu.lua' <<<"$out" || fail "gpu-mode narration did not target the user's gpu.lua"

# absent by default: no ryoku-gpu mode call when RYOKU_GPU_MODE is unset.
out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
  RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole \
  bash "$root/installation/backend/ryoku-install" 2>&1)" || fail "no-gpu-mode dry run exited nonzero: $out"
grep -qF 'ryoku-gpu mode' <<<"$out" && fail "ryoku-gpu mode narrated when RYOKU_GPU_MODE was unset"


# Browser, login-shell, and app-choice wiring. Direct backend calls use the same
# Firefox/Fish defaults as the TUI, and every losing package reaches both the
# package filters and the provisioning ledger.
pcount() {
  grep -oE 'installing [0-9]+ packages' <<<"$1" | grep -oE '[0-9]+' | head -1
}
out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
  RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole \
  bash "$root/installation/backend/ryoku-install" 2>&1)" || fail "default product-choice dry run exited nonzero: $out"
grep -qF 'DRYRUN: set firefox as the default web browser' <<<"$out" \
  || fail "the empty browser choice did not default to firefox"
grep -qF 'useradd -M -U -d /home/ryoku -s /usr/bin/fish -G wheel,input,video ryoku' <<<"$out" \
  || fail "the empty login-shell choice did not create the user with fish"
seed_intent="$(grep -F 'DRYRUN: users seed-self account=ryoku ' <<<"$out" | tail -n1)"
[[ -n $seed_intent ]] || fail "the delegated account seed was not narrated"
[[ $seed_intent == *" rice=default "* ]] \
  || fail "the delegated account seed did not request the default rice: $seed_intent"
[[ $seed_intent == *" rashin=1 "* ]] \
  || fail "the delegated account seed did not request the first-login Rashin marker: $seed_intent"
grep -qF 'DRYRUN: set nautilus as the default file manager (inode/directory + desktop.apps.files)' <<<"$out" \
  || fail "the empty file-manager choice did not default to nautilus"
grep -qF 'ryoku-rashin ryotunes ryomanager nautilus nautilus-python' <<<"$out" \
  || fail "fresh-install app and file-manager packages were not in the desktop transaction"
base_n=$(pcount "$out")
[[ -n $base_n ]] || fail "no pacstrap package count in the default dry run"

out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
  RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole \
  RYOKU_BROWSER=chromium RYOKU_LOGIN_SHELL=zsh \
  bash "$root/installation/backend/ryoku-install" 2>&1)" || fail "chromium/zsh dry run exited nonzero: $out"
grep -qF 'DRYRUN: set chromium as the default web browser' <<<"$out" \
  || fail "RYOKU_BROWSER=chromium did not narrate chromium"
grep -qF 'useradd -M -U -d /home/ryoku -s /usr/bin/zsh -G wheel,input,video ryoku' <<<"$out" \
  || fail "RYOKU_LOGIN_SHELL=zsh did not reach useradd"
seed_intent="$(grep -F 'DRYRUN: users seed-self account=ryoku ' <<<"$out" | tail -n1)"
[[ -n $seed_intent ]] || fail "the drop list did not reach delegated account seeding"
ledger_pkgs="${seed_intent##* provisioned-ledger=}"
ledger_pkgs="${ledger_pkgs//,/ }"
for pkg in firefox zen-browser-bin fish blesh; do
  [[ " $ledger_pkgs " == *" $pkg "* ]] || fail "chromium/zsh ledger is missing $pkg: $seed_intent"
done
for pkg in chromium zsh zsh-autosuggestions zsh-history-substring-search zsh-syntax-highlighting ryoku-oh-my-zsh; do
  [[ " $ledger_pkgs " != *" $pkg "* ]] || fail "chromium/zsh ledger drops selected package $pkg: $seed_intent"
done

for shell in fish zsh bash; do
  out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
    RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole RYOKU_LOGIN_SHELL="$shell" \
    bash "$root/installation/backend/ryoku-install" 2>&1)" || fail "$shell dry run exited nonzero: $out"
  grep -qF "useradd -M -U -d /home/ryoku -s /usr/bin/$shell -G wheel,input,video ryoku" <<<"$out" \
    || fail "$shell did not reach useradd"
done

# Dropping two base-set apps removes exactly two packages from the transaction.
out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
  RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole RYOKU_DROP_PACKAGES=docker,flatpak \
  bash "$root/installation/backend/ryoku-install" 2>&1)" || fail "drop dry run exited nonzero: $out"
drop_n=$(pcount "$out")
[[ $drop_n -eq $((base_n - 2)) ]] \
  || fail "RYOKU_DROP_PACKAGES=docker,flatpak did not remove exactly 2 packages ($base_n -> $drop_n)"

# Unknown choices are refused before anything runs.
out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
  RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole RYOKU_BROWSER=brave \
  bash "$root/installation/backend/ryoku-install" 2>&1)" && fail "RYOKU_BROWSER=brave was accepted"
grep -qF 'not one of the shipped browsers' <<<"$out" \
  || fail "the browser gate did not explain the allowed values"
out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
  RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole RYOKU_LOGIN_SHELL=nu \
  bash "$root/installation/backend/ryoku-install" 2>&1)" && fail "RYOKU_LOGIN_SHELL=nu was accepted"
grep -qF 'not one of the shipped login shells' <<<"$out" \
  || fail "the login-shell gate did not explain the allowed values"

# File manager and optional Rashin decisions share the drop ledger with the
# existing browser and login-shell choices.
out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
  RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole \
  RYOKU_FILE_MANAGER=dolphin RYOKU_RASHIN=0 RYOKU_HIDE_APPS=ryotunes.desktop \
  bash "$root/installation/backend/ryoku-install" 2>&1)" \
  || fail "file-manager/app-choice dry run exited nonzero: $out"
grep -qF 'DRYRUN: set dolphin as the default file manager (inode/directory + desktop.apps.files)' <<<"$out" \
  || fail "RYOKU_FILE_MANAGER=dolphin did not reach the files role"
grep -qF 'ryomanager dolphin' <<<"$out" \
  || fail "dolphin and the default apps were not in the desktop transaction"
seed_intent="$(grep -F 'DRYRUN: users seed-self account=ryoku ' <<<"$out" | tail -n1)"
[[ $seed_intent == *" hidden-apps=ryotunes.desktop "* ]] \
  || fail "the delegated account seed missed the hidden launcher id: $seed_intent"
[[ $seed_intent == *" rashin=0 "* ]] \
  || fail "RYOKU_RASHIN=0 did not disable the delegated Rashin marker: $seed_intent"
ledger="${seed_intent##* provisioned-ledger=}"
ledger=${ledger//,/ }
for pkg in nautilus nautilus-python thunar ryoku-rashin; do
  [[ " $ledger " == *" $pkg "* ]] || fail "dolphin/Rashin-off ledger is missing $pkg: $seed_intent"
done
[[ " $ledger " != *" dolphin "* ]] || fail "dolphin/Rashin-off ledger drops selected dolphin: $seed_intent"

out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
  RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole RYOKU_FILE_MANAGER=pcmanfm \
  bash "$root/installation/backend/ryoku-install" 2>&1)" && fail "unknown file manager was accepted"
grep -qF 'not one of the shipped file managers' <<<"$out" \
  || fail "the file-manager gate did not explain the allowed values"
out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
  RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole RYOKU_HIDE_APPS=../bad.desktop \
  bash "$root/installation/backend/ryoku-install" 2>&1)" && fail "path-like hidden desktop id was accepted"
grep -qF 'use plain *.desktop file names' <<<"$out" \
  || fail "the hidden-app gate did not explain the required desktop id form"
for bad_env in RYOKU_RASHIN RYOKU_CACHYOS_LAYER; do
  out="$(env RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
    RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole "$bad_env"=yes \
    bash "$root/installation/backend/ryoku-install" 2>&1)" && fail "$bad_env=yes was accepted"
  grep -qF "$bad_env must be 0 or 1" <<<"$out" \
    || fail "$bad_env gate did not explain its allowed values"
done

# The optional gaming layer is plain-only and network-backed. Its package list
# comes from the installer component manifest, and a successful layer makes
# linux-cachyos the default while retaining stock linux as the flat fallback.
mkdir -p "$tmp/offline-repo"
: >"$tmp/offline-repo/offline.db"
out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
  RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole \
  RYOKU_CACHYOS_LAYER=1 RYOKU_ONLINE=0 RYOKU_OFFLINE_REPO="$tmp/offline-repo" \
  bash "$root/installation/backend/ryoku-install" 2>&1)" \
  || fail "offline-base CachyOS layer dry run exited nonzero: $out"
grep -qF 'check the target CPU for x86-64-v3 and the CachyOS mirror for reachability' <<<"$out" \
  || fail "offline-base CachyOS layer did not narrate its reachability check"
grep -qF 'run pacman -Syu for linux-cachyos' <<<"$out" \
  || fail "offline-base CachyOS layer did not narrate its full-upgrade transaction"
out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
  RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole \
  RYOKU_CACHYOS_LAYER=1 RYOKU_ONLINE=1 \
  bash "$root/installation/backend/ryoku-install" 2>&1)" \
  || fail "plain online CachyOS layer dry run exited nonzero: $out"
grep -qF 'enable [cachyos-v3], [cachyos], and multilib with the shipped helpers' <<<"$out" \
  || fail "plain CachyOS layer did not narrate repository setup"
grep -qF 'linux-cachyos linux-cachyos-headers cachyos-settings scx-scheds cachyos-gaming-meta steam' <<<"$out" \
  || fail "plain CachyOS layer did not read its gaming package set from components.tsv"
grep -qF 'add linux-cachyos-nvidia-open for Arch nvidia-open or verify the DKMS module with modinfo' <<<"$out" \
  || fail "plain CachyOS layer did not narrate NVIDIA kernel-module safety"
grep -qF 'feed every installed usr/lib/modules/<ver>/vmlinuz with a pkgbase to /usr/share/libalpm/scripts/mkinitcpio install' <<<"$out" \
  || fail "plain CachyOS layer did not narrate refreshing every installed kernel boot asset"
grep -qxF '        |     kernel_path: boot():/vmlinuz-linux-cachyos' <<<"$out" \
  || fail "plain CachyOS layer did not make linux-cachyos the default flat entry"
grep -qxF '        | /Arch Linux (fallback)' <<<"$out" \
  || fail "plain CachyOS layer did not retain Arch linux as the fallback entry"
grep -qxF '        |     kernel_path: boot():/vmlinuz-linux' <<<"$out" \
  || fail "plain CachyOS layer fallback does not point at stock linux"

# Pacman's install hook is masked during the layer transaction. Exercise the
# explicit replay against a fake target and chroot to prove both an upgraded
# stock kernel and the newly installed CachyOS kernel reach the packaged hook.
kernel_target="$tmp/kernel-target"
hook_input="$tmp/mkinitcpio-hook.stdin"
mkdir -p "$kernel_target/usr/lib/modules/6.12-arch" \
  "$kernel_target/usr/lib/modules/6.12-cachy" "$kernel_target/boot" \
  "$kernel_target/etc/mkinitcpio.d"
printf '%s\n' linux >"$kernel_target/usr/lib/modules/6.12-arch/pkgbase"
printf '%s\n' linux-cachyos >"$kernel_target/usr/lib/modules/6.12-cachy/pkgbase"
printf '%s\n' stock-image >"$kernel_target/usr/lib/modules/6.12-arch/vmlinuz"
printf '%s\n' cachyos-image >"$kernel_target/usr/lib/modules/6.12-cachy/vmlinuz"
cat >"$tmp/fake-kernel-chroot" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
target=$1
shift
[[ $* == '/usr/share/libalpm/scripts/mkinitcpio install' ]]
cat >"$RYOKU_HOOK_STDIN"
while IFS= read -r source; do
  module_dir=${source%/vmlinuz}
  pkgbase=$(<"$target/$module_dir/pkgbase")
  cp "$target/$source" "$target/boot/vmlinuz-$pkgbase"
  : >"$target/etc/mkinitcpio.d/$pkgbase.preset"
done <"$RYOKU_HOOK_STDIN"
EOF
chmod +x "$tmp/fake-kernel-chroot"
(
  log() { :; }
  # shellcheck source=../installation/backend/lib/cachyos.sh
  source "$root/installation/backend/lib/cachyos.sh"
  export RYOKU_HOOK_STDIN="$hook_input"
  RYOKU_DRYRUN='' \
    RYOKU_CACHYOS_TARGET_ROOT="$kernel_target" \
    RYOKU_TARGET_CHROOT="$tmp/fake-kernel-chroot" \
    ryoku_cachyos_refresh_kernel_assets
) || fail "CachyOS layer kernel hook replay failed"
grep -qxF 'usr/lib/modules/6.12-arch/vmlinuz' "$hook_input" \
  || fail "kernel hook replay omitted upgraded stock linux"
grep -qxF 'usr/lib/modules/6.12-cachy/vmlinuz' "$hook_input" \
  || fail "kernel hook replay omitted linux-cachyos"
grep -qxF 'stock-image' "$kernel_target/boot/vmlinuz-linux" \
  || fail "kernel hook replay did not refresh the stock fallback image"
grep -qxF 'cachyos-image' "$kernel_target/boot/vmlinuz-linux-cachyos" \
  || fail "kernel hook replay did not install the CachyOS boot image"

# A missing manifest row disables only the optional layer and leaves the install
# on its stock kernel instead of silently falling back to a duplicated list.
missing_components="$tmp/components-without-cachyos-layer.tsv"
printf 'id\tlabel\tgroup\tgroup_label\tpackages\tdesktop_ids\tdefault\tstep\tneeds_network\teditions\tlock_editions\tlock_reason\tdetail\n' \
  >"$missing_components"
out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
  RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole \
  RYOKU_CACHYOS_LAYER=1 RYOKU_INSTALLER_COMPONENTS_FILE="$missing_components" \
  bash "$root/installation/backend/ryoku-install" 2>&1)" \
  || fail "missing CachyOS component row made the whole install fail: $out"
grep -qF 'the cachyos-layer package row is missing; skipping the layer' <<<"$out" \
  || fail "missing CachyOS component row was not reported"
grep -qxF '        | linux' <<<"$out" \
  || fail "missing CachyOS component row did not leave stock linux as default"
grep -qF '        | /Arch Linux (fallback)' <<<"$out" \
  && fail "missing CachyOS component row still added the layer fallback entry"

# Void applies the same file-manager, app, and Rashin contract and ignores the
# CachyOS-only switch.
void_out=$tmp/void.out
install_dryrun_run void "$void_out" \
  RYOKU_DISK_STRATEGY=whole RYOKU_FIRMWARE_MODE=uefi \
  RYOKU_COMPOSITOR=niri RYOKU_COMPOSITOR_CONFIG_DIR=niri \
  RYOKU_COMPOSITOR_GPU_PIN=gpu.kdl RYOKU_FILE_MANAGER=thunar \
  RYOKU_RASHIN=0 RYOKU_CACHYOS_LAYER=1 \
  || fail "Void install-choice dry run exited nonzero: $(<"$void_out")"
out=$(<"$void_out")
grep -qF 'CachyOS layer is not available on Void' <<<"$out" \
  || fail "Void did not ignore the CachyOS layer"
grep -qF 'DRYRUN: set thunar as the default file manager (inode/directory + desktop.apps.files)' <<<"$out" \
  || fail "Void did not set the selected file-manager role"
grep -qF 'ryotunes' <<<"$out" && grep -qF 'ryomanager' <<<"$out" && grep -qF 'Thunar' <<<"$out" \
  || fail "Void desktop transaction missed ryotunes, ryomanager, or Thunar"
seed_intent="$(grep -F 'DRYRUN: users seed-self account=ryoku ' <<<"$out" | tail -n1)"
[[ $seed_intent == *" rashin=0 "* ]] \
  || fail "Void RYOKU_RASHIN=0 did not disable the delegated Rashin marker: $seed_intent"
transactions=$(grep -F 'xbps-install -r /mnt' <<<"$out")
grep -qw 'ryoku-rashin' <<<"$transactions" \
  && fail "Void installed ryoku-rashin with RYOKU_RASHIN=0"
ledger="${seed_intent##* provisioned-ledger=}"
[[ " ${ledger//,/ } " == *" ryoku-rashin "* ]] || fail "Void Rashin-off ledger is missing ryoku-rashin: $seed_intent"

# Void's desktop package does not pull Rashin in; the installer names it.
install_dryrun_run void "$void_out" \
  RYOKU_DISK_STRATEGY=whole RYOKU_FIRMWARE_MODE=uefi \
  RYOKU_COMPOSITOR=niri RYOKU_COMPOSITOR_CONFIG_DIR=niri \
  RYOKU_COMPOSITOR_GPU_PIN=gpu.kdl \
  || fail "Void default dry run exited nonzero: $(<"$void_out")"
out=$(<"$void_out")
transactions=$(grep -F 'xbps-install -r /mnt' <<<"$out")
grep -qw 'ryoku-rashin' <<<"$transactions" \
  || fail "Void did not install ryoku-rashin by default"
seed_intent="$(grep -F 'DRYRUN: users seed-self account=ryoku ' <<<"$out" | tail -n1)"
[[ $seed_intent == *" rice=default "* && $seed_intent == *" rashin=1 "* ]] \
  || fail "Void did not delegate the default rice and Rashin first-login marker: $seed_intent"

echo "install-dryrun-matrix: browser/shell/app-choice checks passed"
echo "install-dryrun-matrix: all checks passed"
