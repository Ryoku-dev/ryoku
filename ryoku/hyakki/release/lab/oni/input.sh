#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
# input.sh VERB ...: pointer and keys into the lab's nested compositor only.
#   move X Y              absolute move, then a 1 px nudge so Hyprland re-picks the surface
#   click X Y [right]     move there and click (left by default)
#   drag X1 Y1 X2 Y2      press at the first point, move in 24 steps, release at the second
#   scroll X Y N          N wheel notches at X Y (positive scrolls down)
#   ptr STEP...           raw pointer steps: "a X Y", "m DX DY", "b left|right",
#                         "d" (press), "u" (release), "s N", "w MS"
#   key ARG...            one wtype call, e.g. key -M ctrl -k q -m ctrl
#   type TEXT             type TEXT into the focused surface
# Coordinates are global logical px over every output. One wlr virtual
# pointer lives for the whole step list, because Hyprland drops pointer
# focus when a virtual mouse goes away. It is compiled on first use into the
# lab directory from the wlr-protocols XML found on this system.
set -eu
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
. "$HERE/env.sh"
lab_load

usage() {
    sed -n '3,12p' "$HERE/input.sh" >&2
    exit 2
}
[ "$#" -ge 1 ] || usage
verb=$1
shift

num() {
    for _v in "$@"; do
        case $_v in '' | *[!0-9-]* | ?*-*) lab_die "not an integer: $_v" ;; esac
    done
}

build_dir=$ONI_LAB/build
ptr_bin=$build_dir/oni-ptr

