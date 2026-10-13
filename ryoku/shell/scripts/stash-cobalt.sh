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
# the picker reads one more line from the probe subcommand, TIERS <json>, which
# carries the tier list of every mode at once: the format table answers in one
# extraction, so it is not extracted again per chip. that answer is cached
# for a day per link, so a link already read costs nothing to ask again.
# usage: stash-cobalt.sh download <url> [auto|audio|mute] [selector]
#          | probe <url> | remux <file> | sites
set -u
# the shell cancels a job by signalling this script, so background work has to
# die with it: job control gives each job its own process group and the group
# kill takes yt-dlp's whole tree, not just the subshell that started it.
set -m
# shellcheck disable=SC2154  # j is bound by the for inside the trap body
trap 'for j in $(jobs -p); do kill -- -$j 2>/dev/null || kill $j 2>/dev/null; done; exit 143' TERM INT

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
# the pipeline runs behind a wait because wait is what a signal interrupts: in
# the foreground the transfer would hold the cancel trap until curl finished.
fetch() {
  local url="$1" dest="$2" rc
  (
    curl -fL --max-time 1800 -A "Mozilla/5.0 (X11; Linux x86_64)" -o "$dest" "$url" 2>&1 \
      | stdbuf -oL tr '\r' '\n' \
      | stdbuf -oL grep -oE '[0-9]{1,3}\.[0-9]' \
      | while read -r p; do emit PROGRESS "${p%.*}"; done
    exit "${PIPESTATUS[0]}"
  ) &
  wait $!
  rc=$?
  return "$rc"
}

