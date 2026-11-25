#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# phase10.sh niri|hyprland OUTDIR: the Phase 10 window, perch and screen-ask
# gate on one compositor. Starts its own lab (up.sh --app --surface --model;
# Hyprland with two outputs), opens the fixture windows (window.sh), drives
# the real daemon, surface and CLI, tears the lab down with down.sh and
# leaves only evidence in OUTDIR:
#   0 census        every advertised global (oni-fixture --globals): ext
#                   foreign toplevel list, ext image copy capture and its
#                   toplevel source, wlr screencopy; the capture path offered
#   1 tracker       the notes fixture and hyakki-app: perch boxes within 1 px
#                   of the compositor's; a niri tiled window refused
#                   no-geometry with no PerchFrame
#   2 drag, drop    drop on the empty desktop refuses; a drag over notes
#                   draws the outline padPx outside the frame, the drop
#                   perches, the chip and the daemon name app and title
#   3 perch         seated by the perch rules, curl ring at the corner in two
#                   crops, tips radiusPx from it
#   4 follow        short move within one frame, long move rewinds and
#                   reperches quietly, output change (Hyprland), focus moves
#                   the perch only with perchFollowsFocus
#   5 detach        drag-off, chip, CLI, window close, Oni off: crops back to
#                   baseline, no strand/chip/outline commit after
#   6 refusals      no perch, inspect off, consent before any byte (frame,
#                   route, preview), no, the asker leaving, the 120 s expiry:
#                   the fixture model's log stays empty
#   7 ask           the fixture model gets one JPEG of the window's size and
#                   nothing else; the point and the drawn strand tip land on
#                   Save notes; a question is answered in words
#   8 ladder        each capture path alone (HYAKKI_ONI_CAPTURE_PATHS), none
#                   refuses, the disclosure differs per path
#   9 masking       on the crop path the model's bytes are white where Oni is
#   10 injection    a hostile reply: one call, no tool offered, nothing run
#   11 sensitive    the password field refused in code
#   12 read-only    no input-executor name under internal/oni, the boundary
#                   and allowlist tests, every daemon child on the allowlist
#   13 disk         no picture file or inline picture left in the lab
#   14 reduced      one commit per part, arrival at once
#   15 settlement   measure.sh perched (30 s) and detached
#   16 display      every ledger line names the lab's display
# OUTDIR/phase10.json holds every step's checks, the limits and the
# measurements; crops/, model/ (the JPEGs the fixture model received),
# answers/, measure-*.json, commands.log and logs/ sit beside it. OUTDIR must
# be outside the repository and the lab. Exit 0 only when everything passed.
# The model is the deterministic loopback fixture (model.py), never a real
# provider. HYAKKI_BIN selects the binaries as for up.sh; strand.test,
# feature.test and window.test there (the latter from ryoku/wm/oniwindow) run
# the replay, boundary and allowlist checks. Takes about 25 minutes.
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"
ROOT=$(CDPATH='' cd -- "$HERE/../../../../.." && pwd -P)

usage() {
    sed -n '3,48p' "$HERE/phase10.sh" >&2
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
        printf 'phase10.sh: down.sh reported a problem, see %s/down.txt\n' "$out" >&2
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
mkdir -p "$out/fixtures"
for name in notes list; do
    python3 "$HERE/fixture.py" png "$name" "$out/fixtures/$name.png"
    python3 "$HERE/fixture.py" manifest "$name" >"$out/fixtures/$name.json"
done
python3 "$LAB_KIT" phase10 "$ONI_LAB" "$out"
