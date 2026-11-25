#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# phase13.sh niri|hyprland OUTDIR: the Phase 13 quick-command, reminder,
# routine, note and typed-decision gate on one compositor. Starts its own lab
# (up.sh --app --surface --model --decisions; Hyprland with two outputs), the
# Phase 11 voice graph, chord FIFO and speech model, the lab's launcher
# catalogue (launchfix.py: "Lindqvist Notes" and "Oni Lab Browser", the
# https default), the MPRIS player (mprisfix.py) and the notification server
# (notifyfix.py) on the lab's private bus, drives the real daemon, surface and
# CLI (hyakki oni reminders|routines|reflex log|keep|skip, hyakki routines
# wake, hyakki service routines, hyakki memory list, hyakki off) by voice
# against the deterministic loopback model and decision fixture (model.py
# with decidefix.py), tears the lab down with down.sh and leaves only evidence
# in OUTDIR:
#   0 census        tools, surface globals, the systemd-run and refusal
#                   stubs, the catalogue and the bus fixtures; reflex and its
#                   four switches off, no reminders or routines file, the
#                   decision route seeded local and loopback, the spec's
#                   reflex numbers; the Phase 11 graph, key source and meter
#   1 off is zero   reflex off: "open Lindqvist notes" is an ordinary Ask (one
#                   model request), nothing launched, no row, no file; the
#                   five reflex rows denied
#   2 launch        a Press opens the notes window (compositor-listed) once,
#                   one ack of at most five words, one done row; the child is
#                   the catalogue's argv, detached (scope=fallback, and with a
#                   systemd marker the refused systemd-run is followed by the
#                   same argv bare); it keeps the lab's display and bus, and
#                   outlives oni's TERM; open mic opens nothing unless Oni is
#                   named
#   3 tabs          "open a new tab" raises tabs.json's count by one through
#                   the browser's own launcher (about:blank); "next tab" and
#                   "close the tab" say "Can't do that here." and change
#                   nothing; no input program ever runs
#   4 window verbs  each verb only where the provider honestly supports it:
#                   niri maximizes the focused column (geometry changes);
#                   snaps without the Ryoku seam, Hyprland's verbs and
#                   minimize everywhere say so with an "unsupported" row
#   5 audio         the lab sink: volume down and up by the 5 % step, full
#                   refused at 100 %, mute and unmute; "pause the music" is
#                   one MPRIS PlayPause; the graph still has no device; a
#                   hidden Oni announces through the notification server
#   6 timers and reminders  with reflex off a five second timer still rings
#                   (voice alone): bubble frame, ledger bubble, pixels and
#                   speech in the recorded PCM; reminders need the switch;
#                   "remind me in ten seconds to stretch" rings with its words;
#                   bare "remind me" is an Ask; list, cancel, eight days
#                   refused; one due while the daemon was down rings once,
#                   late, then is done; the file is 0600 with no transcript
#   7 zero at rest  idle exit at one minute: with the wake unit refused the
#                   daemon stays up while a reminder is pending and oni
#                   reminders reports noWake; with the accepting stub the
#                   enable and disable requests are logged and a reminder 40
#                   minutes off lets the daemon exit; hyakki routines wake
#                   starts it only for a reminder due within the horizon, not
#                   with reflex or Hyakki off; the runit worker (hyakki service
#                   routines) does the same; hyakki off reaches only stubs
#   8 storm         the 43-repeat utterance launches at most once; ten clauses
#                   run at most six; the per-minute cap; a second launch inside
#                   4 s is a cooldown row; a dangling "open" completes within
#                   6 s and not after; "I insert a table" opens nothing;
#                   "salt and pepper" is one whole search
#   9 forbidden     send, buy, delete, submit: no action, launch or input
#                   program, each an ordinary Ask; reflex.test's registry
#                   invariant
#   10 routines     three commands within 20 s offer once; open-mic and wake
#                   "yes" save nothing; expiry saves nothing; a pressed "yes"
#                   saves (0600, at most 20 x 8, no words); no second offer in
#                   the run; "run my routine" replays with zero model and
#                   decision requests; a removed app is skipped with its line
#   11 notes        memory on: the note is in hyakki memory list; memory off:
#                   "Memory is off."; a password refused before memory; no
#                   notes file anywhere
#   12 decisions, local  a phrase the rules miss is classified by the local
#                   route and run; unsure, fail and slow fall through to the
#                   Ask within the budget; hostile runs nothing; requests hold
#                   only the utterance and the registry's labels; loopback
#   13 decisions, hosted  no account: the Ember route and automatic with
#                   allowHosted make zero hosted decisions, show no card and
#                   never reach the local fixture for Ember
#   14 self-commands  move, bigger, home and sounds off change placement and
#                   settings; turning open mic, wake or hands on is refused and
#                   the settings file is unchanged; wake and open mic never
#                   flip a setting
#   15 read-only    no input-executor name under internal/oni or
#                   internal/reflex; no third-party name in reflex copy;
#                   feature.test, reflex.test, remind.test, routine.test and
#                   act.test; every daemon child on its allowlist and every
#                   launched app through the lab launcher (scope=fallback);
#                   Hands off and unavailable
#   16 disk         no audio, transcript or window-text file; the reminders and
#                   routines files hold nothing beyond their own words; tmp/
#                   empty
#   17 reduced      a reminder's bubble commits at most once per part
#   18 settlement   measure.sh settled: reflex on and nothing pending, one
#                   reminder pending (resident), reflex off
#   19 display      every ledger line names the lab's display; launched apps
#                   got the lab's display
# OUTDIR/phase13.json holds every step's checks, the limits and the
# measurements; audio/, crops/, model/ (chat and decision requests),
# fixtures/ (tabs.json, tabs-requests.jsonl, mpris.jsonl, notify.jsonl,
# wake.log, refused.log), frames.jsonl, measure-*.json, commands.log and logs/
# sit beside it. OUTDIR must be outside the repository and the lab. Exit 0
# only when everything passed. The model and the decision service are
# loopback fixtures: they prove routing, thresholds and fall-through, not
# model quality. Speech models as for phase11.sh. HYAKKI_BIN selects the
# binaries as for up.sh; feature.test, reflex.test, remind.test, routine.test
# and act.test there (go test -c of internal/oni/feature, internal/oni/reflex,
# internal/oni/remind, internal/oni/routine and internal/reflex/act) run steps
# 9 and 15. Takes about 50 minutes. ONI_LAB_P13_STEPS=6,7 runs step 0 and
# the listed steps only, to retry them after a fix; the others are recorded
# as skipped, so such a run never passes.
#
# Inside a finished lab, for reflexkit.py:
#   phase13.sh daemon stop|start [HYAKKI_ONI_*=NUMBER...]   stop the oni
#                   daemon (TERM) or start it, with lab hooks when given
#   phase13.sh fixtures start|stop   mprisfix.py and notifyfix.py on the lab bus
#   phase13.sh app install|remove   the launcher catalogue (launchfix.py);
#                   remove takes dev.ryoku.OniNotes away
#   phase13.sh wakestub accept|refuse   bin/ryoku-host as the stub that accepts
#                   exactly the wake unit's enable and disable (logged to
#                   log/wake.log, exit 0), or back to the refusal stub
#   phase13.sh scope on|off   create or remove the systemd user manager's
#                   marker in the lab runtime (rt/systemd/private, an empty
#                   file), so the launcher tries bin/systemd-run (the refusal
#                   stub) first; refused unless that stub is the lab's
#   phase13.sh worker start|stop   hyakki service routines (the runit worker)
#   phase13.sh ui close|open   close/reopen only this lab's Hyakki app client
#                   (pause the frame watch as well for idle and wake proofs)
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"
ROOT=$(CDPATH='' cd -- "$HERE/../../../../.." && pwd -P)
NOTES_APP=dev.ryoku.OniNotes

