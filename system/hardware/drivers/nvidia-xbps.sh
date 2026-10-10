#!/usr/bin/env bash
# Void's NVIDIA package, dracut, and kernel-hook policy. Sourced by nvidia.sh
# after the shared GPU detection has found an NVIDIA display controller.

void_kernel_series() {
  local state pkgver rest pkg
  while read -r state pkgver rest; do
    [[ $state == ii ]] || continue
    pkg=${pkgver%%-[0-9]*}
    [[ $pkg =~ ^linux[0-9]+\.[0-9]+$ ]] && printf '%s\n' "$pkg"
  done < <(xbps-query -l 2>/dev/null || true)
}

void_wants_lib32() {
  pkg_installed void-repo-multilib || pkg_installed void-repo-multilib-nonfree
}

void_nvidia_modules_ready() {
  local d kv found=0
  [[ $RYOKU_DRYRUN == 1 ]] && return 0
  for d in "${RYOKU_MODULES_DIR:-/usr/lib/modules}"/*/; do
    [[ -d $d ]] || continue
    found=1
    kv=${d%/}; kv=${kv##*/}
    modinfo -k "$kv" nvidia >/dev/null 2>&1 || return 1
  done
  (( found ))
}

void_write_nvidia_config() {
  local module_present=$1 preserve=$2
  local modprobe=${RYOKU_MODPROBE_CONF:-/etc/modprobe.d/nvidia.conf}
  local dracut=${RYOKU_DRACUT_CONF:-/etc/dracut.conf.d/nvidia.conf}
  run "${PRIV[@]}" mkdir -p "$(dirname "$modprobe")" "$(dirname "$dracut")"
  if (( module_present )); then
    {
      printf '%s\n' 'options nvidia_drm modeset=1 fbdev=1'
      (( preserve )) && printf '%s\n' 'options nvidia NVreg_PreserveVideoMemoryAllocations=1'
      printf '%s\n' 'blacklist nouveau' 'options nouveau modeset=0' 'blacklist nova_core' 'blacklist nova_drm'
    } | write_root "$modprobe"
    write_root "$dracut" <<'EOF'
force_drivers+=" nvidia nvidia_modeset nvidia_drm "
add_drivers+=" nvidia_uvm "
EOF
    echo "nvidia.sh: nvidia module present, enabled early KMS and shadowed the package blacklist safely"
  else
    write_root "$modprobe" <<'EOF'
# Ryoku keeps this file so it shadows the NVIDIA package default.
# Nouveau remains available until DKMS produces a loadable nvidia module.
EOF
    write_root "$dracut" <<'EOF'
# Ryoku leaves NVIDIA out of the initramfs until its kernel module exists.
EOF
    echo "nvidia.sh: WARNING: no nvidia module exists for the installed kernel(s); nouveau remains available"
  fi
}

void_install_guard() {
  local hook=${RYOKU_KERNEL_HOOK:-/etc/kernel.d/post-install/15-ryoku-nvidia}
  run "${PRIV[@]}" mkdir -p "$(dirname "$hook")"
  write_root "$hook" <"$DRIVER_DIR/15-ryoku-nvidia"
  run "${PRIV[@]}" chmod 0755 "$hook"
}

void_reconfigure_kernels() {
  local series
  for series in "$@"; do
    echo "nvidia.sh: rebuilding the $series initramfs"
    run "${PRIV[@]}" xbps-reconfigure -f "$series"
  done
}

mapfile -t void_kernels < <(void_kernel_series | sort -u)
headers=(linux-headers)
for kb in "${void_kernels[@]}"; do headers+=("$kb-headers"); done

lib32=0
if void_wants_lib32; then lib32=1; fi
repos=(void-repo-nonfree)
if (( lib32 )); then repos+=(void-repo-multilib-nonfree); fi

driver_pkg=
preserve=0
if nvidia_has_gsp; then
  driver_pkg=nvidia
  preserve=1
  echo "nvidia.sh: Turing or newer GPU, using Void's nvidia branch."
elif nvidia_is_kepler; then
  driver_pkg=nvidia470
  echo "nvidia.sh: Kepler GPU, using Void's nvidia470 branch."
elif nvidia_is_maxwell_volta; then
  driver_pkg=nvidia580
  preserve=1
  echo "nvidia.sh: Maxwell, Pascal, or Volta GPU, using Void's nvidia580 branch."
else
  echo "nvidia.sh: Fermi or older GPU has no supported Void NVIDIA branch; keeping nouveau."
  void_write_nvidia_config 0 0
  void_install_guard
  void_reconfigure_kernels "${void_kernels[@]}"
  return 0
fi

# Repository packages alter XBPS configuration, so sync them before asking the
# host seam to resolve the nonfree driver in a separate transaction.
if ! install_pkgs "${repos[@]}"; then
  echo "nvidia.sh: WARNING: could not enable Void's NVIDIA repositories; keeping nouveau available."
  void_write_nvidia_config 0 "$preserve"
  void_install_guard
  void_reconfigure_kernels "${void_kernels[@]}"
  return 0
fi

pkgs=("${headers[@]}" "$driver_pkg")
if (( lib32 )); then pkgs+=("$driver_pkg-libs-32bit"); fi
if ! install_pkgs "${pkgs[@]}"; then
  echo "nvidia.sh: WARNING: NVIDIA driver installation failed; keeping nouveau available."
fi

module=0
if void_nvidia_modules_ready; then module=1; fi
void_write_nvidia_config "$module" "$preserve"
void_install_guard
void_reconfigure_kernels "${void_kernels[@]}"
