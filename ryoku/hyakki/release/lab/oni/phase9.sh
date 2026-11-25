#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# phase9.sh niri|hyprland OUTDIR: the Phase 9 strand gate on one compositor.
# Starts its own lab (up.sh --app --surface; Hyprland with two outputs), plays
# the strand fixtures through the surface's fake daemon (HYAKKI_ONI_SCRIPT),
# tears the lab down with down.sh and leaves only evidence in OUTDIR:
#   0 census        --globals; the strand draws beyond the sprite's layer,
#                   under the sprite, and pointer events never reach it
#   1 reach/tap/hold  TIMESCALE 0.1: the farthest strand pixel at t = 0,
#                   0.3, 0.74 and 0.92 s matches the ledger's tip within 2 px;
#                   the ledger replays through the strand engine
#   2 retarget      A to B mid-reach never goes home; the swing takes 300 ms
#   3 rewind        back to the hidden baseline, the strand unmapped, no
#                   strand commit after
#   4 perch         the hyakki-app window as the frame: curl at the nearer
#                   top corner
#   5 lasso         two ring crops: thrown 7 px wider than cinched
#   6 region pen    the character at scale .55 rides 26 ahead, 8 outside
#   7 bubble        no bubble rect overlaps its target
#   8 arrival       once per seq, never a stale one, skipped when covered,
#                   none after EOF
#   9 reduced       one strand commit per target and retract, arrival at once
#   10 settlement   measure.sh strand-held and strand-retracted
#   11 tier         battery saver: strand frames at 60 fps, breathing at 30
#   12 audio        a private PipeWire graph (audio.sh): recorded pitch and
#                   timing per motif, silence while the mic is live and after
#                   link loss, a cut within 20 ms of mic-open, no player
#                   process while sounds are off or idle
#   13 display      every ledger line names the lab's display
# OUTDIR/phase9.json holds every step's checks and the measurements; crops/,
# fixtures/, arrivals/, ledgers/, audio/, measure-*.json, globals.json,
# commands.log and logs/ sit beside it. OUTDIR must be outside the repository
# and the lab. Exit 0 only when everything passed. HYAKKI_BIN selects the
# binaries as for up.sh; a strand.test there (go test -c of
# internal/oni/strand) replays the ledgers through the strand engine, or set
# ONI_LAB_STRAND_TEST. Takes about 20 minutes.
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"
ROOT=$(CDPATH='' cd -- "$HERE/../../../../.." && pwd -P)

usage() {
    sed -n '3,35p' "$HERE/phase9.sh" >&2
    exit 2
}
[ "$#" = 2 ] || usage
wm=$1
case $wm in niri | hyprland) ;; *) usage ;; esac
out=$2
case $out in /*) ;; *) out=$PWD/$out ;; esac
case $out/ in "$ROOT"/* | "$ONI_LAB"/*) lab_die "OUTDIR must be outside the repository and the lab" ;; esac
[ ! -e "$out" ] || [ -z "$(ls -A "$out")" ] || lab_die "OUTDIR $out is not empty"
mkdir -p "$out"

finish() {
    st=$?
    trap - EXIT INT TERM HUP
    ONI_LAB_SAVE=$out/logs "$HERE/down.sh" >"$out/down.txt" 2>&1 || {
        printf 'phase9.sh: down.sh reported a problem, see %s/down.txt\n' "$out" >&2
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
python3 "$LAB_KIT" phase9 "$ONI_LAB" "$out"
