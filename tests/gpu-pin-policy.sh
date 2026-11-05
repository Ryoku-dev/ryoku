#!/usr/bin/env bash
# Hermetic coverage for panel-safe defaults, Performance gating, and doctor verdicts.
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
  mkdir -p "$d/drm/renderD$((128 + $3))"
  printf 'DRIVER=%s\nPCI_SLOT_NAME=0000:0%s:00.0\n' "$2" "$3" >"$d/uevent"
}
mk_card card0 amdgpu 1
printf '2000000000\n' >"$drm/card0/device/mem_info_vram_total"
printf '2000000000\n' >"$drm/card0/device/mem_info_vis_vram_total"
mk_card card1 nvidia 2
mkdir -p "$drm/card0-eDP-1" && printf 'connected\n' >"$drm/card0-eDP-1/status"
mkdir -p "$drm/card2/device"
printf 'DRIVER=appletbdrm\nPCI_SLOT_NAME=0000:03:00.0\n' >"$drm/card2/device/uevent"

# fake /dev/dri with the boot-stable symlinks, so value assembly is
# deterministic and never reads the host's real /dev/dri.
dri="$tmp/dri"; mkdir -p "$dri"
: >"$dri/card0"; : >"$dri/card1"
ln -s card0 "$dri/ryoku-gpu-0000-01-00-0"
ln -s card1 "$dri/ryoku-gpu-0000-02-00-0"

# Stub a hybrid MUX so unsafe Performance must refuse without changing hardware.
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

# --- 0. KMS-only display devices never enter the GPU inventory
detect_json="$(run detect --json)"
[[ $detect_json != *appletbdrm* && $detect_json != *card2* ]] \
  || fail "KMS-only appletbdrm was ranked as a render GPU: $detect_json"

