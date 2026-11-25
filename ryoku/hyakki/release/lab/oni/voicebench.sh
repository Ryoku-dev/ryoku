#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# voicebench.sh VERB: the speech-to-text bench behind voice/stt's default.
# Everything it downloads, builds or renders lives in $VOICEBENCH, a
# directory under /dev/shm; only the evidence it writes leaves it. No step
# opens a microphone or a speaker: clips are files, engines read files or
# loopback HTTP, and every child runs with a private, empty runtime
# directory and PipeWire/Pulse pointed at nothing.
#   stage     download the pinned models and LibriSpeech test-clean
#             (CC-BY 4.0), check every SHA-256 (the tarball against
#             OpenSLR's published md5 too), keep 50 utterances of 2-9 s as
#             16 kHz WAV and delete the tarball; render espeak-ng clips
#             (--stdout, never a device); use /usr/bin/whisper-server when it
#             runs, else build whisper.cpp v1.9.1 (CPU, no ffmpeg) from source
#   run OUT   bench every candidate; write OUT/voicebench.json and
#             OUT/voicebench.md
#   clean     delete $VOICEBENCH
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
VOICEBENCH=${VOICEBENCH:-/dev/shm/hyakki-voicebench}

die() {
    printf '%s: %s\n' "${0##*/}" "$*" >&2
    exit 1
}
usage() {
    sed -n '3,19p' "$HERE/voicebench.sh" >&2
    exit 2
}
case $VOICEBENCH in
    /dev/shm/?*) ;;
    *) die "VOICEBENCH must be a directory under /dev/shm (got $VOICEBENCH)" ;;
esac
case $VOICEBENCH in
    *[!A-Za-z0-9/._-]* | */../* | */.. | *//*) die "VOICEBENCH may only hold letters, digits, '/', '.', '_' and '-'" ;;
esac
[ "$#" -ge 1 ] || usage

HF=https://huggingface.co
# file|url|sha256, matching the voice/stt catalogue pins
MODELS="ggml-base.en-q5_1.bin|$HF/ggerganov/whisper.cpp/resolve/main/ggml-base.en-q5_1.bin|4baf70dd0d7c4247ba2b81fafd9c01005ac77c2f9ef064e00dcf195d0e2fdd2f
ggml-large-v3-turbo-q5_0.bin|$HF/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo-q5_0.bin|394221709cd5ad1f40c46e6031ca61bce88931e6e088c188294c6d5a55ffa7e2
ggml-silero-v6.2.0.bin|$HF/ggml-org/whisper-vad/resolve/main/ggml-silero-v6.2.0.bin|2aa269b785eeb53a82983a20501ddf7c1d9c48e33ab63a41391ac6c9f7fb6987
parakeet-tdt-0.6b-v2-Q8_0.gguf|$HF/handy-computer/parakeet-tdt-0.6b-v2-gguf/resolve/main/parakeet-tdt-0.6b-v2-Q8_0.gguf|f0d0e99cebb6d3b83f1f7069b82b5d3c2e39a54545b0da039cb4bafd9c4e5caa"
LIBRISPEECH=https://www.openslr.org/resources/12
WHISPER_TAG=v1.9.1
WHISPER_COMMIT=f049fff95a089aa9969deb009cdd4892b3e74916

# fetch FILE URL SHA256: download into models/ unless the pinned file is
# already there; a mismatch is deleted and fails the stage.
fetch() {
    dest=$VOICEBENCH/models/$1
    if [ -f "$dest" ] && printf '%s  %s\n' "$3" "$dest" | sha256sum -c --status; then
        return
    fi
    curl -sSfL --retry 3 -o "$dest.part" "$2"
    if ! printf '%s  %s\n' "$3" "$dest.part" | sha256sum -c --status; then
        rm -f "$dest.part"
        die "$1 does not hash to its pin $3"
    fi
    mv "$dest.part" "$dest"
}

