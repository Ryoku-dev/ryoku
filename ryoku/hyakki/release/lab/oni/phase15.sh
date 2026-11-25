#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# phase15.sh niri|hyprland OUTDIR: the Phase 15 guarded app and separate
# desktop Hands gate on one compositor. The contract it rests on (hooks,
# wire, labels, ledger parts) is the head of appkit.py and
# local://hyakki-p15-lab-contract.md.
#
# Starts its own lab with up.sh --app --surface --model, on Hyprland with two
# outputs. Then it sets up:
#   - the Phase 11 voice graph, chord FIFO and speech model;
#   - the Esc FIFO and the any-key FIFO the daemon reads in the lab
#     (HYAKKI_ONI_STOP_FIFO and HYAKKI_ONI_ANYKEY_FIFO: two pipes of their
#     own, never the chord's, so the daemon never opens /dev/input);
#   - appmodel.py on model.py's port;
#   - appfix.py's catalogue (Kansai Notes and Lab Files) and the
#     HandsFiles folder under the lab HOME's Documents;
#   - a daemon whose separate desktop lives in $ONI_LAB/nest
#     (HYAKKI_ONI_HANDS_NEST_DIR), never in /dev/shm/hyakki-nest-<uid>.
#
# The "live" desktop Hands may act on is the lab's own compositor. The
# separate desktop is the daemon's own weston and niri inside the lab, so
# nothing reaches the user's session. The lab plays the person:
#   - the app (Settings > Oni > Hands, found by tesseract in grim pictures
#     of the app window and clicked with the virtual pointer);
#   - the pill;
#   - the chord, wake and open mic;
#   - the Esc and any-key FIFOs.
# The CLI is driven too (hyakki oni hands status|stop|tasks|log). The
# fixtures are appwin.py's windows, all on the lab desktop:
#   - target: dev.ryoku.OniTarget, floating, a text view and Mark;
#   - tiled: dev.ryoku.OniTiled, tiled on niri, the keys-only row;
#   - pin: the target as "Enter PIN";
#   - sentinel: dev.ryoku.OniSentinel, which logs every input event it gets.
# The model is appmodel.py.
#
# Tears the lab down with down.sh and leaves only evidence in OUTDIR. The
# steps, all through appkit.py's Phase15 class:
#   0 census        tools (weston, niri, grim, tesseract, wtype, GTK 4,
#                   espeak-ng), the fixtures' selftests, oni.hands off,
#                   hands.app and hands.desktop guarded and denied, no nest
#                   dir, no weston or niri child, no journal, no /dev/input
#                   fd in the daemon, the nested desktop's missing list empty
#   1 refusals      stage off and inspect off refuse; the CLI's settings
#                   patch and its raw windows/select/start/resume/forget/
#                   clear are forbidden, its select/start/resume/forget verbs
#                   refused locally; "turn on hands" by voice changes
#                   nothing; only the app's switches move the stage, and one
#                   on turns the others off
#   2 desktop up    the desktop stage's Start runs weston and niri with the
#                   exact argv in $ONI_LAB/nest; their environment (read
#                   while alive) names only the nested dir and display; no
#                   window appears on the lab desktop; the task's end, Stop,
#                   daemon TERM and daemon KILL each end it (KILL: gone by
#                   the next boot's Sweep)
#   3 desktop type  open_app opens Kansai Notes on the separate desktop from
#                   the catalogue's argv; "hello lantern" lands there and is
#                   read back yes; the sentinel, focused on the lab desktop,
#                   logs nothing
#   4 desktop click a click on Mark lands on the separate desktop; the
#                   sentinel under the lab pointer logs nothing; no glow on
#                   any live output
#   5 window pick   the app's picker lists the target, marks the tiled
#                   window (niri) keys only, and never offers the Hyakki app
#                   or Oni's surface; nothing starts before a pick; a pick
#                   binds the target's app id
#   6 window type   typed chunk by chunk, at most 40 runes each, each with a
#                   check before and after, and read back yes; the daemon's
#                   only input program is the exact wtype argv on the lab
#                   display; the sentinel logs nothing
#   7 theft pauses  a retitle (title), a move (geometry), another window's
#                   focus (focus), a key on the any-key FIFO (key), each with
#                   no input after; closing the target ends it (closed); only
#                   the pill or the app resumes; the resume window ends a
#                   pause
#   8 private       the Hyakki app and Oni's surface are never a target; the
#                   PIN window is refused; a secret-shaped text is refused
#                   before any input; the app taking focus mid-run pauses
#   9 risky keys    Enter, ctrl+w, Delete, alt+F4 and ctrl+v: a handoff or
#                   refusal and no wtype of them; a spoken yes and the CLI's
#                   yes never press
#   10 stop         Esc in a slow turn and while typing; the pill, the CLI,
#                   open-mic "stop" and inspect off; a killed presenter
#                   pauses (presenter); stop twice harmless; a stopped
#                   nested task ends its desktop
#   11 shown not run find_files and show_file in HandsFiles: every kind
#                   (exec bit, #!, .sh, .desktop, .AppImage, .run, .txt) is
#                   shown by its folder through Lab Files; no ran- marker,
#                   no child running a fixture file
#   12 restart hold kill -9 mid-task (window and desktop): the journal row
#                   interrupted, no input program, pointer or nested child
#                   started after the restart, voice and the CLI cannot
#                   start it, the app's Start again can; the journal 0600
#                   holds no goal, title or typed word
#   13 queue        one running plus queueMax up next, a full queue refused,
#                   the next starts only from the app; a Hands error (model
#                   500) is not retried
#   14 approvals    a teams.agent_* call and an approval answer from the CLI
#                   are forbidden, approval_decide refused; none of it
#                   starts a task, a card or an input; no approval frame
#                   names the desktop (the hosted member itself is a limit:
#                   creating a team is the app's alone)
#   15 exclusive    the app stage refuses the desktop stage's work and the
#                   reverse; the browser stage refuses both; the desktop
#                   switch never turns on as a side effect
#   16 protocol     two actions, an invented tool, a plan, a bad permission,
#                   a tool foreign to the scope, bad JSON, the screen's
#                   "ignore your rules" (fenced), repeat, endless, a done
#                   with nothing read back
#   17 boundaries   input-tool names only in internal/hands/input and
#                   dictate/insert; the unit test binaries; every daemon
#                   child on its allowlist
#   18 disk         no picture, OCR text, title, goal or typed word in any
#                   Hyakki file; the ledger and the journal hold codes only;
#                   tmp/ empty
#   19 init and compositor   the stubs refuse; the same code on both hosts;
#                   the separate desktop is niri on weston under either
#   20 settlement   measure.sh: hands off, app stage with a pick and no
#                   task, paused, after stop (no key reader FIFO open, no
#                   nested child, glow hidden)
#   21 reduced      reduce motion: one glow commit per output; stop still one
#                   handler
#   22 display      every ledger line names the lab's display
#
# The evidence in OUTDIR:
#   - phase15.json holds every step's checks, the limits and the
#     measurements;
#   - beside it: crops/, model/, fixtures/ (target.json, desk.json,
#     files.jsonl, sentinel.jsonl, the journal copy), frames.jsonl,
#     measure-*.json, commands.log and logs/.
# OUTDIR must be outside the repository and the lab. Exit 0 only when
# everything passed. HYAKKI_BIN selects the binaries as for up.sh; step 17
# runs the test binaries named in appkit.py's TEST_PKGS15, from there (go
# test -c). Takes about 90 minutes.
#
# Inside a finished lab, for appkit.py:
#   phase15.sh daemon stop|kill|start [HYAKKI_ONI_*=NUMBER...]
#                   stop the oni daemon (TERM), kill it (KILL, no clean-up),
#                   or start it with both key FIFOs and the nest dir, plus
#                   any number hooks given (HYAKKI_ONI_HANDS_CARD_MS,
#                   _RESUME_MS)
#   phase15.sh keys
#                   make the Esc and any-key FIFOs (keys/stop.fifo,
#                   keys/anykey.fifo)
#   phase15.sh esc|anykey EVENT...
#                   write EVENTs into that FIFO: down, up, press, letter,
#                   wait:MS
#   phase15.sh route ROUTE
#                   ask the running app to show ROUTE
#   phase15.sh win target|tiled|pin|sentinel
#                   open that fixture window on the lab desktop and print its
#                   record
#   phase15.sh retitle TEXT
#                   the target's command FIFO: retitle it (title theft)
#   phase15.sh focus|close ID
#                   focus or close the window with compositor id ID
#   phase15.sh move ID X Y
#                   move the floating window ID's top-left to global X Y
#   phase15.sh files
#                   (re)install appfix.py's catalogue and HandsFiles
#   phase15.sh model
#                   swap model.py for appmodel.py on the same port
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"
ROOT=$(CDPATH='' cd -- "$HERE/../../../../.." && pwd -P)

