#!/usr/bin/env bash
set -euo pipefail

repo="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
helper="$repo/system/hardware/power/ryoku-clamshell"
tmp="$(mktemp -d)"
holder=""
legacy_pid=""
policy_pid=""
session_probe_pid=""
cleanup() {
  [[ -z $holder ]] || kill "$holder" 2>/dev/null || true
  [[ -z $legacy_pid ]] || kill "$legacy_pid" 2>/dev/null || true
  [[ -z $policy_pid ]] || kill "$policy_pid" 2>/dev/null || true
  [[ -z $session_probe_pid ]] || kill "$session_probe_pid" 2>/dev/null || true
  rm -rf "$tmp"
}
trap cleanup EXIT

bin="$tmp/bin"
ps="$tmp/power_supply"
drm="$tmp/drm"
lid="$tmp/lid"
run="$tmp/run"
events="$tmp/events"
state="$run/clamshell.inhibitor"
event_fifo="$run/clamshell.events"
activity_state="$run/clamshell.activity"
daemon_state="$run/clamshell.daemon"
event_state="$run/clamshell.lid"
panel_state="$run/clamshell.panels"
suspend_state="$run/clamshell.suspend"
mkdir -p "$bin" "$ps/AC" "$drm/card0-eDP-1" "$drm/card0-HDMI-A-1" "$lid" "$run"
printf 'Mains\n' >"$ps/AC/type"
printf 'connected\n' >"$drm/card0-eDP-1/status"

cat >"$bin/ryoku-hw-laptop" <<'EOF'
#!/usr/bin/env bash
[[ ${1:-} == is-laptop ]]
EOF
cat >"$bin/ryoku-shell" <<'EOF'
#!/usr/bin/env bash
if [[ ${1:-} == suspend-cancel && ${2:-} == transaction ]]; then
  [[ -z ${RYOKU_SHELL_BLOCK_DIR:-} ]] || : >"$RYOKU_SHELL_BLOCK_DIR/cancelled-$3"
  exit 0
fi
if [[ ${1:-} == suspend && ${2:-} == transaction ]]; then
  printf 'shell suspend\n' >>"$CLAMSHELL_EVENTS"
  if [[ -n ${RYOKU_SHELL_BLOCK_DIR:-} ]]; then
    : >"$RYOKU_SHELL_BLOCK_DIR/started-$3"
    while [[ ! -e $RYOKU_SHELL_BLOCK_DIR/cancelled-$3 ]]; do sleep 0.01; done
    exit 1
  fi
  [[ ${RYOKU_SHELL_FAIL:-0} != 1 ]]
  exit
fi
printf 'shell %s\n' "$*" >>"$CLAMSHELL_EVENTS"
[[ ${RYOKU_SHELL_FAIL:-0} != 1 ]]
EOF
cat >"$bin/ryoku" <<'EOF'
#!/usr/bin/env bash
[[ -n ${RYOKU_WM_CALL_TIMEOUT:-} ]] || exit 97
printf 'ryoku %s\n' "$*" >>"$CLAMSHELL_EVENTS"
if [[ $* == "wm state" ]]; then
  printf '{"outputs":[{"name":"eDP-1","disabled":%s}]}\n' \
    "${RYOKU_OUTPUT_DISABLED:-false}"
  exit 0
fi
if [[ -n ${RYOKU_FAIL_PANEL_ONCE:-} && $* == *"output.enable"* &&
      ! -e $RYOKU_FAIL_PANEL_ONCE ]]; then
  : >"$RYOKU_FAIL_PANEL_ONCE"
  exit 1
fi
EOF
cat >"$bin/ryoku-monitor" <<'EOF'
#!/usr/bin/env bash
printf 'monitor %s\n' "$*" >>"$CLAMSHELL_EVENTS"
EOF
cat >"$bin/systemd-inhibit" <<'EOF'
#!/usr/bin/env bash
if [[ ${1:-} == --list ]]; then
  printf '%s\n' "${INHIBIT_JSON:-[]}"
  exit 0
