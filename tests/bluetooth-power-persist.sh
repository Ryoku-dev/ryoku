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
STATE="$WORK/state/bluetooth-power"

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
    if [[ ${2:-} == on ]]; then
      polls=0
      if IFS= read -r polls <"$RYOKU_TEST_POLLS"; then :; fi
      ((polls >= RYOKU_TEST_APPEAR_AFTER)) || exit 1
    fi
    printf '%s\n' "$2" >"$RYOKU_TEST_LIVE"
    ;;
  show)
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
printf 'on\n' >"$RYOKU_TEST_LIVE"
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
export RYOKU_TEST_APPEAR_AFTER=1
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
assert_eq 'bluetoothctl power off' "$(<"$LOG")" 'off command sequence'

# Simulate systemd restarting bluetoothd with AutoEnable=true. Restore must win.
reset_commands
"$RESET"
assert_eq off "$(<"$LIVE")" 'off after systemd login reset'
assert_eq $'systemctl restart bluetooth.service\nbluetoothctl power off' \
  "$(<"$LOG")" 'systemd reset command sequence'

# Void has no systemd-rfkill boot restore, so its login reset applies the same
# per-user state immediately after restarting bluetoothd through runit.
reset_commands
RYOKU_SYSTEMD_RUNTIME="$WORK/no-systemd" "$RESET"
assert_eq off "$(<"$LIVE")" 'off after runit login reset'
assert_eq $'sv restart bluetoothd\nbluetoothctl power off' \
  "$(<"$LOG")" 'runit reset command sequence'

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

printf 'bluetooth power persistence checks passed\n'
