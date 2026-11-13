#!/bin/sh
set -eu

: "${RYOKU_SRC:?RYOKU_SRC is required}"
: "${RYOKU_BUILD_DIR:?RYOKU_BUILD_DIR is required}"

build_dir="$RYOKU_BUILD_DIR/build-vulkan"

case "${1:-}" in
build)
	mkdir -p "$RYOKU_BUILD_DIR"
	cmake -S "$RYOKU_SRC/ryoku/dictation" -B "$build_dir" -G Ninja \
		-DCMAKE_BUILD_TYPE=Release \
		-DRYOKU_DICTATION_GPU=ON \
		-DRYOKU_DICTATION_TEST=OFF
	cmake --build "$build_dir" -j"$(nproc)"
	;;
install)
	destdir=${2:?usage: payload.sh install DESTDIR}
	# Only the Vulkan module and its shader blob ship here. The core, the CPU
	# module and the binary belong to ryoku-dictation at the same version.
	for module in "$build_dir"/bin/libggml-vulkan.so*; do
		[ -e "$module" ] || continue
		install -Dm755 "$module" "$destdir/usr/lib/ryoku/dictation/$(basename "$module")"
	done
	install -Dm644 "$RYOKU_SRC/ryoku/dictation/NOTICE.md" \
		"$destdir/usr/share/licenses/ryoku-dictation-vulkan/NOTICE.md"
	;;
*)
	printf 'usage: payload.sh build | install DESTDIR\n' >&2
	exit 2
	;;
esac
