#!/usr/bin/env python3
"""Private, dated API-equivalent token estimates for local Claude history.

Prices come from public catalogs with a bundled offline fallback. No credentials
are read. Missing historical prices remain explicit; saved rates stay frozen.
"""
from copy import deepcopy
from datetime import datetime
import fcntl
import hashlib
import json
import math
import os
from pathlib import Path
import re
import runpy
import tempfile
import time


PRICE_SOURCE = "https://platform.claude.com/docs/en/about-claude/pricing"
PRICING = runpy.run_path(str(Path(__file__).resolve().with_name("model-pricing.py")))
VERIFIED_AT = "2026-11-09"
VERIFIED_TIMESTAMP = "2026-11-09T10:16:46Z"
VERIFIED_PRICES = PRICING["read_bundle"]("anthropic")["models"]
COUNTS = ("input", "output", "cache_read", "cache_write")
SPLITS = ("cache_write_5m", "cache_write_1h")


def _count(value):
    return isinstance(value, int) and not isinstance(value, bool) and 0 <= value <= 2**63 - 1


def _event(message):
    """Store only usage metadata; never persist prompts, paths or account data."""
    identity = message.get("event_id")
    stamp = message.get("time")
    if not isinstance(identity, str) or not identity or isinstance(stamp, bool) \
            or not isinstance(stamp, (int, float)) or not math.isfinite(stamp):
        return None
    try:
        day = datetime.fromtimestamp(stamp / 1000).astimezone().strftime("%Y-%m-%d")
    except (ValueError, OSError, OverflowError):
        return None
    values = {name: message.get(name, 0) for name in COUNTS}
    reasoning = message.get("reasoning", 0)
    if not all(_count(value) for value in values.values()) or not _count(reasoning):
        return None
    # The caller's output excludes reasoning. Native output includes it once.
    values["output"] += reasoning
    if not _count(values["output"]):
        return None
    for name in SPLITS:
        values[name] = message.get(name, 0 if values["cache_write"] == 0 else None)
        if values["cache_write"] == 0 and values[name] is None:
            values[name] = 0
    split_known = all(_count(values[name]) for name in SPLITS) \
        and sum(values[name] for name in SPLITS) == values["cache_write"]
    tools = message.get("server_tool_use") or {}
    # This is a token-cost estimate. Web fetch has no separate fee; searches
    # and other server tools may have fees that these token counters cannot rate.
    extra_tools = not isinstance(tools, dict) or any(
        name != "web_fetch_requests" and value != 0 for name, value in tools.items())
    event = dict(values, date=day, model=str(message.get("model_id") or "unknown"),
                 provider=str(message.get("provider_id") or ""),
                 service_tier=str(message.get("service_tier") or "standard"),
                 inference_geo=str(message.get("inference_geo") or "global"),
                 speed=str(message.get("speed") or "standard"),
                 split_known=split_known, extra_tools=extra_tools)
    return identity, hashlib.sha256(identity.encode()).hexdigest(), event


def price_event(event, prices):
    """Return USD for verified native token classes, or None for unknown rates."""
    if event.get("provider") not in ("", "anthropic") \
            or event.get("service_tier") not in ("standard", "default", "auto", "batch") \
            or event.get("speed") not in ("standard", "fast") \
            or event.get("inference_geo") not in ("global", "us") \
            or not event.get("split_known"):
        return None
    if event.get("service_tier") == "batch" and event.get("speed") == "fast":
        return None
    tier = "fast" if event.get("speed") == "fast" else event.get("service_tier", "standard")
    rate = PRICING["resolve_rate"](prices, event["model"],
        sum(event[name] for name in ("input", "cache_read", "cache_write")), tier)
    if not isinstance(rate, dict):
        return None
    amount = 0.0
    for name in ("input", "output", "cache_read", *SPLITS):
        count, value = event[name], rate.get(name)
        if not count:
            continue
        if isinstance(value, bool) or not isinstance(value, (int, float)) \
                or not math.isfinite(value) or value < 0:
            return None
        amount += count * (value or 0) / 1_000_000
    if event["inference_geo"] == "us":
        amount *= 1.1
    return round(amount, 10)


