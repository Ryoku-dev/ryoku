#!/usr/bin/env bash
set -euo pipefail

# The session guard and generation flocks must never reach the lock client's
# process tree. A watcher coprocess that outlives the client (the SDDM shim's
# dbus-monitor) inherits any open guard fd, and the inherited flock keeps
# blocking every later lock launch for the rest of the session while the
# wrapper exits 0 without a word. This test fails before the fd-close fix:
# the first lock exits clean, its orphaned watcher holds the guard, and the
# second lock never starts a client.

repo="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
W=""
W2=""
cleanup() {
  local pid
  for pid in "$W" "$W2"; do
    [[ -z $pid ]] || kill -9 "$pid" 2>/dev/null || true
  done
  rm -rf "$tmp"
}
trap cleanup EXIT

export HOME="$tmp/home"
export XDG_RUNTIME_DIR="$tmp/runtime"
export XDG_SESSION_ID=citest
export XDG_SESSION_TYPE=wayland
mkdir -p "$HOME" "$XDG_RUNTIME_DIR"

dir="$HOME/.local/share/quickshell-lockscreen"
mkdir -p "$dir/themes_link/clockwork/orbital"
cp "$repo/ryoku/lockscreen/qylock/quickshell-lockscreen/lock.sh" "$dir/lock.sh"
cp "$repo/ryoku/lockscreen/qylock/quickshell-lockscreen/proof.sh" "$dir/proof.sh"
printf 'import QtQuick\n' >"$dir/lock_shell.qml"
printf 'theme\n' >"$dir/themes_link/clockwork/orbital/Main.qml"

watchers="$tmp/watchers"
mkdir -p "$tmp/fakebin"
cat >"$tmp/fakebin/quickshell" <<'EOF'
#!/usr/bin/env bash
# Stand in for the lock client: start a coprocess that outlives us, exactly
# like the shim's session watcher, then exit with the clean-unlock status.
sleep 600 &
printf '%s\n' "$!" >>"$QYLOCK_TEST_WATCHERS"
exit 0
EOF
cat >"$tmp/fakebin/ryoku" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat >"$tmp/fakebin/killall" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "$tmp/fakebin/quickshell" "$tmp/fakebin/ryoku" "$tmp/fakebin/killall"
export PATH="$tmp/fakebin:$PATH"
export QYLOCK_TEST_WATCHERS="$watchers"

RYOKU_QYLOCK_LOCK_SCRIPT="$dir/lock.sh" bash "$repo/ryoku/lockscreen/ryoku-qylock-lock"

[[ -s $watchers ]] || {
  printf 'qylock-fd: first lock never started a client\n' >&2
  exit 1
}
W="$(head -n1 "$watchers")"

# The watcher must hold none of the lock's flock files.
leak=""
for fd in "/proc/$W"/fd/*; do
  target="$(readlink "$fd" 2>/dev/null || true)"
  [[ $target == *qylock*lock* ]] || continue
  leak="$target"
  break
done
if [[ -n $leak ]]; then
  printf 'qylock-fd: the client watcher inherited a lock guard fd: %s\n' "$leak" >&2
  kill -9 "$W" 2>/dev/null || true
  exit 1
fi

# The real consumer of that guard: a second lock while the first one's watcher
# is still alive must still start a client.
: >"$watchers"
RYOKU_QYLOCK_LOCK_SCRIPT="$dir/lock.sh" bash "$repo/ryoku/lockscreen/ryoku-qylock-lock"
[[ -s $watchers ]] || {
  printf 'qylock-fd: the second lock silently no-oped; the guard is stuck\n' >&2
  kill -9 "$W" 2>/dev/null || true
  exit 1
}

W2="$(head -n1 "$watchers")"
kill -9 "$W" "$W2" 2>/dev/null || true
printf 'qylock-fd: guard fds stay out of the client tree\n'
