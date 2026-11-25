#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
set -eu
ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd -P)
[ "$#" -le 1 ] || { printf 'usage: release/check.sh [STAGE]\n' >&2; exit 2; }
python3 "$ROOT/release/native.py" notices --check
python3 -B "$ROOT/release/test_oni_legal.py"
for script in "$ROOT"/release/*.sh "$ROOT"/release/lab/*/*.sh "$ROOT"/runit/*/run \
    "$ROOT"/runit/*/finish "$ROOT"/usage/hyakki-usage; do
    [ ! -f "$script" ] || sh -n "$script"
done
bash -n "$ROOT/release/packages/arch/PKGBUILD"
sh -n "$ROOT/release/packages/arch/hyakki.install"
bash -n "$ROOT/release/packages/void/template"
exec python3 "$ROOT/release/check.py" "$@"
