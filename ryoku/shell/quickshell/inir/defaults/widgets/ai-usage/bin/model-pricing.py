#!/usr/bin/env python3
"""Public API token prices for the widget's Codex and Claude collectors.

Only public metadata is downloaded. A bundled catalog and private last-good
cache keep collection working offline. Ledgers own their dated price snapshots.
"""
from concurrent.futures import ThreadPoolExecutor
from copy import deepcopy
from datetime import datetime, timezone
import json
import math
import os
from pathlib import Path
import re
import tempfile
import time
import urllib.request

CATALOG_URL = "https://models.dev/api.json"
OFFICIAL_URLS = {
    "openai": "https://developers.openai.com/api/docs/pricing.md",
    "anthropic": "https://platform.claude.com/docs/en/about-claude/pricing.md",
}
FIELDS = ("input", "output", "cache_read", "cache_write", "cache_write_5m",
          "cache_write_1h", "cache_write_12h")
REFRESH_SECONDS = 86400
RETRY_SECONDS = 900
MAX_BYTES = 16 * 1024 * 1024
BUNDLE = Path(__file__).resolve().with_name("model-prices.json")


def number(value):
    return isinstance(value, (int, float)) and not isinstance(value, bool) \
        and 0 <= value <= 1_000_000 and math.isfinite(value)


def clean_rate(value):
    if not isinstance(value, dict):
        return None
    result = {key: float(value[key]) for key in FIELDS if number(value.get(key))}
    if not all(key in result for key in ("input", "output")):
        return None
    if value.get("unsupported") is True:
        return None
    tiers = []
    raw_tiers = value.get("tiers", [])
    raw_services = value.get("service_tiers", {})
    if not isinstance(raw_tiers, list) or not isinstance(raw_services, dict):
        return None
    for tier in raw_tiers:
        above = tier.get("above") if isinstance(tier, dict) else None
        if not isinstance(above, (int, float)) or isinstance(above, bool) \
                or not 0 < above <= 2**63 - 1 or int(above) != above:
            return None
        if tier.get("unsupported") is True:
            tiers.append({"above": int(tier["above"]), "unsupported": True})
            continue
        row = clean_rate({key: val for key, val in tier.items() if key != "tiers"})
        if row is None:
            return None
        tiers.append(dict(row, above=int(tier["above"])))
    if tiers:
        result["tiers"] = sorted(tiers, key=lambda row: row["above"])
    services = {}
    for name, row in raw_services.items():
        if name not in ("fast", "flex", "batch", "ultrafast"):
            continue
        if not isinstance(row, dict):
            continue
        rate = clean_rate({key: val for key, val in row.items() if key != "service_tiers"})
        if rate:
            services[name] = rate
    if services:
        result["service_tiers"] = services
    for key in ("source", "release_date"):
        if isinstance(value.get(key), str):
            result[key] = value[key][:300]
    return result


def clean_models(models):
    if not isinstance(models, dict) or len(models) > 10000:
        return {}
    result = {}
    for name, value in models.items():
        if not isinstance(name, str) or not re.fullmatch(r"[a-z0-9][a-z0-9._:/-]{0,199}", name):
            continue
        rate = clean_rate(value)
        if rate is not None:
            result[name] = rate
    return result


def normalize_catalog(catalog, provider):
    result = {}
    if not isinstance(catalog, dict) or not isinstance(catalog.get(provider), dict):
        return result
    models = catalog[provider].get("models")
    if not isinstance(models, dict):
        return result
    for name, model in models.items():
        if not isinstance(model, dict) or not isinstance(model.get("cost"), dict):
            continue
        modalities = model.get("modalities") or {}
        if not isinstance(modalities, dict):
            continue
        if modalities.get("output") and modalities["output"] != ["text"]:
            continue
        value = deepcopy(model["cost"])
        if not isinstance(value.get("tiers", []), list) or any(
                not isinstance(row, dict) or not isinstance(row.get("tier"), dict)
                for row in value.get("tiers", [])):
            continue
        value["tiers"] = [dict(row, above=(row.get("tier") or {}).get("size"))
                          for row in value.get("tiers", [])
                          if isinstance(row, dict)]
        if any((row.get("tier") or {}).get("type") != "context"
               for row in value.get("tiers", [])):
            continue
        if provider == "anthropic":
            for row in [value, *value["tiers"]]:
                if number(row.get("cache_write")):
                    row["cache_write_5m"] = row["cache_write"]
                if number(row.get("input")):
                    row["cache_write_1h"] = row["input"] * 2
        value.update(source=CATALOG_URL, release_date=str(model.get("release_date") or ""))
        rate = clean_rate(value)
        if rate is not None:
            result[name] = rate
    return result


def dollars(cell):
    found = re.search(r"\$([0-9]+(?:\.[0-9]+)?)", cell)
    return float(found[1]) if found else None


