#!/usr/bin/env bash
set -euo pipefail

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
renderer="$root/ryoku/shell/quickshell/reload-cover/ReloadCover.qml"
media="$root/ryoku/shell/quickshell/reload-cover/ReloadMedia.qml"
shell="$root/ryoku/shell/quickshell/shell/shell.qml"
cover_shell="$root/ryoku/shell/quickshell/reload-cover/shell.qml"

# The hold deadline is mode-based: a reload gets the supervised-restart
# budget, a boot waits on the whole session chain.
grep -qF 'interval: root.mode === "boot" ? 45000 : 16500' "$cover_shell"
grep -qF 'readonly property string mode: Quickshell.env("RYOKU_RELOAD_COVER_MODE")' "$cover_shell"

grep -qF 'id: reloadHold' "$shell"
grep -qF 'interval: 1500' "$shell"
grep -qF 'reloadHold.restart()' "$shell"
# One read at load: a stale shell must never steal the next instance's finish.
grep -qF 'watchChanges: false' "$shell"

grep -qF 'id: loadingSweep' "$media"
grep -qF 'NumberAnimation on x' "$media"
grep -qF 'running: defaultLogo.visible && root.active' "$media"
grep -qF 'duration: 1200' "$media"
grep -qF 'ColorOverlay {' "$media"
grep -qF 'text: cover.mode === "boot" ? I18n.tr("LOADING DESKTOP") : I18n.tr("SHELL RELOADING")' "$renderer"
grep -qF 'visible: cover.phase !== "failed" && media.showingDefault && cover.mediaOpacity > 0' "$renderer"
grep -qF 'import QtQuick.Shapes' "$renderer"
grep -qF 'fillRule: ShapePath.OddEvenFill' "$renderer"
grep -qF 'PathSvg {' "$renderer"
test "$(grep -cF 'duration: 520' "$renderer")" = 2
grep -qF 'visible: cover.phase === "closing" || cover.phase === "opening"' "$renderer"
grep -qF 'interval: 520' "$cover_shell"
grep -qF 'interval: 560' "$cover_shell"
live_runtime="${XDG_RUNTIME_DIR:-}"
reload_pid=""

cleanup() {
    local status="$1" token
    if [[ -n $reload_pid ]] && kill -0 "$reload_pid" 2>/dev/null; then
        wait "$reload_pid" 2>/dev/null || true
    fi
    if [[ -n $live_runtime && -e "$live_runtime/ryoku-reload-cover.json" ]]; then
        token=$(env XDG_RUNTIME_DIR="$live_runtime" jq -r '.token // empty' "$live_runtime/ryoku-reload-cover.json" 2>/dev/null || true)
        [[ $token =~ ^[0-9a-f]{32}$ ]] && env XDG_RUNTIME_DIR="$live_runtime" ryoku-reload-cover finish "$token" >/dev/null 2>&1 || true
    fi
    rm -rf "$tmp"
    trap - EXIT
    exit "$status"
}
trap 'cleanup $?' EXIT

export XDG_RUNTIME_DIR="$tmp/runtime"
mkdir -p "$XDG_RUNTIME_DIR"
cover="$root/ryoku/shell/scripts/ryoku-reload-cover"

token=$(RYOKU_RELOAD_COVER_TEST=1 "$cover" begin)
test -n "$token"
test "$(stat -c %a "$XDG_RUNTIME_DIR/ryoku-reload-cover.json")" = 600
test "$(jq -r .token "$XDG_RUNTIME_DIR/ryoku-reload-cover.json")" = "$token"
test "$(jq -r .pid "$XDG_RUNTIME_DIR/ryoku-reload-cover.json")" -gt 0
test -n "$(jq -r .pidStart "$XDG_RUNTIME_DIR/ryoku-reload-cover.json")"
RYOKU_RELOAD_COVER_TEST=1 "$cover" finish wrong-token
test "$(jq -r .token "$XDG_RUNTIME_DIR/ryoku-reload-cover.json")" = "$token"
RYOKU_RELOAD_COVER_TEST=1 "$cover" finish "$token"
test ! -e "$XDG_RUNTIME_DIR/ryoku-reload-cover.json"

for out in "$tmp/a" "$tmp/b"; do
    (RYOKU_RELOAD_COVER_TEST=1 "$cover" begin >"$out" 2>/dev/null) &
done
wait || true
test "$(awk 'NF { n++ } END { print n + 0 }' "$tmp/a" "$tmp/b")" = 1
token=$(jq -r .token "$XDG_RUNTIME_DIR/ryoku-reload-cover.json")
RYOKU_RELOAD_COVER_TEST=1 "$cover" finish "$token"

# Boot mode is a first-class begin argument: the autostart chain raises the
# cover with it and the cover holds to the longer boot deadline.
token=$(RYOKU_RELOAD_COVER_TEST=1 "$cover" begin boot)
test -n "$token"
RYOKU_RELOAD_COVER_TEST=1 "$cover" finish "$token"
# Any other mode is a usage error, not a silent reload.
if RYOKU_RELOAD_COVER_TEST=1 "$cover" begin sideways >/dev/null 2>&1; then
    echo "begin accepted an unknown mode" >&2
    exit 1
fi
# Outside a compositor session begin must fail fast (a TTY doctor repair
# burns no map-poll budget); the gate sits before any qs spawn.
if env -u WAYLAND_DISPLAY -u RYOKU_RELOAD_COVER_TEST "$cover" begin >/dev/null 2>&1; then
    echo "begin succeeded with no Wayland session" >&2
    exit 1
