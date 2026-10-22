#!/bin/bash

# Re-derives the Caelestia scheme from the wallpaper that is on screen.
#
# caelestia derives dynamic colours from the wallpaper it was last told about
# (`caelestia wallpaper -f`). path.txt is written directly by the deploy script
# and by the wallpaper picker's still-frame path, so caelestia can be left
# without a wallpaper; `caelestia scheme set -n dynamic` then writes nothing at
# all, the palette stays on the shell's built-in default, and the whole desktop
# comes back on the wrong colours until something re-derives.
#
# Run at shell start so a session always ends up on the wallpaper it is showing.
# A scheme the user picked is left alone.
#
# Arguments are passed on to the `caelestia` commands below, which is how the shell states its
# smart-scheme setting (`--no-smart`): the pipeline keeps no copy of it, so a caller that says
# nothing gets the wallpaper's choice.

set -euo pipefail

CAELESTIA_STATE="${XDG_STATE_HOME:-$HOME/.local/state}/caelestia"
WALLPAPER_FILE="$CAELESTIA_STATE/wallpaper/path.txt"
SCHEME_FILE="$CAELESTIA_STATE/scheme.json"

if ! command -v caelestia >/dev/null 2>&1; then
    exit 0
fi

if [[ ! -s "$WALLPAPER_FILE" ]]; then
    exit 0
fi

WALLPAPER="$(<"$WALLPAPER_FILE")"
if [[ -z "$WALLPAPER" || ! -f "$WALLPAPER" ]]; then
    exit 0
fi

# Only a dynamic scheme is derived from the wallpaper. Anything else was chosen
# deliberately, so leave it as it is.
CURRENT=""
if [[ -s "$SCHEME_FILE" ]] && command -v python3 >/dev/null 2>&1; then
    CURRENT="$(python3 -c 'import json, sys; print(json.load(open(sys.argv[1])).get("name", ""))' "$SCHEME_FILE" 2>/dev/null || true)"
fi
if [[ -n "$CURRENT" && "$CURRENT" != "dynamic" ]]; then
    exit 0
fi

scheme_mtime() {
    stat -c %Y "$SCHEME_FILE" 2>/dev/null || echo 0
}

before="$(scheme_mtime)"
timeout 10s caelestia scheme set -n dynamic "$@" >/dev/null 2>&1 || true

# Nothing written means the CLI had no wallpaper to derive from. Hand it the one
# that is on screen and ask again.
if [[ "$(scheme_mtime)" == "$before" ]]; then
    timeout 10s caelestia wallpaper -f "$WALLPAPER" "$@" >/dev/null 2>&1 || true
    timeout 10s caelestia scheme set -n dynamic "$@" >/dev/null 2>&1 || true
fi
