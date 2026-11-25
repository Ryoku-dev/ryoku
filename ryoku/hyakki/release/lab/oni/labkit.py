#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Helpers for the Oni lab scripts: compositor state, the surface ledger,
process counters, crops, measure.sh and phase8.sh. Lab processes are only
ever started through the lab's shell scripts, never from here directly."""
import datetime
import json
import math
import os
import socket
import signal
import subprocess
import sys
import threading
import time
from pathlib import Path

HERE = Path(__file__).resolve().parent
CLK_TCK = os.sysconf("SC_CLK_TCK")
POSE_CHANNELS = ("scale", "squash", "leanX", "leanY", "unravel", "straighten", "tempo", "bright")
STATE_ORDER = ("idle", "listening", "thinking", "reading", "speaking", "error")
ACCENT_STATES = ("listening", "thinking", "speaking", "error")
# Ledger kinds that are something happening to the character or its strand,
# as opposed to its clock (step, arm, stop) or its pixels (commit).
EVENT_EVS = {"start", "frame", "state", "gesture", "reset", "hide", "show", "output",
             "drag", "drop", "place", "dormant", "redial", "exit",
             "strand-target", "strand-retract", "strand-arrive", "strand-skip", "bubble", "sound",
             "perch", "detach", "highlight", "overlay", "strand-reframe", "strand-reperch"}
# Proposed in the Phase 8 scoping report; the owner ratifies them. The real
# numbers are recorded either way.
THRESHOLDS = {"commits": 0, "arms": 0, "cpu_percent_max": 0.05, "wakeups_per_s_max": 2.0}
SCENARIOS = ("motion", "settled", "asleep", "covered", "hidden", "off", "strand-held", "strand-retracted",
             "perched", "detached")
# compositor and capture programs the daemon may run only while a perch is
# out (the tracker's event stream or poll, a capture); none may be left after
COMPOSITOR_CHILDREN = ("niri", "hyprctl", "grim", "ryoku")
# The native Nix launcher execs its .oni-wrapped ELF.
ONI_COMMS = ("oni", ".oni-wrapped")
# oni-surface's audio players; aplay must never appear
PLAYERS = ("pw-cat", "paplay", "aplay")


class UserSocket:
    """A real CLI connection, kept across disclosure/apply/undo.
    Never claims app identity or gains permissions absent from the CLI."""
    def __init__(self, lab):
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.settimeout(120)
        self.sock.connect(str(Path(lab) / "rt/hyakki/oni.sock"))
        self.reader = self.sock.makefile("rb")
        self.seq = 0
        self.call("hello", {"client":"cli", "version":1, "pid":os.getpid()})

    def call(self, method, params):
        self.seq += 1
        self.sock.sendall(json.dumps({"id":self.seq, "method":method, "params":params}).encode()+b"\n")
        while True:
            line = self.reader.readline((16 << 20) + 1)
            if not line or len(line) > 16 << 20:
                raise RuntimeError("CLI socket closed or frame exceeded protocol bound")
            frame = json.loads(line)
            if frame.get("id") != self.seq: continue
            if frame.get("error"): raise RuntimeError(json.dumps(frame["error"]))
            return frame.get("result") or {}

    def close(self):
        self.reader.close()
        self.sock.close()


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat(timespec="seconds")


def lab_env(lab):
    values = {}
    for line in (Path(lab) / "lab.env").read_text().splitlines():
        key, _, value = line.partition("=")
        if key and value.startswith("'") and value.endswith("'"):
            values[key] = value[1:-1]
    return values


# --- compositor state -------------------------------------------------------

def normalize_state(wm, outs, wins, layers):
    outputs, windows, surfaces = [], [], None
    if wm == "niri":
        for name, o in sorted(outs.items()):
            lg = o.get("logical") or {}
            outputs.append({"name": name, "x": lg.get("x", 0), "y": lg.get("y", 0),
                            "width": lg.get("width", 0), "height": lg.get("height", 0),
                            "scale": lg.get("scale"), "enabled": bool(lg)})
        for w in wins:
            lay = w.get("layout") or {}
            # the tile's place in the workspace view plus the window's offset
            # in its tile, output-local; global once the output's x, y is added
            tile, off = lay.get("tile_pos_in_workspace_view"), lay.get("window_offset_in_tile")
            # niri's windows carry no output: the lab's nested niri has one
            out = next((o for o in outputs if o["name"] == w.get("output")), None) or \
                (outputs[0] if len(outputs) == 1 else None)
            pos = box = None
            size = lay.get("window_size")
            if tile is not None and off is not None and out is not None:
                local = [tile[0] + off[0], tile[1] + off[1]]
                pos = [out["x"] + local[0], out["y"] + local[1]]
                box = [local[0], local[1], *(size or [0, 0])]
            windows.append({"id": w.get("id"), "app_id": w.get("app_id"), "title": w.get("title"),
                            "size": size, "pos": pos, "box": box, "output": out and out["name"],
                            "column": lay.get("pos_in_scrolling_layout"), "offset": off,
                            "floating": w.get("is_floating"), "focused": w.get("is_focused"),
                            "pid": w.get("pid"), "fullscreen": w.get("is_fullscreen")})
        if isinstance(layers, list):
            surfaces = [{"namespace": s.get("namespace"), "output": s.get("output"),
                         "layer": str(s.get("layer") or "").lower(), "rect": None} for s in layers]
    else:
        for m in outs:
            scale = m.get("scale") or 1
            outputs.append({"name": m["name"], "x": m.get("x", 0), "y": m.get("y", 0),
                            "width": round(m["width"] / scale), "height": round(m["height"] / scale),
                            "scale": scale, "enabled": not m.get("disabled", False), "id": m.get("id")})
        for c in wins:
            mon = next((o for o in outputs if o.get("id") == c.get("monitor")), None)
            at, size = c.get("at"), c.get("size")
            box = [at[0] - mon["x"], at[1] - mon["y"], *size] if mon and at and size else None
            windows.append({"id": c.get("address"), "app_id": c.get("class"), "title": c.get("title"),
                            "size": size, "pos": at, "box": box, "monitor": c.get("monitor"),
                            "output": mon and mon["name"], "floating": c.get("floating"),
                            "focused": c.get("focusHistoryID") == 0, "pid": c.get("pid"),
                            # 1 is maximised, 2 fullscreen (3 both): only 2 covers the output
                            "fullscreen": bool((c.get("fullscreen") or 0) & 2)})
        if isinstance(layers, dict):
            names = {0: "background", 1: "bottom", 2: "top", 3: "overlay"}
            surfaces = []
            for output, mon in sorted(layers.items()):
                for level, items in sorted((mon.get("levels") or {}).items()):
                    for s in items:
                        surfaces.append({"namespace": s.get("namespace"), "output": output,
                                         "layer": names.get(int(level), level),
                                         "rect": [s.get("x"), s.get("y"), s.get("w"), s.get("h")]})
    return {"wm": wm, "outputs": outputs, "windows": windows, "layers": surfaces}


def extent(state):
    live = [o for o in state["outputs"] if o["enabled"] and o["width"] > 0 and o["height"] > 0]
    if not live:
        return 0, 0
    return max(o["x"] + o["width"] for o in live), max(o["y"] + o["height"] for o in live)


def window_fullscreen(state, app_id):
    sizes = {(o["width"], o["height"]) for o in state["outputs"] if o["enabled"]}
    for w in state["windows"]:
        if w.get("app_id") != app_id:
            continue
        if w.get("fullscreen") is not None:
            return bool(w["fullscreen"])
        size = w.get("size")
        return bool(size) and tuple(size) in sizes
    raise ValueError(f"no {app_id} window")


# --- ledger -----------------------------------------------------------------

class Ledger:
    """oni-surface's JSONL ledger at a fixed path; a restarted surface starts
    a new file there, which a mark taken before the restart notices. Reads are
    incremental: a running surface writes a line per frame, and re-parsing
    megabytes on every poll made the harness late for short gestures."""

    def __init__(self, path):
        self.path = Path(path)
        self._ino, self._offset, self._lines, self._starts = None, 0, [], []

    def mark(self):
        try:
            st = self.path.stat()
        except FileNotFoundError:
            return (None, 0)
        return (st.st_ino, st.st_size)

    def _refresh(self):
        try:
            with self.path.open("rb") as f:
                ino = os.fstat(f.fileno()).st_ino
                if ino != self._ino:
                    self._ino, self._offset, self._lines, self._starts = ino, 0, [], []
                f.seek(self._offset)
                data = f.read()
        except FileNotFoundError:
            self._ino, self._offset, self._lines, self._starts = None, 0, [], []
            return
        end = data.rfind(b"\n")
        if end < 0:
            return
        pos = self._offset
        for raw in data[:end].split(b"\n"):
            try:
                line = json.loads(raw)
            except ValueError:
                line = {"ev": "unparsable", "raw": raw.decode("utf-8", "replace")}
            self._starts.append(pos)
            self._lines.append(line)
            pos += len(raw) + 1
        self._offset += end + 1

    def since(self, mark=(None, 0)):
        self._refresh()
        if self._ino is None:
            return []
        if mark[0] != self._ino:
            return list(self._lines)
        import bisect
        return self._lines[bisect.bisect_left(self._starts, mark[1]):]

    def current(self):
        """The last committed frame, without copying the whole ledger."""
        self._refresh()
        return last_commit(self._lines)

    def wait(self, pred, mark=(None, 0), timeout=10.0):
        deadline = time.monotonic() + timeout
        while True:
            for line in self.since(mark):
                if pred(line):
                    return line
            if time.monotonic() >= deadline:
                return None
            time.sleep(0.1)

    def quiet(self, seconds, timeout):
        """Wait until no commit has been written for SECONDS; return the
        seconds waited, or None on timeout."""
        start = time.monotonic()
        last_count, last_change = -1, start
        while True:
            count = sum(1 for line in self.since() if line.get("ev") == "commit")
            t = time.monotonic()
            if count != last_count:
                last_count, last_change = count, t
            elif t - last_change >= seconds:
                return round(t - start, 2)
            if t - start >= timeout:
                return None
            time.sleep(0.2)


def last_commit(lines):
    """The character's last committed frame; strand commits carry a part."""
    for line in reversed(lines):
        if line.get("ev") == "commit" and line.get("rect") and not line.get("part"):
            return line
    return None


def perched_window(line):
    """The window a ledger perch line attached to; None for a refusal."""
    if line.get("ev") != "perch" or (line.get("reason") or "") not in ("", "move", "reattach"):
        return None
    return (line.get("perch") or {}).get("window")


def summary(lines):
    """The character's situation after these ledger lines, and its strand's:
    the phase of the last strand commit, or home once the strand unmapped;
    the window it is perched on, if any."""
    out = {"state": None, "visible": None, "reason": None, "exited": False, "output": None,
           "strand": None, "strand_seq": None, "perch": None}
    for line in lines:
        ev = line.get("ev")
        if line.get("part") == "strand":
            if ev == "commit" and line.get("strand"):
                out["strand"], out["strand_seq"] = line["strand"].get("phase"), line["strand"].get("seq")
            elif ev == "hide":
                out["strand"] = "home"
            continue
        if line.get("part"):  # the perch chip and the drag highlight
            continue
        if ev == "perch" and perched_window(line) is not None:
            out["perch"] = perched_window(line)
        elif ev == "detach":
            out["perch"] = None
        if ev == "state" or (ev == "commit" and line.get("state")):
            out["state"] = line.get("state", out["state"])
        if ev == "hide":
            out["visible"], out["reason"] = False, line.get("reason")
        elif ev in ("show", "commit"):
            out["visible"], out["reason"] = True, None
        elif ev == "exit":
            out["exited"], out["reason"] = True, line.get("reason")
        if line.get("output"):
            out["output"] = line["output"]
    return out


def match(line, conds):
    return all(str(line.get(k)) == v for k, v in conds.items())


# --- processes --------------------------------------------------------------

def lab_pids(lab_id, comm=None):
    tag = f"/oni-lab-{lab_id}-"
    found = []
    for d in os.listdir("/proc"):
        if not d.isdigit():
            continue
        try:
            cgroup = open(f"/proc/{d}/cgroup").read()
            name = open(f"/proc/{d}/comm").read().strip()
            state = open(f"/proc/{d}/stat").read().rsplit(")", 1)[1].split()[0]
        except OSError:
            continue
        if (tag in cgroup and state != "Z"
                and (comm is None or name == comm or (comm == "oni" and name in ONI_COMMS))):
            found.append(int(d))
    return sorted(found)


def comm_of(pid):
    try:
        return open(f"/proc/{pid}/comm").read().strip()
    except OSError:
        return None


def ppid_of(pid):
    try:
        return int(open(f"/proc/{pid}/stat").read().rsplit(")", 1)[1].split()[1])
    except (OSError, IndexError, ValueError):
        return None


def daemon_children(lab_id):
    """Compositor IPC and capture processes (niri msg, hyprctl, grim, ryoku)
    that the oni daemon itself started, at any depth: the window tracker's
    stream or poll and the capture service. The lab's own lab_niri and
    lab_hyprctl run under systemd-run, never under oni."""
    found = []
    for pid in lab_pids(lab_id):
        name = comm_of(pid)
        if name not in COMPOSITOR_CHILDREN:
            continue
        up, hops = ppid_of(pid), 0
        while up and up > 1 and hops < 8:
            if comm_of(up) in ONI_COMMS:
                found.append({"pid": pid, "comm": name, "daemon": up})
                break
            up, hops = ppid_of(up), hops + 1
    return found


def role_pids(lab_id, role):
    """Processes in the lab scopes whose role starts with ROLE
    (oni-lab-<id>-<role>...)."""
    tag = f"/oni-lab-{lab_id}-{role}"
    found = []
    for pid in lab_pids(lab_id):
        try:
            if tag in open(f"/proc/{pid}/cgroup").read():
                found.append(pid)
        except OSError:
            continue
    return found


def stop_role(lab_id, role, sig):
    signum = {"TERM": signal.SIGTERM, "KILL": signal.SIGKILL}[sig]
    for attempt in (signum, signal.SIGKILL):
        pids = role_pids(lab_id, role)
        if not pids:
            return 0
        for pid in pids:
            try:
                os.kill(pid, attempt)
            except OSError:
                pass
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline and role_pids(lab_id, role):
            time.sleep(0.1)
    return 1 if role_pids(lab_id, role) else 0


def scope_of(pid):
    try:
        for line in open(f"/proc/{pid}/cgroup"):
            path = line.strip().split(":", 2)[2]
            return path.rsplit("/", 1)[-1]
    except OSError:
        return None
    return None


def systemctl_show(unit, prop):
    out = subprocess.run(["systemctl", "--user", "show", "-p", prop, "--value", unit],
                         capture_output=True, text=True, check=False).stdout.strip()
    return out


def scope_cpu_ns(unit):
    value = systemctl_show(unit, "CPUUsageNSec")
    return int(value) if value.isdigit() else None


def counters(pid):
    c = {"pid": pid, "comm": None, "scope": scope_of(pid), "threads": 0,
         "utime_stime_ns": None, "schedstat_ns": 0, "timeslices": 0,
         "voluntary": 0, "nonvoluntary": 0, "leader_voluntary": None,
         "leader_nonvoluntary": None, "pss_kib": None, "pss_error": None}
    try:
        c["comm"] = open(f"/proc/{pid}/comm").read().strip()
        fields = open(f"/proc/{pid}/stat").read().rsplit(")", 1)[1].split()
        c["utime_stime_ns"] = (int(fields[11]) + int(fields[12])) * 1_000_000_000 // CLK_TCK
        for line in open(f"/proc/{pid}/status"):
            if line.startswith("voluntary_ctxt_switches:"):
                c["leader_voluntary"] = int(line.split()[1])
            elif line.startswith("nonvoluntary_ctxt_switches:"):
                c["leader_nonvoluntary"] = int(line.split()[1])
    except (OSError, IndexError, ValueError):
        return None
    for task in os.listdir(f"/proc/{pid}/task") if os.path.isdir(f"/proc/{pid}/task") else []:
        try:
            ns, _, slices = open(f"/proc/{pid}/task/{task}/schedstat").read().split()[:3]
            c["schedstat_ns"] += int(ns)
            c["timeslices"] += int(slices)
            for line in open(f"/proc/{pid}/task/{task}/status"):
                if line.startswith("voluntary_ctxt_switches:"):
                    c["voluntary"] += int(line.split()[1])
                elif line.startswith("nonvoluntary_ctxt_switches:"):
                    c["nonvoluntary"] += int(line.split()[1])
            c["threads"] += 1
        except (OSError, ValueError):
            continue
    try:
        for line in open(f"/proc/{pid}/smaps_rollup"):
            if line.startswith("Pss:"):
                c["pss_kib"] = int(line.split()[1])
    except OSError as error:
        c["pss_error"] = error.strerror
    if c["scope"]:
        c["scope_cpu_ns"] = scope_cpu_ns(c["scope"])
    return c


def stop(lab_id, comm, sig):
    signum = {"TERM": signal.SIGTERM, "KILL": signal.SIGKILL, "INT": signal.SIGINT}[sig]
    for attempt in (signum, signal.SIGKILL):
        pids = lab_pids(lab_id, comm)
        if not pids:
            return 0
        for pid in pids:
            try:
                os.kill(pid, attempt)
            except OSError:
                pass
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline and lab_pids(lab_id, comm):
            time.sleep(0.1)
    left = lab_pids(lab_id, comm)
    if left:
        print(f"labkit: {comm} still running: {left}", file=sys.stderr)
        return 1
    return 0


# --- measurement ------------------------------------------------------------

def run_script(name, *args, capture=True, check=False, timeout=600):
    return subprocess.run([str(HERE / name), *map(str, args)], capture_output=capture, text=True,
                          check=check, timeout=timeout)


def lab_state_now():
    out = run_script("ctl.sh", "state", check=True).stdout
    return json.loads(out)


def global_rect(commit, state):
    x, y, w, h = commit["rect"]
    for o in state["outputs"]:
        if o["name"] == commit.get("output"):
            return [o["x"] + x, o["y"] + y, w, h]
    return [x, y, w, h]


def jsonable(value):
    """numpy scalars and arrays in a report, as plain JSON."""
    if hasattr(value, "tolist"):
        return value.tolist()
    raise TypeError(f"{type(value).__name__} is not JSON serializable")


def centre(rect):
    return rect[0] + rect[2] // 2, rect[1] + rect[3] // 2


class Poker(threading.Thread):
    """The motion control: poke the character every PERIOD seconds."""

    def __init__(self, point, period=2.0):
        super().__init__(daemon=True)
        self.point, self.period, self.done, self.pokes, self.failures = point, period, threading.Event(), 0, 0

    def run(self):
        while not self.done.is_set():
            r = run_script("input.sh", "click", *self.point)
            if r.returncode == 0:
                self.pokes += 1
            else:
                self.failures += 1
            self.done.wait(self.period)


def delta(before, after):
    out = {}
    for key in ("utime_stime_ns", "schedstat_ns", "timeslices", "voluntary", "nonvoluntary",
                "leader_voluntary", "leader_nonvoluntary", "scope_cpu_ns"):
        a, b = before.get(key), after.get(key)
        out[key] = None if a is None or b is None else b - a
    if out["voluntary"] is not None and out["nonvoluntary"] is not None:
        out["context_switches"] = out["voluntary"] + out["nonvoluntary"]
    return out


def daemon_quiet(lab_id, seconds, timeout):
    """Wait until the oni daemon has not been scheduled for SECONDS; return
    the seconds waited, or None on timeout. No daemon is quiet at once."""
    start = time.monotonic()
    last, since = None, start
    while True:
        pids = lab_pids(lab_id, "oni")
        if not pids:
            return 0.0
        count = sum((counters(p) or {}).get("timeslices", 0) for p in pids)
        t = time.monotonic()
        if count != last:
            last, since = count, t
        elif t - since >= seconds:
            return round(t - start, 2)
        if t - start >= timeout:
            return None
        time.sleep(0.2)


def measure(lab, scenario, out_path, seconds, quiet_s, timeout):
    env = lab_env(lab)
    lab_id = env["LAB_ID"]
    ledger = Ledger(Path(lab) / "ledger/surface.jsonl")
    record = {"scenario": scenario, "lab_id": lab_id, "wm": env.get("LAB_WM"),
              "started_at": now(), "seconds": seconds, "quiet_s": quiet_s,
              "ledger": str(ledger.path), "thresholds": THRESHOLDS,
              "thresholds_status": "proposed in the Phase 8 scoping report; owner to ratify",
              "checks": []}

    def check(name, value, ok, threshold=None):
        record["checks"].append({"name": name, "value": value, "threshold": threshold, "pass": bool(ok)})

    poker = None
    if scenario == "off":
        deadline = time.monotonic() + timeout
        while lab_pids(lab_id, "oni-surface") and time.monotonic() < deadline:
            time.sleep(0.2)
        # Turning Oni off is itself work (settings saved, the app told, the
        # presenter closed), which went on for some seconds after the surface
        # exited; the scenario is what follows once the daemon is quiet.
        quiet = daemon_quiet(lab_id, quiet_s, max(0.0, deadline - time.monotonic()))
        record["settle"] = {"ok": not lab_pids(lab_id, "oni-surface") and quiet is not None,
                            "daemon_quiet_after_s": quiet}
    elif scenario == "motion":
        commit = last_commit(ledger.since())
        if commit is None:
            raise SystemExit("measure: the ledger has no committed frame to poke")
        point = centre(global_rect(commit, lab_state_now()))
        poker = Poker(point)
        record["settle"] = {"ok": True, "control": f"pokes at {point[0]},{point[1]} every 2 s"}
    else:
        waited = ledger.quiet(quiet_s, timeout)
        record["settle"] = {"ok": waited is not None, "waited_s": waited}
    situation = summary(ledger.since())
    record["situation"] = situation
    want = {
        "motion": situation["visible"] is True and situation["state"] != "sleeping",
        "settled": situation["visible"] is True and situation["state"] != "sleeping",
        "asleep": situation["visible"] is True and situation["state"] == "sleeping",
        "covered": situation["visible"] is False and situation["reason"] == "courtesy",
        "hidden": situation["visible"] is False and situation["reason"] == "hidden",
        "off": situation["exited"] and situation["reason"] == "oni-off",
        "strand-held": situation["visible"] is True and situation["strand"] == "hold",
        "strand-retracted": situation["visible"] is True and situation["strand"] == "home",
        "perched": situation["visible"] is True and situation["perch"] is not None,
        "detached": situation["visible"] is True and situation["perch"] is None
        and situation["strand"] in (None, "home"),
    }[scenario]
    check("precondition", situation, want)
    check("settled before sampling", record["settle"], record["settle"]["ok"])
    if scenario in ("perched", "detached"):
        record["daemon_children_before"] = daemon_children(lab_id)

    procs = {pid: ("surface" if comm == "oni-surface" else "daemon")
             for comm in ("oni-surface", "oni") for pid in lab_pids(lab_id, comm)}
    before = {pid: counters(pid) for pid in procs}
    mark = ledger.mark()
    t0 = time.monotonic()
    if poker:
        poker.start()
    time.sleep(seconds)
    if poker:
        poker.done.set()
        poker.join(timeout=10)
    elapsed = time.monotonic() - t0
    after = {pid: counters(pid) for pid in procs}
    lines = ledger.since(mark)
    evs = {}
    for line in lines:
        evs[line.get("ev")] = evs.get(line.get("ev"), 0) + 1
    record["elapsed_s"] = round(elapsed, 3)
    record["ledger_window"] = {"lines": len(lines), "by_ev": evs,
                               "commits": evs.get("commit", 0), "arms": evs.get("arm", 0)}
    if poker:
        record["control"] = {"pokes": poker.pokes, "failures": poker.failures}

    record["processes"] = []
    for pid, role in sorted(procs.items()):
        b, a = before[pid], after[pid]
        entry = {"role": role, "pid": pid, "before": b, "after": a}
        if b is None or a is None:
            entry["error"] = "process exited during the sample"
        else:
            d = delta(b, a)
            cpu_ns = d["schedstat_ns"]
            entry["delta"] = d
            entry["rates"] = {
                "cpu_percent": round(100 * cpu_ns / (elapsed * 1e9), 5),
                "timeslices_per_s": round(d["timeslices"] / elapsed, 3),
                "wakeups_per_s": round(d.get("context_switches", 0) / elapsed, 3),
            }
        record["processes"].append(entry)

    surfaces = [p for p in record["processes"] if p["role"] == "surface"]
    daemons = [p for p in record["processes"] if p["role"] == "daemon"]
    units = subprocess.run(["systemctl", "--user", "list-units", "--all", "--plain", "--no-legend",
                            f"oni-lab-{lab_id}-surface-*"], capture_output=True, text=True,
                           check=False).stdout.split()
    record["surface_scopes"] = [{"unit": u, "active": systemctl_show(u, "ActiveState")}
                                for u in units if u.endswith(".scope")]
    commits, arms = record["ledger_window"]["commits"], record["ledger_window"]["arms"]
    if scenario == "motion":
        check("commits during the control", commits, commits > 0, "> 0")
        check("surface process present", len(surfaces), len(surfaces) == 1, 1)
    elif scenario == "off":
        check("no oni-surface process", len(surfaces), not surfaces, 0)
        active = [s for s in record["surface_scopes"] if s["active"] == "active"]
        holders = {p["before"]["scope"] for p in daemons if p.get("before")}
        stray = [s for s in active if s["unit"] not in holders]
        check("surface scopes inactive (or holding only a daemon the surface started)",
              record["surface_scopes"], not stray)
        # The daemon serves more than Oni and stays up; what Oni off must
        # leave it is the idle budget. A CPU delta of exactly zero is not
        # reachable for a live Go process (sysmon and the forced GC wake it a
        # few times a minute: 1 wakeup, 11.8 us in 45 s with no client at all).
        for p in daemons:
            rates = p.get("rates") or {}
            cpu, wake = rates.get("cpu_percent"), rates.get("wakeups_per_s")
            check(f"daemon {p['pid']} cpu percent", cpu,
                  cpu is not None and cpu < THRESHOLDS["cpu_percent_max"], f"< {THRESHOLDS['cpu_percent_max']}")
            check(f"daemon {p['pid']} wakeups per second", wake,
                  wake is not None and wake <= THRESHOLDS["wakeups_per_s_max"],
                  f"<= {THRESHOLDS['wakeups_per_s_max']}")
        check("commits", commits, commits == 0, 0)
    else:
        # the scenario must hold for the whole window: an event (the daemon's
        # sleep, a state, a show) would measure something else
        happened = {k: v for k, v in evs.items() if k in EVENT_EVS}
        check("no character event during the window", happened, not happened, {})
        check("commits", commits, commits == THRESHOLDS["commits"], THRESHOLDS["commits"])
        check("frame callbacks armed", arms, arms == THRESHOLDS["arms"], THRESHOLDS["arms"])
        check("surface process present", len(surfaces), len(surfaces) == 1, 1)
        for p in surfaces:
            rates = p.get("rates") or {}
            cpu, wake = rates.get("cpu_percent"), rates.get("wakeups_per_s")
            check("surface cpu percent", cpu, cpu is not None and cpu < THRESHOLDS["cpu_percent_max"],
                  f"< {THRESHOLDS['cpu_percent_max']}")
            check("surface wakeups per second", wake,
                  wake is not None and wake <= THRESHOLDS["wakeups_per_s_max"],
                  f"<= {THRESHOLDS['wakeups_per_s_max']}")
        if scenario in ("perched", "detached"):
            # a held perch costs the daemon what a settled character does;
            # after a detach the tracker's stream or poll is gone as well
            for p in daemons:
                rates = p.get("rates") or {}
                cpu, wake = rates.get("cpu_percent"), rates.get("wakeups_per_s")
                check(f"daemon {p['pid']} cpu percent", cpu,
                      cpu is not None and cpu < THRESHOLDS["cpu_percent_max"], f"< {THRESHOLDS['cpu_percent_max']}")
                check(f"daemon {p['pid']} wakeups per second", wake,
                      wake is not None and wake <= THRESHOLDS["wakeups_per_s_max"],
                      f"<= {THRESHOLDS['wakeups_per_s_max']}")
            record["daemon_children_after"] = daemon_children(lab_id)
            if scenario == "detached":
                left = record["daemon_children_before"] + record["daemon_children_after"]
                check("no compositor or capture process under the daemon (tracker off)", left, not left, [])
    record["pass"] = all(c["pass"] for c in record["checks"])
    record["finished_at"] = now()
    Path(out_path).write_text(json.dumps(record, indent=2) + "\n")
    s = surfaces[0]["rates"] if surfaces and surfaces[0].get("rates") else {}
    print(f"{scenario}: pass={record['pass']} commits={commits} arms={arms} "
          f"surface cpu={s.get('cpu_percent')}% wakeups/s={s.get('wakeups_per_s')} -> {out_path}")
    return 0 if record["pass"] else 1


