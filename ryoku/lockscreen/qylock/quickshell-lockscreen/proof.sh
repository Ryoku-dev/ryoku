#!/usr/bin/env bash
set -euo pipefail

action=${1:-}
if [[ $action == status ]]; then
    token=""
    session=${2:-${XDG_SESSION_ID:-}}
else
    token=${2:-}
    session=${3:-${XDG_SESSION_ID:-}}
    [[ $token =~ ^[A-Za-z0-9-]{16,128}$ ]] || exit 2
fi
[[ $session =~ ^[A-Za-z0-9_.-]+$ ]] || exit 2

runtime=${XDG_RUNTIME_DIR:-/tmp}
marker="$runtime/qylock.$session.locked"
expected="$runtime/qylock.$session.expected"
lock="$runtime/qylock.$session.proof.lock"

exec 8>"$lock"
flock 8
read_value() {
    local path=$1 value=""
    [[ -r $path ]] && read -r value <"$path" || true
    printf '%s\n' "$value"
}
write_value() {
    local path=$1 value=$2 tmp
    tmp="${path}.$$"
    (umask 077; printf '%s\n' "$value" >"$tmp")
    mv -f "$tmp" "$path"
}
process_holds_proof() {
    local wanted=$1 proc cmdline entry process_session="" process_token=""
    for proc in /proc/[0-9]*; do
        [[ -r $proc/environ && -r $proc/cmdline ]] || continue
        cmdline="$(cat "$proc/cmdline" 2>/dev/null | tr '\0' ' ' 2>/dev/null || true)"
        [[ $cmdline == *quickshell*lock_shell.qml* ]] || continue
        process_session=""
        process_token=""
        while IFS= read -r -d '' entry; do
            case "$entry" in
                XDG_SESSION_ID=*) process_session=${entry#*=} ;;
                QYLOCK_PROOF_TOKEN=*) process_token=${entry#*=} ;;
            esac
        done <"$proc/environ" 2>/dev/null || true
        if [[ $process_session == "$session" && $process_token == "$wanted" ]]; then
            return 0
        fi
    done
    return 1
}

case "$action" in
    begin)
        rm -f "$marker"
        write_value "$expected" "$token"
        ;;
    publish)
        [[ $(read_value "$expected") == "$token" ]] || exit 1
        write_value "$marker" "$token"
        ;;
    clear)
        if [[ $(read_value "$marker") == "$token" ]]; then
            rm -f "$marker"
        fi
        ;;
    retire)
        if [[ $(read_value "$marker") == "$token" ]]; then
            rm -f "$marker"
        fi
        if [[ $(read_value "$expected") == "$token" ]]; then
            rm -f "$expected"
        fi
        ;;
    status)
        token="$(read_value "$expected")"
        [[ $token =~ ^[A-Za-z0-9-]{16,128}$ ]] || exit 1
        [[ $(read_value "$marker") == "$token" ]] || exit 1
        process_holds_proof "$token"
        ;;
    *)
        printf 'usage: %s <begin|publish|clear|retire> <token> [session-id]\n' "$0" >&2
        printf '       %s status [session-id]\n' "$0" >&2
        exit 2
        ;;
esac