usage() {
    sed -n '3,128p' "$HERE/phase13.sh" >&2
    exit 2
}
[ "$#" -ge 1 ] || usage

# --- inside a finished lab -----------------------------------------------------

wait_line() { # FILE WORD TENTHS
    _n=$3
    while [ "$_n" -gt 0 ]; do
        grep -qx "$2" "$1" 2>/dev/null && return 0
        _n=$((_n - 1))
        sleep 0.1
    done
    return 1
}

fixtures() {
    case $1 in
        start)
            mkdir -p "$ONI_LAB/out"
            lab_app mpris 128M python3 "$HERE/mprisfix.py" serve --log "$LAB_LOG/mpris.jsonl" \
                >"$LAB_LOG/mpris.log" 2>&1 </dev/null &
            wait_line "$LAB_LOG/mpris.log" ready 100 || lab_die "mprisfix.py never owned its name, see $LAB_LOG/mpris.log"
            lab_app notify 128M python3 "$HERE/notifyfix.py" serve --log "$LAB_LOG/notify.jsonl" \
                >"$LAB_LOG/notify.log" 2>&1 </dev/null &
            wait_line "$LAB_LOG/notify.log" ready 100 || lab_die "notifyfix.py never owned its name, see $LAB_LOG/notify.log"
            ;;
        stop)
            python3 "$LAB_KIT" stop-role "$LAB_ID" mpris TERM
            python3 "$LAB_KIT" stop-role "$LAB_ID" notify TERM
            ;;
        *) usage ;;
    esac
}

