#!/usr/bin/env bash
# Fixture test for ryoku/rashin/usage/codex-usage: Codex limits must come from
# OpenCode's active chatgpt-browser login when it is present, without requiring
# the Codex CLI, and must map the 5h/weekly account windows into the cache the
# AI usage card reads.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
collector="$here/../ryoku/rashin/usage/codex-usage"

if ! command -v python3 >/dev/null 2>&1; then
  echo "codex-usage: python3 not found, skipping" >&2
  exit 0
fi

work="$(mktemp -d)"
server_pid=""
cleanup() {
  [[ -n "$server_pid" ]] && kill "$server_pid" 2>/dev/null || true
  rm -rf "$work"
}
trap cleanup EXIT

fail=0

# --- 1. pure mapping: active chatgpt-browser record and both windows ---
python3 - "$collector" <<'PY' || fail=1
import runpy, sys

g = runpy.run_path(sys.argv[1], run_name="codex_usage")
export = [
    {"integrationID": "openai", "active": False,
     "value": {"type": "oauth", "methodID": "chatgpt-browser", "access": "inactive"}},
    {"integrationID": "openai", "active": True,
     "value": {"type": "oauth", "methodID": "chatgpt-browser", "access": "test-token",
               "metadata": {"accountID": "acct"}}},
]
cred = g["opencode_credential"](export)
assert cred and cred["access"] == "test-token", cred

snapshot = g["opencode_snapshot"]({
    "plan_type": "plus",
    "rate_limit": {
        "primary_window": {"used_percent": 83, "reset_at": 1795709291,
                           "limit_window_seconds": 18000},
        "secondary_window": {"used_percent": 47, "reset_at": 1796000000,
                             "limit_window_seconds": 604800},
    },
})
assert snapshot["source"] == "opencode", snapshot
assert snapshot["plan"] == "plus", snapshot
assert snapshot["rateLimits"]["primary"]["windowDurationMins"] == 300
assert snapshot["rateLimits"]["secondary"]["windowDurationMins"] == 10080
assert snapshot["rateLimits"]["primary"]["usedPercent"] == 83
assert snapshot["rateLimits"]["secondary"]["usedPercent"] == 47
print("  ok: pure OpenCode mapping")
PY

# --- 2. end to end: fake opencode export + local usage endpoint ---
mkdir -p "$work/bin" "$work/home/.cache" "$work/codex" "$work/http"
cat > "$work/http/usage" <<'JSON'
{"plan_type":"plus","rate_limit":{"primary_window":{"used_percent":83,"reset_at":1795709291,"limit_window_seconds":18000},"secondary_window":{"used_percent":47,"reset_at":1796000000,"limit_window_seconds":604800},"limit_reached":false}}
JSON
cat > "$work/bin/opencode" <<'SH'
#!/bin/sh
printf '%s\n' '[{"integrationID":"openai","active":true,"value":{"type":"oauth","methodID":"chatgpt-browser","access":"test-token","metadata":{"accountID":"acct"}}}]'
SH
chmod +x "$work/bin/opencode"

port="$(python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
)"
python3 -m http.server "$port" --bind 127.0.0.1 --directory "$work/http" \
  >"$work/http.log" 2>&1 &
server_pid=$!
sleep 0.4

HOME="$work/home" PATH="$work/bin:$PATH" CODEX_HOME="$work/codex" \
  CODEX_USAGE_CACHE_FILE="$work/cache.json" \
  CODEX_USAGE_ACTIVITY_FILE="$work/activity.json" \
  CODEX_USAGE_DISABLE_RPC=1 \
  CODEX_USAGE_OPENCODE_URL="http://127.0.0.1:$port/usage" \
  "$collector"

python3 - "$work/cache.json" <<'PY' || fail=1
import json, sys
raw = open(sys.argv[1]).read()
data = json.loads(raw)
assert data["_source"] == "opencode", data.get("_source")
assert data["_plan"] == "plus", data.get("_plan")
assert data["5h-utilization"] == "0.8300", data.get("5h-utilization")
assert data["5h-reset"] == "1795709291", data.get("5h-reset")
assert data["7d-utilization"] == "0.4700", data.get("7d-utilization")
assert data["7d-reset"] == "1796000000", data.get("7d-reset")
assert [w["minutes"] for w in data["windows"]] == [300, 10080]
assert "test-token" not in raw
print("  ok: OpenCode cache mapping")
PY

# --- 3. no active credential: keep the last cache as stale, never crash ---
mkdir -p "$work/empty/bin" "$work/empty/home/.cache" "$work/empty/codex"
cat > "$work/empty/bin/opencode" <<'SH'
#!/bin/sh
printf '%s\n' '[]'
SH
chmod +x "$work/empty/bin/opencode"
printf '%s\n' '{"_source":"opencode","5h-utilization":"0.1"}' \
  > "$work/empty/home/.cache/codex-usage.json"
HOME="$work/empty/home" PATH="$work/empty/bin:$PATH" CODEX_HOME="$work/empty/codex" \
  CODEX_USAGE_CACHE_FILE="$work/empty/home/.cache/codex-usage.json" \
  CODEX_USAGE_ACTIVITY_FILE="$work/empty/activity.json" \
  CODEX_USAGE_DISABLE_RPC=1 \
  "$collector"

python3 - "$work/empty/home/.cache/codex-usage.json" <<'PY' || fail=1
import json, sys
data = json.load(open(sys.argv[1]))
assert data["_source"] == "stale", data.get("_source")
print("  ok: no credential falls back to stale cache")
PY

# --- 4. the service guard actually dispatches codex-usage on an OpenCode-only box ---
mkdir -p "$work/guard/bin" "$work/nocodex" "$work/guard/home"
cp "$work/bin/opencode" "$work/guard/bin/opencode"
cat > "$work/guard/bin/codex-usage" <<'SH'
#!/bin/sh
printf '%s\n' called > "$CODEX_CALLED"
SH
cat > "$work/guard/bin/opencode-usage" <<'SH'
#!/bin/sh
printf '%s\n' called > "$OPENCODE_CALLED"
SH
chmod +x "$work/guard/bin/codex-usage" "$work/guard/bin/opencode-usage"
HOME="$work/guard/home" PATH="$work/guard/bin" CODEX_HOME="$work/nocodex" \
  CODEX_CALLED="$work/guard/codex-called" \
  OPENCODE_CALLED="$work/guard/opencode-called" \
  RYOKU_AI_USAGE_BIN_DIR="$work/guard/bin" \
  "$here/../ryoku/rashin/usage/ryoku-ai-usage"
if [[ -f "$work/guard/codex-called" ]]; then
  echo "  ok: OpenCode alone dispatches the Codex collector"
else
  echo "  FAIL: OpenCode alone did not dispatch the Codex collector" >&2
  fail=1
fi

if (( fail )); then
  echo "codex-usage: FAILED" >&2
  exit 1
fi
echo "codex-usage: OpenCode subscription limits read correctly"
