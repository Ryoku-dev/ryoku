#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
POWER="$ROOT/system/hardware/bluetooth/ryoku-bluetooth-power"
RESET="$ROOT/system/hardware/bluetooth/ryoku-bluetooth-reset"
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bin" "$WORK/systemd"
LOG="$WORK/commands"
LIVE="$WORK/live"
BLOCKED="$WORK/blocked"
POLLS="$WORK/polls"
BUSY="$WORK/busy"
STATE="$WORK/state/bluetooth-power"
SHOW_FAILS="$WORK/show-fails"

cat >"$WORK/bin/rfkill" <<'EOF'
#!/usr/bin/env bash
set -eu
printf 'rfkill %s\n' "$*" >>"$RYOKU_TEST_LOG"
case ${1:-} in
  block) printf 'yes\n' >"$RYOKU_TEST_BLOCKED" ;;
  unblock)
    [[ ${RYOKU_TEST_RFKILL_UNBLOCK_STATUS:-0} == 0 ]] || exit "$RYOKU_TEST_RFKILL_UNBLOCK_STATUS"
    printf 'no\n' >"$RYOKU_TEST_BLOCKED"
    ;;
esac
EOF
cat >"$WORK/bin/bluetoothctl" <<'EOF'
#!/usr/bin/env bash
set -eu
printf 'bluetoothctl %s\n' "$*" >>"$RYOKU_TEST_LOG"
case ${1:-} in
  list)
    polls=0
    if IFS= read -r polls <"$RYOKU_TEST_POLLS"; then :; fi
    polls=$((polls + 1))
    printf '%s\n' "$polls" >"$RYOKU_TEST_POLLS"
    if ((polls >= RYOKU_TEST_APPEAR_AFTER)); then
      printf 'Controller 00:11:22:33:44:55 test [default]\n'
    fi
    ;;
  power)
    # BlueZ answers org.bluez.Error.Busy while AutoEnable is still powering a
    # freshly registered controller on, for power on and power off alike.
    busy=0
    if IFS= read -r busy <"$RYOKU_TEST_BUSY"; then :; fi
    if ((busy > 0)); then
      printf '%s\n' "$((busy - 1))" >"$RYOKU_TEST_BUSY"
      exit 1
    fi
    if [[ ${2:-} == on ]]; then
      polls=0
      if IFS= read -r polls <"$RYOKU_TEST_POLLS"; then :; fi
      ((polls >= RYOKU_TEST_APPEAR_AFTER)) || exit 1
      sleep "${RYOKU_TEST_SLOW_ON:-0}"
    fi
    printf '%s\n' "$2" >"$RYOKU_TEST_LIVE"
    ;;
  show)
    show_fails=0
    if IFS= read -r show_fails <"$RYOKU_TEST_SHOW_FAILS"; then :; fi
    if ((show_fails > 0)); then
      printf '%s\n' "$((show_fails - 1))" >"$RYOKU_TEST_SHOW_FAILS"
      exit 1
    fi
    state=off
    if IFS= read -r state <"$RYOKU_TEST_LIVE"; then :; fi
    [[ $state == on ]] && printf 'Powered: yes\n' || printf 'Powered: no\n'
    ;;
esac
EOF
cat >"$WORK/bin/systemctl" <<'EOF'
#!/usr/bin/env bash
set -eu
printf 'systemctl %s\n' "$*" >>"$RYOKU_TEST_LOG"
printf '%s\n' "${RYOKU_TEST_AUTOENABLE:-on}" >"$RYOKU_TEST_LIVE"
EOF
cat >"$WORK/bin/sv" <<'EOF'
#!/usr/bin/env bash
set -eu
printf 'sv %s\n' "$*" >>"$RYOKU_TEST_LOG"
printf 'on\n' >"$RYOKU_TEST_LIVE"
EOF
cat >"$WORK/bin/timeout" <<'EOF'
#!/usr/bin/env bash
set -eu
shift
exec "$@"
EOF
chmod +x "$WORK/bin/"*

