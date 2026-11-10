#!/usr/bin/env python3
"""Read Claude's local turn lifecycle and native session registry.

This module never starts Claude, reads authentication, or treats an open app or
a recent transcript as proof of execution. Session registry PIDs are checked
against Linux process start times to reject exited and reused processes.
"""
import json
import os
from pathlib import Path
import time
from datetime import datetime


def _epoch_ms(value):
    if isinstance(value, bool):
        return 0
    if isinstance(value, (int, float)):
        try:
            return max(0, int(value))
        except (ValueError, OverflowError):
            return 0
    if isinstance(value, str):
        try:
            return max(0, int(datetime.fromisoformat(value.replace("Z", "+00:00")).timestamp() * 1000))
        except (ValueError, OverflowError, OSError):
            pass
    return 0


def _text(value, limit=160):
    return " ".join(value.split())[:limit] if isinstance(value, str) else ""


def _interrupted(content):
    # These are native Claude transcript control messages, not arbitrary errors.
    markers = ("[Request interrupted by user]", "[Request interrupted by user for tool use]")
    if isinstance(content, str):
        return content.strip() in markers
    if isinstance(content, list):
        return any(isinstance(block, dict) and (
            _interrupted(block.get("text")) or
            (block.get("type") == "tool_result" and _interrupted(block.get("content"))))
            for block in content)
    return False


def _transition(event):
    kind = event.get("type")
    message = event.get("message")
    message = message if isinstance(message, dict) else {}
    if kind == "user" and not event.get("isMeta") and not event.get("isCompactSummary"):
        if _interrupted(message.get("content")):
            return "interrupted", "user_interruption"
        return "pending", "tool_result" if event.get("toolUseResult") is not None else "user_message"
    if kind == "api-request":
        return "pending", "api_request"
    if kind == "assistant":
        reason = message.get("stop_reason")
        if reason in ("end_turn", "stop_sequence", "max_tokens", "refusal"):
            return "completed", "assistant_" + reason
        if reason in ("tool_use", "pause_turn") or reason is None:
            return "pending", "assistant_" + str(reason or "streaming")
    if kind == "system":
        subtype = event.get("subtype")
        if subtype == "stop_hook_summary" and not event.get("preventedContinuation"):
            return "completed", "stop_hook_summary"
        if subtype in ("turn_end", "session_end"):
            return "completed", subtype
        if subtype in ("turn_interrupted", "turn_cancelled", "turn_aborted"):
            return "interrupted", subtype
    return None


def _process_state(metadata, proc_root):
    """Return alive/dead/unknown without inspecting command lines or secrets."""
    pid = metadata.get("pid")
    if isinstance(pid, bool) or not isinstance(pid, int) or pid <= 0:
        return "unknown"
    domain = metadata.get("pidDomain")
    same_namespace = False
    if isinstance(domain, str) and ":pid:[" in domain:
        parts = domain.split(":")
        if (len(parts) == 4 and parts[0] == "linux" and len(parts[1]) == 32
                and all(char in "0123456789abcdef" for char in parts[1].lower())):
            try:
                machine_id = Path("/etc/machine-id").read_text().strip().lower()
            except OSError:
                return "unknown"
            if machine_id != parts[1].lower():
                return "unknown"
        try:
            same_namespace = domain[domain.rfind(":pid:[") + 1:] == os.readlink(proc_root / "self/ns/pid")
        except OSError:
            return "unknown"
        if not same_namespace:
            return "unknown"
    # pidDomain contains a machine ID and PID namespace, but no boot ID.
    # Reject registry files left by an earlier boot even if a PID and its
    # process-start tick happen to be reused after restarting the machine.
    try:
        boot_ms = next((int(line.split()[1]) * 1000
            for line in (proc_root / "stat").read_text().splitlines()
            if line.startswith("btime ")), 0)
    except (OSError, ValueError, IndexError):
        boot_ms = 0
    started_at = _epoch_ms(metadata.get("startedAt"))
    if boot_ms and started_at and started_at + 60000 < boot_ms:
        return "dead"
    try:
        stat = (proc_root / str(pid) / "stat").read_text()
    except FileNotFoundError:
        return "dead" if same_namespace else "unknown"
    except OSError:
        return "unknown"
    # comm (field 2) may contain spaces and parentheses. Field 22 is index 19
    # after its final closing parenthesis.
    if ")" not in stat:
        return "unknown"
    fields = stat[stat.rfind(")") + 1:].split()
    if len(fields) < 20:
        return "unknown"
    if fields[0] in ("Z", "X", "x"):
        return "dead"
    started = metadata.get("procStart")
    if not isinstance(started, (str, int)) or isinstance(started, bool):
        return "unknown"
    return "alive" if str(started) == fields[19] else "dead"