usage() {
    sed -n '3,161p' "$HERE/phase15.sh" >&2
    exit 2
}
[ "$#" -ge 1 ] || usage

# --- inside a finished lab -----------------------------------------------------

stop_fifo() { printf '%s/keys/stop.fifo\n' "$ONI_LAB"; }
anykey_fifo() { printf '%s/keys/anykey.fifo\n' "$ONI_LAB"; }
nest_dir() { printf '%s/nest\n' "$ONI_LAB"; }

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

window_ok() {
    case $1 in '' | *[!0-9a-fx]*) lab_die "not a window id: $1" ;; esac
}

int_ok() {
    case $1 in '' | *[!0-9-]* | ?*-*) lab_die "not an integer: $1" ;; esac
}

# win_record APP_ID: the window record of the one window with APP_ID, if any
win_record() {
    lab_state | python3 -c '
import json, sys
ws = [w for w in json.load(sys.stdin)["windows"] if w.get("app_id") == sys.argv[1]]
if len(ws) != 1:
    sys.exit(1)
print(json.dumps(ws[0]))' "$1"
}

win() {
    mkdir -p "$ONI_LAB/out"
    out=$ONI_LAB/out
    case $1 in
        target)
            [ -p "$out/target.cmd" ] || mkfifo -m 600 "$out/target.cmd"
            app=dev.ryoku.OniTarget
            set -- target --app-id "$app" --title "Kansai notes" --state "$out/target.json" --cmd "$out/target.cmd"
            ;;
        tiled)
            app=dev.ryoku.OniTiled
            set -- target --app-id "$app" --title "Tiled notes" --state "$out/tiled.json"
            ;;
        pin)
            app=dev.ryoku.OniTarget
            set -- target --app-id "$app" --title "Kansai notes" --state "$out/target.json" --variant pin
            ;;
        sentinel)
            app=dev.ryoku.OniSentinel
            set -- sentinel --log "$out/sentinel.jsonl"
            ;;
        *) usage ;;
    esac
    ! win_record "$app" >/dev/null 2>&1 || lab_die "a $app window is already open"
    lab_run "appwin-$(lab_rand)" 256M env -i PATH=/usr/bin:/bin HOME="$ONI_LAB/home" LANG=C.UTF-8 \
        XDG_RUNTIME_DIR="$LAB_WRT" WAYLAND_DISPLAY="$LAB_DISPLAY" GIO_USE_VFS=local \
        python3 "$HERE/appwin.py" "$@" >>"$LAB_LOG/appwin.log" 2>&1 </dev/null &
    n=150
    while [ "$n" -gt 0 ]; do
        if win_record "$app" 2>/dev/null; then
            return 0
        fi
        n=$((n - 1))
        sleep 0.1
    done
    lab_die "no $app window, see $LAB_LOG/appwin.log"
}

