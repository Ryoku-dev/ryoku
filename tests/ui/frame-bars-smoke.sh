#!/usr/bin/env bash
set -euo pipefail

shell_bin="${RYOKU_SHELL_BIN:-ryoku-shell}"
hyprctl_bin="${HYPRCTL_BIN:-hyprctl}"
grim_bin="${GRIM_BIN:-grim}"
output_dir="${1:-$(mktemp -d -t frame-bars-smoke.XXXXXX)}"

if [[ "$#" -gt 1 ]]; then
    echo "usage: $0 [screenshot-directory]" >&2
    exit 2
fi

if [[ "$shell_bin" == */* ]]; then
    [[ -x "$shell_bin" ]] || { echo "frame-bars-smoke: executable not found: $shell_bin" >&2; exit 1; }
else
    command -v "$shell_bin" >/dev/null || { echo "frame-bars-smoke: command not found: $shell_bin" >&2; exit 1; }
fi
command -v "$hyprctl_bin" >/dev/null || { echo "frame-bars-smoke: command not found: $hyprctl_bin" >&2; exit 1; }
command -v "$grim_bin" >/dev/null || { echo "frame-bars-smoke: command not found: $grim_bin" >&2; exit 1; }
command -v jq >/dev/null || { echo "frame-bars-smoke: command not found: jq" >&2; exit 1; }

mkdir -p "$output_dir"
monitor="$("$hyprctl_bin" activeworkspace -j | jq -er '.monitor')"

exercise() {
    local name="$1"
    shift
    local reply

    # ryoku-shell returns only after the daemon response. A bare "ok" is silent;
    # an "err ..." reply makes the client fail, so a zero exit is an affirmative IPC response.
    if ! reply="$("$shell_bin" "$@" 2>&1)"; then
        printf 'frame-bars-smoke: %s rejected: %s\n' "$name" "$reply" >&2
        return 1
    fi

    "$grim_bin" -o "$monitor" "$output_dir/$name.png"
    [[ -s "$output_dir/$name.png" ]] || {
        printf 'frame-bars-smoke: empty screenshot for %s\n' "$name" >&2
        return 1
    }
    printf 'frame-bars-smoke: %s acknowledged; %s\n' "$name" "$output_dir/$name.png"
}

exercise quicksettings quicksettings
exercise stash stash
exercise power power
exercise launcher launcher

echo "frame-bars-smoke: verified active monitor $monitor"
