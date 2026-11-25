#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# voice.sh VERB ...: Phase 11's voice fixtures and key source in the lab.
# Nothing here opens a microphone, a speaker or an input device: speech is
# WAV files played into the lab's virtual microphone (audio.sh mic, inject),
# and the chord is struct input_event bytes written into a FIFO that
# oni-surface reads in place of /dev/input, in the lab only.
#   fixtures DIR      render the speech WAVs (espeak-ng -w, never a device)
#                     and fixtures.json into DIR, under /dev/shm; no lab needed
#   clips DIR         generate the tone-coded clip pack (manifest.json, WAVs,
#                     LICENSE, PROVENANCE.md) into DIR, under /dev/shm; no lab
#   stage-model ID    link speech model ID (stt's catalogue) from
#                     $VOICEBENCH/models (default /dev/shm/hyakki-voicebench)
#                     into voice/stt's model folder in the lab
#                     (data/hyakki/oni/voice/models), after checking its
#                     SHA-256 against the catalogue pin
#   character ID      install a character ID whose voice is the clip pack
#                     (the testmoth profile fixture with voice.engine clips)
#                     into the lab's characters
#   keys              make the chord FIFO and restart oni-surface with
#                     HYAKKI_ONI_KEYS_FIFO pointing at it; prints the FIFO path
#   chord EVENT...    write EVENTs into the FIFO: down, up (left Ctrl and
#                     left Alt), altgr-down, altgr-up (left Ctrl and right
#                     Alt, which never counts), letter (a key outside the
#                     chord, pressed and released), wait:MS
#   text open NAME OUT.txt   open dictation target NAME (a or b, textwin.py)
#                     whose text is written to OUT.txt, outside the lab
#   text get|focus|close|kill NAME   its window record, focus it, close it as
#                     a user would, or end the client
#   clipboard         the nested compositor's clipboard and primary
#                     selection, as JSON
# Every verb but fixtures and clips refuses unless up.sh finished.
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"
VOICE_KIT=$HERE/voicekit.py

usage() {
    sed -n '3,32p' "$HERE/voice.sh" >&2
    exit 2
}
[ "$#" -ge 1 ] || usage
verb=$1
shift

shm_dir() {
    case $1 in /dev/shm/?*) ;; *) lab_die "DIR must be under /dev/shm" ;; esac
    case $1 in *[!A-Za-z0-9/._-]* | */../* | */..) lab_die "DIR may only hold letters, digits, '/', '.', '_' and '-'" ;; esac
}

case $verb in
    fixtures)
        [ "$#" = 1 ] || usage
        shm_dir "$1"
        command -v espeak-ng >/dev/null 2>&1 || lab_die "missing espeak-ng"
        exec python3 "$VOICE_KIT" fixtures "$1"
        ;;
    clips)
        [ "$#" = 1 ] || usage
        shm_dir "$1"
        exec python3 "$VOICE_KIT" clips "$1"
        ;;
esac

lab_load
LAB_KEYS=$ONI_LAB/keys/chord.fifo
LAB_DATA=$ONI_LAB/data

text_app() {
    case $1 in
        a) printf 'dev.ryoku.OniDictateA\n' ;;
        b) printf 'dev.ryoku.OniDictateB\n' ;;
        *) lab_die "unknown text window $1 (a or b)" ;;
    esac
}
text_record() {
    lab_state | python3 "$LAB_KIT" window "$(text_app "$1")"
}
field() {
    python3 -c 'import json, sys; v = json.load(sys.stdin)[sys.argv[1]]; print("" if v is None else v)' "$1"
}
text_wait() {
    _n=$2
    while [ "$_n" -gt 0 ]; do
        if _r=$(text_record "$1" 2>/dev/null); then
            case $3 in
                present) printf '%s\n' "$_r"; return 0 ;;
                *) [ "$(printf '%s' "$_r" | python3 "$LAB_KIT" window-check "$3")" = yes ] && { printf '%s\n' "$_r"; return 0; } ;;
            esac
        elif [ "$3" = gone ]; then
            return 0
        fi
        _n=$((_n - 1))
        sleep 0.1
    done
    return 1
}
text_focus() {
    id=$(text_record "$1" | field id) || lab_die "text window $1 is not open"
    if [ "$LAB_WM" = niri ]; then
        lab_niri action focus-window --id "$id" >/dev/null
    else
        lab_hyprctl dispatch "hl.dsp.focus({ window = \"address:$id\" })" >>"$LAB_LOG/window.log" 2>&1 || true
    fi
}