model_swap() {
    port=$(cat "$ONI_LAB/model/port" 2>/dev/null) || lab_die "no model port file (up.sh --model)"
    case $port in '' | *[!0-9]*) lab_die "the model port file holds $port" ;; esac
    python3 "$LAB_KIT" stop-role "$LAB_ID" model TERM || lab_die "model.py did not stop"
    rm -f "$ONI_LAB/model/port"
    lab_run "model-app-$(lab_rand)" 256M env -i PATH=/usr/bin:/bin python3 "$HERE/appmodel.py" serve \
        --port-file "$ONI_LAB/model/port" --log "$LAB_LOG/model-requests.jsonl" \
        --manifest "$ONI_LAB/build/notes.json" --port "$port" >>"$LAB_LOG/model.log" 2>&1 </dev/null &
    lab_wait 150 test -s "$ONI_LAB/model/port" || lab_die "appmodel.py did not listen, see $LAB_LOG/model.log"
    [ "$(cat "$ONI_LAB/model/port")" = "$port" ] || lab_die "appmodel.py took another port than model.py's"
}

case $1 in
    ui)
        [ "$#" = 2 ] || usage
        exec "$HERE/phase13.sh" ui "$2"
        ;;
    daemon)
        [ "$#" -ge 2 ] || usage
        lab_load
        case $2 in
            stop) [ "$#" = 2 ] || usage; lab_stop oni TERM ;;
            kill) [ "$#" = 2 ] || usage; lab_stop oni KILL ;;
            start)
                shift 2
                for _a in "$@"; do
                    case $_a in
                        *[!A-Z0-9_=]* | *=*=*) lab_die "only HYAKKI_ONI_*=NUMBER hooks may be set (got $_a)" ;;
                        HYAKKI_ONI_[A-Z]*=[0-9]*) ;;
                        *) lab_die "only HYAKKI_ONI_*=NUMBER hooks may be set (got $_a)" ;;
                    esac
                done
                [ -p "$(stop_fifo)" ] && [ -p "$(anykey_fifo)" ] || lab_die "no key FIFOs (phase15.sh keys)"
                lab_daemon "HYAKKI_ONI_STOP_FIFO=$(stop_fifo)" "HYAKKI_ONI_ANYKEY_FIFO=$(anykey_fifo)" \
                    "HYAKKI_ONI_HANDS_NEST_DIR=$(nest_dir)" "$@" ||
                    lab_die "oni did not listen, see $LAB_LOG/oni.log"
                ;;
            *) usage ;;
        esac
        exit 0
        ;;
    keys)
        [ "$#" = 1 ] || usage
        lab_load
        mkdir -p "$ONI_LAB/keys"
        [ -p "$(stop_fifo)" ] || mkfifo -m 600 "$(stop_fifo)"
        [ -p "$(anykey_fifo)" ] || mkfifo -m 600 "$(anykey_fifo)"
        lab_env_put LAB_STOP "$(stop_fifo)"
        lab_env_put LAB_ANYKEY "$(anykey_fifo)"
        stop_fifo
        anykey_fifo
        exit 0
        ;;
    esc | anykey)
        [ "$#" -ge 2 ] || usage
        lab_load
        if [ "$1" = esc ]; then f=$(stop_fifo); else f=$(anykey_fifo); fi
        [ -p "$f" ] || lab_die "no FIFO $f (phase15.sh keys)"
        shift
        exec python3 "$HERE/handskit.py" esc "$f" "$@"
        ;;
    route)
        exec "$HERE/phase14.sh" "$@"
        ;;
    win)
        [ "$#" = 2 ] || usage
        lab_load
        win "$2"
        exit 0
        ;;
    retitle)
        [ "$#" = 2 ] || usage
        lab_load
        case $2 in '' | *[!A-Za-z0-9\ .,-]*) lab_die "TEXT may hold letters, digits, spaces and .,- only" ;; esac
        [ -p "$ONI_LAB/out/target.cmd" ] || lab_die "no target window command FIFO"
        printf 'title %s\n' "$2" >"$ONI_LAB/out/target.cmd"
        exit 0
        ;;
    focus | close)
        [ "$#" = 2 ] || usage
        lab_load
        window_ok "$2"
        if [ "$LAB_WM" = niri ]; then
            if [ "$1" = focus ]; then
                lab_niri action focus-window --id "$2" >/dev/null
            else
                lab_niri action close-window --id "$2" >/dev/null
            fi
        else
            lab_hyprctl dispatch "hl.dsp.focus({ window = \"address:$2\" })" >>"$LAB_LOG/window.log" 2>&1
            if [ "$1" = close ]; then
                sleep 0.3
                lab_hyprctl dispatch 'hl.dsp.window.close()' >>"$LAB_LOG/window.log" 2>&1
            fi
        fi
        exit 0
        ;;
    move)
        [ "$#" = 4 ] || usage
        lab_load
        window_ok "$2"
        int_ok "$3"
        int_ok "$4"
        if [ "$LAB_WM" = niri ]; then
            outname=$(lab_state | python3 -c '
import json, sys
ws = [w for w in json.load(sys.stdin)["windows"] if str(w.get("id")) == sys.argv[1]]
print(ws[0]["output"] if ws else "")' "$2")
            [ -n "$outname" ] || lab_die "no window $2"
            # shellcheck disable=SC2046
            set -- "$2" "$3" "$4" $(lab_state | python3 "$LAB_KIT" output-origin "$outname")
            lab_niri action move-floating-window --id "$1" -x "$(($2 - $4))" -y "$(($3 - $5))" >/dev/null
        else
            lab_hyprctl dispatch "hl.dsp.window.move({ x = $3, y = $4, window = \"address:$2\" })" \
                >>"$LAB_LOG/window.log" 2>&1 || true
        fi
        exit 0
        ;;
    files)
        [ "$#" = 1 ] || usage
        lab_load
        mkdir -p "$ONI_LAB/out"
        python3 "$HERE/appfix.py" install --data-home "$ONI_LAB/data" --home "$ONI_LAB/home" --out "$ONI_LAB/out" \
            --here "$HERE"
        exit 0
        ;;
    model)
        [ "$#" = 1 ] || usage
        lab_load
        model_swap
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
for t in espeak-ng paplay parec pactl grim cc python3 tesseract wtype weston niri; do
    command -v "$t" >/dev/null 2>&1 || lab_die "missing $t"