fi
test ! -e "$XDG_RUNTIME_DIR/ryoku-reload-cover.json"

# A graphics hang can stop the cover's QML timer with the surface still mapped.
# The launcher's process watchdog must tear that instance down independently.
mkdir -p "$tmp/bin"
cat >"$tmp/bin/qs" <<'EOF'
#!/usr/bin/env bash
if [[ ${3:-} == ipc ]]; then
    case "${6:-}" in
        mapped|covered) echo true ;;
        status) echo hold ;;
        *) echo false ;;
    esac
    exit 0
fi
exec sleep 300
EOF
chmod +x "$tmp/bin/qs"
token=$(PATH="$tmp/bin:$PATH" WAYLAND_DISPLAY=wayland-test \
    RYOKU_SHELL_DIR="$root/ryoku/shell" \
    RYOKU_RELOAD_COVER_WATCHDOG_TICKS=10 "$cover" begin boot)
test -n "$token"
stuck_pid=$(jq -r .pid "$XDG_RUNTIME_DIR/ryoku-reload-cover.json")
for _ in $(seq 1 40); do
    [[ ! -e $XDG_RUNTIME_DIR/ryoku-reload-cover.json ]] && break
    sleep 0.1
done
test ! -e "$XDG_RUNTIME_DIR/ryoku-reload-cover.json"
if kill -0 "$stuck_pid" 2>/dev/null; then
    echo "process watchdog left a stuck cover alive" >&2
    exit 1
fi

echo "reload cover launcher: PASS"

if [[ ${RYOKU_RELOAD_COVER_LIVE:-0} == 1 ]]; then
    [[ -n $live_runtime ]] || { echo "missing live runtime" >&2; exit 1; }
    env XDG_RUNTIME_DIR="$live_runtime" ryoku-shell reload >"$tmp/reload.log" 2>&1 &
    reload_pid=$!
    seen=0
    for _ in $(seq 1 200); do
        if env XDG_RUNTIME_DIR="$live_runtime" hyprctl layers -j | grep -q '"ryoku-reload-cover"'; then
            seen=1
            break
        fi
        sleep 0.05
    done
    test "$seen" = 1
    token=$(jq -r '.token // empty' "$live_runtime/ryoku-reload-cover.json")
    test "$token" != ""
    opening_seen=0
    for _ in $(seq 1 800); do
        phase=$(env XDG_RUNTIME_DIR="$live_runtime" qs -p "${XDG_CONFIG_HOME:-$HOME/.config}/quickshell/reload-cover" ipc call reload-cover status "$token" 2>/dev/null || true)
        if [[ $phase == opening ]]; then
            sleep 0.05
            env XDG_RUNTIME_DIR="$live_runtime" grim "$tmp/reload-cover-opening.png"
            cp "$tmp/reload-cover-opening.png" /tmp/ryoku-reload-cover-opening.png
            opening_seen=1
            break
        fi
        sleep 0.02
    done
    test "$opening_seen" = 1

    delay=0
    for mark in 0 100 300 700 1200 1700 2000; do
        sleep "$delay"
        env XDG_RUNTIME_DIR="$live_runtime" grim "$tmp/reload-cover-$mark.png"
        case "$mark" in
            0) delay=0.1 ;;
            100) delay=0.2 ;;
            300) delay=0.4 ;;
            700) delay=0.5 ;;
            1200) delay=0.5 ;;
            1700) delay=0.3 ;;
        esac
    done

    magick montage "$tmp"/reload-cover-*.png -thumbnail 560x -tile 4x2 -geometry +4+4 /tmp/ryoku-reload-cover-montage.png

    for _ in $(seq 1 240); do
        kill -0 "$reload_pid" 2>/dev/null || break
        sleep 0.05
    done
    if kill -0 "$reload_pid" 2>/dev/null; then
        echo "reload did not complete" >&2
        exit 1
    fi
    wait "$reload_pid"
    reload_pid=""
    cleared=0
    for _ in $(seq 1 60); do
        if ! env XDG_RUNTIME_DIR="$live_runtime" hyprctl layers -j | grep -q '"ryoku-reload-cover"'; then
            cleared=1
            break
        fi
        sleep 0.05
    done
    if [[ $cleared != 1 ]]; then
        echo "reload cover remained after finish" >&2
        exit 1
    fi
    if [[ ${RYOKU_RELOAD_COVER_WATCHDOG:-0} == 1 ]]; then
        token=$(env XDG_RUNTIME_DIR="$live_runtime" ryoku-reload-cover begin)
        sleep 15.2
        phase=$(env XDG_RUNTIME_DIR="$live_runtime" qs -p "${XDG_CONFIG_HOME:-$HOME/.config}/quickshell/reload-cover" ipc call reload-cover status "$token")
        test "$phase" = failed
        env XDG_RUNTIME_DIR="$live_runtime" hyprctl layers -j | grep -q '"ryoku-reload-cover"'
        sleep 2.3
        if env XDG_RUNTIME_DIR="$live_runtime" hyprctl layers -j | grep -q '"ryoku-reload-cover"'; then
            echo "failed cover did not release" >&2
            exit 1
        fi
        test ! -e "$live_runtime/ryoku-reload-cover.json"
    fi
    echo "reload cover live: PASS (/tmp/ryoku-reload-cover-montage.png)"
fi
