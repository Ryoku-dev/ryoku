#!/usr/bin/env bash
# hermetic test for ryoku-gpu-mux: the ASUS GPU MUX reporter/switcher. The two
# firmware knob roots (RYOKU_MUX_ARMOURY_ROOT, RYOKU_MUX_LEGACY_ROOT) and the DRM
# tree (RYOKU_GPU_DRM_ROOT, read by the sourced detector) point at fake sysfs in a
# tmp dir, and nvidia-smi is a sentinel stub on PATH, so no firmware is touched,
# no dGPU is woken, and no root is needed: the tmp knob files are writable, so
# `set` writes directly and never reaches sudo.
#
# Load-bearing case: when both interfaces are present the modern asus-armoury
# tree MUST win. The legacy asus-nb-wmi node was measured returning stale/wrong
# gpu_mux_mode values on real firmware, so trusting it would misreport the mode;
# precedence, not a coin flip, is what keeps the answer correct.
#
# Also load-bearing: `get`/`capable`/`set` must NEVER invoke nvidia-smi. Waking
# the dGPU to read power.draw is exactly the idle cost this tool exists to remove,
# so only `status` may pay it, and even then only against the real DRM tree.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
helper="$here/../system/hardware/gpu/ryoku-gpu-mux"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fail() { echo "FAIL: $1" >&2; exit 1; }

# nvidia-smi stub: drops a sentinel if ever called, so a spawn is provable.
bin="$tmp/bin"; mkdir -p "$bin"
sentinel="$tmp/nvidia-smi-called"
cat >"$bin/nvidia-smi" <<EOF
#!/usr/bin/env bash
echo called >"$sentinel"
exit 0
EOF
chmod +x "$bin/nvidia-smi"
export PATH="$bin:$PATH"

empty="$tmp/empty"; mkdir -p "$empty"   # a root with no knob under it

# armoury knob: <root>/asus-armoury/attributes/gpu_mux_mode/current_value, with an
# optional sibling pending_reboot attribute.
mk_armoury() { # <root> <value> [pending]
  local a="$1/asus-armoury/attributes"
  mkdir -p "$a/gpu_mux_mode"
  printf '%s\n' "$2" >"$a/gpu_mux_mode/current_value"
  if [[ -n ${3:-} ]]; then printf '%s\n' "$3" >"$a/pending_reboot"; fi
}
mk_legacy() { # <root> <value>: <root>/asus-nb-wmi/gpu_mux_mode
  mkdir -p "$1/asus-nb-wmi"
  printf '%s\n' "$2" >"$1/asus-nb-wmi/gpu_mux_mode"
}
armoury_value() { cat "$1/asus-armoury/attributes/gpu_mux_mode/current_value"; }

# fake DRM card whose device uevent names its kernel driver, and connectors.
mk_card() { # <drm-root> <cardN> <driver>
  local n="${2#card}" d="$1/$2/device"
  mkdir -p "$d/drm/renderD$((128 + n))"
  printf 'DRIVER=%s\nPCI_SLOT_NAME=0000:0%s:00.0\n' "$3" "$n" >"$d/uevent"
}
mk_conn() { # <drm-root> <cardN-CONNECTOR> <connected|disconnected>
  mkdir -p "$1/$2"; printf '%s\n' "$3" >"$1/$2/status"
}

# every invocation pins all three roots so a real ASUS box under test never
# leaks its own firmware-attributes / platform / drm trees into the result.
mux() { # <armoury-root> <legacy-root> <drm-root> [args...]
  local a="$1" l="$2" d="$3"; shift 3
  RYOKU_MUX_ARMOURY_ROOT="$a" RYOKU_MUX_LEGACY_ROOT="$l" RYOKU_GPU_DRM_ROOT="$d" \
    "$helper" "$@"
}

# --- armoury wins over legacy, and reports the armoury value ------------------
# armoury says hybrid (1), legacy says discrete (0, the stale reading). armoury
# must be chosen: iface=armoury and get=hybrid, never the legacy discrete.
both_a="$tmp/both-arm"; mk_armoury "$both_a" 1
both_l="$tmp/both-leg"; mk_legacy "$both_l" 0
json="$(mux "$both_a" "$both_l" "$empty" status --json)"
jq -e '.iface == "armoury"' <<<"$json" >/dev/null || fail "both interfaces: iface not armoury"
[[ "$(mux "$both_a" "$both_l" "$empty" get)" == hybrid ]] \
  || fail "both interfaces: get returned the legacy value, not armoury's"

# --- value mapping both ways, and never a raw integer -------------------------
disc="$tmp/arm-disc"; mk_armoury "$disc" 0
hyb="$tmp/arm-hyb"; mk_armoury "$hyb" 1
out="$(mux "$disc" "$empty" "$empty" get)"
[[ $out == discrete ]] || fail "armoury 0 did not map to discrete (got '$out')"
grep -qE '[0-9]' <<<"$out" && fail "get leaked a raw integer for discrete"
out="$(mux "$hyb" "$empty" "$empty" get)"
[[ $out == hybrid ]] || fail "armoury 1 did not map to hybrid (got '$out')"
grep -qE '[0-9]' <<<"$out" && fail "get leaked a raw integer for hybrid"

# --- legacy fallback when only the legacy knob exists -------------------------
leg="$tmp/leg-only"; mk_legacy "$leg" 1
json="$(mux "$empty" "$leg" "$empty" status --json)"
jq -e '.iface == "legacy"' <<<"$json" >/dev/null || fail "legacy-only: iface not legacy"
[[ "$(mux "$empty" "$leg" "$empty" get)" == hybrid ]] || fail "legacy-only: get wrong"

