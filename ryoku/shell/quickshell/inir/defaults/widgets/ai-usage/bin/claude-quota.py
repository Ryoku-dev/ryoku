"""Read Claude account quotas without changing authentication or model state.

Desktop writes server percentages to plan-usage-history.json. Its version 2
samples use epoch milliseconds and fh/sd for five_hour/seven_day. These are
account limits, not token counters. Account metadata must bind the Desktop
account, CLI organization and history sample before any sample is published.
The complete cached usage response also supplies exact reset timestamps.
The installed Desktop polls every 5 or 15 minutes and throttles history samples
to 4.5 minutes. Samples older than 20 minutes are last-known data only.
"""
import json
import hashlib
import math
import os
import re
import struct
from datetime import datetime
from email.utils import parsedate_to_datetime
from pathlib import Path
import time
import urllib.error
import urllib.request
import zlib


MAX_BYTES = 1024 * 1024
FRESH_SECONDS = 20 * 60
OAUTH_URL = "https://api.anthropic.com/api/oauth/usage"


def _number(value):
    try:
        return isinstance(value, (int, float)) and not isinstance(value, bool) and math.isfinite(value)
    except OverflowError:
        return False


def _read_json(path):
    try:
        with Path(path).open("rb") as handle:
            raw = handle.read(MAX_BYTES + 1)
        return json.loads(raw) if len(raw) <= MAX_BYTES else None
    except (OSError, ValueError, UnicodeError):
        return None


def _empty(message="Account usage unavailable"):
    return {"_windows": [], "_plan": "", "5h-utilization": "", "7d-utilization": "",
            "5h-reset": "", "7d-reset": "", "_quota": {
                "available": False, "source": "unavailable", "observed_at": None,
                "age_seconds": None, "stale": False, "message": message}}


def _plan(account):
    value = str(account.get("subscriptionType") or account.get("organizationType") or "").lower()
    return {"claude_pro": "pro", "pro": "pro", "claude_max": "max", "max": "max",
            "team": "team", "enterprise": "enterprise", "free": "free"}.get(value, "")


def _window(key, value, reset=0, source="desktop-history"):
    if not _number(value) or not 0 <= value <= 100:
        return None
    minutes, label = (300, "5h") if key == "five_hour" else (10080, "7d")
    return {"key": key, "minutes": minutes, "label": label,
            "utilization": value / 100, "reset": reset, "source": source}


def _publish(windows, plan, source, observed, now):
    result = _empty()
    age = max(0, int(now - observed))
    stale = age > FRESH_SECONDS or any(0 < w["reset"] <= now for w in windows)
    result["_plan"] = plan
    result["_quota"] = {"available": bool(windows), "source": source,
                        "observed_at": int(observed), "age_seconds": age,
                        "stale": stale, "message": "Last captured account usage" if stale
                        else "Account usage from Claude Desktop" if source.startswith("desktop-")
                        else "Account usage from Claude OAuth"}
    if stale:
        # Stale captures never enter the current utilization or status fields.
        result["_quota"]["last_known_windows"] = windows
        return result
    result["_windows"] = windows
    for window in windows:
        prefix = "5h" if window["minutes"] == 300 else "7d"
        result[prefix + "-utilization"] = f"{window['utilization']:.4f}"
        result[prefix + "-reset"] = str(window["reset"]) if window["reset"] else ""
    return result


def _reset_at(value, observed, minutes):
    try:
        stamp = datetime.fromisoformat(str(value or "").replace("Z", "+00:00"))
        reset = stamp.timestamp()
        if stamp.tzinfo is not None and observed < reset <= observed + minutes * 60 + 60:
            return int(reset)
    except (ValueError, OverflowError, OSError):
        pass
    return 0


