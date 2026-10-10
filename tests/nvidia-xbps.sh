#!/usr/bin/env bash
set -euo pipefail

ROOT=${RYOKU_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}
driver="$ROOT/system/hardware/drivers/nvidia.sh"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

make_fakes() {
  local dir=$1 gpu=$2
  mkdir -p "$dir/bin" "$dir/etc/modprobe.d" "$dir/etc/dracut.conf.d" "$dir/etc/kernel.d/post-install" "$dir/modules/6.12.0" "$dir/modules/6.6.0"
  cat >"$dir/bin/lspci" <<EOF
#!/usr/bin/env bash
printf '%s\n' '$gpu'
EOF
  cat >"$dir/bin/ryoku-host" <<'EOF'
#!/usr/bin/env bash
if [[ $1 == pkgmgr ]]; then printf '%s\n' xbps; exit 0; fi
[[ $1 == pkg ]] || exit 2
shift
if [[ $1 == installed ]]; then
  case ${2:-} in
    void-repo-multilib) [[ ${WANT_LIB32:-0} == 1 ]] ;;
    *) exit 1 ;;
  esac
  exit
fi
if [[ $1 == install ]]; then
  printf 'pkg install' >>"$NVIDIA_TEST_LOG"
  shift
  printf ' %s' "$@" >>"$NVIDIA_TEST_LOG"
  printf '\n' >>"$NVIDIA_TEST_LOG"
  exit 0
fi
exit 1
EOF
  cat >"$dir/bin/xbps-query" <<'EOF'
#!/usr/bin/env bash
if [[ ${1:-} == -l ]]; then
  printf '%s\n' 'ii linux6.12-6.12.58_1 The Linux kernel' 'ii linux6.6-6.6.91_1 The Linux kernel'
  exit 0
fi
[[ ${1:-} == "${XBPS_DRIVER:-}" ]]
EOF
  cat >"$dir/bin/modinfo" <<'EOF'
#!/usr/bin/env bash
[[ ${MODULE_PRESENT:-0} == 1 ]]
EOF
  cat >"$dir/bin/xbps-reconfigure" <<'EOF'
#!/usr/bin/env bash
printf 'reconfigure %s\n' "$*" >>"$NVIDIA_TEST_LOG"
EOF
  cat >"$dir/bin/logger" <<'EOF'
#!/usr/bin/env bash
printf 'logger %s\n' "$*" >>"$NVIDIA_TEST_LOG"
EOF
  cat >"$dir/bin/sudo" <<'EOF'
#!/usr/bin/env bash
[[ ${1:-} == -n ]] && shift
exec "$@"
EOF
  chmod +x "$dir/bin"/*
}

run_case() {
  local name=$1 gpu=$2 module=$3 lib32=$4
  local dir="$work/$name"
  make_fakes "$dir" "$gpu"
  : >"$dir/log"
  PATH="$dir/bin:$PATH" NVIDIA_TEST_LOG="$dir/log" MODULE_PRESENT=$module WANT_LIB32=$lib32 \
    RYOKU_MODULES_DIR="$dir/modules" \
    RYOKU_MODPROBE_CONF="$dir/etc/modprobe.d/nvidia.conf" \
    RYOKU_DRACUT_CONF="$dir/etc/dracut.conf.d/nvidia.conf" \
    RYOKU_KERNEL_HOOK="$dir/etc/kernel.d/post-install/15-ryoku-nvidia" \
    bash "$driver" >"$dir/out" 2>&1
}

assert_contains() {
  local file=$1 text=$2
  grep -Fq -- "$text" "$file" || { printf 'missing %q in %s\n' "$text" "$file" >&2; exit 1; }
}

assert_not_contains() {
  local file=$1 text=$2
  ! grep -Fq -- "$text" "$file" || { printf 'unexpected %q in %s\n' "$text" "$file" >&2; exit 1; }
}

assert_repo_first() {
  local file=$1
  local first second
  first=$(grep -n '^pkg install' "$file" | sed -n '1s/:.*//p')
  second=$(grep -n '^pkg install' "$file" | sed -n '2s/:.*//p')
  [[ -n $first && -n $second && $first -lt $second ]] || { echo "repositories and driver were not separate ordered transactions" >&2; exit 1; }
  sed -n "${first}p" "$file" | grep -Fq 'void-repo-nonfree'
}

