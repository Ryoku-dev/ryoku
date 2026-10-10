#!/bin/sh
set -eu

: "${RYOKU_SRC:?RYOKU_SRC is required}"
: "${RYOKU_BUILD_DIR:?RYOKU_BUILD_DIR is required}"

case "${1:-}" in
build)
	mkdir -p "$RYOKU_BUILD_DIR"
	cd "$RYOKU_SRC/ryoku/palette-bridge"
	CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags='-s -w' \
		-o "$RYOKU_BUILD_DIR/ryoku-palette-bridge" .
	;;
install)
	destdir=${2:?usage: payload.sh install DESTDIR systemd|runit}
	init=${3:?usage: payload.sh install DESTDIR systemd|runit}
	case "$init" in
	systemd|runit) ;;
	*) printf 'payload.sh: unsupported init system: %s\n' "$init" >&2; exit 2 ;;
	esac
	install -Dm755 "$RYOKU_BUILD_DIR/ryoku-palette-bridge" \
		"$destdir/usr/bin/ryoku-palette-bridge"
	install -Dm755 "$RYOKU_SRC/ryoku/palette-bridge/doctor.sh" \
		"$destdir/usr/bin/ryoku-palette-bridge-doctor"
	install -Dm755 "$RYOKU_SRC/ryoku/palette-bridge/remove-integrations.sh" \
		"$destdir/usr/bin/ryoku-palette-bridge-remove-integrations"
	if [ "$init" = systemd ]; then
		install -Dm644 "$RYOKU_SRC/ryoku/palette-bridge/packaging/systemd/ryoku-palette-bridge.service" \
			"$destdir/usr/lib/systemd/user/ryoku-palette-bridge.service"
	fi
	install -dm755 "$destdir/usr/share/ryoku/palette-bridge"
	cp -a "$RYOKU_SRC/ryoku/palette-bridge/." "$destdir/usr/share/ryoku/palette-bridge/"
	if [ "$init" = runit ]; then
		rm -rf "$destdir/usr/share/ryoku/palette-bridge/packaging/systemd"
	fi
	install -Dm644 "$RYOKU_SRC/ryoku/palette-bridge/LICENSE" \
		"$destdir/usr/share/licenses/ryoku-palette-bridge/LICENSE"
	;;
*)
	printf 'usage: payload.sh build | install DESTDIR systemd|runit\n' >&2
	exit 2
	;;
esac