# whisper_server: a whisper-server that starts, or a CPU build of the
# pinned tag. The answer goes to $VOICEBENCH/whisper-server.path.
whisper_server() {
    if /usr/bin/whisper-server --help >"$VOICEBENCH/usr-whisper-server.help" 2>&1; then
        printf '%s\n' /usr/bin/whisper-server >"$VOICEBENCH/whisper-server.path"
        return
    fi
    src=$VOICEBENCH/whisper.cpp
    if [ ! -x "$src/build/bin/whisper-server" ]; then
        rm -rf "$src"
        git clone -q --depth 1 --branch "$WHISPER_TAG" https://github.com/ggml-org/whisper.cpp.git "$src"
        [ "$(git -C "$src" rev-parse HEAD)" = "$WHISPER_COMMIT" ] || die "whisper.cpp $WHISPER_TAG is not commit $WHISPER_COMMIT"
        systemd-run --user --scope -q -p MemoryMax=6G -p MemorySwapMax=0 choom -n 1000 -- sh -c "
            cmake -S '$src' -B '$src/build' -G Ninja -DCMAKE_BUILD_TYPE=Release -DWHISPER_FFMPEG=OFF \
                -DWHISPER_SDL2=OFF -DGGML_CUDA=OFF -DGGML_VULKAN=OFF -DBUILD_SHARED_LIBS=OFF \
                -DWHISPER_BUILD_TESTS=OFF -DGGML_NATIVE=ON >'$VOICEBENCH/whisper-build.log' 2>&1 &&
            nice -n 10 cmake --build '$src/build' -j8 --target whisper-server >>'$VOICEBENCH/whisper-build.log' 2>&1"
    fi
    "$src/build/bin/whisper-server" --help >"$VOICEBENCH/whisper-server.help" 2>&1 || die "the built whisper-server does not start"
    printf '%s\n' "$src/build/bin/whisper-server" >"$VOICEBENCH/whisper-server.path"
}

stage() {
    mkdir -p "$VOICEBENCH/models" "$VOICEBENCH/corpus"
    printf '%s\n' "$MODELS" | while IFS='|' read -r file url sum; do
        fetch "$file" "$url" "$sum"
    done
    tarball=$VOICEBENCH/corpus/test-clean.tar.gz
    if [ ! -f "$VOICEBENCH/corpus/librispeech.json" ]; then
        [ -f "$tarball" ] || curl -sSfL --retry 3 -o "$tarball" "$LIBRISPEECH/test-clean.tar.gz"
        curl -sSfL -o "$VOICEBENCH/corpus/md5sum.txt" "$LIBRISPEECH/md5sum.txt"
        want=$(awk '$2 == "test-clean.tar.gz" { print $1 }' "$VOICEBENCH/corpus/md5sum.txt")
        [ -n "$want" ] || die "OpenSLR's md5sum.txt has no test-clean.tar.gz line"
        printf '%s  %s\n' "$want" "$tarball" | md5sum -c --status || die "test-clean.tar.gz does not match OpenSLR's md5 $want"
        python3 "$HERE/voicebench.py" corpus --dir "$VOICEBENCH" --tarball "$tarball" \
            --source "$LIBRISPEECH/test-clean.tar.gz" --md5 "$want"
        rm -f "$tarball"
    fi
    python3 "$HERE/voicebench.py" espeak --dir "$VOICEBENCH"
    whisper_server
}

case $1 in
    stage) stage ;;
    run)
        [ "$#" -eq 2 ] || usage
        mkdir -p "$2"
        [ -f "$VOICEBENCH/whisper-server.path" ] || die "run stage first"
        exec systemd-run --user --scope -q -p MemoryMax=8G -p MemorySwapMax=0 choom -n 1000 -- \
            python3 "$HERE/voicebench.py" run --dir "$VOICEBENCH" --whisper "$(cat "$VOICEBENCH/whisper-server.path")" --out "$2"
        ;;
    clean) rm -rf "$VOICEBENCH" ;;
    *) usage ;;
esac