app() {
    case $1 in
        install)
            mkdir -p "$ONI_LAB/out"
            python3 "$HERE/launchfix.py" install --data-home "$ONI_LAB/data" --config-home "$ONI_LAB/cfg" \
                --out "$ONI_LAB/out" >>"$LAB_LOG/launchfix.log" 2>&1 ||
                lab_die "launchfix.py install failed, see $LAB_LOG/launchfix.log"
            ;;
        remove)
            python3 "$HERE/launchfix.py" remove --data-home "$ONI_LAB/data" "$NOTES_APP" >>"$LAB_LOG/launchfix.log" 2>&1 ||
                lab_die "launchfix.py remove failed, see $LAB_LOG/launchfix.log"
            ;;
        *) usage ;;
    esac
}

# The accepting wake stub: built from this source once per lab into
# $ONI_LAB/stub, never into the repository. It answers exactly the wake
# unit's enable and disable through the Ryoku bridge
# (ryoku-host svc --user enable|disable --now oni-routines.timer|oni-routines)
# with exit 0 and a log/wake.log line, and refuses everything else as
# refuse.c does.
build_wakestub() {
    mkdir -p "$ONI_LAB/stub"
    cat >"$ONI_LAB/stub/wakeok.c" <<'EOF'
#include <limits.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>
int main(int argc, char **argv) {
    char exe[PATH_MAX];
    ssize_t n = readlink("/proc/self/exe", exe, sizeof exe - 1);
    exe[n > 0 ? n : 0] = '\0';
    const char *name = strrchr(exe, '/');
    name = name ? name + 1 : (argc > 0 ? argv[0] : "?");
    int ok = strcmp(name, "ryoku-host") == 0 && argc == 6 && strcmp(argv[1], "svc") == 0 &&
             strcmp(argv[2], "--user") == 0 && (strcmp(argv[3], "enable") == 0 || strcmp(argv[3], "disable") == 0) &&
             strcmp(argv[4], "--now") == 0 &&
             (strcmp(argv[5], "oni-routines.timer") == 0 || strcmp(argv[5], "oni-routines") == 0);
    FILE *log = fopen(ok ? WAKE_LOG : REFUSED_LOG, "a");
    if (log) {
        fprintf(log, ok ? "wake: %s " : "refused: %s ", name);
        for (int i = 1; i < argc; i++)
            fprintf(log, i > 1 ? " %s" : "%s", argv[i]);
        fputc('\n', log);
        fclose(log);
    }
    return ok ? 0 : 1;
}
EOF
    lab_run "cc-$(lab_rand)" 256M cc -O2 -Wall -Wextra -Werror -DREFUSED_LOG="\"$LAB_LOG/refused.log\"" \
        -DWAKE_LOG="\"$LAB_LOG/wake.log\"" -o "$ONI_LAB/stub/wakeok" "$ONI_LAB/stub/wakeok.c" ||
        lab_die "the accepting wake stub did not build"
    rm -f "$ONI_LAB/stub/wakeok.c"
}

wakestub() {
    case $1 in
        accept) [ -x "$ONI_LAB/stub/wakeok" ] || lab_die "no accepting wake stub (the gate builds it)"; src=$ONI_LAB/stub/wakeok ;;
        refuse) src=$ONI_LAB/bin/ryoku ;;
        *) usage ;;
    esac
    cp "$src" "$ONI_LAB/bin/.ryoku-host.new"
    chmod 755 "$ONI_LAB/bin/.ryoku-host.new"
    mv -f "$ONI_LAB/bin/.ryoku-host.new" "$ONI_LAB/bin/ryoku-host"
}

# bin/systemd-run must be the lab's refusal stub before anything may make the
# launcher believe a user manager runs: the real tool would reach the live one.
scope_stub_ok() {
    [ -f "$ONI_LAB/bin/systemd-run" ] && [ ! -L "$ONI_LAB/bin/systemd-run" ] || return 1
    _before=$(wc -l <"$LAB_LOG/refused.log" 2>/dev/null || echo 0)
    if "$ONI_LAB/bin/systemd-run" --lab-census >/dev/null 2>&1; then
        return 1
    fi
    _after=$(wc -l <"$LAB_LOG/refused.log" 2>/dev/null || echo 0)
    [ "$_after" -gt "$_before" ] && tail -n 1 "$LAB_LOG/refused.log" | grep -qx 'refused: systemd-run --lab-census'
}

