#!/bin/sh
set -eu

: "${RYOKU_SRC:?RYOKU_SRC is required}"
: "${RYOKU_BUILD_DIR:?RYOKU_BUILD_DIR is required}"

case "${1:-}" in
build)
	mkdir -p "$RYOKU_BUILD_DIR"
	cd "$RYOKU_SRC/ryoku/hub/backend"
	CGO_ENABLED=0 go build -trimpath -mod=vendor -o "$RYOKU_BUILD_DIR/ryoku-hub" .
	;;
install)
	destdir=${2:?usage: payload.sh install DESTDIR systemd|runit}
	init=${3:?usage: payload.sh install DESTDIR systemd|runit}
	case "$init" in
	systemd|runit) ;;
	*) printf 'payload.sh: unsupported init system: %s\n' "$init" >&2; exit 2 ;;
	esac
	install -Dm755 "$RYOKU_BUILD_DIR/ryoku-hub" "$destdir/usr/bin/ryoku-hub"
	;;
*)
	printf 'usage: payload.sh build | install DESTDIR systemd|runit\n' >&2
	exit 2
	;;
esac
