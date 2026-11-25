#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# phase16.sh niri|hyprland OUTDIR: isolated memory/import/recall/rewrite,
# verified undo, authorized Hands-tab focus guard and assignment acceptance.
# Requires a frozen HYAKKI_BIN payload; never builds the product or attaches
# to a live browser. Every process, microphone, PRIMARY selection and input
# belongs to this driver's private lab. Exit zero only for the complete gate.
# memkit.py documents the exercised steps and writes phase16.json + evidence.
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"
ROOT=$(CDPATH='' cd -- "$HERE/../../../../.." && pwd -P)
usage() { printf '%s\n' 'usage: phase16.sh niri|hyprland OUTDIR' >&2; exit 2; }
[ "$#" -ge 1 ] || usage
case $1 in
    daemon|route|focus|close|ui|stopkeys|esc|layout) exec "$HERE/phase14.sh" "$@" ;;
    window)
        [ "$#" = 2 ] || usage
        lab_load
        case $2 in rewrite|assignment|terminal|private|sentinel|browser) ;; *) usage ;; esac
        mkdir -p "$ONI_LAB/out"
        python3 "$LAB_KIT" stop-role "$LAB_ID" phase16-target TERM
        [ -p "$ONI_LAB/out/phase16.cmd" ] || mkfifo -m 600 "$ONI_LAB/out/phase16.cmd"
        lab_tool phase16-target python3 "$HERE/memkit.py" window "$2" "$ONI_LAB/out/phase16.json" \
            "$ONI_LAB/out/phase16.cmd" >>"$LAB_LOG/phase16-window.log" 2>&1 </dev/null &
        exit 0
        ;;
    model)
        [ "$#" = 1 ] || usage
        lab_load
        port=$(cat "$ONI_LAB/model/port")
        case $port in ''|*[!0-9]*) lab_die 'invalid model port' ;; esac
        python3 "$LAB_KIT" stop-role "$LAB_ID" model TERM
        rm -f "$ONI_LAB/model/port"
        lab_run "model-memory-$(lab_rand)" 256M env -i PATH=/usr/bin:/bin python3 "$HERE/memkit.py" serve \
            --port "$port" --port-file "$ONI_LAB/model/port" --log "$LAB_LOG/model-requests.jsonl" \
            --manifest "$ONI_LAB/build/notes.json" >>"$LAB_LOG/model.log" 2>&1 </dev/null &
        n=150
        while [ "$n" -gt 0 ] && [ ! -s "$ONI_LAB/model/port" ]; do sleep 0.1; n=$((n-1)); done
        [ -s "$ONI_LAB/model/port" ] && [ "$(cat "$ONI_LAB/model/port")" = "$port" ] || lab_die 'memory model did not bind'
        exit 0
        ;;
esac
[ "$#" = 2 ] || usage
wm=$1
case $wm in niri|hyprland) ;; *) usage ;; esac
out=$2
case $out in /*) ;; *) out=$PWD/$out ;; esac
out=$(realpath -m "$out")
case $out/ in "$ROOT"/*|"$ONI_LAB"/*) lab_die 'OUTDIR must be outside repository and lab' ;; esac
[ ! -e "$out" ] || [ -z "$(find "$out" -mindepth 1 -maxdepth 1 -print -quit)" ] || lab_die 'OUTDIR is not empty'
for t in espeak-ng paplay parec pactl grim cc python3 tesseract wtype wl-paste; do
    command -v "$t" >/dev/null 2>&1 || lab_die "missing $t"
done
mkdir -p "$out"
finish() {
    st=$?
    trap - EXIT INT TERM HUP
    ONI_LAB_SAVE=$out/logs "$HERE/down.sh" >"$out/down.txt" 2>&1 || st=1
    exit "$st"
}
trap finish EXIT
trap 'exit 130' INT TERM HUP
if [ "$wm" = hyprland ]; then
    "$HERE/up.sh" "$wm" --app --surface --model --hands --outputs 2 >"$out/up.txt" 2>&1
else
    "$HERE/up.sh" "$wm" --app --surface --model --hands >"$out/up.txt" 2>&1
fi
lab_load
"$HERE/window.sh" build >"$out/build.txt" 2>&1
mkdir -p "$ONI_LAB/voice"
"$HERE/voice.sh" fixtures "$ONI_LAB/voice/fixtures" >"$out/fixtures.txt" 2>&1
python3 "$HERE/reflexkit.py" fixtures "$ONI_LAB/voice/reflex" >>"$out/fixtures.txt" 2>&1
python3 "$HERE/handskit.py" fixtures "$ONI_LAB/voice/hands" >>"$out/fixtures.txt" 2>&1
python3 "$HERE/memkit.py" fixtures "$ONI_LAB/voice/memory" >>"$out/fixtures.txt" 2>&1
models=${ONI_LAB_STT_MODEL:-$(python3 "$HERE/voicekit.py" stt-defaults)}
for m in $models; do "$HERE/voice.sh" stage-model "$m" >>"$out/model.txt" 2>&1; done
if ! /usr/bin/whisper-server --help >/dev/null 2>&1; then
    ws=$(cat "${VOICEBENCH:-/dev/shm/hyakki-voicebench}/whisper-server.path")
    "$ws" --help >/dev/null 2>&1 || lab_die 'whisper-server is unavailable'
    ln -sf "$ws" "$ONI_LAB/bin/whisper-server"
fi
"$0" model
"$0" stopkeys >/dev/null
lab_stop oni TERM
"$0" daemon start
python3 "$HERE/memkit.py" phase16 "$ONI_LAB" "$out"