scope() {
    case $1 in
        on)
            scope_stub_ok || lab_die "bin/systemd-run is not the lab's refusal stub (up.sh builds scoperefuse.c)"
            mkdir -p "$LAB_RT/systemd"
            : >"$LAB_RT/systemd/private"
            ;;
        off)
            rm -f "$LAB_RT/systemd/private"
            rmdir "$LAB_RT/systemd" 2>/dev/null || true
            ;;
        *) usage ;;
    esac
}

worker() {
    case $1 in
        start)
            lab_app "worker-$(lab_rand)" 256M "$HYAKKI_BIN/hyakki" service routines >>"$LAB_LOG/worker.log" 2>&1 </dev/null &
            ;;
        stop) python3 "$LAB_KIT" stop-role "$LAB_ID" "worker-" TERM ;;
        *) usage ;;
    esac
}

# The app maintains a daemon connection and reconnects after shutdown. Idle
# and wake proofs must remove that client, not just pause `oni watch`.
# lab_stop is scoped by LAB_ID; this never closes the user's Hyakki window.
ui() {
    case $1 in
        close) lab_stop hyakki-app TERM ;;
        open)
            if [ -z "$(python3 "$LAB_KIT" pids "$LAB_ID" hyakki-app)" ]; then
                lab_app app 1536M "$HYAKKI_BIN/hyakki-app" --size 1280x800 \
                    >>"$LAB_LOG/hyakki-app.log" 2>&1 </dev/null &
            fi
            _n=100
            while [ "$_n" -gt 0 ]; do
                if lab_state | python3 -c 'import json,sys; s=json.load(sys.stdin); sys.exit(not any(w.get("app_id") == "dev.ryoku.Hyakki" for w in s["windows"]))'; then
                    return 0
                fi
                sleep 0.1
                _n=$((_n - 1))
            done
            lab_die "the lab app did not open, see $LAB_LOG/hyakki-app.log"
            ;;
        *) usage ;;
    esac
}

case $1 in
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
                lab_daemon "$@" || lab_die "oni did not listen, see $LAB_LOG/oni.log"
                ;;
            *) usage ;;
        esac
        exit 0
        ;;
    fixtures | app | wakestub | scope | worker | ui)
        [ "$#" = 2 ] || usage
        lab_load
        "$1" "$2"
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
for t in espeak-ng paplay parec pactl grim cc python3; do
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
        for f in mpris.jsonl notify.jsonl wake.log refused.log model-decisions.jsonl; do
            [ ! -f "$ONI_LAB/log/$f" ] || cp "$ONI_LAB/log/$f" "$out/fixtures/"
        done
        for f in tabs.json tabs-requests.jsonl; do
            [ ! -f "$ONI_LAB/out/$f" ] || cp "$ONI_LAB/out/$f" "$out/fixtures/"
        done
    fi
    ONI_LAB_SAVE=$out/logs "$HERE/down.sh" >"$out/down.txt" 2>&1 || {
        printf 'phase13.sh: down.sh reported a problem, see %s/down.txt\n' "$out" >&2
        st=1
    }
    cat "$out/down.txt"
    exit "$st"
}
trap finish EXIT
trap 'exit 130' INT TERM HUP

if [ "$wm" = hyprland ]; then
    "$HERE/up.sh" hyprland --app --surface --model --decisions --outputs 2 >"$out/up.txt" 2>&1 ||
        { cat "$out/up.txt" >&2; exit 1; }
else
    "$HERE/up.sh" niri --app --surface --model --decisions >"$out/up.txt" 2>&1 || { cat "$out/up.txt" >&2; exit 1; }
fi
lab_load
scope_stub_ok || lab_die "bin/systemd-run is not the lab's refusal stub: up.sh must build scoperefuse.c"
"$HERE/window.sh" build >"$out/build.txt" 2>&1 || { cat "$out/build.txt" >&2; exit 1; }
mkdir -p "$ONI_LAB/voice"
"$HERE/voice.sh" fixtures "$ONI_LAB/voice/fixtures" >"$out/fixtures.txt" 2>&1 || { cat "$out/fixtures.txt" >&2; exit 1; }
python3 "$HERE/reflexkit.py" fixtures "$ONI_LAB/voice/reflex" >>"$out/fixtures.txt" 2>&1 ||
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
# the catalogue, the bus fixtures and the accepting wake stub, then a daemon
# that reads them fresh (up.sh started one before the catalogue existed)
app install
fixtures start
build_wakestub
lab_stop oni TERM
lab_daemon || lab_die "oni did not listen, see $LAB_LOG/oni.log"
python3 "$HERE/reflexkit.py" phase13 "$ONI_LAB" "$out"
