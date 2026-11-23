#!/bin/sh
# Stands in for yt-dlp: -J prints $STUB_JSON, otherwise it plays a download.
if [ "$2" = "--help" ]; then
	[ -n "$STUB_HELP_COUNT" ] && echo x >> "$STUB_HELP_COUNT"
	[ -n "$STUB_HELP" ] && echo "    --js-runtimes [RUNTIME[:PATH]]  Additional JavaScript runtime to enable"
	exit 0
fi
out="" tmpl="" url="" dumpjson=""
prev=""
for a in "$@"; do
	case "$prev" in
	-o) out="$a" ;;
	--progress-template) tmpl="${a#download:}" ;;
	esac
	case "$a" in -J) dumpjson=1 ;; esac
	prev="$a"
	url="$a"
done
[ -n "$STUB_ARGS" ] && printf '%s\n' "$@" > "$STUB_ARGS"
case "$url" in
*fail*) echo "WARNING: something" >&2; echo "ERROR: [generic] Private video. Sign in if you've been granted access" >&2; exit 1 ;;
*nope*) echo "ERROR: Unsupported URL: $url" >&2; exit 1 ;;
esac
if [ -n "$dumpjson" ]; then cat "$STUB_JSON"; exit 0; fi
trap 'touch "$out.interrupted"; exit 130' INT
prefix="${tmpl%%%(progress)j}"
final="$out.mp4"
for n in 0 400 800; do
	echo "${prefix}{\"status\": \"downloading\", \"downloaded_bytes\": $n, \"total_bytes\": 1000, \"filename\": \"$out.f1.mp4\"}"
	[ -n "$STUB_SLOW" ] && sleep 5
done
echo "${prefix}{\"status\": \"finished\", \"downloaded_bytes\": 1000, \"total_bytes\": 1000, \"filename\": \"$out.f1.mp4\"}"
echo "${prefix}{\"status\": \"downloading\", \"downloaded_bytes\": 250, \"total_bytes_estimate\": 500, \"filename\": \"$out.f2.m4a\"}"
echo "${prefix}{\"status\": \"finished\", \"downloaded_bytes\": 500, \"total_bytes\": 500, \"filename\": \"$out.f2.m4a\"}"
echo "[Merger] Merging formats into \"$final\""
printf 'merged' > "$final"
echo "RYOGET-PATH $final"