def _read_ledger(path, month):
    try:
        if path.is_symlink():
            return None
        ledger = json.loads(path.read_text())
    except FileNotFoundError:
        return {"schemaVersion": 1, "month": month, "currency": "USD", "days": {}, "events": {}}
    except (OSError, ValueError):
        return None
    if not isinstance(ledger, dict) or ledger.get("schemaVersion") != 1 \
            or ledger.get("month") != month or ledger.get("currency") != "USD" \
            or not isinstance(ledger.get("days"), dict) or not isinstance(ledger.get("events"), dict):
        return None
    for day, row in ledger["days"].items():
        if not isinstance(day, str) or not re.fullmatch(month + r"-\d{2}", day) \
                or not isinstance(row, dict) or not isinstance(row.get("prices", {}), dict):
            return None
    for event in ledger["events"].values():
        if not isinstance(event, dict) or not isinstance(event.get("date"), str) or event["date"] not in ledger["days"] \
                or not all(isinstance(event.get(name), str) for name in
                           ("model", "provider", "service_tier", "inference_geo", "speed")) \
                or not all(_count(event.get(name)) for name in COUNTS) \
                or not all(name in event and (event[name] is None or _count(event[name])) for name in SPLITS) \
                or not isinstance(event.get("split_known"), bool) \
                or not isinstance(event.get("extra_tools"), bool):
            return None
        if event["split_known"] and (not all(_count(event[name]) for name in SPLITS)
                                      or sum(event[name] for name in SPLITS) != event["cache_write"]):
            return None
    return ledger


