#!/usr/bin/env bash

# Current directory
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# The stable ryoku-qylock-lock launcher holds the shared generation lock before
# this replaceable script is selected. This wrapper then owns only its
# session-scoped launch guard for the full compositor-lock lifetime.
runtime_dir="${XDG_RUNTIME_DIR:-/tmp}"
# Session lock acquisition is process-global, not just daemon-local. Take the
# guard before touching the secure marker or any locker process so simultaneous
# lid, idle, and manual requests cannot race two WlSessionLock clients.
session_id="${XDG_SESSION_ID:-}"
if [[ ! $session_id =~ ^[A-Za-z0-9_.-]+$ ]]; then
    echo "qylock: XDG_SESSION_ID is required for session-scoped lock proof" >&2
    exit 1
fi
guard_file="$runtime_dir/qylock-${UID:-$(id -u)}.$session_id.lock"
if ! exec 9>"$guard_file"; then
    echo "qylock: cannot open launch guard $guard_file" >&2
    exit 1
fi
flock -n 9 || exit 0
proof_helper="$DIR/proof.sh"
generation=""
for generation_file in "$DIR/../.generation" "$DIR/../ryoku/qylock-live-generation"; do
    if [[ -r $generation_file ]]; then
        generation="$(<"$generation_file")"
        break
    fi
done
if [[ $generation =~ ^[a-f0-9]{64}$ ]]; then
    export QYLOCK_GENERATION="$generation"
else
    unset QYLOCK_GENERATION
fi
proof_token=""
cleanup_proof() {
    if [[ -n $proof_token ]]; then
        "$proof_helper" retire "$proof_token" "$session_id" >/dev/null 2>&1 || true
    fi
}
trap cleanup_proof EXIT

# Set library paths
export QML2_IMPORT_PATH="$DIR/imports:${QML2_IMPORT_PATH:-}"
export QML_XHR_ALLOW_FILE_READ=1

# The lock is spawned by the shell daemon, a systemd user service whose env is
# whatever was imported at login; a daemon (re)started outside that import has
# no XCURSOR_*, Qt falls back to the "default" theme, and where that resolves to
# nothing the lock surface sets a null cursor: no visible pointer. Default to
# the shipped Bibata set (ryoku-cursors) at the size env.lua uses. A session
# that exported its own theme keeps it.
export XCURSOR_THEME="${XCURSOR_THEME:-Bibata-Modern-Ice}"
export XCURSOR_SIZE="${XCURSOR_SIZE:-24}"
export HYPRCURSOR_THEME="${HYPRCURSOR_THEME:-$XCURSOR_THEME}"
export HYPRCURSOR_SIZE="${HYPRCURSOR_SIZE:-$XCURSOR_SIZE}"

# Get session type: the env when set, else this session's logind record.
# XDG_SESSION_ID pins the right session; scraping `loginctl | grep user`
# picked the first of several (re-login, nested session) and could misread.
if [ -z "${XDG_SESSION_TYPE:-}" ]; then
    sid="${XDG_SESSION_ID:-$(loginctl list-sessions --no-legend 2>/dev/null | awk -v u="$(id -un)" '$3 == u {print $1; exit}')}"
    XDG_SESSION_TYPE="$(loginctl show-session "$sid" -p Type --value 2>/dev/null || true)"
    [ -n "$XDG_SESSION_TYPE" ] || XDG_SESSION_TYPE=wayland
fi
export XDG_SESSION_TYPE
if [[ $XDG_SESSION_TYPE != wayland ]]; then
    echo "qylock: secure in-session locking requires a Wayland session" >&2
    exit 1
fi


# User theme preference
# Get user theme
CONFIG_FILE="$HOME/.config/qylock/theme"
if [ -n "${1:-}" ]; then
    export QS_THEME="$1"
elif [ -f "$CONFIG_FILE" ]; then
    QS_THEME=$(cat "$CONFIG_FILE")
    export QS_THEME
else
    export QS_THEME="clockwork/orbital"
fi

# A staged generation carries its core theme beside the wrapper. Prefer that
# complete generation; optional user themes deliberately fall back through the
# live themes_link because they are owned by RyoStore, not this installer.
staged_theme="$DIR/../themes/$QS_THEME"
linked_theme="$DIR/themes_link/$QS_THEME"
if [ -f "$staged_theme/Main.qml" ]; then
    export QS_THEME_PATH="$staged_theme"
else
    export QS_THEME_PATH="$linked_theme"
fi

# A theme that vanished (an uninstalled skin still named by the config, a
# broken themes_link) must never lock into a black screen with no unlock UI:
# fall back to the stock theme from this generation before considering live.
if [ ! -f "$QS_THEME_PATH/Main.qml" ]; then
    echo "qylock: theme '$QS_THEME' not found at $QS_THEME_PATH; falling back to clockwork/orbital" >&2
    for fb in "$DIR/../themes/clockwork/orbital" "$DIR/themes_link/clockwork/orbital"; do
        if [ -f "$fb/Main.qml" ]; then
            export QS_THEME="clockwork/orbital"
            export QS_THEME_PATH="$fb"
            break
        fi
    done
fi

echo "Locking with Quickshell using theme: $QS_THEME"
echo "Theme path: $QS_THEME_PATH"


# Some compositors draw the lock-surface pointer from a compositor-set cursor,
# not the client's XCURSOR env, so re-assert it through the provider (a no-op
# where the provider has no imperative cursor set).
if [ "$XDG_SESSION_TYPE" = wayland ] && command -v ryoku >/dev/null 2>&1; then
    ryoku wm act cursor.set "$XCURSOR_THEME" "$XCURSOR_SIZE" >/dev/null 2>&1 || true
fi
# Kill active lockers
killall -9 hyprlock swaylock wlogout 2>/dev/null || true

# Execute and supervise the lock screen in this long-lived wrapper. It survives
# a ryoku-shell service restart (KillMode=process), so a later qylock crash can
# recover after the compositor or GPU settles. Three quick retries lead into a
# capped backoff instead of either a hot crash loop or a permanently black lock.
short_failures=0
backoff=1
while :; do
    proof_token="$(< /proc/sys/kernel/random/uuid)"
    export QYLOCK_PROOF_TOKEN="$proof_token"
    "$proof_helper" begin "$proof_token" "$session_id"
    attempt_started=$(date +%s%N)
    # The guard and generation flocks must not reach the client's tree: an
    # inherited fd keeps the flock alive in any coprocess that outlives the
    # client, and a stuck guard makes every later lock a silent no-op.
    if (exec 7>&- 9>&-; exec quickshell -p "$DIR/lock_shell.qml"); then
        rc=0
    else
        rc=$?
    fi
    lifetime_ns=$(( $(date +%s%N) - attempt_started ))
    "$proof_helper" retire "$proof_token" "$session_id" >/dev/null 2>&1 || true
    proof_token=""
    (( rc == 0 )) && exit 0

    # A client that stayed healthy before crashing receives a fresh recovery
    # budget. Repeated short-lived starts get three quick attempts, then retry
    # forever at a bounded rate so a recovering compositor regains its UI.
    if (( lifetime_ns >= 2000000000 )); then
        short_failures=0
        backoff=1
        sleep 0.1
        continue
    fi
    short_failures=$((short_failures + 1))
    if (( short_failures < 3 )); then
        sleep 0.1
        continue
    fi
    echo "qylock: lock client failed repeatedly; retrying in ${backoff}s" >&2
    sleep "$backoff"
    (( backoff < 30 )) && backoff=$((backoff * 2))
    (( backoff > 30 )) && backoff=30
done
