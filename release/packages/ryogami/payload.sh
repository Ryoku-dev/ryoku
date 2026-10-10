#!/bin/sh
set -eu

: "${RYOKU_SRC:?RYOKU_SRC is required}"
: "${RYOKU_BUILD_DIR:?RYOKU_BUILD_DIR is required}"

case "${1:-}" in
build)
	mkdir -p "$RYOKU_BUILD_DIR"
	cd "$RYOKU_SRC/ryoku/shell/ryogami/daemon"
	CGO_ENABLED=0 go build -trimpath -mod=vendor -o "$RYOKU_BUILD_DIR/ryogami" .
	"$RYOKU_SRC/ryoku/shell/livewall/build.sh" "$RYOKU_BUILD_DIR/ryogami-live"
	RYOGAMI_PICKER_BUILD="$RYOKU_BUILD_DIR/picker-build" \
		"$RYOKU_SRC/ryoku/shell/ryogami/picker/build.sh" "$RYOKU_BUILD_DIR/qml"
	;;
install)
	destdir=${2:?usage: payload.sh install DESTDIR systemd|runit}
	init=${3:?usage: payload.sh install DESTDIR systemd|runit}
	case "$init" in
	systemd|runit) ;;
	*) printf 'payload.sh: unsupported init system: %s\n' "$init" >&2; exit 2 ;;
	esac
	install -Dm755 "$RYOKU_BUILD_DIR/ryogami" "$destdir/usr/bin/ryogami"
	install -Dm755 "$RYOKU_BUILD_DIR/ryogami-live" "$destdir/usr/bin/ryogami-live"
	if [ "$init" = systemd ]; then
		install -Dm644 "$RYOKU_SRC/ryoku/shell/systemd/user/ryogami.service" \
			"$destdir/usr/lib/systemd/user/ryogami.service"
	fi
	install -dm755 "$destdir/usr/lib/qt6/qml/Ryoku/Ryogami"
	cp -a "$RYOKU_BUILD_DIR/qml/Ryoku/Ryogami/." "$destdir/usr/lib/qt6/qml/Ryoku/Ryogami/"
	chmod -R u=rwX,go=rX "$destdir/usr/lib/qt6"
	install -Dm644 "$RYOKU_SRC/ryoku/shell/ryogami/picker/shell.qml" \
		"$destdir/usr/share/ryogami/shell.qml"
	install -Dm644 "$RYOKU_SRC/ryoku/shell/ryogami/picker/data/ryogami.desktop" \
		"$destdir/usr/share/applications/ryogami.desktop"
	install -Dm644 "$RYOKU_SRC/ryoku/shell/ryogami/picker/LICENSE" \
		"$destdir/usr/share/licenses/ryogami/LICENSE"
	install -Dm644 "$RYOKU_SRC/ryoku/shell/ryogami/picker/NOTICE" \
		"$destdir/usr/share/licenses/ryogami/NOTICE"
	install -Dm644 -t "$destdir/usr/share/licenses/ryogami/" \
		"$RYOKU_SRC"/ryoku/shell/ryogami/picker/qml/theme/fonts/*.txt
	;;
*)
	printf 'usage: payload.sh build | install DESTDIR systemd|runit\n' >&2
	exit 2
	;;
esac
