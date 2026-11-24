#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$here/../.."
qs_bin="$(command -v qs)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/Ryoku" "$work/bin" "$work/run" "$work/config/ryoku"
chmod 700 "$work/run"
ln -s "$repo/ryoku/ui" "$work/Ryoku/Ui"
cp -a "$repo/ryoku/hub/quickshell" "$work/hub"
cp "$here/bar-studio-card-probe.qml" "$work/hub/probe.qml"
echo '{}' >"$work/config/ryoku/shell.json"

# A Store style with a verified view, the way RyoStore lays one out.
view="barstyle-views/probe-style/$(printf probe | sha256sum | cut -c1-64)"
mkdir -p "$work/state/ryoku/store/$view"
printf '[{"id":"probe-style","version":"1.0.0","scene":"Scene.qml","view":"%s"}]\n' "$view" \
    >"$work/state/ryoku/store/barstyles.json"
card="$work/state/ryoku/store/$view/studio-card.json"
cat >"$card" <<'JSON'
{"schema": 1, "cards": [{"title": "PROBE", "glyph": "試", "blocks": [
  {"text": "A Store style's own card."},
  {"buttons": [{"label": "OPEN PROBE SETTINGS", "ipc": "probe-style", "call": "open", "args": ["a b", "$(id)"]}]}
]}]}
JSON

# Every program the Hub would start only records how it was called.
for tool in qs ryoku-hub ryoku ryoku-shell ryostore; do
    printf '#!/usr/bin/env bash\nprintf "%%s|" %s "$@" >>"%s/exec.log"; echo >>"%s/exec.log"\n' \
        "$tool" "$work" "$work" >"$work/bin/$tool"
    chmod +x "$work/bin/$tool"
done

run() {
    env -u RYOKU_SHELL_DIR \
    PATH="$work/bin:$PATH" \
    XDG_RUNTIME_DIR="$work/run" \
    XDG_CONFIG_HOME="$work/config" \
    XDG_STATE_HOME="$work/state" \
    XDG_CACHE_HOME="$work/cache" \
    QT_QPA_PLATFORM=offscreen \
    PROBE_EXPECT="$1" \
    PROBE_STYLE="${2:-}" \
    QML2_IMPORT_PATH="$work:${QML2_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}" \
        timeout 30 "$qs_bin" -p "$work/hub/probe.qml" >"$work/log" 2>&1 || true
    if ! grep -q CARD-PROBE-DONE "$work/log"; then
        grep CARD-PROBE "$work/log" || sed -n '1,80p' "$work/log"
        exit 1
    fi
}

run card
# the arguments reach qs as written, a shell expansion included
# shellcheck disable=SC2016
want='qs|-c|shell|ipc|call|probe-style|open|a b|$(id)|'
grep -qxF "$want" "$work/exec.log" || { echo "bar-studio-card-probe: the button ran:"; cat "$work/exec.log"; exit 1; }

# A Store card may only call its own IPC target: the file is ignored whole.
sed -i 's/"ipc": "probe-style"/"ipc": "desktop"/' "$card"
run layout
grep -q "bar studio: ignoring .*must be the style's own id" "$work/log" \
    || { echo "bar-studio-card-probe: no log line for the refused file"; exit 1; }

# A refused file falls through: a Store "obi" with a bad file still gets the
# Hub's own Obi card.
obi="barstyle-views/obi/$(printf obi | sha256sum | cut -c1-64)"
mkdir -p "$work/state/ryoku/store/$obi"
printf '[{"id":"obi","version":"1.0.0","scene":"Scene.qml","view":"%s"}]\n' "$obi" \
    >"$work/state/ryoku/store/barstyles.json"
echo '{"schema": 1, "cards": []}' >"$work/state/ryoku/store/$obi/studio-card.json"
run obi obi

echo "bar-studio-card-probe: a Store style's card draws, runs its own IPC call, a bad file falls back, and a refused file falls through"