# --- no knob at all -----------------------------------------------------------
if mux "$empty" "$empty" "$empty" capable; then fail "capable exited 0 with no knob"; fi
json="$(mux "$empty" "$empty" "$empty" status --json)"
jq -e '.capable == false and .mode == "unknown" and .iface == "none" and .dgpu_watts == -1' \
  <<<"$json" >/dev/null || fail "no-knob status --json fields wrong"

# --- panel_on_dgpu from a two-card DRM tree -----------------------------------
# card0 amdgpu (integrated: no VRAM node -> classified integrated), card1 nvidia
# (discrete). Only the integrated card's connector is connected -> panel off dGPU.
drmI="$tmp/drm-igpu"; mk_card "$drmI" card0 amdgpu; mk_card "$drmI" card1 nvidia
mk_conn "$drmI" card0-eDP-1 connected; mk_conn "$drmI" card1-DP-1 disconnected
json="$(mux "$empty" "$empty" "$drmI" status --json)"
jq -e '.panel_on_dgpu == false' <<<"$json" >/dev/null || fail "panel on iGPU misreported as on dGPU"
# same two cards, but the connected connector is the nvidia card's -> on dGPU.
drmD="$tmp/drm-dgpu"; mk_card "$drmD" card0 amdgpu; mk_card "$drmD" card1 nvidia
mk_conn "$drmD" card0-eDP-1 disconnected; mk_conn "$drmD" card1-DP-1 connected
json="$(mux "$empty" "$empty" "$drmD" status --json)"
jq -e '.panel_on_dgpu == true' <<<"$json" >/dev/null || fail "panel on dGPU misreported as off"

# --- set: write to switch away, refuse a no-op, reject garbage ----------------
sarm="$tmp/set-arm"; mk_armoury "$sarm" 0   # a discrete fixture
out="$(mux "$sarm" "$empty" "$empty" set discrete)"   # already discrete: no-op, exit 0
[[ "$(armoury_value "$sarm")" == 0 ]] || fail "set discrete no-op still wrote the knob"
grep -qi 'already in discrete' <<<"$out" || fail "set discrete no-op gave no notice"
out="$(mux "$sarm" "$empty" "$empty" set hybrid)"     # a real switch
[[ "$(armoury_value "$sarm")" == 1 ]] || fail "set hybrid did not write 1"
grep -q 'REBOOT REQUIRED' <<<"$out" || fail "set hybrid did not warn a reboot is required"
if mux "$sarm" "$empty" "$empty" set sideways >/dev/null 2>&1; then
  fail "set accepted an invalid mode"
fi

# --- reboot_pending reflects a truthy sibling attribute -----------------------
# with no internal panel visible the booted wiring is unknown, so the firmware's
# own flag is the only answer.
rarm="$tmp/reboot-arm"; mk_armoury "$rarm" 0 1
json="$(mux "$rarm" "$empty" "$empty" status --json)"
jq -e '.reboot_pending == true and .booted_mode == "unknown"' <<<"$json" >/dev/null \
  || fail "reboot_pending not true"

# --- reboot_pending follows the booted wiring, not the sticky firmware flag ----
# The firmware keeps pending_reboot set after a switch and a switch back, so a
# change undone before restarting must stop asking for one. The built-in panel
# on the iGPU means the box booted hybrid.
parm="$tmp/pending-arm"; mk_armoury "$parm" 1 1
drmB="$tmp/drm-booted-hybrid"; mk_card "$drmB" card0 amdgpu; mk_card "$drmB" card1 nvidia
mk_conn "$drmB" card0-eDP-1 connected; mk_conn "$drmB" card1-eDP-2 disconnected
mk_conn "$drmB" card1-DP-1 connected
json="$(mux "$parm" "$empty" "$drmB" status --json)"
jq -e '.booted_mode == "hybrid" and .reboot_pending == false' <<<"$json" >/dev/null \
  || fail "switched back to the booted hybrid wiring still reports a reboot: $json"
printf '0\n' >"$parm/asus-armoury/attributes/gpu_mux_mode/current_value"
json="$(mux "$parm" "$empty" "$drmB" status --json)"
jq -e '.reboot_pending == true' <<<"$json" >/dev/null \
  || fail "a discrete choice on a hybrid boot does not report a reboot: $json"
drmV="$tmp/drm-booted-discrete"; mk_card "$drmV" card0 amdgpu; mk_card "$drmV" card1 nvidia
mk_conn "$drmV" card0-eDP-1 disconnected; mk_conn "$drmV" card1-eDP-2 connected
json="$(mux "$parm" "$empty" "$drmV" status --json)"
jq -e '.booted_mode == "discrete" and .reboot_pending == false' <<<"$json" >/dev/null \
  || fail "the booted discrete wiring misread: $json"

# --- status --json is valid JSON with exactly the eight contracted keys -------
json="$(mux "$disc" "$empty" "$empty" status --json)"
jq -e . <<<"$json" >/dev/null || fail "status --json is not valid JSON"
keys="$(jq -r 'keys_unsorted | join(",")' <<<"$json")"
[[ $keys == "capable,mode,iface,panel_on_dgpu,dgpu_slot,dgpu_watts,reboot_pending,booted_mode" ]] \
  || fail "status --json keys are not exactly the eight contracted ones: $keys"

# --- nvidia-smi is NEVER spawned for get, capable, or set ---------------------
for c in "get" "capable" "set hybrid"; do
  rm -f "$sentinel"
  # "set hybrid" is two arguments, so split the case into an argv array rather
  # than relying on an unquoted expansion.
  read -r -a argv <<<"$c"
  mux "$sarm" "$empty" "$empty" "${argv[@]}" >/dev/null 2>&1 || true
  [[ -e $sentinel ]] && fail "nvidia-smi was invoked for '$c' (dGPU woken)"
done

echo "gpu-mux: all checks passed"