# paste_json KIND: what wl-paste reads from the nested compositor (KIND is
# clipboard or primary) as a JSON value; null when nothing is offered.
paste_json() {
    _flag=
    [ "$1" = primary ] && _flag=--primary
    # shellcheck disable=SC2086
    _out=$(lab_tool "paste-$(lab_rand)" timeout 5 wl-paste --no-newline $_flag 2>"$ONI_LAB/tmp/paste.err") && _rc=0 || _rc=$?
    python3 -c '
import json, sys
rc, out, err = int(sys.argv[1]), sys.argv[2], open(sys.argv[3], errors="replace").read().strip()
empty = rc != 0 and any(s in err for s in ("Nothing is copied", "No selection", "No suitable type"))
print(json.dumps({"rc": rc, "text": None if empty or rc != 0 else out, "empty": empty or (rc == 0 and out == ""),
                  "stderr": err[-300:]}))' "$_rc" "${_out:-}" "$ONI_LAB/tmp/paste.err"
    rm -f "$ONI_LAB/tmp/paste.err"
}

case $verb in
    stage-model)
        [ "$#" = 1 ] || usage
        exec python3 "$VOICE_KIT" stage-model "$ONI_LAB" "$1"
        ;;
    character)
        [ "$#" = 1 ] || usage
        exec python3 "$VOICE_KIT" character "$ONI_LAB" "$1" "$LAB_DATA/hyakki/oni/characters"
        ;;
    keys)
        [ "$#" = 0 ] || usage
        mkdir -p "$ONI_LAB/keys"
        [ -p "$LAB_KEYS" ] || mkfifo -m 600 "$LAB_KEYS"
        lab_stop oni-surface TERM
        lab_surface "HYAKKI_ONI_KEYS_FIFO=$LAB_KEYS" || lab_die "oni-surface wrote no ledger, see $LAB_LOG/oni-surface.log"
        lab_env_put LAB_KEYS "$LAB_KEYS"
        printf '%s\n' "$LAB_KEYS"
        ;;
    chord)
        [ "$#" -ge 1 ] || usage
        [ -p "$LAB_KEYS" ] || lab_die "no chord FIFO (voice.sh keys)"
        exec python3 "$VOICE_KIT" key "$LAB_KEYS" "$@"
        ;;
    text)
        [ "$#" -ge 2 ] || usage
        action=$1 name=$2
        app=$(text_app "$name")
        case $action in
            open)
                [ "$#" = 3 ] || usage
                out=$3
                case $out in /*.txt) ;; *) lab_die "OUT must be an absolute path ending in .txt" ;; esac
                case $out in "$ONI_LAB"/*) lab_die "OUT must be outside the lab" ;; esac
                ! text_record "$name" >/dev/null 2>&1 || lab_die "text window $name is already open"
                mkdir -p "$(dirname -- "$out")"
                lab_tool "textwin-$name" HOME="$ONI_LAB/home" DBUS_SESSION_BUS_ADDRESS=disabled: \
                    python3 "$HERE/textwin.py" "$app" "Oni dictation $name" "$out" \
                    >>"$LAB_LOG/textwin-$name.log" 2>&1 </dev/null &
                text_wait "$name" 150 present || lab_die "the compositor never listed $app, see $LAB_LOG/textwin-$name.log"
                ;;
            get)
                [ "$#" = 2 ] || usage
                text_record "$name" || lab_die "text window $name is not open"
                ;;
            focus)
                [ "$#" = 2 ] || usage
                text_focus "$name"
                text_wait "$name" 30 focused || lab_die "the compositor does not report text window $name focused"
                ;;
            close)
                [ "$#" = 2 ] || usage
                id=$(text_record "$name" | field id) || lab_die "text window $name is not open"
                if [ "$LAB_WM" = niri ]; then
                    lab_niri action close-window --id "$id" >/dev/null
                else
                    text_focus "$name"
                    text_wait "$name" 30 focused >/dev/null || lab_die "could not focus $name to close it"
                    lab_hyprctl dispatch 'hl.dsp.window.close()' >>"$LAB_LOG/window.log" 2>&1 || true
                fi
                text_wait "$name" 50 gone || lab_die "the compositor still lists text window $name after closing it"
                ;;
            kill)
                [ "$#" = 2 ] || usage
                python3 "$LAB_KIT" stop-role "$LAB_ID" "textwin-$name-" TERM
                text_wait "$name" 50 gone || lab_die "the compositor still lists text window $name"
                ;;
            *) usage ;;
        esac
        ;;
    clipboard)
        [ "$#" = 0 ] || usage
        printf '{"clipboard": %s, "primary": %s}\n' "$(paste_json clipboard)" "$(paste_json primary)"
        ;;
    *) usage ;;
esac
