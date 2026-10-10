#!/bin/sh
set -eu

: "${RYOKU_SRC:?RYOKU_SRC is required}"
: "${RYOKU_BUILD_DIR:?RYOKU_BUILD_DIR is required}"

case "${1:-}" in
build)
	mkdir -p "$RYOKU_BUILD_DIR"
	RYOKU_BLOBS_BUILD="$RYOKU_BUILD_DIR/build" \
		"$RYOKU_SRC/ryoku/shell/plugin/build.sh" "$RYOKU_BUILD_DIR/qml"
	;;
install)
	destdir=${2:?usage: payload.sh install DESTDIR systemd|runit}
	init=${3:?usage: payload.sh install DESTDIR systemd|runit}
	case "$init" in
	systemd|runit) ;;
	*) printf 'payload.sh: unsupported init system: %s\n' "$init" >&2; exit 2 ;;
	esac
	install -dm755 "$destdir/usr/lib/qt6/qml/Ryoku/Blobs"
	cp -a "$RYOKU_BUILD_DIR/qml/Ryoku/Blobs/." "$destdir/usr/lib/qt6/qml/Ryoku/Blobs/"
	chmod -R u=rwX,go=rX "$destdir/usr/lib/qt6"
	;;
*)
	printf 'usage: payload.sh build | install DESTDIR systemd|runit\n' >&2
	exit 2
	;;
esac