export PATH="$WORK/bin:/usr/bin:/bin"
export RYOKU_TEST_LOG="$LOG"
export RYOKU_TEST_LIVE="$LIVE"
export RYOKU_TEST_BLOCKED="$BLOCKED"
export RYOKU_TEST_POLLS="$POLLS"
export RYOKU_TEST_BUSY="$BUSY"
export RYOKU_TEST_APPEAR_AFTER=1
export RYOKU_TEST_SHOW_FAILS="$SHOW_FAILS"
export RYOKU_TEST_RFKILL_UNBLOCK_STATUS=0
export RYOKU_BLUETOOTH_STATE_FILE="$STATE"
export RYOKU_BLUETOOTH_POWER_BIN="$POWER"

assert_eq() {
  local want=$1 got=$2 label=$3
  if [[ $got != "$want" ]]; then
    printf 'FAIL: %s: want %q, got %q\n' "$label" "$want" "$got" >&2
    exit 1
  fi
}

reset_commands() {
  : >"$LOG"
  printf '0\n' >"$POLLS"
  printf '0\n' >"$BUSY"
  printf '0\n' >"$SHOW_FAILS"
}

# No saved choice is a fresh install, which deliberately defaults to on.
printf 'off\n' >"$LIVE"
printf 'no\n' >"$BLOCKED"
reset_commands
"$POWER" restore
assert_eq on "$(<"$LIVE")" 'fresh install default'
assert_eq $'rfkill unblock bluetooth\nbluetoothctl list\nbluetoothctl power on' \
  "$(<"$LOG")" 'fresh install command sequence'

# An unusable rfkill device must not prevent BlueZ power-on when the controller
# is already available.
export RYOKU_TEST_RFKILL_UNBLOCK_STATUS=1
printf 'no\n' >"$BLOCKED"
reset_commands
"$POWER" on
assert_eq on "$(<"$LIVE")" 'power on despite failed unblock'
assert_eq $'rfkill unblock bluetooth\nbluetoothctl list\nbluetoothctl power on' \
  "$(<"$LOG")" 'failed unblock command sequence'
export RYOKU_TEST_RFKILL_UNBLOCK_STATUS=0

# Turning Bluetooth off records the choice through BlueZ without applying a
# persistent kernel soft block.
reset_commands
"$POWER" off
assert_eq off "$(<"$STATE")" 'saved off choice'
assert_eq off "$(<"$LIVE")" 'live off choice'
assert_eq $'bluetoothctl list\nbluetoothctl power off\nbluetoothctl show' "$(<"$LOG")" 'off command sequence'

# A failed status read is not confirmation that the controller is off.
reset_commands
printf '1\n' >"$SHOW_FAILS"
"$POWER" off
assert_eq $'bluetoothctl list\nbluetoothctl power off\nbluetoothctl show\nbluetoothctl list\nbluetoothctl power off\nbluetoothctl show' \
  "$(<"$LOG")" 'off retries after an unreadable state'

# Simulate systemd restarting bluetoothd with AutoEnable=true. Restore must win.
reset_commands
"$RESET"
assert_eq off "$(<"$LIVE")" 'off after systemd login reset'
assert_eq $'systemctl restart bluetooth.service\nbluetoothctl list\nbluetoothctl power off\nbluetoothctl show' \
  "$(<"$LOG")" 'systemd reset command sequence'

# Void has no systemd-rfkill boot restore, so its login reset applies the same
# per-user state immediately after restarting bluetoothd through runit.
reset_commands
RYOKU_SYSTEMD_RUNTIME="$WORK/no-systemd" "$RESET"
assert_eq off "$(<"$LIVE")" 'off after runit login reset'
assert_eq $'sv restart bluetoothd\nbluetoothctl list\nbluetoothctl power off\nbluetoothctl show' \
  "$(<"$LOG")" 'runit reset command sequence'

# The login reset races bluetoothd: the controller registers a beat after the
# restart and AutoEnable is still powering it on, so power off first finds no
# controller, then gets Busy. The saved off choice must still win.
export RYOKU_TEST_APPEAR_AFTER=2
reset_commands
printf '1\n' >"$BUSY"
"$RESET" || { printf 'FAIL: login reset failed on a late, busy controller\n' >&2; exit 1; }
assert_eq off "$(<"$LIVE")" 'off after a late, busy controller'
assert_eq $'systemctl restart bluetooth.service\nbluetoothctl list\nbluetoothctl list\nbluetoothctl power off\nbluetoothctl list\nbluetoothctl power off\nbluetoothctl show' \
  "$(<"$LOG")" 'late, busy controller command sequence'
