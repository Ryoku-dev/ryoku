#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# up.sh niri|hyprland [--app] [--surface] [--model [--decisions]] [--hands] [--outputs N]: start an invisible,
# isolated compositor lab.
#   niri:      headless weston (--idle-time=0, kiosk shell) hosting a nested niri.
#   hyprland:  headless tinywl on the integrated GPU's render node hosting a
#              nested Hyprland with a Lua config and headless outputs.
#   --app:     also start a private session bus, the real oni daemon and
#              hyakki-app, and wait for the dev.ryoku.Hyakki window.
#   --surface: also start the bus and daemon with Oni on, and oni-surface with
#              its lab hooks (ledger at $ONI_LAB/ledger/surface.jsonl); it is
#              started before the app, so the app's own launch is a no-op.
#   --outputs N: Hyprland only, N (1-4) headless outputs side by side.
#   --model:   with --app or --surface, start the lab's vision fixture
#              (model.py, loopback only) before the daemon and seed it as the
#              llama.cpp address; the Ollama address is pointed at a path the
#              fixture refuses, so no live local server is ever asked.
#   --decisions: with --model, also seed the typed decision route (the app's
#              alone) local at the fixture's /v1, model oni-fixture-decisions,
#              oni.reflex local (decidefix.py serves it inside model.py).
#   --hands:   also start Phase 14's web fixture (handsfix.py, loopback only:
#              origins A and B on two random 127.0.0.1 ports); its port file
#              is $ONI_LAB/hands/port, its request log log/hands-requests.jsonl,
#              and lab.env gets LAB_HANDS_A and LAB_HANDS_B (the two origins).
#              The Hands browser itself is only ever the daemon's child.
# Both compositors float the fixture windows (window.sh), Phase 12's guide
# window (dev.ryoku.OniGuide) and Phase 13's lab browser
# (dev.ryoku.OniBrowser) by window rule; Hyprland also floats Phase 13's notes
# window (dev.ryoku.OniNotes), which niri leaves tiled so its maximize (a
# column verb) shows in the geometry. Both float Phase 14's Hands browser
# (the daemon's Chromium: app id chromium, or its --class hyakki-hands-<hex>)
# beside the app: niri on the right half, Hyprland on the second output when
# there is one. Both float Phase 15's target and sentinel windows
# (dev.ryoku.OniTarget, dev.ryoku.OniSentinel): niri on the right half,
# Hyprland on the second output when there is one; Phase 15's
# dev.ryoku.OniTiled has no rule and stays tiled.
# Binaries come from HYAKKI_BIN, default HYAKKI_BUILD_DIR, default the
# module's build/native (release/build.sh's output).
# Everything lives under $ONI_LAB (default /dev/shm/oni-lab; ONI_LAB_ID=13 gives
# /dev/shm/oni-lab-13, a second lab beside the first); down.sh removes it.
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"
ROOT=$(CDPATH='' cd -- "$HERE/../../../../.." && pwd -P)

usage() {
    printf 'usage: %s niri|hyprland [--app] [--surface] [--model [--decisions]] [--hands] [--outputs N]\n' "${0##*/}" >&2
    exit 2
}
[ "$#" -ge 1 ] || usage
LAB_WM=$1
shift
case $LAB_WM in niri | hyprland) ;; *) usage ;; esac
APP=0 SURFACE=0 OUTPUTS=1 MODEL=0 DECISIONS=0 HANDS=0
while [ "$#" -gt 0 ]; do
    case $1 in
        --app) APP=1 ;;
        --surface) SURFACE=1 ;;
        --model) MODEL=1 ;;
        --decisions) DECISIONS=1 ;;
        --hands) HANDS=1 ;;
        --outputs)
            [ "$#" -ge 2 ] || usage
            OUTPUTS=$2
            shift
            ;;
        *) usage ;;
    esac
    shift
done
case $OUTPUTS in 1 | 2 | 3 | 4) ;; *) lab_die "--outputs takes 1 to 4" ;; esac
[ "$MODEL" = 0 ] || [ "$APP" = 1 ] || [ "$SURFACE" = 1 ] || lab_die "--model needs --app or --surface (a daemon to use it)"
[ "$DECISIONS" = 0 ] || [ "$MODEL" = 1 ] || lab_die "--decisions needs --model (the decision fixture is model.py's)"
[ "$OUTPUTS" = 1 ] || [ "$LAB_WM" = hyprland ] ||
    lab_die "nested niri has one winit output; use outputs.sh scale/off/on for display changes"