def _cache_streams(raw, key):
    """Read Chromium Simple Cache v5, rejecting partial/corrupt/wrong-key data.

    Format: chromium/src/net/disk_cache/simple/simple_entry_format.h.
    Only the exact account usage URL's hashed filename is read, never cookies.
    """
    if len(raw) < 96 or len(raw) > MAX_BYTES:
        return None
    magic, version, size, _, padding = struct.unpack_from("<QIIII", raw)
    if magic != 0xfcfb6d1ba7725c30 or version != 5 or padding or size != len(key):
        return None
    if raw[24:24+size] != key:
        return None
    end, flags, crc, header_size, padding = struct.unpack_from("<QIIII", raw, len(raw)-24)
    if end != 0xf4fa6f45970d41d8 or flags & ~3 or padding:
        return None
    hash_size = 32 if flags & 2 else 0
    header_end = len(raw) - 24 - hash_size
    header_start = header_end - header_size
    body_start = 24 + size
    if header_start < body_start + 24:
        return None
    if hash_size and raw[header_end:header_end+32] != hashlib.sha256(key).digest():
        return None
    headers = raw[header_start:header_end]
    if flags & 1 and zlib.crc32(headers) != crc:
        return None
    end, flags, crc, _, padding = struct.unpack_from("<QIIII", raw, header_start-24)
    if end != 0xf4fa6f45970d41d8 or flags & ~1 or padding:
        return None
    body = raw[body_start:header_start-24]
    if flags & 1 and zlib.crc32(body) != crc:
        return None
    return body, headers


def _cache_body(body, encoding):
    if encoding == "zstd":
        try:
            from compression import zstd
        except ImportError:
            return None
        decoder = zstd.ZstdDecompressor()
        try:
            body = decoder.decompress(body, max_length=MAX_BYTES+1)
        except zstd.ZstdError:
            return None
        if not decoder.eof or decoder.unused_data:
            return None
    elif encoding in ("gzip", "deflate"):
        decoder = zlib.decompressobj(31 if encoding == "gzip" else 15)
        try:
            body = decoder.decompress(body, MAX_BYTES+1)
        except zlib.error:
            return None
        if not decoder.eof or decoder.unused_data:
            return None
    elif encoding not in ("", "identity"):
        return None
    if len(body) > MAX_BYTES:
        return None
    try:
        return json.loads(body)
    except (ValueError, UnicodeError):
        return None


def desktop_cached_quota(desktop, org, plan, now):
    """Use only complete HTTP 200 usage responses for the bound organization."""
    latest = None
    url = "https://claude.ai/api/organizations/" + org + "/usage"
    for query in ("", "?skip_spend=1", "?cedar_ember=1&skip_spend=1", "?cedar_ember=1"):
        key = ("1/0/" + url + query).encode()
        # Chromium simple_util.cc hashes the key's first SHA1 uint64 in LE.
        name = hashlib.sha1(key).digest()[:8][::-1].hex() + "_0"
        path = desktop / "Cache/Cache_Data" / name
        try:
            if path.is_symlink():
                continue
            with path.open("rb") as handle:
                streams = _cache_streams(handle.read(MAX_BYTES+1), key)
        except OSError:
            continue
        if streams is None:
            continue
        body, headers = streams
        status = re.search(rb"HTTP/1\.[01] 200(?: [^\x00]*)?\x00", headers)
        if status is None:
            continue
        fields = {}
        for line in headers[status.end():].split(b"\x00\x00", 1)[0].split(b"\x00"):
            name, sep, value = line.partition(b":")
            if sep and name.lower() in (b"date", b"content-encoding"):
                fields.setdefault(name.lower(), []).append(value.strip())
        if len(fields.get(b"date", [])) != 1 or len(fields.get(b"content-encoding", [])) > 1:
            continue
        try:
            observed = parsedate_to_datetime(fields[b"date"][0].decode("ascii"))
            if observed.tzinfo is None:
                continue
            observed = observed.timestamp()
            encoding = fields.get(b"content-encoding", [b""])[0].decode("ascii").lower()
        except (ValueError, TypeError, UnicodeError, OverflowError, OSError):
            continue
        if not 0 < observed <= now:
            continue
        data = _cache_body(body, encoding)
        if not isinstance(data, dict):
            continue
        windows = []
        for name, minutes in (("five_hour", 300), ("seven_day", 10080)):
            row = data.get(name)
            if isinstance(row, dict):
                window = _window(name, row.get("utilization"),
                    _reset_at(row.get("resets_at"), observed, minutes), "desktop-cache")
                if window:
                    windows.append(window)
        if windows and (latest is None or observed > latest["_quota"]["observed_at"]):
            latest = _publish(windows, plan, "desktop-cache", observed, now)
    return latest


