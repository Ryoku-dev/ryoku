#!/usr/bin/env bash
# Fixture test for ryoku-cmd-radio: the start/stop/status/aside contract the
# launcher's "@" provider builds on. Runs the real script with a stub mpv
# (writes the URL it was handed, then sleeps) and a stub yt-dlp whose behavior
# a marker file controls, so both the happy path and the fall-back-to-direct
# path run without network, sound, or a real player.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
radio="$here/../ryoku/shell/scripts/ryoku-cmd-radio"

command -v jq >/dev/null 2>&1 || { echo "SKIP: jq required" >&2; exit 0; }

work="$(mktemp -d)"
# stop FIRST: the state file (the only record of the supervisor's pgid) lives
# inside $work, deleting it first would turn stop into a no-op and strand the
# stub player for its full 600s nap whenever a case aborts mid-broadcast.
trap '"$radio" stop >/dev/null 2>&1 || true; rm -rf "$work"' EXIT

export RYOKU_RADIO_STATE_FILE="$work/state.json"
export RYOKU_RADIO_ASIDE_FILE="$work/aside.json"
export RYOKU_RADIO_LOCK_FILE="$work/lock"
export RYOKU_RADIO_MPV="$work/bin/mpv"
export RYOKU_RADIO_YTDLP="$work/bin/yt-dlp"
export RYOKU_RADIO_RESOLVE_TIMEOUT=5

mkdir -p "$work/bin"
cat >"$work/bin/mpv" <<EOF
#!/bin/sh
for a in "\$@"; do last="\$a"; done
printf '%s\n' "\$last" >>"$work/played.log"
sleep 600
EOF
cat >"$work/bin/yt-dlp" <<EOF
#!/bin/sh
[ -e "$work/yt-broken" ] && exit 1
echo "https://resolved.example/hls.m3u8"
EOF
chmod +x "$work/bin/mpv" "$work/bin/yt-dlp"

fail=0; pass=0
ok()  { printf 'PASS %s\n' "$1"; pass=$((pass + 1)); }
bad() { printf 'FAIL %s\n  %s\n' "$1" "$2" >&2; fail=$((fail + 1)); }

wait_played() { # $1 = expected line count
  for _ in $(seq 1 40); do
    [ "$( { wc -l <"$work/played.log"; } 2>/dev/null || echo 0)" -ge "$1" ] && return 0
    sleep 0.25
  done
  return 1
}

# ---- stations: the catalog rows the provider reads --------------------------
# headliners first (youtube, each with a direct fallback), directs after; the
# direct stations carry their published cover art in the catalog itself.
if "$radio" stations | jq -se '
    .[0].id=="lofi" and .[0].fallback=="groove"
    and (map(select(.kind=="youtube")) | all(.fallback != ""))
    and (map(select(.kind=="direct")) | length >= 2)
    and (map(select(.kind=="direct")) | all(.art | length > 0))' >/dev/null; then
  ok "stations catalog shape"
else
  bad "stations catalog shape" "$("$radio" stations)"
fi

# ---- off status --------------------------------------------------------------
if "$radio" status | jq -e '.available==true and .on==false and .aside==null' >/dev/null; then
  ok "silent status reports the player"
else
  bad "silent status reports the player" "$("$radio" status)"
fi

# ---- start: resolves and hands mpv the stream --------------------------------
"$radio" start lofi
if wait_played 1 && grep -q 'resolved.example' "$work/played.log"; then
  ok "start resolves lofi via yt-dlp and plays it"
else
  bad "start resolves lofi via yt-dlp and plays it" "played=$(cat "$work/played.log" 2>/dev/null)"
fi
if "$radio" status | jq -e '.on==true and .station=="lofi" and .live==true and .fellBack==false' >/dev/null; then
  ok "on-air status"
else
  bad "on-air status" "$("$radio" status)"
fi

# ---- stop: kills the player group and clears state ---------------------------
"$radio" stop
sleep 0.5
if "$radio" status | jq -e '.on==false' >/dev/null && ! pgrep -f "$work/bin/mpv" >/dev/null; then
  ok "stop kills the radio"
else
  bad "stop kills the radio" "$("$radio" status); pgrep=$(pgrep -f "$work/bin/mpv" || true)"
fi

# ---- broken resolver: falls back to the direct station -----------------------
: >"$work/played.log"
touch "$work/yt-broken"
"$radio" start lofi
if wait_played 1 && grep -q 'somafm.com' "$work/played.log"; then
  ok "dead yt-dlp falls back to the direct stream"
