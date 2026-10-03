#!/usr/bin/env bash
# End-to-end dry-run coverage for strategy, encryption, swap, ESP mode, and ISO
# variant. Every successful run must emit the canonical stage sentinels and end
# with exactly one @@RYOKU_DONE.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$here/.."
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
  rc=0
  out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" \
    RYOKU_DISK=/dev/vda RYOKU_PASSWORD_HASH='$6$fake$hash' \
    RYOKU_DISK_STRATEGY="$1" RYOKU_ENCRYPT="$2" RYOKU_SWAP_GIB="$3" \
    RYOKU_ESP_MODE="${4:-auto}" RYOKU_VARIANT="${5:-plain}" \
    RYOKU_LUKS_PASSPHRASE=passphrase \
    bash "$root/installation/backend/ryoku-install" 2>&1)" || rc=$?
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


# Browser + app-choice wiring: the default (no RYOKU_BROWSER) installs Zen and
# drops the other two browsers; naming one browser drops the others; the drop
# list removes shipped apps from the pacstrap set, one package per name.
pcount() {
  grep -oE 'installing [0-9]+ packages' <<<"$1" | grep -oE '[0-9]+' | head -1
}
out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
  RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole \
  bash "$root/installation/backend/ryoku-install" 2>&1)" || fail "default-browser dry run exited nonzero: $out"
grep -qF 'DRYRUN: set zen as the default web browser' <<<"$out" \
  || fail "the default browser pick did not narrate zen"
base_n=$(pcount "$out")
[[ -n $base_n ]] || fail "no pacstrap package count in the default dry run"

out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
  RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole RYOKU_BROWSER=firefox \
  bash "$root/installation/backend/ryoku-install" 2>&1)" || fail "firefox dry run exited nonzero: $out"
grep -qF 'DRYRUN: set firefox as the default web browser' <<<"$out" \
  || fail "RYOKU_BROWSER=firefox did not narrate firefox"
grep -qF 'DRYRUN: seed /mnt/home/ryoku/.local/state/ryoku/provisioned with the dropped packages' <<<"$out" \
  || fail "the drop list did not narrate provisioning-ledger seeding"
ff_n=$(pcount "$out")
# the zen default drops BOTH repo browsers from the base set; picking firefox
# trades the dropped firefox for the dropped chromium, so exactly one package
# comes back into the transaction.
[[ $ff_n -eq $((base_n + 1)) ]] \
  || fail "picking firefox did not re-add exactly firefox to the base set (zen default $base_n -> firefox $ff_n)"

# dropping two base-set apps removes exactly two packages from the transaction.
out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
  RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole RYOKU_DROP_PACKAGES=docker,flatpak \
  bash "$root/installation/backend/ryoku-install" 2>&1)" || fail "drop dry run exited nonzero: $out"
drop_n=$(pcount "$out")
[[ $drop_n -eq $((base_n - 2)) ]] \
  || fail "RYOKU_DROP_PACKAGES=docker,flatpak did not remove exactly 2 packages ($base_n -> $drop_n)"

# an unknown browser is refused before anything runs.
out="$(RYOKU_DRYRUN=1 RYOKU_REPO="$root" RYOKU_DISK=/dev/vda \
  RYOKU_PASSWORD_HASH='$6$fake$hash' RYOKU_DISK_STRATEGY=whole RYOKU_BROWSER=brave \
  bash "$root/installation/backend/ryoku-install" 2>&1)" && fail "RYOKU_BROWSER=brave was accepted"
grep -qF 'not one of the shipped browsers' <<<"$out" \
  || fail "the browser gate did not explain the allowed values"

echo "install-dryrun-matrix: browser/app-choice checks passed"
echo "install-dryrun-matrix: all checks passed"
