#!/usr/bin/env bash
# Behaviour of ryoku-cmd-record around GPU Screen Recorder (GSR), run against a
# stand-in gpu-screen-recorder so no GPU or display is needed. It pins what users
# hit: a machine without a hardware encoder must still record (CPU, or another
# hardware codec), a failed GSR probe must never block a capture, region boxes go
# straight to -w, and a capture that dies explains itself (an NVIDIA update that
# needs a restart, a session with no GPU driver, or GSR's own words). It also pins
# the state the shell reads: recorder-status.json while a capture is live (target,
# pause accounting, the stopping flag) and recorder-result.json for every start,
# written before the status file goes.
set -euo pipefail

repo="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
# Every capture runs from $tmp, so on any exit, a failed check included, the
# background jobs and anything still running from that tree are killed and
# waited for, and no stand-in capture outlives the test.
started=()
spawn() {
  "$@" &
  started+=("$!")
}
reap() {
  local pid _
  for pid in "${started[@]}"; do kill -TERM "$pid" 2>/dev/null || true; done
  pkill -TERM -f -- "$tmp/" 2>/dev/null || true
  for _ in $(seq 40); do
    pgrep -f -- "$tmp/" >/dev/null 2>&1 || break
    sleep 0.05
  done
  pkill -KILL -f -- "$tmp/" 2>/dev/null || true
  for pid in "${started[@]}"; do wait "$pid" 2>/dev/null || true; done
  rm -rf "$tmp"
}
trap reap EXIT
trap 'exit 130' INT TERM

fail() { echo "FAIL: $*" >&2; exit 1; }

bin="$tmp/bin"
mkdir -p "$bin" "$tmp/under-test" "$tmp/lib"
# A copy, so the post-save hook resolves to the stand-in below, not its sibling.
cp "$repo/ryoku/shell/scripts/ryoku-cmd-record" "$tmp/under-test/ryoku-cmd-record"
recorder="$tmp/under-test/ryoku-cmd-record"
events="$tmp/events"
argv_file="$tmp/gsr-argv"

cat >"$bin/gpu-screen-recorder" <<'EOF'
#!/usr/bin/env bash
if [[ ${1:-} == --info ]]; then
  [[ -f $GSR_INFO ]] && cat "$GSR_INFO"
  exit "${GSR_INFO_RC:-0}"
fi
printf '%s\n' "$*" >"$GSR_ARGV"
if [[ $GSR_MODE == fail ]]; then
  printf '%s\n' "$GSR_STDERR" >&2
  exit 1
fi
out=""
while (($# > 0)); do
  [[ $1 == -o ]] && out="$2"
  shift
done
# live: answer gsr-cli until it asks for a stop, then take a moment to save.
if [[ $GSR_MODE == live ]]; then
  : >"$XDG_RUNTIME_DIR/stand-in-gsr.live"
  # bounded, and gone with the test's tree, so a stand-in never waits forever
  for _ in $(seq 600); do
    [[ -d $XDG_RUNTIME_DIR && ! -f $XDG_RUNTIME_DIR/stand-in-gsr.stop ]] || break
    sleep 0.05
  done
  sleep "${GSR_SAVE_SECONDS:-0}"
  printf 'video' >"$out"
  rm -f "$XDG_RUNTIME_DIR/stand-in-gsr.live" "$XDG_RUNTIME_DIR/stand-in-gsr.stop"
  exit 0
fi
printf 'video' >"$out"
EOF
# Speaks for the live stand-in above; with no live capture every verb fails, as
# the real gsr-cli does on a dead socket.
cat >"$bin/gsr-cli" <<'EOF'
#!/usr/bin/env bash
[[ -f $XDG_RUNTIME_DIR/stand-in-gsr.live ]] || exit 1
case "${3:-}" in
set-paused) printf 'set-paused %s\n' "${4:-}" >>"$EVENTS" ;;
stop)
  printf 'stop status=%s\n' "$(cat "$RYOKU_STATE_PATH/recorder-status.json" 2>/dev/null)" >>"$EVENTS"
  : >"$XDG_RUNTIME_DIR/stand-in-gsr.stop"
  for _ in $(seq 600); do
    [[ -f $XDG_RUNTIME_DIR/stand-in-gsr.live ]] || break
    sleep 0.05
  done
  ;;
