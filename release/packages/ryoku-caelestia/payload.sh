#!/bin/sh
set -eu

: "${RYOKU_SRC:?RYOKU_SRC is required}"
: "${RYOKU_BUILD_DIR:?RYOKU_BUILD_DIR is required}"

stage_prebuilt() {
	[ -n "${RYOKU_PREBUILT_DIR:-}" ] || return 1
	mkdir -p "$RYOKU_BUILD_DIR"
	cp -a "$RYOKU_PREBUILT_DIR/." "$RYOKU_BUILD_DIR/"
}

case "${1:-}" in
build)
	stage_prebuilt && exit 0
	mkdir -p "$RYOKU_BUILD_DIR"
	RYOKU_CAELESTIA_BUILD="$RYOKU_BUILD_DIR/build" \
		"$RYOKU_SRC/ryoku/shell/quickshell/caelestia/plugin/build.sh" "$RYOKU_BUILD_DIR/qml"
	;;
install)
	destdir=${2:?usage: payload.sh install DESTDIR systemd|runit}
	init=${3:?usage: payload.sh install DESTDIR systemd|runit}
	case "$init" in
	systemd|runit) ;;
	*) printf 'payload.sh: unsupported init system: %s\n' "$init" >&2; exit 2 ;;
	esac
	install -dm755 "$destdir/usr/lib/qt6/qml/Caelestia" "$destdir/usr/lib/qt6/qml/M3Shapes"
	cp -a "$RYOKU_BUILD_DIR/qml/Caelestia/." "$destdir/usr/lib/qt6/qml/Caelestia/"
	cp -a "$RYOKU_BUILD_DIR/qml/M3Shapes/." "$destdir/usr/lib/qt6/qml/M3Shapes/"
	chmod -R u=rwX,go=rX "$destdir/usr/lib/qt6"
	;;
*)
	printf 'usage: payload.sh build | install DESTDIR systemd|runit\n' >&2
	exit 2
	;;
esac
