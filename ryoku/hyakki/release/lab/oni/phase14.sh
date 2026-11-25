#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# phase14.sh niri|hyprland OUTDIR: the Phase 14 browser Hands gate on one
# compositor. Starts its own lab (up.sh --app --surface --model --hands;
# Hyprland with two outputs), the Phase 11 voice graph, chord FIFO and speech
# model, and the Esc stop FIFO the daemon reads in the lab
# (HYAKKI_ONI_STOP_FIFO, its own pipe, never the chord's); drives the real
# daemon, surface, app (Settings > Oni > Hands, found by tesseract in grim
# pictures of the app window and clicked with the virtual pointer, as a user
# would) and CLI (hyakki oni hands open|tabs|stop|status|log) against the
# deterministic loopback model (model.py, mode hands) and the two loopback
# origins of handsfix.py; the Hands browser is the daemon's own Chromium on
# the lab's profile. Tears the lab down with down.sh and leaves only evidence
# in OUTDIR:
#   0 census        tools, surface globals, the refusal stubs, both fixtures'
#                   selftests and origins; oni.hands off, hands.browser
#                   guarded and denied, hands.app and hands.desktop
#                   unavailable; no hands-browser profile, no Chromium, no
#                   listener but the fixtures'; the Phase 11 graph and keys
#   1 refusals      hands off: open refused with its reason and no browser;
#                   inspect off with hands on: refused; the CLI's settings
#                   patch of oni.hands, and its raw authorize, start and
#                   resume, forbidden; the CLI's authorize, start, resume and
#                   forget verbs refused locally (nothing sent); "turn on
#                   hands" by chord, wake and open mic changes nothing; the
#                   app's switch turns it on
#   2 launch        open starts exactly the allowlisted argv (real binary,
#                   the lab profile, --remote-debugging-pipe, a fresh
#                   hyakki-hands-<hex> class, origin A's /contact); no
#                   --remote-debugging-port and no listening socket of the
#                   browser or oni; the window by PID or app id (which held
#                   is recorded); profile 0700, no extension; a second open
#                   reuses the browser
#   3 authorize     before a tab is authorized nothing acts and a voice start
#                   is not claimed; tabs lists origin A and the lab's tab B;
#                   the app's Authorize binds origin A; tab B sees no input
#                   for the whole run
#   4 fill          the app's Start with a goal, its step disclosure allowed
#                   in the app: Name and Message filled in tab A only (the
#                   page's own /state reports), one JSON action per model
#                   turn, fill rows with no words, glow and pill commits
#                   while working, the hands-start motif
#   5 send          Send is handed off, never pressed: a hands.browser firm
#                   local card (h-<n>), phase waiting; a spoken yes by chord,
#                   wake and open mic gets "Use the button for that." and no
#                   act; the CLI's consent answer and a forged oni.hands call
#                   are refused; /submit stays at 0; the lab presses Send
#                   itself with the virtual pointer, /submit lands, a "user"
#                   row, and the done line names the result
#   6 delete        Delete draft is handed off the same way: Not now ends it,
#                   the card's 120 s expiry ends it, an unrelated Press ends
#                   it, /delete stays 0; the lab's own press lands once
#   7 sensitive     the password and card fields are refused before any input
#                   (private-field rows, empty in /state, the canary nowhere);
#                   Pay now is a handoff and /pay stays 0
#   8 away          a foreign window's focus, a tab switch, another origin, a
#                   popup, a dialog and a download each pause with their
#                   word and no act after; spoken and CLI resume refused, the
#                   pill's Resume goes on; closing the tab stops it; the five
#                   minute resume window ends a pause
#   9 stop          Esc (the stop FIFO) during a slow model turn: the
#                   inactive frame and the glow taken down in one handler, the
#                   model request aborted, a stop row (esc) within
#                   hands.stopBudgetMs, no act after; the same from the pill,
#                   the CLI, open-mic "stop", inspect off and a killed
#                   presenter; stop twice harmless; the browser stays open
#   10 protocol     two actions, an invented tool, a plan, a bad permission, a
#                   filler turn and bad JSON (retried once) run nothing and
#                   are refused rows; the hostile page's order stays inside
#                   the fence and changes nothing; repeat halts at 3; endless
#                   ends at the budget
#   11 bounds       10,000 controls and a 5 MiB text node: the control list
#                   and the prompt within hands.page, no listener anywhere
#   12 boundaries   no input-executor name under internal/oni, none in
#                   internal/hands outside internal/hands/input; the unit
#                   test binaries; every daemon child on its allowlist (the
#                   browser's exact argv and its own helpers) and no input
#                   program
#   13 disk         no picture, typed word, goal or page text in any Hyakki
#                   file (the browser profile excepted, a stated limit);
#                   tmp/ empty
#   14 residency    the daemon stays up past its idle exit while the browser
#                   is open; closing the browser lets it exit; hyakki off and a
#                   killed oni leave no Chromium; the app's Forget deletes the
#                   profile and only it
#   15 settlement   measure.sh settled: hands off, authorized and idle,
#                   paused (no glow tick), after stop (no glow surface); the
#                   glow's rate and half strength while a card waits, recorded
#   16 reduced      reduce motion on: one glow commit per output, stop still
#                   one handler
#   17 display      every ledger line names the lab's display
# OUTDIR/phase14.json holds every step's checks, the limits and the
# measurements; crops/ (app and browser pictures the lab read), model/,
# fixtures/ (hands-requests.jsonl, hands.log, refused.log), frames.jsonl,
# measure-*.json, commands.log and logs/ sit beside it. OUTDIR must be outside
# the repository and the lab. Exit 0 only when everything passed. The model
# and the pages are loopback fixtures: they prove the contract, consent,
# routing and stop paths, not model quality. Speech models as for
# phase11.sh. HYAKKI_BIN selects the binaries as for up.sh; feature.test,
# act.test, consent.test, proto.test, loop.test, cdp.test, chrome.test and
# page.test there (go test -c of internal/oni/feature, internal/reflex/act,
# internal/oni/hands/{consent,proto,loop} and internal/hands/{cdp,chrome,page})
# run step 12. Takes about 75 minutes.
#
# Inside a finished lab, for handskit.py:
#   phase14.sh daemon stop|start [HYAKKI_ONI_*=NUMBER...]   stop the oni
#                   daemon (TERM) or start it with the stop FIFO and origin A's
#                   /contact as the Hands start address (lab hooks), plus any
#                   number hooks given (HYAKKI_ONI_HANDS_CARD_MS, _RESUME_MS)
#   phase14.sh stopkeys   make the Esc stop FIFO (keys/stop.fifo)
#   phase14.sh esc EVENT...   write EVENTs into the stop FIFO: down, up, press
#                   (down and up), letter (a key that is not Esc), wait:MS
#   phase14.sh route ROUTE   ask the running app to show ROUTE (hyakki-app
#                   --route, e.g. settings/hands)
#   phase14.sh focus|close ID   focus the window with compositor id ID, or close
#                   it as a user would
#   phase14.sh layout ID   niri: float the app on the left half, giving the
#                   compositor a real reported position for OCR and clicks;
#                   the Hands browser stays right. Hyprland uses two outputs.
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"
ROOT=$(CDPATH='' cd -- "$HERE/../../../../.." && pwd -P)

