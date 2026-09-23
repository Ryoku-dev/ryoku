#!/bin/bash
# cobalt-backed media fetcher + local remuxer for the Ryoku stash.
# downloads through a cobalt API instance (https://github.com/imputnet/cobalt)
# when one's reachable, falls back to yt-dlp otherwise so a fresh box still works.
# remux is always a lossless ffmpeg container copy, same thing cobalt's own
# on-device remux does (no re-encode).
#
# point it at your instance with COBALT_API_URL (default http://localhost:9000);
# run one per https://github.com/imputnet/cobalt/blob/main/docs/run-an-instance.md
#
# tab-separated, line-buffered status the stash queue parses:
#   START <name> | PROGRESS <0-100> | SAVED <filename> | ERROR <message>
# usage: stash-cobalt.sh download <url> [auto|audio|mute] | remux <file> | sites
set -u

STASH="${STASH_DIR:-${XDG_DOWNLOAD_DIR:-$HOME/Downloads}/Stash}"
# `-` (not `:-`): only an *unset* var falls back to the default. The shell passes
# COBALT_API_URL="" to mean engine-off (yt-dlp only), and that empty stays empty.
COBALT="${COBALT_API_URL-http://localhost:9000}"
mkdir -p "$STASH"

emit() { printf '%s\t%s\n' "$1" "${2:-}"; }

# free path in the stash for the wanted name: "name.ext", then "name (1).ext".
dest_for() {
  local base; base=$(basename "$1"); [ -n "$base" ] || base="download"
  [ -e "$STASH/$base" ] || { printf '%s' "$STASH/$base"; return; }
  local stem ext i=1; stem="${base%.*}"; ext="${base##*.}"
  [ "$stem" = "$base" ] && ext=""
  while :; do
    local cand="$STASH/$stem ($i)${ext:+.$ext}"
    [ -e "$cand" ] || { printf '%s' "$cand"; return; }
    i=$((i + 1))
  done
}

cobalt_up() { curl -fsS --max-time 2 -H "Accept: application/json" "$COBALT/" >/dev/null 2>&1; }

# fetch one direct URL to DEST. curl's default meter parsed for a coarse %.
fetch() {
  local url="$1" dest="$2" rc
  curl -fL --max-time 1800 -A "Mozilla/5.0 (X11; Linux x86_64)" -o "$dest" "$url" 2>&1 \
    | stdbuf -oL tr '\r' '\n' \
    | stdbuf -oL grep -oE '[0-9]{1,3}\.[0-9]' \
    | while read -r p; do emit PROGRESS "${p%.*}"; done
  rc=${PIPESTATUS[0]}
  return "$rc"
}