def parse_official(text, provider):
    """Read named text-token tables; unrelated audio/image tables are excluded."""
    result, headers, service, section = {}, [], "standard", ""
    for raw in text.splitlines():
        line = raw.strip()
        if provider == "openai":
            label = line.lstrip("# ").lower()
            if label in ("standard", "fast", "flex", "batch", "ultrafast"):
                service = label
            elif label.endswith(" pricing data") and label.split()[0] in ("standard", "fast", "batch", "ultrafast"):
                service = label.split()[0]
        elif line.startswith("## ") or line.startswith("### "):
            section = line.lstrip("# ").lower()
        if not line.startswith("|"):
            continue
        cells = [cell.strip() for cell in line.strip("|").split("|")]
        if "Model" in cells or "Model configuration" in cells:
            headers = ["Model" if cell == "Model configuration" else cell for cell in cells]
            continue
        if not headers or len(cells) != len(headers):
            continue
        if provider == "anthropic":
            if section not in ("model pricing", "fast mode pricing"):
                continue
            names = re.findall(r"Claude ([A-Za-z]+) ([0-9]+(?:\.[0-9]+)?)", cells[0])
            for family, version in names:
                name = "claude-" + family.lower() + "-" + version.replace(".", "-")
                if len(cells) == 6 and section == "model pricing":
                    rate = dict(zip(("input", "cache_write_5m", "cache_write_1h", "cache_read", "output"),
                                    map(dollars, cells[1:])))
                    rate["cache_write"] = rate["cache_write_5m"]
                    threshold = re.search(r"over ([\d,]+) tokens", cells[0])
                    if threshold:
                        if name in result:
                            result[name].setdefault("tiers", []).append(dict(rate, above=int(threshold[1].replace(",", ""))))
                    elif clean_rate(rate):
                        result[name] = dict(rate, source=OFFICIAL_URLS[provider])
                elif len(cells) == 3 and name in result and section == "fast mode pricing":
                    base = result[name]
                    inp, out = map(dollars, cells[1:])
                    if inp is not None and out is not None and base["input"] > 0:
                        factor = inp / base["input"]
                        fast = {key: val * factor for key, val in base.items() if key in FIELDS}
                        fast.update(input=inp, output=out)
                        base.setdefault("service_tiers", {})["fast"] = fast
            continue
        model_cell = cells[headers.index("Model")]
        match = re.match(r"(gpt-[a-z0-9.-]+|o[1-9][a-z0-9.-]*|codex-[a-z0-9.-]+|chat-latest)(?:\s|$)", model_cell)
        if not match or "Modality" in headers or "Use case" in headers:
            continue
        name = match[1]
        columns = {header.lower(): dollars(value) for header, value in zip(headers, cells)}
        short = "short context input" in columns
        prefix = "short context " if short else ""
        rate = dict(input=columns.get(prefix + "input"), output=columns.get(prefix + "output"),
                    cache_read=columns.get(prefix + "cached input"))
        if short:
            rate.update(cache_write=columns.get(prefix + "30m write"),
                        cache_write_12h=columns.get(prefix + "12h write"))
            long_rate = dict(input=columns.get("long context input"), output=columns.get("long context output"),
                             cache_read=columns.get("long context cached input"),
                             cache_write=columns.get("long context 30m write"),
                             cache_write_12h=columns.get("long context 12h write"))
            if clean_rate(long_rate):
                rate["tiers"] = [dict(long_rate, above=272000)]
            elif clean_rate(rate):
                # A dash in the long-context tier means unavailable, not base price.
                rate["tiers"] = [dict(input=0, output=0, above=272000, unsupported=True)]
        cleaned = clean_rate(rate)
        if cleaned is None:
            continue
        if service == "standard":
            previous = result.get(name, {})
            result[name] = dict(cleaned, source=OFFICIAL_URLS[provider])
            if previous.get("service_tiers"):
                result[name]["service_tiers"] = previous["service_tiers"]
        elif name in result:
            result[name].setdefault("service_tiers", {})[service] = cleaned
    if provider == "anthropic" and ("50 percent discount" in text or "50% discount" in text):
        for rate in result.values():
            def discounted(row):
                values = {key: value / 2 for key, value in row.items() if key in FIELDS}
                if row.get("tiers"):
                    values["tiers"] = [dict(discounted(tier), above=tier["above"]) for tier in row["tiers"]]
                return values
            rate.setdefault("service_tiers", {})["batch"] = discounted(rate)
    return clean_models(result)


def read_bundle(provider):
    try:
        raw = json.loads(BUNDLE.read_text())
        if not isinstance(raw, dict) or not isinstance(raw.get("models"), dict) \
                or not isinstance(raw.get("verified_at"), str):
            raise ValueError("invalid bundled price catalog")
        return {"models": clean_models(raw.get("models", {}).get(provider, {})),
                "fetched_at": datetime.fromisoformat(raw["verified_at"].replace("Z", "+00:00")).timestamp(),
                "source": "bundled"}
    except (OSError, ValueError, KeyError, TypeError):
        return {"models": {}, "fetched_at": 0, "source": "unavailable"}


