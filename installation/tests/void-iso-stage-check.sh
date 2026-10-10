#!/usr/bin/env bash
set -euo pipefail

HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
BUILD=$HERE/../../void/iso/build.sh

if ! command -v go >/dev/null 2>&1; then
	printf '%s\n' 'void-iso-stage-check: SKIP (go not found)'
	exit 0
fi
[[ -x $BUILD ]] || {
	printf 'void-iso-stage-check: FAIL (build script is not executable: %s)\n' "$BUILD" >&2
	exit 1
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

run_stage() {
	local number=$1
	if ! RYOKU_VOID_ISO_STAGE="$tmp/stage$number" \
		RYOKU_VOID_ISO_WORK="$tmp/work$number" \
		RYOKU_VOID_ISO_OUT="$tmp/out$number" \
		"$BUILD" --stage-only > "$tmp/log$number" 2>&1; then
		printf 'void-iso-stage-check: FAIL (staging run %s errored)\n' "$number" >&2
		cat "$tmp/log$number" >&2
		exit 1
	fi
}

printf '%s\n' 'void-iso-stage-check: staging run 1 ...'
run_stage 1
printf '%s\n' 'void-iso-stage-check: staging run 2 ...'
run_stage 2

rm -f "$tmp/stage1/rootfs/usr/share/ryoku/.payload"
rm -f "$tmp/stage2/rootfs/usr/share/ryoku/.payload"
if diff --no-dereference -qr "$tmp/stage1" "$tmp/stage2" > "$tmp/diff" 2>&1; then
	printf '%s\n' 'void-iso-stage-check: PASS (staging tree is reproducible)'
else
	printf '%s\n' 'void-iso-stage-check: FAIL (staging tree differs between runs)' >&2
	cat "$tmp/diff" >&2
	exit 1
fi
