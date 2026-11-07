#!/usr/bin/env python3
"""End-to-end check of the AI usage cache the widget reads.

Runs the repository collector against a temporary cache, then asserts the
contract IrisCompact.qml and the shell's own AI panel rely on. Exit code 0 = every field present and sane.

    python3 test/cache_contract.py
"""
import json
import os
import sqlite3
import subprocess
import sys
import tempfile
from datetime import datetime, timedelta
from pathlib import Path

COLLECTOR = Path(__file__).resolve().parent.parent / "ryoku" / "rashin" / "usage" / "opencode-usage"
WORK = tempfile.TemporaryDirectory(prefix="ai-usage-contract-")
CACHE = Path(WORK.name) / "usage.json"

failures = []


def check(condition, message):
    if not condition:
        failures.append(message)


def main():
    # A clean CI runner has no OpenCode account or store. Build the same cache
    # contract from isolated message/session rows rather than the caller's data.
    data_dir = Path(WORK.name) / "opencode"
    data_dir.mkdir()
    now_ms = int(datetime.now().timestamp() * 1000)
    with sqlite3.connect(data_dir / "opencode.db") as db:
        db.execute("create table session_v2 (id text primary key, title text, "
                   "directory text, time_created integer, time_updated integer, "
                   "time_idle integer, time_busy integer, tokens_input integer, "
                   "tokens_output integer, tokens_reasoning integer, "
                   "tokens_cache_read integer, tokens_cache_write integer, "
                   "cost real, model text)")
        db.execute("create table session_message (id text primary key, session_id text, "
                   "type text, time_created integer, time_updated integer, data text)")
        for index, age_ms in enumerate((3600000, 0)):
            sid = f"contract-session-{index}"
            stamp = now_ms - age_ms
            model = {"id": f"contract-model-{index}", "providerID": "fixture"}
            inp = 100 * (index + 1)
            db.execute("insert into session_v2 values (?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
                       (sid, "Contract fixture", "/fixture", stamp, stamp,
                        stamp if index == 0 else 0, stamp - 1, inp, 20, 5,
                        200, 10, 0.01, json.dumps(model)))
            message = {"role": "assistant", "model": model,
                       "tokens": {"input": inp, "output": 20, "reasoning": 5,
                                  "cache": {"read": 200, "write": 10}},
                       "time": {"created": stamp, "completed": stamp}, "cost": 0.01}
            db.execute("insert into session_message values (?,?,?,?,?,?)",
                       (f"message-{index}", sid, "assistant", stamp, stamp,
                        json.dumps(message)))
    if COLLECTOR.exists():
        env = dict(os.environ, HOME=str(Path(WORK.name) / "home"),
                   OPENCODE_DATA_DIR=str(data_dir))
        subprocess.run([str(COLLECTOR), "--cache", str(CACHE)], check=True, env=env)

    check(CACHE.exists(), f"cache missing: {CACHE}")
    if not CACHE.exists():
        return report()

    data = json.loads(CACHE.read_text())

    # Keys the shipped shell panel still reads.
    for key in ("5h-utilization", "7d-utilization", "_recent_days", "_models",
                "_tokens_used", "_window_limit", "_today_tokens", "_7d_tokens"):
        check(key in data, f"missing shell key: {key}")

    # Daily history for Trend / Activity.
    days = data.get("_days", [])
    check(isinstance(days, list) and len(days) > 0, "_days empty")
    check(all({"date", "tokens", "cacheRead"} <= set(day) for day in days),
          "_days rows need date+tokens+cacheRead")
    check([day["date"] for day in days] == sorted(day["date"] for day in days),
          "_days not oldest-to-newest")
    check(days and days[-1]["date"] == datetime.now().strftime("%Y-%m-%d"),
          "_days does not end today")
    check(all(day["tokens"] >= 0 for day in days), "negative token count in _days")
    check(all(day["cacheRead"] >= 0 for day in days), "negative cacheRead in _days")

    # Hourly history.
    hours = data.get("_hours", [])
    check(isinstance(hours, list) and len(hours) == 48, f"_hours should be 48, got {len(hours)}")
    check(all({"start", "tokens", "cacheRead"} <= set(hour) for hour in hours),
          "_hours rows need start+tokens+cacheRead")
    check(all(hour["cacheRead"] >= 0 for hour in hours), "negative cacheRead in _hours")
    if hours:
        now_hour = datetime.now().replace(minute=0, second=0, microsecond=0)
        check(abs(hours[-1]["start"] - int(now_hour.timestamp() * 1000)) < 60_000,
              "_hours does not end in the current hour")

    # Period slices drive every card.
    periods = data.get("_periods", {})
    for key in ("today", "24h", "7", "30", "all"):
        period = periods.get(key)
        check(period is not None, f"_periods missing {key}")
        if not period:
            continue
        for field in ("tokens", "cacheRate", "cost", "sessions", "models"):
            check(field in period, f"_periods[{key}] missing {field}")
        check(0 <= period.get("cacheRate", 0) <= 1, f"_periods[{key}] cacheRate out of range")
        check(period.get("tokens", 0) >= 0, f"_periods[{key}] negative tokens")
        models = period.get("models", [])
        check(len(models) <= 8, f"_periods[{key}] more than 8 models")
        weights = [m["totalWithCacheRead"] for m in models]
        check(weights == sorted(weights, reverse=True),
              f"_periods[{key}] models not sorted by cache-inclusive total")
        for model in models:
            check({"name", "total", "totalLabel", "totalWithCacheRead",
                   "totalWithCacheReadLabel", "cacheRate", "pct"} <= set(model),
                  f"_periods[{key}] model row incomplete: {model.get('name')}")
            check(model.get("totalWithCacheRead", 0) >= model.get("total", 0),
                  f"_periods[{key}] cache-inclusive total below total")
            check(0 <= model.get("cacheRate", 0) <= 1, f"_periods[{key}] model cacheRate range")
            check(0 <= model.get("pct", 0) <= 100, f"_periods[{key}] model pct range")

    # Windows must nest.
    token = lambda key: (periods.get(key) or {}).get("tokens", 0)
    check(token("today") <= token("24h"), "today larger than 24h")
    check(token("7") <= token("30") <= token("all"), "7d/30d/all not nested")

    # Live sessions for the Now card.
    sessions = data.get("_sessions", [])
    check(isinstance(sessions, list) and len(sessions) > 0, "_sessions empty")
    for session in sessions:
        check({"id", "title", "started", "updated", "tokens", "running"} <= set(session),
              f"session row incomplete: {session.get('id')}")
        check(isinstance(session.get("running"), bool), "session.running is not a bool")
        check(session.get("tokens", 0) >= 0, "session negative tokens")
    if sessions:
        check([s["updated"] for s in sessions] == sorted((s["updated"] for s in sessions), reverse=True),
              "_sessions not newest-first")

    # Stale guard: the file must be fresher than a day.
    updated = data.get("updated_at", "")
    check(bool(updated), "missing updated_at")

    return report()


def report():
    if failures:
        print("FAIL")
        for failure in failures:
            print("  -", failure)
        return 1
    print("OK: cache contract holds")
    return 0


if __name__ == "__main__":
    sys.exit(main())