# --- images -----------------------------------------------------------------

def image_stats(png):
    from PIL import Image, ImageStat
    im = Image.open(png).convert("RGB")
    small = im.resize((min(480, im.width), max(1, round(im.height * min(480, im.width) / im.width))))
    colours = small.getcolors(maxcolors=1 << 20) or []
    dominant = max((c for c, _ in colours), default=0) / (small.width * small.height)
    stddev = sum(ImageStat.Stat(small).stddev) / 3
    return {"path": str(png), "width": im.width, "height": im.height,
            "distinct_colours": len(colours), "dominant_fraction": round(dominant, 4),
            "stddev": round(stddev, 2), "nonblank": len(colours) >= 16 and stddev > 2}


def shot_record(png, dst, lab_id, output, region, state):
    image = image_stats(png)
    record = {"captured_at": now(), "lab_id": lab_id, "output": output,
              "region": region or None, "image": image, "nonblank": image.pop("nonblank"),
              "state": state}
    Path(dst).write_text(json.dumps(record, indent=2))
    apps = sorted({w.get("app_id") or "" for w in state["windows"]})
    print(f"{png}: {image['width']}x{image['height']} region={region or 'output'} "
          f"colours={image['distinct_colours']} stddev={image['stddev']} "
          f"nonblank={record['nonblank']} windows={apps}")


def foreground(img, base):
    """Mask of pixels that differ from the background crop by more than 24 in
    any channel."""
    from PIL import ImageChops
    if base.size != img.size:
        base = base.resize(img.size)
    r, g, b = ImageChops.difference(img, base).split()
    return ImageChops.lighter(ImageChops.lighter(r, g), b).point(lambda v: 255 if v > 24 else 0)


