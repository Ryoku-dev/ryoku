#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# Frozen-binary acceptance only. No builds of Hyakki and no production services.
# Usage: HYAKKI_BIN=/absolute/frozen ONI_LAB_ID=17 sh phase17.sh niri|hyprland /dev/shm/evidence
# Required compiled tests in HYAKKI_BIN: protocol.test, daemon.test,
# settings.test, shell.test, mcpserve.test. See settingskit.py TESTS.
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"
ROOT=$(CDPATH='' cd -- "$HERE/../../../../.." && pwd -P)
usage() { printf 'usage: %s niri|hyprland OUTDIR | ui open|close|route VIEW | mcp\n' "$0" >&2; exit 2; }
[ "$#" -ge 1 ] || usage
case $1 in
    ui)
        lab_load
        [ "$#" -ge 2 ] || usage
        case $2 in
            close) [ "$#" = 2 ] || usage; lab_stop hyakki-app TERM ;;
            open)
                [ "$#" = 2 ] || usage
                lab_app "app-$(lab_rand)" 1536M "$HYAKKI_BIN/hyakki-app" --size 1280x800 >>"$LAB_LOG/hyakki-app.log" 2>&1 </dev/null &
                ;;
            route)
                [ "$#" = 3 ] || usage
                case $3 in
                    rashin|features|providers|api|ask|agents|teams|memory|skills|index|oni|hands|appearance|keys|oni-voice|oni-screen|oni-commands|oni-writing|oni-privacy|parts)
                        lab_hyakki open --tab settings --view "$3" ;;
                    inbox) lab_hyakki open --tab inbox ;;
                    *) lab_die "unknown Phase 17 route: $3" ;;
                esac ;;
            *) usage ;;
        esac
        exit 0 ;;
    mcp)
        [ "$#" = 1 ] || usage
        lab_load
        lab_hyakki mcp
        exit $? ;;
esac
[ "$#" = 2 ] || usage
wm=$1
case $wm in niri|hyprland) ;; *) usage ;; esac
out=$2
case $out in /dev/shm/*) ;; *) lab_die 'OUTDIR must be private scratch under /dev/shm' ;; esac
case $out/ in "$ROOT"/*|"$ONI_LAB"/*|*/../*|*//*) lab_die 'unsafe OUTDIR' ;; esac
[ ! -e "$ONI_LAB" ] && [ ! -L "$ONI_LAB" ] || lab_die 'refusing an existing lab; choose a free ONI_LAB_ID'
umask 077
# An existing evidence directory is never overwritten, even after a failed run.
mkdir "$out" || lab_die 'OUTDIR must not already exist (parent must exist)'
finish() {
    st=$?
    trap - EXIT INT TERM HUP
    ONI_LAB_SAVE=$out/logs "$HERE/down.sh" >"$out/down.txt" 2>&1 || st=1
    python3 "$HERE/settingskit.py" finalize "$out" "$st" || st=1
    exit "$st"
}
trap finish EXIT
trap 'exit 130' INT TERM HUP
# Do not use --surface: that seeds master=true and invalidates first-run proof.
ONI_LAB_FIRST_RUN=1 ONI_LAB_WIDTH=1440 ONI_LAB_HEIGHT=900 \
    "$HERE/up.sh" "$wm" --app >"$out/up.txt" 2>&1 || exit 1
lab_load
lab_run phase17 2G python3 "$HERE/settingskit.py" phase17 "$ONI_LAB" "$out"
