#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# ctl.sh VERB ...: drive the processes under test inside a finished lab.
#   surface start|restart [HYAKKI_ONI_*=VALUE...]   (re)start oni-surface with
#                         the lab hooks on; earlier ledgers are kept
#                         (HYAKKI_ONI_SCRIPT, _TIMESCALE, _POWER, _PROFILE,
#                         _ARRIVALS and _SOUND_FILE; paths absolute)
#   surface stop|kill     SIGTERM or SIGKILL oni-surface and wait until it is gone
#   daemon start|restart [HYAKKI_ONI_CAPTURE_PATHS=LIST]   the oni daemon;
#                         LIST (comma separated capture paths) is the only
#                         capture the daemon may use, so the lab can hide a
#                         protocol; daemon stop|kill likewise
#   hyakki ARG...         the hyakki command line as a lab client
#   globals               oni-surface --globals against the nested compositor
#   state                 the compositor's outputs, windows and layer surfaces
#   fullscreen on|off     put the dev.ryoku.Hyakki window into or out of
#                         fullscreen and confirm the compositor reports it
# Refuses unless up.sh finished.
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"
lab_load

usage() {
    sed -n '3,18p' "$HERE/ctl.sh" >&2
    exit 2
}
[ "$#" -ge 1 ] || usage
verb=$1
shift

hooks() {
    for _a in "$@"; do
        case $_a in
            HYAKKI_ONI_SCRIPT=* | HYAKKI_ONI_TIMESCALE=* | HYAKKI_ONI_POWER=* | HYAKKI_ONI_PROFILE=/*) ;;
            HYAKKI_ONI_ARRIVALS=/* | HYAKKI_ONI_SOUND_FILE=/*) ;;
            *) lab_die "only HYAKKI_ONI_SCRIPT, _TIMESCALE, _POWER, and the absolute paths _PROFILE, _ARRIVALS and _SOUND_FILE may be set (got $_a)" ;;
        esac
    done
}

surface_start() {
    hooks "$@"
    lab_surface "$@" || lab_die "oni-surface wrote no ledger, see $LAB_LOG/oni-surface.log"
}

fullscreen_state() {
    lab_state | python3 "$LAB_KIT" fullscreen dev.ryoku.Hyakki
}

fullscreen() {
    want=$1
    [ "$(fullscreen_state)" = "$want" ] && return 0
    if [ "$LAB_WM" = niri ]; then
        id=$(lab_niri -j windows | python3 "$LAB_KIT" window-id dev.ryoku.Hyakki) ||
            lab_die "no dev.ryoku.Hyakki window"
        lab_niri action focus-window --id "$id" >/dev/null
        lab_niri action fullscreen-window --id "$id" >/dev/null
        sleep 0.5
        [ "$(fullscreen_state)" = "$want" ] && return 0
    else
        # Hyprland 0.56 takes Lua dispatchers only; a classic string is
        # wrapped into hl.dispatch(...) and fails to parse.
        lab_hyprctl dispatch 'hl.dsp.focus({ window = "class:dev.ryoku.Hyakki" })' >>"$LAB_LOG/fullscreen.log" 2>&1 || true
        for toggle in 'hl.dsp.window.fullscreen()' 'hl.dsp.window.fullscreen({ action = "toggle" })'; do
            lab_hyprctl dispatch "$toggle" >>"$LAB_LOG/fullscreen.log" 2>&1 || true
            sleep 0.5
            [ "$(fullscreen_state)" = "$want" ] && return 0
        done
    fi
    lab_die "the compositor does not report fullscreen $want for dev.ryoku.Hyakki"
}

case $verb in
    surface)
        [ "$#" -ge 1 ] || usage
        action=$1
        shift
        case $action in
            start) surface_start "$@" ;;
            restart)
                lab_stop oni-surface TERM
                surface_start "$@"
                ;;
            stop) [ "$#" = 0 ] || usage; lab_stop oni-surface TERM ;;
            kill) [ "$#" = 0 ] || usage; lab_stop oni-surface KILL ;;
            *) usage ;;
        esac
        ;;
    daemon)
        [ "$#" -ge 1 ] || usage
        action=$1
        shift
        for _a in "$@"; do
            case $_a in
                HYAKKI_ONI_CAPTURE_PATHS=*) ;;
                *) lab_die "only HYAKKI_ONI_CAPTURE_PATHS may be set for the daemon (got $_a)" ;;
            esac
        done
        case $action in
            start) lab_daemon "$@" || lab_die "oni did not listen, see $LAB_LOG/oni.log" ;;
            restart)
                lab_stop oni TERM
                lab_daemon "$@" || lab_die "oni did not listen, see $LAB_LOG/oni.log"
                ;;
            stop) [ "$#" = 0 ] || usage; lab_stop oni TERM ;;
            kill) [ "$#" = 0 ] || usage; lab_stop oni KILL ;;
            *) usage ;;
        esac
        ;;
    hyakki)
        [ "$#" -ge 1 ] || usage
        lab_hyakki "$@"
        ;;
    globals)
        [ "$#" = 0 ] || usage
        lab_app "globals-$(lab_rand)" 128M "$HYAKKI_BIN/oni-surface" --globals
        ;;
    state)
        [ "$#" = 0 ] || usage
        lab_state
        ;;
    fullscreen)
        [ "$#" = 1 ] || usage
        case $1 in on | off) fullscreen "$1" ;; *) usage ;; esac
        ;;
    *) usage ;;
esac
