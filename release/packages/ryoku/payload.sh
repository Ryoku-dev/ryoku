#!/bin/sh
set -eu

: "${RYOKU_SRC:?RYOKU_SRC is required}"
: "${RYOKU_BUILD_DIR:?RYOKU_BUILD_DIR is required}"

case "${1:-}" in
build)
	mkdir -p "$RYOKU_BUILD_DIR"
	cd "$RYOKU_SRC/ryoku/cli"
	CGO_ENABLED=0 go build -trimpath -mod=vendor -o "$RYOKU_BUILD_DIR/ryoku" .
	CGO_ENABLED=0 go build -trimpath -mod=vendor -o "$RYOKU_BUILD_DIR/ryoku-host" ./cmd/ryoku-host
	;;
install)
	destdir=${2:?usage: payload.sh install DESTDIR systemd|runit}
	init=${3:?usage: payload.sh install DESTDIR systemd|runit}
	case "$init" in
	systemd|runit) ;;
	*) printf 'payload.sh: unsupported init system: %s\n' "$init" >&2; exit 2 ;;
	esac
	install -Dm755 "$RYOKU_BUILD_DIR/ryoku" "$destdir/usr/bin/ryoku"
	install -Dm755 "$RYOKU_BUILD_DIR/ryoku-host" "$destdir/usr/bin/ryoku-host"
	if [ "$init" = systemd ]; then
		install -Dm644 "$RYOKU_SRC/ryoku/cli/systemd/ryoku-boot-guard.service" \
			"$destdir/usr/lib/systemd/system/ryoku-boot-guard.service"
		install -Dm644 "$RYOKU_SRC/ryoku/cli/systemd/ryoku.tmpfiles.conf" \
			"$destdir/usr/lib/tmpfiles.d/ryoku.conf"
	else
		# ryoku-host translates Arch package names through this table on Void.
		install -Dm644 "$RYOKU_SRC/void/packages/translations.tsv" \
			"$destdir/usr/share/ryoku/packages/void.tsv"
	fi
	;;
*)
	printf 'usage: payload.sh build | install DESTDIR systemd|runit\n' >&2
	exit 2
	;;
esac
