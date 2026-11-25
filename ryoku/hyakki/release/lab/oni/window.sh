#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# window.sh VERB ...: the lab's fixture windows (fixture.py pictures shown by
# oni-fixture, an xdg-toplevel built on first use from fixturewin.c).
#   build [DIR]       build oni-fixture and the pictures into DIR (no lab
#                     needed) or into the lab's build/
#   globals           every global the nested compositor advertises (JSON)
#   open NAME         show fixture NAME (notes, notes-tiled, list) and print
#                     its window record once the compositor lists it
#   get NAME          the window record: id, app_id, title, size, pos (global),
#                     box (output-local), output, floating, focused
#   move NAME X Y     move the floating window's top-left to X Y (global
#                     logical px) and confirm the compositor reports it
#   output NAME OUT   move the window onto output OUT, keeping its place
#                     there (Hyprland: a floating window moved by pixels
#                     stays on its workspace's monitor; nested niri has one
#                     output, so niri refuses)
#   focus NAME        focus the window
#   close NAME        close it through the compositor, as a user would, and
#                     wait until it is gone
#   kill NAME         end the client without the compositor's close
# notes and list float by window rule; niri tiles notes-tiled (no position).
# Every client runs through lab_tool: only the nested compositor sees it.
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"

usage() {
    sed -n '3,23p' "$HERE/window.sh" >&2
    exit 2
}
[ "$#" -ge 1 ] || usage
verb=$1
shift

# build_fixture DIR: oni-fixture and the pictures, once.
build_fixture() {
    dir=$1
    [ -x "$dir/oni-fixture" ] && [ -f "$dir/notes.xrgb" ] && return 0
    xml=
    pkgdir=$(pkg-config --variable=pkgdatadir wayland-protocols 2>/dev/null || true)
    for d in "${ONI_LAB_WAYLAND_PROTOCOLS:-}" "$pkgdir" /usr/share/wayland-protocols; do
        if [ -n "$d" ] && [ -f "$d/stable/xdg-shell/xdg-shell.xml" ]; then
            xml=$d/stable/xdg-shell/xdg-shell.xml
            break
        fi
    done
    [ -n "$xml" ] || lab_die "no xdg-shell.xml: install wayland-protocols or set ONI_LAB_WAYLAND_PROTOCOLS"
    for t in wayland-scanner cc pkg-config python3; do
        command -v "$t" >/dev/null 2>&1 || lab_die "missing $t"
    done
    mkdir -p "$dir"
    printf '%s\n' "$xml" >"$dir/protocol-source"
    lab_run "scan-$(lab_rand)" 128M wayland-scanner client-header "$xml" "$dir/xdg-shell-client-protocol.h"
    lab_run "scan-$(lab_rand)" 128M wayland-scanner private-code "$xml" "$dir/xdg-shell-protocol.c"
    flags=$(pkg-config --cflags --libs wayland-client)
    # shellcheck disable=SC2086
    lab_run "cc-$(lab_rand)" 512M cc -O2 -Wall -Wextra -Werror -I"$dir" -o "$dir/oni-fixture" \
        "$HERE/fixturewin.c" "$dir/xdg-shell-protocol.c" $flags
    lab_run "pictures-$(lab_rand)" 256M python3 "$HERE/fixture.py" build "$dir" >/dev/null
}

if [ "$verb" = build ] && [ "$#" = 1 ]; then
    # outside a lab: a scope of its own, named like the lab's
    case $1 in /dev/shm/?*) ;; *) lab_die "build DIR must be under /dev/shm" ;; esac
    LAB_ID=dry-$(lab_rand)
    build_fixture "$1"
    printf 'built %s/oni-fixture and the pictures\n' "$1"
    exit 0
fi

lab_load
build_dir=$ONI_LAB/build

manifest() {
    _m=$build_dir/$1.json
    [ -f "$_m" ] || lab_die "unknown fixture $1 (notes, notes-tiled, list)"
    printf '%s\n' "$_m"
}

app_id() {
    python3 -c 'import json, sys; print(json.load(open(sys.argv[1]))["app_id"])' "$(manifest "$1")"
}

record() {
    lab_state | python3 "$LAB_KIT" window "$(app_id "$1")"
}

field() {
    python3 -c 'import json, sys; v = json.load(sys.stdin)[sys.argv[1]]; print("" if v is None else v)' "$1"
}