usage() {
    sed -n '3,119p' "$HERE/phase14.sh" >&2
    exit 2
}
[ "$#" -ge 1 ] || usage

# --- inside a finished lab -----------------------------------------------------

stop_fifo() {
    printf '%s/keys/stop.fifo\n' "$ONI_LAB"
}

hands_a() {
    [ -n "${LAB_HANDS_A:-}" ] || lab_die "no LAB_HANDS_A in lab.env (up.sh --hands)"
    case $LAB_HANDS_A in http://127.0.0.1:[0-9]*) ;; *) lab_die "LAB_HANDS_A is not a loopback origin ($LAB_HANDS_A)" ;; esac
    printf '%s\n' "$LAB_HANDS_A"
}

window_ok() {
    case $1 in '' | *[!0-9a-fx]*) lab_die "not a window id: $1" ;; esac
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
            start)
                shift 2
                for _a in "$@"; do
                    case $_a in
                        *[!A-Z0-9_=]* | *=*=*) lab_die "only HYAKKI_ONI_*=NUMBER hooks may be set (got $_a)" ;;
                        HYAKKI_ONI_[A-Z]*=[0-9]*) ;;
                        *) lab_die "only HYAKKI_ONI_*=NUMBER hooks may be set (got $_a)" ;;
                    esac
                done
                [ -p "$(stop_fifo)" ] || lab_die "no stop FIFO (phase14.sh stopkeys)"
                lab_daemon "HYAKKI_ONI_STOP_FIFO=$(stop_fifo)" "HYAKKI_ONI_HANDS_START_URL=$(hands_a)/contact" "$@" ||
                    lab_die "oni did not listen, see $LAB_LOG/oni.log"
                ;;
            *) usage ;;
        esac
        exit 0
        ;;
    stopkeys)
        [ "$#" = 1 ] || usage
        lab_load
        mkdir -p "$ONI_LAB/keys"
        [ -p "$(stop_fifo)" ] || mkfifo -m 600 "$(stop_fifo)"
        lab_env_put LAB_STOP "$(stop_fifo)"
        stop_fifo
        exit 0
        ;;
    esc)
        [ "$#" -ge 2 ] || usage
        lab_load
        [ -p "$(stop_fifo)" ] || lab_die "no stop FIFO (phase14.sh stopkeys)"
        shift
        exec python3 "$HERE/handskit.py" esc "$(stop_fifo)" "$@"
        ;;
    route)
        [ "$#" = 2 ] || usage
        lab_load
        case $2 in '' | *[!a-z/-]* | /* | */ | *//*) lab_die "ROUTE may only hold lowercase letters, hyphens and inner '/' (got $2)" ;; esac
        # The new socket precedes the surviving app's reconnecting subscription.
        _tries=10
        until lab_app "route-$(lab_rand)" 256M "$HYAKKI_BIN/hyakki-app" --route "$2" >>"$LAB_LOG/route.log" 2>&1 </dev/null; do
            _tries=$((_tries - 1))
            [ "$_tries" -gt 0 ] || lab_die "the running app did not take route $2 after reconnect, see $LAB_LOG/route.log"
            sleep 0.5
        done
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
    layout)
        [ "$#" = 2 ] || usage
        lab_load
        window_ok "$2"
        if [ "$LAB_WM" = niri ]; then
            lab_niri action move-window-to-floating --id "$2" >/dev/null
            lab_niri action set-window-width --id "$2" 928 >/dev/null
            lab_niri action set-window-height --id "$2" 1032 >/dev/null
            lab_niri action move-floating-window --id "$2" --x 16 --y 24 >/dev/null
        fi
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
for t in espeak-ng paplay parec pactl grim cc python3 tesseract wtype; do
    command -v "$t" >/dev/null 2>&1 || lab_die "missing $t"