else
  bad "dead yt-dlp falls back to the direct stream" "played=$(cat "$work/played.log" 2>/dev/null)"
fi
if "$radio" status | jq -e '.on==true and .fellBack==true' >/dev/null; then
  ok "fallback is honest in status"
else
  bad "fallback is honest in status" "$("$radio" status)"
fi
if "$radio" status | jq -e '.art | contains("somafm")' >/dev/null; then
  ok "fallback carries the station cover"
else
  bad "fallback carries the station cover" "$("$radio" status)"
fi
rm -f "$work/yt-broken"

# ---- aside: stop --aside remembers, resume picks it back up ------------------
"$radio" stop --aside
sleep 0.5
if "$radio" status | jq -e '.on==false and .aside.station=="lofi"' >/dev/null; then
  ok "stop --aside remembers the station"
else
  bad "stop --aside remembers the station" "$("$radio" status)"
fi
: >"$work/played.log"
"$radio" resume
if wait_played 1 && "$radio" status | jq -e '.on==true and .station=="lofi" and .aside==null' >/dev/null; then
  ok "resume tunes the aside station back in"
else
  bad "resume tunes the aside station back in" "$("$radio" status)"
fi

# ---- toggle off --------------------------------------------------------------
"$radio" toggle
sleep 0.5
if "$radio" status | jq -e '.on==false' >/dev/null; then
  ok "toggle tunes out"
else
  bad "toggle tunes out" "$("$radio" status)"
fi

# ---- back-to-back starts leave exactly one broadcast --------------------------
# regression: a second start landing in the pgid-registration window used to
# spawn a sibling supervisor the stop could never find, doubled audio, an
# orphan streaming until logout. start now holds the lock until the group is
# stamped, and the broadcast token disowns any predecessor mid-lap.
: >"$work/played.log"
"$radio" start lofi
"$radio" start lofi
sleep 2
sup_count="$(pgrep -fc "_supervise lofi" || true)"
mpv_count="$(pgrep -fc "$work/bin/mpv" || true)"
if [ "${sup_count:-0}" -eq 1 ] && [ "${mpv_count:-0}" -eq 1 ]; then
  ok "double start keeps a single broadcast"
else
  bad "double start keeps a single broadcast" "supervisors=$sup_count players=$mpv_count"
fi
"$radio" stop
sleep 1
if ! pgrep -f "_supervise lofi" >/dev/null && ! pgrep -f "$work/bin/mpv" >/dev/null; then
  ok "stop after double start leaves nothing behind"
else
  bad "stop after double start leaves nothing behind" "$(pgrep -af "_supervise lofi|$work/bin/mpv" || true)"
fi

# ---- toggle;toggle is on-then-off, never a second radio -----------------------
"$radio" toggle lofi
"$radio" toggle lofi
sleep 1
if "$radio" status | jq -e '.on==false' >/dev/null \
  && ! pgrep -f "_supervise lofi" >/dev/null; then
  ok "rapid toggle pair lands off"
else
  bad "rapid toggle pair lands off" "$("$radio" status); $(pgrep -af '_supervise lofi' || true)"
fi
# ---- missing player is explicit ----------------------------------------------
mv "$work/bin/mpv" "$work/bin/mpv.off"
if "$radio" status | jq -e '.available==false and .on==false' >/dev/null; then
  ok "missing mpv is explicit in status"
else
  bad "missing mpv is explicit in status" "$("$radio" status)"
fi
mv "$work/bin/mpv.off" "$work/bin/mpv"


# ---- one title prefix across the stack ----------------------------------------
# the engine's forced title, the launcher's matcher and the media service's copy
# must agree, or the broadcast dress silently falls off a surface.
eng="$here/../ryoku/shell/scripts/ryoku-cmd-radio"
if grep -q 'force-media-title="LIVE · ' "$eng" \
  && grep -q 'TITLE_PREFIX = "LIVE · "' "$here/../ryoku/shell/quickshell/shell/modules/launcher/shared/lib/radio.js" \
  && grep -q '"LIVE · "' "$here/../ryoku/shell/quickshell/shell/services/Media.qml"; then
  ok "LIVE title prefix agrees across engine, launcher, media"
else
  bad "LIVE title prefix agrees across engine, launcher, media" "grep the three files for 'LIVE · '"
fi

printf '%d passed, %d failed\n' "$pass" "$fail"
exit "$((fail > 0 ? 1 : 0))"
