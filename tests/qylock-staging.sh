#!/usr/bin/env bash
set -euo pipefail

repo="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
export HOME="$tmp/home"
# qylock's runtime contract is fixed under HOME; a custom XDG_DATA_HOME must
# not make the activator miss the installer's staged generation.
export XDG_DATA_HOME="$HOME/alternate-data"
share="$HOME/.local/share"
stage="$share/ryoku/qylock-next"
live_lock="$share/quickshell-lockscreen"
live_theme="$share/qylock/themes"
mkdir -p "$live_lock" "$live_theme/clockwork/orbital" "$live_theme/custom"
mkdir -p "$tmp/fakebin"
cat >"$tmp/fakebin/pgrep" <<'EOF'
#!/usr/bin/env bash
exit 1
EOF
chmod +x "$tmp/fakebin/pgrep"
export PATH="$tmp/fakebin:$PATH"
export XDG_RUNTIME_DIR="$tmp/runtime"
mkdir -p "$XDG_RUNTIME_DIR"
printf 'legacy\n' >"$live_lock/version"
printf 'legacy theme\n' >"$live_theme/clockwork/orbital/Main.qml"
printf 'before stage\n' >"$live_theme/custom/Main.qml"

# An interrupted stage is never allowed to displace the known-good client.
mkdir -p "$stage/lockscreen" "$stage/themes"
if "$repo/ryoku/lockscreen/ryoku-qylock-activate" 2>/dev/null; then
  printf 'activator accepted an incomplete staged lockscreen\n' >&2
  exit 1
fi
[[ $(<"$live_lock/version") == legacy ]] || {
  printf 'rejected stage damaged the live lockscreen\n' >&2
  exit 1
}
rm -rf "$stage"

RYOKU_QYLOCK_USER_ONLY=1 RYOKU_QYLOCK_MODE=stage \
  RYOKU_QYLOCK_BUNDLE="$repo/ryoku/lockscreen/qylock" \
  "$repo/ryoku/lockscreen/install-qylock" >/dev/null

[[ $(<"$live_lock/version") == legacy ]] || {
  printf 'staged install replaced the lock client used by the live daemon\n' >&2
  exit 1
}
[[ $(<"$live_theme/clockwork/orbital/Main.qml") == "legacy theme" ]] || {
  printf 'staged install replaced the theme used by the live lock client\n' >&2
  exit 1
}
[[ -f $stage/.complete && -x $stage/lockscreen/unlock.sh ]] || {
  printf 'staged install did not publish a complete replacement generation\n' >&2
  exit 1
}
[[ ! -e $stage/themes/custom ]] || {
  printf 'staged generation snapshotted an optional theme\n' >&2
  exit 1
}
printf 'changed after stage\n' >"$live_theme/custom/Main.qml"

cat >"$tmp/fakebin/systemctl" <<'EOF'
#!/usr/bin/env bash
[[ $* == *"is-active"*"ryoku-qylock-generation-guard.service"* &&
   -r "$XDG_RUNTIME_DIR/ryoku-qylock-generation-guard.ready" ]]
EOF
chmod +x "$tmp/fakebin/systemctl"
: >"$XDG_RUNTIME_DIR/ryoku-qylock-generation-guard.ready"

# An external client-drain guard does not confer mutation ownership. A second
# installer still waits on the dedicated publisher lock.
exec 6>"$XDG_RUNTIME_DIR/ryoku-qylock-mutation.lock"
flock 6
RYOKU_QYLOCK_USER_ONLY=1 RYOKU_QYLOCK_MODE=stage \
  RYOKU_QYLOCK_GENERATION_GUARDED=1 \
  RYOKU_QYLOCK_BUNDLE="$repo/ryoku/lockscreen/qylock" \
  "$repo/ryoku/lockscreen/install-qylock" >/dev/null &
mutation_publisher_pid=$!
sleep 0.1
kill -0 "$mutation_publisher_pid" 2>/dev/null || {
  printf 'guarded installer bypassed the qylock mutation lock\n' >&2
  exit 1
}
flock -u 6
exec 6>&-
wait "$mutation_publisher_pid"
unset mutation_publisher_pid
rm -f "$XDG_RUNTIME_DIR/ryoku-qylock-generation-guard.ready"

# If another service releases the external drain guard while a publisher is
# queued, the publisher rechecks after acquiring mutation ownership and aborts
# without touching the complete staged generation.
stage_before_lost_guard="$(<"$stage/.generation")"
: >"$XDG_RUNTIME_DIR/ryoku-qylock-generation-guard.ready"
exec 6>"$XDG_RUNTIME_DIR/ryoku-qylock-mutation.lock"
flock 6
RYOKU_QYLOCK_USER_ONLY=1 RYOKU_QYLOCK_MODE=stage \
  RYOKU_QYLOCK_GENERATION_GUARDED=1 \
  RYOKU_QYLOCK_BUNDLE="$repo/ryoku/lockscreen/qylock" \
  "$repo/ryoku/lockscreen/install-qylock" >/dev/null 2>&1 &