def desktop_quota(config, desktop, now):
    """Use only version 2 samples bound to the currently selected account."""
    metadata = _read_json(desktop / "config.json")
    # Claude uses ~/.claude.json for its default profile; alternate config
    # directories keep their own metadata. Do not borrow the default account.
    default = Path.home() / ".claude"
    account_path = config.parent / ".claude.json" if config == default else config / ".claude.json"
    profile = _read_json(account_path)
    account = profile.get("oauthAccount") if isinstance(profile, dict) else None
    if not isinstance(metadata, dict) or not isinstance(account, dict):
        return _empty("Claude Desktop account metadata unavailable")
    aid, org = account.get("accountUuid"), account.get("organizationUuid")
    if not isinstance(aid, str) or not aid or not isinstance(org, str) or not org:
        return _empty("Claude Desktop account metadata unavailable")
    if metadata.get("lastKnownAccountUuid") != aid:
        return _empty("Claude Desktop account changed; usage awaits a matching account")
    response = desktop_cached_quota(desktop, org, _plan(account), now)
    history = _read_json(desktop / "plan-usage-history.json")
    if not isinstance(history, dict) or history.get("version") != 2:
        return response or _empty("Claude Desktop quota history unavailable")
    samples = history.get("samples")
    if not isinstance(samples, list):
        return response or _empty("Claude Desktop quota history unavailable")
    candidates = [row for row in samples if isinstance(row, dict) and row.get("org") == org
                  and _number(row.get("t")) and 0 < row["t"] / 1000 <= now]
    if not candidates:
        return response or _empty("No captured usage for the selected Claude account")
    sample = max(candidates, key=lambda row: row["t"])
    usage = sample.get("u")
    if not isinstance(usage, dict):
        return response or _empty("Claude Desktop quota sample is malformed")
    windows = [window for key, short in (("five_hour", "fh"), ("seven_day", "sd"))
               if (window := _window(key, usage.get(short))) is not None]
    if not windows:
        return response or _empty("Claude Desktop quota sample has no measured limits")
    # History's client timestamp follows the same response's HTTP Date by a
    # small rounding/network delay. Do not replace newer history with old data.
    if response and response["_quota"]["observed_at"] >= sample["t"] / 1000 - 5:
        return response
    return _publish(windows, _plan(account), "desktop-history", sample["t"] / 1000, now)


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def _fetch_oauth(token):
    request = urllib.request.Request(OAUTH_URL, headers={
        "Authorization": "Bearer " + token, "anthropic-beta": "oauth-2025-04-20",
        "Accept": "application/json", "User-Agent": "ai-usage-widget/claude-quota"})
    # Never forward a credential to a redirect destination or persist responses.
    with urllib.request.build_opener(_NoRedirect()).open(request, timeout=4) as response:
        raw = response.read(MAX_BYTES + 1)
    if len(raw) > MAX_BYTES:
        raise ValueError("response too large")
    return json.loads(raw)


def _previous_oauth(previous, fingerprint, now):
    if not isinstance(previous, dict):
        return None
    previous = previous.get("_quota_oauth_probe", previous)
    if not isinstance(previous, dict):
        return None
    meta = previous.get("_quota")
    if not isinstance(meta, dict) or meta.get("source") != "oauth" or meta.get(
            "credential_fingerprint") != fingerprint:
        return None
    observed = meta.get("observed_at")
    raw_windows = previous.get("_windows") or meta.get("last_known_windows") or []
    windows = []
    for row in raw_windows if isinstance(raw_windows, list) else []:
        if not isinstance(row, dict) or row.get("minutes") not in (300, 10080):
            continue
        value = row.get("utilization")
        if not _number(value) or not 0 <= value <= 1:
            continue
        reset = row.get("reset")
        reset = int(reset) if _number(reset) and _number(observed) and observed < reset <= observed + 8*86400 else 0
        window = _window("five_hour" if row["minutes"] == 300 else "seven_day",
                         value * 100, reset, "oauth")
        windows.append(window)
    if _number(observed) and 0 < observed <= now and windows:
        cached = _publish(windows, str(previous.get("_plan") or ""), "oauth", observed, now)
        if meta.get("stale"):
            cached["_windows"] = []
            for prefix in ("5h", "7d"):
                cached[prefix + "-utilization"] = cached[prefix + "-reset"] = ""
            cached["_quota"].update(stale=True, last_known_windows=windows)
    else:
        cached = _empty(str(meta.get("message") or "Claude OAuth usage unavailable"))
    cached["_quota"].update(source="oauth", credential_fingerprint=fingerprint,
                            checked_at=meta.get("checked_at"))
    return cached


