#!/usr/bin/env bash
# The Stage notification service hears every notification the shell's own
# server hears. One notify-send must chime once, through the shell's server and
# the Hub's cue, and the Stage service plays its own notifications through that
# cue too. paplay and ryoku-shell are shims that log what they were asked to
# play; a private session bus carries the notification to both servers. Under
# Do Not Disturb the shell holds back a normal notification but lets a critical
# one through, and Game Mode holds back both.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/imports/Ryoku" "$tmp/bin" "$tmp/home" "$tmp/config/ryoku/stage" "$tmp/state" "$tmp/cache" "$tmp/data" "$tmp/runtime"
chmod 700 "$tmp/runtime"
# Offscreen there is no compositor behind Quickshell's WindowManager, so the
# copy of Ryoku.Ui gets a Wm that reports one output with nothing fullscreen.
cp -r "$root/ryoku/ui" "$tmp/imports/Ryoku/Ui"
ln -s "$root/ryoku/shell/framebars" "$tmp/imports/Ryoku/FrameBars"
cat > "$tmp/imports/Ryoku/Ui/Singletons/Wm.qml" <<'EOF'
pragma Singleton
import Quickshell

Singleton {
    readonly property string focusedOutput: ""
    function outputHasFullscreen(name) { return false; }
}
EOF
cp "$root/tests/notification-sounds.qml" "$tmp/shell.qml"
: > "$tmp/calls"
for tool in paplay pw-play ryoku-shell; do
    printf '#!/bin/sh\nprintf "%%s %%s\\n" "%s" "$*" >> "%s"\n' "$tool" "$tmp/calls" > "$tmp/bin/$tool"
    chmod +x "$tmp/bin/$tool"
done
# A custom clip that exists, so a Stage chime would reach paplay instead of
# missing the theme files a build box does not have.
: > "$tmp/chime.oga"
printf '{"sounds":{"custom":{"notifications":"%s"}}}\n' "$tmp/chime.oga" > "$tmp/config/ryoku/stage/stage-editor.json"
# A bus of its own with no service files, so no installed notification daemon
# is activated in place of the server under test.
cat > "$tmp/bus.conf" <<EOF
<!DOCTYPE busconfig PUBLIC "-//freedesktop//DTD D-Bus Bus Configuration 1.0//EN"
 "http://www.freedesktop.org/standards/dbus/1.0/busconfig.dtd">
<busconfig>
  <type>session</type>
  <listen>unix:dir=$tmp/runtime</listen>
  <policy context="default">
    <allow send_destination="*" eavesdrop="true"/>
    <allow eavesdrop="true"/>
    <allow own="*"/>
  </policy>
</busconfig>
EOF
env -u WAYLAND_DISPLAY -u DISPLAY -u HYPRLAND_INSTANCE_SIGNATURE -u NIRI_SOCKET -u DBUS_SESSION_BUS_ADDRESS \
    PATH="$tmp/bin:$PATH" HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/config" XDG_STATE_HOME="$tmp/state" \
    XDG_CACHE_HOME="$tmp/cache" XDG_DATA_HOME="$tmp/data" XDG_RUNTIME_DIR="$tmp/runtime" \
    RYOKU_TEST_CALLS="$tmp/calls" QT_QPA_PLATFORM=offscreen QS_DISABLE_CRASH_HANDLER=1 \
    QML_IMPORT_PATH="$tmp/imports:$root/ryoku/shell/quickshell:${QML_IMPORT_PATH:-}" \
    QML2_IMPORT_PATH="$tmp/imports:$root/ryoku/shell/quickshell:${QML2_IMPORT_PATH:-}" \
    timeout 40s dbus-run-session --config-file="$tmp/bus.conf" -- qs -p "$tmp/shell.qml" > "$tmp/output" 2>&1 || {
        cat "$tmp/output"
        exit 1
    }
if grep -q 'FAIL:' "$tmp/output" || ! grep -q 'PASS:' "$tmp/output"; then
    cat "$tmp/output"
    exit 1
fi
grep 'PASS:' "$tmp/output"