WIDTH=${ONI_LAB_WIDTH:-1920}
HEIGHT=${ONI_LAB_HEIGHT:-1080}
case $WIDTH:$HEIGHT in *[!0-9:]* | :* | *:) lab_die "ONI_LAB_WIDTH/ONI_LAB_HEIGHT must be numbers" ;; esac
HYAKKI_BIN=${HYAKKI_BIN:-${HYAKKI_BUILD_DIR:-$ROOT/ryoku/hyakki/build/native}}
case $HYAKKI_BIN in /*) ;; *) lab_die "HYAKKI_BIN must be absolute" ;; esac

for t in systemd-run choom setsid python3 grim cc; do
    command -v "$t" >/dev/null 2>&1 || lab_die "missing $t"
done
if [ "$LAB_WM" = niri ]; then
    for t in weston niri; do command -v "$t" >/dev/null 2>&1 || lab_die "missing $t"; done
else
    for t in Hyprland hyprctl; do command -v "$t" >/dev/null 2>&1 || lab_die "missing $t"; done
    TINYWL=${ONI_LAB_TINYWL:-$HOME/.cache/hyprnest/wlroots-build/tinywl/tinywl}
    [ -x "$TINYWL" ] || lab_die "no tinywl at $TINYWL (set ONI_LAB_TINYWL; see README.md)"
fi
if [ "$APP" = 1 ] || [ "$SURFACE" = 1 ]; then
    command -v dbus-daemon >/dev/null 2>&1 || lab_die "missing dbus-daemon"
    bins="oni hyakki"
    [ "$APP" = 1 ] && bins="$bins hyakki-app"
    [ "$SURFACE" = 1 ] && bins="$bins oni-surface"
    for b in $bins; do
        [ -x "$HYAKKI_BIN/$b" ] || lab_die "no $b in HYAKKI_BIN=$HYAKKI_BIN"
    done
fi
[ ! -e "$ONI_LAB" ] && [ ! -L "$ONI_LAB" ] || lab_die "$ONI_LAB already exists; run down.sh first"

umask 077
mkdir -p "$ONI_LAB"
for d in w rt home cfg/hyakki state cache data bin log ledger tmp wm/hypr; do mkdir -p "$ONI_LAB/$d"; done
# The scope tag: random, and led by ONI_LAB_ID for a non-default lab, so
# "systemctl --user list-units 'oni-lab-13-*'" lists exactly that lab's scopes.
LAB_ID=${ONI_LAB_ID:+$ONI_LAB_ID-}$(lab_rand)
: >"$LAB_ENV"
lab_env_put LAB_ID "$LAB_ID"
lab_env_put LAB_WM "$LAB_WM"
lab_env_put LAB_USERENV_MD5 "$(systemctl --user show-environment | md5sum | cut -d' ' -f1)"
lab_env_put HYAKKI_BIN "$HYAKKI_BIN"
lab_env_put LAB_WIDTH "$WIDTH"
lab_env_put LAB_HEIGHT "$HEIGHT"

on_exit() {
    st=$?
    trap - EXIT
    if [ "$st" != 0 ]; then
        printf 'up.sh: failed (%s); tearing the lab down, logs under %s are removed unless ONI_LAB_SAVE is set\n' "$st" "$LAB_LOG" >&2
        ONI_LAB=$ONI_LAB "$HERE/down.sh" >&2 || true
    fi
    exit "$st"
}
trap on_exit EXIT
trap 'exit 130' INT TERM HUP

# lab_wait TENTHS COMMAND...: poll COMMAND every 0.1 s.
lab_wait() {
    _n=$1
    shift
    while [ "$_n" -gt 0 ]; do
        if "$@"; then return 0; fi
        _n=$((_n - 1))
        sleep 0.1
    done
    return 1
}
first_socket() {
    for _s in "$@"; do
        [ -S "$_s" ] && { printf '%s\n' "$_s"; return 0; }
    done
    return 1
}

if [ "$LAB_WM" = niri ]; then
    lab_run weston 512M setsid -w env -i HOME="$ONI_LAB/home" PATH=/usr/bin:/bin XDG_RUNTIME_DIR="$LAB_WRT" \
        weston --backend=headless --renderer=gl --width="$WIDTH" --height="$HEIGHT" \
        --socket=wl-host --idle-time=0 --shell=kiosk-shell.so \
        >"$LAB_LOG/weston.log" 2>&1 </dev/null &
    lab_wait 100 test -S "$LAB_WRT/wl-host" || lab_die "weston did not start, see $LAB_LOG/weston.log"
    cat >"$ONI_LAB/wm/niri.kdl" <<'EOF'
input {
    keyboard {
        xkb {
            layout "us"
        }
    }
}
output "winit" {
    scale 1
}
layout {
    gaps 16
    center-focused-column "never"
    default-column-width { proportion 0.75; }
}
prefer-no-csd
hotkey-overlay {
    skip-at-startup
}
animations {
    off
}
xwayland-satellite {
    off
}
// Released niri reports screen positions for floating windows, not tiles.
window-rule {
    match app-id="^dev.ryoku.Hyakki$"
    open-floating true
    default-floating-position x=16 y=16 relative-to="top-left"
}
// window.sh's fixtures: notes and list float where the lab puts them;
// notes-tiled (dev.ryoku.OniFixtureTiled) stays tiled, the no-geometry case
window-rule {
    match app-id="^dev.ryoku.OniFixture$"
    open-floating true
    default-floating-position x=1040 y=120 relative-to="top-left"
}
window-rule {
    match app-id="^dev.ryoku.OniFixtureList$"
    open-floating true
    default-floating-position x=1120 y=640 relative-to="top-left"
}
window-rule {
    match app-id="^dev.ryoku.OniGuide$"
    open-floating true
    default-floating-position x=1120 y=440 relative-to="top-left"
}
// Phase 13's lab browser floats; its notes window stays tiled, so niri's
// maximize (a column verb) changes its width
window-rule {
    match app-id="^dev.ryoku.OniBrowser$"
    open-floating true
    default-floating-position x=240 y=120 relative-to="top-left"
}
// Phase 14's Hands browser floats on the right half, beside the app
window-rule {
    match app-id="^(chromium|hyakki-hands-[0-9a-f]{8})$"
    open-floating true
    default-floating-position x=976 y=24 relative-to="top-left"
    default-column-width { fixed 928; }
    default-window-height { fixed 1032; }
}
// Phase 15's target and sentinel float on the right half; OniTiled stays tiled
window-rule {
    match app-id="^dev.ryoku.OniTarget$"
    open-floating true
    default-floating-position x=1040 y=200 relative-to="top-left"
}
window-rule {
    match app-id="^dev.ryoku.OniSentinel$"
    open-floating true
    default-floating-position x=1120 y=640 relative-to="top-left"
}
EOF
    lab_run niri 768M setsid -w env -i HOME="$ONI_LAB/home" PATH=/usr/bin:/bin XDG_RUNTIME_DIR="$LAB_WRT" \
        WAYLAND_DISPLAY=wl-host niri -c "$ONI_LAB/wm/niri.kdl" \
        >"$LAB_LOG/niri.log" 2>&1 </dev/null &
    lab_wait 150 test -S "$LAB_WRT/wayland-1" || lab_die "nested niri did not start, see $LAB_LOG/niri.log"
    # shellcheck disable=SC2046
    lab_wait 50 first_socket $(printf '%s ' "$LAB_WRT"/niri.wayland-1.*.sock) >/dev/null ||
        lab_die "nested niri has no IPC socket, see $LAB_LOG/niri.log"
    # shellcheck disable=SC2046
    LAB_NIRI_SOCKET=$(first_socket $(printf '%s ' "$LAB_WRT"/niri.wayland-1.*.sock))
    LAB_DISPLAY=$LAB_WRT/wayland-1
    LAB_WM_ENV=NIRI_SOCKET=$LAB_NIRI_SOCKET
    LAB_DESKTOP=niri
    geometry=$(lab_niri -j outputs | python3 -c '
import json, sys
outs = json.load(sys.stdin)
name, o = sorted(outs.items())[0]
l = o.get("logical") or {}
print(name, l.get("width", 0), l.get("height", 0))')
else
    drm=${ONI_LAB_DRM:-}
    if [ -z "$drm" ]; then
        for node in /sys/class/drm/renderD*; do
            case $(basename "$(readlink -f "$node/device/driver" 2>/dev/null)") in
                amdgpu | i915 | xe) drm=/dev/dri/${node##*/}; break ;;
            esac
        done
    fi
    [ -n "$drm" ] || drm=/dev/dri/renderD128
    [ -e "$drm" ] || lab_die "render node $drm does not exist"
    lab_env_put LAB_DRM "$drm"
    lab_run tinywl 512M setsid -w env -i HOME="$ONI_LAB/home" PATH=/usr/bin:/bin XDG_RUNTIME_DIR="$LAB_WRT" \
        WLR_BACKENDS=headless WLR_RENDERER=gles2 WLR_RENDER_DRM_DEVICE="$drm" "$TINYWL" \
        >"$LAB_LOG/tinywl.log" 2>&1 </dev/null &
    host_socket() {
        for _s in "$LAB_WRT"/wayland-*; do
            case $_s in *.lock) continue ;; esac
            [ -S "$_s" ] && { printf '%s\n' "${_s##*/}"; return 0; }
        done
        return 1
    }
    lab_wait 100 host_socket >/dev/null || lab_die "tinywl did not start, see $LAB_LOG/tinywl.log"
    host=$(host_socket)
    i=2 monitors=
    while [ "$i" -le $((OUTPUTS + 1)) ]; do
        monitors="$monitors HEADLESS-$i"
        i=$((i + 1))
    done
    {
        x=0
        for m in $monitors; do
            printf 'hl.monitor({ output = "%s", mode = "%sx%s@60", position = "%sx0", scale = 1 })\n' "$m" "$WIDTH" "$HEIGHT" "$x"
            x=$((x + WIDTH))
        done
        cat <<'EOF'
hl.config({
    misc = { disable_hyprland_logo = true, disable_splash_rendering = true },
    animations = { enabled = false },
    xwayland = { enabled = false },
    debug = { disable_logs = false },
})
hl.window_rule({ name = "oni-fixture", match = { class = "^dev\\.ryoku\\.OniFixture$" }, float = true, move = "1040 120" })
hl.window_rule({ name = "oni-fixture-list", match = { class = "^dev\\.ryoku\\.OniFixtureList$" }, float = true, move = "1120 640" })
hl.window_rule({ name = "oni-guide", match = { class = "^dev\\.ryoku\\.OniGuide$" }, float = true, move = "1120 440" })
hl.window_rule({ name = "oni-browser", match = { class = "^dev\\.ryoku\\.OniBrowser$" }, float = true, move = "240 120" })
hl.window_rule({ name = "oni-notes", match = { class = "^dev\\.ryoku\\.OniNotes$" }, float = true, move = "240 600" })
hl.window_rule({ name = "oni-app", match = { class = "^dev\\.ryoku\\.Hyakki$" }, float = true, move = "16 16", size = "1280 800" })
hl.window_rule({ name = "oni-phase16", match = { class = "^(dev\\.ryoku\\.OniPhase16|kitty|org\\.mozilla\\.firefox)$" }, float = true, move = "1320 120", size = "576 420" })
EOF
        # Phase 14's Hands browser: on the second output when there is one,
        # else the right half beside the app
        if [ "$OUTPUTS" -gt 1 ]; then
            rule_monitor='monitor = "HEADLESS-3", '
            hx=40 hw=$((WIDTH - 80))
        else
            rule_monitor=
            hx=976 hw=928
        fi
        printf 'hl.window_rule({ name = "oni-hands", match = { class = "^(chromium|hyakki-hands-[0-9a-f]{8})$" }, float = true, %smove = "%s 40", size = "%s %s" })\n' \
            "$rule_monitor" "$hx" "$hw" "$((HEIGHT - 80))"
        # Keep ordinary targets clear of the glow strips; capture masks stay on.
        if [ "$OUTPUTS" -gt 1 ]; then px=200; else px=1040; fi
        printf 'hl.window_rule({ name = "oni-target", match = { class = "^dev\\\\.ryoku\\\\.OniTarget$" }, float = true, %smove = "%s 200" })\n' "$rule_monitor" "$px"
        printf 'hl.window_rule({ name = "oni-sentinel", match = { class = "^dev\\\\.ryoku\\\\.OniSentinel$" }, float = true, %smove = "%s 640" })\n' "$rule_monitor" "$((px + 80))"
    } >"$ONI_LAB/wm/hypr/hyprland.lua"
    lab_run hyprland 1024M setsid -w env -i HOME="$ONI_LAB/home" PATH=/usr/bin:/bin XDG_RUNTIME_DIR="$LAB_WRT" \
        XDG_CONFIG_HOME="$ONI_LAB/wm" XDG_CACHE_HOME="$ONI_LAB/cache" XDG_DATA_HOME="$ONI_LAB/data" \
        XDG_STATE_HOME="$ONI_LAB/state" HYPRLAND_NO_SD_VARS=1 LIBSEAT_BACKEND=seatd \
        WAYLAND_DISPLAY="$host" Hyprland \
        >"$LAB_LOG/hyprland.log" 2>&1 </dev/null &
    # The instance must belong to this lab: it runs in the lab's Hyprland
    # scope and its socket is not the host's.
    hypr_instance() {
        _j=$(lab_run "hyprctl-$(lab_rand)" 64M env -i PATH=/usr/bin:/bin XDG_RUNTIME_DIR="$LAB_WRT" \
            hyprctl instances -j 2>/dev/null) || return 1
        python3 -c '
import json, sys
scope, host = sys.argv[2], sys.argv[3]
try:
    instances = json.loads(sys.argv[1])
except ValueError:
    sys.exit(1)
for i in instances:
    try:
        cgroup = open("/proc/%d/cgroup" % i["pid"]).read()
    except OSError:
        continue
    if scope in cgroup and i.get("wl_socket") not in ("", host):
        print(i["instance"], i["wl_socket"])
        sys.exit(0)
sys.exit(1)' "$_j" "/oni-lab-$LAB_ID-hyprland.scope" "$host"
    }
    lab_wait 150 hypr_instance >/dev/null || lab_die "nested Hyprland did not start, see $LAB_LOG/hyprland.log"
    instance=$(hypr_instance)
    read -r LAB_HYPR_INSTANCE wm_socket <<EOF