# yt-dlp fallback, honours download mode; prints its own clean percent.
ytdlp() {
  local url="$1" mode="$2" out err wd status f dest saved=0
  out=$(mktemp); err=$(mktemp); wd=$(mktemp -d); trap 'rm -f "$out" "$err"; rm -rf "$wd"' RETURN
  local fmt=(--merge-output-format mp4)
  case "$mode" in
    audio) fmt=(-x --audio-format mp3) ;;
    mute)  fmt=(-f "bv*" --merge-output-format mp4) ;;
  esac
  # download into a private dir so distinct videos that happen to share a title
  # (e.g. two captionless posts by the same author) never collide: yt-dlp would
  # otherwise find the first file and skip the rest as "already downloaded",
  # exiting 0 while nothing new lands. dest_for then moves each result into the
  # stash under a free name, matching the cobalt path. stdin from /dev/null so a
  # postprocessor (ffmpeg merge) can never block on the inherited pipe.
  ( yt-dlp --no-playlist --no-mtime --no-warnings --restrict-filenames --newline \
      "${fmt[@]}" -P "$wd" -o "%(title).80s.%(ext)s" "$url" </dev/null >"$out" 2>"$err" ) &
  local pid=$!
  ( tail -f --pid=$pid "$out" 2>/dev/null | stdbuf -oL grep -oE '\[download\] +[0-9]{1,3}\.[0-9]%' \
      | while read -r line; do line=${line##* }; emit PROGRESS "${line%%.*}"; done ) &
  wait $pid; status=$?
  if [ "$status" -ne 0 ]; then tail -n2 "$err" >&2; return "$status"; fi
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    dest=$(dest_for "$(basename "$f")")
    mv -f "$f" "$dest" || return 1
    emit SAVED "$(basename "$dest")"
    saved=1
  done < <(find "$wd" -maxdepth 1 -type f | sort)
  [ "$saved" -eq 1 ] || return 1
}

# cobalt POST + tunnel/redirect/picker; nonzero return triggers the fallback.
cobalt_get() {
  local url="$1" mode="$2" body resp st durl fn dest
  body=$(printf '{"url":%s,"downloadMode":"%s","videoQuality":"1080","audioFormat":"mp3","audioBitrate":"320","filenameStyle":"basic"}' "$(printf '%s' "$url" | jq -Rs .)" "$mode")
  resp=$(curl -fsS --max-time 60 -X POST "$COBALT/" \
    -H "Accept: application/json" -H "Content-Type: application/json" -d "$body" 2>/dev/null) || return 1
  st=$(printf '%s' "$resp" | jq -r '.status // "error"')
  case "$st" in
    tunnel | redirect)
      durl=$(printf '%s' "$resp" | jq -r '.url')
      fn=$(printf '%s' "$resp" | jq -r '.filename // "download"')
      [ -n "$durl" ] && [ "$durl" != "null" ] || return 1
      emit START "$fn"; dest=$(dest_for "$fn")
      fetch "$durl" "$dest" || return 1
      emit SAVED "$(basename "$dest")"
      ;;
    picker)
      local n=0 i u t
      n=$(printf '%s' "$resp" | jq -r '.picker | length')
      [ "$n" -gt 0 ] || return 1
      for i in $(seq 0 $((n - 1))); do
        u=$(printf '%s' "$resp" | jq -r ".picker[$i].url")
        t=$(printf '%s' "$resp" | jq -r ".picker[$i].type // \"media\"")
        [ -n "$u" ] && [ "$u" != "null" ] || continue
        emit START "item $((i + 1)) of $n"
        dest=$(dest_for "cobalt-$t-$((i + 1)).${u##*.}")
        fetch "$u" "$dest" && emit SAVED "$(basename "$dest")"
      done
      ;;
    local-processing)
      # newer cobalt hands the raw parts back and asks the client to finish the
      # job locally (lossless): merge video+audio, drop audio, extract audio, or
      # remux the container. type is merge|mute|audio|gif|remux.
      local otype ofn ntun i tu tmpd rc=0
      otype=$(printf '%s' "$resp" | jq -r '.type // "remux"')
      ofn=$(printf '%s' "$resp" | jq -r '.output.filename // "download"')
      ntun=$(printf '%s' "$resp" | jq -r '(.tunnel // []) | length')
      [ "$ntun" -gt 0 ] || return 1
      emit START "$ofn"
      tmpd=$(mktemp -d) || return 1
      for i in $(seq 0 $((ntun - 1))); do
        tu=$(printf '%s' "$resp" | jq -r ".tunnel[$i]")
        [ -n "$tu" ] && [ "$tu" != "null" ] || { rm -rf "$tmpd"; return 1; }
        fetch "$tu" "$tmpd/part$i" || { rm -rf "$tmpd"; return 1; }
      done
      dest=$(dest_for "$ofn")
      case "$otype" in
        merge) ffmpeg -y -nostdin -hide_banner -loglevel error -i "$tmpd/part0" -i "$tmpd/part1" -map 0:v:0 -map 1:a:0 -c copy -movflags +faststart "$dest" </dev/null || rc=$? ;;
        mute)  ffmpeg -y -nostdin -hide_banner -loglevel error -i "$tmpd/part0" -c copy -an -movflags +faststart "$dest" </dev/null || rc=$? ;;
        audio) ffmpeg -y -nostdin -hide_banner -loglevel error -i "$tmpd/part0" -vn -c:a copy "$dest" </dev/null 2>/dev/null \
          || ffmpeg -y -nostdin -hide_banner -loglevel error -i "$tmpd/part0" -vn "$dest" </dev/null || rc=$? ;;
        gif)   ffmpeg -y -nostdin -hide_banner -loglevel error -i "$tmpd/part0" "$dest" </dev/null || rc=$? ;;
        *)     ffmpeg -y -nostdin -hide_banner -loglevel error -i "$tmpd/part0" -map 0 -c copy -movflags +faststart "$dest" </dev/null || rc=$? ;;
      esac
      rm -rf "$tmpd"
      [ "$rc" -eq 0 ] || { rm -f "$dest"; return 1; }
      emit SAVED "$(basename "$dest")"
      ;;
    *) return 1 ;;
  esac
}

