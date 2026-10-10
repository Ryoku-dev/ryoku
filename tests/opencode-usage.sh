#!/usr/bin/env bash
# Fixture test for ryoku/shell/bin/opencode-usage: the AI usage pill must see
# OpenCode's local session store across every storage layout, including the 2.x
# session_message table that replaced the pre-2.x one (#365). Builds throwaway
# sqlite/file fixtures in temp dirs, points the collector at them through
# OPENCODE_DATA_DIR, and asserts the cache it writes carries real tokens.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
collector="$here/../ryoku/shell/bin/opencode-usage"

if ! command -v python3 >/dev/null 2>&1; then
  echo "opencode-usage: python3 not found, skipping" >&2
  exit 0
fi

fail=0

# check LABEL FIELD EXPECTED CACHE_JSON
check() {
  local label="$1" field="$2" want="$3" cache="$4" got
  if [[ ! -f "$cache" ]]; then
    echo "  FAIL: $label -> collector wrote no cache (source stayed stale)" >&2
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

# --- 1. 2.x session_message table (the regression: was read as stale/zero) ---
d="$work/v2"
mkdir -p "$d/data" "$d/home/.cache"
python3 - "$d/data/opencode.db" <<'PY'
import sqlite3, sys, json, time
db = sqlite3.connect(sys.argv[1])
db.execute("create table session_message(id text primary key, session_id text,"
           " type text, time_created integer, time_updated integer, data text)")
now = int(time.time() * 1000)
db.executemany("insert into session_message values(?,?,?,?,?,?)", [
    ("m1", "ses_1", "assistant", now, now, json.dumps({
        "role": "assistant",
        "tokens": {"input": 5000, "output": 1200, "reasoning": 80,
                   "cache": {"read": 40000, "write": 10}},
        "model": {"id": "gpt-5.5", "providerID": "openai"},
        "time": {"created": now - 60000, "completed": now - 30000},
        "cost": 0.42})),
    ("m2", "ses_1", "user", now, now, json.dumps({"role": "user"})),
])
db.commit()
PY
HOME="$d/home" OPENCODE_DATA_DIR="$d/data" "$collector"
cache="$d/home/.cache/opencode-usage.json"
check "2.x session_message source" _source storage "$cache"
# 5000+1200+80+10 = 6290 (cache.read excluded from the cap total by design).
check "2.x token total" _tokens_used 6290 "$cache"
check "2.x newest model" _model "gpt-5.5 (openai)" "$cache"

# --- 2. schema drift: blob/role/time columns named differently ---
d="$work/drift"
mkdir -p "$d/data" "$d/home/.cache"
python3 - "$d/data/opencode.db" <<'PY'
import sqlite3, sys, json, time
db = sqlite3.connect(sys.argv[1])
db.execute("create table session_message(id text, session_id text,"
           " type text, created_at integer, content text)")
now = int(time.time() * 1000)
db.execute("insert into session_message values(?,?,?,?,?)",
           ("x", "s", "assistant", now,
            json.dumps({"tokens": {"input": 7, "output": 3},
                        "model": {"id": "drifted", "providerID": "p"},
                        "time": {"completed": now}, "cost": 0})))
db.commit()
PY
HOME="$d/home" OPENCODE_DATA_DIR="$d/data" "$collector"
cache="$d/home/.cache/opencode-usage.json"
check "drifted column names still read" _tokens_used 10 "$cache"
check "drifted model label" _model "drifted (p)" "$cache"

# --- 3. legacy sqlite message table still works ---
d="$work/legacy"
mkdir -p "$d/data" "$d/home/.cache"
python3 - "$d/data/opencode.db" <<'PY'
import sqlite3, sys, json, time
db = sqlite3.connect(sys.argv[1])
db.execute("create table message(session_id text, time_created integer,"
           " time_updated integer, data text)")
now = int(time.time() * 1000)
db.execute("insert into message values(?,?,?,?)",
           ("ses_9", now, now, json.dumps({
               "role": "assistant", "modelID": "gemini-x", "providerID": "google",
               "tokens": {"input": 100, "output": 50},
               "time": {"created": now, "completed": now}, "cost": 0.01})))
db.commit()
PY
HOME="$d/home" OPENCODE_DATA_DIR="$d/data" "$collector"
cache="$d/home/.cache/opencode-usage.json"
check "legacy message table" _tokens_used 150 "$cache"

# --- 4. filesystem JSON store still works ---
d="$work/fs"
mkdir -p "$d/data/storage/message/sess_a" "$d/home/.cache"
python3 - "$d/data/storage/message/sess_a/m1.json" <<'PY'
import sys, json, time
now = int(time.time() * 1000)
json.dump({"role": "assistant", "modelID": "fs-model", "providerID": "p",
           "tokens": {"input": 20, "output": 5},
           "time": {"completed": now}, "cost": 0}, open(sys.argv[1], "w"))
PY
HOME="$d/home" OPENCODE_DATA_DIR="$d/data" "$collector"
cache="$d/home/.cache/opencode-usage.json"
check "filesystem store" _tokens_used 25 "$cache"

# --- 5. an absent store degrades to stale, never a crash ---
d="$work/none"
mkdir -p "$d/data" "$d/home/.cache"
echo '{"_source":"storage","_tokens_used":9}' > "$d/home/.cache/opencode-usage.json"
HOME="$d/home" OPENCODE_DATA_DIR="$d/data" "$collector"
check "no data marks prior cache stale" _source stale "$d/home/.cache/opencode-usage.json"

if (( fail )); then
  echo "opencode-usage: FAILED" >&2
  exit 1
fi
echo "opencode-usage: all store layouts read correctly"