def _write_ledger(path, value):
    fd, temporary = tempfile.mkstemp(prefix="." + path.name + ".", dir=path.parent)
    try:
        with os.fdopen(fd, "w") as handle:
            json.dump(value, handle, separators=(",", ":"), allow_nan=False)
            handle.write("\n")
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def estimate_spending(messages, *, state_dir=None, now=None):
    """Return (_spending summary, {input event_id: USD or None}).

    Messages contain time (milliseconds), stable event_id, exact model_id,
    uncached input, nonthinking output, reasoning, cache_read, cache_write,
    cache_write_5m/cache_write_1h, service_tier, inference_geo and speed.
    Optional server_tool_use marks additional, unpriced server-tool fees.
    Only today's first snapshot receives the currently verified rates. Rates
    for past days stay frozen, including absence of a price snapshot. Saved
    events survive transcript removal, and rescans replace rather than add them.
    """
    now = time.time() if now is None else now
    today = datetime.fromtimestamp(now).astimezone().strftime("%Y-%m-%d")
    month = today[:7]
    summary = dict(kind="api-equivalent", month=month, currency="USD", amount=None,
                   partial=True, priced_tokens=0, unpriced_tokens=0,
                   priced_days=[], unpriced_days=[], stale_prices=False,
                   scope="local-history", token_only=True, charges=False,
                   source=PRICE_SOURCE, verified_at=VERIFIED_TIMESTAMP, price_date="")
    canonical, identities, costs, invalid = {}, {}, {}, 0
    for message in messages:
        parsed = _event(message)
        if parsed is None:
            invalid += 1
            continue
        identity, key, event = parsed
        if event["date"] > today:
            invalid += 1
            continue
        identities[key] = identity
        costs[identity] = None
        if key in canonical:
            # Defensive dedup for callers that pass cumulative block snapshots.
            old = canonical[key]
            for name in (*COUNTS, *SPLITS):
                if old[name] is not None and event[name] is not None:
                    event[name] = max(old[name], event[name])
            event["date"] = min(old["date"], event["date"])
            event["split_known"] = old["split_known"] and event["split_known"] \
                and all(_count(event[name]) for name in SPLITS) \
                and sum(event[name] for name in SPLITS) == event["cache_write"]
            event["extra_tools"] |= old["extra_tools"]
            if any(old[name] != event[name] for name in
                   ("model", "provider", "service_tier", "inference_geo", "speed")):
                event["service_tier"] = "unknown"
        canonical[key] = event
    grouped = {}
    for key, event in canonical.items():
        grouped.setdefault(event["date"][:7], {})[key] = event
    grouped.setdefault(month, {})
    root = Path(state_dir or os.environ.get("CLAUDE_USAGE_STATE_DIR") or
                Path(os.environ.get("XDG_STATE_HOME") or Path.home() / ".local/state") / "ai-usage")
    root.mkdir(mode=0o700, parents=True, exist_ok=True)
    price_snapshot = PRICING["load_prices"]("anthropic", state_dir=root, now=now,
        disabled=os.environ.get("CLAUDE_USAGE_DISABLE_PRICES") == "1", fallback=VERIFIED_PRICES,
        required=[(event["model"], sum(event[name] for name in ("input", "cache_read", "cache_write")),
                   "fast" if event["speed"] == "fast" else event["service_tier"])
                  for event in canonical.values() if event["date"] == today])
    fd = os.open(root / "claude-spending.lock", os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    try:
        os.fchmod(fd, 0o600)
        fcntl.flock(fd, fcntl.LOCK_EX)
        for key, rows in sorted(grouped.items()):
            path = root / f"claude-spending-{key}.json"
            ledger = _read_ledger(path, key)
            if ledger is None:
                if key == month:
                    summary["state_error"] = "unreadable-ledger"
                continue
            days, events = ledger["days"], ledger["events"]
            if key == month:
                snapshot = days.setdefault(today, {})
                if VERIFIED_AT <= today:
                    snapshot.update(prices=PRICING["merge_models"](snapshot.get("prices", {}),
                                        price_snapshot["models"], preserve=not snapshot.get("stale_prices", False)),
                                    source=price_snapshot["source"],
                                    price_date=price_snapshot["price_date"],
                                    verified_at=VERIFIED_TIMESTAMP, saved_at=now,
                                    stale_prices=price_snapshot["stale"])
            events.update(rows)
            for event in events.values():
                days.setdefault(event["date"], {})
            for day in days.values():
                day.update(cost=0.0, priced_tokens=0, unpriced_tokens=0, models={}, extra_tools=False)
            for event_key, event in events.items():
                day = days[event["date"]]
                price = price_event(event, day.get("prices", {}))
                tokens = sum(event[name] for name in COUNTS)
                model = day["models"].setdefault(event["model"], dict(
                    input=0, output=0, cache_read=0, cache_write=0, cost=0.0))
                for name in COUNTS:
                    model[name] += event[name]
                if price is None:
                    day["unpriced_tokens"] += tokens
                else:
                    day["priced_tokens"] += tokens
                    day["cost"] += price
                    model["cost"] += price
                day["extra_tools"] |= bool(event.get("extra_tools"))
                if event_key in identities:
                    costs[identities[event_key]] = price
            for day in days.values():
                day["cost"] = round(day["cost"], 10)
                day["partial"] = day["unpriced_tokens"] > 0 or day["extra_tools"]
            ledger["updated_at"] = now
            _write_ledger(path, ledger)
            if key == month:
                priced = sorted(date for date, row in days.items() if row["priced_tokens"])
                missing = sorted(date for date, row in days.items() if row["unpriced_tokens"])
                summary.update(amount=round(sum(row["cost"] for row in days.values()), 6) if priced else None,
                    partial=bool(missing) or bool(invalid) or any(row["extra_tools"] for row in days.values()),
                    priced_tokens=sum(row["priced_tokens"] for row in days.values()),
                    unpriced_tokens=sum(row["unpriced_tokens"] for row in days.values()),
                    priced_days=priced, unpriced_days=missing,
                    stale_prices=any(row.get("stale_prices", False) for row in days.values()),
                    price_date=days.get(today, {}).get("price_date", ""),
                    additional_costs_unpriced=any(row["extra_tools"] for row in days.values()))
    finally:
        os.close(fd)
    if invalid:
        summary["invalid_events"] = invalid
    return summary, costs