class TranscriptState:
    def __init__(self, now_ms):
        self.now_ms = now_ms
        self.sessions = {}
        self.transitions = {}
        self.titles = {}

    def session(self, sid):
        return self.sessions.setdefault(sid, {"id": sid, "title": "Claude session", "directory": "",
            "started": 0, "updated": 0, "running": False,
            "activityState": "unknown", "activityEvidence": "no_lifecycle"})

    def observe(self, event, path, index):
        if not isinstance(event, dict) or event.get("isSidechain"):
            return
        sid = str(event.get("sessionId") or Path(path).stem)
        session = self.session(sid)
        stamp = _epoch_ms(event.get("timestamp"))
        if stamp and stamp <= self.now_ms + 60000:
            session["started"] = min(session["started"] or stamp, stamp)
            session["updated"] = max(session["updated"], stamp)
            if event.get("cwd"):
                session["directory"] = str(event["cwd"])
            transition = _transition(event)
            if transition:
                # Timestamp first makes copied/older transcript files
                # unable to resurrect a completed turn.
                order = (stamp, index, str(path))
                if order >= self.transitions.get(sid, ((0, 0, ""), None))[0]:
                    self.transitions[sid] = (order, transition)
        if event.get("type") == "custom-title":
            title = _text(event.get("customTitle"))
            if title:
                self.titles[sid] = title


def collect_sessions(config, now_ms=None, proc_root=Path("/proc"), transcript_state=None):
    """Return session metadata keyed by ID, suitable for merging with usage.

    ``running`` requires a registered, verified live Claude process and either
    an explicit working status or an unfinished transcript turn. Native idle
    and waiting states override older unfinished transcripts. A newer transcript
    transition overrides an older native status. Inaccessible processes,
    unknown registry formats, queued prompts, and old history remain unknown or
    inactive. ``activityState`` and ``activityEvidence`` explain the result.
    Sessions appear before their first assistant usage record.
    """
    config, proc_root = Path(config), Path(proc_root)
    now_ms = int(time.time() * 1000) if now_ms is None else now_ms
    state = transcript_state if transcript_state is not None else TranscriptState(now_ms)
    if transcript_state is None:
        for path in sorted((config / "projects").rglob("*.jsonl")):
            try:
                handle = path.open(errors="replace")
            except OSError:
                continue
            with handle:
                for index, line in enumerate(handle):
                    try:
                        event = json.loads(line)
                    except ValueError:
                        continue  # Claude may still be writing the final JSON line.
                    state.observe(event, path, index)
    sessions, transitions, titles = state.sessions, state.transitions, state.titles
    get_session = state.session

    metadata_by_session = {}
    for path in sorted((config / "sessions").glob("*.json")):
        try:
            metadata = json.loads(path.read_text())
        except (OSError, ValueError):
            continue
        if not isinstance(metadata, dict) or not metadata.get("sessionId"):
            continue
        sid = str(metadata["sessionId"])
        stamp = _epoch_ms(metadata.get("updatedAt"))
        if stamp > now_ms + 60000:
            continue
        if stamp >= metadata_by_session.get(sid, (0, {}))[0]:
            metadata_by_session[sid] = (stamp, metadata)

    for sid in set(sessions) | set(metadata_by_session):
        session = get_session(sid)
        transition = transitions.get(sid, ((0, 0, ""), ("unknown", "no_lifecycle")))
        lifecycle_time = transition[0][0]
        lifecycle, evidence = transition[1]
        session.update(activityState=lifecycle, activityEvidence=evidence)
        metadata = metadata_by_session.get(sid, (0, {}))[1]
        process = _process_state(metadata, proc_root) if metadata else "unknown"
        session["processState"] = process
        if metadata:
            started = _epoch_ms(metadata.get("startedAt"))
            updated = _epoch_ms(metadata.get("updatedAt"))
            if started and started <= now_ms + 60000:
                session["started"] = min(session["started"] or started, started)
            session["updated"] = max(session["updated"], updated)
            session["directory"] = session["directory"] or str(metadata.get("cwd") or "")
            # updatedAt may change for a name or other registry metadata while
            # its working/idle state remains old. Prefer the state's own clock.
            status_time = _epoch_ms(metadata.get("statusUpdatedAt")) or updated
            if status_time > now_ms + 60000:
                status_time = 0
            status = metadata.get("status") if status_time >= lifecycle_time else None
            if status in ("idle", "waiting", "waiting_for_permission", "waiting_for_input"):
                session.update(activityState="idle" if status == "idle" else "waiting",
                    activityEvidence="native_" + status)
            elif process == "dead":
                session.update(activityState="inactive", activityEvidence="process_exited")
            elif process == "alive" and (status in ("working", "running", "busy") or lifecycle == "pending"):
                session.update(running=True, activityState="running",
                    activityEvidence="native_" + status if status in ("working", "running", "busy")
                    else "live_process_unfinished_turn")
            elif lifecycle == "pending" or status in ("working", "running", "busy"):
                session.update(activityState="unknown", activityEvidence="process_unverified")
            if not titles.get(sid):
                titles[sid] = _text(metadata.get("name"))
        elif lifecycle == "pending":
            session.update(activityState="unknown", activityEvidence="unfinished_turn_unverified")
        session["title"] = titles.get(sid) or "Claude session"
    return sessions