def hue_histogram(img, mask):
    """Hue histogram (36 bins) of the foreground's coloured pixels. Coloured
    means chroma (max - min channel) of at least 32: the accents are desktop
    palette roles, and Ryoku's lapis (speaking) is a muted blue near
    (117,134,172), HSV saturation 0.32, which an HSV saturation cut of 0.35
    dropped although it is plainly blue; idle draws no pixel above 30."""
    hist = [0] * 36
    for (r, g, b), (h, _, v), m in zip(img.getdata(), img.convert("HSV").getdata(), mask.getdata()):
        if m and v > 64 and max(r, g, b) - min(r, g, b) >= 32:
            hist[h * 36 // 256] += 1
    return hist


def mean_diff(a, b):
    from PIL import ImageChops, ImageStat
    if a.size != b.size:
        size = (min(a.width, b.width), min(a.height, b.height))
        a, b = a.resize(size), b.resize(size)
    return sum(ImageStat.Stat(ImageChops.difference(a, b)).mean) / 3


def hue_distance(a, b):
    d = abs(a - b) % 360
    return min(d, 360 - d)


# --- phase 8 ----------------------------------------------------------------

class Phase8:
    REPORT = "phase8.json"

    def __init__(self, lab, out):
        self.lab, self.out = Path(lab), Path(out)
        self.env = lab_env(lab)
        self.wm = self.env["LAB_WM"]
        self.ledger = Ledger(self.lab / "ledger/surface.jsonl")
        self.results = {"wm": self.wm, "lab_id": self.env["LAB_ID"], "started_at": now(),
                        "hyakki_bin": self.env.get("HYAKKI_BIN"), "steps": [], "measurements": {}}
        self.log = (self.out / "commands.log").open("a")
        self.crops = {}

    def save(self):
        steps = self.results["steps"]
        meas = self.results["measurements"].values()
        self.results["pass"] = bool(steps) and all(s["pass"] for s in steps) and all(m.get("pass") for m in meas)
        (self.out / self.REPORT).write_text(json.dumps(self.results, indent=2, default=jsonable) + "\n")

    def sh(self, name, *args, check=True, timeout=600):
        r = run_script(name, *args, timeout=timeout)
        self.log.write(f"{now()} {name} {' '.join(map(str, args))} -> {r.returncode}\n{r.stdout}{r.stderr}\n")
        self.log.flush()
        if check and r.returncode:
            raise RuntimeError(f"{name} {' '.join(map(str, args))} failed: {r.stderr.strip() or r.stdout.strip()}")
        return r

    def call(self, method, params=None):
        args = [method] if params is None else [method, json.dumps(params)]
        out = self.sh("ctl.sh", "hyakki", "call", *args).stdout
        return json.loads(out) if out.strip() else {}

    def settings(self, **oni):
        """settings.set as any CLI client: SettingsSetParams carries the merge
        patch under "patch". A bare {"oni":...} has no patch, which the daemon
        reads as reaching everything and refuses to a non-app client; the oni
        keys themselves are not the app's alone (daemon/gate.go settingsForApp)."""
        return self.call("settings.set", {"patch": {"oni": oni}})

    def placement(self):
        result = self.call("oni.open", {"renderer": "observer"})
        for frame in result.get("frames", []):
            if frame.get("kind") == "placement":
                return frame["data"]
        return None

    def state(self):
        return lab_state_now()

    def output(self, name, state=None):
        for o in (state or self.state())["outputs"]:
            if o["name"] == name:
                return o
        return None

    def settle(self, seconds=1.0, timeout=30):
        """Wait until the committed rect and output have not changed for
        SECONDS. An awake character keeps breathing, so commits alone never
        go quiet before the 30 s settled stop."""
        start = time.monotonic()
        last, since = None, start
        while time.monotonic() - start < timeout:
            commit = self.ledger.current()
            key = commit and (tuple(commit["rect"]), commit.get("output"))
            t = time.monotonic()
            if key != last:
                last, since = key, t
            elif key is not None and t - since >= seconds:
                return True
            time.sleep(0.2)
        raise RuntimeError(f"the character did not hold still for {seconds} s within {timeout} s")

    def current(self):
        commit = self.ledger.current()
        if commit is None:
            raise RuntimeError("the ledger has no committed frame")
        return commit

    def surface_now(self):
        """The surface's clock now, as far as the ledger shows it: the t of
        its newest line (a stopped surface writes none, so this lags then)."""
        lines = self.ledger.since()
        return lines[-1].get("t", 0.0) if lines else 0.0

    def shoot(self, name, rect):
        rect = [int(round(value)) for value in rect]
        png = self.out / "crops" / f"{name}.png"
        self.sh("shot.sh", png, f"{rect[0]},{rect[1]} {rect[2]}x{rect[3]}")
        return png

    def wait(self, mark, timeout, **conds):
        line = self.ledger.wait(lambda l: match(l, conds), mark, timeout)
        if line is None:
            raise RuntimeError(f"ledger never showed {conds} within {timeout} s")
        return line

    def step(self, number, name, fn):
        record = {"step": number, "name": name, "started_at": now()}
        try:
            details, ok = fn()
            record.update(details=details, pass_=ok)
        except Exception as error:  # a failed step is recorded and the run goes on
            record.update(error=f"{type(error).__name__}: {error}", pass_=False)
        record["pass"] = bool(record.pop("pass_"))
        record["finished_at"] = now()
        self.results["steps"].append(record)
        self.save()
        print(f"step {number} {name}: {'pass' if record['pass'] else 'FAIL'}"
              + (f" ({record['error']})" if "error" in record else ""))

    def measure(self, scenario, timeout=180, seconds=60, name=None, gate=True):
        """measure.sh SCENARIO into measure-NAME.json. An ungated record
        (a cost the run states as a limit) is kept beside the gate's
        measurements, not among them."""
        name = name or scenario
        path = self.out / f"measure-{name}.json"
        env = dict(os.environ, ONI_MEASURE_TIMEOUT=str(timeout), ONI_MEASURE_SECONDS=str(seconds))
        r = subprocess.run([str(HERE / "measure.sh"), scenario, str(path)], capture_output=True,
                           text=True, env=env, timeout=timeout + 300)
        self.log.write(f"{now()} measure.sh {scenario} ({name}) -> {r.returncode}\n{r.stdout}{r.stderr}\n")
        try:
            record = json.loads(path.read_text())
        except (OSError, ValueError):
            record = {"pass": False, "error": (r.stderr or r.stdout).strip()}
        if gate:
            self.results["measurements"][name] = record
        else:
            self.results.setdefault("recorded_measurements", {})[name] = record
        self.save()
        print(f"measure {name}: {'pass' if record.get('pass') else 'FAIL'}{'' if gate else ' (recorded, not gated)'}")
        return record

    # Step 0: what the compositor offers, and that the surface is a layer surface.
    def census(self):
        globals_ = json.loads(self.sh("ctl.sh", "globals").stdout)
        (self.out / "globals.json").write_text(json.dumps(globals_, indent=2))
        state = self.state()
        listed = state["layers"] is not None
        ours = [s for s in state["layers"] or [] if s["namespace"] == "hyakki-oni"]
        start = next((l for l in self.ledger.since() if l.get("ev") == "start"), None)
        ok = "zwlr_layer_shell_v1" in globals_.get("bound", {}) and (not listed or bool(ours))
        return {"globals": globals_, "layer_listing": listed, "hyakki_oni_layers": ours,
                "first_ledger_line": start,
                "note": None if listed else "the compositor lists no layer surfaces; the ledger stands in"}, ok

    # Step 1: distinct states, driven by the fake daemon's script.
    def states(self):
        mark = self.ledger.mark()
        self.sh("ctl.sh", "surface", "restart", f"HYAKKI_ONI_SCRIPT={HERE / 'fixtures/states.jsonl'}")
        self.wait(mark, 15, ev="commit")
        time.sleep(2)
        state = self.state()
        shots, poses, rects = {}, {}, {}
        for name in STATE_ORDER:
            if name != "idle":
                self.wait(mark, 20, ev="state", state=name)
                time.sleep(1.5)
            commit = self.current()
            rect = global_rect(commit, state)
            rects[name], poses[name] = rect, commit.get("pose")
            shots[name] = self.shoot(f"state-{name}", rect)
        self.wait(mark, 20, ev="hide", reason="hidden")
        time.sleep(0.5)
        x0 = min(r[0] for r in rects.values())
        y0 = min(r[1] for r in rects.values())
        x1 = max(r[0] + r[2] for r in rects.values())
        y1 = max(r[1] + r[3] for r in rects.values())
        base_png = self.shoot("state-baseline", [x0, y0, x1 - x0, y1 - y0])
        return self.analyse_states(shots, rects, poses, base_png, (x0, y0, x1 - x0, y1 - y0))

    def analyse_states(self, shots, rects, poses, base_png, box):
        from PIL import Image
        base = Image.open(base_png).convert("RGB")
        factor = base.width / box[2]
        crops, masks, hists, metrics = {}, {}, {}, {}
        for name, png in shots.items():
            img = Image.open(png).convert("RGB")
            r = rects[name]
            left, top = round((r[0] - box[0]) * factor), round((r[1] - box[1]) * factor)
            bg = base.crop((left, top, left + img.width, top + img.height))
            mask = foreground(img, bg)
            crops[name], masks[name], hists[name] = img, mask, hue_histogram(img, mask)
            visible = mask.histogram()[255] / (img.width * img.height)
            metrics[name] = {"rect": r, "foreground_fraction": round(visible, 4),
                             "saturated_pixels": sum(hists[name])}
        checks = []
        for name in STATE_ORDER:
            checks.append({"name": f"{name} drawn", "value": metrics[name]["foreground_fraction"],
                           "pass": metrics[name]["foreground_fraction"] >= 0.01})
        accents = {}
        for name in ACCENT_STATES:
            excess = [max(0, a - b) for a, b in zip(hists[name], hists["idle"])]
            mass = sum(excess) / max(1, masks[name].histogram()[255])
            peak = max(range(36), key=lambda i: excess[i])
            accents[name] = {"hue_deg": peak * 10 + 5, "excess_fraction": round(mass, 4)}
            checks.append({"name": f"{name} accent absent in idle", "value": accents[name],
                           "pass": mass >= 0.01})
        for i, a in enumerate(ACCENT_STATES):
            for b in ACCENT_STATES[i + 1:]:
                d = hue_distance(accents[a]["hue_deg"], accents[b]["hue_deg"])
                checks.append({"name": f"{a} vs {b} accent hue distance", "value": d, "pass": d >= 20})
        diffs = {}
        for i, a in enumerate(STATE_ORDER):
            for b in STATE_ORDER[i + 1:]:
                d = round(mean_diff(crops[a], crops[b]), 3)
                diffs[f"{a}/{b}"] = d
                checks.append({"name": f"{a} vs {b} pixels differ", "value": d, "pass": d >= 1.0})
        pose_diffs = {}
        for i, a in enumerate(STATE_ORDER):
            for b in STATE_ORDER[i + 1:]:
                pa, pb = poses[a] or {}, poses[b] or {}
                keys = [k for k in pa if isinstance(pa.get(k), (int, float)) and isinstance(pb.get(k), (int, float))]
                pose_diffs[f"{a}/{b}"] = round(max((abs(pa[k] - pb[k]) for k in keys), default=0), 5)
        details = {"metrics": metrics, "accents": accents, "pixel_diffs": diffs,
                   "pose_max_channel_diffs": pose_diffs, "poses": poses, "checks": checks,
                   "baseline": str(base_png)}
        return details, all(c["pass"] for c in checks)

    # Step 2: every gesture, slowed tenfold, mid-gesture crop and return to base.
    def gestures(self):
        spec = json.loads((HERE.parents[2] / "internal/oni/spec/oni-spec.json").read_text())
        durations = spec["gestures"]["durationS"]
        mark = self.ledger.mark()
        self.sh("ctl.sh", "surface", "restart", "HYAKKI_ONI_TIMESCALE=0.1",
                f"HYAKKI_ONI_SCRIPT={HERE / 'fixtures/gestures.jsonl'}")
        self.wait(mark, 15, ev="commit")
        state = self.state()
        results, checks = {}, []
        script = [json.loads(l) for l in (HERE / "fixtures/gestures.jsonl").read_text().splitlines() if l.strip()]
        order = [e["frame"]["data"]["id"] for e in script if e.get("frame", {}).get("kind") == "gesture"]
        for gid in order:
            slow = durations[gid] * 10
            base_commit = self.current()
            line = self.wait(mark, slow + 30, ev="gesture", gesture=gid)
            before = [l for l in self.ledger.since(mark) if l.get("ev") == "commit"]
            base = (last_commit([l for l in before if l.get("t", 0) <= line.get("t", 0)]) or base_commit)["pose"]
            # Aim at the surface's own clock: the newest ledger line is "now"
            # there, so detection lag does not push the crop past the gesture.
            time.sleep(max(0.0, line["t"] + slow / 2 - self.surface_now()))
            mid_commit = self.current()
            png = self.shoot(f"gesture-{gid}", global_rect(mid_commit, state))
            time.sleep(max(0.0, line["t"] + slow + 2 - self.surface_now()))
            final = self.current()["pose"]
            mid = mid_commit["pose"]
            channels = [k for k in POSE_CHANNELS if k in base and k in mid and k in final]
            dev = max((abs(mid[k] - base[k]) for k in channels), default=0)
            ret = max((abs(final[k] - base[k]) for k in channels), default=math.inf)
            results[gid] = {"slowed_s": slow, "mid_deviation": round(dev, 5), "return_deviation": round(ret, 5),
                            "mid_gesture_field": mid.get("gesture"), "mid_t": mid_commit.get("t"),
                            "gesture_t": line.get("t"), "crop": str(png),
                            "base": base, "mid": mid, "final": final}
            checks.append({"name": f"{gid} crop is mid-gesture", "value": mid.get("gesture"),
                           "pass": mid.get("gesture") == gid})
            checks.append({"name": f"{gid} moves the pose", "value": dev, "pass": dev > 1e-3})
            checks.append({"name": f"{gid} returns to the base pose", "value": ret, "pass": ret <= 0.05})
        return {"gestures": results, "checks": checks}, all(c["pass"] for c in checks)

    # Step 3: drag, then the same pixels after a surface and a daemon restart.
    def drag(self):
        mark = self.ledger.mark()
        self.sh("ctl.sh", "surface", "restart")
        self.wait(mark, 15, ev="commit")
        self.settle(1.5)
        state = self.state()
        c0 = self.current()
        out = self.output(c0["output"], state)
        x, y, w, h = c0["rect"]
        g0 = global_rect(c0, state)
        sx, sy = centre(g0)
        # Since Phase 10 a drop over a window perches on it, so the drop goes
        # to the first of these fractions whose pointer end lands on bare
        # desktop (a niri tiled window has no position and cannot be
        # perched on) and moves the character at least 200 px
        boxes = [[*win["pos"], *win["size"]] for win in state["windows"] if win.get("pos") and win.get("size")]
        drop = None
        for fx, fy in ((0.30, 0.40), (0.90, 0.40), (0.90, 0.10), (0.50, 0.10), (0.10, 0.90), (0.90, 0.90)):
            tx, ty = round(fx * (out["width"] - w)), round(fy * (out["height"] - h))
            ex, ey = sx + (tx - x), sy + (ty - y)
            bare = not any(bx <= ex < bx + bw and by <= ey < by + bh for bx, by, bw, bh in boxes)
            if bare and math.hypot(tx - x, ty - y) >= 200:
                drop = (fx, fy)
                break
        if drop is None:
            return {"from": c0["rect"], "windows": boxes,
                    "checks": [{"name": "bare desktop to drop on", "value": boxes, "pass": False}]}, False
        mark = self.ledger.mark()
        self.sh("input.sh", "drag", sx, sy, ex, ey)
        self.wait(mark, 10, ev="drop")
        self.settle(1.5)
        c1 = self.current()
        r1 = c1["rect"]
        place = self.placement()
        saved = next((o for o in (place or {}).get("outputs", []) if o.get("id") == c1["output"]), None)
        span_x, span_y = out["width"] - w, out["height"] - h
        fx_r, fy_r = r1[0] / span_x, r1[1] / span_y
        file = self.lab / "state/hyakki/oni-placement.json"
        persisted = json.loads(file.read_text()) if file.is_file() else None
        perched = [l for l in self.ledger.since(mark) if l.get("ev") == "perch" and l.get("perch")]
        checks = [
            {"name": "the drop on bare desktop perched on nothing", "value": perched, "pass": not perched},
            {"name": "rect moved to the drop point", "value": [r1, [tx, ty]],
             "pass": abs(r1[0] - tx) <= 2 and abs(r1[1] - ty) <= 2},
            {"name": "placement is free, not a corner", "value": (place or {}).get("corner"),
             "pass": place is not None and place.get("corner") == ""},
            {"name": "saved fraction equals the drop point within 1/extent",
             "value": {"saved": saved, "from_rect": [fx_r, fy_r]},
             "pass": saved is not None and abs(saved["fx"] - fx_r) <= 1 / span_x
             and abs(saved["fy"] - fy_r) <= 1 / span_y},
            {"name": "placement file written", "value": persisted, "pass": persisted is not None},
        ]
        mark = self.ledger.mark()
        self.sh("ctl.sh", "surface", "restart")
        self.wait(mark, 15, ev="commit")
        self.settle(1.5)
        r2 = self.current()["rect"]
        checks.append({"name": "same rect after a surface restart", "value": r2, "pass": r2 == r1})
        self.sh("ctl.sh", "surface", "stop")
        self.sh("ctl.sh", "daemon", "restart")
        mark = self.ledger.mark()
        self.sh("ctl.sh", "surface", "start")
        self.wait(mark, 15, ev="commit")
        self.settle(1.5)
        r3 = self.current()["rect"]
        checks.append({"name": "same rect after a daemon restart", "value": r3, "pass": r3 == r1})
        self.results["dropped"] = {"output": c1["output"], "rect": r1}
        return {"from": c0["rect"], "drop_fraction": drop, "windows": boxes, "drag": [sx, sy, ex, ey],
                "placement": place, "checks": checks}, all(c["pass"] for c in checks)

    # Step 4: reduced motion, one commit per event and none for hovering.
    def reduced(self):
        mark = self.ledger.mark()
        self.settings(reduceMotion="on")
        self.wait(mark, 10, ev="frame", kind="motion")
        time.sleep(1)
        state = self.state()
        rect = global_rect(self.current(), state)
        cx, cy = centre(rect)
        self.sh("input.sh", "click", cx, cy)
        time.sleep(1)
        steps = [f"a {rect[0] + 2} {cy}"] + [f"m {max(1, rect[2] // 12)} 0" for _ in range(10)] + ["w 200"]
        self.sh("input.sh", "ptr", *steps)
        time.sleep(3)
        lines = self.ledger.since(mark)
        start = next((i for i, l in enumerate(lines)
                      if l.get("ev") == "frame" and l.get("kind") == "motion"), 0)
        lines = lines[start:]
        commits, events, bare = 0, 0, 0
        pending = False
        for line in lines:
            ev = line.get("ev")
            if ev in EVENT_EVS:
                events += 1
                pending = True
            elif ev == "commit":
                commits += 1
                if not pending:
                    bare += 1
                pending = False
        self.settings(reduceMotion="follow")
        checks = [{"name": "at least one commit", "value": commits, "pass": commits >= 1},
                  {"name": "no commit without an event before it (no hover or loop frames)",
                   "value": bare, "pass": bare == 0},
                  {"name": "commits do not exceed events", "value": [commits, events], "pass": commits <= events}]
        return {"commits": commits, "events": events, "checks": checks}, all(c["pass"] for c in checks)

    # Step 5: display changes.
    def display(self):
        return self.display_hyprland() if self.wm == "hyprland" else self.display_niri()

    def flight(self, lines):
        rects = []
        for line in lines:
            if line.get("ev") == "commit" and line.get("rect") and line["rect"] not in rects:
                rects.append(line["rect"])
        return rects

    def display_hyprland(self):
        state = self.state()
        primary = self.env["LAB_OUTPUT"]
        others = [o for o in state["outputs"] if o["enabled"] and o["name"] != primary]
        if not others:
            raise RuntimeError("needs up.sh hyprland --outputs 2")
        second = others[0]
        self.settle(1)
        c1 = self.current()
        r1, o1 = c1["rect"], c1["output"]
        g1 = global_rect(c1, state)
        sx, sy = centre(g1)
        ex = second["x"] + second["width"] // 2
        ey = second["y"] + second["height"] // 2
        mark = self.ledger.mark()
        self.sh("input.sh", "drag", sx, sy, ex, ey)
        moved = self.ledger.wait(lambda l: l.get("ev") == "commit" and l.get("output") == second["name"],
                                 mark, 10)
        how = "drag"
        if moved is None:
            # A drag stays on the output it started on (the layer surface is
            # per output), so the drop at the shared edge is now what the
            # first output remembers: that is where the character must return.
            how = "oni.place"
            self.wait(mark, 10, ev="drop")
            self.settle(1.5)
            c1 = self.current()
            r1 = c1["rect"]
            mark = self.ledger.mark()
            self.call("oni.place", {"output": second["name"], "fx": 0.5, "fy": 0.5, "corner": ""})
            self.wait(mark, 10, ev="commit", output=second["name"])
        self.settle(1.5)
        r2 = self.current()["rect"]
        mark = self.ledger.mark()
        self.sh("outputs.sh", "remove", second["name"])
        self.wait(mark, 10, ev="output")
        self.settle(2, 20)
        after = self.ledger.since(mark)
        back = self.current()
        rects = self.flight(after)
        mark = self.ledger.mark()
        self.sh("outputs.sh", "add", second["name"], second["x"])
        returned = self.ledger.wait(lambda l: l.get("ev") == "output", mark, 10)
        self.settle(2, 20)
        final = self.current()
        checks = [
            {"name": f"character reached {second['name']}", "value": {"how": how, "rect": r2}, "pass": True},
            {"name": f"back on {o1} after removing {second['name']}", "value": back.get("output"),
             "pass": back.get("output") == o1},
            {"name": "flew to the remembered fraction", "value": [back["rect"], r1],
             "pass": all(abs(a - b) <= 1 for a, b in zip(back["rect"], r1))},
            {"name": "flight frames between the outputs", "value": len(rects), "pass": len(rects) >= 3},
        ]
        return {"first": {"output": o1, "rect": r1}, "second": {"output": second["name"], "rect": r2},
                "flight_rects": rects, "after_readd": {"output_event": returned, "commit": final},
                "checks": checks}, all(c["pass"] for c in checks)

    def display_niri(self):
        output = self.env["LAB_OUTPUT"]
        self.settle(1)
        c1 = self.current()
        r1 = c1["rect"]
        place1 = self.placement()
        mark = self.ledger.mark()
        self.sh("outputs.sh", "scale", output, "1.5")
        self.wait(mark, 10, ev="output")
        self.settle(2, 20)
        scaled_lines = self.ledger.since(mark)
        cs = self.current()
        o = self.output(output)
        w, h = cs["rect"][2], cs["rect"][3]
        saved = next((s for s in (place1 or {}).get("outputs", []) if s.get("id") == output), None)
        fx = cs["rect"][0] / (o["width"] - w)
        fy = cs["rect"][1] / (o["height"] - h)
        place2 = self.placement()
        mark = self.ledger.mark()
        self.sh("outputs.sh", "scale", output, "1")
        self.wait(mark, 10, ev="output")
        self.settle(2, 20)
        r_back = self.current()["rect"]
        mark = self.ledger.mark()
        self.sh("outputs.sh", "off", output)
        turned_off = not (self.output(output) or {}).get("enabled", True)
        hidden = self.ledger.wait(lambda l: l.get("ev") == "hide", mark, 10) if turned_off else None
        self.sh("outputs.sh", "on", output)
        shown = self.ledger.wait(lambda l: l.get("ev") in ("show", "commit"), mark, 15)
        self.settle(2, 20)
        r_on = self.current()["rect"]
        checks = [
            {"name": "fraction kept at scale 1.5", "value": {"saved": saved, "at_scale": [fx, fy]},
             "pass": saved is not None and abs(saved["fx"] - fx) <= 1 / (o["width"] - w)
             and abs(saved["fy"] - fy) <= 1 / (o["height"] - h)},
            {"name": "saved placement unchanged by the scale change", "value": [place1, place2],
             "pass": place1 is not None and place2 is not None
             and place1.get("outputs") == place2.get("outputs")},
            {"name": "same rect back at scale 1", "value": [r_back, r1], "pass": r_back == r1},
            {"name": "flight frames to the place at scale 1.5", "value": len(self.flight(scaled_lines)),
             "pass": len(self.flight(scaled_lines)) >= 3},
            {"name": "same rect after the output is back", "value": [r_on, r1], "pass": r_on == r1},
        ]
        if turned_off:
            checks.append({"name": "hidden while the output is off", "value": hidden, "pass": hidden is not None})
        else:
            # Nested niri runs on one winit output, which `niri msg output
            # winit off` leaves enabled: removal cannot be shown here. It is
            # Hyprland's step 5 (a headless output removed and re-added).
            self.results.setdefault("limits", []).append(
                {"step": 5, "check": "hidden while the output is off",
                 "evidence": {"after_off": self.output(output)},
                 "reason": "niri's winit output stays enabled after `output winit off`"})
        return {"scaled": cs, "flight_rects_at_scale": self.flight(scaled_lines), "shown": shown,
                "turned_off": turned_off, "checks": checks}, all(c["pass"] for c in checks)

    def app_output(self, state):
        """The output the dev.ryoku.Hyakki window is on."""
        win = next(w for w in state["windows"] if w.get("app_id") == "dev.ryoku.Hyakki")
        for o in state["outputs"]:
            if win.get("monitor") is not None and o.get("id") == win["monitor"]:
                return o["name"]
        return self.env["LAB_OUTPUT"]

    def wake(self):
        """Wake the character as a click would (oni.interact wake, then poke:
        a poke is what restarts the daemon's 90 s sleep timer, as the earlier
        runs showed), so a scenario that needs it awake cannot have the sleep
        fall inside its window; wait until the ledger says it is awake."""
        mark = self.ledger.mark()
        self.call("oni.interact", {"kind": "wake"})
        if summary(self.ledger.since()).get("state") == "sleeping":
            self.wait(mark, 10, ev="state", state="idle")
        self.call("oni.interact", {"kind": "poke"})

    # Step 6: fullscreen hyakki-app covers the character's output.
    def covered(self):
        app_out = self.app_output(self.state())
        here = self.current().get("output")
        checks = []
        if here != app_out:
            # A fullscreen window covers only its own output: first show that
            # the character elsewhere stays, then put it under the window.
            mark = self.ledger.mark()
            self.sh("ctl.sh", "fullscreen", "on")
            stray = self.ledger.wait(lambda l: l.get("ev") == "hide", mark, 3)
            self.sh("ctl.sh", "fullscreen", "off")
            checks.append({"name": f"shown on {here} while {app_out} is fullscreen", "value": stray,
                           "pass": stray is None})
            mark = self.ledger.mark()
            self.call("oni.place", {"output": app_out, "fx": 0.5, "fy": 0.5, "corner": ""})
            self.wait(mark, 10, ev="commit", output=app_out)
            self.settle(1.5)
        # the covered window must not hold the daemon's 90 s sleep
        self.wake()
        mark = self.ledger.mark()
        self.sh("ctl.sh", "fullscreen", "on")
        hide = self.ledger.wait(lambda l: l.get("ev") == "hide" and l.get("reason") == "courtesy", mark, 10)
        measured = self.measure("covered") if hide else None
        mark = self.ledger.mark()
        self.sh("ctl.sh", "fullscreen", "off")
        show = self.ledger.wait(lambda l: l.get("ev") in ("show", "commit"), mark, 10)
        checks += [{"name": "courtesy hide recorded", "value": hide, "pass": hide is not None},
                   {"name": "no commits while covered",
                    "value": (measured or {}).get("ledger_window"), "pass": bool(measured and measured.get("pass"))},
                   {"name": "back after leaving fullscreen", "value": show, "pass": show is not None}]
        return {"app_output": app_out, "character_output_before": here, "checks": checks}, all(c["pass"] for c in checks)

    def measurements(self):
        self.wake()
        self.measure("motion")
        # The daemon's sleep (spec sleep, 90 s) counts from the control's
        # last poke; the settled stop takes 30 s and the quiet wait 5 s more,
        # so a 60 s window ran into the sleep gesture. 45 s ends before it.
        self.measure("settled", timeout=120, seconds=45)
        mark = self.ledger.mark()
        self.settings(hidden=True)
        if self.ledger.wait(lambda l: l.get("ev") == "hide" and l.get("reason") == "hidden", mark, 10):
            self.measure("hidden")
        else:
            self.results["measurements"]["hidden"] = {"pass": False, "error": "no hidden hide in the ledger"}
        mark = self.ledger.mark()
        self.settings(hidden=False)
        self.ledger.wait(lambda l: l.get("ev") in ("show", "commit"), mark, 10)
        # Asleep needs the real 90 s without input: park the pointer off the character.
        state = self.state()
        rect = global_rect(self.current(), state)
        out = self.output(self.env["LAB_OUTPUT"], state)
        px = out["x"] + (out["width"] - 80 if rect[0] < out["width"] // 2 else 80)
        self.sh("input.sh", "move", px, out["y"] + out["height"] - 80)
        mark = self.ledger.mark()
        if self.ledger.wait(lambda l: l.get("ev") == "state" and l.get("state") == "sleeping", mark, 180):
            self.measure("asleep", timeout=60)
        else:
            self.results["measurements"]["asleep"] = {"pass": False, "error": "never fell asleep within 180 s"}
        mark = self.ledger.mark()
        self.settings(master=False)
        self.ledger.wait(lambda l: l.get("ev") == "exit", mark, 15)
        self.measure("off", timeout=60)

    def display_check(self):
        display = self.env["LAB_DISPLAY"]
        bad, seen = [], 0
        for path in sorted((self.lab / "ledger").glob("*.jsonl")):
            for line in Ledger(path).since():
                seen += 1
                d = line.get("display")
                if d not in (display, os.path.basename(display)):
                    bad.append({"file": path.name, "display": d})
        return {"lines": seen, "lab_display": display, "foreign": bad[:20]}, seen > 0 and not bad

    def run(self):
        (self.out / "crops").mkdir(parents=True, exist_ok=True)
        self.step(0, "census", self.census)
        self.step(1, "distinct states", self.states)
        self.step(2, "gestures", self.gestures)
        self.step(3, "drag and saved geometry", self.drag)
        self.step(4, "reduced motion", self.reduced)
        self.step(5, "display change", self.display)
        if any(w.get("app_id") == "dev.ryoku.Hyakki" for w in self.state()["windows"]):
            self.step(6, "covered", self.covered)
        else:
            self.results["steps"].append({"step": 6, "name": "covered", "pass": False,
                                          "error": "no dev.ryoku.Hyakki window (up.sh --app)"})
        try:
            self.measurements()
        except Exception as error:
            self.results["measurements_error"] = f"{type(error).__name__}: {error}"
        self.step(7, "ledger display is the lab's", self.display_check)
        self.results["finished_at"] = now()
        self.save()
        for limit in self.results.get("limits", []):
            print(f"limit (step {limit['step']}): {limit['check']} not shown: {limit['reason']}")
        return 0 if self.results["pass"] else 1


# --- phase 9 ----------------------------------------------------------------

def strand_commits(lines):
    return [l for l in lines if l.get("ev") == "commit" and l.get("part") == "strand" and l.get("strand")]


def tip_of(line):
    tip = line["strand"]["tip"]
    return (tip["X"], tip["Y"])


def dist(a, b):
    return math.hypot(a[0] - b[0], a[1] - b[1])


def overlap(a, b):
    """The area two x, y, w, h rectangles share."""
    return (max(0.0, min(a[0] + a[2], b[0] + b[2]) - max(a[0], b[0]))
            * max(0.0, min(a[1] + a[3], b[1] + b[3]) - max(a[1], b[1])))


def polyline_dist(p, pts):
    if not pts:
        return math.inf
    if len(pts) == 1:
        return dist(p, pts[0])
    best = math.inf
    for a, b in zip(pts, pts[1:]):
        dx, dy = b[0] - a[0], b[1] - a[1]
        n = dx * dx + dy * dy
        k = 0.0 if n == 0 else max(0.0, min(1.0, ((p[0] - a[0]) * dx + (p[1] - a[1]) * dy) / n))
        best = min(best, dist(p, (a[0] + k * dx, a[1] + k * dy)))
    return best


def xywh(d):
    return (d["x"], d["y"], d["w"], d["h"])


def grow(rect, pad):
    return [rect[0] - pad, rect[1] - pad, rect[2] + 2 * pad, rect[3] + 2 * pad]


def inside(p, rect):
    return rect[0] <= p[0] <= rect[0] + rect[2] and rect[1] <= p[1] <= rect[1] + rect[3]


def load_rgb(png):
    import numpy as np
    from PIL import Image
    return np.asarray(Image.open(png).convert("RGB")).astype(np.int16)


def erode(mask, k):
    """Keep a pixel only where every pixel up to K away across and down is
    set too."""
    import numpy as np
    kept = mask.copy()
    for i in range(1, k + 1):
        for dx, dy in ((i, 0), (-i, 0), (0, i), (0, -i)):
            kept &= np.roll(np.roll(mask, dy, axis=0), dx, axis=1)
    return kept


def mask_points(mask, rect, f):
    """A crop mask's pixels as global logical points (pixel centres)."""
    import numpy as np
    ys, xs = np.nonzero(mask)
    if not len(xs):
        return np.zeros((0, 2))
    return np.stack([rect[0] + (xs + 0.5) / f, rect[1] + (ys + 0.5) / f], axis=1)


def read_wav_f32(path):
    """Mono float32 PCM and its rate from a WAV parec wrote."""
    import numpy as np
    data = Path(path).read_bytes()
    pos, rate, pcm = 12, None, None
    while pos + 8 <= len(data):
        cid, size = data[pos:pos + 4], int.from_bytes(data[pos + 4:pos + 8], "little")
        body = data[pos + 8:pos + 8 + size]
        if cid == b"fmt ":
            rate = int.from_bytes(body[4:8], "little")
            channels = int.from_bytes(body[2:4], "little")
            if channels != 1:
                raise ValueError(f"{path}: {channels} channels")
        elif cid == b"data":
            pcm = np.frombuffer(body[:len(body) // 4 * 4], dtype="<f4")
        pos += 8 + size + (size & 1)
    if rate is None or pcm is None:
        raise ValueError(f"{path}: no fmt or data chunk")
    return pcm, rate


def midi_hz(note):
    return 440.0 * 2 ** ((note - 69) / 12)


def peak_hz(segment, rate):
    import numpy as np
    n = 1 << 16
    spectrum = np.abs(np.fft.rfft(segment * np.hanning(len(segment)), n))
    freqs = np.arange(len(spectrum)) * rate / n
    band = (freqs > 100) & (freqs < 4000)
    i = int(np.flatnonzero(band)[0] + np.argmax(spectrum[band]))
    a, b, c = spectrum[i - 1], spectrum[i], spectrum[i + 1]
    shift = 0.5 * (a - c) / (a - 2 * b + c) if a - 2 * b + c else 0.0
    return (i + shift) * rate / n


def sound_segments(pcm, rate, floor=1e-4, gap_s=0.01):
    """Runs of audible samples as (start, end) seconds, joining gaps under gap_s."""
    import numpy as np
    idx = np.flatnonzero(np.abs(pcm) > floor)
    if not len(idx):
        return []
    out, start, prev = [], idx[0], idx[0]
    gap = int(gap_s * rate)
    for i in idx[1:]:
        if i - prev > gap:
            out.append((start / rate, (prev + 1) / rate))
            start = i
        prev = i
    out.append((start / rate, (prev + 1) / rate))
    return out


def player_procs(lab_id):
    """oni-surface's audio players in this lab: pid, comm and the
    environment that decides which audio server they reach."""
    found = {}
    for pid in lab_pids(lab_id):
        try:
            comm = open(f"/proc/{pid}/comm").read().strip()
        except OSError:
            continue
        if comm not in PLAYERS:
            continue
        env = {}
        try:
            for entry in open(f"/proc/{pid}/environ", "rb").read().split(b"\0"):
                k, _, v = entry.decode(errors="replace").partition("=")
                if k in ("XDG_RUNTIME_DIR", "PIPEWIRE_REMOTE", "PIPEWIRE_RUNTIME_DIR",
                         "PULSE_SERVER", "PULSE_RUNTIME_PATH", "HOME"):
                    env[k] = v
        except OSError:
            pass
        found[pid] = {"comm": comm, "scope": scope_of(pid), "env": env}
    return found


class PlayerWatch(threading.Thread):
    """Samples the lab's audio player processes every PERIOD seconds, and the
    ledger clock, so player lifetimes map onto ledger time; for the first
    players it asks the private graph whether it holds their client."""

    def __init__(self, lab_id, ledger_path, period=0.05):
        super().__init__(daemon=True)
        self.lab_id, self.period, self.done = lab_id, period, threading.Event()
        self.ledger = Ledger(ledger_path)
        self.players, self.offsets, self.lookups = {}, [], []

    def run(self):
        while not self.done.is_set():
            wall = time.monotonic()
            lines = self.ledger.since()
            if lines:
                self.offsets.append(wall - lines[-1].get("t", 0.0))
            for pid, info in player_procs(self.lab_id).items():
                seen = self.players.get(pid)
                if seen is None:
                    self.players[pid] = dict(info, first=wall, last=wall, private_client=None)
                    if len(self.lookups) < 6:
                        th = threading.Thread(target=self.lookup, args=(pid,), daemon=True)
                        self.lookups.append(th)
                        th.start()
                else:
                    seen["last"] = wall
            self.done.wait(self.period)
        for th in self.lookups:
            th.join(timeout=10)

    def lookup(self, pid):
        r = run_script("audio.sh", "clients", timeout=20)
        try:
            objects = json.loads(r.stdout)
        except ValueError:
            return
        for o in objects:
            props = (o.get("info") or {}).get("props") or {}
            if o.get("type", "").endswith(":Client") and str(props.get("application.process.id")) == str(pid):
                self.players[pid]["private_client"] = {
                    "id": o.get("id"), "binary": props.get("application.process.binary"),
                    "name": props.get("application.name")}

    def offset(self):
        """wall - ledger t: the newest ledger line is never ahead of now, so
        the smallest difference seen is the clock offset."""
        return min(self.offsets) if self.offsets else None


class Phase9(Phase8):
    REPORT = "phase9.json"

    def __init__(self, lab, out):
        super().__init__(lab, out)
        self.spec = json.loads((HERE.parents[2] / "internal/oni/spec/oni-spec.json").read_text())
        stroke = self.spec["strand"]["stroke"]
        # the white core's round cap is centred on the tip: the farthest
        # strand pixel lies half the ink width past it, away from the anchor
        self.cap = (stroke["widthPx"] + 2 * stroke["outlinePx"]) / 2
        for d in ("crops", "fixtures", "arrivals", "ledgers", "audio"):
            (self.out / d).mkdir(parents=True, exist_ok=True)
        test = os.environ.get("ONI_LAB_STRAND_TEST") or str(Path(self.env.get("HYAKKI_BIN", "/")) / "strand.test")
        self.strand_test = test if os.access(test, os.X_OK) else None
        self.results["strand_test"] = self.strand_test

    # --- plumbing ---

    def fixture(self, name, rewrite=None):
        src = HERE / "fixtures" / f"{name}.jsonl"
        entries = [json.loads(l) for l in src.read_text().splitlines() if l.strip()]
        for e in entries:
            if rewrite and e.get("frame"):
                rewrite(e["frame"])
        dst = self.out / "fixtures" / f"{name}.jsonl"
        dst.write_text("".join(json.dumps(e, separators=(",", ":")) + "\n" for e in entries))
        return dst

    def play(self, name, timescale=None, power=None, arrivals=False, rewrite=None):
        """Restart the surface on a fixture; return the arrivals file."""
        hooks = [f"HYAKKI_ONI_SCRIPT={self.fixture(name, rewrite)}"]
        if timescale:
            hooks.append(f"HYAKKI_ONI_TIMESCALE={timescale}")
        if power:
            hooks.append(f"HYAKKI_ONI_POWER={power}")
        path = None
        if arrivals:
            path = self.out / "arrivals" / f"{name}.jsonl"
            hooks.append(f"HYAKKI_ONI_ARRIVALS={path}")
        mark = self.ledger.mark()
        self.sh("ctl.sh", "surface", "restart", *hooks)
        self.wait(mark, 15, ev="commit")
        return path

    def perch_to(self, output, box):
        def rewrite(frame):
            if frame.get("kind") == "perch" and frame["data"].get("attached"):
                frame["data"]["output"] = output
                frame["data"]["box"] = {"x": box[0], "y": box[1], "w": box[2], "h": box[3]}
        return rewrite

    def lines(self):
        return self.ledger.since()

    def until(self, pred, timeout, what):
        line = self.ledger.wait(pred, timeout=timeout)
        if line is None:
            raise RuntimeError(f"ledger never showed {what} within {timeout} s")
        return line

    def ev(self, name, timeout, **conds):
        return self.until(lambda l: l.get("ev") == name and all(l.get(k) == v for k, v in conds.items()),
                          timeout, f"{name} {conds}")

    def phase(self, phase, timeout, seq=None):
        return self.until(lambda l: l.get("ev") == "commit" and l.get("part") == "strand" and l.get("strand")
                          and l["strand"]["phase"] == phase and (seq is None or l["strand"]["seq"] == seq),
                          timeout, f"a strand commit in phase {phase}")

    def strand_home(self, after, timeout):
        return self.until(lambda l: l.get("ev") == "hide" and l.get("part") == "strand" and l.get("t", 0) >= after,
                          timeout, "the strand unmapped")

    def wait_surface(self, t, timeout=60):
        """Sleep until the ledger's clock reaches t."""
        deadline = time.monotonic() + timeout
        while self.surface_now() < t - 0.004:
            if time.monotonic() > deadline:
                raise RuntimeError(f"the surface clock never reached {t}")
            time.sleep(0.004)

    def anchor(self, near=None):
        """The strand's root (output-local) from the step that stepped it
        nearest to NEAR (the newest when NEAR is None)."""
        steps = [l for l in self.lines() if l.get("strandEnv")]
        if not steps:
            raise RuntimeError("no step carried the strand's anchor")
        line = steps[-1] if near is None else min(steps, key=lambda l: abs(l.get("t", 0) - near))
        a = line["strandEnv"]["anchor"]
        return (a["X"], a["Y"])

    def where(self):
        """The compositor state, the character's output and its global rect."""
        state = self.state()
        commit = self.current()
        out = self.output(commit["output"], state)
        return state, out, global_rect(commit, state)

    def frac_box(self, frac, out):
        return [out["x"] + frac[0] * out["width"], out["y"] + frac[1] * out["height"],
                frac[2] * out["width"], frac[3] * out["height"]]

    def region(self, *rects, pad=40, out=None):
        x0 = min(r[0] for r in rects) - pad
        y0 = min(r[1] for r in rects) - pad
        x1 = max(r[0] + r[2] for r in rects) + pad
        y1 = max(r[1] + r[3] for r in rects) + pad
        if out:
            x0, y0 = max(x0, out["x"]), max(y0, out["y"])
            x1, y1 = min(x1, out["x"] + out["width"]), min(y1, out["y"] + out["height"])
        return [int(x0), int(y0), int(math.ceil(x1 - x0)), int(math.ceil(y1 - y0))]

    def strand_mask(self, png, base_png, rect, exclude=()):
        """Pixels that differ from the baseline, as a mask and as global
        logical points, with the EXCLUDE rects (the sprite) left out."""
        import numpy as np
        img, base = load_rgb(png), load_rgb(base_png)
        mask = np.abs(img - base).max(axis=2) > 24
        f = img.shape[1] / rect[2]
        for ex in exclude:
            x0, y0 = int(math.floor((ex[0] - rect[0]) * f)), int(math.floor((ex[1] - rect[1]) * f))
            x1, y1 = int(math.ceil((ex[0] + ex[2] - rect[0]) * f)), int(math.ceil((ex[1] + ex[3] - rect[1]) * f))
            mask[max(0, y0):max(0, y1), max(0, x0):max(0, x1)] = False
        return img, mask, mask_points(mask, rect, f), f

    def pixel_tip(self, img, mask, rect, f, anchor):
        """The tip the pixels show. The landing burst draws inked lines past
        the tip, so the thread is found by its white core: the colour of the
        thread's deepest pixels (those that survive the most erosion), then
        that colour's pixels eroded by 1 px, which drops the burst's 1 px
        cores and keeps the 3.2 px thread core. Its farthest pixel from the
        anchor lies on the core's round cap."""
        import numpy as np
        middle, k = mask, 0
        while k < 8:
            deeper = erode(mask, k + 1)
            if deeper.sum() < 20:
                break
            middle, k = deeper, k + 1
        if k == 0:
            return None, None
        white = np.median(img[middle], axis=0)
        core = erode(mask & (np.abs(img - white).max(axis=2) < 48), 1)
        pts = mask_points(core, rect, f)
        if len(pts) < 3:
            return None, None
        d = np.hypot(pts[:, 0] - anchor[0], pts[:, 1] - anchor[1])
        far = pts[int(np.argmax(d))]
        n = float(np.max(d))
        u = ((far[0] - anchor[0]) / n, (far[1] - anchor[1]) / n)
        # the core's cap radius, less about one pixel of erosion along the
        # larger direction component
        back = self.spec["strand"]["stroke"]["widthPx"] / 2 - max(abs(u[0]), abs(u[1]))
        return (float(far[0] - back * u[0]), float(far[1] - back * u[1])), (float(far[0]), float(far[1]))

    def replay(self, name):
        """Replay the running surface's ledger through the strand engine."""
        copy = self.out / "ledgers" / f"{name}.jsonl"
        copy.write_bytes(self.ledger.path.read_bytes())
        if not self.strand_test:
            return {"ran": False, "pass": False, "error": "no strand.test (set ONI_LAB_STRAND_TEST)"}
        unit = f"oni-lab-{self.env['LAB_ID']}-replay-{os.urandom(4).hex()}"
        r = subprocess.run(["systemd-run", "--user", "--scope", "--quiet", "--collect", f"--unit={unit}",
                            "-p", "MemoryMax=1G", "-p", "MemorySwapMax=0", "--",
                            "env", "-i", "PATH=/usr/bin:/bin", f"HYAKKI_ONI_REPLAY={copy}",
                            self.strand_test, "-test.run", "^TestLedgerReplay$", "-test.v", "-test.count=1"],
                           capture_output=True, text=True, timeout=300, cwd=self.out)
        out = (r.stdout + r.stderr).strip()
        return {"ran": True, "ledger": str(copy), "rc": r.returncode, "output": out[-1500:],
                "pass": r.returncode == 0 and "--- PASS: TestLedgerReplay" in out}

    def checks_pass(self, checks):
        return all(c["pass"] for c in checks)

    # --- step 0: census ---

    def census9(self):
        import numpy as np
        globals_ = json.loads(self.sh("ctl.sh", "globals").stdout)
        (self.out / "globals.json").write_text(json.dumps(globals_, indent=2))
        # the target straight above the character: the thread leaves the
        # head's bottom and runs up through the head, so whether it is drawn
        # over the sprite or under it shows in the head's own pixels
        state, out, sprite = self.where()
        cx = sprite[0] - out["x"] + sprite[2] / 2
        top = sprite[1] - out["y"] - 320
        frac = ((cx - 20) / out["width"], top / out["height"], 40 / out["width"], 30 / out["height"])

        def above(frame):
            if frame.get("kind") == "strand.target":
                frame["data"].update(x=round(frac[0], 5), y=round(frac[1], 5), w=round(frac[2], 5), h=round(frac[3], 5))
        self.play("strand-census", rewrite=above)
        state, out, sprite = self.where()
        box = self.frac_box(frac, out)
        rect = self.region(sprite, box, pad=60, out=out)
        base = self.shoot("census-baseline", rect)
        if any(l.get("ev") == "strand-target" for l in self.lines()):
            raise RuntimeError("the strand was already sent while the baseline was taken")
        self.phase("hold", 20)
        time.sleep(0.6)
        state, out, sprite = self.where()
        held = self.shoot("census-strand", rect)
        img, mask, pts, f = self.strand_mask(held, base, rect, [grow(sprite, 3)])
        outside = [p for p in pts if not inside(p, sprite)]
        beyond = max((max(sprite[0] - p[0], p[0] - sprite[0] - sprite[2], sprite[1] - p[1], p[1] - sprite[1] - sprite[3])
                      for p in outside), default=0)
        ours = [s for s in state["layers"] or [] if s["namespace"] == "hyakki-oni"]
        checks = [
            {"name": "layer shell and subcompositor bound", "value": sorted(globals_.get("bound", {})),
             "pass": "zwlr_layer_shell_v1" in globals_.get("bound", {}) and "wl_subcompositor" in globals_.get("bound", {})},
            {"name": "strand pixels drawn outside the layer surface", "value": len(outside), "pass": len(outside) >= 100},
            {"name": "strand reaches far past the layer surface (px)", "value": round(beyond, 1),
             "pass": beyond >= 0.8 * (sprite[1] - box[1] - box[3])},
        ]
        if state["layers"] is not None:
            # the layer surface stays at the sprite and, while the strand is
            # out, reaches the output's bottom-right corner: Hyprland squeezes
            # the part of a subsurface past its layer surface's right or
            # bottom edge into it (surface/origin.go layerSize); niri lists
            # layer surfaces without their geometry
            rects = [s["rect"] for s in ours if s.get("rect")]
            corner = (out["x"] + out["width"], out["y"] + out["height"])
            checks.append({"name": "one hyakki-oni layer at the sprite, reaching the output's bottom-right with the strand out",
                           "value": {"layers": ours, "sprite": sprite, "output_corner": corner},
                           "pass": len(ours) == 1 and all(r[:2] == sprite[:2] and r[0] + r[2] >= corner[0]
                                                          and r[1] + r[3] >= corner[1] for r in rects)})
        # z-order: along the thread's way through the head, the character's
        # own pixels must stay the character's, not turn the core's colour
        a = self.anchor()
        ag = (out["x"] + a[0], out["y"] + a[1])
        middle, k = mask, 0
        while k < 8 and erode(mask, k + 1).sum() >= 20:
            k += 1
            middle = erode(mask, k)
        white = np.median(img[middle], axis=0)
        bx, by = box[0] + box[2] / 2, box[1] + box[3] / 2
        n = dist(ag, (bx, by))
        u = ((bx - ag[0]) / n, (by - ag[1]) / n)
        base_img = load_rgb(base)
        band = set()
        s = 0.0
        while inside((ag[0] + s * u[0], ag[1] + s * u[1]), sprite):
            for w in range(-6, 7):
                gx, gy = ag[0] + s * u[0] - w * u[1], ag[1] + s * u[1] + w * u[0]
                band.add((int((gy - rect[1]) * f), int((gx - rect[0]) * f)))
            s += 0.5
        # the character's own pixels: unlike the desktop behind the sprite
        # (its corners are transparent) and unlike the core's colour
        sx0, sy0 = int((sprite[0] - rect[0]) * f), int((sprite[1] - rect[1]) * f)
        backdrop = base_img[max(0, sy0 + 1), max(0, sx0 + 1)]
        body = [(y, x) for y, x in band if 0 <= y < img.shape[0] and 0 <= x < img.shape[1]
                and np.abs(base_img[y, x] - white).max() > 60 and np.abs(base_img[y, x] - backdrop).max() > 40]
        flipped = [(y, x) for y, x in body if np.abs(img[y, x] - white).max() < 40]
        core_outside = int((mask & (np.abs(img - white).max(axis=2) < 40)).sum())
        checks.append({"name": "the strand is under the sprite: the head's pixels on its way stay the character's",
                       "value": {"anchor": ag, "core_rgb": white.tolist(), "path_through_sprite_px": round(s, 1),
                                 "character_pixels_on_the_way": len(body), "turned_core_colour": len(flipped),
                                 "core_pixels_outside_the_sprite": core_outside},
                       "pass": len(body) >= 30 and len(flipped) <= max(5, 0.03 * len(body)) and core_outside >= 100})
        # pointer: cross the thread far from the sprite and click it
        # the thread's middle; on a dark backdrop the ink edges may not
        # differ from it, leaving only the 3.2 px core in the mask
        k = 2 if erode(mask, 2).sum() >= 50 else 1 if erode(mask, 1).sum() >= 50 else 0
        cys, cxs = np.nonzero(erode(mask, k))
        far = [(rect[0] + (x + 0.5) / f, rect[1] + (y + 0.5) / f) for x, y in zip(cxs, cys)]
        far = [p for p in far if not inside(p, grow(sprite, 80)) and not inside(p, grow(box, 40))]
        if not far:
            raise RuntimeError("no strand core far from the sprite to cross")
        far.sort(key=lambda p: dist(p, ag))
        px, py = (round(c) for c in far[len(far) // 2])
        nx, ny = round(-u[1] * 3), round(u[0] * 3)
        self.sh("input.sh", "move", out["x"] + out["width"] // 2, out["y"] + out["height"] - 60)
        time.sleep(0.5)
        mark = self.ledger.mark()
        self.sh("input.sh", "ptr", f"a {px - 8 * nx} {py - 8 * ny}", *([f"m {nx} {ny}"] * 16), "w 200")
        self.sh("input.sh", "click", px, py)
        time.sleep(1.5)
        touched = [l for l in self.ledger.since(mark)
                   if l.get("ev") in ("drag", "drop", "gesture")
                   or (l.get("ev") == "step" and (l.get("input") or {}).get("pointer") is not None)]
        checks.append({"name": "pointer crossing and clicking the strand reaches nothing",
                       "value": {"point": [px, py], "lines": touched[:5]}, "pass": not touched})
        return {"globals": globals_, "region": rect, "sprite": sprite, "target": box,
                "crops": [str(base), str(held)], "checks": checks}, self.checks_pass(checks)

    # --- steps 1 and 3: reach, tap, hold, then rewind ---

    def reach(self):
        scale = 0.1
        self.play("strand-reach", timescale=scale)
        state, out, sprite = self.where()
        box = self.frac_box((0.74, 0.64, 0.03, 0.03), out)
        rect = self.region(sprite, box, pad=60, out=out)
        base = self.shoot("reach-baseline", rect)
        if any(l.get("ev") == "strand-target" for l in self.lines()):
            raise RuntimeError("the strand was already sent while the baseline was taken")
        self._reach = {"rect": rect, "base": base, "out": out}
        target = self.ev("strand-target", 20, seq=1)
        t0 = target["t"]
        samples, checks = [], []
        for ts in (0.0, 0.3, 0.74, 0.92):
            self.wait_surface(t0 + ts / scale)
            tb = self.surface_now()
            sprite_now = global_rect(self.current(), state)
            png = self.shoot(f"reach-{ts:.2f}", rect)
            ta = self.surface_now()
            near = [c for c in strand_commits(self.lines()) if c["strand"]["seq"] == 1]
            window = [c for c in near if tb - 0.05 <= c["t"] <= ta + 0.02] or \
                [c for c in near if c["t"] <= ta][-1:]
            track = [(out["x"] + x, out["y"] + y) for x, y in map(tip_of, window)]
            a = self.anchor(ta)
            ag = (out["x"] + a[0], out["y"] + a[1])
            img, mask, pts, f = self.strand_mask(png, base, rect, [grow(sprite_now, 3)])
            sample = {"t_s": ts, "surface_t": [tb, ta], "crop": str(png), "ledger_track": track,
                      "u": [c["strand"]["u"] for c in window], "phase": [c["strand"]["phase"] for c in window],
                      "strand_pixels": len(pts)}
            if len(pts) < 5:
                # the thread has not left the sprite yet: so must the ledger say
                ok = all(inside(p, grow(sprite_now, 6)) for p in track)
                sample.update(tip_px=None, ok=ok, note="no strand pixel outside the sprite")
            else:
                tip, far = self.pixel_tip(img, mask, rect, f, ag)
                d = math.inf if tip is None else polyline_dist(tip, track)
                sample.update(tip_px=tip, farthest_core_px=far, distance_px=round(d, 2), ok=d <= 2.0)
            samples.append(sample)
            checks.append({"name": f"t={ts} s: pixel tip on the ledger tip within 2 px",
                           "value": sample.get("distance_px", sample.get("note")), "pass": sample["ok"]})
        arrive = self.ev("strand-arrive", 30, seq=1)
        self.phase("hold", 10, seq=1)
        arrivals = [l for l in self.lines() if l.get("ev") == "strand-arrive"]
        checks.append({"name": "arrived once, at the end of the tap (0.92 s)",
                       "value": {"count": len(arrivals), "after_s": round((arrive["t"] - t0) * scale, 3)},
                       "pass": len(arrivals) == 1 and abs((arrive["t"] - t0) * scale - 0.92) <= 0.03})
        rep = self.replay("reach-hold")
        checks.append({"name": "the ledger replays through the strand engine", "value": rep, "pass": rep["pass"]})
        return {"target": box, "region": rect, "baseline": str(base), "samples": samples,
                "checks": checks}, self.checks_pass(checks)

    def rewind(self):
        scale = 0.1
        if not getattr(self, "_reach", None):
            raise RuntimeError("step 1 did not set up the reach")
        rect, base, out = self._reach["rect"], self._reach["base"], self._reach["out"]
        retract = self.ev("strand-retract", 40, seq=1)
        hide = self.strand_home(retract["t"], 20)
        time.sleep(1.0)
        state = self.state()
        sprite = global_rect(self.current(), state)
        png = self.shoot("rewind-final", rect)
        _, _, pts, _ = self.strand_mask(png, base, rect, [grow(sprite, 3)])
        time.sleep(3)
        after = [c for c in strand_commits(self.lines()) if c["t"] > hide["t"]]
        lasted = (hide["t"] - retract["t"]) * scale
        rep = self.replay("reach-rewind")
        checks = [
            {"name": "back to the hidden baseline outside the sprite (differing pixels)", "value": len(pts),
             "pass": len(pts) == 0},
            {"name": "the strand surface unmapped (hide home)", "value": hide, "pass": hide.get("reason") == "home"},
            {"name": "rewind took 0.52 s", "value": round(lasted, 3), "pass": abs(lasted - 0.52) <= 0.03},
            {"name": "no strand commit after the unmap", "value": len(after), "pass": not after},
            {"name": "the ledger replays through the strand engine", "value": rep, "pass": rep["pass"]},
        ]
        return {"crop": str(png), "checks": checks}, self.checks_pass(checks)

    # --- step 2: retarget ---

    def retarget(self):
        scale = 0.1
        arrivals = self.play("strand-retarget", timescale=scale, arrivals=True)
        state, out, sprite = self.where()
        first = self.ev("strand-target", 20, seq=1)
        second = self.ev("strand-target", 20, seq=2)
        mid = self.shoot("retarget-swing", self.region(sprite, self.frac_box((0.74, 0.62, 0.03, 0.03), out),
                                                         self.frac_box((0.8, 0.5, 0.03, 0.03), out), pad=60, out=out))
        arrive = self.ev("strand-arrive", 60, seq=2)
        lines = self.lines()
        commits = strand_commits(lines)
        before = [c for c in commits if c["t"] < second["t"]]
        during = [c for c in commits if second["t"] <= c["t"] <= arrive["t"]]
        u0 = before[-1]["strand"]["u"] if before else 0.0
        floor = max(self.spec["strand"]["retarget"]["reachFrom"], u0)
        phases = sorted({c["strand"]["phase"] for c in during})
        swing = [c for c in during if c["strand"]["phase"] == "swing"]
        after_swing = next((c for c in during if c["t"] > (swing[-1]["t"] if swing else math.inf)), None)
        lasted = ((after_swing["t"] if after_swing else math.nan) - second["t"]) * scale
        a = self.anchor(second["t"])
        tip_a = tip_of(before[-1]) if before else a
        tip_b = tip_of(during[-1]) if during else a
        closest = min((dist(tip_of(c), a) for c in swing), default=0.0)
        lowest = min((c["strand"]["u"] for c in during), default=0.0)
        seqs = [json.loads(l) for l in arrivals.read_text().splitlines()] if arrivals and arrivals.exists() else []
        checks = [
            {"name": "never goes home or rewinds between A and B", "value": phases,
             "pass": bool(during) and not {"home", "rewind"} & set(phases)},
            {"name": "u never drops below max(reachFrom, u at the retarget)",
             "value": {"u_at_retarget": round(u0, 4), "floor": round(floor, 4), "lowest": round(lowest, 4)},
             "pass": lowest >= floor - 1e-6},
            {"name": "the swing stays out: closest tip to the anchor (px)",
             "value": {"closest": round(closest, 1), "tip_a": round(dist(tip_a, a), 1), "tip_b": round(dist(tip_b, a), 1)},
             "pass": bool(swing) and closest >= 0.9 * min(dist(tip_a, a), dist(tip_b, a))},
            {"name": "swing completes in 300 ms", "value": round(lasted, 4), "pass": abs(lasted - 0.3) <= 0.02},
            {"name": "arrival only for B, once", "value": seqs,
             "pass": [s.get("seq") for s in seqs] == [2]},
        ]
        return {"first_t": first["t"], "second_t": second["t"], "crop": str(mid), "checks": checks}, self.checks_pass(checks)

    # --- step 4: perch ---

    def perch(self):
        state = self.state()
        win = next((w for w in state["windows"] if w.get("app_id") == "dev.ryoku.Hyakki"), None)
        commit = self.current()
        out = self.output(commit["output"], state)
        source = "compositor"
        if win and not win.get("pos") and self.wm == "niri" and win.get("column") == [1, 1] and win.get("offset"):
            # niri reports a tile's place only for floating windows. The lab's
            # app window is the one tiled column, which with
            # center-focused-column "never" starts one gap in from the
            # output's top-left corner.
            kdl = (self.lab / "wm/niri.kdl").read_text()
            gaps = next((float(l.split()[1]) for l in kdl.splitlines() if l.strip().startswith("gaps ")), None)
            if gaps is not None:
                win["pos"] = [out["x"] + gaps + win["offset"][0], out["y"] + gaps + win["offset"][1]]
                source = f"niri tiled column 1, gaps {gaps:g}"
        if not win or not win.get("pos") or not win.get("size"):
            raise RuntimeError(f"no position for the dev.ryoku.Hyakki window: {win}")
        gx, gy = win["pos"]
        if not (out["x"] <= gx < out["x"] + out["width"] and out["y"] <= gy < out["y"] + out["height"]):
            raise RuntimeError(f"the app window is not on the character's output {out['name']}")
        frame = [gx - out["x"], gy - out["y"], win["size"][0], win["size"][1]]
        self.play("strand-perch", rewrite=self.perch_to(out["name"], frame))
        target = self.ev("strand-target", 20, seq=1)
        hold = self.phase("hold", 20, seq=1)
        time.sleep(0.6)
        a = self.anchor(target["t"])
        tl, tr = (frame[0], frame[1]), (frame[0] + frame[2], frame[1])
        nearer = tl if dist(tl, a) < dist(tr, a) or (dist(tl, a) == dist(tr, a) and abs(tl[0] - out["width"] / 2) < abs(tr[0] - out["width"] / 2)) else tr
        other = tr if nearer == tl else tl
        curl = [c for c in strand_commits(self.lines()) if c["strand"]["phase"] == "curl" and c["strand"]["seq"] == 1]
        radius = self.spec["strand"]["perch"]["radiusPx"]
        ds = sorted(dist(tip_of(c), nearer) for c in curl)
        cg = (out["x"] + nearer[0], out["y"] + nearer[1])
        crop = self.shoot("perch-corner", self.region([cg[0], cg[1], 0, 0], pad=70, out=out))
        checks = [
            {"name": "the target carries the window frame", "value": target.get("target"),
             "pass": (target.get("target") or {}).get("frame") is not None},
            {"name": "curl frames drawn", "value": len(curl), "pass": len(curl) >= 3},
            {"name": f"curl tips {radius} px from the nearer top corner",
             "value": {"corner": nearer, "median": round(ds[len(ds) // 2], 2) if ds else None,
                       "min": round(ds[0], 2) if ds else None, "max": round(ds[-1], 2) if ds else None},
             "pass": bool(ds) and abs(ds[len(ds) // 2] - radius) <= 2 and ds[-1] <= radius + 4},
            {"name": "every curl tip nearer that corner than the other",
             "value": {"other": other},
             "pass": bool(curl) and all(dist(tip_of(c), nearer) < dist(tip_of(c), other) for c in curl)},
        ]
        return {"frame": frame, "frame_source": source, "output": out["name"], "anchor": a, "hold_t": hold["t"],
                "crop": str(crop),
                "checks": checks}, self.checks_pass(checks)

    # --- step 5: lasso ---

    def lasso(self):
        scale = 0.1
        self.play("strand-lasso", timescale=scale)
        state, out, sprite = self.where()
        box = self.frac_box((0.7, 0.5, 0.06, 0.06), out)
        rect = self.region(box, pad=60, out=out)
        base = self.shoot("lasso-baseline", rect)
        target = self.ev("strand-target", 20, seq=1)
        thrown_line = self.until(lambda l: l.get("ev") == "commit" and l.get("part") == "strand" and l.get("strand")
                                 and l["strand"]["phase"] == "lasso" and l["strand"]["u"] >= 0.999, 30, "the ring thrown")
        thrown = self.shoot("lasso-thrown", rect)
        cinched_line = self.phase("hold", 30, seq=1)
        time.sleep(0.5)
        cinched = self.shoot("lasso-cinched", rect)
        a = self.anchor(target["t"])
        ag = (out["x"] + a[0], out["y"] + a[1])
        cx, cy = box[0] + box[2] / 2, box[1] + box[3] / 2
        # measure on the two sides facing away from the character, which the
        # lead line never crosses
        sides = ["left" if ag[0] > cx else "right", "top" if ag[1] > cy else "bottom"]

        def extents(png):
            _, _, pts, _ = self.strand_mask(png, base, rect)
            ext = {}
            for side in sides:
                if side in ("left", "right"):
                    band = [p for p in pts if box[1] + 0.25 * box[3] <= p[1] <= box[1] + 0.75 * box[3]]
                    ext[side] = (box[0] - min(p[0] for p in band)) if side == "left" else \
                        (max(p[0] for p in band) - box[0] - box[2]) if band else None
                else:
                    band = [p for p in pts if box[0] + 0.25 * box[2] <= p[0] <= box[0] + 0.75 * box[2]]
                    ext[side] = (box[1] - min(p[1] for p in band)) if side == "top" else \
                        (max(p[1] for p in band) - box[1] - box[3]) if band else None
            return ext

        e_thrown, e_cinched = extents(thrown), extents(cinched)
        thrown_px = self.spec["strand"]["lasso"]["thrownPx"]
        checks = []
        for side in sides:
            d = None if e_thrown[side] is None or e_cinched[side] is None else e_thrown[side] - e_cinched[side]
            checks.append({"name": f"{side}: ring {thrown_px} px wider thrown than cinched",
                           "value": {"thrown": e_thrown[side], "cinched": e_cinched[side],
                                     "difference": None if d is None else round(d, 2)},
                           "pass": d is not None and abs(d - thrown_px) <= 2})
        return {"target": box, "thrown_t": thrown_line["t"], "cinched_t": cinched_line["t"],
                "crops": [str(base), str(thrown), str(cinched)], "checks": checks}, self.checks_pass(checks)

    # --- step 6: region pen ---

    def region_pen(self):
        self.play("strand-region")
        state, out, sprite = self.where()
        self.ev("strand-target", 20, seq=1)
        self.phase("region", 15, seq=1)
        mid = self.shoot("region-pen", self.region([out["x"] + 860, out["y"] + 380, 480, 270], pad=50, out=out))
        self.ev("strand-arrive", 20, seq=1)
        lines = self.lines()
        pens = [c for c in strand_commits(lines) if c["strand"]["phase"] == "region"]
        carried = [l for l in lines if l.get("ev") == "commit" and not l.get("part") and l.get("char")]
        r = self.spec["strand"]["region"]
        want = math.hypot(r["aheadPx"], r["outsidePx"])
        # a step logs the strand frame it computed, then the sprite's commit,
        # then the strand's commit: pair each carried sprite commit with the
        # strand of the step just before it
        pairs, step = [], None
        for line in lines:
            if line.get("ev") == "step" and line.get("strand"):
                step = line
            elif line in carried and step is not None and step["strand"]["phase"] == "region":
                ch = (line["char"]["x"], line["char"]["y"])
                pairs.append({"t": line["t"], "scale": line["char"]["scale"], "distance": dist(ch, tip_of(step))})
        scales = sorted({round(p["scale"], 4) for p in pairs})
        ds = sorted(p["distance"] for p in pairs)
        checks = [
            {"name": "pen frames with the character carried", "value": {"pen": len(pens), "carried": len(carried),
                                                                       "paired": len(pairs)},
             "pass": len(pairs) >= 10},
            {"name": f"character scale {r['penScale']}", "value": scales,
             "pass": bool(scales) and all(abs(s - r["penScale"]) <= 0.005 for s in scales)},
            {"name": f"character centre {r['aheadPx']} ahead and {r['outsidePx']} outside the tip ({want:.1f} px)",
             "value": {"min": round(ds[0], 2) if ds else None, "median": round(ds[len(ds) // 2], 2) if ds else None,
                       "max": round(ds[-1], 2) if ds else None},
             "pass": bool(ds) and ds[0] >= want - 0.5 and ds[-1] <= want + 0.5},
        ]
        return {"crop": str(mid), "checks": checks}, self.checks_pass(checks)

    # --- step 7: bubble ---

    def bubble(self):
        self.play("strand-bubble")
        state, out, sprite = self.where()
        self.until(lambda l: l.get("ev") == "bubble" and l.get("seq") == 5, 40, "the last bubble")
        bubbles = [l for l in self.lines() if l.get("ev") == "bubble"]
        checks, rows = [], []
        for b in bubbles:
            if b.get("box"):
                card, target = xywh(b["box"]), xywh((b.get("target") or {}).get("box") or b["box"])
                inside_out = card[0] >= 0 and card[1] >= 0 and card[0] + card[2] <= out["width"] and card[1] + card[3] <= out["height"]
                rows.append({"seq": b.get("seq"), "card": card, "target": target, "overlap": overlap(card, target),
                             "on_output": inside_out})
        checks.append({"name": "four bubbles placed", "value": len(rows), "pass": len(rows) == 4})
        checks.append({"name": "no bubble overlaps its target", "value": rows,
                       "pass": bool(rows) and all(r["overlap"] == 0 for r in rows)})
        checks.append({"name": "every bubble on the output", "value": [r["on_output"] for r in rows],
                       "pass": bool(rows) and all(r["on_output"] for r in rows)})
        last = bubbles[-1] if bubbles else {}
        checks.append({"name": "a bubble with no strand out is not placed", "value": last,
                       "pass": last.get("seq") == 5 and last.get("reason") == "no-strand"})
        self.results.setdefault("limits", []).append(
            {"step": 7, "check": "bubble crop", "reason": "the surface places the bubble but draws no text yet; "
             "the ledger rect is the evidence"})
        return {"bubbles": bubbles, "checks": checks}, self.checks_pass(checks)

    # --- step 8: arrival handshake against the fake daemon ---

    def arrival(self):
        scale = 0.1
        path = self.play("strand-arrival", timescale=scale, arrivals=True)
        state = self.state()
        app_out, here = self.app_output(state), self.current().get("output")
        self.ev("strand-arrive", 60, seq=6)
        self.ev("strand-retract", 60, seq=6)
        t7 = self.ev("strand-target", 60, seq=7)
        covered = None
        if app_out == here:
            self.sh("ctl.sh", "fullscreen", "on")
            covered = self.until(lambda l: l.get("ev") == "strand-skip" and l.get("seq") == 7, 15, "seq 7 skipped")
            self.sh("ctl.sh", "fullscreen", "off")
        else:
            self.results.setdefault("limits", []).append(
                {"step": 8, "check": "skipped when covered",
                 "reason": f"the app window is on {app_out}, the character on {here}"})
        self.ev("strand-target", 60, seq=8)
        time.sleep(4)
        rows = [json.loads(l) for l in path.read_text().splitlines()] if path.exists() else []
        by = {}
        for r in rows:
            by.setdefault(r.get("seq"), []).append(r)
        ledger_arrive = [l.get("seq") for l in self.lines() if l.get("ev") == "strand-arrive"]
        checks = [
            # "advanced" is the fake daemon's own bookkeeping: it awaits the
            # last target it sent, which here is the stale 4 the surface ignores
            {"name": "seq 6 arrived exactly once, not skipped", "value": by.get(6),
             "pass": len(by.get(6, [])) == 1 and not by[6][0].get("skipped")},
            {"name": "the superseded seq 5 and the stale seq 4 never arrive", "value": {"5": by.get(5), "4": by.get(4)},
             "pass": not by.get(5) and not by.get(4)},
            {"name": "the ledger arrived once (seq 6)", "value": ledger_arrive, "pass": ledger_arrive == [6]},
            {"name": "nothing for seq 8, cut by EOF mid-reach", "value": by.get(8), "pass": not by.get(8)},
        ]
        if covered is not None:
            checks.append({"name": "seq 7 reported once, skipped covered", "value": {"ledger": covered, "arrivals": by.get(7)},
                           "pass": covered.get("skipped") == "covered" and len(by.get(7, [])) == 1
                           and by[7][0].get("skipped") == "covered"})
        return {"arrivals": rows, "app_output": app_out, "character_output": here,
                "checks": checks}, self.checks_pass(checks)

    # --- step 9: reduced motion ---

    def reduced9(self):
        path = self.play("strand-reduced", arrivals=True)
        self.ev("strand-retract", 40, seq=3)
        self.strand_home(0, 10)
        time.sleep(3)
        lines = self.lines()
        events = [l for l in lines if l.get("ev") in ("strand-target", "strand-retract")]
        segments = []
        for i, e in enumerate(events):
            end = events[i + 1]["t"] if i + 1 < len(events) else math.inf
            seg = [l for l in lines if e["t"] <= l.get("t", 0) < end]
            commits = len(strand_commits(seg))
            arrive = next((l for l in seg if l.get("ev") == "strand-arrive"), None)
            hide = next((l for l in seg if l.get("ev") == "hide" and l.get("part") == "strand"), None)
            segments.append({"ev": e["ev"], "seq": e.get("seq"), "t": e["t"], "strand_commits": commits,
                             "arrive_after_s": None if arrive is None else round(arrive["t"] - e["t"], 4),
                             "unmapped": hide is not None})
        rows = [json.loads(l) for l in path.read_text().splitlines()] if path.exists() else []
        targets = [s for s in segments if s["ev"] == "strand-target"]
        retracts = [s for s in segments if s["ev"] == "strand-retract"]
        checks = [
            {"name": "three targets and three retracts", "value": [len(targets), len(retracts)],
             "pass": len(targets) == 3 and len(retracts) == 3},
            {"name": "exactly one strand commit per target", "value": [s["strand_commits"] for s in targets],
             "pass": all(s["strand_commits"] == 1 for s in targets)},
            {"name": "arrival at once (same step)", "value": [s["arrive_after_s"] for s in targets],
             "pass": all(s["arrive_after_s"] is not None and s["arrive_after_s"] <= 0.05 for s in targets)},
            {"name": "a retract commits at most one frame and unmaps",
             "value": [(s["strand_commits"], s["unmapped"]) for s in retracts],
             "pass": all(s["strand_commits"] <= 1 and s["unmapped"] for s in retracts)},
            {"name": "the fake daemon heard one arrival per seq", "value": [r.get("seq") for r in rows],
             "pass": sorted(r.get("seq") for r in rows) == [1, 2, 3]},
        ]
        return {"segments": segments, "checks": checks}, self.checks_pass(checks)

    # --- step 10: settlement ---

    def settlement(self):
        self.play("strand-hold")
        self.ev("strand-target", 20, seq=1)
        self.phase("hold", 20, seq=1)
        held = self.measure("strand-held", timeout=120, seconds=60)
        retract = self.ev("strand-retract", 200, seq=1)
        self.strand_home(retract["t"], 20)
        gone = self.measure("strand-retracted", timeout=120, seconds=45)
        checks = [{"name": "strand-held measurement", "value": held.get("ledger_window"), "pass": bool(held.get("pass"))},
                  {"name": "strand-retracted measurement", "value": gone.get("ledger_window"), "pass": bool(gone.get("pass"))}]
        return {"checks": checks}, self.checks_pass(checks)

    # --- step 11: frame tier on battery saver ---

    def tier(self):
        self.play("strand-tier", power="saver")
        target = self.ev("strand-target", 20, seq=1)
        arrive = self.ev("strand-arrive", 20, seq=1)
        lines = self.lines()
        sprite_before = [l for l in lines if l.get("ev") == "commit" and not l.get("part") and l["t"] < target["t"]]
        reach = [c for c in strand_commits(lines) if target["t"] <= c["t"] <= arrive["t"]]

        def median_gap(cs):
            gaps = sorted(b["t"] - a["t"] for a, b in zip(cs, cs[1:]))
            return round(gaps[len(gaps) // 2] * 1000, 2) if gaps else None

        b_fps = sorted({l.get("fps") for l in sprite_before})
        r_fps = sorted({c.get("fps") for c in reach})
        b_gap, r_gap = median_gap(sprite_before[-60:]), median_gap(reach)
        checks = [
            {"name": "breathing at 30 fps before the reach", "value": {"fps": b_fps, "median_gap_ms": b_gap},
             "pass": b_fps == [30] and b_gap is not None and b_gap >= 30},
            {"name": "strand frames at 60 fps", "value": {"fps": r_fps, "median_gap_ms": r_gap, "frames": len(reach)},
             "pass": r_fps == [60] and r_gap is not None and r_gap <= 20},
        ]
        return {"checks": checks}, self.checks_pass(checks)

    # --- step 12: sound through a private PipeWire graph ---

    def audio(self):
        import numpy as np
        self.sh("audio.sh", "up", timeout=120)
        graph = json.loads((self.lab / "log/audio-graph.json").read_text())
        (self.out / "audio" / "graph.json").write_text(json.dumps(graph, indent=2))
        wav = self.out / "audio" / "strand-audio.wav"
        self.sh("audio.sh", "record", wav)
        state = self.state()
        commit = self.current()
        out = self.output(commit["output"], state)
        recording_started = time.monotonic()
        # the watch reads the new surface's ledger, so it starts after the
        # restart; the fixture plays nothing for its first seconds
        self.play("strand-audio", rewrite=self.perch_to(out["name"], [100, 100, 1280, 800]))
        watch = PlayerWatch(self.env["LAB_ID"], self.ledger.path)
        watch.start()
        try:
            t6 = self.ev("strand-target", 80, seq=6)
            time.sleep(6)
        finally:
            watch.done.set()
            watch.join(timeout=30)
            self.sh("audio.sh", "stop")
        recorded_wall = time.monotonic() - recording_started
        lines = self.lines()
        (self.out / "ledgers" / "audio.jsonl").write_bytes(self.ledger.path.read_bytes())
        pcm, rate = read_wav_f32(wav)
        synth, motifs = self.spec["sound"]["synth"], self.spec["sound"]["motifs"]
        gating = self.spec["sound"]["gating"]
        sounds = [l for l in lines if l.get("ev") == "sound"]
        played = [s for s in sounds if s.get("reason") == "played"]
        segs = sound_segments(pcm, rate)
        voices = [l for l in lines if l.get("ev") == "frame" and l.get("kind") == "voice"]
        motions = [l for l in lines if l.get("ev") == "frame" and l.get("kind") == "motion"]
        target = {l["seq"]: l for l in lines if l.get("ev") == "strand-target"}
        retract = {l["seq"]: l for l in lines if l.get("ev") == "strand-retract"}
        checks = []

        def note_env(a_s, b_s, note):
            """The 30 ms envelope of one note's frequency over [a_s, b_s).
            30 ms, not shorter: a sound that replaces another can start on a
            note two semitones from the one it cuts (reach 62, settle 64),
            and a 10 ms box lets 0.8 of the old note through; at 30 ms it is
            under 0.1. The half-peak crossing of a centred box stays on the
            onset whatever its length."""
            a, b = max(0, int(a_s * rate)), max(0, int(b_s * rate))
            x = pcm[a:b].astype(np.float64)
            tt = np.arange(len(x)) / rate
            n = int(0.03 * rate)
            return np.abs(np.convolve(x * np.exp(-2j * np.pi * midi_hz(note) * tt), np.ones(n) / n, mode="same"))

        def rise(a_s, b_s, note):
            """When a note first reaches half its peak in [a_s, b_s)."""
            env = note_env(a_s, b_s, note)
            return a_s + int(np.argmax(env >= 0.5 * env.max())) / rate if len(env) and env.max() > 0 else None

        # The recording's clock drifts from the ledger's by up to about 1 %
        # (measured), and each pw-cat starts a few tens of ms late, so sounds
        # are found one after another: each where its own segment starts (or,
        # inside a run where a new sound replaced the one playing, where its
        # first note rises) within 150 ms of the ledger time plus its delay
        # on the clock the previous sound left. A segment start belongs to one
        # sound: two sounds close together (a flight's land replacing the
        # reach that began under it) do not both claim it. Delays are then
        # checked against the next sound nearby.
        onsets, preds, lag, claimed = [], [], 0.0, set()
        if played and segs:
            m0 = motifs[played[0]["motif"]]
            first_rise = rise(segs[0][0] - 0.02, segs[0][0] + 0.1, m0["notes"][0])
            lag = (first_rise - segs[0][0]) if first_rise is not None else 0.0
            off = segs[0][0] - (played[0]["t"] + m0["delayS"])
            for s in played:
                m = motifs[s["motif"]]
                p = s["t"] + m["delayS"] + off
                cand = [g[0] for g in segs if abs(g[0] - p) <= 0.15 and g[0] not in claimed]
                if cand:
                    o = min(cand, key=lambda x: abs(x - p))
                    claimed.add(o)
                else:
                    r_ = rise(p - 0.15, p + 0.15, m["notes"][0])
                    o = None if r_ is None else r_ - lag
                onsets.append(o)
                preds.append(p)
                if o is not None:
                    off = o - (s["t"] + m["delayS"])
        paired = bool(played) and len(onsets) == len(played) and all(o is not None for o in onsets)
        fit = None
        if paired:
            xs = [s["t"] + motifs[s["motif"]]["delayS"] for s in played]
            slope, intercept = np.polyfit(xs, onsets, 1) if len(xs) > 1 else (1.0, onsets[0] - xs[0])
            fit = (float(intercept), float(slope))

        def to_rec(t):
            """A ledger time on the recording, interpolated between sounds."""
            xs = [s["t"] + motifs[s["motif"]]["delayS"] for s in played]
            if t <= xs[0]:
                return onsets[0] + (t - xs[0])
            if t >= xs[-1]:
                return onsets[-1] + (t - xs[-1])
            return float(np.interp(t, xs, onsets))

        # A new sound replaces the one playing the moment its player starts:
        # its delay is leading silence in the same stream (sound.Player,
        # Gate.Track). So a sound is replaced when the next one's player
        # (its onset less its delay) starts before this one has finished; it
        # has no length of its own, and only its notes that had 30 ms before
        # that start are checked. The cut sound's queued audio can run on 10
        # to 60 ms past it (measured), so later notes may be partly heard and
        # are not held to pitch or spacing.
        rows = []
        for i, s in enumerate(played):
            m = motifs[s["motif"]]
            row = {"t": s["t"], "event": s.get("event"), "motif": s["motif"], "notes": m["notes"]}
            if paired:
                start = onsets[i]
                row["onset_error_ms"] = round((start - preds[i]) * 1000, 1)
                own = next((g for g in segs if abs(g[0] - start) <= 0.015), None)
                row["recorded"] = [round(start, 4), None if own is None else round(own[1], 4)]
                core = (len(m["notes"]) - 1) * m["spacingS"] + m["lengthS"]
                natural = own[1] if own is not None else start + core + synth["releaseS"]
                nxt = None
                if i + 1 < len(played):
                    nxt = onsets[i + 1] - motifs[played[i + 1]["motif"]]["delayS"]
                end = nxt if nxt is not None and nxt < natural else None
                if end is not None:
                    row["replaced_by_next"] = True
                elif own is not None:
                    row["duration_s"] = round(own[1] - own[0], 4)
                    row["duration_ok"] = core - 0.01 <= own[1] - own[0] <= core + synth["releaseS"] + 0.05
                heard = [k for k in range(len(m["notes"])) if end is None or start + k * m["spacingS"] + 0.03 <= end]
                if end is not None:
                    row["heard_notes"] = len(heard)
                cents = []
                for k in heard:
                    note = m["notes"][k]
                    a_s = start + k * m["spacingS"]
                    b_s = a_s + (m["spacingS"] if k + 1 < len(m["notes"]) else m["lengthS"])
                    if end is not None:
                        b_s = min(b_s, end)
                    a, b = int((a_s + 0.004) * rate), int((b_s - 0.002) * rate)
                    hz = peak_hz(pcm[a:b].astype(np.float64), rate)
                    cents.append(round(1200 * math.log2(hz / midi_hz(note)), 1))
                row["pitch_cents"] = cents
                starts = [rise(start - 0.01 + k * m["spacingS"], start + k * m["spacingS"] + m["spacingS"] - 0.005,
                               m["notes"][k]) for k in heard]
                row["spacing_ms"] = [None if a is None or b is None else round((b - a) * 1000, 1)
                                     for a, b in zip(starts, starts[1:])]
                stop = start + core if end is None else min(start + core, end)
                row["peak"] = round(float(np.max(np.abs(pcm[int(start * rate):max(int(stop * rate), int(start * rate) + 1)]))), 5)
            rows.append(row)
        checks.append({"name": "every played sound found in the recording", "value": {"played": len(played),
                                                                                      "segments": len(segs)},
                       "pass": paired})
        reached = {s["motif"] for s in played}
        checks.append({"name": "strand motifs played: reach, arrive, rewind, bubble, perch",
                       "value": sorted(reached), "pass": {"reach", "arrive", "rewind", "bubble", "perch"} <= reached})
        # the reach's 0.36 s delay, against the arrive that follows it: two
        # sounds under a second apart share the recording's clock
        delays = []
        for i, r_ in enumerate(rows):
            nxt = rows[i + 1] if i + 1 < len(rows) else None
            if paired and r_["motif"] == "reach" and nxt and nxt["motif"] == "arrive" and nxt["t"] - r_["t"] < 2:
                d = (onsets[i] - onsets[i + 1]) - (played[i]["t"] - played[i + 1]["t"])
                delays.append(round(d, 4))
        reach_delay = motifs["reach"]["delayS"]
        # measured, not specified: the reach's pw-cat reaches the graph 40 to
        # 60 ms later than the arrive's, so the bound is the delay and at
        # most 80 ms more; the numbers are kept
        checks.append({"name": f"reach starts at least {reach_delay} s after its event and at most 80 ms later "
                               "(against the arrive after it)",
                       "value": delays,
                       "pass": bool(delays) and all(reach_delay - 0.01 <= d <= reach_delay + 0.08 for d in delays)})
        checks.append({"name": "each sound starts within 150 ms of the clock the sound before it left",
                       "value": {"drift_ppm": None if fit is None else round((fit[1] - 1) * 1e6, 1),
                                 "errors_ms": [(r["motif"], r.get("onset_error_ms")) for r in rows]},
                       "pass": paired and all(abs(r["onset_error_ms"]) <= 150 for r in rows)})
        checks.append({"name": "notes start at the motif's spacing, within 5 ms",
                       "value": [(r["motif"], r.get("spacing_ms"), round(motifs[r["motif"]]["spacingS"] * 1000, 1)) for r in rows],
                       "pass": paired and all(sp is not None and abs(sp - motifs[r["motif"]]["spacingS"] * 1000) <= 5
                                              for r in rows for sp in r["spacing_ms"])})
        pitched = [r for r in rows if "pitch_cents" in r]
        checks.append({"name": "every note at its MIDI pitch within 50 cents",
                       "value": [(r["motif"], r["pitch_cents"]) for r in pitched],
                       "pass": len(pitched) == len(rows) and all(abs(c) <= 50 for r in pitched for c in r["pitch_cents"])})
        checks.append({"name": "each sound lasts its notes plus at most the release",
                       "value": [(r["motif"], r.get("duration_s")) for r in pitched],
                       "pass": bool(pitched) and any("duration_s" in r for r in pitched)
                       and all(r.get("duration_ok") for r in pitched if "duration_s" in r
                               and not (r["event"] == "strand-reach" and 3 in target
                                        and abs(r["t"] - target[3]["t"]) < 0.1))})
        peak_max = synth["master"] * synth["peak"]
        checks.append({"name": f"peak at most master x peak ({peak_max:.3f})",
                       "value": [r.get("peak") for r in pitched],
                       "pass": bool(pitched) and all(r["peak"] <= peak_max + 1e-3 for r in pitched)})

        def silent(t_from, t_to):
            if fit is None:
                return None
            a, b = int(max(0.0, to_rec(t_from)) * rate), int(max(0.0, to_rec(t_to)) * rate)
            return float(np.max(np.abs(pcm[a:b]))) if b > a else 0.0

        # off: before the motion frame that turns sounds on
        on = motions[1]["t"] if len(motions) > 1 else None
        early = [s for s in sounds if on is not None and s["t"] < on]
        checks.append({"name": "sounds off: nothing played or recorded before the setting",
                       "value": {"ledger": early, "recorded_peak": None if on is None else silent(0, on)},
                       "pass": on is not None and all(s.get("reason") != "played" for s in early) and silent(0, on) == 0.0})
        # mic live: listening after target 3, speaking..idle after retract 3
        if 3 in target and 3 in retract:
            listening = next((v for v in voices if v["t"] > target[3]["t"]), None)
            reopened = next((v for v in voices if listening and v["t"] > listening["t"]), None)
            speak = [v for v in voices if v["t"] > retract[3]["t"]][:3]
            live = [(listening["t"], reopened["t"])] if listening and reopened else []
            if len(speak) == 3:
                live.append((speak[0]["t"], speak[2]["t"]))
            during = [s for s in sounds for a, b in live if a <= s["t"] < b]
            # the reach cut by mic-open, on its own clock (its onset less its
            # delay), so pw-cat's start jitter does not count against the cut
            reach3 = next((i for i, s in enumerate(played)
                           if s.get("event") == "strand-reach" and abs(s["t"] - target[3]["t"]) < 0.1), None)
            cut = {}
            if reach3 is not None and listening and paired and rows[reach3]["recorded"][1] is not None:
                seg = tuple(rows[reach3]["recorded"])
                own = seg[0] - (played[reach3]["t"] + motifs["reach"]["delayS"])
                v_rec = listening["t"] + own
                win = int(0.005 * rate)
                env = np.sqrt(np.convolve(pcm.astype(np.float64) ** 2, np.ones(win) / win, mode="same"))
                ref = float(np.median(env[int((v_rec - 0.03) * rate):int((v_rec - 0.005) * rate)]))
                k = int(v_rec * rate)
                fade = next((i for i in range(k, int(seg[1] * rate) + 1) if env[i:i + win].max() < 0.5 * ref), None)
                cut = {"mic_open_in_recording_s": round(v_rec, 4), "sound": seg,
                       "fade_below_half_ms": None if fade is None else round((fade / rate - v_rec) * 1000, 1),
                       "silent_after_ms": round((seg[1] - v_rec) * 1000, 1)}
            checks.append({"name": "silent while the mic is live (listening, speaking, transcribing)",
                           "value": {"windows": live, "ledger": during,
                                     "recorded_peak": [silent(a + 0.06, b) for a, b in live]},
                           "pass": len(live) == 2 and all(s.get("reason") != "played" for s in during)
                           and all((silent(a + 0.06, b) or 0) == 0.0 for a, b in live)})
            limit_fade = gating["quietMs"] + 10
            limit_silent = gating["quietMs"] + gating["muteRampMs"] + 10
            checks.append({"name": f"mic-open cuts a playing sound: below half within {limit_fade} ms, silent within {limit_silent} ms",
                           "value": cut,
                           "pass": bool(cut) and cut["fade_below_half_ms"] is not None
                           and cut["fade_below_half_ms"] <= limit_fade and cut["silent_after_ms"] <= limit_silent})
        else:
            checks.append({"name": "mic-live windows found", "value": None, "pass": False})
        # link loss: EOF two seconds after target 6
        eof = t6["t"] + 2.0
        lost = [s for s in sounds if s["t"] >= eof]
        end = (len(pcm) / rate - fit[0]) / fit[1] if fit else eof
        checks.append({"name": "silent after the link is lost",
                       "value": {"ledger": lost, "recorded_peak": silent(eof, end)},
                       "pass": all(s.get("reason") != "played" for s in lost) and silent(eof, end) == 0.0})
        # player processes: one per played sound, each only around its sound
        w_off = watch.offset()
        procs = []
        for pid, p in sorted(watch.players.items(), key=lambda kv: kv[1]["first"]):
            t_first, t_last = (p["first"] - w_off, p["last"] - w_off) if w_off is not None else (None, None)
            near = [s for s in played if t_first is not None and s["t"] - 0.2 <= t_first <= s["t"] + 0.5]
            procs.append({"pid": pid, "comm": p["comm"], "ledger_t": [t_first, t_last], "env": p["env"],
                          "scope": p["scope"], "private_client": p["private_client"], "sound": near[:1]})
        checks.append({"name": "one player process per played sound, started with it",
                       "value": [(p["comm"], p["ledger_t"], bool(p["sound"])) for p in procs],
                       "pass": len(procs) == len(played) and all(p["sound"] for p in procs)})
        checks.append({"name": "no player lives past its sound by more than the 2 s reap",
                       "value": [round(p["ledger_t"][1] - p["sound"][0]["t"], 3) for p in procs if p["sound"]],
                       "pass": all(p["sound"] and p["ledger_t"][1] - p["sound"][0]["t"] <= 0.36 + 1.0 + 2.0 for p in procs)})
        checks.append({"name": "players are pw-cat (never aplay), pointed only at the lab's runtime directory",
                       "value": [(p["comm"], p["env"]) for p in procs],
                       "pass": bool(procs) and all(p["comm"] in ("pw-cat", "paplay")
                                                   and p["env"].get("XDG_RUNTIME_DIR") == str(self.lab / "rt")
                                                   and all(v.startswith(str(self.lab)) for k, v in p["env"].items()
                                                           if k != "XDG_RUNTIME_DIR")
                                                   for p in procs)})
        matched = [p for p in procs if p["private_client"]]
        checks.append({"name": "the private graph holds the players' clients",
                       "value": [(p["pid"], p["private_client"]) for p in procs[:6]],
                       "pass": bool(matched) and all(m["private_client"]["binary"] == m["comm"] for m in matched)})
        devices = [n for n in graph.get("sinks", []) + graph.get("sources", [])
                   if (n.get("properties") or {}).get("device.api")]
        checks.append({"name": "the graph has no device (no ALSA, Bluetooth or other hardware)",
                       "value": [n.get("name") for n in devices], "pass": not devices})
        return {"wav": str(wav), "rate": rate, "seconds": round(len(pcm) / rate, 2),
                "recorded_wall_s": round(recorded_wall, 2), "clock": fit,
                "segments": [(round(a, 4), round(b, 4)) for a, b in segs], "sounds": sounds, "rows": rows,
                "players": procs, "checks": checks}, self.checks_pass(checks)

    def run(self):
        self.step(0, "census", self.census9)
        self.step(1, "reach, tap, hold", self.reach)
        self.step(3, "rewind", self.rewind)
        self.step(2, "retarget", self.retarget)
        self.step(4, "perch", self.perch)
        self.step(5, "lasso", self.lasso)
        self.step(6, "region pen", self.region_pen)
        self.step(7, "bubble", self.bubble)
        if any(w.get("app_id") == "dev.ryoku.Hyakki" for w in self.state()["windows"]):
            self.step(8, "arrival", self.arrival)
        else:
            self.results["steps"].append({"step": 8, "name": "arrival", "pass": False,
                                          "error": "no dev.ryoku.Hyakki window (up.sh --app)"})
        self.step(9, "reduced motion", self.reduced9)
        self.step(10, "settlement", self.settlement)
        self.step(11, "frame tier", self.tier)
        self.step(12, "audio", self.audio)
        self.step(13, "ledger display is the lab's", self.display_check)
        self.results["finished_at"] = now()
        self.save()
        for limit in self.results.get("limits", []):
            print(f"limit (step {limit['step']}): {limit['check']}: {limit['reason']}")
        return 0 if self.results["pass"] else 1


# --- phase 10 ---------------------------------------------------------------

APP_ID = "dev.ryoku.Hyakki"
FIXTURE_APPS = {"notes": "dev.ryoku.OniFixture", "notes-tiled": "dev.ryoku.OniFixtureTiled",
                "list": "dev.ryoku.OniFixtureList"}
CAPTURE_PATHS = ("toplevel", "niri-window", "output-crop", "portal", "x11")
# what an input executor is called; no read-only Oni package may name one
INPUT_MARKS = ("wtype", "ydotool", "dotool", "uinput", "zwlr_virtual_pointer", "zwp_virtual_keyboard",
               "virtual_pointer", "virtual-pointer", "virtual_keyboard", "virtual-keyboard")
INPUT_PROGRAMS = ("wtype", "ydotool", "dotool", "xdotool")
PRIVATE_REFUSAL = "That field holds private details, so I am leaving it to you."
QUESTION_POINT = "Where do I save these notes?"
QUESTION_ABOUT = "What is this window for?"
QUESTION_SECRET = "Where do I type my password?"


def argv_allowed(argv):
    """The read verbs the daemon may run (window package allowlist) and grim
    writing to stdout (the capture service); nothing else."""
    if not argv:
        return False
    prog, rest = os.path.basename(argv[0]), list(argv[1:])
    if prog == "niri":
        return len(rest) == 3 and rest[:2] == ["msg", "--json"] and rest[2] in (
            "windows", "outputs", "workspaces", "event-stream")
    if prog == "hyprctl":
        return len(rest) == 2 and rest[0] == "-j" and rest[1] in ("clients", "monitors", "cursorpos", "layers")
    if prog == "ryoku":
        if rest[:2] in (["wm", "state"], ["wm", "caps"]):
            return len(rest) == 2
        topics = rest[2:]
        return rest[:2] == ["wm", "watch"] and len(set(topics)) == len(topics) and all(
            t in ("windows", "outputs", "workspaces", "focus") for t in topics)
    if prog == "grim":
        flags = {"-T", "-g", "-o", "-t", "-q", "-l", "-s", "-c"}
        i = 0
        while i < len(rest) - 1:
            if rest[i] not in flags:
                return False
            i += 1 if rest[i] == "-c" else 2
        return bool(rest) and rest[-1] == "-"
    return False


def under_daemon(pid):
    up, hops = ppid_of(pid), 0
    while up and up > 1 and hops < 8:
        if comm_of(up) in ONI_COMMS:
            return up
        up, hops = ppid_of(up), hops + 1
    return None


class ChildWatch(threading.Thread):
    """Records every compositor, capture or input program the oni daemon
    starts during the run, with its argv: step 12 holds them to the
    allowlist. A watcher cannot see a process shorter than its period, so
    it also keeps the count of passes."""

    def __init__(self, lab_id, period=0.05):
        super().__init__(daemon=True)
        self.lab_id, self.period = lab_id, period
        self.done = threading.Event()
        self.seen, self.passes = {}, 0
        # pids found already exited (a zombie's cmdline reads empty): no
        # argv to judge, but the program name still counts for step 12
        self.unread = {}

    def run(self):
        watched = COMPOSITOR_CHILDREN + INPUT_PROGRAMS
        daemons, found_at = [], 0.0
        while not self.done.is_set():
            self.passes += 1
            # the daemons are looked up once a second (a /proc scan) and
            # their descendants every pass, through the kernel's children
            # lists; every half second a full scan by parentage as well, in
            # case the non-dumpable daemon's lists cannot be read
            if time.monotonic() - found_at >= 1.0:
                daemons, found_at = lab_pids(self.lab_id, "oni"), time.monotonic()
            pids = {pid: d for d in daemons for pid in descendants(d)}
            if self.passes % 10 == 0:
                for pid in lab_pids(self.lab_id):
                    if pid not in pids and comm_of(pid) in watched:
                        up = under_daemon(pid)
                        if up is not None:
                            pids[pid] = up
            for pid, daemon in pids.items():
                if pid in self.seen or comm_of(pid) not in watched:
                    continue
                try:
                    argv = [a.decode("utf-8", "replace")
                            for a in open(f"/proc/{pid}/cmdline", "rb").read().split(b"\0")[:-1]]
                except OSError:
                    continue
                if not argv:
                    self.unread[pid] = comm_of(pid)
                    continue
                self.seen[pid] = {"pid": pid, "daemon": daemon, "comm": comm_of(pid), "argv": argv, "t": now()}
            self.done.wait(self.period)


def descendants(pid):
    """Every process below PID, from /proc/PID/task/*/children."""
    out, stack = [], [pid]
    while stack:
        p = stack.pop()
        try:
            tasks = os.listdir(f"/proc/{p}/task")
        except OSError:
            continue
        for t in tasks:
            try:
                kids = [int(k) for k in open(f"/proc/{p}/task/{t}/children").read().split()]
            except (OSError, ValueError):
                continue
            out += kids
            stack += kids
    return out


class LabModel:
    """The lab's vision fixture (model.py) through its control routes."""

    def __init__(self, url):
        import urllib.request
        self.url = url
        self.opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

    def _req(self, path, body=None, timeout=10):
        import urllib.request
        data = None if body is None else json.dumps(body).encode()
        req = urllib.request.Request(self.url + path, data, {"Content-Type": "application/json"} if data else {})
        with self.opener.open(req, timeout=timeout) as r:
            return r.read()

    def requests(self, timeout=10):
        return json.loads(self._req("/_lab/requests", timeout=timeout))

    def reset(self):
        self._req("/_lab/reset", {})

    def mode(self, mode, delay=0):
        self._req("/_lab/mode", {"mode": mode, "delay": delay})

    def image(self, n, k=0):
        return self._req(f"/_lab/image/{n}/{k}")


class Watch:
    """`hyakki oni watch` in the lab, collecting every oni frame line."""

    def __init__(self, lab_id):
        self.lab_id, self.lines = lab_id, []
        self.proc = subprocess.Popen([str(HERE / "ctl.sh"), "hyakki", "oni", "watch"], stdout=subprocess.PIPE,
                                     stderr=subprocess.DEVNULL, stdin=subprocess.DEVNULL, text=True)
        self.thread = threading.Thread(target=self._read, daemon=True)
        self.thread.start()

    def _read(self):
        for raw in self.proc.stdout:
            try:
                self.lines.append({"wall": time.time(), "frame": json.loads(raw)})
            except ValueError:
                continue

    def frames(self, kind):
        return [l for l in self.lines if (l["frame"] or {}).get("kind") == kind]

    def stop(self):
        for pid in lab_pids(self.lab_id, "hyakki"):
            try:
                if b"watch" in open(f"/proc/{pid}/cmdline", "rb").read():
                    os.kill(pid, signal.SIGTERM)
            except OSError:
                pass
        try:
            self.proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            self.proc.kill()
        self.thread.join(timeout=5)


def close_box(a, b, tol):
    return a is not None and b is not None and all(abs(float(x) - float(y)) <= tol for x, y in zip(a, b))


def box_of(d):
    return None if not d else [d["x"], d["y"], d["w"], d["h"]]


def jpeg_rgb(data):
    import io
    import numpy as np
    from PIL import Image
    return np.asarray(Image.open(io.BytesIO(data)).convert("RGB")).astype(np.int16)


def consent_of(stderr):
    """What the CLI said before asking for a yes: the capture path, the
    disclosure, where the picture would go and its size."""
    import re
    lines = [l.strip() for l in stderr.splitlines()]
    out = {"prompted": "Allow once?" in stderr, "path": None, "disclosure": None, "destination": None,
           "width": None, "height": None}
    for i, line in enumerate(lines):
        m = re.match(r"Oni took a picture of the perched window, (\d+) × (\d+) pixels, through (\S+)\.$", line)
        if m:
            out.update(width=int(m.group(1)), height=int(m.group(2)), path=m.group(3))
            rest = []
            for later in lines[i + 1:]:
                if later.startswith("It stays on this computer"):
                    out["destination"] = "local"
                    break
                if later.startswith("It goes to"):
                    out["destination"] = "hosted"
                    break
                rest.append(later)
            out["disclosure"] = " ".join(rest) or None
    return out


class Phase10(Phase9):
    REPORT = "phase10.json"

    def __init__(self, lab, out):
        super().__init__(lab, out)
        for d in ("model", "answers"):
            (self.out / d).mkdir(parents=True, exist_ok=True)
        self.perch_spec = self.spec["perch"]
        url = self.env.get("LAB_MODEL_URL")
        self.model = LabModel(url) if url else None
        self.results["model"] = {
            "url": url, "id": "oni-fixture-vision",
            "kind": "deterministic loopback fixture (release/lab/oni/model.py) on the OpenAI-compatible wire; "
                    "it proves routing, consent, capture and pixels, not model quality"}
        self.results.setdefault("limits", [])
        self.start_wall = time.time()
        self.children = ChildWatch(self.env["LAB_ID"])
        self.ladder_paths = []
        self.manifest = json.loads((self.lab / "build/notes.json").read_text())

    # --- plumbing ---

    def limit(self, step, check, reason):
        self.results["limits"].append({"step": step, "check": check, "reason": reason})

    def cli(self, *args, stdin=None, timeout=300):
        r = subprocess.run([str(HERE / "ctl.sh"), "hyakki", *map(str, args)], input=stdin, capture_output=True,
                           text=True, timeout=timeout)
        self.log.write(f"{now()} hyakki {' '.join(map(str, args))} -> {r.returncode}\n{r.stdout}{r.stderr}\n")
        self.log.flush()
        return r

    def try_call(self, method, params=None):
        r = self.cli("call", method, json.dumps(params if params is not None else {}))
        if r.returncode == 0:
            try:
                return True, json.loads(r.stdout) if r.stdout.strip() else {}
            except ValueError:
                return True, {"raw": r.stdout.strip()}
        return False, (r.stderr.strip() or r.stdout.strip())

    def oni_settings(self):
        value = self.call("settings.get", {})
        for key in ("oni",):
            if isinstance(value.get(key), dict):
                return value[key]
            if isinstance(value.get("settings"), dict) and isinstance(value["settings"].get(key), dict):
                return value["settings"][key]
        return {}

    def fixture_open(self, name):
        out = self.sh("window.sh", "open", name).stdout.strip().splitlines()
        return json.loads(out[-1])

    def fixture(self, name):
        r = self.sh("window.sh", "get", name, check=False)
        return json.loads(r.stdout) if r.returncode == 0 and r.stdout.strip() else None

    def gbox(self, rec):
        return [rec["pos"][0], rec["pos"][1], rec["size"][0], rec["size"][1]]

    def region_box(self, rec, region):
        """A fixture region's global box in the window as placed now."""
        x, y, w, h = self.manifest["regions"][region]["box"]
        return [rec["pos"][0] + x, rec["pos"][1] + y, w, h]

    def perch_cli(self, window_id):
        r = self.cli("oni", "perch", str(window_id), "--json")
        value = None
        if r.returncode == 0:
            try:
                value = json.loads(r.stdout)
            except ValueError:
                value = {"raw": r.stdout.strip()}
        return r.returncode == 0, value, r.stderr.strip()

    def detach_cli(self):
        return self.cli("oni", "detach", "--json").returncode == 0

    def perch_frame(self):
        """The daemon's attached PerchFrame from an observer snapshot (a
        detached one is not in the snapshot)."""
        for frame in self.call("oni.open", {"renderer": "observer"}).get("frames", []):
            if frame.get("kind") == "perch" and (frame.get("data") or {}).get("attached"):
                return frame["data"]
        return None

    def after(self, mark, pred, timeout, what=None):
        line = self.ledger.wait(pred, mark, timeout)
        if line is None and what:
            raise RuntimeError(f"ledger never showed {what} within {timeout} s")
        return line

    def perch_line(self, mark, timeout=15, window=None, reasons=("",)):
        return self.after(mark, lambda l: l.get("ev") == "perch" and (l.get("reason") or "") in reasons
                          and (window is None or str((l.get("perch") or {}).get("window")) == str(window)),
                          timeout, f"a perch line {reasons} for {window}")

    def quiet_parts(self, seconds=1.0, timeout=20, parts=("strand", "chip", "highlight")):
        """Wait until no commit of PARTS for SECONDS."""
        start, last, since = time.monotonic(), None, time.monotonic()
        while time.monotonic() - start < timeout:
            n = sum(1 for l in self.lines() if l.get("ev") == "commit" and l.get("part") in parts)
            t = time.monotonic()
            if n != last:
                last, since = n, t
            elif t - since >= seconds:
                return True
            time.sleep(0.1)
        return False

    def silent(self, seconds=2.0, timeout=45):
        """Wait until the surface writes no ledger line at all for SECONDS:
        its loop has stopped (a held strand steps without committing until
        its 30 s stop, which a commit-only quiet wait does not see)."""
        start, last, since = time.monotonic(), None, time.monotonic()
        while time.monotonic() - start < timeout:
            n = len(self.lines())
            t = time.monotonic()
            if n != last:
                last, since = n, t
            elif t - since >= seconds:
                return True
            time.sleep(0.2)
        return False

    def ensure_detached(self):
        if summary(self.lines())["perch"] is not None or self.perch_frame():
            mark = self.ledger.mark()
            self.detach_cli()
            self.ledger.wait(lambda l: l.get("ev") == "detach", mark, 10)
        self.quiet_parts(1.0, 20)

    def perch_on(self, name, retries=1):
        """Perch through the CLI and wait for the curl to settle."""
        rec = self.fixture(name)
        if rec is None:
            raise RuntimeError(f"fixture {name} is not open")
        for _ in range(retries):
            mark = self.ledger.mark()
            ok, value, err = self.perch_cli(rec["id"])
            if ok:
                line = self.perch_line(mark, 15, rec["id"])
                self.quiet_parts(1.0, 20, ("strand",))
                return rec, value, line
            time.sleep(1.0)
        raise RuntimeError(f"perch on {name} refused: {err}")

    def focus(self, name):
        self.sh("window.sh", "focus", name)

    def move_fixture(self, name, x, y):
        """window.sh move takes whole logical pixels; window records may
        carry fractional positions (scaled outputs)."""
        self.sh("window.sh", "move", name, round(x), round(y))

    def seat(self, box, corner, side_w, side_h, out):
        """Where the perch rules put the character's frame (output-local
        top-left): outside the chosen top corner, flipped inside on an axis
        that would cross the output margin, then clamped."""
        s = self.perch_spec["seat"]
        gap, lift, margin = s["gapPx"], s["liftPx"], s["marginPx"]
        x, y, w, h = box
        W, H = out["width"], out["height"]
        if corner == "top-right":
            sx = x + w + gap
            if sx + side_w > W - margin:
                sx = x + w - gap - side_w
        else:
            sx = x - gap - side_w
            if sx < margin:
                sx = x + gap
        sy = y - lift - side_h
        if sy < margin:
            sy = y + lift
        return (min(max(sx, margin), W - margin - side_w), min(max(sy, margin), H - margin - side_h))

    def empty_point(self, state, avoid, out, prefer=None):
        """A point on OUT that no window and no AVOID rect is near, away from
        niri's hot corner: as far from every window as possible, or the free
        point nearest PREFER."""
        rects = [grow([w["pos"][0], w["pos"][1], *w["size"]], 48) for w in state["windows"] if w.get("pos") and w.get("size")]
        # a niri tiled window has no place: the lab's columns start at the
        # output's left edge, so treat it as filling the output's height there
        rects += [[out["x"], out["y"], w["size"][0] + 96, out["height"]] for w in state["windows"]
                  if not w.get("pos") and w.get("size")]
        rects += [grow(r, 96) for r in avoid]
        best, best_d = None, -math.inf
        for gx in range(out["x"] + 80, out["x"] + out["width"] - 80, 40):
            for gy in range(out["y"] + 80, out["y"] + out["height"] - 80, 40):
                if gx - out["x"] < 160 and gy - out["y"] < 160:
                    continue
                if any(inside((gx, gy), r) for r in rects):
                    continue
                d = min([math.hypot(gx - (r[0] + r[2] / 2), gy - (r[1] + r[3] / 2)) for r in rects] or [1e9])
                if prefer is not None:
                    d = -math.hypot(gx - prefer[0], gy - prefer[1])
                if d > best_d:
                    best, best_d = (gx, gy), d
        if best is None:
            raise RuntimeError(f"no empty desktop left on {out['name']}")
        return best

    def drag_steps(self, a, b, hold_ms=0, n=24):
        steps = [f"a {round(a[0])} {round(a[1])}", "w 80", "d", "w 120"]
        for i in range(1, n + 1):
            steps += [f"a {round(a[0] + (b[0] - a[0]) * i / n)} {round(a[1] + (b[1] - a[1]) * i / n)}", "w 16"]
        if hold_ms:
            steps.append(f"w {hold_ms}")
        return steps + ["u"]

    def park_pointer(self):
        """Move the lab's pointer onto empty desktop, away from every window
        and the character: a region grab takes whatever the compositor draws
        over the window, and nested Hyprland draws its cursor into
        screencopy. Empty desktop, because Hyprland's focus follows the
        mouse and an unfocused window is refused on the crop path."""
        state, out, sprite = self.where()
        self.sh("input.sh", "move", *self.empty_point(state, [sprite], out))

    def ask(self, question, answer="y", preview=None, timeout=300):
        """hyakki oni ask --json on one connection, answering the consent
        prompt from stdin; returns the exit code, the result, what the
        consent prompt said and stderr."""
        self.park_pointer()
        args = ["oni", "ask", "--json"]
        if preview:
            args += ["--preview", str(preview)]
        r = self.cli(*args, question, stdin=answer, timeout=timeout)
        result = None
        if r.returncode == 0:
            try:
                result = json.loads(r.stdout)
            except ValueError:
                result = None
        return {"rc": r.returncode, "result": result, "consent": consent_of(r.stderr), "stderr": r.stderr.strip()}

    def save_model_picture(self, n, name):
        data = self.model.image(n, 0)
        path = self.out / "model" / f"{name}.jpg"
        path.write_bytes(data)
        return path, data

    def restart_daemon(self, *hooks):
        self.sh("ctl.sh", "daemon", "restart", *hooks)
        # The surface redials a lost link once, at once; the new daemon is
        # not listening yet, so it goes dormant until the user clicks
        # (Phase 8's link-loss rule). A session restarts its presenter with
        # the daemon, and so does the lab.
        self.sh("ctl.sh", "surface", "restart")
        self.wait((None, 0), 15, ev="commit")
        self.wake()
        time.sleep(1.0)

    def reperch_notes(self, timeout=20):
        deadline = time.monotonic() + timeout
        while True:
            try:
                self.ensure_detached()
                self.focus("notes")
                return self.perch_on("notes")
            except RuntimeError:
                if time.monotonic() > deadline:
                    raise
                time.sleep(1.0)

    # --- step 0: census ---

    def census10(self):
        adv = json.loads(self.sh("window.sh", "globals").stdout)["globals"]
        (self.out / "globals-advertised.json").write_text(json.dumps(adv, indent=2))
        bound = json.loads(self.sh("ctl.sh", "globals").stdout)
        (self.out / "globals.json").write_text(json.dumps(bound, indent=2))
        probes = {g: adv.get(g) for g in ("ext_foreign_toplevel_list_v1", "ext_image_copy_capture_manager_v1",
                                           "ext_foreign_toplevel_image_capture_source_manager_v1",
                                           "ext_output_image_capture_source_manager_v1", "zwlr_screencopy_manager_v1",
                                           "zwlr_foreign_toplevel_manager_v1")}
        toplevel = bool(probes["ext_image_copy_capture_manager_v1"]
                        and probes["ext_foreign_toplevel_image_capture_source_manager_v1"])
        output = bool(probes["zwlr_screencopy_manager_v1"] or (probes["ext_image_copy_capture_manager_v1"]
                                                              and probes["ext_output_image_capture_source_manager_v1"]))
        offered = "toplevel" if toplevel else ("output-crop" if output else None)
        self.results["capture_offer"] = {"probes": probes, "toplevel": toplevel, "output": output,
                                         "compositor_offers": offered}
        if not toplevel:
            self.limit(0, "toplevel capture", f"{self.wm} here advertises no ext toplevel capture source")
        self.limit(0, "toplevel rung", "the daemon gives capture no toplevel resolver, so its ladder starts at "
                                       "output-crop whatever the compositor offers; step 8 records what ran")
        checks = [{"name": f"probe {g}", "value": v, "pass": True} for g, v in probes.items()]
        checks += [
            {"name": "the fixture client lists the compositor's globals", "value": len(adv), "pass": len(adv) > 0},
            {"name": "a capture path is offered (toplevel or output)", "value": offered, "pass": offered is not None},
            {"name": "the surface binds layer-shell", "value": sorted(bound.get("bound", {})),
             "pass": "zwlr_layer_shell_v1" in bound.get("bound", {})},
        ]
        return {"advertised": len(adv), "checks": checks}, self.checks_pass(checks)

    # --- step 1: tracker ---

    def tracker(self):
        self.fixture_open("notes")
        if self.wm == "niri":
            self.fixture_open("notes-tiled")
        time.sleep(0.5)
        state = self.state()
        targets = [("notes", self.fixture("notes")),
                   ("hyakki-app", next(w for w in state["windows"] if w.get("app_id") == APP_ID))]
        if self.wm == "niri":
            targets.append(("notes-tiled", self.fixture("notes-tiled")))
        rows, checks = [], []
        for name, rec in targets:
            self.ensure_detached()
            mark = self.ledger.mark()
            ok, value, err = self.perch_cli(rec["id"])
            line = self.ledger.wait(lambda l: l.get("ev") == "perch", mark, 5)
            frame = self.perch_frame()
            placeable = rec.get("box") is not None
            row = {"name": name, "compositor": rec, "perched": ok, "daemon": value, "refusal": None if ok else err,
                   "ledger": line, "snapshot": frame, "placeable": placeable}
            rows.append(row)
            if placeable:
                dbox = box_of((value or {}).get("box"))
                checks += [
                    {"name": f"{name}: perched", "value": err or None, "pass": ok},
                    {"name": f"{name}: the daemon's box is the compositor's within 1 px",
                     "value": {"daemon": dbox, "compositor": rec["box"]}, "pass": close_box(dbox, rec["box"], 1)},
                    {"name": f"{name}: id, app id and title as the compositor lists them",
                     "value": {k: (value or {}).get(k) for k in ("window", "app", "title")},
                     "pass": str((value or {}).get("window")) == str(rec["id"]) and (value or {}).get("app") == rec["app_id"]
                     and (value or {}).get("title") == rec["title"]},
                    {"name": f"{name}: the snapshot carries the attached perch", "value": frame,
                     "pass": bool(frame) and str(frame.get("window")) == str(rec["id"])},
                ]
                self.detach_cli()
            else:
                checks += [
                    {"name": f"{name}: no geometry, so the perch is refused", "value": err, "pass": not ok},
                    {"name": f"{name}: the refusal says no-geometry and why", "value": err,
                     "pass": "no-geometry" in err or "position" in err.lower()},
                    {"name": f"{name}: no PerchFrame after the refusal", "value": frame, "pass": frame is None},
                ]
        if self.wm == "niri" and any(not r["placeable"] for r in rows):
            self.limit(1, "tiled windows", "released niri gives tiled windows no position: perch and crop are "
                                           "floating-only and a tiled window is refused no-geometry")
        if self.wm == "niri":
            self.sh("window.sh", "close", "notes-tiled")
        self.ensure_detached()
        return {"rows": rows, "checks": checks}, self.checks_pass(checks)

    # --- step 2: drag, highlight, drop ---

    def drag_drop(self):
        import numpy as np
        self.ensure_detached()
        self.wake()
        self.settle(1.0)
        notes = self.fixture("notes")
        state, out, sprite = self.where()
        hl = self.perch_spec["highlight"]
        checks = []
        # an empty desktop refuses
        empty = self.empty_point(state, [sprite], out)
        mark = self.ledger.mark()
        self.sh("input.sh", "drag", *map(round, centre(sprite)), *empty)
        time.sleep(2.5)
        lines = self.ledger.since(mark)
        attached = [l for l in lines if perched_window(l) is not None]
        refusals = [l for l in lines if l.get("ev") == "perch" and perched_window(l) is None]
        frame = self.perch_frame()
        checks += [
            {"name": "a drop on the empty desktop perches nothing", "value": attached, "pass": not attached},
            {"name": "and leaves no PerchFrame", "value": frame, "pass": frame is None},
            {"name": "the refusal is in the ledger with its reason", "value": refusals,
             "pass": bool(refusals) and bool(refusals[-1].get("reason"))},
        ]
        # a drag onto the notes window: the outline while held, then the perch
        self.settle(1.0)
        state, out, sprite = self.where()
        g = self.gbox(notes)
        crop_rect = self.region(g, pad=40, out=out)
        base = self.shoot("drag-baseline", crop_rect)
        target = (g[0] + g[2] * 0.5, g[1] + g[3] * 0.73)
        mark = self.ledger.mark()
        runner = threading.Thread(target=lambda: self.sh("input.sh", "ptr", *self.drag_steps(centre(sprite), target, 3000),
                                                         check=False))
        runner.start()
        hline = self.ledger.wait(lambda l: l.get("ev") == "highlight" and l.get("box"), mark, 10)
        time.sleep(hl["fadeInS"] + 0.4)
        held = self.shoot("drag-highlight", crop_rect)
        carried = global_rect(self.current(), self.state())
        runner.join(timeout=60)
        perch = self.ledger.wait(lambda l: perched_window(l) is not None, mark, 15)
        img, before = load_rgb(held), load_rgb(base)
        f = img.shape[1] / crop_rect[2]
        changed = np.abs(img - before).max(axis=2) > 24
        ys, xs = np.mgrid[0:img.shape[0], 0:img.shape[1]]
        X, Y = crop_rect[0] + (xs + 0.5) / f, crop_rect[1] + (ys + 0.5) / f
        dx = np.maximum(np.maximum(g[0] - X, X - (g[0] + g[2])), 0)
        dy = np.maximum(np.maximum(g[1] - Y, Y - (g[1] + g[3])), 0)
        d = np.hypot(dx, dy)
        sp = grow(carried, 6)
        off_sprite = ~((X >= sp[0]) & (X <= sp[0] + sp[2]) & (Y >= sp[1]) & (Y <= sp[1] + sp[3]))
        line_d = hl["padPx"]
        # the outline is a light stroke with a dark edge, padPx outside the
        # frame: along each straight run (the rounded corners left out), the
        # stroke counts at a position when a pixel within 1.5 px of padPx
        # changed there. A share of changed band pixels would depend on the
        # desktop behind: on Hyprland's black one the dark edge never differs
        # and a whole 2 px stroke is half its 4 px band.
        r = hl["radiusPx"] + line_d
        band = (d >= line_d - 1.5) & (d <= line_d + 1.5) & off_sprite
        runs = {"left": X < g[0], "right": X > g[0] + g[2], "top": Y < g[1], "bottom": Y > g[1] + g[3]}
        along = {"left": (Y, g[1], g[3]), "right": (Y, g[1], g[3]), "top": (X, g[0], g[2]), "bottom": (X, g[0], g[2])}
        sides = {}
        for side, where in runs.items():
            axis, lo, span = along[side]
            pos = np.floor(axis).astype(int)
            keep = band & where & (axis > lo + r) & (axis < lo + span - r)
            seen = set(pos[keep].tolist())
            if seen:
                sides[side] = round(len(set(pos[keep & changed].tolist())) / len(seen), 3)
        interior = (X > g[0] + 2) & (X < g[0] + g[2] - 2) & (Y > g[1] + 2) & (Y < g[1] + g[3] - 2) & off_sprite
        inner_frac = float(changed[interior].mean()) if interior.any() else 1.0
        hbox = box_of((hline or {}).get("box"))
        pad = hl["padPx"]
        checks += [
            {"name": "the highlight is in the ledger while held", "value": hline, "pass": hline is not None},
            {"name": "its box is the window's (or the window's grown by padPx), within 1 px",
             "value": {"ledger": hbox, "window": notes["box"]},
             "pass": close_box(hbox, notes["box"], 1) or close_box(hbox, grow(notes["box"], pad), 1)},
            {"name": f"the outline drawn {pad} px outside the frame along every straight run",
             "value": sides, "pass": len(sides) == 4 and min(sides.values()) >= 0.9},
            {"name": "the window's own pixels untouched (outside the carried character)",
             "value": round(inner_frac, 4), "pass": inner_frac <= 0.01},
            {"name": "the drop perches on the notes window", "value": perch,
             "pass": perch is not None and str(perched_window(perch)) == str(notes["id"])},
        ]
        self.quiet_parts(1.0, 20)
        frame = self.perch_frame()
        chip = next((l for l in reversed(self.ledger.since(mark)) if l.get("ev") == "commit" and l.get("part") == "chip"), None)
        chip_png = None
        if chip and chip.get("rect"):
            st = self.state()
            chip_png = self.shoot("chip", grow(global_rect({"rect": chip["rect"], "output": chip.get("output")}, st), 4))
        checks += [
            {"name": "the daemon's perch names the exact app and title", "value": frame,
             "pass": bool(frame) and frame.get("app") == notes["app_id"] and frame.get("title") == notes["title"]},
            {"name": "the chip is drawn", "value": chip and chip.get("rect"),
             "pass": chip is not None and chip_png is not None and image_stats(chip_png)["distinct_colours"] >= 3},
        ]
        return {"empty_point": empty, "target": target, "crops": [str(base), str(held), str(chip_png)],
                "carried": carried, "checks": checks}, self.checks_pass(checks)

    # --- step 3: perch ---

    def perch10(self):
        import numpy as np
        self.ensure_detached()
        self.wake()
        notes = self.fixture("notes")
        state = self.state()
        out = self.output(notes["output"], state)
        g = self.gbox(notes)
        corners = {"top-left": (g[0], g[1]), "top-right": (g[0] + g[2], g[1])}
        crops = {k: self.region([c[0], c[1], 0, 0], pad=64, out=out) for k, c in corners.items()}
        bases = {k: self.shoot(f"perch-{k}-baseline", r) for k, r in crops.items()}
        before = global_rect(self.current(), state)
        mark = self.ledger.mark()
        ok, value, err = self.perch_cli(notes["id"])
        line = self.perch_line(mark, 15, notes["id"])
        corner = (line.get("perch") or {}).get("corner")
        curl_first = self.after(mark, lambda l: l.get("ev") == "commit" and l.get("part") == "strand"
                                and (l.get("strand") or {}).get("phase") == "curl", 20, "a curl frame")
        # the ledger can have the first curl frame before nested Hyprland
        # shows it: shoot a little way into the curl (strand.perch.curlS)
        time.sleep(min(0.15, self.spec["strand"]["perch"]["curlS"] / 3))
        mid = self.shoot("perch-curl-a", crops.get(corner, crops["top-left"]))
        self.quiet_parts(1.0, 20, ("strand",))
        done = self.shoot("perch-curl-b", crops.get(corner, crops["top-left"]))
        commit = self.current()
        rect = commit["rect"]
        lbox = box_of((line.get("perch") or {}).get("box"))
        exp = self.seat(notes["box"], corner, rect[2], rect[3], out)
        local_corner = (notes["box"][0] + (notes["box"][2] if corner == "top-right" else 0), notes["box"][1])
        curl = [c for c in strand_commits(self.ledger.since(mark)) if c["strand"]["phase"] == "curl"]
        radius = self.spec["strand"]["perch"]["radiusPx"]
        reach = self.spec["strand"]["perch"]["offsetPx"] + radius
        ds = sorted(dist(tip_of(c), local_corner) for c in curl)
        rings = {}
        gc = (out["x"] + local_corner[0], out["y"] + local_corner[1])
        now_sprite = global_rect(commit, self.state())
        for name, png in (("a", mid), ("b", done)):
            _, mask, pts, _ = self.strand_mask(png, bases.get(corner, bases["top-left"]), crops.get(corner),
                                               exclude=(grow(now_sprite, 4), grow(before, 4)))
            near = int(np.sum(np.hypot(pts[:, 0] - gc[0], pts[:, 1] - gc[1]) <= reach + 8)) if len(pts) else 0
            rings[name] = near
        frame = self.perch_frame()
        checks = [
            {"name": "perched through the CLI", "value": err or value, "pass": ok},
            {"name": "the ledger's perch box is the compositor's within 1 px", "value": {"ledger": lbox, "compositor": notes["box"]},
             "pass": close_box(lbox, notes["box"], 1)},
            {"name": "a top corner chosen", "value": corner, "pass": corner in ("top-left", "top-right")},
            {"name": "the character seated by the perch rules within 2 px", "value": {"rect": rect, "seat": exp},
             "pass": close_box(rect[:2], exp, 2)},
            {"name": "curl frames drawn", "value": len(curl), "pass": len(curl) >= 3},
            {"name": f"curl tips {radius} px from the corner", "value": {"median": ds[len(ds) // 2] if ds else None,
                                                                         "max": ds[-1] if ds else None},
             "pass": bool(ds) and abs(ds[len(ds) // 2] - radius) <= 2 and ds[-1] <= radius + 4},
            {"name": "two crops show the ring at the corner (mid-curl and settled)", "value": rings,
             "pass": rings.get("a", 0) >= 20 and rings.get("b", 0) >= 20},
            {"name": "the snapshot's perch matches (window, corner, box)", "value": frame,
             "pass": bool(frame) and str(frame.get("window")) == str(notes["id"]) and frame.get("corner") == corner
             and close_box(box_of(frame.get("box")), notes["box"], 1)},
        ]
        return {"corner": corner, "first_curl_t": curl_first["t"], "crops": [str(mid), str(done)],
                "checks": checks}, self.checks_pass(checks)

    # --- step 4: follow ---

    def follow(self):
        checks, rows = [], {}
        notes = self.fixture("notes")
        if summary(self.lines())["perch"] is None:
            self.perch_on("notes")
        home = notes["pos"][:]
        snap = self.perch_spec["reattach"]["snapPx"]
        budget = 0.05
        # a short move: the character jumps with the window, no second arrival
        mark = self.ledger.mark()
        self.move_fixture("notes", home[0] - 60, home[1] + 40)
        moved = self.fixture("notes")
        pl = self.perch_line(mark, 5, notes["id"], reasons=("move",))
        corner = (pl.get("perch") or {}).get("corner")
        out = self.output(moved["output"], self.state())
        rect = self.current()["rect"]
        exp = self.seat(moved["box"], corner, rect[2], rect[3], out)
        at = self.after(mark, lambda l: l.get("ev") == "commit" and not l.get("part") and l.get("rect")
                        and close_box(l["rect"][:2], exp, 2), 5, "the character at the moved seat")
        lines = self.ledger.since(mark)
        rows["short"] = {"perch": pl, "seat": exp, "commit": at, "lag_s": round(at["t"] - pl["t"], 4)}
        checks += [
            {"name": "a short move: the perch box follows the compositor's", "value": {"ledger": box_of((pl.get("perch") or {}).get("box")),
                                                                                       "compositor": moved["box"]},
             "pass": close_box(box_of((pl.get("perch") or {}).get("box")), moved["box"], 1)},
            {"name": f"the character at the new seat within one frame budget ({budget} s)", "value": rows["short"]["lag_s"],
             "pass": at["t"] - pl["t"] <= budget},
            {"name": "no second arrival, no reach", "value": [l.get("ev") for l in lines if l.get("ev") in ("strand-arrive", "strand-target")],
             "pass": not any(l.get("ev") in ("strand-arrive", "strand-target") for l in lines)},
        ]
        # a long move: home, fly, curl again, quietly
        self.quiet_parts(1.0, 20)
        mark = self.ledger.mark()
        far = (max(out["x"] + 40, home[0] - snap * 4), home[1] + 140)
        self.move_fixture("notes", *far)
        moved = self.fixture("notes")
        self.perch_line(mark, 5, notes["id"], reasons=("move", "reattach"))
        self.quiet_parts(1.5, 30, ("strand",))
        self.settle(1.0)
        lines = self.ledger.since(mark)
        rect = self.current()["rect"]
        last = [l for l in lines if l.get("ev") == "perch"][-1]
        exp = self.seat(moved["box"], (last.get("perch") or {}).get("corner"), rect[2], rect[3], out)
        homed = any(l.get("ev") == "hide" and l.get("part") == "strand" for l in lines)
        reperch = [l for l in lines if l.get("ev") == "strand-reperch"]
        rows["long"] = {"to": far, "seat": exp, "rect": rect, "homed": homed, "reperch": reperch}
        checks += [
            {"name": "a long move winds the thread home first", "value": homed, "pass": homed},
            {"name": "then reperches quietly (no arrival)", "value": {"reperch": len(reperch),
                                                                       "arrive": sum(l.get("ev") == "strand-arrive" for l in lines)},
             "pass": bool(reperch) and not any(l.get("ev") == "strand-arrive" for l in lines)},
            {"name": "the character ends at the new seat", "value": {"rect": rect, "seat": exp},
             "pass": close_box(rect[:2], exp, 2)},
        ]
        # an output change
        state = self.state()
        others = [o for o in state["outputs"] if o["enabled"] and o["name"] != moved["output"]]
        if self.wm == "hyprland" and others:
            o2 = others[0]
            home_output = moved["output"]
            mark = self.ledger.mark()
            # onto the other monitor (a pixel move would leave it on its
            # workspace's monitor), then to a place there
            self.sh("window.sh", "output", "notes", o2["name"])
            self.move_fixture("notes", o2["x"] + 400, o2["y"] + 200)
            moved = self.fixture("notes")
            self.after(mark, lambda l: l.get("ev") == "perch" and str((l.get("perch") or {}).get("window")) == str(notes["id"])
                       and l.get("output") == o2["name"], 10, f"a perch line on {o2['name']}")
            self.quiet_parts(1.5, 30, ("strand",))
            self.settle(1.0)
            pl = [l for l in self.ledger.since(mark) if l.get("ev") == "perch"][-1]
            commit = self.current()
            exp = self.seat(moved["box"], (pl.get("perch") or {}).get("corner"), commit["rect"][2], commit["rect"][3], o2)
            rows["output"] = {"to": o2["name"], "perch": pl, "commit": commit, "seat": exp}
            checks += [
                {"name": f"onto {o2['name']}: the perch names the new output", "value": pl.get("output"),
                 "pass": pl.get("output") == o2["name"] or (pl.get("perch") or {}).get("output") == o2["name"]},
                {"name": "the character re-perched on that output", "value": {"output": commit.get("output"), "rect": commit["rect"], "seat": exp},
                 "pass": commit.get("output") == o2["name"] and close_box(commit["rect"][:2], exp, 2)},
            ]
        else:
            home_output = None
            self.limit(4, "output change", "nested niri has one winit output; the output-change re-perch is Hyprland's")
        if home_output and self.fixture("notes")["output"] != home_output:
            self.sh("window.sh", "output", "notes", home_output)
        self.move_fixture("notes", *home)
        self.quiet_parts(1.5, 30, ("strand",))
        # focus: nothing happens unless perchFollowsFocus is on
        before = self.oni_settings().get("perchFollowsFocus", False)
        self.settings(perchFollowsFocus=False)
        lst = self.fixture_open("list")
        mark = self.ledger.mark()
        self.focus("list")
        time.sleep(2.5)
        moved_lines = [l for l in self.ledger.since(mark) if l.get("ev") in ("perch", "detach")]
        frame = self.perch_frame()
        checks += [
            {"name": "follow off: a focus change moves no perch", "value": moved_lines, "pass": not moved_lines},
            {"name": "follow off: still perched on notes", "value": frame,
             "pass": bool(frame) and str(frame.get("window")) == str(notes["id"])},
        ]
        self.focus("notes")
        time.sleep(1.0)
        self.settings(perchFollowsFocus=True)
        mark = self.ledger.mark()
        self.focus("list")
        followed = self.ledger.wait(lambda l: perched_window(l) is not None and str(perched_window(l)) == str(lst["id"]), mark, 15)
        checks.append({"name": "follow on: the perch moves to the focused window", "value": followed, "pass": followed is not None})
        self.quiet_parts(1.5, 30, ("strand",))
        self.settings(perchFollowsFocus=before)
        self.sh("window.sh", "close", "list")
        self.ensure_detached()
        self.focus("notes")
        return {"rows": rows, "checks": checks}, self.checks_pass(checks)

    # --- step 5: detach ---

    def detach10(self):
        checks, rows = [], []
        self.ensure_detached()
        notes = self.fixture("notes")
        state = self.state()
        out = self.output(notes["output"], state)
        crop_rect = self.region(self.gbox(notes), pad=70, out=out)

        def pixels_back(name, base_png, base_sprite):
            self.quiet_parts(1.5, 20)
            after = self.shoot(f"detach-{name}", crop_rect)
            sprite_now = global_rect(self.current(), self.state())
            _, mask, pts, _ = self.strand_mask(after, base_png, crop_rect, exclude=(grow(sprite_now, 6), grow(base_sprite, 6)))
            frac = float(mask.mean())
            return after, frac

        def zero_after(name):
            mark = self.ledger.mark()
            time.sleep(3.0)
            n = sum(1 for l in self.ledger.since(mark) if l.get("ev") == "commit" and l.get("part") in ("strand", "chip", "highlight"))
            return n

        for trigger in ("dragged-off", "chip", "cli"):
            self.ensure_detached()
            # niri draws a focus ring round the focused window: the baseline
            # is taken with notes already focused, as it is when perched
            self.focus("notes")
            self.settle(1.0)
            base_sprite = global_rect(self.current(), self.state())
            base = self.shoot(f"detach-{trigger}-baseline", crop_rect)
            self.perch_on("notes")
            self.quiet_parts(1.0, 20)
            mark = self.ledger.mark()
            if trigger == "dragged-off":
                st, o, sprite = self.where()
                empty = self.empty_point(st, [sprite], o)
                self.sh("input.sh", "drag", *map(round, centre(sprite)), *empty)
            elif trigger == "chip":
                chip = next((l for l in reversed(self.lines()) if l.get("ev") == "commit" and l.get("part") == "chip"), None)
                if not chip:
                    raise RuntimeError("no chip commit to click")
                cr = global_rect({"rect": chip["rect"], "output": chip.get("output")}, self.state())
                # the detach action is the square at the chip's right end
                # (render.Chip: padX in from the edge, as tall as the line)
                c = self.perch_spec["chip"]
                line = cr[3] - 2 * c["padYPx"]
                self.sh("input.sh", "click", round(cr[0] + cr[2] - c["padXPx"] - line / 2), round(cr[1] + cr[3] / 2))
            else:
                self.detach_cli()
            dl = self.ledger.wait(lambda l: l.get("ev") == "detach", mark, 10)
            png, frac = pixels_back(trigger, base, base_sprite)
            n = zero_after(trigger)
            rows.append({"trigger": trigger, "detach": dl, "changed_fraction": round(frac, 5), "part_commits_after": n,
                         "crop": str(png)})
            # a detach the surface makes carries its own reason; one the
            # daemon sends (the CLI's) reaches the presenter as a detached
            # PerchFrame, which the ledger records as "daemon"
            want = {"dragged-off": "dragged-off", "chip": "user", "cli": "daemon"}[trigger]
            checks += [
                {"name": f"{trigger}: detached ({want})", "value": dl and dl.get("reason"),
                 "pass": dl is not None and dl.get("reason") == want},
                {"name": f"{trigger}: the crop is back to its baseline (strand, chip, outline gone)",
                 "value": round(frac, 5), "pass": frac <= 0.002},
                {"name": f"{trigger}: no strand, chip or outline commit afterwards", "value": n, "pass": n == 0},
            ]
        # the window closes under the perch
        self.ensure_detached()
        self.focus("notes")
        self.perch_on("notes")
        mark = self.ledger.mark()
        self.sh("window.sh", "close", "notes")
        dl = self.ledger.wait(lambda l: l.get("ev") == "detach", mark, 10)
        self.quiet_parts(1.5, 20)
        hidden = {p: any(l.get("ev") == "hide" and l.get("part") == p for l in self.ledger.since(mark)) for p in ("strand", "chip")}
        n = zero_after("closed")
        frame = self.perch_frame()
        rows.append({"trigger": "window-close", "detach": dl, "hidden": hidden, "part_commits_after": n})
        checks += [
            {"name": "window close: detached", "value": dl, "pass": dl is not None},
            {"name": "window close: strand and chip taken down", "value": hidden, "pass": all(hidden.values())},
            {"name": "window close: no PerchFrame left", "value": frame, "pass": frame is None},
            {"name": "window close: no part commit afterwards", "value": n, "pass": n == 0},
        ]
        self.fixture_open("notes")
        # Oni off
        self.focus("notes")
        self.perch_on("notes")
        mark = self.ledger.mark()
        self.settings(master=False)
        gone = self.ledger.wait(lambda l: l.get("ev") in ("detach", "exit"), mark, 15)
        self.ledger.wait(lambda l: l.get("ev") == "exit", mark, 15)
        self.settings(master=True)
        self.sh("ctl.sh", "surface", "restart")
        self.wait((None, 0), 15, ev="commit")
        time.sleep(1.0)
        frame = self.perch_frame()
        again = [l for l in self.lines() if perched_window(l) is not None]
        rows.append({"trigger": "oni-off", "line": gone, "after_snapshot": frame})
        checks += [
            {"name": "Oni off: the surface left (detach or exit oni-off)", "value": gone, "pass": gone is not None},
            {"name": "Oni back on: no perch survives", "value": {"snapshot": frame, "ledger": again},
             "pass": frame is None and not again},
        ]
        return {"rows": rows, "checks": checks}, self.checks_pass(checks)

    # --- step 6: refusals before send ---

    def refusals(self):
        checks = []
        before = self.oni_settings()
        self.model.reset()
        self.model.mode("point")
        self.ensure_detached()
        self.settings(inspect=True, disclosure="always")
        ok, val = self.try_call("oni.inspect", {})
        checks.append({"name": "oni.inspect with no perch is refused", "value": val, "pass": not ok})
        a = self.ask(QUESTION_POINT)
        checks.append({"name": "oni ask with no perch is refused", "value": a["stderr"], "pass": a["rc"] != 0})
        self.focus("notes")
        self.perch_on("notes")
        self.settings(inspect=False)
        a = self.ask(QUESTION_POINT)
        checks.append({"name": "inspect off: refused, no consent asked", "value": a["stderr"],
                       "pass": a["rc"] != 0 and not a["consent"]["prompted"]})
        checks.append({"name": "inspect off: the model heard nothing", "value": len(self.model.requests()),
                       "pass": not self.model.requests()})
        self.settings(inspect=True, disclosure="always")
        # the consent frame and its route, before any byte leaves
        watch = Watch(self.env["LAB_ID"])
        time.sleep(1.0)
        preview = self.out / "answers" / "consent-preview.jpg"
        a = self.ask(QUESTION_POINT, answer="n\n", preview=preview)
        time.sleep(0.5)
        watch.stop()
        frames = [l["frame"].get("data") or {} for l in watch.frames("consent")]
        c = a["consent"]
        pdata = preview.read_bytes() if preview.exists() else b""
        pw = ph = None
        if pdata:
            import io
            from PIL import Image
            pw, ph = Image.open(io.BytesIO(pdata)).size
        reqs = self.model.requests()
        checks += [
            {"name": "consent asked before anything is sent", "value": c, "pass": c["prompted"]},
            {"name": "it names the capture path that ran", "value": c["path"], "pass": c["path"] in CAPTURE_PATHS},
            {"name": "it says the picture stays on this computer (the lab's route is local)", "value": c["destination"],
             "pass": c["destination"] == "local"},
            {"name": "the ConsentFrame went out with the same destination and route", "value": frames,
             "pass": bool(frames) and frames[-1].get("destination") == "local"
             and "oni-fixture-vision" in (frames[-1].get("route") or "") and frames[-1].get("feature") == "vision.inspect"},
            {"name": "the preview is a thumbnail of the picture to be sent (its shape, at most 320 px)",
             "value": {"preview": [pw, ph], "prompt": [c["width"], c["height"]]},
             "pass": bool(pdata) and pdata[:3] == b"\xff\xd8\xff" and bool(c["width"]) and bool(c["height"])
             and max(pw, ph) <= 320 and abs(pw / ph - c["width"] / c["height"]) <= 0.03},
            {"name": "no: nothing sent", "value": [a["rc"], len(reqs)], "pass": a["rc"] != 0 and not reqs},
        ]
        # the asker leaves mid-question: the picture is dropped
        p = subprocess.Popen([str(HERE / "ctl.sh"), "hyakki", "oni", "ask", "--json", QUESTION_POINT],
                             stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        time.sleep(4.0)
        for pid in lab_pids(self.env["LAB_ID"], "hyakki"):
            try:
                if b"ask" in open(f"/proc/{pid}/cmdline", "rb").read():
                    os.kill(pid, signal.SIGKILL)
            except OSError:
                pass
        try:
            p.communicate(timeout=20)
        except subprocess.TimeoutExpired:
            p.kill()
        time.sleep(1.0)
        checks.append({"name": "the asker left: nothing sent", "value": len(self.model.requests()),
                       "pass": not self.model.requests()})
        # the question runs out of time: 120 s, then dropped. stdin stays open
        # and silent (communicate would close it, which the CLI reads as no);
        # the little the CLI prints fits the pipes.
        ttl0 = time.monotonic()
        p = subprocess.Popen([str(HERE / "ctl.sh"), "hyakki", "oni", "ask", "--json", QUESTION_POINT],
                             stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        try:
            p.wait(timeout=200)
        except subprocess.TimeoutExpired:
            p.kill()
            p.wait()
        waited = time.monotonic() - ttl0
        p.stdin.close()
        err = p.stderr.read()
        said = err.strip()[-200:]
        checks += [
            {"name": "after the 120 s expiry: dropped, nothing sent", "value": {"waited_s": round(waited, 1), "said": said,
                                                                               "requests": len(self.model.requests())},
             "pass": p.returncode != 0 and 115 <= waited <= 190 and "waited too long" in said
             and not self.model.requests()},
        ]
        self.settings(disclosure=before.get("disclosure", "session"), inspect=True)
        return {"checks": checks}, self.checks_pass(checks)

    # --- step 7: ask, real daemon ---

    def ask10(self):
        # the lab writes nothing into the repository, bytecode included
        sys.dont_write_bytecode = True
        sys.path.insert(0, str(HERE))
        import model as lab_model
        checks = []
        self.ensure_detached()
        self.focus("notes")
        self.perch_on("notes")
        self.settings(inspect=True, disclosure="session")
        self.model.reset()
        self.model.mode("point")
        notes = self.fixture("notes")
        out = self.output(notes["output"], self.state())
        save = self.region_box(notes, "save")
        crop_rect = self.region(save, pad=60, out=out)
        base = self.shoot("ask-baseline", crop_rect)
        base_sprite = global_rect(self.current(), self.state())
        mark = self.ledger.mark()
        a = self.ask(QUESTION_POINT, preview=self.out / "answers" / "ask-preview.jpg")
        (self.out / "answers" / "ask-point.json").write_text(json.dumps(a, indent=2))
        reqs = self.model.requests()
        result = a["result"] or {}
        ans = result.get("answer") or {}
        img_path, data = self.save_model_picture(0, "ask-point") if reqs and reqs[0]["images"] else (None, b"")
        im = reqs[0]["images"][0] if reqs and reqs[0]["images"] else {}
        W, H = notes["size"]
        long_side = max(W, H)
        want = (W, H) if long_side <= 1568 else (round(W * 1568 / long_side), round(H * 1568 / long_side))
        located = lab_model.locate(data, "save") if data else None
        manifest_save = [b / s for b, s in zip(self.manifest["regions"]["save"]["box"],
                                               (self.manifest["width"], self.manifest["height"]) * 2)]
        point = box_of(ans.get("point"))
        gpoint = None if not point else [notes["pos"][0] + point[0] * W, notes["pos"][1] + point[1] * H,
                                         point[2] * W, point[3] * H]
        target = self.ledger.wait(lambda l: l.get("ev") == "strand-target", mark, 20)
        hold = None
        if target:
            hold = self.ledger.wait(lambda l: l.get("ev") == "commit" and l.get("part") == "strand"
                                    and (l.get("strand") or {}).get("phase") == "hold"
                                    and l["strand"].get("seq") == target.get("seq"), mark, 20)
        time.sleep(0.5)
        tip_png = self.shoot("ask-tip", crop_rect)
        ledger_tip = None
        if hold:
            ledger_tip = (out["x"] + tip_of(hold)[0], out["y"] + tip_of(hold)[1])
        sprite_now = global_rect(self.current(), self.state())
        px_tip, anchor = None, None
        if hold:
            a_local = self.anchor(hold["t"])
            if a_local is not None:
                anchor = (out["x"] + a_local[0], out["y"] + a_local[1])
        try:
            img, mask, pts, f = self.strand_mask(tip_png, base, crop_rect, exclude=(grow(sprite_now, 4), grow(base_sprite, 4)))
            if anchor is not None:
                px_tip, _ = self.pixel_tip(img, mask, crop_rect, f, anchor)
        except RuntimeError:
            px_tip = None
        # A point reach stops short of its box (strand.tipAnchor): at the box
        # grown by tipGapPx, then tipStopPx back toward the anchor, so the
        # thread points at Save notes without covering it.
        bp = self.spec["strand"]["bowedPath"]
        reach = bp["tipGapPx"] + bp["tipStopPx"]
        zone = grow(save, reach + 1.5)
        expect_tip = None
        tbox = box_of((target or {}).get("target", {}).get("box")) if target else None
        if tbox and anchor is not None:
            g = grow([out["x"] + tbox[0], out["y"] + tbox[1], tbox[2], tbox[3]], bp["tipGapPx"])
            p = (min(g[0] + g[2], max(g[0], anchor[0])), min(g[1] + g[3], max(g[1], anchor[1])))
            d = math.hypot(anchor[0] - p[0], anchor[1] - p[1])
            if d > 1e-9:
                s = min(bp["tipStopPx"], d) / d
                expect_tip = (p[0] + (anchor[0] - p[0]) * s, p[1] + (anchor[1] - p[1]) * s)
        checks += [
            {"name": "answered (consent asked once, yes)", "value": [a["rc"], a["consent"]], "pass": a["rc"] == 0 and a["consent"]["prompted"]},
            {"name": "the model got exactly one request with one picture and no tools",
             "value": [{"images": len(r["images"]), "tools": r["tools"], "kind": r.get("kind")} for r in reqs],
             "pass": len(reqs) == 1 and len(reqs[0]["images"]) == 1 and not reqs[0]["tools"]},
            {"name": "a JPEG of exactly the window's size class", "value": {"got": [im.get("width"), im.get("height")],
                                                                            "want": list(want), "mime": im.get("mime")},
             "pass": im.get("mime") == "image/jpeg" and close_box([im.get("width") or 0, im.get("height") or 0], want, 1)},
            {"name": "the picture is the window and nothing else (Save found where the window has it)",
             "value": {"located": located, "window": manifest_save},
             "pass": located is not None and close_box(located, manifest_save, 0.012)},
            {"name": "the window's words reached the model inside the fenced block", "value": reqs and reqs[0].get("fenced"),
             "pass": bool(reqs) and reqs[0].get("fenced") is True},
            {"name": "the answer text is present", "value": ans.get("text"), "pass": bool((ans.get("text") or "").strip())},
            {"name": "the route is the plan's, on this computer", "value": {k: ans.get(k) for k in ("route", "destination", "path")},
             "pass": ans.get("destination") == "local" and "oni-fixture-vision" in (ans.get("route") or "")},
            {"name": "the point lands inside Save notes", "value": {"point": gpoint, "save": save},
             "pass": gpoint is not None and inside(centre(gpoint), save)},
            {"name": f"the ledger's strand tip points at Save notes (within tipGap+tipStop = {reach} px of it)",
             "value": {"tip": ledger_tip, "zone": zone}, "pass": ledger_tip is not None and inside(ledger_tip, zone)},
            {"name": "the ledger's tip is where the reach rule puts it for the model's box (within 1.5 px)",
             "value": {"tip": ledger_tip, "expected": expect_tip},
             "pass": ledger_tip is not None and expect_tip is not None and dist(ledger_tip, expect_tip) <= 1.5},
            {"name": "pixel proof: the drawn tip points at Save notes", "value": {"tip": px_tip, "zone": zone},
             "pass": px_tip is not None and inside(px_tip, grow(zone, 2))},
        ]
        # a question about the window: answered in words, no prompt the second time (session)
        self.model.reset()
        b = self.ask(QUESTION_ABOUT)
        (self.out / "answers" / "ask-about.json").write_text(json.dumps(b, indent=2))
        reqs = self.model.requests()
        bans = ((b["result"] or {}).get("answer") or {})
        checks += [
            {"name": "session disclosure: no second prompt", "value": b["consent"], "pass": b["rc"] == 0 and not b["consent"]["prompted"]},
            {"name": "pointing said [ANSWER], so the answer prompt ran", "value": [r.get("kind") for r in reqs],
             "pass": [r.get("kind") for r in reqs] == ["answer-route", "answer"]},
            {"name": "the answer is words only", "value": bans,
             "pass": bans.get("text") == lab_model.ANSWER and not bans.get("point")},
        ]
        # always: a prompt every time
        self.settings(disclosure="always")
        self.model.reset()
        c = self.ask(QUESTION_POINT)
        checks.append({"name": "disclosure always: asked again", "value": c["consent"], "pass": c["rc"] == 0 and c["consent"]["prompted"]})
        self.settings(disclosure="session")
        self.ensure_detached()
        return {"picture": str(img_path), "crops": [str(base), str(tip_png)], "checks": checks}, self.checks_pass(checks)

    # --- step 8: the capture ladder ---

    def ladder(self):
        rows, checks = [], []
        self.model.reset()
        self.settings(inspect=True, disclosure="always")
        for paths in (None,) + CAPTURE_PATHS + ("none",):
            self.restart_daemon(*([f"HYAKKI_ONI_CAPTURE_PATHS={paths}"] if paths else []))
            try:
                self.reperch_notes()
            except RuntimeError as error:
                rows.append({"paths": paths, "error": f"perch: {error}"})
                continue
            a = self.ask(QUESTION_POINT, answer="n\n")
            rows.append({"paths": paths, "rc": a["rc"], "consent": a["consent"], "stderr": a["stderr"][-400:]})
        self.restart_daemon()
        self.settings(disclosure="session")
        ran = {r["paths"]: (r.get("consent") or {}).get("path") for r in rows}
        texts = {r["consent"]["path"]: r["consent"]["disclosure"] for r in rows if r.get("consent") and r["consent"]["path"]}
        single = [p for p in CAPTURE_PATHS if ran.get(p)]
        self.ladder_paths = single
        checks += [
            {"name": "the full ladder picks a path and says which", "value": ran.get(None), "pass": ran.get(None) in CAPTURE_PATHS},
            {"name": "the full ladder takes the first rung that works", "value": {"full": ran.get(None), "working": single},
             "pass": bool(single) and ran.get(None) == single[0]},
            {"name": "each single path runs itself or refuses, never another (no silent fallback)", "value": ran,
             "pass": all(ran.get(p) in (None, p) for p in CAPTURE_PATHS)},
            {"name": "with no path allowed it refuses, naming why", "value": next((r for r in rows if r["paths"] == "none"), None),
             "pass": ran.get("none") is None and any(r["paths"] == "none" and r.get("rc") and r.get("stderr") for r in rows)},
            {"name": "the disclosure text differs per path that ran", "value": texts,
             "pass": len(set(texts.values())) == len(texts) and all(texts.values())},
            {"name": "nothing was sent (every consent answered no)", "value": len(self.model.requests()),
             "pass": not self.model.requests()},
        ]
        if len(single) < 2:
            self.limit(8, "fallback between two working paths", f"only {single or 'no path'} works here; hiding it "
                                                                 "proves the refusal, not a second rung")
        return {"rows": rows, "checks": checks}, self.checks_pass(checks)

    # --- step 9: masking ---

    def masking(self):
        import numpy as np
        if "output-crop" not in self.ladder_paths:
            self.limit(9, "masking", "the output-crop path did not run here, and the toplevel path leaves Oni out by construction")
            return {"skipped": True}, True
        checks = []
        self.restart_daemon("HYAKKI_ONI_CAPTURE_PATHS=output-crop")
        notes = self.fixture("notes")
        home = notes["pos"][:]
        out = self.output(notes["output"], self.state())
        # The seat flips inside the window on an axis that would cross the
        # output margin: against the output's top edge, with the right edge
        # within gap + side of the output's but the curl's radius inside it,
        # a top-right perch seats over the window's top-right corner. The
        # corner is the one nearer where Oni stands, so Oni is first set
        # down at the bottom right of the output, away from every window.
        st = self.perch_spec["seat"]
        right = out["x"] + out["width"] - (st["gapPx"] + st["marginPx"] + 60)
        self.move_fixture("notes", right - notes["size"][0], out["y"] + 8)
        self.ensure_detached()
        state, o, sprite = self.where()
        spot = self.empty_point(state, [sprite], out, prefer=(out["x"] + out["width"], out["y"] + out["height"]))
        self.sh("input.sh", "drag", *map(round, centre(sprite)), *spot)
        self.settle(1.0)
        self.reperch_notes()
        notes = self.fixture("notes")
        commit = self.current()
        sprite = commit["rect"]
        overlay = next((l for l in reversed(self.lines()) if l.get("ev") == "overlay"), None)
        rects = [box_of(r) for r in (overlay or {}).get("rects", [])] or [sprite]
        self.settings(inspect=True, disclosure="session")
        self.model.reset()
        a = self.ask(QUESTION_POINT)
        reqs = self.model.requests()
        path, data = self.save_model_picture(0, "masked") if reqs and reqs[0]["images"] else (None, b"")
        img = jpeg_rgb(data) if data else None
        wb = notes["box"]
        cells = []
        if img is not None:
            fx, fy = img.shape[1] / wb[2], img.shape[0] / wb[3]
            for r in rects:
                x0, y0 = max(r[0], wb[0]), max(r[1], wb[1])
                x1, y1 = min(r[0] + r[2], wb[0] + wb[2]), min(r[1] + r[3], wb[1] + wb[3])
                if x1 - x0 < 8 or y1 - y0 < 8:
                    continue
                px = img[int((y0 - wb[1]) * fy) + 3:int((y1 - wb[1]) * fy) - 3, int((x0 - wb[0]) * fx) + 3:int((x1 - wb[0]) * fx) - 3]
                cells.append({"rect": [x0, y0, x1 - x0, y1 - y0], "mean": round(float(px.mean()), 1),
                              "min_p5": int(np.percentile(px, 5))})
        quiet_mark = self.ledger.mark()
        time.sleep(3.0)
        # every overlay line must differ from the one before it
        ov = [l.get("rects") for l in self.lines() if l.get("ev") == "overlay"]
        fresh = [l.get("rects") for l in self.ledger.since(quiet_mark) if l.get("ev") == "overlay"]
        prior = ov[:len(ov) - len(fresh)]
        seq = prior[-1:] + fresh
        repeats = [b for a_, b in zip(seq, seq[1:]) if a_ == b]
        checks += [
            {"name": "the crop path ran", "value": (a.get("result") or {}).get("answer", {}).get("path"),
             "pass": ((a.get("result") or {}).get("answer") or {}).get("path") == "output-crop"},
            {"name": "the character sits over the window (seat flipped inside)", "value": {"sprite": sprite, "window": wb},
             "pass": overlap(sprite, wb) > 0},
            {"name": "the presenter reported its rects", "value": overlay, "pass": overlay is not None},
            {"name": "white where Oni was, in the bytes the model got", "value": cells,
             "pass": bool(cells) and all(c["mean"] >= 245 and c["min_p5"] >= 225 for c in cells)},
            {"name": "overlay rects reported only on change", "value": len(repeats), "pass": not repeats},
        ]
        self.ensure_detached()
        self.move_fixture("notes", *home)
        self.restart_daemon()
        return {"picture": str(path), "rects": rects, "checks": checks}, self.checks_pass(checks)

    # --- step 10: injection ---

    def injection(self):
        self.reperch_notes()
        self.settings(inspect=True, disclosure="session")
        self.model.reset()
        self.model.mode("hostile")
        seen = set(self.children.seen)
        mark = self.ledger.mark()
        a = self.ask(QUESTION_POINT)
        time.sleep(2.0)
        self.model.mode("point")
        reqs = self.model.requests()
        spawned = [v for k, v in self.children.seen.items() if k not in seen]
        events = sorted({l.get("ev") for l in self.ledger.since(mark)})
        ans = ((a["result"] or {}).get("answer") or {})
        (self.out / "answers" / "injection.json").write_text(json.dumps({"ask": a, "requests": reqs}, indent=2))
        checks = [
            {"name": "one model call, none after the hostile reply", "value": len(reqs), "pass": len(reqs) == 1},
            {"name": "no tool offered to the model", "value": [r["tools"] for r in reqs], "pass": all(not r["tools"] for r in reqs)},
            {"name": "the window's words sat in the fenced block", "value": [r.get("fenced") for r in reqs],
             "pass": bool(reqs) and all(r.get("fenced") for r in reqs)},
            {"name": "the reply's tool call ran nothing (only capture under the daemon)", "value": spawned,
             "pass": all(s["comm"] in ("grim", "niri", "hyprctl", "ryoku") and argv_allowed(s["argv"]) for s in spawned)},
            {"name": "no tool output in the answer", "value": ans.get("text"), "pass": "rm -rf" not in (ans.get("text") or "")},
        ]
        self.ensure_detached()
        return {"answer": ans, "ledger_events": events, "checks": checks}, self.checks_pass(checks)

    # --- step 11: sensitive ---

    def sensitive(self):
        self.reperch_notes()
        self.settings(inspect=True, disclosure="session")
        self.model.reset()
        mark = self.ledger.mark()
        a = self.ask(QUESTION_SECRET)
        time.sleep(3.0)
        reqs = self.model.requests()
        ans = ((a["result"] or {}).get("answer") or {})
        targets = [l for l in self.ledger.since(mark) if l.get("ev") == "strand-target"]
        (self.out / "answers" / "sensitive.json").write_text(json.dumps({"ask": a, "requests": reqs}, indent=2))
        checks = [
            {"name": "refused in code", "value": ans.get("refused"), "pass": bool(ans.get("refused"))},
            {"name": "with the private-field line", "value": ans.get("text"), "pass": PRIVATE_REFUSAL in (ans.get("text") or "")},
            {"name": "no point and no strand toward the field", "value": {"point": ans.get("point"), "targets": targets},
             "pass": not ans.get("point") and not targets},
        ]
        self.limit(11, "refusal before any model call", f"no OCR or control list here: the refusal rests on the "
                                                        f"model's label, after {len(reqs)} model call(s)")
        self.ensure_detached()
        return {"model_calls": len(reqs), "checks": checks}, self.checks_pass(checks)

    # --- step 12: read-only ---

    def readonly(self):
        root = HERE.parents[2] / "internal/oni"
        window_root = root.parents[2] / "wm/oniwindow"
        hits = []
        for source_root in (root, window_root):
            for path in sorted(source_root.rglob("*.go")):
                if path.name.endswith("_test.go"):
                    continue
                for n, line in enumerate(path.read_text(errors="replace").splitlines(), 1):
                    low = line.lower()
                    for mark in INPUT_MARKS:
                        if mark in low:
                            hits.append({"file": str(path.relative_to(root.parents[2])), "line": n, "mark": mark})
        tests = {}
        for pkg, run in (("feature", "^TestPolicyAndProfilesStayApart$"),
                         ("window", "^(TestAllowlistIsReadVerbsOnly|TestProvidersStayInsideTheAllowlist|TestPackageSourceHasNoOtherExecPath)$")):
            binary = os.environ.get(f"ONI_LAB_{pkg.upper()}_TEST") or str(Path(self.env.get("HYAKKI_BIN", "/")) / f"{pkg}.test")
            if not os.access(binary, os.X_OK):
                tests[pkg] = {"ran": False, "pass": False, "error": f"no {pkg}.test (go test -c of internal/oni/{pkg}) in HYAKKI_BIN"}
                continue
            r = subprocess.run(["systemd-run", "--user", "--scope", "--quiet", "--collect",
                                f"--unit=oni-lab-{self.env['LAB_ID']}-{pkg}test-{os.urandom(4).hex()}",
                                "-p", "MemoryMax=1G", "-p", "MemorySwapMax=0", "--",
                                "env", "-i", "PATH=/usr/bin:/bin", binary, "-test.run", run, "-test.v", "-test.count=1"],
                               capture_output=True, text=True, timeout=300,
                               cwd=window_root if pkg == "window" else root / pkg)
            outp = (r.stdout + r.stderr).strip()
            tests[pkg] = {"ran": True, "rc": r.returncode, "output": outp[-1500:],
                          "pass": r.returncode == 0 and "--- PASS" in outp and "--- FAIL" not in outp}
        seen = list(self.children.seen.values())
        bad = [s for s in seen if not argv_allowed(s["argv"])]
        unread = dict(self.children.unread)
        inputs = [s for s in seen if s["comm"] in INPUT_PROGRAMS]
        inputs += [{"pid": p, "comm": c, "argv": None} for p, c in unread.items() if c in INPUT_PROGRAMS]
        refused = (self.lab / "log/refused.log").read_text().splitlines() if (self.lab / "log/refused.log").exists() else []
        checks = [
            {"name": "no input-executor name in any Oni package source", "value": hits, "pass": not hits},
            {"name": "boundary test (feature)", "value": tests.get("feature"), "pass": tests.get("feature", {}).get("pass")},
            {"name": "argv allowlist tests (window)", "value": tests.get("window"), "pass": tests.get("window", {}).get("pass")},
            {"name": "every program the daemon ran this run is on the allowlist (argv read before it exited)",
             "value": {"seen": len(seen), "outside": bad, "exited_unread": len(unread),
                       "unread_programs": sorted({c or "?" for c in unread.values()})},
             "pass": not bad and bool(seen)},
            {"name": "the daemon started no input program", "value": inputs, "pass": not inputs},
        ]
        return {"daemon_programs": seen, "watch_passes": self.children.passes, "refused_stubs": refused,
                "checks": checks}, self.checks_pass(checks)

    # --- step 13: disk ---

    def disk(self):
        """Pictures live in memory only: after the asks, no image file and no
        inline image is anywhere in the lab, whose tmp/ is every lab
        process's TMPDIR."""
        self.ensure_detached()
        start = self.start_wall - 1
        magics = (b"\x89PNG", b"\xff\xd8\xff", b"RIFF", b"GIF8")
        b64 = (b"data:image/", b"/9j/4", b"iVBORw0KGgo")
        images, inline, scanned = [], [], 0
        for dirpath, dirnames, filenames in os.walk(self.lab):
            for name in filenames:
                path = os.path.join(dirpath, name)
                try:
                    st = os.lstat(path)
                    if not os.path.isfile(path) or os.path.islink(path) or st.st_mtime < start or st.st_size == 0:
                        continue
                    scanned += 1
                    with open(path, "rb") as fh:
                        head = fh.read(16)
                        if head.startswith(magics):
                            images.append(path)
                        elif st.st_size <= 32 << 20:
                            body = head + fh.read()
                            if any(m in body for m in b64):
                                inline.append(path)
                except OSError:
                    continue
        tmp = self.lab / "tmp"
        left = sorted(str(p.relative_to(tmp)) for p in tmp.rglob("*")) if tmp.is_dir() else None
        checks = [
            {"name": "no picture file written in the lab during the run", "value": images, "pass": not images},
            {"name": "no inline picture (data URL or base64 JPEG/PNG) in the lab's files", "value": inline, "pass": not inline},
            {"name": "the lab processes' temp dir is empty", "value": left, "pass": left == []},
        ]
        return {"since": start, "files_scanned": scanned, "checks": checks}, self.checks_pass(checks)

    # --- step 14: reduced motion ---

    def reduced10(self):
        before = self.oni_settings().get("reduceMotion", "follow")
        self.ensure_detached()
        self.settings(reduceMotion="on")
        time.sleep(1.0)
        self.focus("notes")
        notes = self.fixture("notes")
        mark = self.ledger.mark()
        ok, _, err = self.perch_cli(notes["id"])
        pl = self.perch_line(mark, 10, notes["id"])
        time.sleep(2.5)
        lines = self.ledger.since(mark)
        parts = {}
        for l in lines:
            if l.get("ev") == "commit" and l.get("t", 0) >= pl["t"]:
                parts[l.get("part") or "character"] = parts.get(l.get("part") or "character", 0) + 1
        target = next((l for l in lines if l.get("ev") == "strand-target"), None)
        arrive = next((l for l in lines if l.get("ev") == "strand-arrive"), None)
        mark = self.ledger.mark()
        self.detach_cli()
        time.sleep(2.0)
        after = self.ledger.since(mark)
        retract_commits = len(strand_commits(after))
        unmapped = any(l.get("ev") == "hide" and l.get("part") == "strand" for l in after)
        self.settings(reduceMotion=before)
        checks = [
            {"name": "perched", "value": err or None, "pass": ok},
            {"name": "one commit per part for the perch event", "value": parts, "pass": bool(parts) and all(v <= 1 for v in parts.values())},
            {"name": "the strand arrives at once", "value": None if not (target and arrive) else round(arrive["t"] - target["t"], 4),
             "pass": target is None or (arrive is not None and arrive["t"] - target["t"] <= 0.05)},
            {"name": "detach: at most one strand commit, then unmapped", "value": [retract_commits, unmapped],
             "pass": retract_commits <= 1 and unmapped},
        ]
        return {"checks": checks}, self.checks_pass(checks)

    # --- step 15: settlement ---

    def settlement10(self):
        rows = {}
        if self.wm == "hyprland":
            # Hyprland publishes no move event, so a floating perch is
            # followed by the geometry poll while Oni is awake; a tiled one
            # moves only on events and polls nothing. The gate holds the
            # tiled perch to the settled budget and records the floating
            # one's cost.
            self.ensure_detached()
            self.fixture_open("notes-tiled")
            self.perch_on("notes-tiled")
            self.wake()
            self.quiet_parts(2.0, 30)
            self.silent(2.0, 45)
            held = self.measure("perched", timeout=120, seconds=30)
            self.ensure_detached()
            self.sh("window.sh", "close", "notes-tiled")
            self.reperch_notes()
            self.wake()
            self.quiet_parts(2.0, 30)
            self.silent(2.0, 45)
            fl = self.measure("perched", timeout=120, seconds=30, name="perched-floating", gate=False)
            daemon = {c["name"]: c["value"] for c in fl.get("checks", []) if c["name"].startswith("daemon ")}
            rows["perched_floating"] = {"pass": fl.get("pass"), "daemon": daemon}
            self.limit(15, "perched on a floating window (Hyprland)",
                       f"Hyprland publishes no move event, so while Oni is awake a floating perch is sampled with one "
                       f"batched monitors-and-clients request on Hyprland's request socket, every 250 ms after a "
                       f"change, backing off to every 2 s while the window holds still (it stops when Oni sleeps): "
                       f"{daemon}; a tiled perch samples nothing and is the gated row")
        else:
            self.reperch_notes()
            self.wake()
            self.quiet_parts(2.0, 30)
            self.silent(2.0, 45)
            held = self.measure("perched", timeout=120, seconds=30)
        mark = self.ledger.mark()
        self.detach_cli()
        self.ledger.wait(lambda l: l.get("ev") == "detach", mark, 10)
        self.quiet_parts(2.0, 30)
        self.wake()
        gone = self.measure("detached", timeout=120, seconds=30)
        checks = [{"name": "perched measurement (30 s held)" + (" on a tiled window" if self.wm == "hyprland" else ""),
                   "value": held.get("ledger_window"), "pass": bool(held.get("pass"))},
                  {"name": "detached measurement, tracker off", "value": gone.get("ledger_window"), "pass": bool(gone.get("pass"))}]
        return {"rows": rows, "checks": checks}, self.checks_pass(checks)

    def run(self):
        if self.model is None:
            self.results["steps"].append({"step": 0, "name": "lab model", "pass": False,
                                          "error": "no LAB_MODEL_URL: phase10 needs up.sh --model"})
            self.save()
            return 1
        self.children.start()
        try:
            self.step(0, "census", self.census10)
            self.step(1, "tracker", self.tracker)
            self.step(2, "drag, highlight, drop", self.drag_drop)
            self.step(3, "perch", self.perch10)
            self.step(4, "follow", self.follow)
            self.step(5, "detach", self.detach10)
            self.step(6, "refusals before send", self.refusals)
            self.step(7, "ask", self.ask10)
            self.step(8, "capture ladder", self.ladder)
            self.step(9, "masking", self.masking)
            self.step(10, "injection", self.injection)
            self.step(11, "sensitive field", self.sensitive)
            self.step(12, "read-only", self.readonly)
            self.step(13, "disk", self.disk)
            self.step(14, "reduced motion", self.reduced10)
            self.step(15, "settlement", self.settlement10)
            self.step(16, "ledger display is the lab's", self.display_check)
        finally:
            self.children.done.set()
            self.children.join(timeout=5)
            self.results["daemon_programs"] = list(self.children.seen.values())
            try:
                (self.out / "model" / "requests.json").write_text(json.dumps(self.model.requests(), indent=2))
            except OSError:
                pass
        self.results["finished_at"] = now()
        self.save()
        for limit in self.results.get("limits", []):
            print(f"limit (step {limit['step']}): {limit['check']}: {limit['reason']}")
        return 0 if self.results["pass"] else 1



# --- command line -----------------------------------------------------------

def main(argv):
    if not argv:
        raise SystemExit("labkit: missing command")
    cmd, args = argv[0], argv[1:]
    if cmd == "state":
        wm, outs, wins, layers = args
        print(json.dumps(normalize_state(wm, json.loads(outs), json.loads(wins), json.loads(layers))))
    elif cmd == "extent":
        print(*extent(json.load(sys.stdin)))
    elif cmd == "output-x":
        for o in json.load(sys.stdin)["outputs"]:
            if o["name"] == args[0]:
                print(o["x"])
                return 0
        return 1
    elif cmd == "fullscreen":
        print("on" if window_fullscreen(json.load(sys.stdin), args[0]) else "off")
    elif cmd == "window-id":
        for w in json.load(sys.stdin):
            if w.get("app_id") == args[0]:
                print(w["id"])
                return 0
        return 1
    elif cmd == "shot":
        png, dst, lab_id, output, region, state = args
        shot_record(png, dst, lab_id, output, region, json.loads(state))
    elif cmd == "wait":
        path, timeout, conds = args[0], 10.0, {}
        rest = args[1:]
        while rest:
            if rest[0] == "--timeout":
                timeout, rest = float(rest[1]), rest[2:]
            else:
                key, _, value = rest[0].partition("=")
                conds[key], rest = value, rest[1:]
        line = Ledger(path).wait(lambda l: match(l, conds), timeout=timeout)
        if line is None:
            return 1
        print(json.dumps(line))
    elif cmd == "stop":
        return stop(*args)
    elif cmd == "pids":
        print(" ".join(map(str, lab_pids(*args))))
    elif cmd == "audio-check":
        graph, lab = json.loads(Path(args[0]).read_text()), args[1]
        nodes = graph["sinks"] + graph["sources"]
        devices = [n.get("name") for n in nodes if (n.get("properties") or {}).get("device.api")]
        server = graph["info"].get("server_string", "")
        problems = []
        if devices:
            problems.append(f"device nodes {devices}")
        if graph["info"].get("default_sink_name") != "oni_lab":
            problems.append(f"default sink {graph['info'].get('default_sink_name')}")
        if not server.startswith(lab + "/"):
            problems.append(f"server {server} is not inside {lab}")
        print(json.dumps({"server": server, "sinks": [n.get("name") for n in graph["sinks"]],
                          "sources": [n.get("name") for n in graph["sources"]], "problems": problems}))
        return 1 if problems else 0
    elif cmd == "measure":
        lab, scenario, out = args
        if scenario not in SCENARIOS:
            raise SystemExit(f"labkit: scenario must be one of {', '.join(SCENARIOS)}")
        return measure(lab, scenario, out, float(os.environ.get("ONI_MEASURE_SECONDS", "60")),
                       float(os.environ.get("ONI_MEASURE_QUIET", "5")),
                       float(os.environ.get("ONI_MEASURE_TIMEOUT", "180")))
    elif cmd == "phase8":
        lab, out = args
        return Phase8(lab, out).run()
    elif cmd == "phase9":
        lab, out = args
        return Phase9(lab, out).run()
    elif cmd == "phase10":
        lab, out = args
        return Phase10(lab, out).run()
    elif cmd == "window":
        # the compositor state on stdin; the first window with this app id
        for w in json.load(sys.stdin)["windows"]:
            if w.get("app_id") == args[0]:
                print(json.dumps(w))
                return 0
        return 1
    elif cmd == "window-check":
        # a window record on stdin against pos=X,Y (within 1 px), output=NAME
        # or focused
        w, cond = json.load(sys.stdin), args[0]
        if cond.startswith("pos="):
            want = [float(v) for v in cond[4:].split(",")]
            ok = close_box(w.get("pos"), want, 1)
        elif cond.startswith("output="):
            ok = w.get("output") == cond[7:]
        else:
            ok = cond == "focused" and bool(w.get("focused"))
        print("yes" if ok else "no")
    elif cmd == "output-origin":
        for o in json.load(sys.stdin)["outputs"]:
            if o["name"] == args[0]:
                print(o["x"], o["y"])
                return 0
        return 1
    elif cmd == "stop-role":
        lab_id, role, sig = args
        return stop_role(lab_id, role, sig)
    else:
        raise SystemExit(f"labkit: unknown command {cmd}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