lost_guard_publisher_pid=$!
sleep 0.1
rm -f "$XDG_RUNTIME_DIR/ryoku-qylock-generation-guard.ready"
flock -u 6
exec 6>&-
if wait "$lost_guard_publisher_pid"; then
  printf 'installer published after losing its external generation guard\n' >&2
  exit 1
fi
[[ $(<"$stage/.generation") == "$stage_before_lost_guard" ]] || {
  printf 'lost generation guard damaged the complete staged generation\n' >&2
  exit 1
}

# A direct staged lock must pair the staged wrapper with the staged core theme;
# themes_link intentionally targets live only for optional user themes.
cat >"$tmp/fakebin/quickshell" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$QS_THEME_PATH" >"$THEME_PATH_OBSERVED"
exit 0
EOF
chmod +x "$tmp/fakebin/quickshell"
if THEME_PATH_OBSERVED="$tmp/x11-theme-path" XDG_SESSION_ID=x11-test \
    XDG_SESSION_TYPE=x11 \
    RYOKU_QYLOCK_LOCK_SCRIPT="$stage/lockscreen/lock.sh" \
    "$repo/ryoku/lockscreen/ryoku-qylock-lock" >/dev/null 2>&1; then
  printf 'qylock accepted an X11 session without secure session-lock surfaces\n' >&2
  exit 1
fi
[[ ! -e $tmp/x11-theme-path ]] || {
  printf 'rejected X11 lock launched an insecure fullscreen client\n' >&2
  exit 1
}
THEME_PATH_OBSERVED="$tmp/theme-path" XDG_SESSION_ID=test \
  XDG_SESSION_TYPE=wayland \
  RYOKU_QYLOCK_LOCK_SCRIPT="$stage/lockscreen/lock.sh" \
  "$repo/ryoku/lockscreen/ryoku-qylock-lock" >/dev/null
[[ $(realpath "$(<"$tmp/theme-path")") == \
   "$(realpath "$stage/themes/clockwork/orbital")" ]] || {
  printf 'staged lock mixed its wrapper with theme %s (want %s)\n' \
    "$(<"$tmp/theme-path")" "$stage/themes/clockwork/orbital" >&2
  exit 1
}

# The stable launcher owns a shared generation lease before selecting a
# replaceable lock script. A publisher cannot replace the stage until that
# loaded generation exits.
cat >"$tmp/held-lock" <<'EOF'
#!/usr/bin/env bash
: >"$HELD_LOCK_READY"
exec sleep 60
EOF
chmod +x "$tmp/held-lock"
HELD_LOCK_READY="$tmp/held-lock.ready" \
  RYOKU_QYLOCK_LOCK_SCRIPT="$tmp/held-lock" \
  "$repo/ryoku/lockscreen/ryoku-qylock-lock" &
held_lock_pid=$!
for _ in {1..100}; do
  [[ -e $tmp/held-lock.ready ]] && break
  sleep 0.01
done
[[ -e $tmp/held-lock.ready ]] || {
  printf 'stable launcher did not enter the retained generation\n' >&2
  exit 1
}
stage_current_qylock_for_guard() {
  RYOKU_QYLOCK_USER_ONLY=1 RYOKU_QYLOCK_MODE=stage \
    RYOKU_QYLOCK_BUNDLE="$repo/ryoku/lockscreen/qylock" \
    "$repo/ryoku/lockscreen/install-qylock" >/dev/null
}
stage_current_qylock_for_guard &
publisher_pid=$!
sleep 0.1
kill -0 "$publisher_pid" 2>/dev/null || {
  printf 'stage publication crossed a loaded generation lease\n' >&2
  exit 1
}
kill "$held_lock_pid"
wait "$held_lock_pid" 2>/dev/null || true
wait "$publisher_pid"
unset held_lock_pid publisher_pid

"$repo/ryoku/lockscreen/ryoku-qylock-activate"
[[ -f $stage/.activated && -x $stage/lockscreen/unlock.sh ]] || {
  printf 'activation did not retain the staged client generation for live locks\n' >&2
  exit 1
}
"$repo/ryoku/lockscreen/ryoku-qylock-activate"
[[ ! -e $live_lock/version ]] || {
  printf 'activation retained the incompatible lock client\n' >&2
  exit 1
}
[[ -x $live_lock/unlock.sh ]] || {
  printf 'activation did not promote the staged lock client\n' >&2
  exit 1
}
[[ -f $live_theme/clockwork/orbital/Main.qml &&
   $live_lock/themes_link -ef $live_theme ]] || {
  printf 'activation did not promote and link the matching lock themes\n' >&2
  exit 1
}
[[ $(<"$live_theme/custom/Main.qml") == "changed after stage" ]] || {
  printf 'activation discarded an optional theme changed after staging\n' >&2
  exit 1
}

