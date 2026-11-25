#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
set -eu
ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd -P)
: "${DESTDIR:?set DESTDIR to a staging directory, never /}"
exec python3 "$ROOT/release/native.py" install \
    --destdir "$DESTDIR" --build-dir "${HYAKKI_BUILD_DIR:-$ROOT/build/native}"
