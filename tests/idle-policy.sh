#!/usr/bin/env bash
set -euo pipefail

repo="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
helper="$repo/system/hardware/power/ryoku-idle"
tmp="$(mktemp -d)"
idle_pid=""
replacement_pid=""
cleanup() {
  [[ -z $idle_pid ]] || kill "$idle_pid" 2>/dev/null || true
  [[ -z $replacement_pid ]] || kill "$replacement_pid" 2>/dev/null || true
  rm -rf "$tmp"
}
trap cleanup EXIT
bin="$tmp/bin"
conf="$tmp/hypridle.conf"
mkdir -p "$bin" "$tmp/proc"

cat >"$bin/ryoku-power" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$RYOKU_IDLE_TEST_POLICY"
EOF
cat >"$bin/ryoku-hw-laptop" <<'EOF'
#!/usr/bin/env bash
[[ ${1:-} == is-laptop ]]
EOF
cat >"$bin/ryoku" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$RYOKU_IDLE_WM_LOG"
if [[ $* == "wm act output.power on" ]]; then
  count=0
  [[ ! -r $RYOKU_IDLE_WM_COUNT ]] || count="$(<"$RYOKU_IDLE_WM_COUNT")"
  count=$((count + 1))
  printf '%s\n' "$count" >"$RYOKU_IDLE_WM_COUNT"
  if [[ -e ${RYOKU_IDLE_WM_FAIL_ALWAYS:-/nonexistent} ]] ||
     (( count <= ${RYOKU_IDLE_WM_FAIL_ON_COUNT:-0} )); then
    exit 1
  fi