build_ptr() {
    [ -x "$ptr_bin" ] && return 0
    xml=
    pkgdir=$(pkg-config --variable=pkgdatadir wlr-protocols 2>/dev/null || true)
    for dir in "${ONI_LAB_WLR_PROTOCOLS:-}" "${pkgdir:+$pkgdir/unstable}" /usr/share/wlr-protocols/unstable \
        "${ONI_LAB_WLROOTS_SRC:-$HOME/.cache/hyprnest/wlroots-src}/protocol"; do
        if [ -n "$dir" ] && [ -f "$dir/wlr-virtual-pointer-unstable-v1.xml" ]; then
            xml=$dir/wlr-virtual-pointer-unstable-v1.xml
            break
        fi
    done
    [ -n "$xml" ] || lab_die "no wlr-virtual-pointer-unstable-v1.xml: install wlr-protocols or set ONI_LAB_WLR_PROTOCOLS to its directory"
    for t in wayland-scanner cc pkg-config; do
        command -v "$t" >/dev/null 2>&1 || lab_die "missing $t"
    done
    mkdir -p "$build_dir"
    printf '%s\n' "$xml" >"$build_dir/protocol-source"
    lab_run "scan-$(lab_rand)" 128M wayland-scanner client-header "$xml" "$build_dir/wlr-virtual-pointer-unstable-v1-client-protocol.h"
    lab_run "scan-$(lab_rand)" 128M wayland-scanner private-code "$xml" "$build_dir/wlr-virtual-pointer-unstable-v1-protocol.c"
    cat >"$build_dir/oni-ptr.c" <<'EOF'
/* One wlr virtual pointer for a whole list of steps. */
#include <linux/input-event-codes.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>
#include <wayland-client.h>

#include "wlr-virtual-pointer-unstable-v1-client-protocol.h"

struct lab {
    struct wl_seat *seat;
    struct zwlr_virtual_pointer_manager_v1 *manager;
};

static void on_global(void *data, struct wl_registry *registry, uint32_t id, const char *name, uint32_t version)
{
    struct lab *lab = data;
    (void)version;
    if (!strcmp(name, wl_seat_interface.name) && !lab->seat)
        lab->seat = wl_registry_bind(registry, id, &wl_seat_interface, 1);
    else if (!strcmp(name, zwlr_virtual_pointer_manager_v1_interface.name))
        lab->manager = wl_registry_bind(registry, id, &zwlr_virtual_pointer_manager_v1_interface, 1);
}

static void on_global_remove(void *data, struct wl_registry *registry, uint32_t id)
{
    (void)data;
    (void)registry;
    (void)id;
}

static const struct wl_registry_listener listener = {on_global, on_global_remove};

static uint32_t stamp(void)
{
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (uint32_t)((uint64_t)ts.tv_sec * 1000 + (uint64_t)ts.tv_nsec / 1000000);
}

static uint32_t extent(const char *name)
{
    const char *v = getenv(name);
    long n = v ? strtol(v, NULL, 10) : 0;
    if (n <= 0) {
        fprintf(stderr, "oni-ptr: %s is not set\n", name);
        exit(2);
    }
    return (uint32_t)n;
}

static void button(struct zwlr_virtual_pointer_v1 *ptr, uint32_t code, int pressed)
{
    zwlr_virtual_pointer_v1_button(ptr, stamp(), code,
                                   pressed ? WL_POINTER_BUTTON_STATE_PRESSED : WL_POINTER_BUTTON_STATE_RELEASED);
    zwlr_virtual_pointer_v1_frame(ptr);
}

int main(int argc, char **argv)
{
    uint32_t w = extent("ONI_PTR_W"), h = extent("ONI_PTR_H");
    struct lab lab = {0};
    struct wl_display *display = wl_display_connect(NULL);
    if (!display) {
        fprintf(stderr, "oni-ptr: no Wayland display\n");
        return 1;
    }
    struct wl_registry *registry = wl_display_get_registry(display);
    wl_registry_add_listener(registry, &listener, &lab);
    wl_display_roundtrip(display);
    if (!lab.manager) {
        fprintf(stderr, "oni-ptr: the compositor offers no zwlr_virtual_pointer_manager_v1\n");
        return 1;
    }
    struct zwlr_virtual_pointer_v1 *ptr = zwlr_virtual_pointer_manager_v1_create_virtual_pointer(lab.manager, lab.seat);
    wl_display_roundtrip(display);

    for (int i = 1; i < argc; i++) {
        const char *s = argv[i];
        char word[16] = "";
        double x = 0, y = 0;
        if (sscanf(s, "a %lf %lf", &x, &y) == 2) {
            if (x < 0 || y < 0 || x > w || y > h) {
                fprintf(stderr, "oni-ptr: %s is outside %ux%u\n", s, w, h);
                return 2;
            }
            zwlr_virtual_pointer_v1_motion_absolute(ptr, stamp(), (uint32_t)x, (uint32_t)y, w, h);
            zwlr_virtual_pointer_v1_frame(ptr);
        } else if (sscanf(s, "m %lf %lf", &x, &y) == 2) {
            zwlr_virtual_pointer_v1_motion(ptr, stamp(), wl_fixed_from_double(x), wl_fixed_from_double(y));
            zwlr_virtual_pointer_v1_frame(ptr);
        } else if (sscanf(s, "b %15s", word) == 1) {
            uint32_t code = !strcmp(word, "left") ? BTN_LEFT : !strcmp(word, "right") ? BTN_RIGHT : 0;
            if (!code) {
                fprintf(stderr, "oni-ptr: unknown button in %s\n", s);
                return 2;
            }
            button(ptr, code, 1);
            wl_display_roundtrip(display);
            usleep(50000);
            button(ptr, code, 0);
        } else if (!strcmp(s, "d") || !strcmp(s, "u")) {
            button(ptr, BTN_LEFT, s[0] == 'd');
        } else if (sscanf(s, "s %lf", &y) == 1) {
            int32_t count = (int32_t)y;
            if (y == (double)count && count != 0) {
                int32_t notch = count < 0 ? -1 : 1;
                /* Some compositors forward one wheel tick per packet,
                 * irrespective of its aggregate discrete count. */
                for (; count != 0; count -= notch) {
                    zwlr_virtual_pointer_v1_axis_source(ptr, WL_POINTER_AXIS_SOURCE_WHEEL);
                    zwlr_virtual_pointer_v1_axis_discrete(ptr, stamp(), WL_POINTER_AXIS_VERTICAL_SCROLL,
                                                          wl_fixed_from_double(notch * 15), notch);
                    zwlr_virtual_pointer_v1_frame(ptr);
                    if (wl_display_roundtrip(display) < 0) {
                        fprintf(stderr, "oni-ptr: the compositor closed the connection\n");
                        return 1;
                    }
                    usleep(25000);
                }
            } else {
                zwlr_virtual_pointer_v1_axis_source(ptr, WL_POINTER_AXIS_SOURCE_WHEEL);
                zwlr_virtual_pointer_v1_axis_discrete(ptr, stamp(), WL_POINTER_AXIS_VERTICAL_SCROLL,
                                                      wl_fixed_from_double(y * 15), (int32_t)y);
                zwlr_virtual_pointer_v1_frame(ptr);
            }
        } else if (sscanf(s, "w %lf", &x) == 1 && x >= 0) {
            wl_display_roundtrip(display);
            usleep((useconds_t)(x * 1000));
            continue;
        } else {
            fprintf(stderr, "oni-ptr: unknown step \"%s\"\n", s);
            return 2;
        }
        if (wl_display_roundtrip(display) < 0) {
            fprintf(stderr, "oni-ptr: the compositor closed the connection\n");
            return 1;
        }
        usleep(25000);
    }
    zwlr_virtual_pointer_v1_destroy(ptr);
    wl_display_roundtrip(display);
    wl_display_disconnect(display);
    return 0;
}
EOF
    flags=$(pkg-config --cflags --libs wayland-client)
    # shellcheck disable=SC2086
    lab_run "cc-$(lab_rand)" 512M cc -O2 -Wall -Wextra -I"$build_dir" -o "$ptr_bin" \
        "$build_dir/oni-ptr.c" "$build_dir/wlr-virtual-pointer-unstable-v1-protocol.c" $flags
}

