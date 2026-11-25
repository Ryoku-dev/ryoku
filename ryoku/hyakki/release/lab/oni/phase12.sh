#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# phase12.sh niri|hyprland OUTDIR: the Phase 12 guide, Keep, replay and
# select-and-ask gate on one compositor. Starts its own lab (up.sh --app
# --surface --model; Hyprland with two outputs), the Phase 11 voice graph,
# chord FIFO and speech model, the GTK 4 guide window (guidewin.py, app id
# dev.ryoku.OniGuide, floated by up.sh's rule), drives the real daemon,
# surface and CLI (hyakki oni guide|keep|skip|keeps|pick) against the
# deterministic loopback model (model.py guide mode), tears the lab down with
# down.sh and leaves only evidence in OUTDIR:
#   0 census        surface globals, tesseract (eng), grim, GTK 4; the guide
#                   window's compositor geometry against its state.json and a
#                   grim capture's markers; guide settings off by default; no
#                   keeps file; the Phase 11 graph, key source and meter
#   1 consent first no, the asker killed and the 120 s expiry send nothing;
#                   disclosure always asks again for the next step's picture
#   2 workflow      one JPEG of the window's size, no tools; the strand lands
#                   on File (ledger and pixels), a click, Next, the changed
#                   screen read, Save, saved.json, the done line, frames in
#                   arrival-gated order
#   3 next, again   Dead: Next on an unchanged screen re-points "Step N ·
#                   again", then explains with the strand and card still out;
#                   an advance waits for the newest strand's arrival
#   4 keep          the offer lapses after the chip's 12 s and writes nothing;
#                   oni keep writes one 0600 entry with no title, typed words
#                   or pixels
#   5 replay        zero model requests, no consent card, the replay kicker,
#                   the strand on File then Save, the replay's closing line; latency
#   6 Save, Save As --variant saveas: replay misses, falls through to a
#                   consented model read; the second miss retires the entry
#   7 sensitive     a PIN goal refused with the private-field line, no
#                   strand; --variant pin over a kept entry refused and retired
#   8 unwind        stop mid-read (slow), detach, window close, Oni off,
#                   presenter lost, the 12-step ceiling, the read timeout
#                   (daemon lab hook, when served), a stale consent answer
#   9 voice first   a Press asks the guide, only a Press answers its consent
#                   and says next; wake and open mic never advance; stop
#   10 select       oni pick: one overlay per output, a click on Export
#                   answers in the bubble, Walk me through it; the PIN refused
#                   before any model call; the repeat guard; cancel unmaps
#   11 hints        off: nothing; on: one hint per kept goal, Skip snoozes,
#                   untouched retracts at 45 s, once per key per day
#   12 injection    hostile replies run nothing, the plan never reaches a
#                   later request (canary), no action tag executed
#   13 read-only    no input-executor name, feature/window/ocr/keep/guide
#                   tests, every daemon child on its allowlist, no picture or
#                   window text file in the lab, tmp/ empty
#   14 reduced      one commit per part per guide frame, arrival at once
#   15 settlement   measure.sh perched as guide-waiting and guide-ended: 0
#                   commits, 0 armed callbacks, no capture process; settle
#                   frames after Next within the spec's maxFrames; on
#                   Hyprland the daemon's wakeups count over the floating
#                   perch's sampler, measured first with no guide
#   16 display      every ledger line names the lab's display
# OUTDIR/phase12.json holds every step's checks, the limits and the
# measurements; guidewin/ (each guide window's state.json and saved.json),
# answers/, crops/, model/, audio/, frames.jsonl, measure-*.json,
# commands.log and logs/ sit beside it. OUTDIR must be outside the
# repository and the lab. Exit 0 only when everything passed. The model is
# the loopback fixture: it proves routing, consent, capture, OCR-verified
# trust, strand landing and model-free replay, not model quality. Speech
# models as for phase11.sh. HYAKKI_BIN selects the binaries as for up.sh;
# feature.test, window.test, ocr.test, keep.test and guide.test there run
# step 13; window.test is compiled from ryoku/wm/oniwindow, the others from
# internal/oni/<pkg>. Takes about 40 minutes.
#
# Inside a finished lab, for guidekit.py:
#   phase12.sh guidewin open VARIANT DIR   start guidewin.py (normal, saveas or
#                   pin) writing state.json and saved.json into DIR (absolute,
#                   outside the lab); print its window record once listed
#   phase12.sh guidewin get|focus|close|kill   its record, focus it, close it
#                   through the compositor, or end the client
#   phase12.sh guidewin move X Y | output OUT   place it (global logical px)
#                   or move it onto another output (Hyprland)
#   phase12.sh daemon [HYAKKI_ONI_GUIDE_*=VALUE...]   restart the oni daemon,
#                   with the guide's lab hooks when given (HYAKKI_ONI_LAB=1)
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"
ROOT=$(CDPATH='' cd -- "$HERE/../../../../.." && pwd -P)
GUIDE_APP=dev.ryoku.OniGuide
GUIDE_TITLE="Oni guide Lindqvist ledger"

