#!/usr/bin/env bash
set -euo pipefail

repo="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
helper="$repo/system/hardware/power/ryoku-idle"
tmp="$(mktemp -d)"
idle_pid=""
cleanup() {
  [[ -z $idle_pid ]] || kill "$idle_pid" 2>/dev/null || true
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
idle_pid=""

printf 'idle policy: ok\n'
