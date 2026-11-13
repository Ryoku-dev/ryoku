#!/bin/bash
set -eu

: "${RYOKU_SRC:?RYOKU_SRC is required}"
: "${RYOKU_BUILD_DIR:?RYOKU_BUILD_DIR is required}"

model=parakeet-tdt_ctc-110m-Q8_0.gguf
build="$RYOKU_BUILD_DIR/build"

case "${1:-}" in
build)
	mkdir -p "$RYOKU_BUILD_DIR"
	# TRANSCRIBE_SRC points at an unpacked engine tree for the builds that have
	# no network (Nix fetches it as a fixed-output source); otherwise CMake
	# fetches the pinned tarball itself.
	engine=()
	[ -n "${TRANSCRIBE_SRC:-}" ] && engine=(-DTRANSCRIBE_SRC="$TRANSCRIBE_SRC")
	cmake -S "$RYOKU_SRC/ryoku/dictation" -B "$build" -G Ninja \
		-DCMAKE_BUILD_TYPE=Release \
		-DRYOKU_DICTATION_TEST=ON \
		-DRYOKU_DICTATION_GPU=OFF \
		"${engine[@]}"
	cmake --build "$build" -j"$(nproc)"
	;;
install)
	destdir=${2:?usage: payload.sh install DESTDIR systemd|runit}
	init=${3:?usage: payload.sh install DESTDIR systemd|runit}
	case "$init" in
	systemd|runit) ;;
	*) printf 'payload.sh: unsupported init system: %s\n' "$init" >&2; exit 2 ;;
	esac

	# One directory holds the engine library and every ggml backend module,
	# because that is where the daemon looks for the optional GPU backend.
	# The rpath is written at link time, so a plain copy is the installed file.
	install -Dm755 -s "$build/ryoku-dictate" "$destdir/usr/bin/ryoku-dictate"
	libdir="$destdir/usr/lib/ryoku/dictation"
	mkdir -p "$libdir"
	while IFS= read -r -d '' lib; do
		install -m755 -s "$lib" "$libdir/$(basename "$lib")"
	done < <(find "$build" \( -name 'libtranscribe.so*' -o -name 'libggml*.so*' \) \
		-not -type d -not -type l -print0 | sort -zu)
	while IFS= read -r -d '' link; do
		ln -sfn "$(basename "$(readlink -f "$link")")" "$libdir/$(basename "$link")"
	done < <(find "$build" -type l \( -name 'libtranscribe.so*' -o -name 'libggml*.so*' \) -print0)
	install -Dm644 "$RYOKU_SRC/ryoku/dictation/NOTICE.md" \
		"$destdir/usr/share/licenses/ryoku-dictation/NOTICE.md"
	install -Dm644 "$RYOKU_SRC/ryoku/dictation/README.md" \
		"$destdir/usr/share/doc/ryoku-dictation/README.md"

	# The model the package ships: dictation works on first boot, offline, with
	# no download. It is never loaded until the user turns dictation on.
	if [ -f "$RYOKU_BUILD_DIR/$model" ]; then
		install -Dm644 "$RYOKU_BUILD_DIR/$model" \
			"$destdir/usr/share/ryoku/dictation/models/$model"
	fi

	if [ "$init" = systemd ]; then
		install -Dm644 "$RYOKU_SRC/ryoku/dictation/systemd/user/ryoku-dictate.service" \
			"$destdir/usr/lib/systemd/user/ryoku-dictate.service"
	fi
	;;
*)
	printf 'usage: payload.sh build | install DESTDIR systemd|runit\n' >&2
	exit 2
	;;
esac