wait_record() {
    _n=$2
    while [ "$_n" -gt 0 ]; do
        if _r=$(record "$1" 2>/dev/null); then
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

case $verb in
    build)
        [ "$#" = 0 ] || usage
        build_fixture "$build_dir"
        printf 'built %s/oni-fixture and the pictures\n' "$build_dir"
        ;;
    globals)
        [ "$#" = 0 ] || usage
        build_fixture "$build_dir"
        lab_tool "fixture-globals" "$build_dir/oni-fixture" --globals
        ;;
    open)
        [ "$#" = 1 ] || usage
        build_fixture "$build_dir"
        m=$(manifest "$1")
        # shellcheck disable=SC2046
        set -- "$1" $(python3 -c 'import json, sys; m = json.load(open(sys.argv[1])); print(m["width"], m["height"])' "$m")
        app=$(app_id "$1")
        title=$(python3 -c 'import json, sys; print(json.load(open(sys.argv[1]))["title"])' "$m")
        ! record "$1" >/dev/null 2>&1 || lab_die "fixture $1 is already open"
        lab_tool "fixture-$1" "$build_dir/oni-fixture" "$app" "$title" "$2" "$3" "$build_dir/$1.xrgb" \
            >>"$LAB_LOG/fixture-$1.log" 2>&1 </dev/null &
        wait_record "$1" 100 present || lab_die "the compositor never listed $app, see $LAB_LOG/fixture-$1.log"
        ;;
    get)
        [ "$#" = 1 ] || usage
        record "$1" || lab_die "fixture $1 is not open"
        ;;
    move)
        [ "$#" = 3 ] || usage
        for v in "$2" "$3"; do
            case $v in '' | *[!0-9-]* | ?*-*) lab_die "not an integer: $v" ;; esac
        done
        r=$(record "$1") || lab_die "fixture $1 is not open"
        id=$(printf '%s' "$r" | field id)
        [ "$(printf '%s' "$r" | field floating)" = True ] || lab_die "fixture $1 is not floating"
        if [ "$LAB_WM" = niri ]; then
            # niri takes the position in the window's output, logical px
            # shellcheck disable=SC2046
            set -- "$1" "$2" "$3" $(lab_state | python3 "$LAB_KIT" output-origin "$(printf '%s' "$r" | field output)")
            lab_niri action move-floating-window --id "$id" -x "$(($2 - $4))" -y "$(($3 - $5))" >/dev/null
        else
            lab_hyprctl dispatch "hl.dsp.window.move({ x = $2, y = $3, window = \"address:$id\" })" \
                >>"$LAB_LOG/window.log" 2>&1 || true
            if ! wait_record "$1" 10 "pos=$2,$3" >/dev/null; then
                lab_hyprctl dispatch movewindowpixel "exact $2 $3,address:$id" >>"$LAB_LOG/window.log" 2>&1 || true
            fi
        fi
        wait_record "$1" 30 "pos=$2,$3" || lab_die "the compositor does not report $1 at $2,$3"
        ;;
    output)
        [ "$#" = 2 ] || usage
        [ "$LAB_WM" = hyprland ] || lab_die "nested niri has one output"
        id=$(record "$1" | field id) || lab_die "fixture $1 is not open"
        lab_hyprctl dispatch "hl.dsp.window.move({ monitor = \"$2\", window = \"address:$id\" })" \
            >>"$LAB_LOG/window.log" 2>&1 || true
        wait_record "$1" 30 "output=$2" || lab_die "the compositor does not report $1 on $2"
        ;;
    focus)
        [ "$#" = 1 ] || usage
        id=$(record "$1" | field id) || lab_die "fixture $1 is not open"
        if [ "$LAB_WM" = niri ]; then
            lab_niri action focus-window --id "$id" >/dev/null
        else
            lab_hyprctl dispatch "hl.dsp.focus({ window = \"address:$id\" })" >>"$LAB_LOG/window.log" 2>&1 || true
        fi
        wait_record "$1" 30 focused || lab_die "the compositor does not report $1 focused"
        ;;
    close)
        [ "$#" = 1 ] || usage
        id=$(record "$1" | field id) || lab_die "fixture $1 is not open"
        if [ "$LAB_WM" = niri ]; then
            lab_niri action close-window --id "$id" >/dev/null
        else
            # close() takes the focused window: confirm the focus first, so a
            # missed focus never closes the app under test instead
            lab_hyprctl dispatch "hl.dsp.focus({ window = \"address:$id\" })" >>"$LAB_LOG/window.log" 2>&1 || true
            wait_record "$1" 30 focused >/dev/null || lab_die "could not focus $1 to close it"
            lab_hyprctl dispatch 'hl.dsp.window.close()' >>"$LAB_LOG/window.log" 2>&1 || true
        fi
        wait_record "$1" 50 gone || lab_die "the compositor still lists fixture $1 after closing it"
        ;;
    kill)
        [ "$#" = 1 ] || usage
        python3 "$LAB_KIT" stop-role "$LAB_ID" "fixture-$1-" TERM
        wait_record "$1" 50 gone || lab_die "the compositor still lists fixture $1"
        ;;
    *) usage ;;
esac