run_case turing '01:00.0 VGA compatible controller: NVIDIA Corporation TU104 [GeForce RTX 2070 SUPER]' 1 1
assert_repo_first "$work/turing/log"
assert_contains "$work/turing/log" 'pkg install void-repo-nonfree void-repo-multilib-nonfree'
assert_contains "$work/turing/log" 'linux-headers linux6.12-headers linux6.6-headers nvidia nvidia-libs-32bit'
assert_contains "$work/turing/etc/modprobe.d/nvidia.conf" 'NVreg_PreserveVideoMemoryAllocations=1'
assert_contains "$work/turing/etc/modprobe.d/nvidia.conf" 'blacklist nova_core'
assert_contains "$work/turing/etc/dracut.conf.d/nvidia.conf" 'force_drivers+=" nvidia nvidia_modeset nvidia_drm "'
[[ -x $work/turing/etc/kernel.d/post-install/15-ryoku-nvidia ]]
assert_contains "$work/turing/log" 'reconfigure -f linux6.12'
assert_contains "$work/turing/log" 'reconfigure -f linux6.6'

run_case pascal '01:00.0 VGA compatible controller: NVIDIA Corporation GP104 [GeForce GTX 1080]' 1 0
assert_repo_first "$work/pascal/log"
assert_contains "$work/pascal/log" 'linux-headers linux6.12-headers linux6.6-headers nvidia580'
assert_not_contains "$work/pascal/log" 'nvidia580-libs-32bit'
assert_contains "$work/pascal/etc/modprobe.d/nvidia.conf" 'NVreg_PreserveVideoMemoryAllocations=1'

run_case kepler '01:00.0 VGA compatible controller: NVIDIA Corporation GK208B [GeForce GT 710]' 1 1
assert_repo_first "$work/kepler/log"
assert_contains "$work/kepler/log" 'nvidia470 nvidia470-libs-32bit'
assert_not_contains "$work/kepler/etc/modprobe.d/nvidia.conf" 'NVreg_PreserveVideoMemoryAllocations=1'

run_case failed_dkms '01:00.0 VGA compatible controller: NVIDIA Corporation GP107 [GeForce GTX 1050 Ti]' 0 0
assert_not_contains "$work/failed_dkms/etc/modprobe.d/nvidia.conf" 'blacklist nouveau'
assert_not_contains "$work/failed_dkms/etc/dracut.conf.d/nvidia.conf" 'force_drivers'
assert_contains "$work/failed_dkms/out" 'nouveau remains available'

run_case fermi '01:00.0 VGA compatible controller: NVIDIA Corporation GF114 [GeForce GTX 560 Ti]' 0 0
assert_contains "$work/fermi/out" 'Fermi or older GPU has no supported Void NVIDIA branch; keeping nouveau.'
assert_not_contains "$work/fermi/log" ' nvidia390'

hook="$work/failed_dkms/etc/kernel.d/post-install/15-ryoku-nvidia"
MODULE_PRESENT=1 XBPS_DRIVER=nvidia580 NVIDIA_TEST_LOG="$work/failed_dkms/log" PATH="$work/failed_dkms/bin:$PATH" \
  RYOKU_MODPROBE_CONF="$work/failed_dkms/etc/modprobe.d/nvidia.conf" \
  RYOKU_DRACUT_CONF="$work/failed_dkms/etc/dracut.conf.d/nvidia.conf" \
  "$hook" linux6.12 6.12.58_1
assert_contains "$work/failed_dkms/etc/modprobe.d/nvidia.conf" 'blacklist nouveau'
assert_contains "$work/failed_dkms/etc/modprobe.d/nvidia.conf" 'NVreg_PreserveVideoMemoryAllocations=1'
assert_contains "$work/failed_dkms/etc/dracut.conf.d/nvidia.conf" 'force_drivers'

MODULE_PRESENT=0 XBPS_DRIVER=nvidia580 NVIDIA_TEST_LOG="$work/failed_dkms/log" PATH="$work/failed_dkms/bin:$PATH" \
  RYOKU_MODPROBE_CONF="$work/failed_dkms/etc/modprobe.d/nvidia.conf" \
  RYOKU_DRACUT_CONF="$work/failed_dkms/etc/dracut.conf.d/nvidia.conf" \
  "$hook" linux6.12 6.12.59_1
assert_not_contains "$work/failed_dkms/etc/modprobe.d/nvidia.conf" 'blacklist nouveau'
assert_not_contains "$work/failed_dkms/etc/dracut.conf.d/nvidia.conf" 'force_drivers'
assert_contains "$work/failed_dkms/log" 'logger -t ryoku-nvidia nvidia module missing for 6.12.59_1 after DKMS; leaving nouveau available'

printf '%s\n' 'nvidia xbps: OK'