cmd="${1:-}"
case "$cmd" in
download)
  url="${2:-}"; mode="${3:-auto}"
  [ -n "$url" ] || { emit ERROR "no link"; exit 2; }
  case "$mode" in auto | audio | mute) ;; *) mode="auto" ;; esac
  emit START "fetching link"
  # engine toggle: only try cobalt when the shell configured a URL (engine on).
  # empty COBALT_API_URL means yt-dlp-only, so skip the probe entirely.
  if [ -n "$COBALT" ] && cobalt_up && cobalt_get "$url" "$mode"; then
    notify-send "Stash" "Downloaded via cobalt" -i emblem-ok-symbolic 2>/dev/null || true
    exit 0
  fi
  # no cobalt, or it declined (auth/unsupported): fall back to yt-dlp.
  if ytdlp "$url" "$mode"; then
    notify-send "Stash" "Downloaded" -i emblem-ok-symbolic 2>/dev/null || true
    exit 0
  fi
  emit ERROR "download failed"
  notify-send "Stash" "Download failed" -i dialog-error 2>/dev/null || true
  exit 1
  ;;
remux)
  src="${2:-}"
  [ -f "$src" ] || { emit ERROR "file not found"; exit 2; }
  base=$(basename "$src"); stem="${base%.*}"; ext="${base##*.}"
  [ "$stem" = "$base" ] && ext="mp4"
  dest=$(dest_for "$stem.remux.$ext")
  emit START "$base"
  # lossless container rebuild: copy every stream, fix the container + timestamps.
  # no re-encode, near-instant, same as cobalt's remux.
  if ffmpeg -nostdin -hide_banner -loglevel error -i "$src" -map 0 -c copy \
      -movflags +faststart "$dest" </dev/null; then
    emit SAVED "$(basename "$dest")"
    notify-send "Stash" "Remuxed $(basename "$dest")" -i emblem-ok-symbolic 2>/dev/null || true
    exit 0
  fi
  rm -f "$dest"
  emit ERROR "remux failed"
  notify-send "Stash" "Remux failed" -i dialog-error 2>/dev/null || true
  exit 1
  ;;
sites)
  # supported services for the UI bubble: live from the instance when reachable,
  # else the built-in list so the bubble still fills on a fresh box.
  if up=$(curl -fsS --max-time 3 -H "Accept: application/json" "$COBALT/" 2>/dev/null) \
     && list=$(printf '%s' "$up" | jq -r '.cobalt.services[]?' 2>/dev/null) && [ -n "$list" ]; then
    printf '%s\n' "$list"
  else
    printf '%s\n' youtube tiktok twitter instagram reddit twitch vimeo soundcloud \
      bilibili facebook tumblr pinterest vk ok snapchat loom newgrounds streamable \
      rutube dailymotion bsky
  fi
  ;;
*)
  echo "usage: stash-cobalt.sh download <url> [auto|audio|mute] | remux <file> | sites" >&2
  exit 2
  ;;
esac
