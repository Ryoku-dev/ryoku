#!/usr/bin/env bash
# hermetic test for the ryoku-gpu render-pin policy: the strongest GPU is pinned
# on desktops AND laptops (GPU work on a laptop's iGPU heats the CPU die), an
# explicit mode choice stored in gpu.lua (`-- ryoku-gpu-mode:`) outranks the
# default, and `check-pin` tells the doctor which way a machine drifted:
# missing-pin (policy wants a pin), stale-pin (the stored mode says no pin),
# forced (a RYOKU_GPU_FORCE override is kept). Fake /sys DRM tree
# (RYOKU_GPU_DRM_ROOT), fake /dev/dri (RYOKU_GPU_DRI_DIR), fake laptop gate
# (RYOKU_ASSUME_LAPTOP, read by the installed ryoku-hw-laptop before any
# sysfs), and redirected config paths, so no hardware, no root, and no
# session file is touched.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
gpu="$here/../system/hardware/gpu/ryoku-gpu"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fail() { echo "FAIL: $1" >&2; exit 1; }

# fake DRM tree: card0 = AMD APU iGPU (UMA carveout, drives the panel),
# card1 = NVIDIA dGPU (no connector).
drm="$tmp/drm"
mk_card() { # <cardN> <driver> <slot-suffix>
  local d="$drm/$1/device"
  mkdir -p "$d"
  printf 'DRIVER=%s\nPCI_SLOT_NAME=0000:0%s:00.0\n' "$2" "$3" >"$d/uevent"
}
mk_card card0 amdgpu 1
printf '2000000000\n' >"$drm/card0/device/mem_info_vram_total"
printf '2000000000\n' >"$drm/card0/device/mem_info_vis_vram_total"
mk_card card1 nvidia 2
mkdir -p "$drm/card0-eDP-1" && printf 'connected\n' >"$drm/card0-eDP-1/status"

# fake /dev/dri with the boot-stable symlinks, so value assembly is
# deterministic and never reads the host's real /dev/dri.
dri="$tmp/dri"; mkdir -p "$dri"
: >"$dri/card0"; : >"$dri/card1"
ln -s card0 "$dri/ryoku-gpu-0000-01-00-0"
ln -s card1 "$dri/ryoku-gpu-0000-02-00-0"

# stub MUX: always capable, always hybrid -> mode performance must print the
# discrete-MUX hint; `set` is never called by these paths.
bin="$tmp/bin"; mkdir -p "$bin"
cat >"$bin/ryoku-gpu-mux" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in
  capable) exit 0 ;;
  get) printf 'hybrid\n'; exit 0 ;;
  *) exit 1 ;;
esac
EOF
chmod +x "$bin/ryoku-gpu-mux"
export PATH="$bin:$PATH"

conf="$tmp/gpu.lua"
niri="$tmp/gpu.kdl"
run() { RYOKU_GPU_DRM_ROOT="$drm" RYOKU_GPU_DRI_DIR="$dri" RYOKU_GPU_CONF="$conf" \
        RYOKU_GPU_NIRI_CONF="$niri" RYOKU_ASSUME_LAPTOP="${LAPTOP:-1}" "$gpu" "$@"; }
verdict() { run check-pin; }

# --- 1. laptop default: persist pins the dGPU, check-pin goes missing-pin -> ok
rm -f "$conf"
[[ "$(verdict)" == "missing-pin" ]] || fail "unpinned laptop must report missing-pin, got $(verdict)"
run persist >/dev/null
grep -Eq '^hl\.env\("AQ_DRM_DEVICES"' "$conf" || fail "persist wrote no pin on a laptop"
grep -q 'ryoku-gpu-primary: 0000:02:00.0' "$conf" || fail "pin primary is not the dGPU"
grep -q 'ryoku-gpu-mode:' "$conf" && fail "default persist must not stamp a mode"
[[ "$(verdict)" == "ok" ]] || fail "fresh laptop pin must be ok, got $(verdict)"

# --- 2. desktop keeps pinning (regression guard)
LAPTOP=0 run persist >/dev/null && [[ "$(verdict)" == "ok" ]] || fail "desktop pin broke"

# --- 3. mode hybrid: unpins, stamps the choice, and persist honours it
run mode hybrid >/dev/null
grep -q 'ryoku-gpu-mode: hybrid' "$conf" || fail "hybrid did not stamp the mode"
grep -Eq '^hl\.env\("AQ_DRM_DEVICES"' "$conf" && fail "hybrid left a pin behind"
[[ "$(verdict)" == "ok" ]] || fail "stored hybrid + no pin must be ok, got $(verdict)"
run persist >/dev/null   # the login autostart call must NOT re-pin over hybrid
grep -Eq '^hl\.env\("AQ_DRM_DEVICES"' "$conf" && fail "persist overwrote an explicit hybrid choice"