$instance
EOF
    LAB_DISPLAY=$LAB_WRT/$wm_socket
    LAB_WM_ENV=HYPRLAND_INSTANCE_SIGNATURE=$LAB_HYPR_INSTANCE
    LAB_DESKTOP=Hyprland
    for m in $monitors; do
        lab_hyprctl output create headless "$m" >/dev/null
    done
    sleep 0.5
    # the host's window is only the nest's door; the headless output is the screen
    for m in $(lab_hyprctl -j monitors all | python3 -c '
import json, sys
print(" ".join(m["name"] for m in json.load(sys.stdin) if not m["name"].startswith("HEADLESS")))'); do
        lab_hyprctl eval "hl.monitor({ output = \"$m\", disabled = true })" >/dev/null 2>&1 || true
    done
    lab_hyprctl dismissnotify >/dev/null 2>&1 || true
    sleep 0.3
    lab_hyprctl configerrors >"$LAB_LOG/hyprland-configerrors.txt" 2>&1 || true
    geometry=$(lab_hyprctl -j monitors | python3 -c '
import json, sys
ms = [m for m in json.load(sys.stdin) if m["name"] == "HEADLESS-2"]
m = ms[0] if ms else {"name": "HEADLESS-2", "width": 0, "height": 0, "scale": 1}
print(m["name"], round(m["width"] / m["scale"]), round(m["height"] / m["scale"]))')
    ln -s "$LAB_WRT/hypr" "$LAB_RT/hypr"
fi

set -- $geometry
LAB_OUTPUT=$1
[ "$2" -gt 0 ] && [ "$3" -gt 0 ] ||
    lab_die "output $LAB_OUTPUT came up ${2}x$3 (wrong render node? see $LAB_LOG)"
extent=$(lab_extent)
read -r LAB_EXTENT_W LAB_EXTENT_H <<EOF
$extent
EOF
lab_env_put LAB_DISPLAY "$LAB_DISPLAY"
lab_env_put LAB_OUTPUT "$LAB_OUTPUT"
lab_env_put LAB_EXTENT_W "$LAB_EXTENT_W"
lab_env_put LAB_EXTENT_H "$LAB_EXTENT_H"
lab_env_put LAB_DESKTOP "$LAB_DESKTOP"
lab_env_put LAB_WM_ENV "$LAB_WM_ENV"
lab_env_put LAB_NIRI_SOCKET "${LAB_NIRI_SOCKET:-}"
lab_env_put LAB_HYPR_INSTANCE "${LAB_HYPR_INSTANCE:-}"

if [ "$LAB_WM" = hyprland ] && [ "$APP" = 1 ]; then
    command -v wtype >/dev/null 2>&1 || lab_die "missing wtype for the private app keyboard"
    # A disappearing last keyboard loses focus between otherwise valid inputs.
    # Split the idle hold to avoid wtype's 32-bit millisecond-to-microsecond overflow.
    lab_tool keyboard-keeper wtype \
        -s 1800000 -s 1800000 -s 1800000 -s 1800000 \
        -s 1800000 -s 1800000 -s 1800000 -s 1800000 \
        >"$LAB_LOG/keyboard-keeper.log" 2>&1 </dev/null &
    keyboard_ready() {
        # An idle empty keymap makes devices -j emit bare "none", not JSON.
        lab_hyprctl devices | python3 -c '
import sys
sys.exit(not any(line.strip() == "hl-virtual-keyboard-wtype" for line in sys.stdin))'
    }
    lab_wait 50 keyboard_ready || lab_die "the private app keyboard did not appear, see $LAB_LOG/keyboard-keeper.log"
fi

# Refusal stubs first on every lab process's PATH: the client would otherwise
# ask the live user manager for oni.socket, and a Ryoku command would reach
# the live desktop. refuse.c is compiled, not a shell script, so the read-only
# scan sees the command the daemon ran rather than /bin/sh running a script.
lab_run "cc-$(lab_rand)" 256M cc -O2 -Wall -Wextra -Werror -DREFUSED_LOG="\"$LAB_LOG/refused.log\"" \
    -o "$ONI_LAB/tmp/refuse" "$HERE/refuse.c" || lab_die "refuse.c did not build"
for stub in systemctl ryoku ryoku-shell ryoku-host; do
    cp "$ONI_LAB/tmp/refuse" "$ONI_LAB/bin/$stub"
    chmod 755 "$ONI_LAB/bin/$stub"
done
# Phase 13's launcher wraps an app in "systemd-run --user --scope --collect
# --" where a user manager runs; the lab's stand-in refuses it as the real
# tool would when the manager says no (scoperefuse.c), so no lab process can
# reach the live user manager.
lab_run "cc-$(lab_rand)" 256M cc -O2 -Wall -Wextra -Werror -DREFUSED_LOG="\"$LAB_LOG/refused.log\"" \
    -o "$ONI_LAB/bin/systemd-run" "$HERE/scoperefuse.c" || lab_die "scoperefuse.c did not build"
chmod 755 "$ONI_LAB/bin/systemd-run"
rm -f "$ONI_LAB/tmp/refuse"

if [ "$HANDS" = 1 ]; then
    # Phase 14's pages: two loopback origins the Hands browser loads; the
    # port file is one JSON object (handsfix.py's docstring)
    mkdir -p "$ONI_LAB/hands"
    lab_run hands 256M env -i PATH=/usr/bin:/bin python3 "$HERE/handsfix.py" serve \
        --port-file "$ONI_LAB/hands/port" --log "$LAB_LOG/hands-requests.jsonl" \
        >"$LAB_LOG/hands.log" 2>&1 </dev/null &
    lab_wait 100 test -s "$ONI_LAB/hands/port" || lab_die "the Hands web fixture did not listen, see $LAB_LOG/hands.log"
    for k in a b; do
        o=$(python3 -c 'import json, sys; print(json.load(open(sys.argv[1]))["origin_" + sys.argv[2]])' "$ONI_LAB/hands/port" "$k") ||
            lab_die "the Hands web fixture's port file is unreadable"
        case $o in http://127.0.0.1:[0-9]*) ;; *) lab_die "the Hands web fixture reported origin $o" ;; esac
        if [ "$k" = a ]; then lab_env_put LAB_HANDS_A "$o"; else lab_env_put LAB_HANDS_B "$o"; fi
    done
fi

if [ "$APP" = 1 ] || [ "$SURFACE" = 1 ]; then
    # "version" 2 is the current schema (config.Version): a file without it is
    # read as a Rashin 1 file and migrated, which drops the oni block, and Oni
    # stayed off (the surface exited at once with reason oni-off).
    local_json=
    if [ "$MODEL" = 1 ]; then
        # The vision fixture listens before the daemon reads its settings;
        # only the app may change a local address, so it is seeded here.
        mkdir -p "$ONI_LAB/model" "$ONI_LAB/build"
        python3 "$HERE/fixture.py" build "$ONI_LAB/build" >/dev/null
        lab_run model 256M env -i PATH=/usr/bin:/bin python3 "$HERE/model.py" serve \
            --port-file "$ONI_LAB/model/port" --log "$LAB_LOG/model-requests.jsonl" \
            --manifest "$ONI_LAB/build/notes.json" >"$LAB_LOG/model.log" 2>&1 </dev/null &
        lab_wait 100 test -s "$ONI_LAB/model/port" || lab_die "the lab model did not listen, see $LAB_LOG/model.log"
        LAB_MODEL_URL=http://127.0.0.1:$(cat "$ONI_LAB/model/port")
        lab_env_put LAB_MODEL_URL "$LAB_MODEL_URL"
        decisions_json=
        if [ "$DECISIONS" = 1 ]; then
            # the typed decision route is the app's alone too: local, loopback
            decisions_json=", \"decisions\": {\"route\": \"local\", \"localAddress\": \"$LAB_MODEL_URL/v1\", \"localModel\": \"oni-fixture-decisions\", \"features\": {\"oni.reflex\": \"local\"}}"
        fi
        local_json=", \"providers\": {\"local\": {\"llamacpp\": \"$LAB_MODEL_URL/v1\", \"ollama-local\": \"$LAB_MODEL_URL/closed/v1\"}$decisions_json}"
    fi
    # Earlier phase proofs start beyond onboarding without changing any
    # optional feature. First-use acceptance explicitly opts into fresh state.
    case ${ONI_LAB_FIRST_RUN:-0} in
        0) onboarding_json=', "onboarding": {"seen": true, "done": true}' ;;
        1) onboarding_json= ;;
        *) lab_die "ONI_LAB_FIRST_RUN must be 0 or 1" ;;
    esac
    if [ "$SURFACE" = 1 ]; then
        printf '{"version": 2, "enabled": true, "oni": {"master": true}%s%s}\n' "$onboarding_json" "$local_json" >"$ONI_LAB/cfg/hyakki/hyakki.json"
    else
        printf '{"version": 2, "enabled": true%s%s}\n' "$onboarding_json" "$local_json" >"$ONI_LAB/cfg/hyakki/hyakki.json"
    fi
    lab_app dbus 128M dbus-daemon --session --address="unix:path=$LAB_RT/bus" --nofork --nopidfile \
        >"$LAB_LOG/dbus.log" 2>&1 </dev/null &
    lab_wait 50 test -S "$LAB_RT/bus" || lab_die "private session bus did not start, see $LAB_LOG/dbus.log"
    lab_daemon || lab_die "oni did not listen, see $LAB_LOG/oni.log"
