#!/bin/bash
# 'noun' below is a literal "file"/"files" label, not $(file) output.
# shellcheck disable=SC2209
# shrink stashed videos/images via ffmpeg/magick. writes "<name>.min.<ext>" next to the original.
# usage: stash-compress.sh [file]   (no arg = every video/image directly in $STASH, non-recursive)
set -u

STASH="${STASH_DIR:-${XDG_DOWNLOAD_DIR:-$HOME/Downloads}/Stash}"

notify() { notify-send "Stash" "$1" -i "$2"; }

# human size, MB-style labels, 1024 base (matches "saved 45.2 MB").
human() {
  awk -v b="$1" 'BEGIN{
    split("B KB MB GB TB", u, " "); i=1; x=b;
    while (x>=1024 && i<5) { x/=1024; i++ }
    if (i==1) printf "%d %s", x, u[i]; else printf "%.1f %s", x, u[i];
  }'
}

# media class from extension (lowercased). empty = nothing to compress.
classify() {
  local ext="${1##*.}"; ext="${ext,,}"
  case "$ext" in
    mp4|mkv|mov|webm|avi|m4v) echo video ;;
    jpg|jpeg)                 echo jpg ;;
    png)                      echo png ;;
    webp)                     echo webp ;;
    bmp)                      echo bmp ;;
    *)                        echo "" ;;
  esac
}

# re-encode in -> out at the same res (never upscales). ffmpeg keeps quality + audio.
# png/bmp go through magick: this ffmpeg's png encoder has no zlib knob, so it
# can't actually compress them.
encode() {
  local in="$1" kind="$2" out="$3"
  case "$kind" in
    video) ffmpeg -nostdin -hide_banner -loglevel error -y -i "$in" \
             -c:v libx264 -crf 24 -preset medium -c:a aac -b:a 128k -movflags +faststart "$out" ;;
    jpg)   ffmpeg -nostdin -hide_banner -loglevel error -y -i "$in" -q:v 3 "$out" ;;
    webp)  ffmpeg -nostdin -hide_banner -loglevel error -y -i "$in" -c:v libwebp -quality 82 "$out" ;;
    png|bmp) magick "$in" -strip -define png:compression-level=9 "$out" ;;
  esac
}

# targets: explicit paths (from the in-shell picker) use those; no args falls
# back to every regular file directly in the stash.
declare -a targets=()
case "${1:-}" in
  "")
    if [ ! -d "$STASH" ]; then
      notify "Stash folder not found" dialog-error
      echo "no stash dir: $STASH"; exit 1
    fi
    shopt -s nullglob
    for f in "$STASH"/*; do [ -f "$f" ] && targets+=("$f"); done
    shopt -u nullglob
    ;;
  *)
    targets=("$@")
    for f in "${targets[@]}"; do
      [ -f "$f" ] || { notify "File not found" dialog-error; echo "not found: $f"; exit 1; }
    done
    ;;
esac
single=0; [ "${#targets[@]}" -eq 1 ] && single=1

compressed=0; optimal=0; failed=0; saved=0

for in in "${targets[@]}"; do
  bn=$(basename -- "$in")
  case "$bn" in *.min.*) continue ;; esac   # don't re-compress our own outputs

  kind=$(classify "$in")
  if [ -z "$kind" ]; then
    if [ "$single" -eq 1 ]; then
      notify "Can't compress: $bn" dialog-error
      echo "unsupported: $bn"; exit 1
    fi
    continue
  fi

  case "$kind" in
    video)   out="${in%.*}.min.mp4" ;;
    jpg)     out="${in%.*}.min.jpg" ;;
    webp)    out="${in%.*}.min.webp" ;;
    png|bmp) out="${in%.*}.min.png" ;;
  esac
  [ -e "$out" ] && { echo "exists, skipping: $(basename -- "$out")"; continue; }

  echo "compressing $bn"
  if ! encode "$in" "$kind" "$out" || [ ! -s "$out" ]; then
    rm -f "$out"; failed=$((failed + 1)); echo "failed: $bn"; continue
  fi

  in_size=$(stat -c%s "$in"); out_size=$(stat -c%s "$out")
  # re-encode that didn't shrink = wasted bytes. drop it, keep the original.
  if [ "$out_size" -ge "$in_size" ]; then
    rm -f "$out"; optimal=$((optimal + 1)); echo "already optimal: $bn"
  else
    compressed=$((compressed + 1)); saved=$((saved + in_size - out_size))
    echo "compressed $bn: $(human "$in_size") -> $(human "$out_size")"
  fi
done

if [ "$compressed" -gt 0 ]; then
  noun=files; [ "$compressed" -eq 1 ] && noun=file
  msg="Compressed $compressed $noun, saved $(human "$saved")"
  notify "$msg" emblem-ok-symbolic; echo "$msg"; exit 0
elif [ "$failed" -gt 0 ]; then
  notify "Compression failed" dialog-error; echo "Compression failed"; exit 1
elif [ "$optimal" -gt 0 ]; then
  notify "All files already optimal" emblem-ok-symbolic; echo "All files already optimal"; exit 0
else
  notify "Nothing to compress" emblem-ok-symbolic; echo "Nothing to compress"; exit 0
fi