esac
exit 0
EOF
# Records what the status file said, and which result was already in place, at
# the moment the recorder removes the status file.
cat >"$bin/rm" <<'EOF'
#!/usr/bin/env bash
for arg in "$@"; do
  if [[ $arg == */recorder-status.json && -f $arg ]]; then
    printf 'rm-status status=%s result=%s\n' "$(cat "$arg")" \
      "$(cat "${arg%/*}/recorder-result.json" 2>/dev/null || printf none)" >>"$EVENTS"
  fi
done
PATH=/usr/bin:/bin exec rm "$@"
EOF
cat >"$bin/slurp" <<'EOF'
#!/usr/bin/env bash
printf '%s' "${SLURP_BOX:-}"
EOF
cat >"$bin/notify-send" <<'EOF'
#!/usr/bin/env bash
printf 'notify %s\n' "$*" >>"$EVENTS"
EOF
cat >"$bin/logger" <<'EOF'
#!/usr/bin/env bash
printf 'logger %s\n' "$*" >>"$EVENTS"
EOF
cat >"$bin/ryoku-cmd-recording-saved" <<'EOF'
#!/usr/bin/env bash
printf 'saved %s\n' "$1" >>"$EVENTS"
EOF
cat >"$bin/ryoku-hub" <<'EOF'
#!/usr/bin/env bash
[[ ${1:-} == outputs ]] && printf '[{"name":"DP-1","focused":true}]\n'
EOF
chmod +x "$bin"/*

# --info as GSR prints it, with only the given encoders in video_codecs.
info_with() {
  {
    printf 'section=system_info\ndisplay_server|wayland\nsection=gpu_info\nvendor|amd\n'
    printf 'section=video_codecs\n'
    local codec
    for codec in "$@"; do printf '%s\n' "$codec"; done
    printf 'section=image_formats\njpeg\npng\nsection=capture_options\nDP-1|2560x1440\nregion\n'
  } >"$tmp/info"
}

# Runs one recorder verb; GSR_MODE, GSR_STDERR, GSR_INFO_RC, GSR_SAVE_SECONDS,
# SLURP_BOX and RYOKU_RECORD_* pass through.
run_recorder() {
  env -i PATH="$bin:/usr/bin:/bin" HOME="$tmp/home" \
    XDG_CONFIG_HOME="$tmp/config" XDG_RUNTIME_DIR="$tmp/run" \
    RYOKU_STATE_PATH="$tmp/state" RYOKU_SHELL_RECORDINGS_DIR="$tmp/out" \
    RYOKU_RECORD_NVIDIA_MODULE_VERSION="$tmp/nvidia-version" \
    RYOKU_RECORD_NVIDIA_LIB_DIRS="$tmp/lib" \
    EVENTS="$events" GSR_ARGV="$argv_file" GSR_INFO="$tmp/info" \
    GSR_MODE="${GSR_MODE:-record}" GSR_STDERR="${GSR_STDERR:-}" GSR_INFO_RC="${GSR_INFO_RC:-0}" \
    GSR_SAVE_SECONDS="${GSR_SAVE_SECONDS:-0}" SLURP_BOX="${SLURP_BOX:-}" \
    ${RYOKU_RECORD_ENCODER:+RYOKU_RECORD_ENCODER="$RYOKU_RECORD_ENCODER"} \
    ${RYOKU_RECORD_CODEC:+RYOKU_RECORD_CODEC="$RYOKU_RECORD_CODEC"} \
    bash "$recorder" "$@"
}

fresh_state() {
  rm -rf "$tmp/state" "$tmp/out" "$tmp/run" "$events" "$argv_file"
  mkdir -p "$tmp/run"
}

# Runs one capture to its end.
record() {
  fresh_state
  run_recorder start "$@" >/dev/null 2>"$tmp/stderr" || true
}

argv() { cat "$argv_file" 2>/dev/null || true; }
# Every given "flag value" pair appears in GSR's argv.
has_args() {
  local pair
  for pair in "$@"; do
    [[ " $(argv) " == *" $pair "* ]] || return 1
  done
}
notified() { grep -qF -- "$1" "$events" 2>/dev/null; }
recorded() { compgen -G "$tmp/out/*.$1" >/dev/null; }

status_now() { cat "$tmp/state/recorder-status.json" 2>/dev/null || true; }
result_now() { cat "$tmp/state/recorder-result.json" 2>/dev/null || true; }
# The status file as the recorder removed it, and the result already in place then.
last_removal() { grep '^rm-status ' "$events" 2>/dev/null | tail -n 1 || true; }
removed_status() { local line; line="$(last_removal)"; line="${line#rm-status status=}"; printf '%s' "${line%% result=*}"; }
removed_with_result() { local line; line="$(last_removal)"; printf '%s' "${line##* result=}"; }
# One field of the recorder's flat JSON, raw (strings keep their quotes).
field() { sed -n "s/.*\"$2\":\(\"[^\"]*\"\|[^,}]*\).*/\1/p" <<<"$1"; }
has_fields() {
  local json="$1" pair
  shift
  for pair in "$@"; do
    [[ $json == *"$pair"* ]] || return 1
  done
}
wait_for() {
  local _
  for _ in $(seq 200); do
    "$@" && return 0
    sleep 0.05
  done
  return 1
}
is_live() { [[ -f $tmp/run/stand-in-gsr.live && -f $tmp/state/recorder-status.json ]]; }
status_gone() { [[ ! -e $tmp/state/recorder-status.json ]]; }
# Fractional seconds as integer milliseconds.
ms() { awk -v s="$1" 'BEGIN { printf "%d", s * 1000 }'; }
# A result whose endedAt is within ten seconds of now.
fresh_result() {
  local ended now
  ended="$(ms "$(field "$1" endedAt)")"
  now="$(ms "${EPOCHREALTIME/,/.}")"
  ((ended > 0 && now - ended < 10000 && ended - now < 1000))
}

# 1. A GPU that encodes the configured codec records exactly as configured.
info_with h264 h264_software hevc av1
record --fullscreen
has_args "-w DP-1" || fail "fullscreen did not target the focused output: $(argv)"
has_args "-k h264" "-encoder gpu" || fail "configured hardware H.264 was not used: $(argv)"
recorded mp4 || fail "no clip was written"
! notified fallback || fail "a fallback was announced with the configured codec available"
# The status the shell read while it ran, and the result that ended it.
launched="$(removed_status)"
has_fields "$launched" '"target":"DP-1"' '"requestedAudioMode":"none"' '"activeAudioMode":"none"' \
  '"audioFallback":false' '"paused":false' '"pausedAt":0,' '"pausedTotal":0,' '"stopping":false' \
  || fail "the status file lacks a launch field: $launched"
[[ $(field "$launched" recorderPid) =~ ^[1-9][0-9]*$ ]] || fail "no recorder pid in the status: $launched"
[[ $(field "$launched" startedAt) =~ ^[0-9]+\.[0-9]{6}$ ]] || fail "startedAt is not fractional seconds: $launched"
clip="$(compgen -G "$tmp/out/*.mp4")"
[[ $(removed_with_result) == "$(result_now)" ]] || fail "the result was not in place before the status file went: $(last_removal)"
has_fields "$(result_now)" '"ok":true' "\"path\":\"$clip\"" '"reason":""' '"cancelled":false' \
  || fail "a saved clip did not end with an ok result: $(result_now)"
fresh_result "$(result_now)" || fail "the result's endedAt is not now: $(result_now)"
status_gone || fail "the status file outlived the capture"
for leftover in recorder.pid recorder.stop .recorder-status.*.tmp .recorder-result.*.tmp; do
  ! compgen -G "$tmp/state/$leftover" >/dev/null || fail "$leftover was left behind"
done
[[ ! -e $tmp/run/ryoku-gsr.sock ]] || fail "the IPC socket path was left behind"

# 2. No hardware encoder, libx264 present (Intel without VA-API, nouveau, VMs):
# record on the CPU instead of refusing.
info_with h264_software
record --fullscreen
has_args "-k h264" "-encoder cpu" || fail "no hardware encoder did not fall back to CPU H.264: $(argv)"
recorded mp4 || fail "the CPU fallback wrote no clip"
notified "Recording with the CPU" || fail "the CPU fallback was not announced"

# 3. Stock Fedora: no H.264 and no libx264, but AV1 in hardware.
info_with av1
record --fullscreen
has_args "-k av1" "-c mp4" || fail "H.264-less GPU did not fall back to AV1 in mp4: $(argv)"

# 4. Only VP9 in hardware: VP9 needs webm, and the clip is named for it.
info_with vp9
record --fullscreen
has_args "-k vp9" "-c webm" || fail "VP9 fallback did not switch to webm: $(argv)"
recorded webm || fail "the VP9 clip is not a .webm"

# 5. A probe that fails (GSR cannot reach the GPU) never blocks the capture:
# GSR runs as configured and speaks for itself.
info_with h264
GSR_INFO_RC=22 record --fullscreen
has_args "-k h264" "-encoder gpu" || fail "a failed probe stopped or changed the capture: $(argv)"

# 6. Nothing encodable at all refuses up front with a reason, and GSR never starts.
info_with
record --fullscreen
[[ ! -e $argv_file ]] || fail "GSR was started with no encoder available: $(argv)"
notified "No video encoder is available" || fail "the missing encoder was not explained"
has_fields "$(result_now)" '"ok":false' '"path":""' '"reason":"No video encoder is available. Install' '"cancelled":false' \
  || fail "a refusal before launch left no failed result with its reason: $(result_now)"
fresh_result "$(result_now)" || fail "the refusal's endedAt is not now: $(result_now)"
status_gone || fail "a refusal left a status file"

# 7. CPU encoding is H.264 only, whatever codec the settings name.
info_with h264_software hevc
RYOKU_RECORD_ENCODER=cpu RYOKU_RECORD_CODEC=hevc record --fullscreen
has_args "-encoder cpu" "-k h264" || fail "CPU encoding was not given H.264: $(argv)"

# 8. A region box goes straight to -w (GSR 6 deprecated -region).
info_with h264
record --region --geometry 960x600+480+300
has_args "-w 960x600+480+300" || fail "region box did not reach -w: $(argv)"
[[ " $(argv) " != *" -region "* ]] || fail "the deprecated -region option is still passed: $(argv)"
[[ $(field "$(removed_status)" target) == '"960x600+480+300"' ]] || fail "a region status did not carry the box: $(removed_status)"

# 8a. A malformed box refuses before launch, with the reason the user was shown.
record --region --geometry 960x600
[[ ! -e $argv_file ]] || fail "GSR was started on a malformed region: $(argv)"
has_fields "$(result_now)" '"ok":false' '"reason":"Invalid region: 960x600"' \
  || fail "a refused region left no failed result: $(result_now)"

# 8c. A region selection the user cancels ends as cancelled, not failed.
SLURP_BOX="" record --region
[[ ! -e $argv_file ]] || fail "GSR was started on a cancelled selection: $(argv)"
has_fields "$(result_now)" '"ok":false' '"reason":""' '"cancelled":true' \
  || fail "a cancelled selection did not end as cancelled: $(result_now)"
SLURP_BOX=1280x720+0+0 record --region
has_args "-w 1280x720+0+0" || fail "a selected region did not reach -w: $(argv)"

# 8b. An output left of or above the primary one has negative layout offsets.
info_with h264
record --region --geometry 1440x900+0+-900
has_args "-w 1440x900+0+-900" || fail "region on a negative offset was refused: $(argv)"

# 9. GSR dies after an NVIDIA update that has not been restarted into: the
# loaded module is older than the installed libraries. Say restart, and journal it.
llvmpipe="gsr error: your opengl environment is not properly setup. It's using llvmpipe (software rendering) for opengl instead of your graphics card."
printf '615.71.09\n' >"$tmp/nvidia-version"
: >"$tmp/lib/libEGL_nvidia.so.615.78.08"
ln -sf libEGL_nvidia.so.615.78.08 "$tmp/lib/libEGL_nvidia.so.0"
info_with h264
GSR_MODE=fail GSR_STDERR="$llvmpipe" record --fullscreen
notified "The NVIDIA driver was updated. Restart the computer to record again." \
  || fail "a pending NVIDIA restart was not named: $(cat "$events" 2>/dev/null)"
grep -q '^logger -t ryoku-cmd-record ' "$events" || fail "the failure was not journalled"
grep -qF "$llvmpipe" "$tmp/state/recorder.log" || fail "GSR's own error was not kept in recorder.log"

# 10. The same failure with matching NVIDIA versions is no restart: the session
# has no GPU driver at all.
printf '615.78.08\n' >"$tmp/nvidia-version"
GSR_MODE=fail GSR_STDERR="$llvmpipe" record --fullscreen
notified "no working GPU driver" || fail "software rendering was not explained: $(cat "$events" 2>/dev/null)"
! notified "Restart the computer" || fail "matching NVIDIA versions still asked for a restart"

# 11. Anything else GSR reports reaches the user in GSR's own words.
GSR_MODE=fail GSR_STDERR="gsr error: failed to find monitor DP-1" record --fullscreen
notified "failed to find monitor DP-1" || fail "GSR's error did not reach the notification"
has_fields "$(result_now)" '"ok":false' '"path":""' '"reason":"failed to find monitor DP-1"' \
  || fail "a failed capture did not carry GSR's reason in its result: $(result_now)"
[[ $(removed_with_result) == "$(result_now)" ]] || fail "the failed result was not in place before the status file went: $(last_removal)"
[[ $(field "$(removed_status)" target) == '"DP-1"' ]] || fail "a failed capture's status lacked its target: $(removed_status)"

# 12. A live capture on a named monitor: pause and resume are idempotent and
# keep the paused time out of the recorded length, stop marks the capture as
# stopping before it asks GSR to save, and a second stop is quiet.
info_with h264
fresh_state
GSR_MODE=live GSR_SAVE_SECONDS=0.6 spawn run_recorder start --monitor HDMI-A-1 --with-desktop-audio >/dev/null 2>"$tmp/stderr"
starter="${started[-1]}"
wait_for is_live || fail "a live capture never published its status: $(cat "$tmp/stderr")"
launched="$(status_now)"
has_fields "$launched" '"target":"HDMI-A-1"' '"requestedAudioMode":"system"' '"paused":false' '"stopping":false' \
  || fail "the live status lacks its target or state: $launched"

run_recorder pause >/dev/null || fail "pause failed"
first="$(status_now)"
[[ $(field "$first" paused) == true && $(field "$first" pausedAt) =~ ^[0-9]+\.[0-9]{6}$ ]] \
  || fail "pause did not record when it paused: $first"
sleep 0.3
run_recorder pause >/dev/null || fail "a second pause failed"
[[ $(field "$(status_now)" pausedAt) == "$(field "$first" pausedAt)" ]] || fail "a second pause moved the pause start: $(status_now)"
sleep 0.4
run_recorder resume >/dev/null || fail "resume failed"
resumed="$(status_now)"
run_recorder resume >/dev/null || fail "a second resume failed"
[[ $(status_now) == "$resumed" ]] || fail "a resume without a pause changed the status: $(status_now)"
[[ $(field "$resumed" paused) == false && $(field "$resumed" pausedAt) == 0 ]] || fail "resume left the capture paused: $resumed"
paused_ms="$(ms "$(field "$resumed" pausedTotal)")"
((paused_ms >= 650 && paused_ms < 2000)) || fail "the paused time is not the pause's length (~0.7 s): ${paused_ms} ms"
[[ $(grep -c '^set-paused true$' "$events") == 1 && $(grep -c '^set-paused false$' "$events") == 1 ]] \
  || fail "a repeated pause or resume reached GSR again: $(grep set-paused "$events")"
[[ $(field "$resumed" startedAt) == "$(field "$launched" startedAt)" ]] || fail "a rewrite moved the launch time"

run_recorder pause >/dev/null
spawn run_recorder stop >/dev/null 2>&1
stopper="${started[-1]}"
wait_for grep -q '^stop status=' "$events" || fail "stop never reached GSR"
[[ $(field "$(grep '^stop status=' "$events")" stopping) == true ]] || fail "stop did not mark the capture stopping before asking GSR to save"
run_recorder stop >/dev/null 2>&1 || fail "a second stop during the save failed"
run_recorder pause >/dev/null || fail "a pause during the save failed"
run_recorder resume >/dev/null || fail "a resume during the save failed"
[[ $(grep -c '^stop status=' "$events") == 1 ]] || fail "a second stop reached GSR again"
! notified "No active recording" || fail "a second stop during the save told the user nothing was recording"
[[ $(grep -c '^set-paused false$' "$events") == 1 ]] || fail "a resume during the save reached GSR"
[[ $(field "$(status_now)" stopping) == true && $(field "$(status_now)" paused) == true ]] \
  || fail "the saving status lost its stopping or paused flag: $(status_now)"
wait "$stopper" || true
wait "$starter" || true
status_gone || fail "the status file outlived a stopped capture"
[[ $(removed_with_result) == "$(result_now)" ]] || fail "the stopped capture's result was not in place before the status file went: $(last_removal)"
has_fields "$(result_now)" '"ok":true' '"cancelled":false' || fail "a stopped capture did not end with an ok result: $(result_now)"
[[ $(field "$(removed_status)" pausedTotal) == "$(field "$resumed" pausedTotal)" ]] || fail "the save changed the pause accounting"

# 13. With nothing recording, stop says so and pause changes nothing.
fresh_state
run_recorder stop >/dev/null 2>&1 || true
notified "No active recording found" || fail "a stop with nothing recording said nothing"
! run_recorder pause >/dev/null 2>&1 || fail "a pause with nothing recording claimed success"
status_gone || fail "a pause with nothing recording wrote a status file"

echo "screen-recorder: codec fallback, probe failures, region target, failure reasons, live status and results hold"