done
python3 -c 'import gi; gi.require_version("Gtk", "4.0"); from gi.repository import Gtk, Gio; import numpy, PIL' 2>/dev/null ||
    lab_die "missing python3 GTK 4 bindings (gi), numpy or PIL"
mkdir -p "$out"

finish() {
    st=$?
    trap - EXIT INT TERM HUP
    if [ -n "${LAB_LOG:-}" ] && [ -d "${ONI_LAB:-}/log" ]; then
        mkdir -p "$out/fixtures"
        for f in refused.log route.log appwin.log; do
            [ ! -f "$ONI_LAB/log/$f" ] || cp "$ONI_LAB/log/$f" "$out/fixtures/"
        done
        for f in target.json tiled.json desk.json files.jsonl sentinel.jsonl; do
            [ ! -f "$ONI_LAB/out/$f" ] || cp "$ONI_LAB/out/$f" "$out/fixtures/"
        done
        [ ! -f "$ONI_LAB/state/hyakki/oni-hands-tasks.json" ] ||
            cp "$ONI_LAB/state/hyakki/oni-hands-tasks.json" "$out/fixtures/journal.json"
    fi
    ONI_LAB_SAVE=$out/logs "$HERE/down.sh" >"$out/down.txt" 2>&1 || {
        printf 'phase15.sh: down.sh reported a problem, see %s/down.txt\n' "$out" >&2
        st=1
    }
    cat "$out/down.txt"
    exit "$st"
}
trap finish EXIT
trap 'exit 130' INT TERM HUP

