#!/usr/bin/env bash
# Regression tests for ryoku-hw-backlight-fix's gates.
#   #54: the AMD-only acpi_backlight=native quirk must not fire on Intel+NVIDIA
#        laptops, where nvidia_wmi_ec_backlight also registers with no AMD GPU.
#   #729: native is restricted to the one confirmed board, and an existing
#         broad drop-in is removed and baked out of the boot entry elsewhere.
set -euo pipefail

here="$(cd "$(dirname "$0")/.." && pwd)"
helper="$here/system/hardware/display/ryoku-hw-backlight-fix"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin"
for generator in limine-mkinitcpio limine-update; do
  printf '#!/bin/sh\nprintf "%%s\\n" "%s" >>"%s"\n' \
    "$generator" "$tmp/generator.log" >"$tmp/bin/$generator"
  chmod +x "$tmp/bin/$generator"
done

# fake sysfs: two DRM cards (card0 the iGPU, card1 the NVIDIA dGPU) plus a
# backlight dir. $1 = amd|intel picks card0's PCI vendor.
setup() {
  local kind="$1" vendor="0x8086"
  [[ $kind == amd ]] && vendor="0x1002"
  rm -rf "$tmp/drm" "$tmp/bl"
  mkdir -p "$tmp/drm/card0/device" "$tmp/drm/card1/device" "$tmp/bl"
  printf '%s\n' "$vendor" >"$tmp/drm/card0/device/vendor"
  printf '%s\n' "0x10de" >"$tmp/drm/card1/device/vendor"
}

run_fix() { # $1 = board name to fake in DMI; $2 = dry-run setting (default 1)
  local board="$1" dryrun="${2:-1}"
  mkdir -p "$tmp/dmi"; printf '%s\n' "$board" >"$tmp/dmi/product_name"
  PATH="$tmp/bin:$PATH" RYOKU_DRYRUN="$dryrun" RYOKU_DRM_PATH="$tmp/drm" \
    RYOKU_BACKLIGHT_PATH="$tmp/bl" RYOKU_DROPIN="$tmp/dropin.conf" \
    RYOKU_DMI_PATH="$tmp/dmi" "$helper" 2>&1
}

# 1. Intel+NVIDIA, nvidia_wmi_ec present, no amdgpu_bl: must NOT fire (#54).
setup intel
mkdir -p "$tmp/bl/nvidia_wmi_ec_backlight"
out="$(run_fix FA506NCG)"
case "$out" in
  *"no AMD GPU"*) ;;
  *) echo "FAIL: fired on Intel+NVIDIA (#54): $out" >&2; exit 1 ;;
esac
case "$out" in
  *write*) echo "FAIL: wrote a drop-in on Intel+NVIDIA (#54): $out" >&2; exit 1 ;;
esac

# 2. The confirmed FA506NCG quirk, nvidia_wmi_ec present, no amdgpu_bl:
#    must apply the fix.
setup amd
mkdir -p "$tmp/bl/nvidia_wmi_ec_backlight"
out="$(run_fix "ASUS TUF Gaming A15 FA506NCG_FA506NCG")"
case "$out" in
  *"write $tmp/dropin.conf"*) ;;
  *) echo "FAIL: did not apply the fix on FA506NCG: $out" >&2; exit 1 ;;
esac

# 3. AMD present but amdgpu_bl* already works: no-op.
setup amd
mkdir -p "$tmp/bl/nvidia_wmi_ec_backlight" "$tmp/bl/amdgpu_bl0"
out="$(run_fix FA506NCG)"
case "$out" in
  *"already present"*) ;;
  *) echo "FAIL: did not no-op when amdgpu_bl* exists: $out" >&2; exit 1 ;;
esac

# 4. FA507XV with a drop-in from the former broad gate must remove it and
#    regenerate the boot entries (#729).
setup amd
mkdir -p "$tmp/bl/nvidia_wmi_ec_backlight"
printf 'KERNEL_CMDLINE[default]+= acpi_backlight=native\n' >"$tmp/dropin.conf"
out="$(run_fix "ASUS TUF Gaming A15 FA507XV_FA507XV" 0)"
case "$out" in
  *"removing $tmp/dropin.conf"*) ;;
  *) echo "FAIL: did not remove the drop-in on FA507XV: $out" >&2; exit 1 ;;
esac
if [[ -e "$tmp/dropin.conf" ]]; then
  echo "FAIL: FA507XV drop-in still exists after repair" >&2
  exit 1
fi
generators="$(cat "$tmp/generator.log")"
if [[ $generators != $'limine-mkinitcpio\nlimine-update' ]]; then
  echo "FAIL: boot entries were not regenerated after removal: $generators" >&2
  exit 1
fi

# 5. FA507XV without a prior drop-in must skip cleanly and write nothing.
rm -f "$tmp/dropin.conf"
out="$(run_fix "ASUS TUF Gaming A15 FA507XV_FA507XV")"
case "$out" in
  *"not confirmed to need acpi_backlight=native, skipping"*) ;;
  *) echo "FAIL: FA507XV did not skip cleanly: $out" >&2; exit 1 ;;
esac
case "$out" in
  *write*) echo "FAIL: wrote a drop-in on FA507XV: $out" >&2; exit 1 ;;
esac

echo "backlight-fix-gate: ok"