done
browser=
for b in chromium chromium-browser google-chrome-stable google-chrome brave vivaldi-stable; do
    if command -v "$b" >/dev/null 2>&1; then browser=$b; break; fi
done
[ -n "$browser" ] || lab_die "no Chromium-family browser on PATH (the Hands browser is never installed by the lab)"
python3 -c 'import gi; gi.require_version("Gtk", "4.0"); from gi.repository import Gtk, Gio; import numpy, PIL' 2>/dev/null ||
    lab_die "missing python3 GTK 4 bindings (gi), numpy or PIL"
mkdir -p "$out"

finish() {
    st=$?
    trap - EXIT INT TERM HUP
    if [ -n "${LAB_LOG:-}" ] && [ -d "${ONI_LAB:-}/log" ]; then
        mkdir -p "$out/fixtures"
        for f in hands-requests.jsonl hands.log refused.log route.log; do
            [ ! -f "$ONI_LAB/log/$f" ] || cp "$ONI_LAB/log/$f" "$out/fixtures/"
        done
    fi
    ONI_LAB_SAVE=$out/logs "$HERE/down.sh" >"$out/down.txt" 2>&1 || {
        printf 'phase14.sh: down.sh reported a problem, see %s/down.txt\n' "$out" >&2
        st=1
    }
    cat "$out/down.txt"
    exit "$st"
}
trap finish EXIT
trap 'exit 130' INT TERM HUP

if [ "$wm" = hyprland ]; then
    "$HERE/up.sh" hyprland --app --surface --model --hands --outputs 2 >"$out/up.txt" 2>&1 ||
        { cat "$out/up.txt" >&2; exit 1; }
else
    "$HERE/up.sh" niri --app --surface --model --hands >"$out/up.txt" 2>&1 || { cat "$out/up.txt" >&2; exit 1; }
fi
lab_load
hands_a >/dev/null
"$HERE/window.sh" build >"$out/build.txt" 2>&1 || { cat "$out/build.txt" >&2; exit 1; }
mkdir -p "$ONI_LAB/voice"
"$HERE/voice.sh" fixtures "$ONI_LAB/voice/fixtures" >"$out/fixtures.txt" 2>&1 || { cat "$out/fixtures.txt" >&2; exit 1; }
# Phase 13's kit, which this gate builds on, reads its own speech fixtures
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
printf '{"browser_on_path": "%s"}\n' "$(command -v "$browser")" >>"$out/model.txt"
# the stop FIFO, then a daemon that reads it in the lab (up.sh started one
# without it) and loads origin A's /contact when the Hands browser opens
"$0" stopkeys >/dev/null
lab_stop oni TERM
"$0" daemon start
"$0" route settings/hands
python3 "$HERE/handskit.py" phase14 "$ONI_LAB" "$out"
