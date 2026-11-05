#!/usr/bin/env bash
# Fixture test for ryoku-monitor's display persistence across a reboot and a
# power-cycled TV (issue #152). A layout saved from the Displays page must come
# back at the next login even when the external display is absent, reports a
# different serial, or returns on a different connector -- the reported case is
# an LG TV over HDMI that reverts on every reboot. Also guards the two matching
# hazards: an EDID-less display must key on its connector (never the bare "||"),
# and booting the same disk on another machine must not drape one panel's
# settings over a different panel on the same port. Runs in fixture mode
# (RYOKU_MONITOR_JSON), so no live compositor is needed.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
mon="$here/../ryoku/hyprland/scripts/ryoku-monitor"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

conf="$tmp/monitors.lua"
export RYOKU_MONITORS_CONF="$conf"
export RYOKU_MONITORS_DIR="$tmp/profiles"
export RYOKU_MONITORS_APPLIED="$tmp/applied.json"
export RYOKU_MONITORS_USER="$tmp/none-user.lua"
export RYOKU_MONITOR_VM=0
# Never read the clamshell state of the machine that runs the test.
export RYOKU_CLAMSHELL_LID_EVENT_STATE="$tmp/clamshell.lid"
export RYOKU_CLAMSHELL_PANEL_STATE="$tmp/clamshell.panels"
export RYOKU_LID_STATE_GLOB="$tmp/lid-switch"
export RYOKU_MONITOR_TOGGLE_STATE="$tmp/toggle.json"

fail() { echo "FAIL: $1" >&2; exit 1; }
has() { grep -qF -- "$2" "$1" || fail "$3"; }
hasnt() { grep -qF -- "$2" "$1" && fail "$3" || true; }

