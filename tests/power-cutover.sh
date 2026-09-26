#!/usr/bin/env bash
# Hermetic behavior test for graphical-session selection and the per-user half
# of the package power cutover.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
helper="$here/../system/hardware/power/ryoku-power-cutover"
tmp="$(mktemp -d)"
session_env_pid=""
lock_client_pid=""
stable_launcher_pid=""
cleanup() {
  [[ -z $session_env_pid ]] || kill "$session_env_pid" 2>/dev/null || true
  [[ -z $lock_client_pid ]] || kill "$lock_client_pid" 2>/dev/null || true
  [[ -z $stable_launcher_pid ]] || kill "$stable_launcher_pid" 2>/dev/null || true
  rm -rf "$tmp"
}
trap cleanup EXIT
fail() { echo "FAIL: $1" >&2; exit 1; }

mkdir -p "$tmp/session-bin" "$tmp/provider-bin" "$tmp/runtime"/{1001,1002,1003,1004,1005,1006}
python3 - \
  "$tmp/runtime/1001/bus" "$tmp/runtime/1002/bus" \
  "$tmp/runtime/1003/bus" "$tmp/runtime/1004/bus" \
  "$tmp/runtime/1005/bus" "$tmp/runtime/1006/bus" <<'PY'
import socket
import sys

for path in sys.argv[1:]:
    sock = socket.socket(socket.AF_UNIX)
    sock.bind(path)
    sock.close()
PY
cat >"$tmp/session-bin/loginctl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ ${LOGINCTL_FAIL_LIST:-0} == 1 && $1 == list-sessions ]]; then
  exit 1
fi
if [[ $1 == list-sessions ]]; then
  cat <<'SESSIONS'
1 1001 alice seat0
7 1001 alice seat0
2 1002 sddm seat0
3 1003 worker -
4 1004 locker seat0
5 1005 carol seat1
6 1006 gnome seat0
SESSIONS
  exit 0
fi
if [[ $1 == show-session && $# == 2 ]]; then
  exit 0
fi
[[ $1 == show-session && $3 == -p && $5 == --value ]]
if [[ ${LOGINCTL_FAIL_PROPERTY:-} == "$2:$4" ]]; then
  exit 1
fi
case "$2:$4" in
  1:Type|2:Type|3:Type|4:Type|6:Type|7:Type) echo wayland ;;
  5:Type) echo x11 ;;
  1:Class|7:Class) echo user ;;
  2:Class) echo greeter ;;
  3:Class) echo background ;;
  4:Class) echo lock-screen ;;
  5:Class) echo user-early ;;
  6:Class) echo user ;;
  1:Desktop|3:Desktop|4:Desktop|7:Desktop) echo Hyprland ;;
  2:Desktop) echo SDDM ;;
  5:Desktop) echo niri ;;
  6:Desktop) echo GNOME ;;
  1:User|7:User) echo 1001 ;;
  2:User) echo 1002 ;;
  3:User) echo 1003 ;;
  4:User) echo 1004 ;;
  5:User) echo 1005 ;;
  6:User) echo 1006 ;;
  1:State) echo online ;;
  5:State|7:State) echo active ;;
  1:Active) echo no ;;
  5:Active|7:Active) echo yes ;;
  *) exit 1 ;;
esac
EOF
cat >"$tmp/session-bin/getent" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
[[ $1 == passwd ]]
case "$2" in
  1001) echo 'alice:x:1001:1001::/home/alice:/bin/bash' ;;
  1002) echo 'sddm:x:1002:1002::/var/lib/sddm:/usr/bin/nologin' ;;
  1003) echo 'worker:x:1003:1003::/var/lib/worker:/usr/bin/nologin' ;;
  1004) echo 'locker:x:1004:1004::/home/locker:/bin/bash' ;;
  1005) echo 'carol:x:1005:1005::/home/carol:/bin/bash' ;;
  1006) echo 'gnome:x:1006:1006::/home/gnome:/bin/bash' ;;
  *) exit 2 ;;
esac
EOF
chmod +x "$tmp/session-bin/loginctl" "$tmp/session-bin/getent"
for provider in hyprland niri; do
  : >"$tmp/provider-bin/ryoku-wm-$provider"
  chmod +x "$tmp/provider-bin/ryoku-wm-$provider"
done
users="$(
  PATH="$tmp/session-bin:$PATH" RYOKU_CUTOVER_RUNTIME_ROOT="$tmp/runtime" \
    RYOKU_CUTOVER_PROVIDER_ROOT="$tmp/provider-bin" \
    bash -c 'source "$1"; session_users' _ "$helper"
)"
[[ $users == 'alice 1001 7' ]] \
  || fail "session selection did not choose only each user's active Ryoku Wayland login: $users"
if LOGINCTL_FAIL_LIST=1 PATH="$tmp/session-bin:$PATH" \
    RYOKU_CUTOVER_RUNTIME_ROOT="$tmp/runtime" \
    RYOKU_CUTOVER_PROVIDER_ROOT="$tmp/provider-bin" \
    bash -c 'source "$1"; session_users' _ "$helper" >/dev/null 2>&1; then
  fail "a login1 session-list failure was accepted as an empty session set"
fi
if LOGINCTL_FAIL_PROPERTY=1:Type PATH="$tmp/session-bin:$PATH" \
    RYOKU_CUTOVER_RUNTIME_ROOT="$tmp/runtime" \
    RYOKU_CUTOVER_PROVIDER_ROOT="$tmp/provider-bin" \
    bash -c 'source "$1"; session_users' _ "$helper" >/dev/null 2>&1; then
  fail "a live session property failure was accepted as an absent session"
fi

mkdir -p "$tmp/guard-bin"
cat >"$tmp/guard-bin/systemctl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  is-active) [[ -e $GUARD_STATE ]] ;;
  reset-failed) ;;
  stop) rm -f "$GUARD_STATE" ;;
  *) exit 2 ;;
