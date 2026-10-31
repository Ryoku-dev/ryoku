#!/usr/bin/env bash
set -euo pipefail

root=${RYOKU_PATH:-$(cd "$(dirname "$0")/.." && pwd)}
viz="$root/ryoku/shell/quickshell/shell/modules/visualizer/Visualizer.qml"
grip="$root/ryoku/shell/quickshell/shell/modules/visualizer/VisualizerGrip.qml"
config="$root/ryoku/shell/quickshell/shell/modules/visualizer/Singletons/Config.qml"
desktop="$root/ryoku/shell/quickshell/shell/modules/desktop/Desktop.qml"
shell="$root/ryoku/shell/quickshell/shell/shell.qml"

fail() { printf 'visualizer placement: %s\n' "$*" >&2; exit 1; }

grep -Fq 'function dataAt(index)' "$config" || fail 'missing stable per-instance accessor'
grep -Fq 'model: Config.count' "$viz" || fail 'render repeater rebuilds on every config object update'
grep -Fq '&& perScreen.st && perScreen.st.visualizerOverlay' "$shell" \
    || fail 'the separate surface is not gated by explicit overlay mode'
! grep -Fq '&& !desktop.hostsVisualizer && !desktop.plainVisualizer' "$shell" \
    || fail 'the separate surface infers overlay mode from inline host timing'

# Placement lives in the Stage Editor now: the grip rides the desktop surface
# and writes the box through the same store the old placer did. The standalone
# placer, its edge bar and its drawer are gone, and the visualizer surface no
# longer carries a placement mode of its own.
[[ -f $grip ]] || fail 'the Stage Editor grip is missing'
grep -Fq 'Config.setBox(win.tx, win.ty, win.tw, win.th, win.aspect)' "$grip" \
    || fail 'the grip no longer sizes through the store'
grep -Fq 'Viz.VisualizerGrip' "$desktop" || fail 'the desktop never mounts the grip'
grep -Fq 'function stageVizGestureStart' "$desktop" \
    || fail 'a grip gesture records no walk-back'
grep -Fq 'signal gestureStarted()' "$grip" \
    || fail 'the grip raises no gesture for the walk-back'

[[ ! -f $root/ryoku/shell/quickshell/shell/modules/visualizer/Placer.qml ]] \
    || fail 'the standalone placer survived the fold-in'
[[ ! -f $root/ryoku/shell/quickshell/shell/modules/visualizer/EditBar.qml ]] \
    || fail 'the standalone edit bar survived the fold-in'
[[ ! -f $root/ryoku/shell/quickshell/shell/modules/visualizer/SettingsPopup.qml ]] \
    || fail 'the standalone settings drawer survived the fold-in'
! grep -Fq 'placing' "$viz" || fail 'the visualizer surface still carries a placement mode'

echo 'visualizer placement: PASS'
