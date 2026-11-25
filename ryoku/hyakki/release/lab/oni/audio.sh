#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# audio.sh VERB ...: a private, silent audio graph for Oni's sounds inside a
# finished lab. Never the desktop's PipeWire, speakers or microphone.
#   up              start pipewire, wireplumber (profile "policy": linking and
#                   defaults, no ALSA, Bluetooth or camera monitors) and
#                   pipewire-pulse in lab scopes, with the lab's HOME, XDG
#                   directories, private session bus and runtime directory
#                   ($ONI_LAB/rt, so oni-surface's pw-cat or paplay finds this
#                   graph at its default socket path and nothing else); load a
#                   null sink oni_lab that never suspends and make it the
#                   default; refuse if any other sink or source is a device
#   graph           the graph as JSON: info, sinks, sources, sink inputs, objects
#   clients         pw-dump of the graph (every object, clients included)
#   record OUT.wav  record oni_lab's monitor with parec (float32, mono,
#                   48 kHz) into OUT.wav, in the background
#   stop            end the recording (SIGINT, so parec finalises the header)
#   down            stop the recorder and the graph; down.sh does this too
# Phase 11's virtual microphone (after up):
#   mic             load a second null sink oni_lab_mic, never suspending, and
#                   a virtual source oni_lab_mic_in that remaps its monitor
#                   (a source, not a monitor, as a microphone chooser offers
#                   one); make oni_lab_mic_in the default source; refuse
#                   unless voicekit.py mic-check passes (no device sink or
#                   source, the lab's defaults, only lab modules)
#   inject WAV      play WAV into oni_lab_mic with paplay and wait until it
#                   ends: what the default source hears
#   play WAV        play WAV into oni_lab (a sync mark in the record)
#   echo on|off     a module-loopback from oni_lab.monitor into oni_lab_mic,
#                   so what Oni says reaches the microphone; off unloads it
#   record-mic OUT.wav  record oni_lab_mic's monitor like record (after
#                   record when both run; stop ends both)
#   recorders       the application name of every capture stream, one a line
#   modules         the loaded modules as JSON
# Refuses unless up.sh finished.
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"
lab_load

usage() {
    sed -n '3,35p' "$HERE/audio.sh" >&2
    exit 2
}
[ "$#" -ge 1 ] || usage

LAB_AUDIO_SINK=oni_lab
pulse() {
    lab_app "pactl-$(lab_rand)" 64M pactl "$@"
}
running() {
    [ -n "$(python3 "$LAB_KIT" pids "$LAB_ID" "$1")" ]
}
wait_for() {
    _n=$1
    shift
    while [ "$_n" -gt 0 ]; do
        if "$@" >/dev/null 2>&1; then return 0; fi
        _n=$((_n - 1))
        sleep 0.1
    done
    return 1
}
default_is_lab() {
    [ "$(pulse get-default-sink 2>/dev/null)" = "$LAB_AUDIO_SINK" ]
}

up() {
    running pipewire && lab_die "the lab's audio graph is already up"
    for t in pipewire wireplumber pipewire-pulse pactl parec pw-dump; do
        command -v "$t" >/dev/null 2>&1 || lab_die "missing $t"
    done
    [ -S "$LAB_RT/bus" ] || lab_die "no private session bus at $LAB_RT/bus (up.sh --app or --surface)"
    # Drop-ins in the lab's XDG_CONFIG_HOME: no realtime portal or rtkit, no
    # X11 bell, no JACK detection and no D-Bus name; nothing leaves the lab.
    mkdir -p "$ONI_LAB/cfg/pipewire/pipewire.conf.d" "$ONI_LAB/cfg/pipewire/pipewire-pulse.conf.d"
    cat >"$ONI_LAB/cfg/pipewire/pipewire.conf.d/oni-lab.conf" <<'EOF'
context.properties = {
    module.rt = false
    module.portal = false
    module.x11.bell = false
    module.jackdbus-detect = false
    support.dbus = false
}
EOF
    cat >"$ONI_LAB/cfg/pipewire/pipewire-pulse.conf.d/oni-lab.conf" <<'EOF'
context.properties = {
    support.dbus = false
}
pulse.properties = {
    server.dbus-name = ""
}
EOF
    lab_app "audio-pw-$(lab_rand)" 256M pipewire >>"$LAB_LOG/audio-pipewire.log" 2>&1 </dev/null &
    wait_for 100 test -S "$LAB_RT/pipewire-0" || lab_die "pipewire did not start, see $LAB_LOG/audio-pipewire.log"
    lab_app "audio-wp-$(lab_rand)" 256M wireplumber -p policy >>"$LAB_LOG/audio-wireplumber.log" 2>&1 </dev/null &
    lab_app "audio-pulse-$(lab_rand)" 256M pipewire-pulse >>"$LAB_LOG/audio-pulse.log" 2>&1 </dev/null &
    wait_for 100 test -S "$LAB_RT/pulse/native" || lab_die "pipewire-pulse did not start, see $LAB_LOG/audio-pulse.log"
    wait_for 100 pulse info || lab_die "pactl cannot reach the lab's graph"
    pulse load-module module-null-sink sink_name="$LAB_AUDIO_SINK" \
        "sink_properties=device.description=OniLab session.suspend-timeout-seconds=0 node.always-process=true" \
        >>"$LAB_LOG/audio.log"
    pulse set-default-sink "$LAB_AUDIO_SINK"
    wait_for 50 default_is_lab || lab_die "$LAB_AUDIO_SINK did not become the default sink"
    graph >"$LAB_LOG/audio-graph.json"
    python3 "$LAB_KIT" audio-check "$LAB_LOG/audio-graph.json" "$ONI_LAB" ||
        lab_die "the lab's audio graph holds a device, see $LAB_LOG/audio-graph.json"
    lab_env_put LAB_AUDIO "$LAB_AUDIO_SINK"
    printf 'audio.sh: private graph up at %s, default sink %s\n' "$LAB_RT/pipewire-0" "$LAB_AUDIO_SINK"
}

