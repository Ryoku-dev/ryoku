#!/bin/bash
# Builds ryoku-dictate in a throwaway directory. The package build does the
# same thing from release/packages/ryoku-dictation; this is the dev loop.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
build="${RYOKU_DICTATION_BUILD:-$here/build}"
if [[ -n "${TRANSCRIBE_SRC:-}" ]]; then
    cmake -S "$here" -B "$build" -G Ninja -DCMAKE_BUILD_TYPE=Release -DTRANSCRIBE_SRC="$TRANSCRIBE_SRC" "${@}"
else
    cmake -S "$here" -B "$build" -G Ninja -DCMAKE_BUILD_TYPE=Release "${@}"
fi
cmake --build "$build" -j"$(nproc)"