fi
EOF
chmod +x "$bin"/*

export PATH="$bin:$PATH"
export HOME="$tmp/home"
export RYOKU_HYPRIDLE_CONF="$conf"
export RYOKU_PROC_DIR="$tmp/proc"
export RYOKU_IDLE_OUTPUT_STATE_DIR="$tmp/output-state"
export RYOKU_IDLE_WM_LOG="$tmp/wm.log"
export RYOKU_IDLE_WM_COUNT="$tmp/wm.count"
export RYOKU_OUTPUT_ON_RETRY_DELAY=0.05
export RYOKU_IDLE_TEST_POLICY='{"enabled":false,"onDesktops":false}'
"$helper" render

grep -Fxq '    lock_cmd = ryoku-shell lock' "$conf"
grep -Fxq '    inhibit_sleep = 0' "$conf"
if grep -Eq 'before_sleep_cmd|after_sleep_cmd' "$conf"; then
  printf 'hypridle still owns suspend or wake hooks\n' >&2
  exit 1
fi
if grep -Fq 'listener {' "$conf"; then
  printf 'disabled idle policy emitted listeners\n' >&2
  exit 1
fi

export RYOKU_IDLE_TEST_POLICY='{"enabled":true,"onDesktops":false,"battery":{"lockSec":30,"screenOffSec":60},"ac":{"lockSec":120,"screenOffSec":300}}'
"$helper" render
[[ $(grep -Fc 'listener {' "$conf") == 4 ]]
grep -Fq 'on-timeout = ryoku-idle on-battery && ryoku-shell lock' "$conf"
grep -Fq 'on-timeout = ryoku-idle on-ac && ryoku-idle output-off' "$conf"
grep -Fq 'on-resume = ryoku-idle output-on' "$conf"
if grep -Eq 'before_sleep_cmd|after_sleep_cmd' "$conf"; then
  printf 'idle listeners reintroduced suspend or global wake ownership\n' >&2
  exit 1
fi

# Void runs the same policy through swayidle when hypridle is unavailable. Keep
# this PATH isolated so a host-installed hypridle cannot mask the fallback.
sway_bin="$tmp/sway-bin"
sway_log="$tmp/swayidle.argv"
mkdir -p "$sway_bin"
for tool in bash dirname id jq mkdir mktemp mv; do
  ln -s "$(command -v "$tool")" "$sway_bin/$tool"
done
ln -s "$bin/ryoku-power" "$sway_bin/ryoku-power"
ln -s "$bin/ryoku-hw-laptop" "$sway_bin/ryoku-hw-laptop"
cat >"$sway_bin/swayidle" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$@" >"$RYOKU_IDLE_SWAY_LOG"
EOF
chmod +x "$sway_bin/swayidle"
export RYOKU_IDLE_SWAY_LOG="$sway_log"
export RYOKU_IDLE_TEST_POLICY='{"enabled":true,"onDesktops":false,"battery":{"dimSec":120,"lockSec":300,"screenOffSec":330,"suspendSec":900},"ac":{"dimSec":300,"lockSec":600,"screenOffSec":660,"suspendSec":1800}}'
PATH="$sway_bin" "$helper" start
cat >"$tmp/swayidle-default.expected" <<'EOF'
-w
lock
ryoku-shell lock
timeout
120
ryoku-idle on-battery && brightnessctl -s set 20%
resume
brightnessctl -r
timeout
300
ryoku-idle on-ac && brightnessctl -s set 20%
resume
brightnessctl -r
timeout
300
ryoku-idle on-battery && ryoku-shell lock
timeout
600
ryoku-idle on-ac && ryoku-shell lock
timeout
330
ryoku-idle on-battery && ryoku-idle output-off
resume
ryoku-idle output-on
timeout
660
ryoku-idle on-ac && ryoku-idle output-off
resume
ryoku-idle output-on
timeout
900
ryoku-idle on-battery && ryoku-shell suspend
timeout
1800
ryoku-idle on-ac && ryoku-shell suspend
EOF
cmp "$tmp/swayidle-default.expected" "$sway_log"

# An active policy with every stage switched off still carries swayidle's lock
# event, but emits no timeout or resume pairs.
export RYOKU_IDLE_TEST_POLICY='{"enabled":true,"onDesktops":false,"battery":{"dimSec":0,"lockSec":0,"screenOffSec":0,"suspendSec":0},"ac":{"dimSec":0,"lockSec":0,"screenOffSec":0,"suspendSec":0}}'
PATH="$sway_bin" "$helper" start
cat >"$tmp/swayidle-off.expected" <<'EOF'
-w
lock
ryoku-shell lock
EOF
cmp "$tmp/swayidle-off.expected" "$sway_log"

rm -f "$sway_log"
export RYOKU_IDLE_TEST_POLICY='{"enabled":false,"onDesktops":false}'
PATH="$sway_bin" "$helper" start
[[ ! -e $sway_log ]]

# hypridle remains preferred when both ext-idle-notify clients are installed.
cat >"$sway_bin/hypridle" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$@" >"$RYOKU_IDLE_HYPR_LOG"
EOF
chmod +x "$sway_bin/hypridle"
export RYOKU_IDLE_HYPR_LOG="$tmp/hypridle.argv"
export RYOKU_IDLE_TEST_POLICY='{"enabled":true,"onDesktops":false,"battery":{"lockSec":30},"ac":{"lockSec":120}}'
PATH="$sway_bin" "$helper" start
printf '%s\n' -c "$conf" >"$tmp/hypridle.expected"
cmp "$tmp/hypridle.expected" "$RYOKU_IDLE_HYPR_LOG"
[[ ! -e $sway_log ]]

# USB-C/USB PD supplies participate in battery detection, while an unreadable
# or absent external supply keeps the conservative AC fallback.
mkdir -p "$tmp/power/usb-c"
printf 'USB_C\n' >"$tmp/power/usb-c/type"
printf '0\n' >"$tmp/power/usb-c/online"
export RYOKU_POWER_SUPPLY_DIR="$tmp/power"
"$helper" on-battery
printf '1\n' >"$tmp/power/usb-c/online"
"$helper" on-ac
rm -rf "$tmp/power/usb-c"
"$helper" on-ac

# Ordinary idle-DPMS resume retries transient WM failures and clears its
# generation only after outputs are confirmed on.
: >"$RYOKU_IDLE_WM_LOG"
rm -f "$RYOKU_IDLE_WM_COUNT"
export RYOKU_IDLE_WM_FAIL_ON_COUNT=2
"$helper" output-off
"$helper" output-on
unset RYOKU_IDLE_WM_FAIL_ON_COUNT
[[ $(grep -Fxc 'wm act output.power on' "$RYOKU_IDLE_WM_LOG") == 3 ]]
[[ ! -e $RYOKU_IDLE_OUTPUT_STATE_DIR/generation ]]

# A newer off edge cancels an older wake generation between bounded retries;
# no stale retry is allowed to turn the newly idle display back on.
: >"$RYOKU_IDLE_WM_LOG"
rm -f "$RYOKU_IDLE_WM_COUNT"
"$helper" output-off
touch "$tmp/fail-on"
export RYOKU_IDLE_WM_FAIL_ALWAYS="$tmp/fail-on"
"$helper" output-on &
wake_pid=$!
for _ in {1..50}; do
  [[ -r $RYOKU_IDLE_WM_COUNT ]] && break
  sleep 0.01
done
"$helper" output-off
wait "$wake_pid"
unset RYOKU_IDLE_WM_FAIL_ALWAYS
last_action="$(tail -n 1 "$RYOKU_IDLE_WM_LOG")"
[[ $last_action == 'wm act output.power off' ]]
[[ -s $RYOKU_IDLE_OUTPUT_STATE_DIR/generation ]]

# An update can begin with the old generated hooks still on disk. `apply`
# replaces them immediately and sends every idle suspend through the shell's
# fail-closed transaction.
cat >"$conf" <<'EOF'
general {
    before_sleep_cmd = ryoku-shell lock
    after_sleep_cmd = ryoku wm act output.power on
}
EOF
export RYOKU_IDLE_TEST_POLICY='{"enabled":true,"onDesktops":false,"battery":{"suspendSec":600},"ac":{"suspendSec":1800}}'
"$helper" apply
[[ $(grep -Fc 'on-timeout = ryoku-idle on-battery && ryoku-shell suspend' "$conf") == 1 ]]
[[ $(grep -Fc 'on-timeout = ryoku-idle on-ac && ryoku-shell suspend' "$conf") == 1 ]]
if grep -Eq 'before_sleep_cmd|after_sleep_cmd|systemctl suspend' "$conf"; then
  printf 'apply preserved the pre-upgrade suspend contract\n' >&2
  exit 1
fi

# `apply` is an update safety gate: a running daemon with the old generated
# hooks must actually be gone before it reports success.
cp /usr/bin/sleep "$bin/hypridle"
"$bin/hypridle" 60 &
idle_pid=$!
ln -s "/proc/$idle_pid" "$tmp/proc/$idle_pid"
export RYOKU_IDLE_TEST_POLICY='{"enabled":false,"onDesktops":false}'
"$helper" apply
if kill -0 "$idle_pid" 2>/dev/null; then
  printf 'apply returned while the pre-upgrade hypridle was still alive\n' >&2
  exit 1
fi
wait "$idle_pid" 2>/dev/null || true
rm -f "$tmp/proc/$idle_pid"
idle_pid=""

# `apply` also replaces a running swayidle process and detaches the replacement
# when no service manager owns the process.
rm -f "$sway_bin/hypridle"
ln -s "$(command -v setsid)" "$sway_bin/setsid"
ln -s "$(command -v sleep)" "$sway_bin/sleep"
cp /usr/bin/sleep "$sway_bin/swayidle"
PATH="$sway_bin" "$sway_bin/swayidle" 60 &
idle_pid=$!
ln -s "/proc/$idle_pid" "$tmp/proc/$idle_pid"
rm -f "$sway_bin/swayidle"
cat >"$sway_bin/swayidle" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$$" >"$RYOKU_IDLE_SWAY_PID"
printf '%s\n' "$@" >"$RYOKU_IDLE_SWAY_LOG"
exec sleep 60
EOF
chmod +x "$sway_bin/swayidle"
export RYOKU_IDLE_SWAY_PID="$tmp/swayidle.pid"
export RYOKU_IDLE_TEST_POLICY='{"enabled":true,"onDesktops":false,"battery":{"lockSec":30},"ac":{"lockSec":120}}'
PATH="$sway_bin" "$helper" apply
if kill -0 "$idle_pid" 2>/dev/null; then
  printf 'apply returned while the old swayidle process was still alive\n' >&2
  exit 1
fi
wait "$idle_pid" 2>/dev/null || true
rm -f "$tmp/proc/$idle_pid"
idle_pid=""
for _ in {1..50}; do
  [[ -r $RYOKU_IDLE_SWAY_PID ]] && break
  sleep 0.01
done
replacement_pid="$(<"$RYOKU_IDLE_SWAY_PID")"
kill -0 "$replacement_pid"
kill "$replacement_pid"
wait "$replacement_pid" 2>/dev/null || true
replacement_pid=""
printf 'idle policy: ok\n'