# --- 4. mode performance: re-pins, stamps performance, and says the MUX word
run mode performance 2>"$tmp/err" >/dev/null
grep -Eq '^hl\.env\("AQ_DRM_DEVICES"' "$conf" || fail "performance wrote no pin"
grep -q 'ryoku-gpu-mode: performance' "$conf" || fail "performance did not stamp the mode"
grep -qi 'mux' "$tmp/err" || fail "MUX-capable laptop must be told about the discrete flip"
[[ "$(verdict)" == "ok" ]] || fail "performance pin must be ok, got $(verdict)"

# --- 5. a pin that contradicts the stored mode is stale-pin
sed -i '/ryoku-gpu-mode: performance/d' "$conf"
printf -- '-- ryoku-gpu-mode: hybrid\n' >>"$conf"
v="$(verdict)"
[[ "$v" == "stale-pin 0000:02:00.0" ]] || fail "pin against stored hybrid must be stale-pin, got $v"
run disable >/dev/null   # bare disable keeps the stored choice
grep -q 'ryoku-gpu-mode: hybrid' "$conf" || fail "bare disable dropped the stored mode"
[[ "$(verdict)" == "ok" ]] || fail "cleared hybrid machine must be ok, got $(verdict)"

# --- 6. passthrough: solo iGPU pin, stamped, and its own verdict is ok
run mode passthrough >/dev/null
grep -q 'ryoku-gpu-mode: passthrough' "$conf" || fail "passthrough did not stamp"
grep -Eq '^hl\.env\("AQ_DRM_DEVICES"' "$conf" || fail "passthrough wrote no solo pin"
grep -q 'ryoku-gpu-0000-01-00-0' "$conf" || fail "passthrough pin is not the iGPU"
grep -q 'ryoku-gpu-0000-02-00-0' "$conf" && fail "passthrough kept the dGPU in the list"
[[ "$(verdict)" == "ok" ]] || fail "passthrough pin must be ok, got $(verdict)"

# --- 7. a forced pin is kept whatever the stored mode says
run mode hybrid >/dev/null
RYOKU_GPU_FORCE=1 run persist >/dev/null
grep -q 'ryoku-gpu-forced' "$conf" || fail "force wrote no marker"
[[ "$(verdict)" == "forced" ]] || fail "forced pin must report forced, got $(verdict)"

# --- 8. single-GPU box: nothing to pin, nothing to repair
drm1="$tmp/drm1"; dri1="$tmp/dri1"; mkdir -p "$drm1/card0/device" "$drm1/card0-eDP-1" "$dri1"
printf 'DRIVER=amdgpu\nPCI_SLOT_NAME=0000:01:00.0\n' >"$drm1/card0/device/uevent"
printf '2000000000\n' >"$drm1/card0/device/mem_info_vram_total"
printf '2000000000\n' >"$drm1/card0/device/mem_info_vis_vram_total"
printf 'connected\n' >"$drm1/card0-eDP-1/status"
[[ "$(RYOKU_GPU_DRM_ROOT="$drm1" RYOKU_GPU_DRI_DIR="$dri1" RYOKU_GPU_CONF="$tmp/conf-solo" \
     RYOKU_GPU_NIRI_CONF="$niri" RYOKU_ASSUME_LAPTOP=1 "$gpu" check-pin)" == "ok" ]] \
  || fail "single-GPU box must be ok"

# --- 9. an eGPU on a laptop with no stored choice pins (missing-pin -> ok)
drme="$tmp/drme"; mkdir -p "$drme/card0/device" "$drme/card9/device" "$drme/card0-eDP-1"
printf 'DRIVER=amdgpu\nPCI_SLOT_NAME=0000:01:00.0\n' >"$drme/card0/device/uevent"
printf '2000000000\n' >"$drme/card0/device/mem_info_vram_total"
printf '2000000000\n' >"$drme/card0/device/mem_info_vis_vram_total"
printf 'connected\n' >"$drme/card0-eDP-1/status"
printf 'DRIVER=nvidia\nPCI_SLOT_NAME=0000:06:00.0\n' >"$drme/card9/device/uevent"
echo removable >"$drme/card9/device/removable"
ln -s card9 "$dri/ryoku-gpu-0000-06-00-0"
rm -f "$tmp/conf-egpu"   # fresh conf: no forced marker from case 7
egpu() { RYOKU_GPU_DRM_ROOT="$drme" RYOKU_GPU_DRI_DIR="$dri" RYOKU_GPU_CONF="$tmp/conf-egpu" \
         RYOKU_GPU_NIRI_CONF="$niri" RYOKU_ASSUME_LAPTOP=1 "$gpu" "$@"; }
[[ "$(egpu check-pin)" == "missing-pin" ]] || fail "unpinned eGPU laptop must report missing-pin"
egpu persist >/dev/null
[[ "$(egpu check-pin)" == "ok" ]] || fail "eGPU pin on a laptop must be ok, got $(egpu check-pin)"

echo "gpu-pin-policy: all cases passed"