stage_current_qylock() {
  RYOKU_QYLOCK_USER_ONLY=1 RYOKU_QYLOCK_MODE=stage \
    RYOKU_QYLOCK_BUNDLE="$repo/ryoku/lockscreen/qylock" \
    "$repo/ryoku/lockscreen/install-qylock" >/dev/null
}

prepare_legacy_tape_stage() {
  rm -rf "$live_theme/clockwork/tape" "$live_theme/clockwork-tape" \
    "$stage/themes/clockwork-tape"
  mkdir -p "$live_theme/clockwork/tape/font" "$stage/themes/clockwork-tape/font" \
    "$HOME/.config/qylock"
  for rel in Main.qml font/Outfit-Black.ttf metadata.desktop theme.conf preview.gif; do
    mkdir -p "$(dirname "$stage/themes/clockwork-tape/$rel")"
    printf 'legacy %s\n' "$rel" >"$live_theme/clockwork/tape/$rel"
    printf 'product %s\n' "$rel" >"$stage/themes/clockwork-tape/$rel"
  done
  (
    cd "$live_theme/clockwork/tape"
    sha256sum Main.qml font/Outfit-Black.ttf metadata.desktop theme.conf preview.gif \
      >"$stage/.legacy-tape.sha256"
  )
  printf 'clockwork/tape\n' >"$HOME/.config/qylock/theme"
}

# A staged exact legacy-theme migration commits with the lock generation and
# updates the preference only at activation time.
stage_current_qylock
prepare_legacy_tape_stage
"$repo/ryoku/lockscreen/ryoku-qylock-activate"
[[ ! -e $live_theme/clockwork/tape &&
   $(<"$live_theme/clockwork-tape/Main.qml") == 'product Main.qml' &&
   $(<"$HOME/.config/qylock/theme") == clockwork-tape ]] || {
  printf 'activation did not atomically promote the staged legacy Tape migration\n' >&2
  exit 1
}

# If the old theme changes after staging, activation must preserve that custom
# tree and leave its preference untouched rather than overwriting user work.
stage_current_qylock
prepare_legacy_tape_stage
printf 'custom after stage\n' >"$live_theme/clockwork/tape/Main.qml"
"$repo/ryoku/lockscreen/ryoku-qylock-activate" 2>/dev/null
[[ ! -e $live_theme/clockwork-tape &&
   $(<"$live_theme/clockwork/tape/Main.qml") == 'custom after stage' &&
   $(<"$HOME/.config/qylock/theme") == clockwork/tape ]] || {
  printf 'activation claimed a legacy Tape theme changed after staging\n' >&2
  exit 1
}

# A pending stage from an older release cannot override the daemon bundle that
# is actually starting. The activator replaces stale A with expected B.
stage_current_qylock
bundle_b="$tmp/qylock-b"
cp -a "$repo/ryoku/lockscreen/qylock" "$bundle_b"
printf '\n// generation B\n' >>"$bundle_b/themes/clockwork/orbital/Main.qml"
generation_b="$(
  RYOKU_QYLOCK_BUNDLE="$bundle_b" \
    "$repo/ryoku/lockscreen/install-qylock" --print-generation
)"
RYOKU_QYLOCK_INSTALLER="$repo/ryoku/lockscreen/install-qylock" \
  RYOKU_QYLOCK_BUNDLE="$bundle_b" \
  "$repo/ryoku/lockscreen/ryoku-qylock-activate"
[[ $(<"$share/ryoku/qylock-live-generation") == "$generation_b" &&
   $(<"$stage/.generation") == "$generation_b" &&
   -f $stage/.activated ]] || {
  printf 'activator paired the current daemon with a stale pending generation\n' >&2
  exit 1
}

# Unlock remains fail-closed when the retained daemon has already exited: the
# stable helper acquires a durable sleep block for the restart gap.
unlock_bin="$tmp/unlock-bin"
unlock_guard="$tmp/unlock-guard"
mkdir -p "$unlock_bin"
cat >"$unlock_bin/ryoku-shell" <<'EOF'
#!/usr/bin/env bash
exit 1
EOF
cat >"$unlock_bin/systemctl" <<'EOF'
#!/usr/bin/env bash
case "${2:-}" in
  is-active) [[ -e $UNLOCK_GUARD ]] ;;
  reset-failed) ;;
  stop) rm -f "$UNLOCK_GUARD" ;;
  *) exit 2 ;;
