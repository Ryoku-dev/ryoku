#!/usr/bin/env bash
# Behaviour of ryoku-cmd-record around GPU Screen Recorder (GSR), run against a
# stand-in gpu-screen-recorder so no GPU or display is needed. It pins what users
# hit: a machine without a hardware encoder must still record (CPU, or another
# hardware codec), a failed GSR probe must never block a capture, region boxes go
# straight to -w, and a capture that dies explains itself (an NVIDIA update that
# needs a restart, a session with no GPU driver, or GSR's own words).
set -euo pipefail

repo="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

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
printf 'video' >"$out"
EOF
cat >"$bin/gsr-cli" <<'EOF'
#!/usr/bin/env bash
exit 1
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

# Runs one capture; GSR_MODE, GSR_STDERR, GSR_INFO_RC and RYOKU_RECORD_* pass through.
record() {
  rm -rf "$tmp/state" "$tmp/out" "$events" "$argv_file"
  env -i PATH="$bin:/usr/bin:/bin" HOME="$tmp/home" \
    XDG_CONFIG_HOME="$tmp/config" XDG_RUNTIME_DIR="$tmp/run" \
    RYOKU_STATE_PATH="$tmp/state" RYOKU_SHELL_RECORDINGS_DIR="$tmp/out" \
    RYOKU_RECORD_NVIDIA_MODULE_VERSION="$tmp/nvidia-version" \
    RYOKU_RECORD_NVIDIA_LIB_DIRS="$tmp/lib" \
    EVENTS="$events" GSR_ARGV="$argv_file" GSR_INFO="$tmp/info" \
    GSR_MODE="${GSR_MODE:-record}" GSR_STDERR="${GSR_STDERR:-}" GSR_INFO_RC="${GSR_INFO_RC:-0}" \
    ${RYOKU_RECORD_ENCODER:+RYOKU_RECORD_ENCODER="$RYOKU_RECORD_ENCODER"} \
    ${RYOKU_RECORD_CODEC:+RYOKU_RECORD_CODEC="$RYOKU_RECORD_CODEC"} \
    bash "$recorder" start "$@" >/dev/null 2>"$tmp/stderr" || true
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

# 1. A GPU that encodes the configured codec records exactly as configured.
info_with h264 h264_software hevc av1
record --fullscreen
has_args "-w DP-1" || fail "fullscreen did not target the focused output: $(argv)"
has_args "-k h264" "-encoder gpu" || fail "configured hardware H.264 was not used: $(argv)"
recorded mp4 || fail "no clip was written"
! notified fallback || fail "a fallback was announced with the configured codec available"

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

# 7. CPU encoding is H.264 only, whatever codec the settings name.
info_with h264_software hevc
RYOKU_RECORD_ENCODER=cpu RYOKU_RECORD_CODEC=hevc record --fullscreen
has_args "-encoder cpu" "-k h264" || fail "CPU encoding was not given H.264: $(argv)"

# 8. A region box goes straight to -w (GSR 6 deprecated -region).
info_with h264
record --region --geometry 960x600+480+300
has_args "-w 960x600+480+300" || fail "region box did not reach -w: $(argv)"
[[ " $(argv) " != *" -region "* ]] || fail "the deprecated -region option is still passed: $(argv)"

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

echo "screen-recorder: codec fallback, probe failures, region target and failure reasons hold"