# --- 1. risky laptop default: leave selection to the compositor
rm -f "$conf"
[[ "$(verdict)" == "ok" ]] || fail "panel-crossing laptop must be safe while unpinned, got $(verdict)"
policy="$(run status --json)"
[[ $policy == '{"performance_available":false,"performance_reason":"Switch the laptop'\''s graphics MUX to discrete first."}' ]] \
  || fail "unsafe topology reported the wrong status JSON: $policy"
run persist 2>"$tmp/err" >/dev/null
grep -Eq '^hl\.env\("AQ_DRM_DEVICES"' "$conf" 2>/dev/null \
  && fail "default persist pinned a dGPU that does not own the internal panel"
grep -qi 'internal panel' "$tmp/err" || fail "safe default did not explain the panel topology"

# --- 2. a legacy strongest-first pin is unsafe-pin and can be cleared
RYOKU_GPU_FORCE=1 run persist >/dev/null
sed -i '/ryoku-gpu-forced/d' "$conf"
v="$(verdict)"
[[ "$v" == "unsafe-pin 0000:02:00.0" ]] || fail "legacy dGPU-first pin must be unsafe-pin, got $v"
run disable >/dev/null
[[ "$(verdict)" == "ok" ]] || fail "cleared unsafe pin must be ok, got $(verdict)"

# --- 3. Performance refuses reverse PRIME without changing the stored choice
run mode hybrid >/dev/null
before="$(<"$conf")"
if run mode performance 2>"$tmp/err" >/dev/null; then
  fail "Performance succeeded while the panel was on the iGPU"
fi
[[ "$(<"$conf")" == "$before" ]] || fail "rejected Performance changed gpu.lua"
grep -q 'ryoku-gpu-mode: hybrid' "$conf" || fail "rejected Performance dropped hybrid"
grep -Eq '^hl\.env\("AQ_DRM_DEVICES"' "$conf" && fail "rejected Performance left a pin"
grep -qi 'mux' "$tmp/err" || fail "rejected Performance did not explain the MUX requirement"

# --- 4. Performance works once the panel is routed to the dGPU
printf 'disconnected\n' >"$drm/card0-eDP-1/status"
mkdir -p "$drm/card1-eDP-1"
printf 'connected\n' >"$drm/card1-eDP-1/status"
run mode performance >/dev/null
policy="$(run status --json)"
[[ $policy == '{"performance_available":true,"performance_reason":""}' ]] \
  || fail "panel-owning dGPU was not reported as Performance-capable: $policy"
grep -Eq '^hl\.env\("AQ_DRM_DEVICES"' "$conf" || fail "safe Performance wrote no pin"
grep -q 'ryoku-gpu-mode: performance' "$conf" || fail "Performance did not stamp the mode"
grep -q 'ryoku-gpu-primary: 0000:02:00.0' "$conf" || fail "Performance primary is not the dGPU"
[[ "$(verdict)" == "ok" ]] || fail "panel-owning Performance pin must be ok, got $(verdict)"

# Restore the common hybrid wiring for the remaining cases.
printf 'connected\n' >"$drm/card0-eDP-1/status"
printf 'disconnected\n' >"$drm/card1-eDP-1/status"
run disable >/dev/null

# --- 5. desktops keep pinning even when connectors span GPUs
rm -f "$conf"
LAPTOP=0 run persist >/dev/null
grep -Eq '^hl\.env\("AQ_DRM_DEVICES"' "$conf" || fail "desktop persist wrote no pin"
[[ "$(LAPTOP=0 run check-pin)" == "ok" ]] || fail "desktop pin broke"

# --- 6. Hybrid unpins, stamps the choice, and login persist honours it
run mode hybrid >/dev/null
grep -q 'ryoku-gpu-mode: hybrid' "$conf" || fail "hybrid did not stamp the mode"
grep -Eq '^hl\.env\("AQ_DRM_DEVICES"' "$conf" && fail "hybrid left a pin behind"
run persist >/dev/null
grep -Eq '^hl\.env\("AQ_DRM_DEVICES"' "$conf" && fail "persist overwrote explicit hybrid"

# --- 7. a desktop pin that contradicts the stored mode is stale-pin
LAPTOP=0 run mode performance >/dev/null
sed -i '/ryoku-gpu-mode: performance/d' "$conf"
printf -- '-- ryoku-gpu-mode: hybrid\n' >>"$conf"
v="$(LAPTOP=0 run check-pin)"
[[ "$v" == "stale-pin 0000:02:00.0" ]] || fail "pin against stored hybrid must be stale-pin, got $v"
run disable >/dev/null
grep -q 'ryoku-gpu-mode: hybrid' "$conf" || fail "bare disable dropped the stored mode"

# --- 8. passthrough: solo iGPU pin, stamped, and its own verdict is ok
run mode passthrough >/dev/null
grep -q 'ryoku-gpu-mode: passthrough' "$conf" || fail "passthrough did not stamp"
grep -Eq '^hl\.env\("AQ_DRM_DEVICES"' "$conf" || fail "passthrough wrote no solo pin"
grep -q 'ryoku-gpu-0000-01-00-0' "$conf" || fail "passthrough pin is not the iGPU"
grep -q 'ryoku-gpu-0000-02-00-0' "$conf" && fail "passthrough kept the dGPU in the list"
[[ "$(verdict)" == "ok" ]] || fail "passthrough pin must be ok, got $(verdict)"

# --- 9. a forced pin is kept despite the unsafe panel route
run mode hybrid >/dev/null
RYOKU_GPU_FORCE=1 run persist >/dev/null
grep -q 'ryoku-gpu-forced' "$conf" || fail "force wrote no marker"
[[ "$(verdict)" == "forced" ]] || fail "forced pin must report forced, got $(verdict)"

# --- 10. single-GPU box: nothing to pin, nothing to repair
drm1="$tmp/drm1"; dri1="$tmp/dri1"
mkdir -p "$drm1/card0/device/drm/renderD128" "$drm1/card0-eDP-1" "$dri1"
printf 'DRIVER=amdgpu\nPCI_SLOT_NAME=0000:01:00.0\n' >"$drm1/card0/device/uevent"
printf '2000000000\n' >"$drm1/card0/device/mem_info_vram_total"
printf '2000000000\n' >"$drm1/card0/device/mem_info_vis_vram_total"
printf 'connected\n' >"$drm1/card0-eDP-1/status"
[[ "$(RYOKU_GPU_DRM_ROOT="$drm1" RYOKU_GPU_DRI_DIR="$dri1" RYOKU_GPU_CONF="$tmp/conf-solo" \
     RYOKU_GPU_NIRI_CONF="$niri" RYOKU_ASSUME_LAPTOP=1 "$gpu" check-pin)" == "ok" ]] \
  || fail "single-GPU box must be ok"

# --- 11. an eGPU does not override an iGPU-owned laptop panel by default
drme="$tmp/drme"
mkdir -p "$drme/card0/device/drm/renderD128" "$drme/card9/device/drm/renderD137" "$drme/card0-eDP-1"
printf 'DRIVER=amdgpu\nPCI_SLOT_NAME=0000:01:00.0\n' >"$drme/card0/device/uevent"
printf '2000000000\n' >"$drme/card0/device/mem_info_vram_total"
printf '2000000000\n' >"$drme/card0/device/mem_info_vis_vram_total"
printf 'connected\n' >"$drme/card0-eDP-1/status"
printf 'DRIVER=nvidia\nPCI_SLOT_NAME=0000:06:00.0\n' >"$drme/card9/device/uevent"
echo removable >"$drme/card9/device/removable"
ln -s card9 "$dri/ryoku-gpu-0000-06-00-0"
rm -f "$tmp/conf-egpu"
egpu() { RYOKU_GPU_DRM_ROOT="$drme" RYOKU_GPU_DRI_DIR="$dri" RYOKU_GPU_CONF="$tmp/conf-egpu" \
         RYOKU_GPU_NIRI_CONF="$niri" RYOKU_ASSUME_LAPTOP=1 "$gpu" "$@"; }
[[ "$(egpu check-pin)" == "ok" ]] || fail "unpinned eGPU laptop must be safe"
egpu persist >/dev/null
grep -Eq '^hl\.env\("AQ_DRM_DEVICES"' "$tmp/conf-egpu" 2>/dev/null \
  && fail "eGPU persist crossed the internal panel"

# --- 12. order --effective still sees an explicit forced pin even when the
# policy order refuses it; the panel diagnostic must be able to inspect that
# deliberate override.
run mode hybrid >/dev/null
RYOKU_GPU_FORCE=1 run persist >/dev/null   # forced pin, hybrid stamp kept
grep -Eq '^hl\.env\("AQ_DRM_DEVICES"' "$conf" || fail "force wrote no pin under hybrid"
if run order >/dev/null 2>&1; then
  fail "plain order must keep refusing under a stored hybrid choice"
fi
v="$(run order --effective)" \
  && [[ "$v" == *"ryoku-gpu-0000-02-00-0:"*"ryoku-gpu-0000-01-00-0" ]] \
  || fail "order --effective must print the pin file's value, got ${v:-<none>}"
run disable >/dev/null
if run order --effective >/dev/null 2>&1; then
  fail "order --effective must exit 1 on an unpinned machine"
fi


echo "gpu-pin-policy: all cases passed"