ptr() {
    build_ptr
    lab_tool ptr "$ptr_bin" "$@"
}

case $verb in
    move)
        [ "$#" = 2 ] || usage
        num "$1" "$2"
        ptr "a $1 $2" "m 1 0" "m -1 0" "w 150"
        ;;
    click)
        [ "$#" = 2 ] || [ "$#" = 3 ] || usage
        num "$1" "$2"
        btn=${3:-left}
        case $btn in left | right) ;; *) usage ;; esac
        ptr "a $1 $2" "m 1 0" "m -1 0" "w 80" "b $btn" "w 200"
        ;;
    drag)
        [ "$#" = 4 ] || usage
        num "$1" "$2" "$3" "$4"
        x1=$1 y1=$2 x2=$3 y2=$4
        set -- "a $x1 $y1" "m 1 0" "m -1 0" "w 80" "d" "w 80"
        i=1
        while [ "$i" -le 24 ]; do
            set -- "$@" "a $((x1 + (x2 - x1) * i / 24)) $((y1 + (y2 - y1) * i / 24))" "w 12"
            i=$((i + 1))
        done
        ptr "$@" "m 1 0" "m -1 0" "w 80" "u" "w 200"
        ;;
    scroll)
        [ "$#" = 3 ] || usage
        num "$1" "$2" "$3"
        ptr "a $1 $2" "m 1 0" "m -1 0" "w 60" "s $3" "w 150"
        ;;
    ptr)
        [ "$#" -ge 1 ] || usage
        ptr "$@"
        ;;
    key)
        [ "$#" -ge 1 ] || usage
        lab_tool wtype wtype -s 50 "$@"
        ;;
    type)
        [ "$#" = 1 ] || usage
        lab_tool wtype wtype -s 50 -- "$1"
        ;;
    *) usage ;;
esac