usage() {
    sed -n '3,73p' "$HERE/phase12.sh" >&2
    exit 2
}
[ "$#" -ge 1 ] || usage

# --- the guide window, inside a finished lab ---------------------------------

record() {
    lab_state | python3 "$LAB_KIT" window "$GUIDE_APP"
}
field() {
    python3 -c 'import json, sys; v = json.load(sys.stdin)[sys.argv[1]]; print("" if v is None else v)' "$1"
}
wait_record() {
    _n=$1
    while [ "$_n" -gt 0 ]; do
        if _r=$(record 2>/dev/null); then
            case $2 in
                present) printf '%s\n' "$_r"; return 0 ;;
                *) [ "$(printf '%s' "$_r" | python3 "$LAB_KIT" window-check "$2")" = yes ] && { printf '%s\n' "$_r"; return 0; } ;;
            esac
        elif [ "$2" = gone ]; then
            return 0
        fi
        _n=$((_n - 1))
        sleep 0.1
    done
    return 1
}
focus_guide() {
    if [ "$LAB_WM" = niri ]; then
        lab_niri action focus-window --id "$1" >/dev/null
    else
        lab_hyprctl dispatch "hl.dsp.focus({ window = \"address:$1\" })" >>"$LAB_LOG/guidewin.log" 2>&1 || true
    fi
}

