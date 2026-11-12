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
	cd "$RYOKU_SRC/ryoku/rashin"
	ldflags="-s -w -X main.version=${RYOKU_PKGVER:-dev}"
	CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags "$ldflags" \
		-o "$RYOKU_BUILD_DIR/rashind" ./cmd/rashind
	CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags "$ldflags" \
		-o "$RYOKU_BUILD_DIR/rashin" ./cmd/rashin
	CGO_ENABLED=1 go build -mod=vendor -trimpath -tags nox11 -ldflags "$ldflags" \
		-o "$RYOKU_BUILD_DIR/rashin-app" ./cmd/rashin-app
	;;
install)
	destdir=${2:?usage: payload.sh install DESTDIR systemd|runit}
	init=${3:?usage: payload.sh install DESTDIR systemd|runit}
	case "$init" in
	systemd|runit) ;;
	*) printf 'payload.sh: unsupported init system: %s\n' "$init" >&2; exit 2 ;;
	esac
	for binary in rashind rashin rashin-app; do
		install -Dm755 "$RYOKU_BUILD_DIR/$binary" "$destdir/usr/bin/$binary"
	done
	install -Dm644 "$RYOKU_SRC/release/packages/ryoku-rashin/dev.ryoku.rashin.desktop" \
		"$destdir/usr/share/applications/dev.ryoku.rashin.desktop"
	install -Dm644 "$RYOKU_SRC/ryoku/assets/brand/rashin-mark.svg" \
		"$destdir/usr/share/icons/hicolor/scalable/apps/dev.ryoku.rashin.svg"
	for collector in ryoku-ai-usage claude-usage codex-usage opencode-usage; do
		install -Dm755 "$RYOKU_SRC/ryoku/rashin/usage/$collector" "$destdir/usr/bin/$collector"
	done
	install -Dm644 "$RYOKU_SRC/LICENSE" \
		"$destdir/usr/share/licenses/ryoku-rashin/LICENSE"
	install -Dm644 "$RYOKU_SRC/ryoku/rashin/NOTICE" \
		"$destdir/usr/share/licenses/ryoku-rashin/NOTICE"
	# the app draws Material Symbols, whose license ships beside the vendored icons
	install -Dm644 "$RYOKU_SRC/ryoku/rashin/vendor/github.com/ryoku-dev/ryoku/docs/lithic/library/go/symbols/LICENSE" \
		"$destdir/usr/share/licenses/ryoku-rashin/LICENSE.material-symbols"
	# it sets its type in Geologica and Martian Mono, embedded from the vendored Lithic fonts
	install -Dm644 "$RYOKU_SRC/ryoku/rashin/vendor/github.com/ryoku-dev/ryoku/docs/lithic/library/go/fonts/Geologica-OFL.txt" \
		"$destdir/usr/share/licenses/ryoku-rashin/LICENSE.geologica"
	install -Dm644 "$RYOKU_SRC/ryoku/rashin/vendor/github.com/ryoku-dev/ryoku/docs/lithic/library/go/fonts/MartianMono-OFL.txt" \
		"$destdir/usr/share/licenses/ryoku-rashin/LICENSE.martian-mono"
	# the vendored Hindsight client checks values with validator.v2, under Apache-2.0
	install -Dm644 "$RYOKU_SRC/ryoku/rashin/vendor/gopkg.in/validator.v2/LICENSE" \
		"$destdir/usr/share/licenses/ryoku-rashin/LICENSE.validator"
	# the memory learner, card and tools adapt Codex, mem0, Letta and Letta Code, under Apache-2.0
	install -Dm644 "$RYOKU_SRC/ryoku/rashin/LICENSE.apache-2.0" \
		"$destdir/usr/share/licenses/ryoku-rashin/LICENSE.apache-2.0"
	# the built-in skill sets: Superpowers, OpenSpec and Spec Kit under MIT, EGM under Apache-2.0
	for set in superpowers openspec speckit egm; do
		install -Dm644 "$RYOKU_SRC/ryoku/rashin/internal/skills/sets/$set/LICENSE" \
			"$destdir/usr/share/licenses/ryoku-rashin/LICENSE.$set"
	done
	if [ "$init" = systemd ]; then
		for unit in rashind.socket rashind.service ryoku-ai-usage.service ryoku-ai-usage.timer \
			rashind-routines.service rashind-routines.timer; do
			install -Dm644 "$RYOKU_SRC/ryoku/rashin/systemd/$unit" \
				"$destdir/usr/lib/systemd/user/$unit"
		done
	fi
	;;
*)
	printf 'usage: payload.sh build | install DESTDIR systemd|runit\n' >&2
	exit 2
	;;
esac