export RYOKU_TEST_APPEAR_AFTER=1

# Power-on waits for a controller that re-enumerates after rfkill is cleared.
export RYOKU_TEST_APPEAR_AFTER=3
printf 'yes\n' >"$BLOCKED"
reset_commands
"$POWER" on
assert_eq on "$(<"$STATE")" 'saved on choice'
assert_eq on "$(<"$LIVE")" 'live on choice'
assert_eq $'rfkill unblock bluetooth\nbluetoothctl list\nbluetoothctl list\nbluetoothctl list\nbluetoothctl power on' \
  "$(<"$LOG")" 'delayed controller command sequence'

# A saved on choice from an older build recovers a machine whose Bluetooth
# switches are still soft-blocked.
printf 'yes\n' >"$BLOCKED"
printf 'off\n' >"$LIVE"
reset_commands
"$POWER" restore
assert_eq on "$(<"$LIVE")" 'restore recovers legacy soft block'
assert_eq $'rfkill unblock bluetooth\nbluetoothctl list\nbluetoothctl list\nbluetoothctl list\nbluetoothctl power on' \
  "$(<"$LOG")" 'legacy restore command sequence'

# The login reset races AutoEnable the same way when the saved choice is on:
# power on is refused with Busy until the controller accepts it.
export RYOKU_TEST_APPEAR_AFTER=1
"$POWER" on
reset_commands
printf '2\n' >"$BUSY"
RYOKU_TEST_AUTOENABLE=off "$RESET" || { printf 'FAIL: login reset failed on a busy power-on\n' >&2; exit 1; }
assert_eq on "$(<"$LIVE")" 'on after a busy power-on'
assert_eq on "$(<"$STATE")" 'saved on choice survives a busy power-on'
assert_eq $'systemctl restart bluetooth.service\nrfkill unblock bluetooth\nbluetoothctl list\nbluetoothctl power on\nbluetoothctl show\nbluetoothctl list\nbluetoothctl power on\nbluetoothctl show\nbluetoothctl list\nbluetoothctl power on' \
  "$(<"$LOG")" 'busy power-on command sequence'

# A Busy answer while AutoEnable has already powered the radio is not a failure.
reset_commands
printf '1\n' >"$BUSY"
"$RESET" || { printf 'FAIL: login reset failed on a radio AutoEnable already powered\n' >&2; exit 1; }
assert_eq $'systemctl restart bluetooth.service\nrfkill unblock bluetooth\nbluetoothctl list\nbluetoothctl power on\nbluetoothctl show' \
  "$(<"$LOG")" 'busy but powered command sequence'

# A slow controller registers well after the restart; the saved on choice
# still wins instead of failing the login unit.
export RYOKU_TEST_APPEAR_AFTER=60
reset_commands
RYOKU_TEST_AUTOENABLE=off "$RESET" || { printf 'FAIL: login reset failed on a slow controller\n' >&2; exit 1; }
assert_eq on "$(<"$LIVE")" 'on after a slow controller'

# A controller that never registers fails within the wait bound.
export RYOKU_TEST_APPEAR_AFTER=1000000
reset_commands
started=$SECONDS
if RYOKU_BLUETOOTH_WAIT=1 RYOKU_TEST_AUTOENABLE=off "$RESET"; then
  printf 'FAIL: login reset passed without a controller\n' >&2
  exit 1
fi
((SECONDS - started <= 5)) || { printf 'FAIL: missing controller wait was not bounded\n' >&2; exit 1; }
export RYOKU_TEST_APPEAR_AFTER=1

# Turning Bluetooth off while the login reset is still powering it on keeps
# it off: the newer choice wins over the older loop.
"$POWER" on
reset_commands
RYOKU_TEST_SLOW_ON=1 RYOKU_TEST_AUTOENABLE=off "$RESET" &
reset_pid=$!
sleep 0.3
"$POWER" off || { printf 'FAIL: off during a login power-on failed\n' >&2; exit 1; }
wait "$reset_pid" || { printf 'FAIL: login reset failed after a newer off\n' >&2; exit 1; }
assert_eq off "$(<"$LIVE")" 'off chosen during a login power-on stays off'
assert_eq off "$(<"$STATE")" 'off choice saved during a login power-on'

printf 'bluetooth power persistence checks passed\n'
