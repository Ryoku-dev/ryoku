#!/usr/bin/env bash
# Fixture test for ryoku/rashin/usage/grok-usage: the AI usage pill counts each
# finished Grok turn once, from the `turn_completed` records in the session
# logs. Builds throwaway GROK_HOME session trees and asserts the cache totals.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
collector="$here/../ryoku/rashin/usage/grok-usage"

if ! command -v python3 >/dev/null 2>&1; then
  echo "grok-usage: python3 not found, skipping" >&2
  exit 0
fi

fail=0

# check LABEL FIELD EXPECTED CACHE_JSON
check() {
  local label="$1" field="$2" want="$3" cache="$4" got
  if [[ ! -f "$cache" ]]; then
    echo "  FAIL: $label -> collector wrote no cache" >&2
    fail=1
    return
  fi
  got="$(GK_FIELD="$field" GK_CACHE="$cache" python3 -c \
'import json, os
print(json.load(open(os.environ["GK_CACHE"])).get(os.environ["GK_FIELD"], ""))')"
  if [[ "$got" == "$want" ]]; then
    echo "  ok: $label"
  else
    echo "  FAIL: $label -> $field=$got, want $want" >&2
    fail=1
  fi
}

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# --- 1. real turns, a fork repeating its parent's turn, and chat noise ---
d="$work/sessions"
mkdir -p "$d/home/.cache"
python3 - "$d/grok" <<'PY'
import json, os, sys, time
root = sys.argv[1]
now = int(time.time())
# Mid-morning today, so the turn is in today's bucket whatever the run time.
today = int(time.mktime(time.localtime()[:3] + (0, 0, 0, 0, 0, -1)))
stamp = max(today + 60, now - 120)

def turn(prompt, total, ticks, model="grok-4.7-build", at=stamp):
    usage = {"inputTokens": total - 100, "outputTokens": 100, "totalTokens": total,
             "cachedReadTokens": total // 2, "cacheCreationTokens": 0,
             "reasoningTokens": 40, "costUsdTicks": ticks}
    return {"timestamp": at, "method": "_x.ai/session/update",
            "params": {"sessionId": "s", "update": {
                "sessionUpdate": "turn_completed", "prompt_id": prompt,
                "usage": dict(usage, modelUsage={model: usage})},
                "_meta": {"agentTimestampMs": at * 1000}}}

# Another update type that names turn_completed and carries usage-shaped data
# must not count: only a turn_completed update is a finished turn.
chunk = {"timestamp": stamp, "method": "session/update", "params": {"sessionId": "s",
         "update": {"sessionUpdate": "agent_message_chunk", "status": "turn_completed",
                    "prompt_id": "noise", "usage": {"totalTokens": 999999}},
         "_meta": {"totalTokens": 999999}}}

def write(cwd, sid, lines):
    path = os.path.join(root, "sessions", cwd, sid)
    os.makedirs(path)
    with open(os.path.join(path, "updates.jsonl"), "w") as f:
        for line in lines:
            f.write(json.dumps(line) + "\n")

write("%2Fsrc%2Fa", "s1", [chunk, turn("p1", 1000, 15_000_000_000), turn("p2", 500, 5_000_000_000)])
# a fork inherits p1 and adds p3
# a turn whose _meta is not an object still counts, by its envelope timestamp
odd = turn("p4", 100, 0)
odd["params"]["_meta"] = ["not", "an", "object"]
write("%2Fsrc%2Fb", "s2", [turn("p1", 1000, 15_000_000_000), turn("p3", 250, 0, model="grok-4.6"), odd])
PY
HOME="$d/home" GROK_HOME="$d/grok" "$collector"
cache="$d/home/.cache/grok-usage.json"
check "source is the session logs" _source sessions "$cache"
# p1 + p2 + p3 + p4 = 1850; the fork's copy of p1 and the chat chunk add nothing.
check "each turn counted once" _today_tokens 1850 "$cache"
check "7-day total" _7d_tokens 1850 "$cache"
check "cost is ticks / 1e10" _cost_today 2.0 "$cache"
check "no log means no allowance" 7d-utilization "" "$cache"

# --- 2. the newest current plan reading in Grok's log is the weekly allowance ---
mkdir -p "$d/grok/logs"
python3 - "$d/grok/logs/unified.jsonl" <<'PY'
import json, sys, time
from datetime import datetime, timezone
iso = lambda t: datetime.fromtimestamp(t, timezone.utc).isoformat()
now = int(time.time())
marker = "billing: fetched credits config"
def reading(pct, end, at):
    return {"ts": iso(at), "msg": marker, "ctx": {"subscriptionTier": "SuperGrok Plus",
            "config": {"creditUsagePercent": pct, "currentPeriod": {
                "type": "USAGE_PERIOD_TYPE_WEEKLY", "start": iso(end - 7 * 86400), "end": iso(end)}}}}
