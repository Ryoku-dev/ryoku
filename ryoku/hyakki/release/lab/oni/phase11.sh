#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# phase11.sh niri|hyprland OUTDIR: the Phase 11 voice gate on one compositor.
# Starts its own lab (up.sh --app --surface --model), a private PipeWire
# graph with a virtual microphone (audio.sh up, mic), the chord FIFO
# (voice.sh keys), speech rendered by espeak-ng into the lab, a tone-coded
# clip character and the staged speech model; drives the real daemon,
# surface and CLI (hyakki oni talk|dictate|voice-test, oni watch); tears the
# lab down with down.sh and leaves only evidence in OUTDIR:
#   0 graph         no device sink or source, the default source is the
#                   virtual microphone, the guard refuses doctored graphs; the
#                   surface reads the chord FIFO; the meter rises on speech,
#                   offers no monitor or device and writes nothing
#   1 push to talk  AltGr and a letter open nothing, a tap is dropped; a held
#                   chord: partials, the final text equals the fixture, one
#                   ask, the ack and the reply spoken and in the recorded PCM,
#                   capture only for the press, level frames bounded
#   2 barge-in      the chord during the reply: listening at once, the
#                   recorded speech fades within the mute ramp, never resumes
#   3 echo          Oni's speech looped into the microphone on open mic is
#                   heard back, not asked
#   4 wake          the phrase opens a session, a clock question answered
#                   locally, non-wake speech leaves no trace anywhere, the
#                   session ceiling, a changed phrase, Hands untouched, disarm
#   5 open mic      "yes" never answers the consent card; the 180 s ceiling
#   6 hygiene       the 43-repeat WAV collapses; a hold past maxHoldS is cut
#   7 dictation     exact, clean and polished into a text window; closing it
#                   mid-dictation types nothing anywhere (other window,
#                   clipboard, primary, files, frames, ledgers)
#   8 polish        the lab model failing or too slow: clean, and it says so
#   9 clip voice    the clock and timer lines decoded from recorded PCM
#   10 sound gate   no motif plays while armed, listening, transcribing or
#                   speaking
#   11 ladder       hotkey to listening, first partial, final, ack; engine
#                   RSS, unload; the first syllable without a warm mic
#   12 settlement   voice on and idle, voice off (measure.sh); wake armed
#                   recorded, not gated
#   13 read-only    no input-executor name outside internal/dictate/insert,
#                   the boundary, insert and target tests, every wtype the
#                   daemon ran was `wtype -s 50 -- CHUNK` inside a dictation
# OUTDIR/phase11.json holds every step's checks, the limits, the ladder and
# the measurements; audio/ (recordings), dictation/ (what each text window
# received), frames.jsonl, measure-*.json, commands.log and logs/ sit beside
# it. OUTDIR must be outside the repository and the lab. Exit 0 only when
# everything passed. The model is the deterministic loopback fixture
# (model.py). Speech is espeak-ng, rendered at run time; it proves the
# pipeline, not recognition of human speech. The speech models come from
# voicebench.sh stage ($VOICEBENCH/models) or ONI_LAB_SPEECH_SOURCE: with
# neither ONI_LAB_STT_ENGINE nor ONI_LAB_STT_MODEL set the settings keep
# Hyakki's choice and the lab stages stt's default model, its fallback and
# the fallback's VAD; ONI_LAB_STT_MODEL (and ONI_LAB_STT_ENGINE) pick one
# catalogue entry instead. whisper-server is /usr/bin's when it starts, else
# voicebench.sh's CPU build ($VOICEBENCH/whisper-server.path), linked into
# the lab's bin.
# HYAKKI_BIN selects the binaries as for up.sh; feature.test, insert.test and
# target.test there (go test -c) run step 13's tests. Takes about 25 minutes.
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"
ROOT=$(CDPATH='' cd -- "$HERE/../../../../.." && pwd -P)

usage() {
    sed -n '3,56p' "$HERE/phase11.sh" >&2
    exit 2
}
[ "$#" = 2 ] || usage
wm=$1
case $wm in niri | hyprland) ;; *) usage ;; esac
out=$2
case $out in /*) ;; *) out=$PWD/$out ;; esac
case $out/ in "$ROOT"/* | "$ONI_LAB"/*) lab_die "OUTDIR must be outside the repository and the lab" ;; esac
[ ! -e "$out" ] || [ -z "$(ls -A "$out")" ] || lab_die "OUTDIR $out is not empty"
for t in espeak-ng paplay parec pactl wl-paste wtype python3; do
    command -v "$t" >/dev/null 2>&1 || lab_die "missing $t"
done
python3 -c 'import gi; gi.require_version("Gtk", "4.0"); import numpy' 2>/dev/null ||
    lab_die "missing python3 GTK 4 bindings (gi) or numpy"
mkdir -p "$out"

finish() {
    st=$?
    trap - EXIT INT TERM HUP
    ONI_LAB_SAVE=$out/logs "$HERE/down.sh" >"$out/down.txt" 2>&1 || {
        printf 'phase11.sh: down.sh reported a problem, see %s/down.txt\n' "$out" >&2
        st=1
    }
    cat "$out/down.txt"
    exit "$st"
}
trap finish EXIT
trap 'exit 130' INT TERM HUP

"$HERE/up.sh" "$wm" --app --surface --model >"$out/up.txt" 2>&1 || { cat "$out/up.txt" >&2; exit 1; }
"$HERE/window.sh" build >"$out/build.txt" 2>&1 || { cat "$out/build.txt" >&2; exit 1; }
mkdir -p "$ONI_LAB/voice"
"$HERE/voice.sh" fixtures "$ONI_LAB/voice/fixtures" >"$out/fixtures.txt" 2>&1 || { cat "$out/fixtures.txt" >&2; exit 1; }
"$HERE/voice.sh" character "${ONI_LAB_CLIP_CHARACTER:-labclips}" >"$out/character.txt" 2>&1 ||
    { cat "$out/character.txt" >&2; exit 1; }
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
python3 "$HERE/voicekit.py" phase11 "$ONI_LAB" "$out"