def merge_models(base, incoming, *, preserve=False):
    """Extend frozen ledgers without changing rates already observed that day."""
    result = deepcopy(base)
    for name, rate in incoming.items():
        if name not in result:
            result[name] = deepcopy(rate)
        elif preserve:
            for field, value in rate.items():
                if field == "service_tiers":
                    for tier, row in value.items():
                        result[name].setdefault(field, {}).setdefault(tier, deepcopy(row))
                else:
                    result[name].setdefault(field, deepcopy(value))
        else:
            services = dict(result[name].get("service_tiers", {}), **rate.get("service_tiers", {}))
            result[name] = deepcopy(rate)
            if services:
                result[name]["service_tiers"] = services
    return result


def resolve_rate(prices, model, context_input=0, service_tier="standard"):
    name = str(model).strip().lower()
    for prefix in ("openai/", "anthropic/", "openai:", "anthropic:"):
        if name.startswith(prefix):
            name = name[len(prefix):]
            break
    rate = clean_rate(prices.get(name)) if isinstance(prices, dict) else None
    if rate is None:
        return None
    service = {"default": "standard", "auto": "standard", "priority": "fast"}.get(service_tier, service_tier)
    if service != "standard":
        rate = (rate.get("service_tiers") or {}).get(service)
        if not isinstance(rate, dict):
            return None
    for tier in rate.get("tiers", []):
        if context_input > tier["above"]:
            if tier.get("unsupported"):
                return None
            rate = tier
    return rate


def download(url):
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, headers, newurl):
            return None
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    request = urllib.request.Request(url, headers={"Accept": "text/plain, application/json",
                                                 "User-Agent": "ai-usage-widget/1"})
    with opener.open(request, timeout=5) as response:
        raw = response.read(MAX_BYTES + 1)
    if len(raw) > MAX_BYTES:
        raise ValueError("oversized public price catalog")
    return raw.decode("utf-8")


def atomic_write(path, data):
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    fd, name = tempfile.mkstemp(prefix="." + path.name + ".", dir=path.parent)
    try:
        with os.fdopen(fd, "w") as handle:
            json.dump(data, handle, separators=(",", ":"), allow_nan=False)
            handle.write("\n")
        os.replace(name, path)
    finally:
        if os.path.exists(name):
            os.unlink(name)


def load_prices(provider, *, state_dir=None, now=None, disabled=False, force=False, fallback=None, required=()):
    if provider not in OFFICIAL_URLS:
        raise ValueError("unsupported price provider")
    now = time.time() if now is None else now
    baseline = read_bundle(provider)
    if fallback is not None:
        baseline["models"] = clean_models(fallback)
    root = Path(state_dir or Path(os.environ.get("XDG_STATE_HOME") or Path.home() / ".local/state") / "ai-usage")
    path = root / ("prices-" + provider + ".json")
    cached = {}
    try:
        if not path.is_symlink() and path.stat().st_size <= MAX_BYTES:
            cached = json.loads(path.read_text())
            if not isinstance(cached, dict) or not clean_models(cached.get("models")) \
                    or any(not isinstance(cached.get(key), (int, float))
                           or isinstance(cached.get(key), bool)
                           or cached[key] < 0 or cached[key] > now + REFRESH_SECONDS
                           or not math.isfinite(cached[key])
                           for key in ("fetched_at", "last_attempt")):
                cached = {}
    except (OSError, ValueError, TypeError):
        pass
    result = dict(baseline)
    if cached:
        result.update(cached, models=merge_models(baseline["models"], clean_models(cached["models"])))
    disabled = disabled or os.environ.get("AI_USAGE_DISABLE_PRICE_DOWNLOADS") == "1"
    age = now - result.get("fetched_at", 0)
    missing = any(resolve_rate(result["models"], *request) is None for request in required)
    due = not cached or age >= REFRESH_SECONDS or force or missing
    if not disabled and due and now - cached.get("last_attempt", 0) >= RETRY_SECONDS:
        incoming, official = {}, {}
        with ThreadPoolExecutor(max_workers=2) as pool:
            requests = {"catalog": pool.submit(download, CATALOG_URL),
                        "official": pool.submit(download, OFFICIAL_URLS[provider])}
            for source, task in requests.items():
                try:
                    text = task.result()
                    if source == "catalog":
                        incoming = normalize_catalog(json.loads(text), provider)
                    else:
                        official = parse_official(text, provider)
                except (OSError, ValueError, TypeError, KeyError):
                    continue
        result["models"] = merge_models(result["models"], incoming)
        result["models"] = merge_models(result["models"], official)
        if incoming or official:
            result.update(fetched_at=now, source=OFFICIAL_URLS[provider] if official else CATALOG_URL)
        result["last_attempt"] = now
        try:
            atomic_write(path, result)
        except OSError:
            pass
    result["stale"] = now - result.get("fetched_at", 0) >= REFRESH_SECONDS
    result["price_date"] = datetime.fromtimestamp(result.get("fetched_at", 0), timezone.utc).strftime("%Y-%m-%d")
    return result
