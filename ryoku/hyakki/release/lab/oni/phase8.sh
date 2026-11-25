#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# phase8.sh niri|hyprland OUTDIR: the Phase 8 character gate on one compositor.
# Starts its own lab (up.sh --app --surface; Hyprland with two outputs), runs
# the acceptance steps and measurements, tears the lab down with down.sh and
# leaves only evidence in OUTDIR:
#   0 census          oni-surface --globals; the compositor's layer surfaces
#   1 distinct states idle, listening, thinking, reading, speaking and error
#                     from fixtures/states.jsonl: crops against a hidden baseline,
#                     state accents mutually distinct and absent in idle
#   2 gestures        all ten at HYAKKI_ONI_TIMESCALE=0.1 from
#                     fixtures/gestures.jsonl: a mid-gesture crop each and the
#                     pose back at its base
#   3 drag            input.sh drag; the saved fraction matches the drop; the
#                     same rect after a surface restart and a daemon restart
#   4 reduced motion  oni.reduceMotion=on: no commit without an event
#   5 display change  Hyprland: move to a second output, remove it, fly back to
#                     the remembered fraction; niri: scale 1.5, back, off, on
#   6 covered         fullscreen hyakki-app: courtesy hide, measure covered
#   then measure.sh motion, settled, hidden, asleep (one real 90 s wait), off,
#   and 7: every ledger line names the lab's display.
# OUTDIR/phase8.json holds every step's checks and the measurements; crops/,
# measure-*.json, globals.json, commands.log and logs/ (with every surface
# ledger under logs/ledger/) sit beside it. OUTDIR must
# be outside the repository and the lab. Exit 0 only when everything passed.
# Takes about 15 minutes. HYAKKI_BIN selects the binaries as for up.sh.
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"
ROOT=$(CDPATH='' cd -- "$HERE/../../../../.." && pwd -P)

[ "$#" = 2 ] || { sed -n '3,26p' "$HERE/phase8.sh" >&2; exit 2; }
wm=$1
case $wm in niri | hyprland) ;; *) sed -n '3,26p' "$HERE/phase8.sh" >&2; exit 2 ;; esac
out=$2
case $out in /*) ;; *) out=$PWD/$out ;; esac
case $out/ in "$ROOT"/* | "$ONI_LAB"/*) lab_die "OUTDIR must be outside the repository and the lab" ;; esac
[ ! -e "$out" ] || [ -z "$(ls -A "$out")" ] || lab_die "OUTDIR $out is not empty"
mkdir -p "$out"

finish() {
    st=$?
    trap - EXIT INT TERM HUP
    ONI_LAB_SAVE=$out/logs "$HERE/down.sh" >"$out/down.txt" 2>&1 || {
        printf 'phase8.sh: down.sh reported a problem, see %s/down.txt\n' "$out" >&2
        st=1
    }
    cat "$out/down.txt"
    exit "$st"
}
trap finish EXIT
trap 'exit 130' INT TERM HUP

if [ "$wm" = hyprland ]; then
    "$HERE/up.sh" hyprland --app --surface --outputs 2 >"$out/up.txt" 2>&1 || { cat "$out/up.txt" >&2; exit 1; }
else
    "$HERE/up.sh" niri --app --surface >"$out/up.txt" 2>&1 || { cat "$out/up.txt" >&2; exit 1; }
fi
python3 "$LAB_KIT" phase8 "$ONI_LAB" "$out"
