#!/bin/sh
set -eu

: "${RYOKU_SRC:?RYOKU_SRC is required}"
: "${RYOKU_BUILD_DIR:?RYOKU_BUILD_DIR is required}"

case "${1:-}" in
build)
	mkdir -p "$RYOKU_BUILD_DIR"
	cd "$RYOKU_SRC/ryoku/rashin/backend"
	CGO_ENABLED=0 go build -trimpath -mod=vendor -o "$RYOKU_BUILD_DIR/ryoku-rashin" .
	"$RYOKU_BUILD_DIR/ryoku-rashin" repo-index "$RYOKU_SRC" "$RYOKU_BUILD_DIR/ryoku-repo.md"
	;;
install)
	destdir=${2:?usage: payload.sh install DESTDIR systemd|runit}
	init=${3:?usage: payload.sh install DESTDIR systemd|runit}
	case "$init" in
	systemd|runit) ;;
	*) printf 'payload.sh: unsupported init system: %s\n' "$init" >&2; exit 2 ;;
	esac
	install -Dm755 "$RYOKU_BUILD_DIR/ryoku-rashin" "$destdir/usr/bin/ryoku-rashin"
	ln -s ryoku-rashin "$destdir/usr/bin/rashin"
	install -Dm644 "$RYOKU_BUILD_DIR/ryoku-repo.md" "$destdir/usr/share/ryoku/rashin/ryoku-repo.md"
	for file in "$RYOKU_SRC"/ryoku/rashin/skills/ryoku/*.md; do
		install -Dm644 "$file" "$destdir/usr/share/ryoku/skills/ryoku/${file##*/}"
	done
	for file in "$RYOKU_SRC"/ryoku/rashin/wiki/*.md; do
		install -Dm644 "$file" "$destdir/usr/share/ryoku/rashin/wiki/${file##*/}"
	done
	if [ "$init" = systemd ]; then
		install -Dm644 "$RYOKU_SRC/ryoku/rashin/systemd/ryoku-rashin.service" \
			"$destdir/usr/lib/systemd/user/ryoku-rashin.service"
		install -Dm644 "$RYOKU_SRC/ryoku/rashin/systemd/ryoku-prowl.service" \
			"$destdir/usr/lib/systemd/user/ryoku-prowl.service"
	fi
	;;
*)
	printf 'usage: payload.sh build | install DESTDIR systemd|runit\n' >&2
	exit 2
	;;
esac