fi

if [ "$SURFACE" = 1 ]; then
    lab_surface || lab_die "oni-surface wrote no ledger, see $LAB_LOG/oni-surface.log"
    python3 "$LAB_KIT" wait "$LAB_LEDGER" --timeout 15 ev=commit >/dev/null ||
        lab_die "oni-surface committed no frame, see $LAB_LOG/oni-surface.log and $LAB_LEDGER"
fi

if [ "$APP" = 1 ]; then
    lab_app app 1536M "$HYAKKI_BIN/hyakki-app" --size 1280x800 >"$LAB_LOG/hyakki-app.log" 2>&1 </dev/null &
    has_window() {
        lab_state 2>/dev/null | python3 -c '
import json, sys
sys.exit(0 if any(w.get("app_id") == "dev.ryoku.Hyakki" for w in json.load(sys.stdin)["windows"]) else 1)'
    }
    lab_wait 60 has_window || lab_die "no dev.ryoku.Hyakki window, see $LAB_LOG/hyakki-app.log"
fi

lab_env_put LAB_READY 1
lab_state
printf 'oni lab up: %s on %s (pointer extent %sx%s), lab %s\n' "$LAB_WM" "$LAB_OUTPUT" "$LAB_EXTENT_W" "$LAB_EXTENT_H" "$ONI_LAB"