if [ "$wm" = hyprland ]; then
    "$HERE/up.sh" hyprland --app --surface --model --outputs 2 >"$out/up.txt" 2>&1 ||
        { cat "$out/up.txt" >&2; exit 1; }
else
    "$HERE/up.sh" niri --app --surface --model >"$out/up.txt" 2>&1 || { cat "$out/up.txt" >&2; exit 1; }
fi
lab_load
"$HERE/window.sh" build >"$out/build.txt" 2>&1 || { cat "$out/build.txt" >&2; exit 1; }
mkdir -p "$ONI_LAB/voice"
"$HERE/voice.sh" fixtures "$ONI_LAB/voice/fixtures" >"$out/fixtures.txt" 2>&1 || { cat "$out/fixtures.txt" >&2; exit 1; }
# the earlier kits this gate builds on read their own speech fixtures
python3 "$HERE/reflexkit.py" fixtures "$ONI_LAB/voice/reflex" >>"$out/fixtures.txt" 2>&1 ||
    { cat "$out/fixtures.txt" >&2; exit 1; }
python3 "$HERE/handskit.py" fixtures "$ONI_LAB/voice/hands" >>"$out/fixtures.txt" 2>&1 ||
    { cat "$out/fixtures.txt" >&2; exit 1; }
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
# appmodel.py on model.py's port, the catalogue and HandsFiles, both key
# FIFOs, then a daemon that reads them (up.sh started one without them)
model_swap
"$0" files >"$out/files.txt" 2>&1 || { cat "$out/files.txt" >&2; exit 1; }
"$0" keys >/dev/null
lab_stop oni TERM
"$0" daemon start
"$0" route settings/hands
python3 "$HERE/appkit.py" phase15 "$ONI_LAB" "$out"