fi
exit 1
EOF
cat >"$bin/pgrep" <<'EOF'
#!/usr/bin/env bash
[[ -n ${CLAMSHELL_LEGACY_PID:-} ]] && printf '%s\n' "$CLAMSHELL_LEGACY_PID"
EOF
cat >"$bin/loginctl" <<'EOF'
#!/usr/bin/env bash
[[ ${SESSION_ACTIVE_QUERY_FAIL:-0} != 1 ]] || exit 1
[[ ${1:-} == show-session && ${3:-} == -p && ${4:-} == Active ]] || exit 1
printf '%s\n' "${SESSION_ACTIVE_VALUE:-yes}"
EOF
cat >"$bin/busctl" <<'EOF'
#!/usr/bin/env bash
state="${UPOWER_LID_STATE:-unknown}"
if [[ -r ${UPOWER_LID_STATE_FILE:-} ]]; then
  state="$(<"$UPOWER_LID_STATE_FILE")"
fi
case "$state" in
  closed) printf 'b true\n' ;;
  open) printf 'b false\n' ;;
  *) exit 1 ;;
esac
EOF
chmod +x "$bin"/*

export PATH="$bin:$PATH"
export RYOKU_POWER_SUPPLY_DIR="$ps"
export RYOKU_DRM_DIR="$drm"
export RYOKU_LID_STATE_GLOB="$lid/state"
export RYOKU_CLAMSHELL_STATE="$state"
export RYOKU_CLAMSHELL_DAEMON_STATE="$daemon_state"
export RYOKU_CLAMSHELL_LID_EVENT_STATE="$event_state"
export RYOKU_CLAMSHELL_EVENT_FIFO="$event_fifo"
export RYOKU_CLAMSHELL_ACTIVITY_STATE="$activity_state"
export RYOKU_CLAMSHELL_PANEL_STATE="$panel_state"
export RYOKU_CLAMSHELL_SUSPEND_STATE="$suspend_state"
export RYOKU_PGREP_BIN="$bin/pgrep"
export RYOKU_BIN="$bin/ryoku"
export RYOKU_BUSCTL_BIN="$bin/busctl"
export XDG_RUNTIME_DIR="$run"
export CLAMSHELL_EVENTS="$events"
export XDG_SESSION_ID=9

set_ac() { printf '%s\n' "$1" >"$ps/AC/online"; }
set_external() { printf '%s\n' "$1" >"$drm/card0-HDMI-A-1/status"; }
set_lid() { printf 'state:      %s\n' "$1" >"$lid/state"; }
reset_case() {
  : >"$events"
  rm -f "$state" "$daemon_state" "$event_state" "$panel_state" "$suspend_state"
  export INHIBIT_JSON='[]'
  unset RYOKU_SHELL_FAIL || true
  unset RYOKU_CLAMSHELL_RETRY || true
  unset RYOKU_FAIL_PANEL_ONCE || true
  unset RYOKU_CLAMSHELL_WM_TIMEOUT || true
  unset RYOKU_OUTPUT_DISABLED || true
  unset SESSION_ACTIVE_QUERY_FAIL || true
  unset SESSION_ACTIVE_VALUE || true
  unset UPOWER_LID_STATE || true
  unset RYOKU_SHELL_BLOCK_DIR || true
}
run_helper() { "$helper" "$@" >"$tmp/helper.out" 2>&1; }
expect_events() {
  local want="$1" got
  got="$(cat "$events" 2>/dev/null || true)"
  if [[ $got != "$want" ]]; then
    printf 'events mismatch\nwant:\n%s\ngot:\n%s\n' "$want" "$got" >&2
    cat "$tmp/helper.out" >&2
    exit 1
  fi
}

# Every non-docked combination uses the shell's secure suspend transaction.
for spec in '0 disconnected' '1 disconnected' '0 connected'; do
  read -r ac external <<<"$spec"
  reset_case
  set_ac "$ac"
  set_external "$external"
  set_lid closed
  run_helper policy close
  expect_events 'shell suspend'
done

# Docked clamshell skips the duplicate pre-lock only for a live inhibitor that
# login1 confirms belongs to this user and helper.
reset_case
set_ac 1
set_external connected
set_lid closed
sleep 60 &
holder=$!
printf '%s\n' "$holder" >"$state"
uid="$(id -u)"
export INHIBIT_JSON="[{\"who\":\"ryoku-clamshell\",\"uid\":$uid,\"pid\":$holder,\"what\":\"handle-lid-switch\",\"mode\":\"block\"}]"
run_helper policy close
expect_events ''
kill "$holder" 2>/dev/null || true
wait "$holder" 2>/dev/null || true
holder=""

# A stale state file must not bypass the secure transaction.
reset_case
set_ac 1
set_external connected
set_lid closed
printf '99999999\n' >"$state"
run_helper policy close
expect_events 'shell suspend'

# `stop` is the updater's safety gate. A live PID that is not present in
# login1's inhibitor table is unverified and must make the command fail rather
# than claiming the old owner was quiesced.
reset_case
sleep 60 &
holder=$!
printf '%s\n' "$holder" >"$state"
export INHIBIT_JSON='[]'
if run_helper stop; then
  printf 'stop accepted an unverified live inhibitor pid\n' >&2
  exit 1
fi
if ! kill -0 "$holder" 2>/dev/null; then
  printf 'stop killed an unverified process\n' >&2
  exit 1
fi
kill "$holder" 2>/dev/null || true
wait "$holder" 2>/dev/null || true
holder=""

# A replayed compositor event while hardware definitely reports open does
# nothing. Unknown state remains fail-safe and suspends through the shell.
reset_case
set_ac 0
set_external disconnected
set_lid open
run_helper policy close
expect_events ''
reset_case
rm -f "$lid/state"
run_helper policy close
expect_events 'shell suspend'

# A transient login1 activity lookup failure cannot consume a close event.
reset_case
set_ac 0
set_external disconnected
set_lid closed
export SESSION_ACTIVE_QUERY_FAIL=1
run_helper policy close
expect_events 'shell suspend'

# UPower seeds the state when the session starts with an already-closed lid and
# firmware exposes no readable ACPI lid file.
reset_case
rm -f "$lid/state" "$event_state"
set_ac 0
set_external disconnected
UPOWER_LID_STATE=closed bash -c '
  source "$1"
  DAEMON_SESSION_ACTIVE=1
  PREV_WANTED=1
  evaluate
' _ "$helper"
expect_events 'shell suspend'

# A new daemon generation discards any persisted compositor edge before using
# UPower as its physical startup seed.
reset_case
rm -f "$lid/state"
printf 'open\n' >"$event_state"
UPOWER_LID_STATE=closed bash -c '
  source "$1"
  reset_lid_event_generation
  DAEMON_SESSION_ACTIVE=1
  PREV_WANTED=1
  evaluate
' _ "$helper"
expect_events 'shell suspend'

# Within that daemon generation, the ordered close remains authoritative until
# a matching open even if authentication/retry takes longer than a wall clock
# grace period and UPower is unavailable or still reports open.
reset_case
rm -f "$lid/state"
printf 'closed\n' >"$event_state"
touch -d '10 seconds ago' "$event_state"
UPOWER_LID_STATE=open bash -c '
  source "$1"
  close_event_current
' _ "$helper"

# The compositor edge is authoritative when ACPI state is unavailable. UPower
# is asynchronous and a stale open property must not discard a real close.
reset_case
rm -f "$lid/state" "$event_state"
set_ac 0
set_external disconnected
UPOWER_LID_STATE=open run_helper policy close
expect_events 'shell suspend'

# If a rejected close is cancelled by the dock returning, remember the recovered
# dock state so a later loss while still closed is a fresh suspend edge.
reset_case
set_ac 1
set_external disconnected
set_lid closed
recovered_calls="$tmp/recovered-calls"
printf '0\n' >"$recovered_calls"
# shellcheck disable=SC2016
RYOKU_CLAMSHELL_RETRY=0.01 bash -c '
  source "$1"
  DAEMON_SESSION_ACTIVE=1
  calls_file=$2
  PREV_WANTED=1
  inhibitor_active() { return 0; }
  request_suspend() {
    calls=$(cat "$calls_file")
    calls=$((calls + 1))
    printf "%s\n" "$calls" >"$calls_file"
    if (( calls == 1 )); then
      printf "connected\n" >"$RYOKU_DRM_DIR/card0-HDMI-A-1/status"
      return 1
    fi
    return 0
  }
  evaluate
  [[ $PREV_WANTED == 1 ]]
  printf "disconnected\n" >"$RYOKU_DRM_DIR/card0-HDMI-A-1/status"
  evaluate
  [[ $(cat "$calls_file") == 2 ]]
' _ "$helper" "$recovered_calls" || {
  printf 'recovered dock state suppressed the next closed-lid dock loss\n' >&2
  exit 1
}

# A secure transaction rejected by a temporary global update guard is retried
# while the physical lid remains closed, then stops cleanly when it reopens.
reset_case
set_ac 0
set_external connected
set_lid closed
export RYOKU_SHELL_FAIL=1
export RYOKU_CLAMSHELL_RETRY=0.01
run_helper policy close &
policy_pid=$!
for _ in {1..100}; do
  [[ $(grep -c '^shell suspend$' "$events" 2>/dev/null || true) -ge 2 ]] && break
  sleep 0.01
done
attempts=$(grep -c '^shell suspend$' "$events" 2>/dev/null || true)
if (( attempts < 2 )); then
  printf 'lid close did not retry a rejected secure transaction\n' >&2
  exit 1
fi
set_lid open
wait "$policy_pid"
policy_pid=""

# A compositor close event remains authoritative when ACPI exposes no readable
# lid state. A rejected transaction retries until the matching open event
# cancels it, rather than becoming a one-shot suspend attempt.
reset_case
set_ac 0
set_external connected
rm -f "$lid/state"
export RYOKU_SHELL_FAIL=1
export RYOKU_CLAMSHELL_RETRY=0.01
run_helper policy close &
policy_pid=$!
for _ in {1..100}; do
  [[ $(grep -c '^shell suspend$' "$events" 2>/dev/null || true) -ge 2 ]] && break
  sleep 0.01
done
attempts=$(grep -c '^shell suspend$' "$events" 2>/dev/null || true)
if (( attempts < 2 )); then
  printf 'unreadable lid state reduced a real close event to one attempt\n' >&2
  exit 1
fi
run_helper policy open
wait "$policy_pid"
policy_pid=""
[[ $(<"$event_state") == open ]] || {
  printf 'niri policy open did not cancel the unreadable-state close retry\n' >&2
  exit 1
}
if grep -q '^ryoku wm act output.enable' "$events"; then
  printf 'niri policy open changed compositor-owned output state\n' >&2
  exit 1
fi

# A matching open cancels the daemon-side transaction, not just its waiting CLI.
reset_case
set_ac 0
set_external disconnected
set_lid closed
block_dir="$tmp/suspend-block"
mkdir -p "$block_dir"
export RYOKU_SHELL_BLOCK_DIR="$block_dir"
run_helper policy close &
policy_pid=$!
for _ in {1..100}; do
  compgen -G "$block_dir/started-*" >/dev/null && break
  sleep 0.01
done
compgen -G "$block_dir/started-*" >/dev/null || {
  printf 'lid close never entered the cancellable suspend transaction\n' >&2
  exit 1
}
# A duplicate compositor/daemon close joins the transaction already retrying;
# it must not replace or cancel that token.
run_helper policy close
[[ $(find "$block_dir" -maxdepth 1 -name 'started-*' -type f | wc -l) -eq 1 ]] || {
  printf 'duplicate lid close started a competing suspend transaction\n' >&2
  exit 1
}
if compgen -G "$block_dir/cancelled-*" >/dev/null; then
  printf 'duplicate lid close cancelled the transaction already in flight\n' >&2
  exit 1
fi
set_lid open
run_helper policy open
wait "$policy_pid"
policy_pid=""
compgen -G "$block_dir/cancelled-*" >/dev/null || {
  printf 'lid open did not reach the daemon-side suspend cancellation\n' >&2
  exit 1
}
expect_events 'shell suspend'

# A daemon retry must not pin lid ownership after its login1 session becomes
# inactive, even while an in-flight shell suspend request is still blocked.
retry_activity="$tmp/retry-activity"
retry_deactivated="$tmp/retry-deactivated"
printf 'active\n' >"$retry_activity"
# shellcheck disable=SC2016
timeout 3 bash -c '
  source "$1"
  retry_activity_path=$2
  retry_deactivated_path=$3
  DAEMON_SESSION_ACTIVE=1
  close_event_current() { return 0; }
  clamshell_wanted() { return 1; }
  inhibitor_active() { return 1; }
  session_active_state() { cat "$retry_activity_path"; }
  deactivate_session_owner() {
    DAEMON_SESSION_ACTIVE=0
    : >"$retry_deactivated_path"
  }
  request_suspend() { sleep 10; return 1; }
  (sleep 0.2; printf "inactive\n" >"$retry_activity_path") &
  changer=$!
  suspend_while_closed
  wait "$changer"
  [[ -e $retry_deactivated_path && $DAEMON_SESSION_ACTIVE == 0 ]]
' _ "$helper" "$retry_activity" "$retry_deactivated" || {
  printf 'inactive session stayed wedged in the suspend retry loop\n' >&2
  exit 1
}

# The Hyprland handoff records only a panel that was enabled before this close.
# A physically connected panel that the user had already disabled stays off
# across both lid edges.
reset_case
set_ac 1
set_external connected
set_lid closed
sleep 60 &
holder=$!
printf '%s\n' "$holder" >"$state"
export INHIBIT_JSON="[{\"who\":\"ryoku-clamshell\",\"uid\":$uid,\"pid\":$holder,\"what\":\"handle-lid-switch\",\"mode\":\"block\"}]"
export RYOKU_OUTPUT_DISABLED=false
run_helper lid close
expect_events $'ryoku wm state\nryoku wm act output.enable eDP-1 off'
[[ $(<"$panel_state") == eDP-1 ]] || {
  printf 'enabled internal panel was not recorded for lid-open restore\n' >&2
  exit 1
}
set_lid open
export SESSION_ACTIVE_VALUE=no
run_helper lid open
expect_events $'ryoku wm state\nryoku wm act output.enable eDP-1 off'
[[ -e $panel_state ]] || {
  printf 'inactive session consumed the active compositor panel receipt\n' >&2
  exit 1
}
export SESSION_ACTIVE_VALUE=yes
run_helper lid open
expect_events $'ryoku wm state\nryoku wm act output.enable eDP-1 off\nryoku wm act output.enable eDP-1 on'
kill "$holder" 2>/dev/null || true
wait "$holder" 2>/dev/null || true
holder=""

# The compositor-owned startup sync handles a session that begins with the lid
# already closed even though no switch edge fires in that compositor instance.
reset_case
set_ac 1
set_external connected
rm -f "$lid/state"
export UPOWER_LID_STATE=closed
sleep 60 &
holder=$!
printf '%s\n' "$holder" >"$state"
export INHIBIT_JSON="[{\"who\":\"ryoku-clamshell\",\"uid\":$uid,\"pid\":$holder,\"what\":\"handle-lid-switch\",\"mode\":\"block\"}]"
export RYOKU_OUTPUT_DISABLED=false
run_helper lid sync
expect_events $'ryoku wm state\nryoku wm act output.enable eDP-1 off'
export UPOWER_LID_STATE=open
run_helper lid sync
expect_events $'ryoku wm state\nryoku wm act output.enable eDP-1 off\nryoku wm act output.enable eDP-1 on'
kill "$holder" 2>/dev/null || true
wait "$holder" 2>/dev/null || true
holder=""

# A close-side provider failure is retried without waiting for another physical
# close edge, and the pre-close enabled state remains owned for reopen restore.
reset_case
set_ac 1
set_external connected
set_lid closed
sleep 60 &
holder=$!
printf '%s\n' "$holder" >"$state"
export INHIBIT_JSON="[{\"who\":\"ryoku-clamshell\",\"uid\":$uid,\"pid\":$holder,\"what\":\"handle-lid-switch\",\"mode\":\"block\"}]"
export RYOKU_OUTPUT_DISABLED=false
export RYOKU_FAIL_PANEL_ONCE="$tmp/panel-close-failed-once"
export RYOKU_CLAMSHELL_RETRY=0.01
run_helper lid close
expect_events $'ryoku wm state\nryoku wm act output.enable eDP-1 off\nryoku wm state\nryoku wm act output.enable eDP-1 off'
[[ $(<"$panel_state") == eDP-1 ]] || {
  printf 'retried close forgot the panel enabled before handoff\n' >&2
  exit 1
}
set_lid open
run_helper lid open
kill "$holder" 2>/dev/null || true
wait "$holder" 2>/dev/null || true
holder=""

reset_case
set_ac 1
set_external connected
set_lid closed
sleep 60 &
holder=$!
printf '%s\n' "$holder" >"$state"
export INHIBIT_JSON="[{\"who\":\"ryoku-clamshell\",\"uid\":$uid,\"pid\":$holder,\"what\":\"handle-lid-switch\",\"mode\":\"block\"}]"
export RYOKU_OUTPUT_DISABLED=true
run_helper lid close
expect_events 'ryoku wm state'
[[ ! -e $panel_state ]] || {
  printf 'user-disabled internal panel was claimed by the lid handoff\n' >&2
  exit 1
}
set_lid open
run_helper lid open
expect_events 'ryoku wm state'
kill "$holder" 2>/dev/null || true
wait "$holder" 2>/dev/null || true
holder=""

# Reopening restores only panels this helper recorded after disabling them; a
# replayed open must not enable a user-disabled internal output.
reset_case
set_lid open
run_helper lid open
expect_events ''
printf 'eDP-1\n' >"$panel_state"
run_helper lid open
expect_events 'ryoku wm act output.enable eDP-1 on'
[[ ! -e $panel_state ]] || {
  printf 'successful panel restore left stale ownership state\n' >&2
  exit 1
}

# A transient provider failure is retried while the matching open edge remains.
reset_case
printf 'eDP-1\n' >"$panel_state"
export RYOKU_FAIL_PANEL_ONCE="$tmp/panel-failed-once"
export RYOKU_CLAMSHELL_RETRY=0.01
run_helper lid open
expect_events $'ryoku wm act output.enable eDP-1 on\nryoku wm act output.enable eDP-1 on'
unset RYOKU_FAIL_PANEL_ONCE

# The login1 activity watcher keeps the daemon alive but transfers low-level
# lid ownership away from an inactive session and back on activation.
rm -f "$state" "$activity_state" "$event_fifo"
mkfifo -m 600 "$event_fifo"
SESSION_CONTROL_FIFO="$event_fifo" SESSION_CONTROL_STATE="$state" \
  SESSION_CONTROL_ACTIVITY="$activity_state" bash -c '
    exec 3<>"$SESSION_CONTROL_FIFO"
    while read -r event <&3; do
      case "$event" in
        "RYOKU_SESSION_ACTIVE no")
          rm -f "$SESSION_CONTROL_STATE"
          printf "inactive\n" >"$SESSION_CONTROL_ACTIVITY"
          ;;
        "RYOKU_SESSION_ACTIVE yes")
          printf "%s\n" "$$" >"$SESSION_CONTROL_STATE"
          printf "active\n" >"$SESSION_CONTROL_ACTIVITY"
          ;;
      esac
    done
  ' &
session_probe_pid=$!
TARGET_PID="$session_probe_pid" bash -c '
  source "$1"
  daemon_pid() { printf "%s\n" "$TARGET_PID"; }
  inhibitor_active() { [[ -e $INHIBIT_STATE ]]; }
  cmd_session_active no
' _ "$helper"
[[ $(<"$activity_state") == inactive && ! -e $state ]] || {
  printf 'inactive session did not release its lid owner\n' >&2
  exit 1
}
TARGET_PID="$session_probe_pid" bash -c '
  source "$1"
  daemon_pid() { printf "%s\n" "$TARGET_PID"; }
  inhibitor_active() { [[ -e $INHIBIT_STATE ]]; }
  cmd_session_active yes
' _ "$helper"
[[ $(<"$activity_state") == active && -e $state ]] || {
  printf 'reactivated session did not reacquire its lid owner\n' >&2
  exit 1
}
kill "$session_probe_pid"
wait "$session_probe_pid" 2>/dev/null || true
session_probe_pid=""
rm -f "$state" "$activity_state" "$event_fifo"

# `stop` adopts a pre-upgrade daemon that has no PID state file, using the
# exact current-user daemon command line rather than killing unrelated jobs.
cat >"$bin/ryoku-clamshell" <<'EOF'
#!/usr/bin/env bash
trap 'exit 0' TERM INT HUP
while :; do sleep 0.05; done
EOF
chmod +x "$bin/ryoku-clamshell"
"$bin/ryoku-clamshell" daemon &
legacy_pid=$!
export CLAMSHELL_LEGACY_PID="$legacy_pid"
sleep 0.05
sleep 60 &
holder=$!
printf '%s\n' "$holder" >"$state"
export INHIBIT_JSON="[{\"who\":\"ryoku-clamshell\",\"uid\":$uid,\"pid\":$holder,\"what\":\"handle-lid-switch\",\"mode\":\"block\"}]"
run_helper stop
for _ in {1..40}; do
  kill -0 "$legacy_pid" 2>/dev/null || break
  sleep 0.05
done
if kill -0 "$legacy_pid" 2>/dev/null; then
  printf 'pre-upgrade clamshell daemon was not adopted and stopped\n' >&2
  exit 1
fi
wait "$legacy_pid" 2>/dev/null || true
legacy_pid=""
if kill -0 "$holder" 2>/dev/null; then
  printf 'pre-upgrade lid inhibitor was left orphaned\n' >&2
  exit 1
fi
wait "$holder" 2>/dev/null || true
holder=""

printf 'clamshell policy: ok\n'