def oauth_quota(config, now, previous=None):
    credentials = _read_json(config / ".credentials.json")
    oauth = credentials.get("claudeAiOauth") if isinstance(credentials, dict) else None
    if not isinstance(oauth, dict):
        return _empty("No readable Claude OAuth usage credentials")
    token = oauth.get("accessToken")
    if not isinstance(token, str) or not token or any(c in token for c in "\r\n"):
        return _empty("No readable Claude OAuth usage credentials")
    expires = oauth.get("expiresAt")
    if _number(expires) and expires / 1000 <= now:
        return _empty("Claude OAuth credentials expired; authentication remains unchanged")
    scopes = oauth.get("scopes")
    if isinstance(scopes, list) and "user:profile" not in scopes:
        return _empty("Claude OAuth token lacks account usage access")
    fingerprint = hashlib.sha256(token.encode()).hexdigest()
    cached = _previous_oauth(previous, fingerprint, now)
    checked = cached["_quota"].get("checked_at") if cached else None
    crossed_reset = cached and any(checked < w.get("reset", 0) <= now
        for w in cached["_quota"].get("last_known_windows", [])) if _number(checked) else False
    if _number(checked) and 0 <= now - checked < 300 and not crossed_reset:
        return cached

    def failed(message):
        result = cached or _empty(message)
        windows = result.get("_windows") or result["_quota"].get("last_known_windows") or []
        result["_windows"] = []
        for prefix in ("5h", "7d"):
            result[prefix + "-utilization"] = result[prefix + "-reset"] = ""
        result["_quota"].update(source="oauth", credential_fingerprint=fingerprint,
                                checked_at=int(now), message=message, stale=bool(windows))
        if windows:
            result["_quota"]["last_known_windows"] = windows
        return result

    try:
        data = _fetch_oauth(token)
    except urllib.error.HTTPError as error:
        return failed("Claude OAuth usage unavailable (HTTP " + str(error.code) + ")")
    except (OSError, ValueError, urllib.error.URLError):
        return failed("Claude OAuth usage could not be read")
    if not isinstance(data, dict):
        return failed("Claude OAuth usage response is malformed")
    windows = []
    for key in ("five_hour", "seven_day"):
        row = data.get(key)
        if not isinstance(row, dict):
            continue
        reset = 0
        try:
            stamp = datetime.fromisoformat(str(row.get("resets_at") or "").replace("Z", "+00:00"))
            if stamp.tzinfo is not None and now < stamp.timestamp() <= now + 8 * 86400:
                reset = int(stamp.timestamp())
        except (ValueError, OverflowError, OSError):
            pass
        window = _window(key, row.get("utilization"), reset, "oauth")
        if window:
            windows.append(window)
    if not windows:
        return failed("Claude OAuth response has no measured limits")
    result = _publish(windows, _plan(oauth), "oauth", now, now)
    result["_quota"].update(credential_fingerprint=fingerprint, checked_at=int(now))
    return result


def collect_quota(config, *, desktop=None, now=None, allow_network=True, previous=None):
    """Return current measured windows, or marked last-known/unavailable data.

    This does not decrypt Desktop cookies, refresh credentials, launch Claude,
    infer quotas from tokens, or read legacy widget caches. Callers can pass
    allow_network=False for deterministic local-only collection.
    """
    config = Path(config).expanduser().resolve()
    desktop = Path(desktop) if desktop is not None else Path(
        os.environ.get("XDG_CONFIG_HOME") or Path.home() / ".config") / "Claude"
    now = time.time() if now is None else now
    cached = desktop_quota(config, desktop, now)
    if cached["_windows"]:
        return cached
    if allow_network:
        live = oauth_quota(config, now, previous)
        if live["_windows"]:
            return live
        if not cached["_quota"]["available"]:
            return live
        # Keep OAuth cooldown state even when the older Desktop capture is the
        # visible result. No raw credential or account identifier is retained.
        cached["_quota_oauth_probe"] = live
    return cached