graph() {
    _info=$(pulse -f json info)
    _sinks=$(pulse -f json list sinks)
    _sources=$(pulse -f json list sources)
    _inputs=$(pulse -f json list sink-inputs)
    _clients=$(lab_app "pwdump-$(lab_rand)" 128M pw-dump)
    printf '{"info": %s, "sinks": %s, "sources": %s, "sink_inputs": %s, "objects": %s}\n' \
        "$_info" "$_sinks" "$_sources" "$_inputs" "$_clients"
}

record() {
    out=$1
    case $out in /*.wav) ;; *) lab_die "OUT must be an absolute path ending in .wav" ;; esac
    case $out in "$ONI_LAB"/*) lab_die "OUT must be outside the lab, which down.sh removes" ;; esac
    running pipewire-pulse || lab_die "the lab's audio graph is not up (audio.sh up)"
    running parec && lab_die "a recording is already running"
    mkdir -p "$(dirname -- "$out")"
    lab_app "audio-rec-$(lab_rand)" 128M parec --client-name=oni-lab-recorder --device="$LAB_AUDIO_SINK.monitor" \
        --file-format=wav --format=float32le --rate=48000 --channels=1 --latency-msec=200 "$out" \
        >>"$LAB_LOG/audio-record.log" 2>&1 </dev/null &
    wait_for 50 recording || lab_die "parec did not start recording, see $LAB_LOG/audio-record.log"
}
recording() {
    [ -n "$(pulse list short source-outputs)" ]
}

# --- Phase 11: the virtual microphone ---

LAB_AUDIO_MIC_SINK=oni_lab_mic
LAB_AUDIO_MIC=oni_lab_mic_in
default_source_is_lab() {
    [ "$(pulse get-default-source 2>/dev/null)" = "$LAB_AUDIO_MIC" ]
}
# recorders: the application name of every capture stream, one per line
# (the remap source and an echo loopback are capture streams too)
recorders() {
    pulse -f json list source-outputs | python3 -c '
import json, sys
for o in json.load(sys.stdin):
    print((o.get("properties") or {}).get("application.name", ""))'
}
has_recorder() {
    recorders | grep -qx "$1"
}
mic_up() {
    pulse list short sinks | cut -f2 | grep -qx "$LAB_AUDIO_MIC_SINK"
}
modules() {
    pulse -f json list modules
}
mic_check() {
    graph >"$LAB_LOG/audio-graph.json"
    modules >"$LAB_LOG/audio-modules.json"
    python3 "$HERE/voicekit.py" mic-check "$LAB_LOG/audio-graph.json" "$LAB_LOG/audio-modules.json" "$ONI_LAB"
}

mic() {
    running pipewire-pulse || lab_die "the lab's audio graph is not up (audio.sh up)"
    mic_up && lab_die "the virtual microphone is already up"
    pulse load-module module-null-sink sink_name="$LAB_AUDIO_MIC_SINK" \
        "sink_properties=device.description=OniLabMic session.suspend-timeout-seconds=0 node.always-process=true" \
        >>"$LAB_LOG/audio.log"
    pulse load-module module-remap-source source_name="$LAB_AUDIO_MIC" master="$LAB_AUDIO_MIC_SINK.monitor" \
        channels=1 "source_properties=device.description=OniLabMicIn node.always-process=true" \
        >>"$LAB_LOG/audio.log"
    pulse set-default-source "$LAB_AUDIO_MIC"
    wait_for 50 default_source_is_lab || lab_die "$LAB_AUDIO_MIC did not become the default source"
    wait_for 50 default_is_lab || lab_die "$LAB_AUDIO_SINK is no longer the default sink"
    mic_check >"$LAB_LOG/audio-mic-check.json" ||
        lab_die "the lab's audio graph failed the microphone check, see $LAB_LOG/audio-mic-check.json"
    lab_env_put LAB_AUDIO_MIC "$LAB_AUDIO_MIC"
    printf 'audio.sh: virtual microphone up: inject into %s, default source %s\n' "$LAB_AUDIO_MIC_SINK" "$LAB_AUDIO_MIC"
}

# paplay_into SINK WAV: play WAV into one of the lab's sinks and wait.
paplay_into() {
    case $2 in /*.wav) ;; *) lab_die "WAV must be an absolute path ending in .wav" ;; esac
    [ -f "$2" ] || lab_die "no file $2"
    running pipewire-pulse || lab_die "the lab's audio graph is not up (audio.sh up)"
    lab_app "audio-play-$(lab_rand)" 64M paplay --client-name=oni-lab-injector --device="$1" "$2" \
        >>"$LAB_LOG/audio-play.log" 2>&1 </dev/null
}

echo_loop() {
    mic_up || lab_die "the virtual microphone is not up (audio.sh mic)"
    case $1 in
        on)
            [ -z "${LAB_AUDIO_ECHO:-}" ] || lab_die "the echo loopback is already on (module $LAB_AUDIO_ECHO)"
            _id=$(pulse load-module module-loopback source="$LAB_AUDIO_SINK.monitor" sink="$LAB_AUDIO_MIC_SINK" \
                latency_msec=20 "sink_input_properties=media.name=oni-lab-echo" \
                "source_output_properties=media.name=oni-lab-echo")
            lab_env_put LAB_AUDIO_ECHO "$_id"
            mic_check >"$LAB_LOG/audio-mic-check.json" ||
                lab_die "the echo loopback failed the microphone check, see $LAB_LOG/audio-mic-check.json"
            printf 'audio.sh: echo on (module %s): %s.monitor -> %s\n' "$_id" "$LAB_AUDIO_SINK" "$LAB_AUDIO_MIC_SINK"
            ;;
        off)
            [ -n "${LAB_AUDIO_ECHO:-}" ] || lab_die "the echo loopback is not on"
            pulse unload-module "$LAB_AUDIO_ECHO"
            lab_env_put LAB_AUDIO_ECHO ""
            printf 'audio.sh: echo off\n'
            ;;
        *) usage ;;
    esac
}

record_mic() {
    out=$1
    case $out in /*.wav) ;; *) lab_die "OUT must be an absolute path ending in .wav" ;; esac
    case $out in "$ONI_LAB"/*) lab_die "OUT must be outside the lab, which down.sh removes" ;; esac
    mic_up || lab_die "the virtual microphone is not up (audio.sh mic)"
    has_recorder oni-lab-mic-recorder && lab_die "a microphone recording is already running"
    mkdir -p "$(dirname -- "$out")"
    lab_app "audio-recmic-$(lab_rand)" 128M parec --client-name=oni-lab-mic-recorder \
        --device="$LAB_AUDIO_MIC_SINK.monitor" --file-format=wav --format=float32le --rate=48000 --channels=1 \
        --latency-msec=200 "$out" >>"$LAB_LOG/audio-record.log" 2>&1 </dev/null &
    wait_for 50 has_recorder oni-lab-mic-recorder || lab_die "parec did not start recording, see $LAB_LOG/audio-record.log"
}

case $1 in
    up) [ "$#" = 1 ] || usage; up ;;
    graph) [ "$#" = 1 ] || usage; graph ;;
    clients) [ "$#" = 1 ] || usage; lab_app "pwdump-$(lab_rand)" 128M pw-dump ;;
    record) [ "$#" = 2 ] || usage; record "$2" ;;
    stop) [ "$#" = 1 ] || usage; lab_stop parec INT ;;
    mic) [ "$#" = 1 ] || usage; mic ;;
    inject) [ "$#" = 2 ] || usage; mic_up || lab_die "the virtual microphone is not up (audio.sh mic)"; paplay_into "$LAB_AUDIO_MIC_SINK" "$2" ;;
    play) [ "$#" = 2 ] || usage; paplay_into "$LAB_AUDIO_SINK" "$2" ;;
    echo) [ "$#" = 2 ] || usage; echo_loop "$2" ;;
    record-mic) [ "$#" = 2 ] || usage; record_mic "$2" ;;
    recorders) [ "$#" = 1 ] || usage; recorders ;;
    modules) [ "$#" = 1 ] || usage; modules ;;
    down)
        [ "$#" = 1 ] || usage
        lab_stop parec INT
        for comm in pipewire-pulse wireplumber pipewire; do lab_stop "$comm" TERM; done
        ;;
    *) usage ;;
esac
