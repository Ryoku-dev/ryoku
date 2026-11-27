#!/usr/bin/env bash
set -euo pipefail

# The Orpheus voice faces must size their text from the notification or the
# media title alone, never from scene.textRoom, which the bar's own overflow
# sets. A face that reads it feeds its own width back and the scene re-polishes
# forever, freezing the shell (#985). The probe builds both faces against a stub
# scene whose textRoom moves and fails if a face follows it.

here="$(cd "$(dirname "$0")" && pwd)"
repo="$here/../.."
work="$(mktemp -d)"
pid=""
cleanup() {
    if [[ -n $pid ]]; then kill "$pid" 2>/dev/null || true; fi
    rm -rf "$work"
}
trap cleanup EXIT

wait_for() {
    for _ in $(seq 1 300); do
        grep -q "$1" "$work/log" 2>/dev/null && return 0
        kill -0 "$pid" 2>/dev/null || break
        sleep 0.1
    done
    cat "$work/log" 2>/dev/null || true
    return 1
}

# The probe hands the media face a fake player, and Media.player is the pick
# the Music service pushes to ryoku-shell over its socket under
# XDG_RUNTIME_DIR. On a running desktop that would replace the now-playing song
# and start cover and lyric lookups for it, so the probe gets a runtime
# directory of its own where no daemon socket exists.
mkdir -m 700 "$work/runtime"

RYOKU_REPO="$repo" \
RYOKU_PROBE_ISOLATED=1 \
XDG_RUNTIME_DIR="$work/runtime" \
QML_IMPORT_PATH="$repo/ryoku/shell/quickshell:${QML_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}" \
QML2_IMPORT_PATH="$repo/ryoku/shell/quickshell:${QML2_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}" \
QT_QPA_PLATFORM=offscreen timeout 60 qs -p "$here/orpheus-voice-room-probe.qml" \
    >"$work/log" 2>&1 &
pid=$!

wait_for 'ORPHEUS-VOICE-ROOM-PROBE-PASS'

# The runtime directory above has no pipewire socket, so the audio service
# reports that one expected failure; any other error still fails the probe.
problems="$(grep -E '(^|[[:space:]])(ERROR|TypeError|ReferenceError)|Failed to load|PROBE-FAIL' "$work/log" \
    | grep -v 'quickshell\.service\.pipewire\.loop.*Failed to connect pipewire context' || true)"
if [[ -n $problems ]]; then
    cat "$work/log"
    exit 1
fi

echo "orpheus-voice-room-probe: the voice faces size from their own text"
