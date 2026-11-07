#!/usr/bin/env bash
# Fixture test for ryoku/rashin/usage/codex-usage: the AI usage card must read a
# real Codex rollout (session_meta, thread settings, token_count events) without
# the app-server, and keep the shape its cache promises: per-day buckets,
# per-session rows, per-period figures, and the plan's own 30-day window.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
collector="$here/../ryoku/rashin/usage/codex-usage"

if ! command -v python3 >/dev/null 2>&1; then
  echo "codex-usage: python3 not found, skipping" >&2
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
  got="$(OC_FIELD="$field" OC_CACHE="$cache" python3 -c \
'import json, os
print(json.load(open(os.environ["OC_CACHE"])).get(os.environ["OC_FIELD"], ""))')"
  if [[ "$got" == "$want" ]]; then
    echo "  ok: $label"
  else
    echo "  FAIL: $label -> $field=$got, want $want" >&2
    fail=1
  fi
}

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

d="$work/rollout"
mkdir -p "$d/home/.cache" "$d/home/.codex/sessions/2026/11/06"
python3 - "$d/home/.codex/sessions/2026/11/06/rollout-fixture.jsonl" <<'PY'
import json, sys, time

path = sys.argv[1]
now = int(time.time())
iso = lambda offset: time.strftime(
    "%Y-%m-%dT%H:%M:%SZ", time.gmtime(now + offset))
session = "01a1b204-fixture"
cwd = "/home/test/project"
lines = [
    {"timestamp": iso(-300), "type": "session_meta",
     "payload": {"id": session, "session_id": session, "cwd": cwd,
                 "timestamp": iso(-300)}},
    {"timestamp": iso(-299), "type": "event_msg",
     "payload": {"type": "thread_settings_applied",
                 "thread_settings": {"model": "gpt-5.6-terra",
                                     "model_provider_id": "openai"}}},
    {"timestamp": iso(-290), "type": "response_item",
     "payload": {"type": "message", "role": "user",
                 "content": [{"type": "input_text",
                              "text": "Fix the failing snapshot test"}]}},
    {"timestamp": iso(-296), "type": "event_msg",
     "payload": {"type": "task_started"}},
    {"timestamp": iso(-280), "type": "event_msg",
     "payload": {"type": "token_count",
                 "info": {"total_token_usage": {
                     "input_tokens": 700, "cached_input_tokens": 200,
                     "output_tokens": 300, "total_tokens": 1000}},
                 "rate_limits": {
                     "limit_id": "codex", "plan_type": "free",
                     "primary": {"used_percent": 0.0,
                                 "window_minutes": 43200,
                                 "resets_at": now + 2500000}}}},
    {"timestamp": iso(-60), "type": "event_msg",
     "payload": {"type": "token_count",
                 "info": {"total_token_usage": {
                     "input_tokens": 1800, "cached_input_tokens": 900,
                     "output_tokens": 1200, "total_tokens": 3000}},
                 "rate_limits": {
                     "limit_id": "codex", "plan_type": "free",
                     "primary": {"used_percent": 48.0,
                                 "window_minutes": 43200,
                                 "resets_at": now + 2500000}}}},
]
lines.append({"timestamp": iso(-50), "type": "event_msg",
              "payload": {"type": "task_complete"}})
with open(path, "w") as fh:
    for line in lines:
        fh.write(json.dumps(line) + "\n")

started = [
    {"timestamp": iso(-40), "type": "session_meta",
     "payload": {"id": "01a1b204-started", "session_id": "01a1b204-started",
                 "cwd": "/home/test/project", "timestamp": iso(-40)}},
    {"timestamp": iso(-39), "type": "event_msg",
     "payload": {"type": "thread_settings_applied",
                 "thread_settings": {"model": "gpt-5.6-terra",
                                     "model_provider_id": "openai"}}},
    {"timestamp": iso(-30), "type": "response_item",
     "payload": {"type": "message", "role": "user",
                 "content": [{"type": "input_text",
                              "text": "Start the long build"}]}},
    {"timestamp": iso(-20), "type": "event_msg",
     "payload": {"type": "task_started"}},
]
with open(str(path).replace("fixture.jsonl", "started.jsonl"), "w") as fh:
    for line in started:
        fh.write(json.dumps(line) + "\n")
PY

cache="$d/home/.cache/codex-usage.json"
HOME="$d/home" CODEX_HOME="$d/home/.codex" \
  CODEX_USAGE_DISABLE_RPC=1 \
  CODEX_USAGE_CACHE_FILE="$cache" \
  CODEX_USAGE_ACTIVITY_FILE="$d/home/.cache/codex-usage-activity.json" \
  "$collector"

check "rollout is the source" _source rollout "$cache"
check "profile from the rollout" _plan free "$cache"

read -r minutes utilization reset <<<"$(OC_CACHE="$cache" python3 -c '
import json, os
window = (json.load(open(os.environ["OC_CACHE"])).get("_windows") or [{}])[0]
print(window.get("minutes", 0), window.get("utilization", 0), window.get("reset", 0))')"
if [[ "$minutes" == "43200" && "$utilization" == "0.48" && "$reset" -gt 0 ]]; then
  echo "  ok: 30-day window with its reset"
else
  echo "  FAIL: window -> minutes=$minutes utilization=$utilization reset=$reset" >&2
  fail=1
fi

read -r day_tokens day_cache day_count <<<"$(OC_CACHE="$cache" python3 -c '
import json, os
days = json.load(open(os.environ["OC_CACHE"])).get("_days") or []
print(sum(row["tokens"] for row in days),
      sum(row["cacheRead"] for row in days), len(days))')"
if [[ "$day_tokens" == "2100" && "$day_cache" == "900" && "$day_count" -ge 1 ]]; then
  echo "  ok: day buckets keep the token deltas"
else
  echo "  FAIL: days -> tokens=$day_tokens cacheRead=$day_cache count=$day_count" >&2
  fail=1
fi

map="$(OC_CACHE="$cache" python3 -c '
import json, os
rows = json.load(open(os.environ["OC_CACHE"])).get("_sessions") or []
for row in rows:
    print(row.get("title", "").replace(" ", "_") + "=" + str(row.get("running")))')"
if grep -q "Fix_the_failing_snapshot_test=False" <<<"$map" \
    && grep -q "Start_the_long_build=True" <<<"$map"; then
  echo "  ok: sessions carry tokens and turn lifecycle"
else
  echo "  FAIL: sessions -> $map" >&2
  fail=1
fi

read -r period_tokens <<<"$(OC_CACHE="$cache" python3 -c '
import json, os
period = (json.load(open(os.environ["OC_CACHE"])).get("_periods") or {}).get("all") or {}
print(period.get("tokens", 0))')"
if [[ "$period_tokens" == "2100" ]]; then
  echo "  ok: period figures"
else
  echo "  FAIL: period -> tokens=$period_tokens" >&2
  fail=1
fi

if (( fail )); then
  echo "codex-usage: FAILED" >&2
  exit 1
fi
echo "codex-usage: rollout fixture read correctly"
