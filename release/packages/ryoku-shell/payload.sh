#!/bin/sh
set -eu

: "${RYOKU_SRC:?RYOKU_SRC is required}"
: "${RYOKU_BUILD_DIR:?RYOKU_BUILD_DIR is required}"

case "${1:-}" in
build)
	mkdir -p "$RYOKU_BUILD_DIR"
	cd "$RYOKU_SRC/ryoku/shell/ipc"
	CGO_ENABLED=0 go build -trimpath -mod=vendor -o "$RYOKU_BUILD_DIR/ryoku-shell" .
	"$RYOKU_SRC/ryoku/shell/livewall/build.sh" "$RYOKU_BUILD_DIR/ryoku-livewall"
	;;
install)
	destdir=${2:?usage: payload.sh install DESTDIR systemd|runit}
	init=${3:?usage: payload.sh install DESTDIR systemd|runit}
	case "$init" in
	systemd|runit) ;;
	*) printf 'payload.sh: unsupported init system: %s\n' "$init" >&2; exit 2 ;;
	esac
	install -Dm755 "$RYOKU_BUILD_DIR/ryoku-shell" "$destdir/usr/bin/ryoku-shell"
	install -Dm755 "$RYOKU_BUILD_DIR/ryoku-livewall" "$destdir/usr/bin/ryoku-livewall"
	for script in "$RYOKU_SRC"/ryoku/shell/scripts/ryoku-*; do
		[ -f "$script" ] || continue
		install -Dm755 "$script" "$destdir/usr/bin/${script##*/}"
	done
	install -Dm755 "$RYOKU_SRC/ryoku/shell/scripts/ryostage" "$destdir/usr/bin/ryostage"
	for script in "$RYOKU_SRC"/ryoku/shell/scripts/*.sh; do
		[ -f "$script" ] || continue
		install -Dm755 "$script" "$destdir/usr/bin/${script##*/}"
	done
	install -d "$destdir/usr/share/ryoku/nomarchy"
	cp -a "$RYOKU_SRC/ryoku/shell/nomarchy/." "$destdir/usr/share/ryoku/nomarchy/"
	install -d "$destdir/usr/share/ryoku/reload-cover"
	cp -a "$RYOKU_SRC/ryoku/shell/quickshell/reload-cover/." "$destdir/usr/share/ryoku/reload-cover/"
	;;
*)
	printf 'usage: payload.sh build | install DESTDIR systemd|runit\n' >&2
	exit 2
	;;
esac
