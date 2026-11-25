#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# Frozen-source acceptance only. Never addresses the live desktop.
# phase18.sh niri|hyprland OUT checkout|package
#   HYAKKI_BIN=/absolute/binaries; ONI_LAB_ID=<unique,1-3 lowercase/alnum>
#   ONI_LAB_SECOND_LANGUAGE_STT_MODEL=<staged multilingual catalogue id>
#   ONI_LAB_SPEECH_SOURCE or VOICEBENCH as for phase11.sh.
# Runs every phase8..17 gate, then five independent observed user sessions
# and scale/reduced-motion/output interactions. No prior receipts are reused.
# checkout means source-built binaries in an isolated nested lab, NOT the live
# development desktop. package requires ONI_LAB_PACKAGE_ROOT containing BIN.
# Each root/compositor cell is separate; `index` requires all four cells.
# phase18.sh index OUT CELL_DIR CELL_DIR CELL_DIR CELL_DIR
# Internal `session` and `target` verbs are only called by personakit.py.
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
export PYTHONDONTWRITEBYTECODE=1
case ${1:-} in
    session)
        [ "$#" = 4 ] || exit 2
        . "$HERE/env.sh"
        wm=$2 out=$3 persona=$4
        case $wm in niri|hyprland) ;; *) exit 2 ;; esac
        case $persona in keyboard-first|mouse-first|voice-first|interrupt-heavy|second-language) ;; *) exit 2 ;; esac
        [ ! -e "$ONI_LAB" ] && [ ! -L "$ONI_LAB" ] || lab_die "lab is occupied; refusing to start or tear it down"
        finish() {
            st=$?
            trap - EXIT INT TERM HUP
            ONI_LAB_SAVE=$out/logs sh "$HERE/down.sh" >"$out/down.txt" 2>&1 || st=1
            exit "$st"
        }
        trap finish EXIT
        trap 'exit 130' INT TERM HUP
        if [ "$wm" = hyprland ]; then
            sh "$HERE/up.sh" "$wm" --app --surface --model --outputs 2 >"$out/up.txt" 2>&1
        else
            sh "$HERE/up.sh" "$wm" --app --surface --model >"$out/up.txt" 2>&1
        fi
        lab_load
        sh "$HERE/window.sh" build >"$out/fixture-build.txt" 2>&1
        mkdir -p "$ONI_LAB/voice"
        sh "$HERE/voice.sh" fixtures "$ONI_LAB/voice/fixtures" >"$out/fixtures.txt" 2>&1
        python3 "$HERE/guidekit.py" fixtures "$ONI_LAB/voice/guide" >>"$out/fixtures.txt" 2>&1
        sh "$HERE/voice.sh" character "${ONI_LAB_CLIP_CHARACTER:-labclips}" >"$out/character.txt" 2>&1
        models=$(python3 "$HERE/voicekit.py" stt-defaults)
        [ -z "${ONI_LAB_STT_MODEL:-}" ] || models="$models $ONI_LAB_STT_MODEL"
        models="$models ${ONI_LAB_SECOND_LANGUAGE_STT_MODEL:?set a multilingual catalogue model}"
        for model in $models; do
            sh "$HERE/voice.sh" stage-model "$model" >>"$out/speech-models.txt" 2>&1
        done
        if ! /usr/bin/whisper-server --help >/dev/null 2>&1; then
            ws=$(cat "${VOICEBENCH:-/dev/shm/hyakki-voicebench}/whisper-server.path")
            "$ws" --help >/dev/null 2>&1
            ln -s "$ws" "$ONI_LAB/bin/whisper-server"
        fi
        lab_run persona-driver 1536M python3 "$HERE/personakit.py" session "$ONI_LAB" "$out" "$persona"
        ;;
    locale)
        [ "$#" = 1 ] || exit 2
        . "$HERE/env.sh"
        lab_load
        # A private generated locale; no host locale installation/configuration.
        mkdir -p "$ONI_LAB/locale"
        lab_run "persona-locale-$(lab_rand)" 256M localedef --no-archive -i es_ES -f UTF-8 \
            "$ONI_LAB/locale/es_ES.UTF-8" >"$LAB_LOG/persona-locale-build.txt" 2>&1
        lab_app "locale-check-$(lab_rand)" 64M LANG=es_ES.UTF-8 LC_ALL=es_ES.UTF-8 LOCPATH="$ONI_LAB/locale" \
            locale -k LC_IDENTIFICATION >"$LAB_LOG/persona-locale.txt"
        lab_stop hyakki-app TERM
        lab_app "app-persona-es-$(lab_rand)" 1536M LANG=es_ES.UTF-8 LC_ALL=es_ES.UTF-8 LOCPATH="$ONI_LAB/locale" \
            "$HYAKKI_BIN/hyakki-app" --size 1280x800 >>"$LAB_LOG/hyakki-app.log" 2>&1 </dev/null &
        ;;
    target)
        [ "$#" = 3 ] || exit 2
        . "$HERE/env.sh"
        lab_load
        case $2 in /dev/shm/*) ;; *) lab_die 'target evidence must be private /dev/shm' ;; esac
        lab_tool "guidewin-$(lab_rand)" python3 "$HERE/personakit.py" target "$2" "$3" \
            >>"$LAB_LOG/persona-target.log" 2>&1 </dev/null &
        ;;
    index)
        shift
        exec python3 "$HERE/personakit.py" index "$@"
        ;;
    niri|hyprland)
        [ "$#" = 3 ] || exit 2
        exec python3 "$HERE/personakit.py" suite "$@"
        ;;
    *) printf '%s\n' 'usage: phase18.sh niri|hyprland OUT checkout|package | index OUT CELL_DIR CELL_DIR CELL_DIR CELL_DIR' >&2; exit 2 ;;
esac
