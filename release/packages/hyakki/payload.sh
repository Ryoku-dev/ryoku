#!/bin/sh
set -eu

: "${RYOKU_SRC:?RYOKU_SRC is required}"
: "${RYOKU_BUILD_DIR:?RYOKU_BUILD_DIR is required}"
module="$RYOKU_SRC/ryoku/hyakki"

case "${1:-}" in
build)
    if [ -n "${RYOKU_PREBUILT_DIR:-}" ]; then
        mkdir -p "$RYOKU_BUILD_DIR"
        for binary in hyakki oni hyakki-app oni-surface; do
            install -m755 "$RYOKU_PREBUILT_DIR/$binary" "$RYOKU_BUILD_DIR/$binary"
        done
    else
        HYAKKI_VERSION="${RYOKU_PKGVER:-dev}" HYAKKI_BUILD_DIR="$RYOKU_BUILD_DIR" \
            "$module/release/build.sh"
    fi
    ;;
install)
    destdir=${2:?usage: payload.sh install DESTDIR systemd|runit}
    init=${3:?usage: payload.sh install DESTDIR systemd|runit}
    case "$init" in
    systemd|runit) ;;
    *) printf 'payload.sh: unsupported init system: %s\n' "$init" >&2; exit 2 ;;
    esac
    HYAKKI_BUILD_DIR="$RYOKU_BUILD_DIR" DESTDIR="$destdir" "$module/release/install.sh"
    if [ "$init" = runit ]; then
        rm -r "$destdir/usr/lib/systemd"
        for service in oni oni-routines hyakki-usage; do
            install -d "$destdir/usr/lib/ryoku/runit/user/$service"
            cp -a "$RYOKU_SRC/void/init/user/$service/." \
                "$destdir/usr/lib/ryoku/runit/user/$service/"
        done
    fi
    ;;
*)
    printf 'usage: payload.sh build | install DESTDIR systemd|runit\n' >&2
    exit 2
    ;;
esac
