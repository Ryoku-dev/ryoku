#!/bin/sh
# Restore a hard sleep block and invalidate this generation's proof in one
# serialized transaction. The stable helper falls back to a durable transient
# inhibitor while the matching daemon generation is restarting.
set -eu

session_id=${XDG_SESSION_ID:-}
case "$session_id" in
    *[!A-Za-z0-9_.-]*|'') exit 2 ;;
esac
prepare="$(dirname "$0")/ryoku-qylock-unlock-prepare"
[ -x "$prepare" ] || prepare="$(command -v ryoku-qylock-unlock-prepare)"
"$prepare" "$session_id"
exec loginctl unlock-session "$session_id"