with open(sys.argv[1], "w") as f:
    for line in (reading(2.0, now + 86400, now - 600),
                 reading(5.0, now + 86400, now - 60),
                 {"ts": iso(now), "msg": "user typed: " + marker, "ctx": {"config": {"creditUsagePercent": 99}}}):
        f.write(json.dumps(line) + "\n")
PY
HOME="$d/home" GROK_HOME="$d/grok" "$collector"
check "newest reading wins" 7d-utilization 0.0500 "$cache"
check "plan tier from the log" _plan "SuperGrok Plus" "$cache"
check "tokens still counted with a limit" _today_tokens 1850 "$cache"

# --- 3. a reading whose period has ended is not shown as current ---
python3 - "$d/grok/logs/unified.jsonl" <<'PY'
import json, sys, time
from datetime import datetime, timezone
iso = lambda t: datetime.fromtimestamp(t, timezone.utc).isoformat()
now = int(time.time())
json.dump({"ts": iso(now - 3 * 86400), "msg": "billing: fetched credits config",
           "ctx": {"config": {"creditUsagePercent": 80.0,
                              "currentPeriod": {"end": iso(now - 3600)}}}}, open(sys.argv[1], "w"))
PY
rm -f "$cache"
HOME="$d/home" GROK_HOME="$d/grok" "$collector"
check "expired period drops the allowance" 7d-utilization "" "$cache"

# --- 3b. a current period that is not weekly is not shown as the weekly window ---
python3 - "$d/grok/logs/unified.jsonl" <<'PY'
import json, sys, time
from datetime import datetime, timezone
iso = lambda t: datetime.fromtimestamp(t, timezone.utc).isoformat()
now = int(time.time())
json.dump({"ts": iso(now - 60), "msg": "billing: fetched credits config",
           "ctx": {"config": {"creditUsagePercent": 30.0,
                              "currentPeriod": {"type": "USAGE_PERIOD_TYPE_MONTHLY",
                                                "end": iso(now + 20 * 86400)}}}}, open(sys.argv[1], "w"))
PY
rm -f "$cache"
HOME="$d/home" GROK_HOME="$d/grok" "$collector"
check "non-weekly period drops the allowance" 7d-utilization "" "$cache"

# --- 3c. a rotated log keeps the last published reading until its week ends ---
: > "$d/grok/logs/unified.jsonl"
now=$(date +%s)
printf '{"7d-utilization":"0.0700","7d-reset":"%s","_plan":"SuperGrok Plus","_limit_checked_at":%s}\n' \
  "$((now + 86400))" "$((now - 3600))" > "$cache"
HOME="$d/home" GROK_HOME="$d/grok" "$collector"
check "rotated log keeps the current reading" 7d-utilization 0.0700 "$cache"
check "kept reading keeps its tier" _plan "SuperGrok Plus" "$cache"
check "kept reading keeps its age" _limit_checked_at "$((now - 3600))" "$cache"
printf '{"7d-utilization":"0.0700","7d-reset":"%s"}\n' "$((now - 60))" > "$cache"
HOME="$d/home" GROK_HOME="$d/grok" "$collector"
check "a kept reading ends with its week" 7d-utilization "" "$cache"

# --- 4. a turn older than the 7-day window does not count ---
d="$work/old"
mkdir -p "$d/home/.cache" "$d/grok/sessions/%2Fold/s9"
python3 - "$d/grok/sessions/%2Fold/s9/updates.jsonl" <<'PY'
import json, sys, time
at = int(time.time()) - 10 * 86400
usage = {"totalTokens": 777, "costUsdTicks": 0}
json.dump({"timestamp": at, "params": {"update": {"sessionUpdate": "turn_completed",
           "prompt_id": "old", "usage": usage}, "_meta": {"agentTimestampMs": at * 1000}}},
          open(sys.argv[1], "w"))
PY
HOME="$d/home" GROK_HOME="$d/grok" "$collector"
check "old turns are outside the window" _7d_tokens 0 "$d/home/.cache/grok-usage.json"

# --- 5. no Grok home degrades to stale, never a crash ---
d="$work/none"
mkdir -p "$d/home/.cache"
echo '{"_source":"sessions","_today_tokens":9}' > "$d/home/.cache/grok-usage.json"
HOME="$d/home" GROK_HOME="$d/missing" "$collector"
check "no sessions marks prior cache stale" _source stale "$d/home/.cache/grok-usage.json"

if (( fail )); then
  echo "grok-usage: FAILED" >&2
  exit 1
fi
echo "grok-usage: session turns counted once"