esac
EOF
cat >"$unlock_bin/systemd-run" <<'EOF'
#!/usr/bin/env bash
: >"$UNLOCK_GUARD"
EOF
cat >"$unlock_bin/systemd-inhibit" <<'EOF'
#!/usr/bin/env bash
[[ ${1:-} == --list && -e $UNLOCK_GUARD ]] && printf '%s\n' "$INHIBITOR_JSON"
EOF
cat >"$unlock_bin/sleep" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat >"$unlock_bin/loginctl" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "${SESSION_ACTIVE:-yes}"
EOF
cat >"$unlock_bin/busctl" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "${PREPARING_FOR_SLEEP:-b false}"
EOF
chmod +x "$unlock_bin"/*
wrong_session_inhibitor='[{"what":"sleep","who":"ryoku-qylock-unlock-other-session","mode":"block"}]'
if INHIBITOR_JSON="$wrong_session_inhibitor" UNLOCK_GUARD="$unlock_guard" \
    PATH="$unlock_bin:$PATH" \
    "$repo/ryoku/lockscreen/qylock/quickshell-lockscreen/ryoku-qylock-unlock-prepare" \
    test-session >/dev/null 2>&1; then
  printf 'unlock accepted another login1 session inhibitor\n' >&2
  exit 1
fi
right_session_inhibitor='[{"what":"sleep","who":"ryoku-qylock-unlock-test-session","mode":"block"}]'
if INHIBITOR_JSON="$right_session_inhibitor" UNLOCK_GUARD="$unlock_guard" \
    PREPARING_FOR_SLEEP="b true" PATH="$unlock_bin:$PATH" \
    "$repo/ryoku/lockscreen/qylock/quickshell-lockscreen/ryoku-qylock-unlock-prepare" \
    test-session >/dev/null 2>&1; then
  printf 'unlock proceeded while login1 was preparing to sleep\n' >&2
  exit 1
fi
if INHIBITOR_JSON="$right_session_inhibitor" UNLOCK_GUARD="$unlock_guard" \
    SESSION_ACTIVE=no PATH="$unlock_bin:$PATH" \
    "$repo/ryoku/lockscreen/qylock/quickshell-lockscreen/ryoku-qylock-unlock-prepare" \
    test-session >/dev/null 2>&1; then
  printf 'unlock proceeded for an inactive login1 session\n' >&2
  exit 1
fi
unlock_token="11111111-2222-3333-4444-555555555555"
printf '%s\n' "$unlock_token" >"$XDG_RUNTIME_DIR/qylock.test-session.locked"
printf '%s\n' "$unlock_token" >"$XDG_RUNTIME_DIR/qylock.test-session.expected"
INHIBITOR_JSON="$right_session_inhibitor" UNLOCK_GUARD="$unlock_guard" \
  QYLOCK_PROOF_TOKEN="$unlock_token" PATH="$unlock_bin:$PATH" \
  "$repo/ryoku/lockscreen/qylock/quickshell-lockscreen/ryoku-qylock-unlock-prepare" \
  test-session
[[ -e $unlock_guard ]] || {
  printf 'daemonless unlock did not acquire its durable sleep block\n' >&2
  exit 1
}
[[ ! -e $XDG_RUNTIME_DIR/qylock.test-session.locked ]] || {
  printf 'daemonless unlock left compositor-secure proof published\n' >&2
  exit 1
}

# A retained generation may still back a live staged qylock wrapper. Another
# stage must wait rather than deleting the helper paths from under that client.
mkdir -p "$tmp/fakebin"
cat >"$tmp/fakebin/pgrep" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "$tmp/fakebin/pgrep"
if PATH="$tmp/fakebin:$PATH" RYOKU_QYLOCK_USER_ONLY=1 RYOKU_QYLOCK_MODE=stage \
    RYOKU_QYLOCK_BUNDLE="$repo/ryoku/lockscreen/qylock" \
    "$repo/ryoku/lockscreen/install-qylock" >/dev/null 2>&1; then
  printf 'staging replaced a generation still used by a live lock client\n' >&2
  exit 1
fi
[[ -f $stage/.activated && -x $stage/lockscreen/proof.sh ]] || {
  printf 'rejected restage damaged the retained client generation\n' >&2
  exit 1
}

mkdir -p "$stage"
printf 'stale\n' >"$stage/version"
RYOKU_QYLOCK_USER_ONLY=1 RYOKU_QYLOCK_MODE=live \
  RYOKU_QYLOCK_BUNDLE="$repo/ryoku/lockscreen/qylock" \
  "$repo/ryoku/lockscreen/install-qylock" >/dev/null
[[ ! -e $stage ]] || {
  printf 'live install left a stale lock generation queued for next login\n' >&2
  exit 1
}

printf 'qylock staging: ok\n'
