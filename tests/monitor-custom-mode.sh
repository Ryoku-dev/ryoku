#!/usr/bin/env bash
# Fixture test for ryoku-monitor's custom-resolution forcing: a WxH@rate mode the
# panel does NOT advertise must be rewritten to a CVT modeline (so Hyprland forces
# it instead of ignoring it), while an advertised mode passes through unchanged.
# Runs in fixture mode (RYOKU_MONITOR_JSON), with a stub cvt (RYOKU_MONITOR_CVT),
# so no live compositor and no libxcvt are needed.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
mon="$here/../ryoku/hyprland/scripts/ryoku-monitor"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
# Never read the clamshell state of the machine that runs the test.
export RYOKU_CLAMSHELL_LID_EVENT_STATE="$tmp/clamshell.lid"
export RYOKU_CLAMSHELL_PANEL_STATE="$tmp/clamshell.panels"

conf="$tmp/monitors.lua"
fail() { echo "FAIL: $1" >&2; exit 1; }
has() { grep -qF -- "$2" "$1" || fail "$3"; }
hasnt() { grep -qF -- "$2" "$1" && fail "$3" || true; }

# Stub cvt: called as `cvt -r W H rate`; emit a real cvt-format Modeline line so
# the parser (fields 3..NF) is exercised. W is $2, H is $3.
cat >"$tmp/cvt" <<'CVT'
#!/usr/bin/env bash
w="$2"; h="$3"
printf 'Modeline "%sx%sR"  241.50  %s 2608 2640 2720  %s 1443 1448 1481 +hsync -vsync\n' "$w" "$h" "$w" "$h"
CVT
chmod +x "$tmp/cvt"

# eDP-1 advertises only 2560x1440@60; DP-1 advertises only 1920x1080@60.
cat >"$tmp/live.json" <<'JSON'
[
  {"name":"eDP-1","make":"A","model":"P","serial":"1","width":2560,"height":1440,"refreshRate":60.0,"x":0,"y":0,"scale":1.0,"transform":0,"vrr":false,"disabled":false,"focused":true,"mirrorOf":"none","availableModes":["2560x1440@60.00Hz"]},
  {"name":"DP-1","make":"B","model":"Q","serial":"2","width":1920,"height":1080,"refreshRate":60.0,"x":2560,"y":0,"scale":1.0,"transform":0,"vrr":false,"disabled":false,"focused":false,"mirrorOf":"none","availableModes":["1920x1080@60.00Hz"]}
]
JSON

# eDP-1 asks for a NON-advertised 2560x1440@120 (custom); DP-1 keeps an advertised mode.
layout='[
  {"id":"A|P|1","output":"eDP-1","mode":"2560x1440@120","position":"0x0","scale":1,"transform":0,"vrr":0,"mirror":"none","disabled":false},
  {"id":"B|Q|2","output":"DP-1","mode":"1920x1080@60","position":"2560x0","scale":1,"transform":0,"vrr":0,"mirror":"none","disabled":false}
]'

RYOKU_MONITOR_JSON="$tmp/live.json" RYOKU_MONITOR_CVT="$tmp/cvt" \
  RYOKU_MONITORS_CONF="$conf" RYOKU_MONITORS_DIR="$tmp/profiles" \
  RYOKU_MONITORS_APPLIED="$tmp/applied.json" "$mon" apply "$layout" >/dev/null

# The custom 120 Hz mode must become a modeline (forced), not the bare WxH@rate.
has "$conf"   'output = "eDP-1", mode = "modeline 241.50 2560 2608 2640 2720 1440 1443 1448 1481 +hsync -vsync"' \
  "custom non-advertised 2560x1440@120 was not rewritten to a CVT modeline"
hasnt "$conf" 'mode = "2560x1440@120"' \
  "the non-advertised WxH@rate mode was written verbatim (Hyprland would ignore it)"

# The advertised mode must pass through untouched (no modeline for DP-1).
has "$conf"   'output = "DP-1", mode = "1920x1080@60"' \
  "advertised 1920x1080@60 should pass through unchanged"
grep 'output = "DP-1"' "$conf" | grep -q 'modeline' && \
  fail "advertised mode was needlessly converted to a modeline" || true

# The applied layout persists the forced modeline so it returns at next login.
jq -e '.monitors[] | select(.output=="eDP-1") | .mode | startswith("modeline ")' \
  "$tmp/applied.json" >/dev/null || fail "applied layout did not persist the modeline"

echo "monitor-custom-mode: all checks passed"