guidewin() {
    [ "$#" -ge 1 ] || usage
    action=$1
    shift
    case $action in
        open)
            [ "$#" = 2 ] || usage
            variant=$1 dir=$2
            case $variant in normal | saveas | pin) ;; *) lab_die "VARIANT is normal, saveas or pin" ;; esac
            case $dir in /*) ;; *) lab_die "DIR must be absolute" ;; esac
            case $dir/ in "$ONI_LAB"/* | "$ROOT"/*) lab_die "DIR must be outside the lab and the repository" ;; esac
            ! record >/dev/null 2>&1 || lab_die "the guide window is already open"
            mkdir -p "$dir"
            lab_tool "guidewin" HOME="$ONI_LAB/home" DBUS_SESSION_BUS_ADDRESS=disabled: \
                python3 "$HERE/guidewin.py" "$GUIDE_APP" "$GUIDE_TITLE" "$dir" --variant "$variant" \
                >>"$LAB_LOG/guidewin.log" 2>&1 </dev/null &
            wait_record 150 present >/dev/null || lab_die "the compositor never listed $GUIDE_APP, see $LAB_LOG/guidewin.log"
            _n=100
            while [ ! -s "$dir/state.json" ] && [ "$_n" -gt 0 ]; do
                _n=$((_n - 1))
                sleep 0.1
            done
            [ -s "$dir/state.json" ] || lab_die "guidewin.py wrote no state.json, see $LAB_LOG/guidewin.log"
            r=$(record)
            [ "$(printf '%s' "$r" | field floating)" = True ] ||
                lab_die "the guide window does not float: up.sh lacks the $GUIDE_APP window rule"
            printf '%s\n' "$r"
            ;;
        get)
            [ "$#" = 0 ] || usage
            record || lab_die "the guide window is not open"
            ;;
        focus)
            [ "$#" = 0 ] || usage
            id=$(record | field id) || lab_die "the guide window is not open"
            focus_guide "$id"
            wait_record 30 focused || lab_die "the compositor does not report the guide window focused"
            ;;
        move)
            [ "$#" = 2 ] || usage
            for v in "$1" "$2"; do
                case $v in '' | *[!0-9-]* | ?*-*) lab_die "not an integer: $v" ;; esac
            done
            r=$(record) || lab_die "the guide window is not open"
            id=$(printf '%s' "$r" | field id)
            if [ "$LAB_WM" = niri ]; then
                # shellcheck disable=SC2046
                set -- "$1" "$2" $(lab_state | python3 "$LAB_KIT" output-origin "$(printf '%s' "$r" | field output)")
                lab_niri action move-floating-window --id "$id" -x "$(($1 - $3))" -y "$(($2 - $4))" >/dev/null
            else
                lab_hyprctl dispatch "hl.dsp.window.move({ x = $1, y = $2, window = \"address:$id\" })" \
                    >>"$LAB_LOG/guidewin.log" 2>&1 || true
                if ! wait_record 10 "pos=$1,$2" >/dev/null; then
                    lab_hyprctl dispatch movewindowpixel "exact $1 $2,address:$id" >>"$LAB_LOG/guidewin.log" 2>&1 || true
                fi
            fi
            wait_record 30 "pos=$1,$2" || lab_die "the compositor does not report the guide window at $1,$2"
            ;;
        output)
            [ "$#" = 1 ] || usage
            [ "$LAB_WM" = hyprland ] || lab_die "nested niri has one output"
            id=$(record | field id) || lab_die "the guide window is not open"
            lab_hyprctl dispatch "hl.dsp.window.move({ monitor = \"$1\", window = \"address:$id\" })" \
                >>"$LAB_LOG/guidewin.log" 2>&1 || true
            wait_record 30 "output=$1" || lab_die "the compositor does not report the guide window on $1"
            ;;
        close)
            [ "$#" = 0 ] || usage
            id=$(record | field id) || lab_die "the guide window is not open"
            if [ "$LAB_WM" = niri ]; then
                lab_niri action close-window --id "$id" >/dev/null
            else
                # close() takes the focused window: confirm the focus first
                focus_guide "$id"
                wait_record 30 focused >/dev/null || lab_die "could not focus the guide window to close it"
                lab_hyprctl dispatch 'hl.dsp.window.close()' >>"$LAB_LOG/guidewin.log" 2>&1 || true
            fi
            wait_record 50 gone || lab_die "the compositor still lists the guide window after closing it"
            ;;
        kill)
            [ "$#" = 0 ] || usage
            python3 "$LAB_KIT" stop-role "$LAB_ID" "guidewin-" TERM
            wait_record 50 gone || lab_die "the compositor still lists the guide window"
            ;;
        *) usage ;;
    esac
}

case $1 in
    guidewin)
        shift
        lab_load
        guidewin "$@"
        exit 0
        ;;
    daemon)
        shift
        lab_load
        for _a in "$@"; do
            case $_a in
                *[!A-Z0-9_=]* | *=*=*) lab_die "only HYAKKI_ONI_GUIDE_*=NUMBER hooks may be set (got $_a)" ;;
                HYAKKI_ONI_GUIDE_[A-Z]*=[0-9]*) ;;
                *) lab_die "only HYAKKI_ONI_GUIDE_*=NUMBER hooks may be set (got $_a)" ;;
            esac
        done
        lab_stop oni TERM
        lab_daemon "$@" || lab_die "oni did not listen, see $LAB_LOG/oni.log"
        exit 0
        ;;
esac

# --- the gate -----------------------------------------------------------------

[ "$#" = 2 ] || usage
wm=$1
case $wm in niri | hyprland) ;; *) usage ;; esac
out=$2
case $out in /*) ;; *) out=$PWD/$out ;; esac
case $out/ in "$ROOT"/* | "$ONI_LAB"/*) lab_die "OUTDIR must be outside the repository and the lab" ;; esac
[ ! -e "$out" ] || [ -z "$(ls -A "$out")" ] || lab_die "OUTDIR $out is not empty"
for t in espeak-ng paplay parec pactl tesseract grim python3; do
    command -v "$t" >/dev/null 2>&1 || lab_die "missing $t"
done
tesseract --list-langs 2>/dev/null | grep -qx eng || lab_die "tesseract has no eng traineddata"
python3 -c 'import gi; gi.require_version("Gtk", "4.0"); from gi.repository import Gtk; import numpy, PIL' 2>/dev/null ||
    lab_die "missing python3 GTK 4 bindings (gi), numpy or PIL"
mkdir -p "$out"

finish() {
    st=$?
    trap - EXIT INT TERM HUP
    ONI_LAB_SAVE=$out/logs "$HERE/down.sh" >"$out/down.txt" 2>&1 || {
        printf 'phase12.sh: down.sh reported a problem, see %s/down.txt\n' "$out" >&2
        st=1
    }
    cat "$out/down.txt"
    exit "$st"
}
trap finish EXIT
trap 'exit 130' INT TERM HUP

if [ "$wm" = hyprland ]; then
    "$HERE/up.sh" hyprland --app --surface --model --outputs 2 >"$out/up.txt" 2>&1 || { cat "$out/up.txt" >&2; exit 1; }
else
    "$HERE/up.sh" niri --app --surface --model >"$out/up.txt" 2>&1 || { cat "$out/up.txt" >&2; exit 1; }
fi
"$HERE/window.sh" build >"$out/build.txt" 2>&1 || { cat "$out/build.txt" >&2; exit 1; }
mkdir -p "$ONI_LAB/voice"
"$HERE/voice.sh" fixtures "$ONI_LAB/voice/fixtures" >"$out/fixtures.txt" 2>&1 || { cat "$out/fixtures.txt" >&2; exit 1; }
python3 "$HERE/guidekit.py" fixtures "$ONI_LAB/voice/guide" >>"$out/fixtures.txt" 2>&1 || { cat "$out/fixtures.txt" >&2; exit 1; }
"$HERE/voice.sh" character "${ONI_LAB_CLIP_CHARACTER:-labclips}" >"$out/character.txt" 2>&1 ||
    { cat "$out/character.txt" >&2; exit 1; }
if [ -n "${ONI_LAB_STT_MODEL:-}" ]; then
    models=$ONI_LAB_STT_MODEL
else
    models=$(python3 "$HERE/voicekit.py" stt-defaults)
fi
: >"$out/model.txt"
for m in $models; do
    "$HERE/voice.sh" stage-model "$m" >>"$out/model.txt" 2>&1 || { cat "$out/model.txt" >&2; exit 1; }
done
if ! /usr/bin/whisper-server --help >/dev/null 2>&1; then
    ws=$(cat "${VOICEBENCH:-/dev/shm/hyakki-voicebench}/whisper-server.path" 2>/dev/null) ||
        lab_die "/usr/bin/whisper-server does not start and voicebench.sh stage built none"
    "$ws" --help >/dev/null 2>&1 || lab_die "$ws does not start"
    ln -sf "$ws" "$ONI_LAB/bin/whisper-server"
    printf '{"whisper-server": "%s", "why": "/usr/bin/whisper-server does not start"}\n' "$ws" >>"$out/model.txt"
fi
python3 "$HERE/guidekit.py" phase12 "$ONI_LAB" "$out"
