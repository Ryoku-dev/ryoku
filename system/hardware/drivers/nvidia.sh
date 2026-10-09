#!/usr/bin/env bash
#
# nvidia.sh: pick + install the right NVIDIA stack if a card is there.
#
# Turing+ (GTX 16xx, RTX 20xx-up, datacenter A/H/L) ship GSP firmware and use
# the open kernel modules. Kepler needs NVIDIA's 470xx legacy branch; Maxwell,
# Pascal, and Volta use 580xx. The early-KMS mkinitcpio config is written only
# when the module actually landed, so a driver that cannot build never breaks the
# initramfs (the machine still boots on the iGPU).
#
# idempotent, gated on detection, dry-run via RYOKU_DRYRUN (or --dry-run).

set -euo pipefail

RYOKU_DRYRUN="${RYOKU_DRYRUN:-0}"
for arg in "$@"; do
  case "$arg" in
    --dry-run) RYOKU_DRYRUN=1 ;;
    -h | --help)
      echo "Usage: nvidia.sh [--dry-run]"
      echo "Install NVIDIA drivers (open for Turing+, proprietary otherwise) if an NVIDIA GPU is present."
      exit 0
      ;;
    *)
      echo "nvidia.sh: unknown argument: $arg" >&2
      exit 1
      ;;
  esac
done

run() {
  if [[ $RYOKU_DRYRUN == 1 ]]; then
    printf 'DRYRUN: %s\n' "$*"
    return 0
  fi
  "$@"
}

write_root() {
  local path=$1
  if [[ $RYOKU_DRYRUN == 1 ]]; then
    printf 'DRYRUN: write %s\n' "$path"
    cat >/dev/null
    return 0
  fi
  if (( EUID == 0 )); then cat >"$path"; else sudo -n tee "$path" >/dev/null; fi
}

PKG=(ryoku-host pkg)
PRIV=()
if (( EUID != 0 )); then
  PKG=(sudo -n ryoku-host pkg)
  PRIV=(sudo -n)
fi

pkg_installed() { ryoku-host pkg installed "$1" >/dev/null 2>&1; }

