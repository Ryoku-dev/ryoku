#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# outputs.sh VERB ...: change the nested compositor's outputs, then record the
# new pointer extent (the union of the enabled outputs) in lab.env.
#   list                  print the outputs, windows and layer surfaces
#   add NAME [X]          Hyprland: a new WIDTHxHEIGHT headless output at X,0
#                         (default: right of the others)
#   remove NAME           Hyprland: remove a headless output
#   scale NAME S          set the output's scale (niri: winit)
#   off NAME | on NAME [X]   turn the output off or back on (Hyprland: at X,0)
# The nested niri has one winit output, so on niri only scale applies (niri
# accepts off but leaves its winit output on). Refuses unless up.sh finished.
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"
lab_load

usage() {
    sed -n '3,12p' "$HERE/outputs.sh" >&2
    exit 2
}
[ "$#" -ge 1 ] || usage
verb=$1
shift

name_ok() {
    case $1 in '' | *[!A-Za-z0-9_-]*) lab_die "bad output name: $1" ;; esac
}
hypr_only() {
    [ "$LAB_WM" = hyprland ] || lab_die "$verb needs Hyprland; the nested niri has one winit output"
}
right_edge() {
    lab_extent | cut -d' ' -f1
}
hypr_monitor() {
    lab_hyprctl eval "hl.monitor({ output = \"$1\", mode = \"${LAB_WIDTH}x${LAB_HEIGHT}@60\", position = \"${2}x0\", scale = ${3:-1} })" >/dev/null
}

case $verb in
    list)
        [ "$#" = 0 ] || usage
        lab_state
        exit 0
        ;;
    add)
        hypr_only
        [ "$#" -ge 1 ] && [ "$#" -le 2 ] || usage
        name_ok "$1"
        x=${2:-$(right_edge)}
        hypr_monitor "$1" "$x"
        lab_hyprctl output create headless "$1" >/dev/null
        ;;
    remove)
        hypr_only
        [ "$#" = 1 ] || usage
        name_ok "$1"
        [ "$1" != "$LAB_OUTPUT" ] || lab_die "$1 is the lab's primary output"
        lab_hyprctl output remove "$1" >/dev/null
        ;;
    scale)
        [ "$#" = 2 ] || usage
        name_ok "$1"
        case $2 in '' | *[!0-9.]* | *.*.*) lab_die "bad scale: $2" ;; esac
        if [ "$LAB_WM" = niri ]; then
            lab_niri output "$1" scale "$2" >/dev/null
        else
            x=$(lab_state | python3 "$LAB_KIT" output-x "$1") || lab_die "no output $1"
            hypr_monitor "$1" "$x" "$2"
        fi
        ;;
    off)
        [ "$#" = 1 ] || usage
        name_ok "$1"
        if [ "$LAB_WM" = niri ]; then
            lab_niri output "$1" off >/dev/null
        else
            lab_hyprctl eval "hl.monitor({ output = \"$1\", disabled = true })" >/dev/null
        fi
        ;;
    on)
        [ "$#" -ge 1 ] && [ "$#" -le 2 ] || usage
        name_ok "$1"
        if [ "$LAB_WM" = niri ]; then
            lab_niri output "$1" on >/dev/null
        else
            hypr_monitor "$1" "${2:-$(right_edge)}"
        fi
        ;;
    *) usage ;;
esac
sleep 0.5
# With every output off the pointer has nowhere to go; keep the last extent.
extent=$(lab_extent)
read -r width height <<EOF
$extent
EOF
if [ "${width:-0}" -gt 0 ] && [ "${height:-0}" -gt 0 ]; then
    lab_env_put LAB_EXTENT_W "$width"
    lab_env_put LAB_EXTENT_H "$height"
fi
lab_state