esac
EOF
cat >"$tmp/guard-bin/systemd-run" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
: >"$GUARD_STATE"
EOF
cat >"$tmp/guard-bin/systemd-inhibit" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
[[ ${1:-} == --list ]]
[[ -e $GUARD_STATE ]] && printf 'sleep ryoku-package-cutover block\n'
EOF
chmod +x "$tmp/guard-bin"/*
GUARD_STATE="$tmp/guard-active" PATH="$tmp/guard-bin:$PATH" \
  bash -c 'source "$1"; start_guard' _ "$helper"
[[ -e $tmp/guard-active ]] \
  || fail "package cutover guard did not survive its acquiring process"
GUARD_STATE="$tmp/guard-active" PATH="$tmp/guard-bin:$PATH" \
  bash -c 'source "$1"; start_guard; stop_guard' _ "$helper"
[[ ! -e $tmp/guard-active ]] \
  || fail "successful package cutover did not release its durable guard"

mkdir -p "$tmp/pre-upgrade-systemd"
for lifecycle in pre_install pre_upgrade; do
  GUARD_STATE="$tmp/guard-active" \
    RYOKU_SYSTEMD_RUNTIME_DIR="$tmp/pre-upgrade-systemd" \
    PATH="$tmp/guard-bin:$PATH" \
    bash -c 'source "$1"; "$2"' _ \
      "$here/../release/packages/ryoku-desktop/ryoku-desktop.install" "$lifecycle"
  [[ -e $tmp/guard-active ]] \
    || fail "$lifecycle did not block sleep before package extraction"
  GUARD_STATE="$tmp/guard-active" PATH="$tmp/guard-bin:$PATH" systemctl stop \
    ryoku-power-cutover-guard.service
done

mkdir -p "$tmp/install-bin"
cat >"$tmp/install-bin/systemd-run" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >"$CUTOVER_SCHEDULE_LOG"
exit "${CUTOVER_SCHEDULE_FAIL:-0}"
EOF
chmod +x "$tmp/install-bin/systemd-run"
RYOKU_MANAGED_UPDATE=1 CUTOVER_SCHEDULE_LOG="$tmp/scheduled" \
  PATH="$tmp/install-bin:$PATH" sh -c '. "$1"; _schedule_power_cutover' _ \
  "$here/../release/packages/ryoku-desktop/ryoku-desktop.install"
[[ ! -e $tmp/scheduled ]] \
  || fail "managed first rollout raced stage2 with a deferred cutover"

mkdir -p "$tmp/systemd-runtime"
: >"$tmp/cutover-helper"
chmod +x "$tmp/cutover-helper"
if CUTOVER_SCHEDULE_FAIL=1 CUTOVER_SCHEDULE_LOG="$tmp/schedule-failed" \
    RYOKU_SYSTEMD_RUNTIME_DIR="$tmp/systemd-runtime" \
    RYOKU_POWER_CUTOVER_HELPER="$tmp/cutover-helper" \
    RYOKU_POWER_CUTOVER_MARKER="$tmp/no-marker" PATH="$tmp/install-bin:$PATH" \
    sh -c '. "$1"; _schedule_power_cutover' _ \
      "$here/../release/packages/ryoku-desktop/ryoku-desktop.install" >/dev/null 2>&1; then
  fail "first-rollout cutover accepted a failed migration enqueue"
fi
CUTOVER_SCHEDULE_LOG="$tmp/schedule-ok" \
  RYOKU_SYSTEMD_RUNTIME_DIR="$tmp/systemd-runtime" \
  RYOKU_POWER_CUTOVER_HELPER="$tmp/cutover-helper" \
  RYOKU_POWER_CUTOVER_MARKER="$tmp/no-marker" PATH="$tmp/install-bin:$PATH" \
  sh -c '. "$1"; _schedule_power_cutover' _ \
    "$here/../release/packages/ryoku-desktop/ryoku-desktop.install"
grep -q -- '--property=StartLimitIntervalSec=0' "$tmp/schedule-ok" \
  || fail "first-rollout migration did not disable transient-unit rate limiting"

if (( EUID == 0 )); then
  echo "power-cutover: user transaction skipped as root"
  exit 0
fi

mkdir -p "$tmp/bin" "$tmp/state" "$tmp/runtime-user"
cat >"$tmp/bin/fake" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
name="$(basename "$0")"
if [[ $name == systemctl && ${1:-} == --user && ${2:-} == is-active ]]; then
  case "${4:-}" in
    ryoku-clamshell.service|ryoku-clamshell-cutover.service) [[ -e $CUTOVER_STATE/clamshell-unit ]] ;;
    ryoku-idle.service|ryoku-idle-cutover.service) [[ -e $CUTOVER_STATE/idle-unit ]] ;;
    ryoku-power-cutover-guard.service) [[ -e $CUTOVER_STATE/user-guard ]] ;;
    ryoku-qylock-generation-guard.service) [[ -e $CUTOVER_STATE/generation-guard ]] ;;
    ryoku-qylock-launch-guard.service) [[ -e $CUTOVER_STATE/launch-guard ]] ;;
    ryoku-session-guard@*.service) [[ ${NO_WATCH_READY:-0} != 1 ]] ;;
    ryoku-session-observer@*.service) [[ -e $CUTOVER_STATE/session-observer ]] ;;
    *) exit 1 ;;
  esac
  exit
fi
if [[ $name == systemctl && ${1:-} == --user &&
      ${2:-} == reset-failed &&
      ${3:-} == ryoku-qylock-*-guard.service ]]; then
  exit 0
fi
if [[ $name == systemctl && ${1:-} == --user && ${2:-} == stop ]]; then
  case "${3:-}" in
    ryoku-qylock-generation-guard.service)
      rm -f "$CUTOVER_STATE/generation-guard" \
        "$XDG_RUNTIME_DIR/ryoku-qylock-generation-guard.ready"
      exit 0
      ;;
    ryoku-qylock-launch-guard.service)
      rm -f "$CUTOVER_STATE/launch-guard" \
        "$XDG_RUNTIME_DIR/ryoku-qylock-launch-guard.ready"
      exit 0
      ;;
  esac
fi
if [[ $name == systemctl && ${1:-} == show && ${2:-} == test.scope ]]; then
  printf '/test.scope\n'
  exit 0
fi
if [[ $name == systemctl && ${1:-} == --user && ${2:-} == cat &&
      ${NO_POWER_UNITS:-0} == 1 ]]; then
  printf '%s %s\n' "$name" "$*" >>"$CUTOVER_LOG"
  exit 1
fi
if [[ $name == systemd-inhibit && ${1:-} == --list ]]; then
  if [[ -e $CUTOVER_STATE/user-guard ]]; then
    printf '[{"what":"sleep","who":"ryoku-session-cutover","why":"test","mode":"block","uid":%s,"pid":1}]\n' "$(id -u)"
  else
    printf '[]\n'
  fi
  exit 0
fi
if [[ $name == systemd-run ]]; then
  if [[ $* == *ryoku-session-lock-* ]]; then
    sid=""
    for arg in "$@"; do
      [[ $arg == XDG_SESSION_ID=* ]] && sid=${arg#*=}
    done
    [[ $sid =~ ^[A-Za-z0-9_.-]+$ ]] || exit 87
    printf 'systemd-run session-lock %s\n' "$sid" >>"$CUTOVER_LOG"
    if [[ ${DISAPPEAR_SESSION:-} == "$sid" ]]; then
      : >"$CUTOVER_STATE/gone-$sid"
    else
      : >"$CUTOVER_STATE/proof-$sid"
    fi
    [[ $* != *'/ryoku/qylock-next/lockscreen/lock.sh'* ]] ||
      : >"$CUTOVER_STATE/staged-lock-used"
  elif [[ $* == *watch-session* ]]; then
    [[ $* == *'--property=StartLimitIntervalSec=0'* ]] || exit 89
    if [[ ${REQUIRE_USER_GUARD:-0} == 1 && ! -e $CUTOVER_STATE/user-guard ]]; then
      exit 88
    fi
    if [[ ${*: -1} == unbound ]]; then
      sid=${*: -2:1}
      printf 'systemd-run session-observer %s\n' "$sid" >>"$CUTOVER_LOG"
      : >"$CUTOVER_STATE/session-observer"
      printf '%s\n' "${*: -4:1}" >"$CUTOVER_STATE/session-observer-helper"
      [[ ${NO_WATCH_READY:-0} == 1 ]] ||
        printf '%s\n' "$sid" >"$XDG_RUNTIME_DIR/ryoku-session-observer.$sid.ready"
    else
      sid=${*: -1}
      printf 'systemd-run session-guard\n' >>"$CUTOVER_LOG"
      if [[ ${NO_WATCH_READY:-0} != 1 ]]; then
        printf '%s\n' "$sid" >"$XDG_RUNTIME_DIR/ryoku-session-guard.$sid.ready"
      fi
    fi
  elif [[ $* == *generation-guard-hold* ]]; then
    : >"$CUTOVER_STATE/generation-guard"
    : >"${*: -1}"
  elif [[ $* == *launch-guard-hold* ]]; then
    : >"$CUTOVER_STATE/launch-guard"
    : >"${*: -1}"
  elif [[ $* == *ryoku-qylock-cutover-wait* ]]; then
    printf 'systemd-run qylock-wait\n' >>"$CUTOVER_LOG"
  elif [[ $* == *ryoku-idle*" start" ]]; then
    printf 'systemd-run idle-daemon\n' >>"$CUTOVER_LOG"
    : >"$CUTOVER_STATE/idle-unit"
  elif [[ $* == *ryoku-session-cutover* ]]; then
    : >"$CUTOVER_STATE/user-guard"
  elif [[ $* == *ryoku-shell-cutover.service* ]]; then
    printf 'systemd-run shell-fallback\n' >>"$CUTOVER_LOG"
  else
    printf 'systemd-run clamshell-daemon\n' >>"$CUTOVER_LOG"
    : >"$CUTOVER_STATE/clamshell-unit"
  fi
  exit 0
fi
if [[ ${REQUIRE_USER_GUARD:-0} == 1 && ! -e $CUTOVER_STATE/user-guard ]]; then
  exit 88
fi
printf '%s %s\n' "$name" "$*" >>"$CUTOVER_LOG"
case "$name:${1:-}" in
  ryoku:wm)
    if [[ $* == "wm act config.reload" && ${WM_RELOAD_UNSUPPORTED:-0} == 1 ]]; then
      exit 4
    fi
    ;;
  ryoku-shell:quit)
    : >"$CUTOVER_STATE/stopped"
    ;;
  ryoku-shell:ping)
    [[ ! -e $CUTOVER_STATE/stopped ]]
    ;;
  ryoku-shell:sleep-ready)
    [[ -e $CUTOVER_STATE/started ]]
    ;;
  ryoku-idle:status)
    printf 'idle=active\nrunning=yes\n'
    ;;
  ryoku-clamshell:status)
    if [[ ${SESSION_INACTIVE:-0} == 1 ]]; then
      printf 'owner-monitor=ready\nsession=inactive\ninhibitor=none\n'
    elif [[ ${LEGACY_CLAMSHELL:-0} == 1 ]]; then
      printf 'clamshell=inactive\ninhibitor=none\n'
    else
      printf 'owner-monitor=ready\nsession=active\ninhibitor=held\n'
    fi
    ;;
  loginctl:list-sessions)
    if [[ ${EXTRA_SESSION:-0} == 1 ]]; then
      printf '9 %s user seat0\n8 %s user seat0\n' "$(id -u)" "$(id -u)"
    fi
    ;;
  loginctl:show-session)
    if [[ -e $CUTOVER_STATE/gone-${2:-} ]]; then
      exit 1
    fi
    if [[ ${EXTRA_SESSION:-0} == 1 ]]; then
      case "${4:-}" in
        Active)
          if [[ ${NO_ACTIVE_SESSIONS:-0} == 1 ]]; then
            printf 'no\n'
          elif [[ ${2:-} == 9 ]]; then
            printf 'yes\n'
          else
            printf 'no\n'
          fi
          ;;
        Scope) printf 'test.scope\n' ;;
        Type) printf 'wayland\n' ;;
        Class) printf 'user\n' ;;
        Desktop) printf 'testwm\n' ;;
        User) id -u ;;
        State) printf 'online\n' ;;
        *) exit 1 ;;
      esac
    else
      case "${4:-}" in
        Active) printf '%s\n' "${SESSION_ACTIVE:-yes}" ;;
        Scope) printf 'test.scope\n' ;;
        Type) printf 'wayland\n' ;;
        Desktop) printf 'testwm\n' ;;
        State) printf '%s\n' "${SESSION_STATE:-active}" ;;
        *) printf '%s\n' "${SESSION_STATE:-active}" ;;
      esac
    fi
    ;;
  systemctl:--user)
    if [[ ${2:-} == restart ]]; then
      if [[ ${3:-} == ryoku-shell.service &&
            ${SHELL_RESTART_FAIL:-0} == 1 ]]; then
        exit 1
      fi
      case "${3:-}" in
        ryoku-shell.service) : >"$CUTOVER_STATE/started" ;;
        ryoku-idle.service) : >"$CUTOVER_STATE/idle-unit" ;;
        ryoku-clamshell.service) : >"$CUTOVER_STATE/clamshell-unit" ;;
      esac
    fi
    if [[ ${2:-} == stop && ${3:-} == ryoku-power-cutover-guard.service ]]; then
      rm -f "$CUTOVER_STATE/user-guard"
    fi
    ;;
esac
EOF
cat >"$tmp/bin/pgrep" <<'EOF'
#!/usr/bin/env bash
[[ ${QYLOCK_ACTIVE:-0} == 1 ]] && exit 0
exit 1
EOF
cat >"$tmp/qylock-install" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ ${1:-} == --print-generation ]]; then
  printf '%064d\n' 0
  exit 0
fi
printf 'qylock-install %s %s\n' "${RYOKU_QYLOCK_USER_ONLY:-}" "${RYOKU_QYLOCK_MODE:-}" >>"$CUTOVER_LOG"
EOF
cat >"$tmp/qylock-lock" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat >"$tmp/qylock-proof" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
[[ ${1:-} == status && -e $CUTOVER_STATE/proof-${2:-} ]]
EOF
cat >"$tmp/bin/ryoku-wm-testwm" <<'EOF'
#!/usr/bin/env bash
if [[ ${1:-} == environment ]]; then
  printf 'TESTWM_SOCKET=session-nine\0'
fi
EOF
chmod +x "$tmp/bin/fake" "$tmp/bin/pgrep" "$tmp/qylock-install" \
  "$tmp/qylock-lock" "$tmp/qylock-proof" "$tmp/bin/ryoku-wm-testwm"
for name in ryoku ryoku-shell ryoku-idle ryoku-clamshell systemctl systemd-run systemd-inhibit loginctl dbus-monitor; do
  ln -s fake "$tmp/bin/$name"
done
mkdir -p "$tmp/cgroup/test.scope"
env -i XDG_SESSION_ID=9 XDG_SESSION_TYPE=wayland WAYLAND_DISPLAY=wayland-test \
  /usr/bin/sleep 300 &
session_env_pid=$!
printf '%s\n' "$session_env_pid" >"$tmp/cgroup/test.scope/cgroup.procs"
export RYOKU_CGROUP_ROOT="$tmp/cgroup"

: >"$tmp/calls"
WM_RELOAD_UNSUPPORTED=1 CUTOVER_LOG="$tmp/calls" CUTOVER_STATE="$tmp/state" \
  PATH="$tmp/bin:$PATH" bash -c 'source "$1"; reload_wm_config' _ "$helper"
grep -qxF 'ryoku wm act config.reload' "$tmp/calls" \
  || fail "unsupported file-watching compositor did not traverse the reload boundary"
: >"$tmp/calls"

# Guard teardown cannot cross a publisher that owns the qylock mutation lock.
# This keeps both full and launch-only admission barriers alive through moves.
guard_runtime="$tmp/guard-runtime"
mkdir -p "$guard_runtime"
CUTOVER_LOG="$tmp/calls" CUTOVER_STATE="$tmp/state" \
  XDG_RUNTIME_DIR="$guard_runtime" PATH="$tmp/bin:$PATH" \
  "$helper" launch-guard-start
exec 6>"$guard_runtime/ryoku-qylock-mutation.lock"
flock 6
CUTOVER_LOG="$tmp/calls" CUTOVER_STATE="$tmp/state" \
  XDG_RUNTIME_DIR="$guard_runtime" PATH="$tmp/bin:$PATH" \
  "$helper" launch-guard-stop &
guard_stop_pid=$!
sleep 0.1
kill -0 "$guard_stop_pid" 2>/dev/null ||
  fail "launch guard teardown crossed qylock mutation ownership"
[[ -e $tmp/state/launch-guard ]] ||
  fail "launch guard dropped while qylock mutation was active"
flock -u 6
exec 6>&-
wait "$guard_stop_pid"
[[ ! -e $tmp/state/launch-guard ]] ||
  fail "launch guard remained after serialized teardown"

: >"$tmp/state/user-guard"
CUTOVER_LOG="$tmp/calls" CUTOVER_STATE="$tmp/state" XDG_RUNTIME_DIR="$tmp/runtime-user" \
  XDG_SESSION_ID=9 XDG_STATE_HOME="$tmp/state-home" RYOKU_CUTOVER_TARGET_ROOT="$tmp/bin" \
  RYOKU_CUTOVER_PROVIDER_ROOT="$tmp/bin" RYOKU_CUTOVER_INSTALLED_HELPER="$helper" \
  RYOKU_QYLOCK_INSTALLER="$tmp/qylock-install" \
  PATH="$tmp/bin:$PATH" "$helper" user

cat >"$tmp/want" <<'EOF'
loginctl show-session 9 -p Active --value
ryoku-clamshell stop
ryoku-idle stop
systemctl --user stop ryoku-shell.service
ryoku-shell quit
qylock-install 1 stage
ryoku materialize
systemctl --user daemon-reload
loginctl show-session 9 -p State --value
loginctl show-session 9 -p Active --value
loginctl list-sessions --no-legend --no-pager
loginctl show-session 9 -p Scope --value
loginctl show-session 9 -p Type --value
loginctl show-session 9 -p Desktop --value
loginctl show-session 9 -p Type --value
systemctl --user unset-environment XDG_SESSION_ID XDG_SESSION_TYPE XDG_CURRENT_DESKTOP XDG_SESSION_DESKTOP XDG_SEAT XDG_VTNR WAYLAND_DISPLAY DISPLAY RYOKU_WM
systemctl --user set-environment XDG_SESSION_ID=9 XDG_SESSION_TYPE=wayland WAYLAND_DISPLAY=wayland-test RYOKU_WM=testwm TESTWM_SOCKET=session-nine
systemctl --user stop ryoku-session.target
systemctl --user stop ryoku-session-guard@*.service
systemctl --user reset-failed ryoku-session-guard@9.service
systemd-run session-guard
systemctl --user start ryoku-session.target
systemctl --user cat ryoku-idle.service
systemctl --user stop ryoku-idle-cutover.service
systemctl --user reset-failed ryoku-idle.service
systemctl --user restart ryoku-idle.service
ryoku-idle status
ryoku wm act config.reload
systemctl --user restart ryoku-shell.service
ryoku-shell sleep-ready
ryoku-clamshell is-laptop
systemctl --user cat ryoku-clamshell.service
systemctl --user stop ryoku-clamshell-cutover.service
systemctl --user reset-failed ryoku-clamshell.service
systemctl --user restart ryoku-clamshell.service
ryoku-clamshell status
systemctl --user restart ryogami.service
loginctl show-session 9 -p State --value
systemctl --user stop ryoku-power-cutover-guard.service
EOF

if ! diff -u "$tmp/want" "$tmp/calls"; then
  fail "session owners were not replaced in the guarded order"
fi
printf 'XDG_SESSION_ID=8\0XDG_SESSION_TYPE=wayland\0WAYLAND_DISPLAY=wayland-eight\0' \
  >"$tmp/runtime-user/ryoku-session.8.environment"
: >"$tmp/calls"
rm -f "$tmp/state/proof-8" "$tmp/state/session-observer"
EXTRA_SESSION=1 CUTOVER_LOG="$tmp/calls" CUTOVER_STATE="$tmp/state" \
  XDG_RUNTIME_DIR="$tmp/runtime-user" RYOKU_CUTOVER_PROVIDER_ROOT="$tmp/bin" \
  RYOKU_CUTOVER_INSTALLED_HELPER="$helper" \
  RYOKU_QYLOCK_LOCK_SCRIPT="$tmp/qylock-lock" \
  RYOKU_QYLOCK_LAUNCHER="$tmp/qylock-lock" \
  RYOKU_QYLOCK_PROOF_HELPER="$tmp/qylock-proof" PATH="$tmp/bin:$PATH" \
  bash -c 'source "$1"; secure_unbound_sessions 9' _ "$helper"
grep -qxF 'systemd-run session-lock 8' "$tmp/calls" \
  || fail "pre-existing inactive session was not locked before guard release"
grep -qxF 'systemd-run session-observer 8' "$tmp/calls" \
  || fail "secured inactive session was not watched for foreground activation"
guard_release_line=$(grep -n \
  '^systemctl --user stop ryoku-qylock-unlock-guard-8.service$' "$tmp/calls" |
  cut -d: -f1)
observer_line=$(grep -n '^systemd-run session-observer 8$' "$tmp/calls" |
  cut -d: -f1)
[[ -n $guard_release_line && -n $observer_line &&
   $guard_release_line -lt $observer_line ]] ||
  fail "proof-backed inactive session kept its fallback sleep guard"
[[ $(readlink -f "$(<"$tmp/state/session-observer-helper")") == "$(readlink -f "$helper")" ]] \
  || fail "checkout observer handed future session rebinds to the packaged helper"

# A session can vanish after its retained launcher starts but before proof or
# the unbound observer exists. That is a completed cleanup, not a failed cutover.
printf 'XDG_SESSION_ID=8\0XDG_SESSION_TYPE=wayland\0WAYLAND_DISPLAY=wayland-eight\0' \
  >"$tmp/runtime-user/ryoku-session.8.environment"
: >"$tmp/calls"
rm -f "$tmp/state/proof-8" "$tmp/state/gone-8"
DISAPPEAR_SESSION=8 CUTOVER_LOG="$tmp/calls" CUTOVER_STATE="$tmp/state" \
  XDG_RUNTIME_DIR="$tmp/runtime-user" \
  RYOKU_QYLOCK_LOCK_SCRIPT="$tmp/qylock-lock" \
  RYOKU_QYLOCK_LAUNCHER="$tmp/qylock-lock" \
  RYOKU_QYLOCK_PROOF_HELPER="$tmp/qylock-proof" PATH="$tmp/bin:$PATH" \
  bash -c 'source "$1"; secure_session_direct 8' _ "$helper"
grep -qxF \
  'systemctl --user stop ryoku-session-lock-8-*.service ryoku-qylock-unlock-guard-8.service' \
  "$tmp/calls" ||
  fail "vanished pre-observer session did not stop its queued lock clients"
[[ ! -e $tmp/runtime-user/ryoku-session.8.environment ]] \
  || fail "vanished pre-observer session kept stale compositor environment"
rm -f "$tmp/state/gone-8"
printf 'XDG_SESSION_ID=8\0XDG_SESSION_TYPE=wayland\0WAYLAND_DISPLAY=wayland-eight\0' \
  >"$tmp/runtime-user/ryoku-session.8.environment"

: >"$tmp/calls"
rm -f "$tmp/state/proof-8" "$tmp/state/proof-9" "$tmp/state/session-observer"
rm -f "$tmp/runtime-user"/ryoku-session-observer.*.ready
NO_ACTIVE_SESSIONS=1 EXTRA_SESSION=1 CUTOVER_LOG="$tmp/calls" \
  CUTOVER_STATE="$tmp/state" XDG_RUNTIME_DIR="$tmp/runtime-user" XDG_SESSION_ID=9 \
  RYOKU_CUTOVER_PROVIDER_ROOT="$tmp/bin" RYOKU_CUTOVER_TARGET_ROOT="$tmp/bin" \
  RYOKU_CUTOVER_INSTALLED_HELPER="$helper" RYOKU_QYLOCK_INSTALLER="$tmp/qylock-install" \
  RYOKU_QYLOCK_LOCK_SCRIPT="$tmp/qylock-lock" \
  RYOKU_QYLOCK_LAUNCHER="$tmp/qylock-lock" \
  RYOKU_QYLOCK_PROOF_HELPER="$tmp/qylock-proof" PATH="$tmp/bin:$PATH" \
  "$helper" user
for sid in 8 9; do
  grep -qxF "systemd-run session-lock $sid" "$tmp/calls" \
    || fail "inactive package cutover did not secure login1 session $sid"
  grep -qxF "systemd-run session-observer $sid" "$tmp/calls" \
    || fail "inactive package cutover did not observe login1 session $sid"
done
first_lock=$(grep -n '^systemd-run session-lock ' "$tmp/calls" | cut -d: -f1 | sort -n | sed -n '1p')
stage_line=$(grep -n '^qylock-install 1 stage$' "$tmp/calls" | cut -d: -f1)
[[ -n $stage_line && -n $first_lock && $stage_line -lt $first_lock ]] \
  || fail "inactive package cutover did not stage qylock before securing old sessions"
shell_stop=$(grep -n '^ryoku-shell quit$' "$tmp/calls" | cut -d: -f1)
[[ -n $shell_stop && -n $stage_line && $shell_stop -lt $stage_line ]] \
  || fail "inactive package cutover published qylock before quiescing the old daemon"
[[ ! -e $tmp/state/user-guard ]] \
  || fail "secure inactive-session cutover left its temporary sleep guard active"

: >"$tmp/calls"
rm -f "$tmp/runtime-user/ryoku-session.id" "$tmp/runtime-user"/ryoku-session-guard.*.ready
if NO_WATCH_READY=1 RYOKU_SESSION_READY_TRIES=2 \
    CUTOVER_LOG="$tmp/calls" CUTOVER_STATE="$tmp/state" \
    XDG_RUNTIME_DIR="$tmp/runtime-user" XDG_SESSION_ID=9 \
    RYOKU_CUTOVER_INSTALLED_HELPER="$helper" PATH="$tmp/bin:$PATH" \
    bash -c 'source "$1"; start_session_lifecycle 9' _ "$helper" >/dev/null 2>&1; then
  fail "session target started without a watcher subscription acknowledgement"
fi
if grep -qxF 'systemctl --user start ryoku-session.target' "$tmp/calls"; then
  fail "session target started before the watcher acknowledged readiness"
fi
[[ ! -e $tmp/runtime-user/ryoku-session.id ]] \
  || fail "failed watcher readiness left a live session marker"

: >"$tmp/calls"
printf '8\n' >"$tmp/runtime-user/ryoku-session.id"
CUTOVER_LOG="$tmp/calls" CUTOVER_STATE="$tmp/state" \
  XDG_RUNTIME_DIR="$tmp/runtime-user" XDG_SESSION_ID=9 \
  RYOKU_CUTOVER_INSTALLED_HELPER="$helper" PATH="$tmp/bin:$PATH" \
  bash -c 'source "$1"; start_session_lifecycle 9' _ "$helper"
mapfile -t handoff_calls <"$tmp/calls"
[[ ${handoff_calls[0]:-} == 'loginctl show-session 9 -p State --value' &&
   ${handoff_calls[1]:-} == 'loginctl show-session 9 -p Active --value' &&
   ${handoff_calls[3]:-} == 'loginctl show-session 8 -p State --value' &&
   ${handoff_calls[4]:-} == 'ryoku-shell lock session 8' ]] \
  || fail "replacement session stopped the outgoing shell before secure lock proof"


: >"$tmp/calls"
rm -f "$tmp/state/user-guard" "$tmp/state/stopped"
REQUIRE_USER_GUARD=1 CUTOVER_LOG="$tmp/calls" CUTOVER_STATE="$tmp/state" \
  XDG_RUNTIME_DIR="$tmp/runtime-user" XDG_SESSION_ID=9 \
  RYOKU_CUTOVER_TARGET_ROOT="$tmp/bin" RYOKU_CUTOVER_INSTALLED_HELPER="$helper" \
  RYOKU_QYLOCK_INSTALLER="$tmp/qylock-install" \
  PATH="$tmp/bin:$PATH" bash -c 'source "$1"; start_login_session 9' _ "$helper"
[[ ! -e $tmp/state/user-guard ]] \
  || fail "successful login startup left its temporary sleep guard active"
grep -qxF 'ryoku wm act config.reload' "$tmp/calls" \
  || fail "login startup enabled lid ownership without a successful config reload"
: >"$tmp/calls"
: >"$tmp/state/user-guard"
if SESSION_STATE=closing CUTOVER_LOG="$tmp/calls" CUTOVER_STATE="$tmp/state" \
    XDG_RUNTIME_DIR="$tmp/runtime-user" XDG_SESSION_ID=9 XDG_STATE_HOME="$tmp/state-home" \
    RYOKU_CUTOVER_TARGET_ROOT="$tmp/bin" RYOKU_CUTOVER_INSTALLED_HELPER="$helper" \
    RYOKU_QYLOCK_INSTALLER="$tmp/qylock-install" PATH="$tmp/bin:$PATH" \
    "$helper" user >/dev/null 2>&1; then
  fail "a closing login1 session released its cutover as ready"
fi
if grep -qxF 'systemctl --user stop ryoku-power-cutover-guard.service' "$tmp/calls"; then
  fail "closing-session cutover released its durable sleep guard"
fi
rm -f "$tmp/state/user-guard"

mkdir -p "$tmp/watch-bin" "$tmp/watch-runtime"
cat >"$tmp/watch-bin/dbus-monitor" <<'EOF'
#!/usr/bin/env bash
printf 'signal sender=org.freedesktop.DBus; interface=org.freedesktop.DBus; member=NameAcquired\n'
if [[ ${REMOVAL_FIRST:-0} != 1 ]]; then
  printf 'signal sender=org.freedesktop.login1; interface=org.freedesktop.DBus.Properties; member=PropertiesChanged\n'
fi
printf 'signal sender=org.freedesktop.login1; interface=org.freedesktop.login1.Manager; member=SessionRemoved\n'
EOF
cat >"$tmp/watch-bin/loginctl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ ${ACTIVE_REBIND_SID:-} && $1 == list-sessions ]]; then
  printf '%s %s user seat0\n' "$ACTIVE_REBIND_SID" "$(id -u)"
  exit 0
fi
if [[ ${ACTIVE_REBIND_SID:-} && $1 == show-session &&
      ${2:-} == "$ACTIVE_REBIND_SID" ]]; then
  case "${4:-}" in
    Type) printf 'wayland\n' ;;
    Class) printf 'user\n' ;;
    Desktop) printf 'testwm\n' ;;
    User) id -u ;;
    State) printf 'active\n' ;;
    Active) printf 'yes\n' ;;
    *) exit 1 ;;
  esac
  exit 0
fi
if [[ $* == *'-p Active --value'* ]]; then
  printf 'no\n'
fi
[[ $1 == show-session ]]
EOF
cat >"$tmp/watch-bin/ryoku-clamshell" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$SESSION_ACTIVITY_LOG"
EOF
cat >"$tmp/watch-bin/systemctl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"$SESSION_WATCH_LOG"
[[ $* != *'is-active --quiet ryoku-session-rebind@'* ]]
EOF
cat >"$tmp/watch-bin/systemd-run" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'systemd-run %s\n' "$*" >>"$SESSION_WATCH_LOG"
EOF
chmod +x "$tmp/watch-bin"/*
mkdir -p "$tmp/watch-provider"
printf '#!/bin/sh\n' >"$tmp/watch-provider/ryoku-wm-testwm"
chmod +x "$tmp/watch-provider/ryoku-wm-testwm"

# A removed login1 session cannot leave a retained qylock wrapper holding its
# generation lease after the compositor and session scope are gone.
env XDG_SESSION_ID=dead-session \
  bash -c "exec -a 'quickshell -p /tmp/qylock-next/lockscreen/lock_shell.qml' sleep 300" &
lock_client_pid=$!
env XDG_SESSION_ID=dead-session \
  bash -c "exec -a 'ryoku-qylock-lock' sleep 300" &
stable_launcher_pid=$!
sleep 0.05
SESSION_WATCH_LOG="$tmp/dead-lock-calls" XDG_RUNTIME_DIR="$tmp/watch-runtime" \
  PATH="$tmp/watch-bin:$PATH" \
  bash -c 'source "$1"; cleanup_removed_session_locks dead-session' _ "$helper"
if kill -0 "$lock_client_pid" 2>/dev/null ||
   kill -0 "$stable_launcher_pid" 2>/dev/null; then
  fail "removed login1 session left a qylock client or queued launcher running"
fi
wait "$lock_client_pid" 2>/dev/null || true
wait "$stable_launcher_pid" 2>/dev/null || true
lock_client_pid=""
stable_launcher_pid=""
grep -qF -- \
  '--user stop ryoku-session-lock-dead\x2dsession-*.service ryoku-qylock-unlock-guard-dead-session.service' \
  "$tmp/dead-lock-calls" ||
  fail "removed session did not stop its lock and unlock-guard units"

# If the activity edge scheduled a rebind just before SessionRemoved, the new
# lifecycle still retires the vanished previous session before binding itself.
mkdir -p "$tmp/missing-session-bin"
cat >"$tmp/missing-session-bin/loginctl" <<'EOF'
#!/usr/bin/env bash
exit 1
EOF
chmod +x "$tmp/missing-session-bin/loginctl"
printf 'dead-session\n' >"$tmp/watch-runtime/ryoku-session.id"
SESSION_WATCH_LOG="$tmp/rebind-cleanup-calls" XDG_RUNTIME_DIR="$tmp/watch-runtime" \
  PATH="$tmp/missing-session-bin:$tmp/watch-bin:$PATH" \
  bash -c 'source "$1"; secure_previous_session 10' _ "$helper"
grep -qF -- \
  '--user stop ryoku-session-lock-dead\x2dsession-*.service ryoku-qylock-unlock-guard-dead-session.service' \
  "$tmp/rebind-cleanup-calls" ||
  fail "rebind did not retire a previous session removed after its activity edge"
rm -f "$tmp/watch-runtime/ryoku-session.id"

# A lifecycle caller holds the transaction lock while it waits for the watcher
# subscription. The watcher must acknowledge the D-Bus match before its initial
# activity synchronization waits on that same lock.
printf '9\n' >"$tmp/watch-runtime/ryoku-session.id"
exec 8>"$tmp/watch-runtime/ryoku-power-cutover.lock"
flock 8
SESSION_WATCH_LOG="$tmp/watch-lock-calls" \
  SESSION_ACTIVITY_LOG="$tmp/watch-lock-activity" \
  XDG_RUNTIME_DIR="$tmp/watch-runtime" PATH="$tmp/watch-bin:$PATH" \
  bash -c 'source "$1"; watch_session 9' _ "$helper" 8>&- &
watch_pid=$!
watch_ready=0
for _ in {1..40}; do
  if [[ -r $tmp/watch-runtime/ryoku-session-guard.9.ready ]]; then
    watch_ready=1
    break
  fi
  sleep 0.01
done
flock -u 8
wait "$watch_pid" 2>/dev/null || true
exec 8>&-
(( watch_ready == 1 )) \
  || fail "session watcher deadlocked before acknowledging its subscription"
rm -f "$tmp/watch-runtime/ryoku-session-guard.9.ready"

printf '9\n' >"$tmp/watch-runtime/ryoku-session.id"
SESSION_WATCH_LOG="$tmp/watch-calls" SESSION_ACTIVITY_LOG="$tmp/activity-calls" \
  XDG_RUNTIME_DIR="$tmp/watch-runtime" PATH="$tmp/watch-bin:$PATH" \
  bash -c 'source "$1"; watch_session 9' _ "$helper"
cat >"$tmp/watch-want" <<'EOF'
--user stop ryoku-session.target
--user stop ryoku-session-lock-9-*.service ryoku-qylock-unlock-guard-9.service
--user is-active --quiet ryoku-power-cutover-guard.service
--user stop ryoku-power-cutover-guard.service
EOF
if ! diff -u "$tmp/watch-want" "$tmp/watch-calls"; then
  fail "removed login1 session did not stop its target and failed cutover guard"
fi
[[ ! -e $tmp/watch-runtime/ryoku-session.id ]] \
  || fail "removed login1 session left its lifecycle marker behind"
[[ $(grep -c '^session-active no$' "$tmp/activity-calls") == 2 ]] \
  || fail "inactive login1 transition did not release clamshell ownership"

: >"$tmp/watch-calls"
: >"$tmp/activity-calls"
printf '10\n' >"$tmp/watch-runtime/ryoku-session.id"
SESSION_WATCH_LOG="$tmp/watch-calls" SESSION_ACTIVITY_LOG="$tmp/activity-calls" \
  XDG_RUNTIME_DIR="$tmp/watch-runtime" PATH="$tmp/watch-bin:$PATH" \
  bash -c 'source "$1"; watch_session 9' _ "$helper"
cat >"$tmp/stale-watch-want" <<'EOF'
--user stop ryoku-session-lock-9-*.service ryoku-qylock-unlock-guard-9.service
EOF
if ! diff -u "$tmp/stale-watch-want" "$tmp/watch-calls"; then
  fail "stale session watcher did not retire that session's lock clients"
fi
[[ ! -s $tmp/activity-calls ]] \
  || fail "a stale session watcher changed the replacement login's lid owner"

: >"$tmp/watch-calls"
: >"$tmp/activity-calls"
printf '9\n' >"$tmp/watch-runtime/ryoku-session.id"
ACTIVE_REBIND_SID=10 SESSION_WATCH_LOG="$tmp/watch-calls" \
  SESSION_ACTIVITY_LOG="$tmp/activity-calls" XDG_RUNTIME_DIR="$tmp/watch-runtime" \
  RYOKU_CUTOVER_PROVIDER_ROOT="$tmp/watch-provider" \
  RYOKU_CUTOVER_INSTALLED_HELPER="$tmp/not-installed" \
  PATH="$tmp/watch-bin:$PATH" bash -c 'source "$1"; watch_session 9' _ "$helper"
if ! grep -qF -- '--unit=ryoku-session-rebind@10.service' "$tmp/watch-calls"; then
  cat "$tmp/watch-calls" >&2
  fail "same-UID active-session handoff did not schedule the replacement login"
fi
grep -qF -- '--property=StartLimitIntervalSec=0' "$tmp/watch-calls" \
  || fail "replacement login retry was subject to systemd start limiting"
grep -qF -- 'ryoku-power-cutover session-start 10' "$tmp/watch-calls" \
  || fail "replacement login did not bind the selected login1 session"
[[ ! -s $tmp/activity-calls ]] \
  || fail "outgoing watcher changed lid ownership after scheduling its replacement"

: >"$tmp/calls"
rm -f "$tmp/state/clamshell-unit"
LEGACY_CLAMSHELL=1 NO_POWER_UNITS=1 CUTOVER_LOG="$tmp/calls" CUTOVER_STATE="$tmp/state" \
  XDG_SESSION_ID=9 PATH="$tmp/bin:$PATH" \
  bash -c 'source "$1"; start_target_clamshell ping' _ "$helper"
grep -qxF 'ryoku-clamshell status' "$tmp/calls" \
  || fail "legacy target daemon was not checked under its conditional inhibitor policy"

: >"$tmp/calls"
SESSION_ACTIVE=no LEGACY_CLAMSHELL=1 NO_POWER_UNITS=1 \
  CUTOVER_LOG="$tmp/calls" CUTOVER_STATE="$tmp/state" XDG_SESSION_ID=9 \
  PATH="$tmp/bin:$PATH" bash -c 'source "$1"; start_target_clamshell ping' _ "$helper"
if grep -q '^systemd-run ' "$tmp/calls"; then
  fail "legacy lid owner started while its login1 session was inactive"
fi

: >"$tmp/calls"
rm -f "$tmp/state/clamshell-unit"
SESSION_INACTIVE=1 CUTOVER_LOG="$tmp/calls" CUTOVER_STATE="$tmp/state" \
  PATH="$tmp/bin:$PATH" bash -c 'source "$1"; start_target_clamshell sleep-ready' _ "$helper"
grep -qxF 'ryoku-clamshell status' "$tmp/calls" \
  || fail "inactive session was not accepted after releasing its lid inhibitor"

: >"$tmp/calls"
RYOKU_CUTOVER_TARGET_ROOT="$tmp/missing-target" \
  PATH="$tmp/bin:$PATH" bash -c 'source "$1"; start_target_session' _ "$helper"
[[ ! -s $tmp/calls ]] \
  || fail "base removal restarted a partial desktop from a surviving shell dependency"

: >"$tmp/calls"
if SHELL_RESTART_FAIL=1 CUTOVER_LOG="$tmp/calls" CUTOVER_STATE="$tmp/state" \
    PATH="$tmp/bin:$PATH" bash -c 'source "$1"; restart_shell_owner' _ "$helper"; then
  fail "failed guarded shell unit was treated as a successful restart"
fi
if grep -qxF 'systemd-run shell-fallback' "$tmp/calls"; then
  fail "failed guarded shell unit was bypassed by a direct daemon fallback"
fi

: >"$tmp/calls"
mkdir -p "$tmp/empty-home"
HOME="$tmp/empty-home" SHELL_RESTART_FAIL=1 NO_POWER_UNITS=1 \
  CUTOVER_LOG="$tmp/calls" CUTOVER_STATE="$tmp/state" PATH="$tmp/bin:$PATH" \
  bash -c 'source "$1"; restart_shell_owner' _ "$helper"
grep -qxF 'systemd-run shell-fallback' "$tmp/calls" \
  || fail "unitless legacy session did not receive the guarded shell fallback"

echo "power-cutover: ok"