install_pkgs() {
  local missing=() p
  for p in "$@"; do pkg_installed "$p" || missing+=("$p"); done
  if (( ${#missing[@]} == 0 )); then
    echo "nvidia.sh: already installed: $*"
    return 0
  fi
  echo "nvidia.sh: installing ${missing[*]}"
  run "${PKG[@]}" install "${missing[@]}"
}

has_lspci() { command -v lspci >/dev/null 2>&1; }

has_nvidia() {
  if has_lspci && lspci 2>/dev/null | grep -qi 'nvidia'; then
    return 0
  fi
  # no lspci (missing in the pacstrap chroot): scan sysfs for a PCI display
  # controller (class 0x03xxxx) made by NVIDIA (vendor 0x10de). works before
  # any driver binds, unlike a DRIVER= uevent match.
  local dev vendor class
  for dev in /sys/bus/pci/devices/*/; do
    [[ -r $dev/vendor && -r $dev/class ]] || continue
    read -r vendor <"$dev/vendor" || continue
    [[ $vendor == 0x10de ]] || continue
    read -r class <"$dev/class" || continue
    [[ $class == 0x03* ]] && return 0
  done
  return 1
}

# GSP-firmware cards (-> open modules): GTX 16xx, RTX 20xx-50xx, RTX Pro,
# Quadro RTX, datacenter A/H/T/L. ported from ryoku-hw-nvidia-gsp.
nvidia_has_gsp() {
  has_lspci || return 0   # no lspci -> assume modern (open). safe default.
  lspci | grep -i 'nvidia' \
    | grep -qE "GTX 16[0-9]{2}|RTX [2-5][0-9]{3}|RTX PRO [0-9]{4}|Quadro RTX|RTX A[0-9]{4}|A[1-9][0-9]{2}|H[1-9][0-9]{2}|T4|L[0-9]+"
}

# Kepler names in pci.ids carry the GPU's GK chip code (GK104/GK106/GK107/GK208).
# NVIDIA's 580xx branch no longer supports it; selecting that branch can leave a
# loadable but unbound module while Nouveau is blacklisted.
nvidia_is_kepler() {
  has_lspci || return 1
  lspci | grep -i 'nvidia' | grep -qE '(^|[^[:alnum:]])GK[0-9]+'
}

# prebuilt_for <kernel-pkgbase>: print the kernel-matched prebuilt open-module
# package for that kernel when a synced repo actually carries one, else fail.
# Stock linux is nvidia-open in [extra]; distro kernels follow the
# <pkgbase>-nvidia-open convention (CachyOS ships linux-cachyos-nvidia-open,
# #214). The repo query is what makes the mapping honest: a name no synced db
# carries (a hand-built kernel, or an offline ISO that never baked it) must
# fall through to DKMS rather than abort the transaction. ryoku-host applies
# the same offline package configuration as the install transaction.
prebuilt_for() {
  local kb=$1 cand
  if [[ $kb == linux ]]; then cand=nvidia-open; else cand="$kb-nvidia-open"; fi
  ryoku-host pkg available "$cand" >/dev/null 2>&1 && printf '%s\n' "$cand"
}

if ! has_nvidia; then
  echo "nvidia.sh: no NVIDIA GPU detected, nothing to do."
  exit 0
fi

if [[ $(ryoku-host pkgmgr) != pacman ]]; then
  echo "nvidia.sh: the proprietary NVIDIA driver is not wired on this distribution yet; it needs a driver branch per GPU, dracut modules, and elogind sleep hooks. The card stays on nouveau and Mesa."
  exit 0
fi

# an installed module package stays: CachyOS boxes ship kernel-matched
# prebuilt modules (linux-cachyos-nvidia-open) that provide NVIDIA-MODULE,
# and adding a -dkms package conflicts with it, aborting the transaction.
have_module_pkg() {
  local candidates=(nvidia nvidia-open nvidia-dkms nvidia-open-dkms nvidia-lts nvidia-open-lts)
  local pb kb pkg
  for pb in "${RYOKU_MODULES_DIR:-/usr/lib/modules}"/*/pkgbase; do
    [[ -r $pb ]] || continue
    read -r kb <"$pb"
    candidates+=("$kb-nvidia" "$kb-nvidia-open")
  done
  for pkg in "${candidates[@]}"; do
    pkg_installed "$pkg" && return 0
  done
  return 1
}

# nvidia_module_present: is the nvidia kernel module actually available for an
# installed target kernel? checks each /usr/lib/modules/<ver> tree with
# modinfo -k (not the chroot's running host kernel). dry-run assumes present.
nvidia_module_present() {
  [[ $RYOKU_DRYRUN == 1 ]] && return 0
  local d kv
  for d in "${RYOKU_MODULES_DIR:-/usr/lib/modules}"/*/; do
    [[ -d $d ]] || continue
    kv=${d%/}; kv=${kv##*/}
    modinfo -k "$kv" nvidia >/dev/null 2>&1 && return 0
  done
  return 1
}

if have_module_pkg; then
  echo "nvidia.sh: an NVIDIA kernel module package is already installed, keeping it"
  install_pkgs nvidia-utils libva-nvidia-driver
else
  # enumerate the installed kernels via pkgbase (linux, linux-zen, ...).
  kernels=()
  for pb in "${RYOKU_MODULES_DIR:-/usr/lib/modules}"/*/pkgbase; do
    [[ -r $pb ]] || continue
    read -r kb <"$pb"
    kernels+=("$kb")
  done
  (( ${#kernels[@]} )) || kernels=(linux)
  mapfile -t kernels < <(printf '%s\n' "${kernels[@]}" | sort -u)

  # NVIDIA driver by GPU generation. Turing+ (GSP firmware) runs the open
  # modules, and the prebuilt kernel-matched package is always preferred over
  # DKMS: stock linux has nvidia-open in [extra], and distro kernels ship their
  # own (CachyOS: linux-cachyos-nvidia-open), so those boxes never pay a
  # per-kernel-transaction compile (#214). Only a kernel with no prebuilt in
  # any synced repo falls to nvidia-open-dkms. Kepler is the older 470xx legacy
  # branch. Maxwell/Pascal/Volta are not covered by the open modules and use
  # the AUR 580xx branch. The ISO bakes both legacy branches into the offline
  # repo best-effort (they are large vendor blobs), so these usually install
  # with no network; when a bake was skipped the card falls back to
  # nouveau/mesa. Headers cover the DKMS build.
  headers=()
  for kb in "${kernels[@]}"; do headers+=("${kb}-headers"); done
  multilib=0
  if pacman-conf --repo-list 2>/dev/null | grep -qx multilib; then multilib=1; fi
  if nvidia_has_gsp; then
    base=(nvidia-utils libva-nvidia-driver)
    if (( multilib )); then base+=(lib32-nvidia-utils); fi
    prebuilts=()
    for kb in "${kernels[@]}"; do
      if p=$(prebuilt_for "$kb" open); then prebuilts+=("$p"); else prebuilts=(); break; fi
    done
    if (( ${#prebuilts[@]} )); then
      echo "nvidia.sh: Turing+ GPU, using the prebuilt open module(s): ${prebuilts[*]}."
      pkgs=("${prebuilts[@]}" "${base[@]}")
    else
      echo "nvidia.sh: Turing+ GPU, no prebuilt module for kernel(s) ${kernels[*]}, using nvidia-open-dkms."
      pkgs=(nvidia-open-dkms "${base[@]}" "${headers[@]}")
    fi
  elif nvidia_is_kepler; then
    echo "nvidia.sh: Kepler GPU, using the legacy 470xx branch (nvidia-470xx-dkms)."
    pkgs=(nvidia-470xx-dkms libva-nvidia-driver "${headers[@]}")
    if (( multilib )); then pkgs+=(lib32-nvidia-470xx-utils); fi
  else
    echo "nvidia.sh: Maxwell, Pascal, or Volta GPU, using the legacy 580xx branch (nvidia-580xx-dkms)."
    pkgs=(nvidia-580xx-dkms libva-nvidia-driver "${headers[@]}")
    if (( multilib )); then pkgs+=(lib32-nvidia-580xx-utils); fi
  fi
  install_pkgs "${pkgs[@]}" || echo "nvidia.sh: WARNING: NVIDIA driver install failed. A Kepler or older card needs the 470xx branch (nvidia-470xx-dkms); a Turing+ card needs a matching kernel/driver. On an offline install the legacy branches are only present if the ISO managed to bake them (they are large vendor blobs, best-effort); nouveau/mesa still brings the desktop up. Install a working driver and run 'mkinitcpio -P' to enable it."
fi

# early KMS + the boot-race fix. DRM modeset is mandatory for a working
# NVIDIA Wayland session, and the modules have to come from the initramfs
# (rebuilt by the bootloader step after this). nouveau gets denylisted so
# it can't also bind the card: with both modules eligible they race at
# boot, and the card only shows up on some boots (the wonky-detection bug).
# PreserveVideoMemoryAllocations = session survives suspend.
#
# All gated on the module existing: denylisting nouveau without a working
# nvidia module leaves an NVIDIA-only desktop with no driver at all.
if nvidia_module_present; then
  echo "nvidia.sh: writing modeset + nouveau denylist"
  write_root /etc/modprobe.d/nvidia.conf <<'EOF'
options nvidia_drm modeset=1 fbdev=1
options nvidia NVreg_PreserveVideoMemoryAllocations=1
blacklist nouveau
options nouveau modeset=0
EOF
  echo "nvidia.sh: nvidia module present, writing initramfs early-KMS config"
  write_root /etc/mkinitcpio.conf.d/nvidia.conf <<'EOF'
MODULES=(nvidia nvidia_modeset nvidia_uvm nvidia_drm)
EOF
else
  echo "nvidia.sh: WARNING: nvidia kernel module not found for the installed kernel(s); leaving nouveau available and skipping the mkinitcpio MODULES drop-in so the machine still gets a display. Install a matching nvidia driver/kernel and run 'ryoku doctor' (or 'mkinitcpio -P') to enable the proper driver."
  run "${PRIV[@]}" rm -f /etc/mkinitcpio.conf.d/nvidia.conf /etc/modprobe.d/nvidia.conf
fi

# suspend/resume: NVIDIA has to save + restore VRAM across sleep or the
# session comes back corrupted. units ship in nvidia-utils; enabling them
# is offline-safe (just symlinks), so this works in the install chroot too.
# missing unit tolerated.
if command -v systemctl >/dev/null 2>&1; then
  echo "nvidia.sh: enabling NVIDIA suspend/resume services"
  run "${PRIV[@]}" systemctl enable nvidia-suspend.service nvidia-hibernate.service nvidia-resume.service || true
fi

# keep the initramfs in step with the NVIDIA modules AND break the SDDM login
# loop on a later kernel update. ryoku-nvidia-guard (shipped to /usr/bin by
# ryoku-desktop) runs after every kernel or NVIDIA-driver transaction: a -dkms
# module that fails to rebuild on a new kernel would otherwise leave nouveau
# blacklisted with no nvidia module -- no driver binds the card, the greeter
# draws on simpledrm but Hyprland can't, and SDDM loops the login. The guard
# restores nouveau in that case; a plain driver update just gets the rebuild.
# `ryoku doctor` writes the same hook onto boxes installed before it existed.
# Path trigger = any kernel; glob package targets = every NVIDIA branch (open,
# dkms, legacy 580xx/470xx, prebuilt linux-*-nvidia). NeedsTargets feeds the
# guard the changed names so it only rebuilds when the baked module can be stale.
echo "nvidia.sh: installing the NVIDIA initramfs + login-loop guard hook"
run "${PRIV[@]}" mkdir -p /etc/pacman.d/hooks
write_root /etc/pacman.d/hooks/ryoku-nvidia.hook <<'EOF'
[Trigger]
Operation=Install
Operation=Upgrade
Operation=Remove
Type=Path
Target=usr/lib/modules/*/vmlinuz

[Trigger]
Operation=Install
Operation=Upgrade
Operation=Remove
Type=Package
Target=nvidia*
Target=lib32-nvidia*
Target=libva-nvidia-driver
Target=linux-*-nvidia*

[Action]
Description=Ryoku: reconciling NVIDIA modules and the initramfs (guards the login loop)...
When=PostTransaction
NeedsTargets
Exec=/usr/bin/ryoku-nvidia-guard
EOF