# yt-dlp fallback, honours download mode; prints its own clean percent.
# a probe selector outranks the mode's own pick, so the tier the user chose is
# the tier that lands. -x still re-encodes to mp3 on the audio path.
ytdlp() {
  local url="$1" mode="$2" sel="${3:-}" out err wd status f dest saved=0
  out=$(mktemp); err=$(mktemp); wd=$(mktemp -d); trap 'rm -f "$out" "$err"; rm -rf "$wd"' RETURN
  local fmt=(--merge-output-format mp4)
  case "$mode" in
    audio) if [ -n "$sel" ]; then fmt=(-f "$sel" -x --audio-format mp3)
           else fmt=(-x --audio-format mp3); fi ;;
    mute)  if [ -n "$sel" ]; then fmt=(-f "$sel" --merge-output-format mp4)
            else fmt=(-f "bv*" --merge-output-format mp4); fi ;;
    *)     [ -n "$sel" ] && fmt=(-f "$sel" --merge-output-format mp4) ;;
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
# cobalt takes a rung of its quality ladder rather than a yt-dlp selector, so a
# chosen tier has to travel as the height the probe put in the selector. Its
# audio settings stay as they are: one bitrate for every audio download.
cobalt_get() {
  local url="$1" mode="$2" sel="${3:-}" vq=1080 n body resp st durl fn dest
  if [ -n "$sel" ] && { [ "$mode" = auto ] || [ "$mode" = mute ]; }; then
    n=${sel#*height<=}; n=${n%%]*}
    case "$n" in 1080 | 720 | 480 | 360) vq="$n" ;; esac
  fi
  body=$(printf '{"url":%s,"downloadMode":"%s","videoQuality":"%s","audioFormat":"mp3","audioBitrate":"320","filenameStyle":"basic"}' "$(printf '%s' "$url" | jq -Rs .)" "$mode" "$vq")
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

# the format table cut down to what a picker can offer: three heights per video
# mode and three bitrate rungs for audio, each with a size, so the choice is
# priced before the bytes move. One extraction answers every mode, because -J
# already carries all of them.
#
# selectors cap a quality instead of naming a format: the probe and the download
# are separate extractions and YouTube rotates its ids between them, so a pinned
# bv[format_id=251-drc] would resolve at probe time and vanish at download time.
#
# the answer is cached on disk, keyed by the link's own sha256, so pasting a known
# link again answers from the file instead of paying for another -J. the cache is
# a pure speed-up: an unwritable dir, or a stale, truncated or half-written file,
# reads as a miss and the probe runs exactly as it always did.
probe() {
  local url="${1:-}" json err msg tiers
  [ -n "$url" ] || { emit ERROR "no link"; return 2; }
  local cdir ckey cfile now ttl hit
  cdir="${STASH_CACHE_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/ryoku}/stash-probe"
  ckey=$(printf '%s' "$url" | sha256sum | cut -d' ' -f1)
  cfile="$cdir/$ckey.json"
  mkdir -p "$cdir" 2>/dev/null || :
  now=$(date +%s)
  ttl="${STASH_PROBE_TTL:-86400}"
  hit=""
  # a hit needs a readable file that parses, a numeric clock to age it by, an
  # entry still inside its ttl, and tiers actually worth serving. anything else
  # (missing, corrupt, empty, expired) is a miss.
  if [ -r "$cfile" ]; then
    hit=$(jq -c --argjson now "$now" --argjson ttl "$ttl" '
          select((.cached_at | type) == "number")
          | select($now - .cached_at < $ttl)
          | .payload
          | select((.tiers // {}) | length > 0)' "$cfile" 2>/dev/null) || :
  fi
  if [ -n "$hit" ]; then
    emit TIERS "$hit"
    return 0
  fi
  err=$(mktemp)
  if ! json=$(yt-dlp -J --no-playlist --socket-timeout 10 "$url" 2>"$err"); then
    msg=$(tr '\n' ' ' <"$err" | tr -s ' ' | sed 's/^ //; s/ $//')
    rm -f "$err"
    emit ERROR "probe failed: ${msg:-yt-dlp could not read the link}"
    return 1
  fi
  rm -f "$err"
  if ! tiers=$(printf '%s' "$json" | jq -c "$(cat <<'JQ'
def abr: (.abr // .tbr);
def vid($v; $h): ($v | map(select(.height == $h)) | sort_by(-(.tbr // 0)) | .[0]);
# what a tier costs: what the format says outright, else its bitrate over the
# runtime (yt-dlp counts tbr in kbit/s, so the 1000 is real), else nothing to
# quote for a source that reports no length. The bitrates are probe-time truth,
# which is all an estimate has to be.
def size_mb($f; $tbr; $d):
  (if $f == null then null else $f.filesize // $f.filesize_approx end) as $bytes
  | (if $bytes == null then (if $d == null then null else $tbr * $d * 1000 / 8 / 1000000 end)
     else $bytes / 1000000 end)
  | if . == null then null else (. * 10 | round) / 10 end;
# an audio rung is served by the strongest stream at or below it, falling back
# to the strongest overall when the source has nothing small, so one stream can
# serve several rungs. A rung above what the source ever offered is dropped: a
# 320 label off a 160 stream is a lie the size column would contradict.
def audio_tiers($al; $d):
  if ($al | length) == 0 then [] else
  ($al[0].abr) as $top
  | [{key: "high", kbps: 320}, {key: "std", kbps: 128}, {key: "light", kbps: 64}]
  | map(. as $r
      | ($al | map(select(.abr <= $r.kbps)) | .[0] // $al[0]) as $f
      | select($r.kbps <= $top)
      | {key: $r.key, bitrate: $r.kbps, sizeMB: size_mb(null; $f.abr; $d),
         fmt: "ba[abr<=\($r.kbps)]/ba"})
  end;
# two tiers that resolve to the same height are one tier, not two rows claiming
# the same file. A muted source has no audio to price and no audio to fetch.
def video_tiers($v; $al; $d; $silent):
  if ($v | length) == 0 then [] else
  ($al[0].abr // 0) as $audio
  | ([$v[].height] | unique) as $hs
  | $hs[-1] as $best
  | (([$hs[] | select(. <= 720)] | last) // $hs[(($hs | length) / 2 | floor)]) as $mid
  | (([$hs[] | select(. >= 360)] | first) // $hs[0]) as $low
  | [{key: "best", v: vid($v; $best)},
     {key: "avg", v: vid($v; $mid)},
     {key: "low", v: vid($v; $low)}]
  | reduce .[] as $x ([]; if ([.[].v.height] | index($x.v.height)) then . else . + [$x] end)
  | map({key: .key, height: .v.height,
         sizeMB: size_mb(.v; (if $silent then (.v.tbr // 0)
                              else ((.v.tbr // 0) + $audio) end); $d),
         fmt: (if $silent then "bv[height<=\(.v.height)]"
               else "bv[height<=\(.v.height)]+ba/bv*[height<=\(.v.height)]" end)})
  end;
(.formats // []) as $f
| (.duration // null) as $d
| (.title // "") as $ti
| [$f[] | select((.vcodec // "none") != "none" and .height != null)] as $v
| ([$f[] | select((.acodec // "none") != "none" and abr != null)] | sort_by(-abr)) as $al
| if ($v | length) == 0 and ($al | length) == 0 then error("no usable format")
  else (audio_tiers($al; $d)) as $at
    | {duration: $d, title: $ti,
       tiers: (if ($v | length) == 0 then {auto: $at, audio: $at, mute: $at}
               else {auto: video_tiers($v; $al; $d; false),
                     audio: $at,
                     mute: video_tiers($v; $al; $d; true)} end)}
  end
JQ
)" 2>/dev/null); then
    emit ERROR "probe failed: no usable formats"
    return 1
  fi
  emit TIERS "$tiers"
  # the clock is read after the extraction, so the ttl counts from the answer
  probe_cache_write "$cdir" "$ckey" "$tiers"
}

# one file per link, written whole: a temp name in the same directory (same
# filesystem, so the move is a rename) and a final mv, so a reader never catches
# half a payload and a crashed write never truncates yesterday's good answer.
# every failure here is swallowed: the cache is an optimisation, not a step.
probe_cache_write() {
  local cdir="$1" ckey="$2" payload="$3" now tmp
  [ -n "$payload" ] || return 0
  now=$(date +%s)
  tmp=$(mktemp "$cdir/.${ckey}.XXXXXX" 2>/dev/null) || return 0
  if ! jq -cn --argjson ts "$now" --argjson p "$payload" \
    '{cached_at: $ts, payload: $p}' >"$tmp" 2>/dev/null; then
    rm -f "$tmp"
    return 0
  fi
  mv -f "$tmp" "$cdir/$ckey.json" 2>/dev/null || rm -f "$tmp"
}

cmd="${1:-}"
case "$cmd" in
download)
  url="${2:-}"; mode="${3:-auto}"; sel="${4:-}"
  [ -n "$url" ] || { emit ERROR "no link"; exit 2; }
  case "$mode" in auto | audio | mute) ;; *) mode="auto" ;; esac
  emit START
  # engine toggle: only try cobalt when the shell configured a URL (engine on).
  # empty COBALT_API_URL means yt-dlp-only, so skip the probe entirely.
  if [ -n "$COBALT" ] && cobalt_up && cobalt_get "$url" "$mode" "$sel"; then
    notify-send "Stash" "Downloaded via cobalt" -i emblem-ok-symbolic 2>/dev/null || true
    exit 0
  fi
  # no cobalt, or it declined (auth/unsupported): fall back to yt-dlp.
  if ytdlp "$url" "$mode" "$sel"; then
    notify-send "Stash" "Downloaded" -i emblem-ok-symbolic 2>/dev/null || true
    exit 0
  fi
  emit ERROR "download failed"
  notify-send "Stash" "Download failed" -i dialog-error 2>/dev/null || true
  exit 1
  ;;
probe)
  probe "${2:-}"
  exit $?
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
  echo "usage: stash-cobalt.sh download <url> [auto|audio|mute] [selector] | probe <url> | remux <file> | sites" >&2
  exit 2
  ;;
esac
