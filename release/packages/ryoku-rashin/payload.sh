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
	cd "$RYOKU_SRC/ryoku/rashin/backend"
	CGO_ENABLED=0 go build -trimpath -mod=vendor -o "$RYOKU_BUILD_DIR/ryoku-rashin" .
	"$RYOKU_BUILD_DIR/ryoku-rashin" repo-index "$RYOKU_SRC" "$RYOKU_BUILD_DIR/ryoku-repo.md"
	cmake -S "$RYOKU_SRC/ryoku/rashin/app" -B "$RYOKU_BUILD_DIR/rashin-app-build" -G Ninja \
		-DCMAKE_BUILD_TYPE=Release
	cmake --build "$RYOKU_BUILD_DIR/rashin-app-build"
	cp "$RYOKU_BUILD_DIR/rashin-app-build/rashin-app" "$RYOKU_BUILD_DIR/rashin-app"
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
	# The companion window (Super+Alt+Space) and its launcher entry.
	install -Dm755 "$RYOKU_BUILD_DIR/rashin-app" "$destdir/usr/bin/rashin-app"
	install -Dm644 "$RYOKU_SRC/ryoku/rashin/app/rashin-app.desktop" \
		"$destdir/usr/share/applications/rashin-app.desktop"
	install -Dm644 "$RYOKU_SRC/ryoku/rashin/app/logo.svg" \
		"$destdir/usr/share/icons/hicolor/scalable/apps/rashin-app.svg"
	# The guard runs a collector only for a tool that is installed or has its
	# local data, so an idle box with none of them launches nothing.
	for collector in ryoku-ai-usage claude-usage codex-usage opencode-usage grok-usage; do
		install -Dm755 "$RYOKU_SRC/ryoku/rashin/usage/$collector" "$destdir/usr/bin/$collector"
	done
	if [ "$init" = systemd ]; then
		install -Dm644 "$RYOKU_SRC/ryoku/rashin/systemd/ryoku-rashin.service" \
			"$destdir/usr/lib/systemd/user/ryoku-rashin.service"
		install -Dm644 "$RYOKU_SRC/ryoku/rashin/systemd/ryoku-prowl.service" \
			"$destdir/usr/lib/systemd/user/ryoku-prowl.service"
		# In the system user unit dir, so the service manifest's one-time
		# `systemctl --global enable` finds the timer.
		install -Dm644 "$RYOKU_SRC/ryoku/rashin/systemd/ryoku-ai-usage.service" \
			"$destdir/usr/lib/systemd/user/ryoku-ai-usage.service"
		install -Dm644 "$RYOKU_SRC/ryoku/rashin/systemd/ryoku-ai-usage.timer" \
			"$destdir/usr/lib/systemd/user/ryoku-ai-usage.timer"
	fi
	;;
*)
	printf 'usage: payload.sh build | install DESTDIR systemd|runit\n' >&2
	exit 2
	;;
esac
