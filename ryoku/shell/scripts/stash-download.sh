#!/bin/bash
# URL -> stash, via yt-dlp (sites + raw media).
# usage: stash-download.sh <url>
set -u

STASH="${STASH_DIR:-${XDG_DOWNLOAD_DIR:-$HOME/Downloads}/Stash}"
url="${1:-}"

if [ -z "$url" ]; then
  notify-send "Stash" "No URL to download" -i dialog-error
  echo "usage: stash-download.sh <url>" >&2
  exit 2
fi

mkdir -p "$STASH"
echo "Downloading $url"

# keep stdout (filename markers) + stderr (ERROR lines) apart, so we can both
# parse the destination and surface a clean failure tail.
out=$(mktemp)
err=$(mktemp)
trap 'rm -f "$out" "$err"' EXIT

# yt-dlp picks up ffmpeg off PATH for merges, also handles bare media URLs.
yt-dlp --no-playlist --no-mtime --no-warnings --restrict-filenames \
  -P "$STASH" -o "%(title).80s.%(ext)s" --merge-output-format mp4 "$url" \
  >"$out" 2>"$err"
status=$?

if [ "$status" -ne 0 ]; then
  notify-send "Stash" "Download failed" -i dialog-error
  tail -n 5 "$err" >&2
  echo "Download failed"
  exit "$status"
fi

# a merge prints the final name and shadows the per-stream Destination lines
# above it; a plain download = Destination only; a cached one = neither.
dest=$(sed -n 's/.*Merging formats into "\(.*\)".*/\1/p' "$out" | tail -n1)
[ -n "$dest" ] || dest=$(sed -n 's/^\[download\] Destination: //p' "$out" | tail -n1)
[ -n "$dest" ] || dest=$(sed -n 's/^\[download\] \(.*\) has already been downloaded.*/\1/p' "$out" | tail -n1)
# last resort: newest file in the stash (--no-mtime keeps source mtime).
[ -n "$dest" ] || dest=$(find "$STASH" -maxdepth 1 -type f -printf '%T@\t%p\n' 2>/dev/null | sort -rn | head -n1 | cut -f2-)

case "$dest" in
  /* | "") ;;                 # absolute, or unknown
  *) dest="$STASH/$dest" ;;   # template is relative to -P
esac

if [ -z "$dest" ]; then
  notify-send "Stash" "Downloaded" -i emblem-ok-symbolic
  echo "$STASH"
  exit 0
fi

notify-send "Stash" "Downloaded $(basename "$dest")" -i emblem-ok-symbolic
echo "$dest"