# Laptop panel + LG TV over HDMI, both live (the state when the user opens the
# Displays page and applies a layout). The TV reports an empty serial, as many do.
cat >"$tmp/both.json" <<'JSON'
[
  {"name":"eDP-1","make":"Acme","model":"Panel","serial":"LP01","width":2560,"height":1600,"refreshRate":60.0,"x":0,"y":0,"scale":1.6,"transform":0,"vrr":false,"disabled":false,"focused":true,"mirrorOf":"none","physicalWidth":300,"physicalHeight":188,"availableModes":["2560x1600@60.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0},
  {"name":"HDMI-A-1","make":"LG Electronics","model":"LG TV SSCR2","serial":"","width":1920,"height":1080,"refreshRate":60.0,"x":2560,"y":0,"scale":1.0,"transform":0,"vrr":false,"disabled":false,"focused":false,"mirrorOf":"none","physicalWidth":1600,"physicalHeight":900,"availableModes":["3840x2160@60.00Hz","1920x1080@60.00Hz","1920x1080@120.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0}
]
JSON

# Reboot: TV asleep / not yet probed at login -> only the panel is live.
cat >"$tmp/absent.json" <<'JSON'
[
  {"name":"eDP-1","make":"Acme","model":"Panel","serial":"LP01","width":2560,"height":1600,"refreshRate":60.0,"x":0,"y":0,"scale":1.6,"transform":0,"vrr":false,"disabled":false,"focused":true,"mirrorOf":"none","physicalWidth":300,"physicalHeight":188,"availableModes":["2560x1600@60.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0}
]
JSON

# Reboot: TV present but reporting a different serial (LG TVs vary or blank it).
cat >"$tmp/serial.json" <<'JSON'
[
  {"name":"eDP-1","make":"Acme","model":"Panel","serial":"LP01","width":2560,"height":1600,"refreshRate":60.0,"x":0,"y":0,"scale":1.6,"transform":0,"vrr":false,"disabled":false,"focused":true,"mirrorOf":"none","physicalWidth":300,"physicalHeight":188,"availableModes":["2560x1600@60.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0},
  {"name":"HDMI-A-1","make":"LG Electronics","model":"LG TV SSCR2","serial":"0x01010101","width":1920,"height":1080,"refreshRate":60.0,"x":2560,"y":0,"scale":1.0,"transform":0,"vrr":false,"disabled":false,"focused":false,"mirrorOf":"none","physicalWidth":1600,"physicalHeight":900,"availableModes":["3840x2160@60.00Hz","1920x1080@60.00Hz","1920x1080@120.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0}
]
JSON

# Reboot: TV back on a different connector, same identity.
cat >"$tmp/renamed.json" <<'JSON'
[
  {"name":"eDP-1","make":"Acme","model":"Panel","serial":"LP01","width":2560,"height":1600,"refreshRate":60.0,"x":0,"y":0,"scale":1.6,"transform":0,"vrr":false,"disabled":false,"focused":true,"mirrorOf":"none","physicalWidth":300,"physicalHeight":188,"availableModes":["2560x1600@60.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0},
  {"name":"HDMI-A-2","make":"LG Electronics","model":"LG TV SSCR2","serial":"","width":1920,"height":1080,"refreshRate":60.0,"x":2560,"y":0,"scale":1.0,"transform":0,"vrr":false,"disabled":false,"focused":false,"mirrorOf":"none","physicalWidth":1600,"physicalHeight":900,"availableModes":["3840x2160@60.00Hz","1920x1080@60.00Hz","1920x1080@120.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0}
]
JSON

# The Displays-page layout: TV kept at 1080p to the right of the 1.6x panel.
layout='[
  {"id":"Acme|Panel|LP01","output":"eDP-1","mode":"2560x1600@60","position":"0x0","scale":1.6,"transform":0,"vrr":0,"mirror":"none","disabled":false},
  {"id":"LG Electronics|LG TV SSCR2|","output":"HDMI-A-1","mode":"1920x1080@60","position":"2560x0","scale":1,"transform":0,"vrr":0,"mirror":"none","disabled":false}
]'

apply_both() { RYOKU_MONITOR_JSON="$tmp/both.json" "$mon" apply "$layout" >/dev/null; }
login() { RYOKU_MONITOR_JSON="$tmp/$1" "$mon" autoscale >/dev/null 2>&1; }

# --- missing output: an absent TV keeps its saved stanza, and the panel keeps
# its own applied settings (the "HDMI reverts, laptop is fine" report). ---------
apply_both
login absent.json
has "$conf" 'output = "HDMI-A-1", mode = "1920x1080@60"' \
  "an absent TV's saved stanza was dropped at login (reverts like freshly connected)"
has "$conf" 'position = "2560x0"' "the absent TV lost its saved position"
has "$conf" 'output = "eDP-1", mode = "2560x1600@60", position = "0x0", scale = 1.6' \
  "the panel lost its own applied settings"
grep 'output = "eDP-1"' "$conf" | grep -qF 'supports_hdr = -1' \
  || fail "login trusted HDR support on an output without HDR enabled"

# --- identity: a drifting serial still resolves to the same TV by make|model ---
apply_both
login serial.json
has "$conf" 'output = "HDMI-A-1", mode = "1920x1080@60", position = "2560x0"' \
  "a serial-drifted TV was treated as a new display and reset from its saved layout"
grep -qF 'output = "HDMI-A-1", mode = "highrr"' "$conf" \
  && fail "the TV was reset to the DPI/highrr fallback instead of its saved mode" || true

# --- identity: a connector rename remaps the saved stanza to the new connector -
apply_both
login renamed.json
has "$conf" 'output = "HDMI-A-2", mode = "1920x1080@60", position = "2560x0"' \
  "a renamed connector was not remapped to the saved TV settings"
hasnt "$conf" 'output = "HDMI-A-1"' "the stale connector name survived a rename"

# --- baseline: exact identity round-trips unchanged ---------------------------
apply_both
login both.json
has "$conf" 'output = "HDMI-A-1", mode = "1920x1080@60", position = "2560x0"' \
  "an exact-identity login did not recall the saved TV settings"

# --- connector fallback: a display with no EDID keys on its connector, so a
# match key is never the bare "||" that every EDID-less output would collide on,
# and its applied layout is recalled. ------------------------------------------
cat >"$tmp/noedid.json" <<'JSON'
[
  {"name":"HDMI-A-1","make":"","model":"","serial":"","width":1920,"height":1080,"refreshRate":60.0,"x":0,"y":0,"scale":1.0,"transform":0,"vrr":false,"disabled":false,"focused":true,"mirrorOf":"none","physicalWidth":0,"physicalHeight":0,"availableModes":["1920x1080@60.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0}
]
JSON
idval="$(RYOKU_MONITOR_JSON="$tmp/noedid.json" "$mon" list | jq -r '.[0].id')"
[[ $idval == "HDMI-A-1" ]] || fail "an EDID-less display keyed on '$idval', not its connector"
noedid_layout='[{"id":"HDMI-A-1","output":"HDMI-A-1","mode":"1920x1080@60","position":"0x0","scale":1,"transform":0,"vrr":0,"mirror":"none","disabled":false}]'
RYOKU_MONITORS_APPLIED="$tmp/noedid-applied.json" RYOKU_MONITOR_JSON="$tmp/noedid.json" "$mon" apply "$noedid_layout" >/dev/null
RYOKU_MONITORS_APPLIED="$tmp/noedid-applied.json" RYOKU_MONITOR_JSON="$tmp/noedid.json" "$mon" autoscale >/dev/null 2>&1
has "$conf" 'output = "HDMI-A-1", mode = "1920x1080@60"' \
  "an EDID-less display did not recall its connector-keyed layout"

# --- another machine, same disk: a different panel on eDP-1 must NOT inherit the
# saved layout; DPI/live scaling wins, and the TV stanza must not leak onto it. -
cat >"$tmp/other.json" <<'JSON'
[
  {"name":"eDP-1","make":"Other","model":"Slab","serial":"S9","width":1920,"height":1080,"refreshRate":60.0,"x":0,"y":0,"scale":1.0,"transform":0,"vrr":false,"disabled":false,"focused":true,"mirrorOf":"none","physicalWidth":510,"physicalHeight":287,"availableModes":["1920x1080@60.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0}
]
JSON
apply_both
login other.json
has "$conf" 'output = "eDP-1"' "the live panel got no stanza on an unrelated machine"
hasnt "$conf" 'position = "2560x0"' "the saved TV stanza leaked onto an unrelated machine"
hasnt "$conf" 'output = "eDP-1", mode = "2560x1600@60"' \
  "a different panel on eDP-1 wrongly inherited the saved panel's mode"

# --- a phantom turned off stays off (support report: an EDID-less HDMI-A-1 at
# 1024x768 came back on every login). Hyprland 0.56 keeps a disabled output's
# last mode, so its size no longer says it is off; the flag and the missing
# workspace do. The Hub's list must show it off, and a login with no saved
# layout must persist it off instead of writing it back enabled. -------------
cat >"$tmp/phantom-off.json" <<'JSON'
[
  {"name":"DP-1","make":"Acme","model":"Desk","serial":"D1","width":1920,"height":1080,"refreshRate":60.0,"x":0,"y":0,"scale":1.0,"transform":0,"vrr":false,"disabled":false,"focused":true,"activeWorkspace":{"id":1,"name":"1"},"mirrorOf":"none","physicalWidth":530,"physicalHeight":300,"availableModes":["1920x1080@60.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0},
  {"name":"HDMI-A-1","make":"","model":"","serial":"","width":1024,"height":768,"refreshRate":60.0,"x":1920,"y":0,"scale":1.0,"transform":0,"vrr":false,"disabled":true,"focused":false,"activeWorkspace":{"id":0,"name":""},"mirrorOf":"none","physicalWidth":0,"physicalHeight":0,"availableModes":["1024x768@60.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0}
]
JSON
offval="$(RYOKU_MONITOR_JSON="$tmp/phantom-off.json" "$mon" list | jq -r '.[] | select(.name == "HDMI-A-1") | .disabled')"
[[ $offval == "true" ]] || fail "a disabled output that keeps its mode listed as enabled ($offval)"
RYOKU_MONITORS_APPLIED="$tmp/phantom-none.json" RYOKU_MONITOR_JSON="$tmp/phantom-off.json" "$mon" autoscale >/dev/null 2>&1
has "$conf" 'hl.monitor({ output = "HDMI-A-1", disabled = true })' \
  "a login persisted the disabled phantom as enabled"
hasnt "$conf" 'output = "HDMI-A-1", mode' "a login gave the disabled phantom a mode"

# --- the reverse: a plainly active output Hyprland mis-flags as disabled (an
# old git build) still holds a workspace and must stay enabled. ----------------
cat >"$tmp/misflagged.json" <<'JSON'
[
  {"name":"eDP-1","make":"Acme","model":"Panel","serial":"LP01","width":2560,"height":1600,"refreshRate":60.0,"x":0,"y":0,"scale":1.6,"transform":0,"vrr":false,"disabled":true,"focused":true,"activeWorkspace":{"id":1,"name":"1"},"mirrorOf":"none","physicalWidth":300,"physicalHeight":188,"availableModes":["2560x1600@60.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0}
]
JSON
flagval="$(RYOKU_MONITOR_JSON="$tmp/misflagged.json" "$mon" list | jq -r '.[0].disabled')"
[[ $flagval == "false" ]] || fail "an active output flagged disabled by Hyprland listed as off"

# --- docked lid close: the clamshell turns the panel off, and the hotplug
# autoscale that follows must not recall the saved layout over it (the panel
# came back on under the shut lid and flapped). The saved preference stays
# enabled, and the panel returns once the lid opens. -------------------------
cat >"$tmp/lid-closed.json" <<'JSON'
[
  {"name":"eDP-1","make":"Acme","model":"Panel","serial":"LP01","width":2560,"height":1600,"refreshRate":60.0,"x":0,"y":0,"scale":1.6,"transform":0,"vrr":false,"disabled":true,"focused":false,"activeWorkspace":{"id":0,"name":""},"mirrorOf":"none","physicalWidth":300,"physicalHeight":188,"availableModes":["2560x1600@60.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0},
  {"name":"HDMI-A-1","make":"LG Electronics","model":"LG TV SSCR2","serial":"","width":1920,"height":1080,"refreshRate":60.0,"x":2560,"y":0,"scale":1.0,"transform":0,"vrr":false,"disabled":false,"focused":true,"activeWorkspace":{"id":1,"name":"1"},"mirrorOf":"none","physicalWidth":1600,"physicalHeight":900,"availableModes":["3840x2160@60.00Hz","1920x1080@60.00Hz","1920x1080@120.00Hz"],"colorManagementPreset":"srgb","sdrBrightness":1.0}
]
JSON
apply_both
printf 'closed\n' >"$RYOKU_CLAMSHELL_LID_EVENT_STATE"
printf 'eDP-1\n' >"$RYOKU_CLAMSHELL_PANEL_STATE"
login lid-closed.json
has "$conf" 'hl.monitor({ output = "eDP-1", disabled = true })' \
  "a hotplug autoscale turned the clamshell's covered panel back on"
has "$conf" 'output = "HDMI-A-1", mode = "1920x1080@60"' \
  "the external display lost its layout while the lid was shut"
RYOKU_MONITOR_JSON="$tmp/lid-closed.json" "$mon" apply "$layout" >/dev/null
has "$conf" 'hl.monitor({ output = "eDP-1", disabled = true })' \
  "an Apply turned the covered panel on under the shut lid"
[[ $(jq -r '.monitors[] | select(.output == "eDP-1") | .disabled' "$RYOKU_MONITORS_APPLIED") == false ]] ||
  fail "a shut lid overwrote the saved panel preference"
printf 'open\n' >"$RYOKU_CLAMSHELL_LID_EVENT_STATE"
login lid-closed.json
has "$conf" 'output = "eDP-1", mode = "2560x1600@60", position = "0x0", scale = 1.6' \
  "the panel did not return to its saved layout when the lid opened"
# A clamshell restart clears the edge file while the lid stays shut. The
# firmware switch then keeps the panel off, and an open switch lets it return.
rm -f "$RYOKU_CLAMSHELL_LID_EVENT_STATE"
printf 'state:      closed\n' >"$RYOKU_LID_STATE_GLOB"
login lid-closed.json
has "$conf" 'hl.monitor({ output = "eDP-1", disabled = true })' \
  "a clamshell restart let a hotplug turn the covered panel on"
printf 'state:      open\n' >"$RYOKU_LID_STATE_GLOB"
login lid-closed.json
has "$conf" 'output = "eDP-1", mode = "2560x1600@60", position = "0x0", scale = 1.6' \
  "an open lid switch did not let the panel return"
# Systems without a firmware lid file use the clamshell owner's UPower-backed
# physical state when its edge file is absent.
rm -f "$RYOKU_CLAMSHELL_LID_EVENT_STATE" "$RYOKU_LID_STATE_GLOB"
cat >"$tmp/clamshell-state" <<'SH'
#!/usr/bin/env bash
[[ ${1:-} == status ]] && printf 'lid=closed\n'
SH
chmod +x "$tmp/clamshell-state"
export RYOKU_CLAMSHELL_BIN="$tmp/clamshell-state"
login lid-closed.json
has "$conf" 'hl.monitor({ output = "eDP-1", disabled = true })' \
  "a laptop without an ACPI lid file let the covered panel return"
unset RYOKU_CLAMSHELL_BIN
rm -f "$RYOKU_CLAMSHELL_LID_EVENT_STATE" "$RYOKU_CLAMSHELL_PANEL_STATE" "$RYOKU_LID_STATE_GLOB"

# A migrated monitors_user.lua pin stays out of the generated file. Closing the
# lid changes only that pinned panel, so autoscale must not rewrite monitors.lua
# and reload the pin over the live clamshell handoff.
rm -f "$RYOKU_MONITORS_APPLIED"
cat >"$RYOKU_MONITORS_USER" <<'LUA'
-- migrated by ryoku-shell-install
hl.monitor({ output = "eDP-1", mode = "highrr", position = "auto", scale = 1.25 })
LUA
login both.json
hasnt "$conf" 'output = "eDP-1"' "autoscale copied a pinned panel into monitors.lua"
before="$(stat -c '%i %Y.%y' "$conf")"
printf 'closed\n' >"$RYOKU_CLAMSHELL_LID_EVENT_STATE"
printf 'eDP-1\n' >"$RYOKU_CLAMSHELL_PANEL_STATE"
login lid-closed.json
[[ $(stat -c '%i %Y.%y' "$conf") == "$before" ]] ||
  fail "a covered pinned panel rewrote monitors.lua and reloaded its pin"
rm -f "$RYOKU_MONITORS_USER" "$RYOKU_CLAMSHELL_LID_EVENT_STATE" "$RYOKU_CLAMSHELL_PANEL_STATE"
apply_both

# --- an unchanged layout leaves monitors.lua untouched. Hyprland reloads its
# config on every write, even an identical one, and the reload hooks start the
# next autoscale, so a rewrite of the same layout kept a reload loop alive. ---
login both.json
before="$(stat -c '%i %Y.%y' "$conf")"
login both.json
[[ $(stat -c '%i %Y.%y' "$conf") == "$before" ]] ||
  fail "an autoscale with no layout change rewrote monitors.lua"
RYOKU_MONITOR_JSON="$tmp/both.json" "$mon" apply "$layout" >/dev/null
[[ $(stat -c '%i %Y.%y' "$conf") == "$before" ]] ||
  fail "an Apply of the same layout rewrote monitors.lua"

# --- Super+P is an undoable session-only mirror. The first toggle snapshots
# the exact applied layout without rewriting monitors.lua. The second restores
# the saved modes, positions and scales, then consumes the snapshot. ------------
apply_both
before_sum="$(sha256sum "$conf" | cut -d' ' -f1)"
RYOKU_MONITOR_JSON="$tmp/both.json" "$mon" toggle >/dev/null
[[ -s $RYOKU_MONITOR_TOGGLE_STATE ]] || fail "the mirror toggle did not snapshot the applied layout"
[[ $(sha256sum "$conf" | cut -d' ' -f1) == "$before_sum" ]] ||
  fail "the mirror toggle persisted its temporary layout"
hasnt "$conf" 'mirror =' "the temporary mirror reached monitors.lua"

jq '.[1].mirrorOf = "eDP-1"' "$tmp/both.json" >"$tmp/mirrored.json"
RYOKU_MONITOR_JSON="$tmp/mirrored.json" "$mon" toggle >/dev/null
[[ ! -e $RYOKU_MONITOR_TOGGLE_STATE ]] || fail "restoring the pre-mirror layout left a stale snapshot"
has "$conf" 'output = "eDP-1", mode = "2560x1600@60", position = "0x0", scale = 1.6' \
  "the second toggle did not restore the panel layout"
has "$conf" 'output = "HDMI-A-1", mode = "1920x1080@60", position = "2560x0", scale = 1' \
  "the second toggle did not restore the TV layout"
hasnt "$conf" 'mirror =' "the restored layout stayed mirrored"

# Mirroring keeps each target's current below-native mode and fractional scale.
# Override only the live apply seam so the exact rule is visible in fixture mode.
jq '.[1].scale = 1.25' "$tmp/both.json" >"$tmp/mixed.json"
mirror_rule="$(RYOKU_MONITOR_JSON="$tmp/mixed.json" \
  RYOKU_MONITOR_TOGGLE_STATE="$tmp/direct-toggle.json" \
  bash -c 'source "$1"; apply_monitor() { printf "%s\n" "$1"; }; cmd_mirror eDP-1' _ "$mon")"
grep -qF 'output = "HDMI-A-1", mode = "1920x1080@60.0", position = "auto", scale = 1.25, mirror = "eDP-1"' \
  <<<"$mirror_rule" || fail "mirror replaced the target's current mode or scale: $mirror_rule"
grep -qF 'mode = "preferred"' <<<"$mirror_rule" &&
  fail "mirror raised a below-native target to its preferred mode"

# Atomic updates follow a user-owned symlink instead of replacing it, and keep
# the target's mode.
backing="$tmp/user-monitors.lua"
linked="$tmp/linked-monitors.lua"
cp "$conf" "$backing"
printf '%s\n' '-- stale' >>"$backing"
chmod 0600 "$backing"
ln -s "$(basename "$backing")" "$linked"
export RYOKU_MONITORS_CONF="$linked"
login both.json
[[ -L $linked ]] || fail "an atomic monitor write replaced a user-owned symlink"
[[ $(stat -c '%a' "$backing") == 600 ]] || fail "an atomic monitor write changed the target mode"
hasnt "$backing" '-- stale' "an atomic monitor write did not refresh the symlink target"

echo "monitor-persist: all checks passed"
