#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Phase 15 helpers for the Oni lab: guarded app Hands (one live window the
person picked) and separate-desktop Hands (the nested desktop Hyakki owns).
It builds on labkit.py, voicekit.py, guidekit.py, reflexkit.py and
handskit.py without changing them, and starts lab processes only through the
lab's shell scripts (phase15.sh daemon|keys|esc|anykey|route|win|retitle|
focus|close|move|files|model among them).

appkit.py selftest            the pure helpers against synthetic data; no lab
appkit.py phase15 LAB OUT     the gate (phase15.sh runs it)

The contract the gate reads (hooks, wire, labels, ledger parts, approvals)
is the HOOKS/WIRE/LABELS/PARTS/APPROVAL tables below; their owners confirmed
them (local://hyakki-p15-lab-contract.md). A rename is one line here.

What "live" means in the lab: the lab's own compositor, never the person's
session. The separate desktop is the lab daemon's own weston and niri in
$ONI_LAB/nest. The person's part is played by the lab: the app's controls
found by tesseract and pressed with the virtual pointer (input.sh), the
pill, the chord and open mic, the Esc FIFO and the any-key FIFO. Hands' own
input program is held to the exact wtype argv shapes; the sentinel window
on the lab desktop counts every input event it gets."""
import json
import os
import re
import stat
import subprocess
import sys
import time
from pathlib import Path

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import appfix  # noqa: E402
import guidekit  # noqa: E402
import handskit  # noqa: E402
import labkit  # noqa: E402
import reflexkit  # noqa: E402
from handskit import (Phase14, StopWriter, act_rows, find_phrase, hands_rows, nearest_in_row,  # noqa: E402
                      source_hits, stop_handler, stop_reason, tree_words, glow_stats)
from labkit import global_rect, grow, now  # noqa: E402
from reflexkit import file_mode, new_rows  # noqa: E402
from voicekit import KeyWriter  # noqa: E402

ROOT = HERE.parents[4]
APP = ROOT / "ryoku/hyakki"

# --- the contract ---------------------------------------------------------------

HOOKS = {"stop": "HYAKKI_ONI_STOP_FIFO", "anykey": "HYAKKI_ONI_ANYKEY_FIFO", "nest": "HYAKKI_ONI_HANDS_NEST_DIR",
         "resume_ms": "HYAKKI_ONI_HANDS_RESUME_MS", "card_ms": "HYAKKI_ONI_HANDS_CARD_MS"}
WIRE = {
    "stages": ("off", "browser", "app", "desktop"),
    "scope_window": "window", "scope_nested": "nested", "scope_desktop": "desktop",
    # refused by the CLI itself (exit 2, nothing sent)
    "cli_local_refused": ("authorize", "start", "resume", "forget", "windows", "select", "clear"),
    # refused by the daemon from the CLI's socket
    "cli_forbidden": ("windows", "select", "clear", "authorize", "start", "resume", "forget"),
    "journal": "oni-hands-tasks.json",
    "interrupted": "interrupted", "restart_reason": "restart",
    "act_tools": ("click", "type", "press", "scroll", "open_app", "find_files", "show_file"),
    "no_file_manager": "no-file-manager",
}
LABELS = {
    "windows": "Choose a window", "authorize": "Authorize this window", "authorized": "Authorized",
    "keys_only": "keys only",
    "goal_window": "What should Hands do in this window?",
    "goal_nested": "What should Hands do on the separate desktop?",
    "start": "Start", "stop": "Stop", "resume": "Resume", "tasks": "Tasks", "clear": "Delete all",
    "queue": "Add to queue",
    "clear_yes": "Delete them", "start_again": "Start again",
    "desktop_ready": "weston, niri and grim found", "allow": "Allow",
}
PARTS = {"pill": "pill", "glow": "glow", "approval": "approval", "overlay_ev": "overlay"}
APPROVAL = {"tool": "request_hands", "method": "teams.agent_hands", "frame": "approval",
            "options": ("once", "always", "no", "app"), "danger_options": ("app",), "id_prefixes": ("ta-", "tp-")}

APP_ID = handskit.APP_ID
TARGET, TILED, SENTINEL = "dev.ryoku.OniTarget", "dev.ryoku.OniTiled", "dev.ryoku.OniSentinel"
DESK, FILES = appfix.DESK, appfix.FILES
TYPED = "hello lantern"
LONG = "lantern " * 12 + "end"  # 99 runes: three chunks of at most 40
CANARIES = (TYPED, "lantern lantern", appfix.QUERY + " lantern", "Thieved title", "Moved title")
GOALS = {
    "type": f'Type "{TYPED}"',
    "type_click": f'Type "{TYPED}", then click Mark',
    "long": f'Type "{LONG}"',
    "slow": f'Type "{TYPED}"',
    "desk": f'Open {appfix.DESK_NAME}, then type "{TYPED}", then click Mark',
    "files": f"Open {appfix.DESK_NAME}, then find {appfix.QUERY}, then show all",
    "outside": "Find ../../etc",
}
SECRET_WORDS15 = (TYPED, LONG.strip(), "Thieved title", appfix.QUERY + "-tool")
TEST_PKGS15 = handskit.TEST_PKGS + (
    ("discipline", "internal/oni/hands/discipline", "."),
    ("apprules", "internal/oni/hands/apprules", "."),
    ("task", "internal/oni/hands/task", "."),
    ("input", "internal/hands/input", "."),
    ("nest", "internal/hands/nest", "."),
    ("app", "internal/hands/app", "."),
    ("keys", "internal/oni/keys", "(?i)(any|stop|fifo)"),
)
# the live session's variables none of the separate desktop's processes may hold
LIVE_VARS = ("NIRI_SOCKET", "HYPRLAND_INSTANCE_SIGNATURE", "DBUS_SESSION_BUS_ADDRESS", "SWAYSOCK", "DISPLAY")
MODS = ("ctrl", "alt", "shift", "super", "logo", "win")
# the separate desktop's window reads (niri msg with the nest's env)
NIRI_READS = ("windows", "outputs", "focused-window", "focused-output", "workspaces", "layers", "version",
              "event-stream")


# --- pure checks ------------------------------------------------------------------

def control_free(s):
    return not any(ord(c) < 0x20 or ord(c) == 0x7F or 0x80 <= ord(c) < 0xA0 or c in "\u2028\u2029" for c in s)


def wtype_allowed(argv):
    """input.Allowed: wtype -s 50 -- CHUNK (1-40 runes, no control
    character), or wtype -s 50 [-M mod]... -k KEY [-m mod]... with distinct
    modifiers released in reverse order."""
    if len(argv) < 4 or os.path.basename(argv[0]) != "wtype" or argv[1:3] != ["-s", "50"]:
        return False
    rest = argv[3:]
    if rest[0] == "--":
        return len(rest) == 2 and 1 <= len(rest[1]) <= 40 and control_free(rest[1]) and not rest[1].startswith("-")
    held, i = [], 0
    while i < len(rest) and rest[i] == "-M":
        if i + 1 >= len(rest) or rest[i + 1] not in MODS or rest[i + 1] in held:
            return False
        held.append(rest[i + 1])
        i += 2
    if i + 1 >= len(rest) or rest[i] != "-k" or not rest[i + 1] or rest[i + 1].startswith("-") \
            or not re.fullmatch(r"[A-Za-z0-9_]+", rest[i + 1]):
        return False
    released = rest[i + 2:]
    want = []
    for m in reversed(held):
        want += ["-m", m]
    return released == want


def wtype_key(argv):
    """The key a chord argv presses, else None."""
    if "-k" in argv:
        k = argv.index("-k")
        return argv[k + 1] if k + 1 < len(argv) else None
    return None


def nest_kind(argv, nest_dir, desk):
    """weston, niri or niri-validate for the separate desktop's exact argv;
    None for anything else."""
    if not argv:
        return None
    prog = os.path.basename(argv[0])
    if prog == "weston":
        want = ["--backend=headless", "--shell=kiosk", "--idle-time=0", "--no-config",
                f"--width={desk['widthPx']}", f"--height={desk['heightPx']}", "--socket=wayland-parent"]
        return "weston" if argv[1:] == want else None
    if prog == "niri":
        if argv[1:] == ["-c", f"{nest_dir}/niri.kdl"]:
            return "niri"
        if len(argv) >= 2 and argv[1] == "validate" and f"{nest_dir}/niri.kdl" in argv:
            return "niri-validate"
    return None


def env_isolated(env, nest_dir, live_dirs):
    """(ok, why): a separate-desktop process's whole environment names the
    nested runtime dir and no live-session socket, bus or directory."""
    why = []
    if env.get("XDG_RUNTIME_DIR") != str(nest_dir):
        why.append(f"XDG_RUNTIME_DIR={env.get('XDG_RUNTIME_DIR')}")
    for k in LIVE_VARS:
        if k in env:
            v = env[k]
            if k == "NIRI_SOCKET" and v.startswith(str(nest_dir) + "/"):
                continue
            why.append(k)
    wd = env.get("WAYLAND_DISPLAY", "")
    if wd and (wd.startswith("/") and not wd.startswith(str(nest_dir) + "/")):
        why.append(f"WAYLAND_DISPLAY={wd}")
    for v in env.values():
        for d in live_dirs:
            if d and (v == d or v.startswith(d + "/")) and not v.startswith(str(nest_dir)):
                why.append(f"names {d}")
                break
    return not why, sorted(set(why))


def full_env(pid):
    env = {}
    try:
        for entry in open(f"/proc/{pid}/environ", "rb").read().split(b"\0"):
            k, sep, v = entry.decode(errors="replace").partition("=")
            if sep:
                env[k] = v
    except OSError:
        return None
    return env


def child_allowed15(rec, ctx):
    """A program the daemon ran this run, on the Phase 15 allowlist:
    the earlier gates' read verbs, tesseract and the voice pipeline
    (guidekit.child_allowed); the separate desktop's weston, niri and niri
    validate; niri msg and grim inside the separate desktop; the exact wtype
    shapes; the catalogue's Kansai Notes and Lab Files argv."""
    argv = rec.get("argv") or []
    if not argv:
        return False
    prog = os.path.basename(argv[0])
    env = rec.get("env") or {}
    if prog == "wtype":
        return wtype_allowed(argv)
    if nest_kind(argv, ctx["nest"], ctx["desk"]):
        return True
    in_nest = env.get("XDG_RUNTIME_DIR") == str(ctx["nest"])
    if prog == "niri" and argv[1:2] == ["msg"] and in_nest:
        verbs = [a for a in argv[2:] if a not in ("--json", "-j")]
        return len(verbs) == 1 and verbs[0] in NIRI_READS
    if prog == "grim" and argv[1:] == ["-t", "png", "-l", "3", "-"] and in_nest:
        return True
    if prog in ("python3", "python") and argv[1:] in (appfix.desk_argv(str(HERE), ctx["out"])[1:],):
        return in_nest
    if prog in ("python3", "python") and argv[1:len(ctx["files_argv"])] == ctx["files_argv"][1:]:
        return in_nest and len(argv) == len(ctx["files_argv"]) + 1
    return guidekit.child_allowed(rec)


def act15(rows):
    return hands_rows(rows, tool=WIRE["act_tools"], outcome="done")


def chunk_rows_ok(rows, n_chunks):
    """Each type chunk sits between a check before and a check after, both
    done, in order."""
    seq = [r for r in rows if r.get("tool") in ("check", "type")]
    types = [i for i, r in enumerate(seq) if r.get("tool") == "type" and r.get("outcome") == "done"]
    if len(types) != n_chunks:
        return False
    for i in types:
        before = seq[i - 1] if i > 0 else None
        after = seq[i + 1] if i + 1 < len(seq) else None
        if not (before and before.get("tool") == "check" and before.get("outcome") == "done"
                and after and after.get("tool") == "check" and after.get("outcome") == "done"):
            return False
    return True


def chunks_of(text, size=40):
    return [text[i:i + size] for i in range(0, len(text), size)]


def sentinel_events(lines, t0, t1=None, kinds=None):
    return [l for l in lines if l.get("t", 0) >= t0 and (t1 is None or l["t"] <= t1)
            and (kinds is None or l.get("ev") in kinds)]


def words_in(value, words):
    blob = json.dumps(value).lower()
    return [w for w in words if w.lower() in blob]


def show_argvs_ok(entries, folder):
    """Every Lab Files launch got exactly the folder, never a file in it."""
    return bool(entries) and all(e.get("argv") == [str(folder)] for e in entries)


def completed_turn_selftest15(check):
    import appmodel
    import threading
    import urllib.request
    from http.server import ThreadingHTTPServer

    server = ThreadingHTTPServer(("127.0.0.1", 0), appmodel.app_handler(appmodel.AppFixture()))
    server.daemon_threads = True
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    client = labkit.LabModel(f"http://127.0.0.1:{server.server_address[1]}")
    try:
        body = {"model": appmodel.model.MODEL,
                "messages": [{"role": "system", "content": "rules"},
                             {"role": "user", "content": appmodel.build_user('Type "hello lantern"')}]}
        request = urllib.request.Request(client.url + "/v1/chat/completions", json.dumps(body).encode(),
                                         {"Content-Type": "application/json"})
        with client.opener.open(request, timeout=5) as response:
            response.read()
        rig = Phase15.__new__(Phase15)
        rig.model = client
        rig.consents = lambda *args: [(0, {"feature": "vision.inspect", "id": "already-allowed"})]
        rig.hframes = lambda *args: []
        refused = False
        try:
            rig.wait_start15(0, 0, TARGET, 0.1, inflight=True)
        except RuntimeError:
            refused = True
        check("a completed real model turn cannot stand in for a live interruption", refused)
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)


def selftest():
    checks = []

    def check(name, ok, value=None):
        checks.append({"name": name, "pass": bool(ok), "value": value})

    good = [["wtype", "-s", "50", "--", "hello lantern"], ["wtype", "-s", "50", "-k", "Tab"],
            ["wtype", "-s", "50", "-M", "shift", "-k", "Tab", "-m", "shift"],
            ["/usr/bin/wtype", "-s", "50", "-M", "ctrl", "-M", "shift", "-k", "z", "-m", "shift", "-m", "ctrl"]]
    bad = [["wtype", "hello"], ["wtype", "-s", "50", "--", "a" * 41], ["wtype", "-s", "50", "--", "a\nb"],
           ["wtype", "-s", "50", "--", "a", "b"], ["wtype", "-d", "10", "--", "x"], ["wtype", "-s", "50", "-k", "-x"],
           ["wtype", "-s", "50", "-M", "ctrl", "-k", "v", "-m", "shift"], ["wtype", "-s", "50", "--", "-rf"],
           ["wtype", "-s", "50", "-M", "ctrl", "-M", "ctrl", "-k", "a", "-m", "ctrl", "-m", "ctrl"],
           ["ydotool", "-s", "50", "--", "x"], ["wtype", "-s", "50", "--", "\u2028"]]
    check("wtype_allowed accepts the chunk and chord shapes", all(wtype_allowed(a) for a in good),
          [a for a in good if not wtype_allowed(a)])
    check("wtype_allowed refuses a long or controlled chunk, a second text, -d, an option key, bad release order",
          not any(wtype_allowed(a) for a in bad), [a for a in bad if wtype_allowed(a)])
    check("wtype_key reads the pressed key", wtype_key(good[2]) == "Tab" and wtype_key(good[0]) is None)
    nd = "/dev/shm/oni-lab-13/nest"
    desk = {"widthPx": 1280, "heightPx": 800}
    w = ["/usr/bin/weston", "--backend=headless", "--shell=kiosk", "--idle-time=0", "--no-config", "--width=1280",
         "--height=800", "--socket=wayland-parent"]
    check("nest_kind: the exact weston and niri argv", nest_kind(w, nd, desk) == "weston"
          and nest_kind(["niri", "-c", nd + "/niri.kdl"], nd, desk) == "niri"
          and nest_kind(["niri", "validate", "-c", nd + "/niri.kdl"], nd, desk) == "niri-validate")
    check("nest_kind refuses another size, a config elsewhere and a second socket",
          nest_kind(w[:-1] + ["--socket=wayland-1"], nd, desk) is None and nest_kind(["niri", "-c", "/x.kdl"], nd, desk)
          is None and nest_kind(w[:5] + ["--width=1920"] + w[6:], nd, desk) is None)
    live = ["/dev/shm/oni-lab-13/rt", "/dev/shm/oni-lab-13/wrt", "/dev/shm/oni-lab-13/cfg"]
    ok_env = {"PATH": "/usr/bin", "HOME": "/dev/shm/oni-lab-13/state/hyakki/hands-home", "XDG_RUNTIME_DIR": nd,
              "WAYLAND_DISPLAY": "wayland-1", "NIRI_SOCKET": nd + "/niri.sock", "LANG": "C.UTF-8"}
    check("env_isolated passes the nested runtime dir and display alone", env_isolated(ok_env, nd, live)[0],
          env_isolated(ok_env, nd, live))
    leaky = [dict(ok_env, NIRI_SOCKET="/dev/shm/oni-lab-13/wrt/niri.sock"), dict(ok_env, XDG_RUNTIME_DIR=live[0]),
             dict(ok_env, DBUS_SESSION_BUS_ADDRESS="unix:path=/dev/shm/oni-lab-13/rt/bus"),
             dict(ok_env, HYPRLAND_INSTANCE_SIGNATURE="x"), dict(ok_env, WAYLAND_DISPLAY=live[1] + "/wayland-1"),
             dict(ok_env, XDG_CONFIG_HOME=live[2])]
    check("env_isolated refuses every live-session socket, bus, display and directory",
          not any(env_isolated(e, nd, live)[0] for e in leaky), [env_isolated(e, nd, live) for e in leaky])
    display_rig = Phase15.__new__(Phase15)
    display_rig.lab = Path("/dev/shm/oni-lab-13")
    display_rig.nest = Path(nd)
    display_rig.env = {"LAB_DISPLAY": "/dev/shm/oni-lab-13/w/wayland-1"}
    display_envs = [
        {"XDG_RUNTIME_DIR": "/dev/shm/oni-lab-13/w", "WAYLAND_DISPLAY": "wayland-1"},
        {"XDG_RUNTIME_DIR": "/dev/shm/oni-lab-13/rt", "WAYLAND_DISPLAY": display_rig.env["LAB_DISPLAY"]},
    ]
    check("live input resolves to the recorded private compositor with either display spelling",
          all(display_rig.on_lab_display({"env": e}) for e in display_envs))
    foreign_envs = [
        dict(display_envs[0], WAYLAND_DISPLAY="/run/user/1000/wayland-1"),
        dict(display_envs[0], WAYLAND_DISPLAY="wayland-2"),
        dict(display_envs[0], XDG_RUNTIME_DIR=nd),
        dict(display_envs[0], XDG_RUNTIME_DIR="/run/user/1000"),
    ]
    check("live input refuses a foreign same-name socket, other display, nested or owner runtime",
          not any(display_rig.on_lab_display({"env": e}) for e in foreign_envs))
    out = "/dev/shm/oni-lab-13/out"
    ctx = {"nest": nd, "desk": desk, "out": out, "files_argv": appfix.files_argv(str(HERE), out)}
    nested = {"XDG_RUNTIME_DIR": nd, "WAYLAND_DISPLAY": "wayland-1"}
    allowed = [{"argv": w, "comm": "weston"}, {"argv": good[0], "comm": "wtype"},
               {"argv": appfix.desk_argv(str(HERE), out), "comm": "python3", "env": nested},
               {"argv": ctx["files_argv"] + [out], "comm": "python3", "env": nested},
               {"argv": ["grim", "-t", "png", "-l", "3", "-"], "comm": "grim", "env": nested},
               {"argv": ["niri", "msg", "--json", "windows"], "comm": "niri", "env": nested}]
    refused = [{"argv": bad[1], "comm": "wtype"}, {"argv": ["sh", "-c", "x"], "comm": "sh"},
               {"argv": appfix.desk_argv(str(HERE), out), "comm": "python3", "env": {"XDG_RUNTIME_DIR": "/run/user/1"}},
               {"argv": ctx["files_argv"] + [out, "x"], "comm": "python3", "env": nested},
               {"argv": [out + "/../home/Documents/HandsFiles/lantern-tool"], "comm": "lantern-tool"},
               {"argv": ["niri", "msg", "action", "spawn", "--", "sh"], "comm": "niri", "env": nested}]
    check("child_allowed15 accepts the separate desktop, wtype shapes and the catalogue argv on it",
          all(child_allowed15(r, ctx) for r in allowed), [r["argv"] for r in allowed if not child_allowed15(r, ctx)])
    check("child_allowed15 refuses a long chunk, a shell, a catalogue app off the separate desktop, an extra "
          "argument, a fixture executable and a spawn through niri msg", not any(child_allowed15(r, ctx) for r in refused),
          [r["argv"] for r in refused if child_allowed15(r, ctx)])
    rows = [{"tool": "check", "outcome": "done", "reason": "before"}, {"tool": "type", "outcome": "done"},
            {"tool": "check", "outcome": "done", "reason": "after"}, {"tool": "type", "outcome": "done"},
            {"tool": "check", "outcome": "done", "reason": "after"}, {"tool": "readback", "outcome": "done"}]
    check("chunk_rows_ok: every chunk between done checks", chunk_rows_ok(rows, 2))
    check("chunk_rows_ok refuses a chunk with no check after, or a wrong count",
          not chunk_rows_ok(rows[:4], 2) and not chunk_rows_ok(rows, 3))
    check("chunks_of cuts at 40", [len(c) for c in chunks_of(LONG)] == [40, 40, 19], [len(c) for c in chunks_of(LONG)])
    sl = [{"t": 1.0, "ev": "motion"}, {"t": 2.0, "ev": "key"}, {"t": 3.0, "ev": "button"}]
    check("sentinel_events filters by time and kind", len(sentinel_events(sl, 1.5)) == 2
          and sentinel_events(sl, 0, 2.5, ("key",)) == [sl[1]])
    check("words_in finds a typed word, never a code", words_in(rows, SECRET_WORDS15) == []
          and words_in({"x": "Hello Lantern"}, (TYPED,)) == [TYPED])
    check("show_argvs_ok: the folder alone", show_argvs_ok([{"argv": ["/f"]}], "/f")
          and not show_argvs_ok([{"argv": ["/f/lantern-tool"]}], "/f") and not show_argvs_ok([], "/f"))
    completed_turn_selftest15(check)
    ok = all(c["pass"] for c in checks)
    print(json.dumps({"pass": ok, "checks": checks}, indent=2, default=str))
    return 0 if ok else 1


# --- the gate ---------------------------------------------------------------------

class Phase15(Phase14):
    REPORT = "phase15.json"

    def __init__(self, lab, out):
        super().__init__(lab, out)
        self.aout = self.lab / "out"
        self.nest = self.lab / "nest"
        self.journal = self.lab / "state/hyakki" / WIRE["journal"]
        self.home = self.lab / "home"
        self.folder = Path(appfix.folder(str(self.home)))
        self.files = appfix.manifest(str(self.home), str(self.aout))
        self.anyw = StopWriter(self.lab / "keys/anykey.fifo")
        self.picked_id = None  # the compositor id of the window last authorized
        self.aspec, self.dspec, self.tspec = self.hspec["app"], self.hspec["desktop"], self.hspec["task"]
        self.ctx = {"nest": str(self.nest), "desk": self.dspec, "out": str(self.aout),
                    "files_argv": appfix.files_argv(str(HERE), str(self.aout))}
        self.live_dirs = [self.env.get("LAB_RT", ""), self.env.get("LAB_WRT", ""), str(self.lab / "cfg"),
                          str(self.lab / "data")]
        self.results["app"] = {
            "spec": {"app": self.aspec, "desktop": self.dspec, "task": self.tspec},
            "contract": {"hooks": HOOKS, "wire": WIRE, "labels": LABELS, "parts": PARTS, "approval": APPROVAL},
            "nest_dir": str(self.nest), "files": self.files,
            "note": "appmodel.py and the appwin.py windows are loopback fixtures: they prove the reply contract, "
                    "the window discipline, consent, isolation and the stop paths, not model quality"}

    # --- plumbing ---

    def step(self, number, name, fn):
        try:
            return super().step(number, name, fn)
        finally:
            for undo in (lambda: self.hcli("stop"), lambda: self.amode("script")):
                try:
                    undo()
                except (OSError, RuntimeError, ValueError, subprocess.TimeoutExpired):
                    pass

    def daemon_stop(self):
        self.sh("phase15.sh", "daemon", "stop", timeout=120)

    def daemon_kill(self):
        self.sh("phase15.sh", "daemon", "kill", timeout=120)

    def daemon_start(self, *hooks):
        self.sh("phase15.sh", "daemon", "start", *hooks, timeout=120)
        self.call("oni.status", {})
        if self.keys:
            self.keys.close()
        fifo = self.sh("voice.sh", "keys").stdout.strip().splitlines()[-1]
        self.keys = KeyWriter(fifo)
        time.sleep(2.0)

    def wire_call(self, method, params):
        peer = labkit.UserSocket(self.lab)
        try:
            try:
                value = peer.call(method, params)
                ok = True
            except RuntimeError as error:
                value = json.loads(str(error))
                if not isinstance(value, dict) or not isinstance(value.get("code"), str):
                    raise
                ok = False
            self.log.write(f"{now()} CLI wire {method} -> {json.dumps(value)}\n")
            self.log.flush()
            return ok, value
        finally:
            peer.close()

    def amode(self, sub, delay=None):
        body = {"sub": sub}
        if delay is not None:
            body["delay"] = delay
        self.model._req("/_lab/app", body)

    def app_records(self, since_n=0):
        return [r for r in self.model_requests()[since_n:] if r.get("kind") == "app"]

    def tasks(self):
        rc, v, _ = self.hcli("tasks")
        return (v.get("tasks") or []) if rc == 0 else []

    def journal_raw(self):
        try:
            return self.journal.read_text()
        except OSError:
            return None

    def oni_fds(self):
        """Every fd target of the lab's oni daemons."""
        out = []
        for pid in self.daemon_pids():
            try:
                out += [os.readlink(f"/proc/{pid}/fd/{fd}") for fd in os.listdir(f"/proc/{pid}/fd")]
            except OSError:
                continue
        return out

    def dev_input_fds(self):
        return [f for f in self.oni_fds() if f.startswith("/dev/input/") or f.startswith("/dev/uinput")]

    def nest_procs(self):
        """Live daemon children of the separate desktop: (rec, kind)."""
        out = []
        for rec in self.vchildren.seen.values():
            k = nest_kind(rec["argv"], self.nest, self.dspec)
            if k in ("weston", "niri") and os.path.exists(f"/proc/{rec['pid']}"):
                out.append((rec, k))
        return out

    def wait_nest(self, up, timeout, removed=False):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            kinds = {k for _, k in self.nest_procs()}
            if (up and {"weston", "niri"} <= kinds) or (not up and not kinds and (not removed or not self.nest.exists())):
                return True
            time.sleep(0.2)
        return False

    def input_runs(self, since):
        return [r for r in self.vchildren.seen.values() if r["comm"] in handskit.INPUT_PROGRAMS
                and r["first"] >= since]

    def on_lab_display(self, rec):
        """Input must resolve to this lab's compositor, never just its name."""
        env = rec.get("env") or {}
        rt = env.get("XDG_RUNTIME_DIR")
        wd = env.get("WAYLAND_DISPLAY", "")
        lab = self.env.get("LAB_DISPLAY", "")
        return rt in (str(self.lab / "w"), str(self.lab / "rt")) and bool(wd and lab) and \
            os.path.normpath(os.path.join(rt, wd)) == os.path.normpath(lab)

    # --- the lab's windows ---

    def win(self, kind):
        out = self.sh("phase15.sh", "win", kind, timeout=60).stdout.strip().splitlines()
        return json.loads(out[-1])

    def window(self, app_id, state=None):
        ws = [w for w in (state or self.state())["windows"] if w.get("app_id") == app_id]
        return ws[0] if len(ws) == 1 else None

    def ensure_win(self, kind):
        app_id = {"target": TARGET, "pin": TARGET, "tiled": TILED, "sentinel": SENTINEL}[kind]
        w = self.window(app_id)
        if w is not None and kind == "pin" and (self.target_state() or {}).get("variant") != "pin":
            self.close_win(TARGET)
            w = None
        if w is not None and kind == "target" and (self.target_state() or {}).get("variant") == "pin":
            self.close_win(TARGET)
            w = None
        return w or self.win(kind)

    def close_win(self, app_id):
        w = self.window(app_id)
        if w is None:
            return True
        self.sh("phase15.sh", "close", w["id"], check=False)
        deadline = time.monotonic() + 10
        while time.monotonic() < deadline:
            if self.window(app_id) is None:
                return True
            time.sleep(0.2)
        return False

    def focus_app_id(self, app_id):
        w = self.window(app_id)
        return w is not None and self.focus(w)

    def target_state(self, name="target.json"):
        try:
            return json.loads((self.aout / name).read_text())
        except (OSError, ValueError):
            return None

    def wait_text(self, want, timeout, name="target.json"):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            st = self.target_state(name) or {}
            if st.get("text") == want:
                return st
            time.sleep(0.3)
        return self.target_state(name)

    def sentinel(self):
        return self.jsonl(self.aout / "sentinel.jsonl")

    def files_log(self):
        return self.jsonl(self.aout / "files.jsonl")

    def ran_markers(self):
        return sorted(p.name for p in self.aout.glob("ran-*"))

    # --- the app ---

    def click_label(self, label, name, pick=0):
        hits, _, _ = self.app_find(label, name)
        if not hits:
            raise RuntimeError(f"the app's Hands page shows no {label!r}")
        return self.click_box(hits[min(pick, len(hits) - 1)])

    def stage(self, want):
        return self.app_stage(want)

    def picker_words(self, app_id=TARGET):
        self.click_label(LABELS["windows"], "windows")
        time.sleep(1.5)
        _, words, box = self.app_find(app_id, "picker")
        return words, box

    def pick(self, app_id):
        """Authorize the picker row of APP_ID; the status names it after."""
        for _ in range(4):
            words, _ = self.picker_words(app_id)
            rows = find_phrase(words, app_id)
            auths = find_phrase(words, LABELS["authorize"])
            if rows and auths:
                b = nearest_in_row(rows[0], auths, tol=60)
                if b is not None:
                    self.click_box(b)
                    deadline = time.monotonic() + 8
                    while time.monotonic() < deadline:
                        st = self.hstatus()
                        if st.get("app") == app_id:
                            w = self.window(app_id)
                            self.picked_id = w.get("id") if w else None
                            return st
                        time.sleep(0.3)
            time.sleep(1.0)
        raise RuntimeError(f"the app's picker offers no authorizable row for {app_id}")

    def wait_start15(self, mark, nreq, foreground, timeout, inflight=False):
        deadline = time.monotonic() + timeout
        resumed = None
        while time.monotonic() < deadline:
            cards = self.consents(mark, "vision.inspect")
            if cards and not inflight:
                return cards[-1][1]
            frames = self.hframes(mark)
            if frames:
                wall, latest = frames[-1]
                if latest.get("active") and latest.get("phase") == "paused":
                    if latest.get("pause") == "self" and wall != resumed:
                        if not self.focus_app_id(foreground):
                            raise RuntimeError("the person could not return to the task before Resume")
                        self.surface_button(PARTS["pill"], "resume")
                        resumed = wall
                    elif latest.get("pause") != "self":
                        if inflight:
                            raise RuntimeError("the task paused before its model turn was in flight")
                        return None
                elif not latest.get("active") and any(d.get("active") for _, d in frames):
                    if inflight:
                        raise RuntimeError("the task ended before the intended interruption")
                    return None
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                break
            requests = self.model.requests(timeout=remaining)[nreq:]
            if any(r.get("kind") == "app" and (not inflight or "ms" not in r) for r in requests):
                return None
            time.sleep(min(0.05, max(0, deadline - time.monotonic())))
        if inflight:
            raise RuntimeError("no live model turn before the original start deadline")
        return None

    def start15(self, goal, scope, allow=True, queued=False):
        """Start or queue from the app, allowing a disclosure only when it is actually asked."""
        self.app_route_identity = None
        label = LABELS["goal_window"] if scope == WIRE["scope_window"] else LABELS["goal_nested"]
        foreground = TARGET if scope == WIRE["scope_window"] else SENTINEL
        last = getattr(self, "last_app_goal", "")
        parts = last.split()
        previous = (last, " ".join(parts[:2])) if parts else ()
        if len(parts) > 3:
            previous += (" ".join(parts[1:-1]),)
        # A retained single-line draft can clip its beginning at the caret.
        hits, _, _ = self.app_find(label, "goal", alternatives=(label.partition(" ")[2], *previous))
        if not hits:
            raise RuntimeError(f"the app's Hands page shows no goal field {label!r}")
        self.click_box(hits[0])
        self.sh("input.sh", "key", "-M", "ctrl", "-k", "a", "-m", "ctrl")
        self.sh("input.sh", "type", goal)
        self.last_app_goal = goal
        nreq = len(self.model_requests()) if not queued else 0
        mark = self.watch.mark()
        self.click_label(LABELS["queue"] if queued else LABELS["start"], "queue" if queued else "start")
        if not self.focus_app_id(foreground):
            raise RuntimeError(f"the person could not return focus to {foreground} after Start")
        if queued:
            return mark, None
        card = self.wait_start15(mark, nreq, foreground, 8)
        if card is not None and allow:
            self.click_label(LABELS["allow"], "allow")
            if not self.focus_app_id(foreground):
                raise RuntimeError(f"the person could not return focus to {foreground} after Allow")
            # a fast local turn can pause while the person returns from approval
            status = self.hstatus()
            if status.get("state") == "paused" and status.get("pause") == "self":
                self.surface_button(PARTS["pill"], "resume")
        return mark, card

    def begin_window(self, goal, sub="script", delay=None, kind="target"):
        """A fresh window task: stage app, the target open, picked, Start from
        the app, then focus handed to the target as the person would:
        (frame mark, ledger mark, rows before, model requests before, t0)."""
        self.hcli("stop")
        if self.oni_settings().get("hands") != "app":
            on, _ = self.stage("app")
            if not on:
                raise RuntimeError("the app's switch did not set oni.hands to app")
        w = self.ensure_win(kind)
        if self.hstatus().get("app") != TARGET or self.picked_id != (self.window(TARGET) or w).get("id"):
            self.pick(TARGET)
        self.sh("phase15.sh", "retitle", "Kansai notes", check=False)
        self.amode(sub, delay)
        before, nreq, lm, t0 = self.hlog(), len(self.model_requests()), self.ledger.mark(), time.time()
        mark, _ = self.start15(goal, WIRE["scope_window"])
        self.focus(self.window(TARGET) or w)
        if delay is not None:
            self.wait_start15(mark, nreq, TARGET, 20, inflight=True)
        elif self.wait_phase(mark, ("working", "waiting", "paused"), 20)[1] is None:
            raise RuntimeError("Start gave no active hands frame")
        return mark, lm, before, nreq, t0

    def begin_nested(self, goal, sub="script", delay=None, stage="desktop"):
        self.hcli("stop")
        if self.oni_settings().get("hands") != stage:
            on, _ = self.stage(stage)
            if not on:
                raise RuntimeError(f"the app's switch did not set oni.hands to {stage}")
        self.amode(sub, delay)
        before, nreq, lm, t0 = self.hlog(), len(self.model_requests()), self.ledger.mark(), time.time()
        mark, _ = self.start15(goal, WIRE["scope_nested"])
        if delay is not None:
            self.wait_start15(mark, nreq, SENTINEL, 30, inflight=True)
        elif self.wait_phase(mark, ("working", "waiting", "paused"), 30)[1] is None:
            raise RuntimeError("Start gave no active hands frame")
        return mark, lm, before, nreq, t0

    def pause_of(self, mark, word, timeout=8):
        return self.paused_by(mark, word, timeout)

    def acts_since(self, before, t):
        rows = self.hrows_since(before)
        return [r for r in act15(rows) if (reflexkit.at_seconds(r.get("at")) or 0) > t]

    def ended(self, mark, timeout):
        return self.wait_inactive(mark, timeout)

    # --- step 0: census ---

    def census15(self):
        voice, _ = self.census11()
        checks = [dict(c, name="voice: " + c["name"]) for c in voice["checks"]]
        checks += self.capability_checks()
        tools = {t: subprocess.run(["sh", "-c", f"command -v {t}"], capture_output=True).returncode == 0
                 for t in ("weston", "niri", "grim", "tesseract", "wtype", "espeak-ng")}
        selftests = {}
        for kit in ("appwin.py", "appfix.py", "appmodel.py", "model.py"):
            r = subprocess.run([sys.executable, str(HERE / kit), "selftest"], capture_output=True, text=True,
                               timeout=600, cwd="/dev/shm", env={"PATH": "/usr/bin:/bin", "HOME": "/dev/shm"})
            try:
                selftests[kit] = json.loads(r.stdout).get("pass")
            except ValueError:
                selftests[kit] = r.returncode == 0
        caps = self.caps()
        rows = {k: v for k, v in caps.items() if k.startswith("hands")}
        status_rc, _, status_error = self.hcli("status")
        stop_rc, st, _ = self.hcli("stop")
        sentinel = self.ensure_win("sentinel")
        checks += [
            {"name": "the tools the gate needs", "value": tools, "pass": all(tools.values())},
            {"name": "the fixtures' selftests pass", "value": selftests, "pass": all(selftests.values())},
            {"name": "oni.hands is off by default", "value": self.oni_settings().get("hands"),
             "pass": self.oni_settings().get("hands", "off") == "off"},
            {"name": "hands.app and hands.desktop guarded and denied (feature-off); hands.browser as Phase 14",
             "value": rows, "pass": all((rows.get(k) or {}).get("stage") == "guarded" and not (rows.get(k) or {}).get("allow")
                                        for k in ("hands.browser", "hands.app", "hands.desktop"))},
            {"name": "no separate desktop: no nest dir, no weston or niri child of oni",
             "value": {"dir": self.nest.exists(), "procs": [r["argv"] for r, _ in self.nest_procs()]},
             "pass": not self.nest.exists() and not self.nest_procs()},
            {"name": "no task journal yet", "value": str(self.journal), "pass": not self.journal.exists()},
            {"name": "the daemon holds no /dev/input or uinput fd", "value": self.dev_input_fds(),
             "pass": not self.dev_input_fds()},
            {"name": "both key FIFOs are the lab's own pipes, apart from the chord's",
             "value": [str(self.stopw.path), str(self.anyw.path)],
             "pass": all(stat.S_ISFIFO(os.stat(p).st_mode) for p in (self.stopw.path, self.anyw.path))
             and str(self.anyw.path) != str(self.stopw.path) and str(self.anyw.path) != str(self.keys.path if self.keys else "")},
            {"name": "status is denied with feature-off while Hands is off",
             "value": {"rc": status_rc, "error": status_error},
             "pass": status_rc == 1 and "(feature-off)" in status_error},
            {"name": "Stop while off reports closed; nothing missing for the separate desktop",
             "value": st, "pass": stop_rc == 0 and st.get("state") == "closed" and not st.get("missing")},
            {"name": "the catalogue holds Kansai Notes and Lab Files; HandsFiles is planted",
             "value": [f["name"] for f in self.files],
             "pass": all((self.lab / "data/applications" / f"{a}.desktop").is_file() for a in (DESK, FILES))
             and all(Path(f["path"]).is_file() for f in self.files)},
            {"name": "the sentinel window is up on the lab desktop", "value": sentinel, "pass": bool(sentinel)},
            {"name": "no ran- marker", "value": self.ran_markers(), "pass": not self.ran_markers()},
        ]
        return {"checks": checks}, self.checks_pass(checks)

    # --- step 1: refusals ---

    def refusals15(self):
        checks, said = [], {}
        n0 = time.time()
        cfg0 = json.dumps(self.cfg().get("oni"), sort_keys=True)
        for stage in ("app", "desktop"):
            for spelling in ({"oni": {"hands": stage}}, {"oni": {"Hands": stage}}):
                ok, msg = self.try_call("settings.set", {"patch": spelling})
                checks.append({"name": f"a CLI settings patch {json.dumps(spelling)} is forbidden",
                               "value": [ok, str(msg)[-200:]], "pass": not ok})
        checks.append({"name": "the settings file is unchanged by those patches", "value": self.cfg().get("oni"),
                       "pass": json.dumps(self.cfg().get("oni"), sort_keys=True) == cfg0})
        for action in WIRE["cli_forbidden"]:
            params = {"action": action}
            if action == "start":
                params["goal"] = "x"
            if action == "select":
                params["window"] = "1"
            ok, msg = self.try_call("oni.hands", params)
            checks.append({"name": f"a raw oni.hands {action} from the CLI is refused by the daemon",
                           "value": [ok, str(msg)[-200:]], "pass": not ok})
        for verb in WIRE["cli_local_refused"]:
            r = self.cli("oni", "hands", verb, stdin="", timeout=60)
            checks.append({"name": f"hyakki oni hands {verb} is refused by the CLI itself (exit 2)",
                           "value": {"rc": r.returncode, "stderr": r.stderr.strip()[-200:]}, "pass": r.returncode == 2})
        for say in (lambda: self.utter("on_hands"), lambda: self.wake_say("on_hands"), lambda: self.open_say("on_hands")):
            u = say()
            said.setdefault("turn_on_hands", []).append({k: u.get(k) for k in ("heard", "lines", "source")})
        checks.append({"name": "\"turn on hands\" by chord, wake and open mic changes nothing",
                       "value": self.oni_settings().get("hands"), "pass": self.oni_settings().get("hands", "off") == "off"})
        # each switch sets its stage and turns the others off
        self.settings(inspect=True, disclosure="session")
        seen = {}
        for stage in ("browser", "app", "desktop", "app"):
            on, how = self.stage(stage)
            caps = self.caps()
            seen[stage] = {"on": on, "click": how, "rows": {k: (caps.get(k) or {}).get("allow") for k in
                                                            ("hands.browser", "hands.app", "hands.desktop")}}
            checks.append({"name": f"the app's {stage!r} switch sets oni.hands={stage}; only its own "
                                   "row allows", "value": seen[stage],
                           "pass": on and self.cfg().get("oni", {}).get("hands") == stage
                           and all(bool(v) == (k == f"hands.{stage}") for k, v in seen[stage]["rows"].items())})
        self.settings(inspect=False)
        time.sleep(1.0)
        caps = self.caps()
        checks.append({"name": "app stage, inspect off: hands.app denied", "value": caps.get("hands.app"),
                       "pass": not (caps.get("hands.app") or {}).get("allow")})
        self.settings(inspect=True, disclosure="session")
        time.sleep(1.0)
        on, _ = self.stage("off")
        checks.append({"name": "turning the app stage's switch off sets oni.hands=off", "value": self.oni_settings().get("hands"),
                       "pass": on and self.oni_settings().get("hands") == "off"})
        st = self.hstatus()
        checks.append({"name": "nothing was started by any refusal: no input program, no separate desktop",
                       "value": {"inputs": self.input_runs(n0), "nest": [r["argv"] for r, _ in self.nest_procs()], "status": st},
                       "pass": not self.input_runs(n0) and not self.nest_procs()})
        self.limit(1, "missing weston or niri", "a host lacking them is proven by nest.Available's and the daemon's "
                      "unit tests and the status's missing list; the lab host has both")
        return {"said": said, "checks": checks}, self.checks_pass(checks)

    # --- step 2: the separate desktop comes up and goes away ---

    def desktop_up15(self):
        checks, notes = [], {}
        lab_windows0 = sorted(w.get("app_id") or "" for w in self.state()["windows"])
        m, lm, b, n, t0 = self.begin_nested(GOALS["type"], sub="slow", delay=20)
        up = self.wait_nest(True, self.dspec["readyS"] + 10)
        procs = self.nest_procs()
        envs = {k: full_env(r["pid"]) for r, k in procs}
        iso = {k: env_isolated(e or {}, self.nest, self.live_dirs) for k, e in envs.items()}
        notes["argv"] = {k: r["argv"] for r, k in procs}
        checks += [
            {"name": "Start on the desktop stage runs weston and niri with the exact argv in $ONI_LAB/nest",
             "value": notes["argv"], "pass": up and {k for _, k in procs} == {"weston", "niri"}},
            {"name": "the nest dir is the lab's, 0700", "value": file_mode(self.nest),
             "pass": self.nest.is_dir() and file_mode(self.nest) == 0o700},
            {"name": "each separate-desktop process's whole environment names only the nested dir and display",
             "value": {k: v[1] for k, v in iso.items()}, "pass": bool(iso) and all(v[0] for v in iso.values())},
            {"name": "no window appeared on the lab desktop",
             "value": sorted(w.get("app_id") or "" for w in self.state()["windows"]),
             "pass": sorted(w.get("app_id") or "" for w in self.state()["windows"]) == lab_windows0},
            {"name": "the hands frame says desktop, the status names no app",
             "value": self.hstatus(), "pass": self.hstatus().get("scope") == WIRE["scope_desktop"]
             and not self.hstatus().get("app")},
        ]
        # Stop ends the task and the desktop
        self.hcli("stop")
        gone = self.wait_nest(False, self.dspec["stopGraceS"] + 10, removed=True)
        checks.append({"name": "Stop ends the separate desktop and removes its dir",
                       "value": {"procs": [r["argv"][0] for r, _ in self.nest_procs()], "dir": self.nest.exists()},
                       "pass": gone and not self.nest.exists()})
        # the task's own end
        m, lm, b, n, t0 = self.begin_nested("Wait", sub="nodone")
        self.wait_nest(True, self.dspec["readyS"] + 10)
        ina = self.ended(m, 120)
        gone = self.wait_nest(False, self.dspec["stopGraceS"] + 10, removed=True)
        checks.append({"name": "the task's end ends the separate desktop",
                       "value": {"end": (ina[1] or {}).get("end"), "dir": self.nest.exists()},
                       "pass": ina[1] is not None and gone and not self.nest.exists()})
        # daemon TERM
        self.begin_nested(GOALS["type"], sub="slow", delay=20)
        self.wait_nest(True, self.dspec["readyS"] + 10)
        self.watch.pause()
        try:
            self.daemon_stop()
            gone = self.wait_nest(False, 15, removed=True)
            term = {"procs": [r["argv"][0] for r, _ in self.nest_procs()], "dir": self.nest.exists()}
            self.daemon_start()
        finally:
            self.watch.resume()
        checks.append({"name": "daemon TERM ends the separate desktop and removes its dir", "value": term,
                       "pass": gone and not term["dir"]})
        # daemon KILL: the kernel ends the children; the next boot sweeps the dir
        self.begin_nested(GOALS["type"], sub="slow", delay=20)
        self.wait_nest(True, self.dspec["readyS"] + 10)
        self.watch.pause()
        try:
            self.daemon_kill()
            gone = self.wait_nest(False, 15)
            after_kill = {"procs": [r["argv"][0] for r, _ in self.nest_procs()], "dir": self.nest.exists()}
            self.daemon_start()
            time.sleep(2.0)
            swept = not self.nest.exists() or not any(self.nest.iterdir())
        finally:
            self.watch.resume()
        checks.append({"name": "daemon KILL: no separate-desktop process outlives it; the next boot's Sweep "
                               "clears the dir", "value": {"after_kill": after_kill, "swept": swept},
                       "pass": gone and swept})
        checks.append({"name": "the restarted daemon holds no /dev/input fd", "value": self.dev_input_fds(),
                       "pass": not self.dev_input_fds()})
        return {"notes": notes, "checks": checks}, self.checks_pass(checks)

    # --- steps 3 and 4: work on the separate desktop ---

    def desktop_type15(self):
        checks = []
        (self.aout / "desk.json").unlink(missing_ok=True)
        m, lm, b, n, t0 = self.begin_nested(GOALS["desk"])
        # the lab's own Start click is done: from here the sentinel holds the
        # lab desktop's focus and nothing of the lab touches it
        self.focus_app_id(SENTINEL)
        time.sleep(0.5)
        s0 = time.time()
        st = self.wait_text(TYPED, 120, "desk.json")
        ina = self.ended(m, 120)
        rows = self.hrows_since(b)
        s1 = time.time()
        opens = [r for r in self.vchildren.seen.values() if r["argv"] == appfix.desk_argv(str(HERE), str(self.aout))
                 and r["first"] >= t0]
        ev = sentinel_events(self.sentinel(), s0, s1)
        recs = self.app_records(n)
        checks += [
            {"name": "open_app opened Kansai Notes on the separate desktop with the catalogue's exact argv",
             "value": [{"argv": r["argv"], "env": r.get("env")} for r in opens],
             "pass": len(opens) == 1 and (opens[0].get("env") or {}).get("XDG_RUNTIME_DIR") == str(self.nest)},
            {"name": f"{TYPED!r} landed in the separate desktop's window", "value": st,
             "pass": (st or {}).get("text") == TYPED},
            {"name": "typed in checked chunks and read back yes",
             "value": hands_rows(rows, tool=("check", "type", "readback")),
             "pass": chunk_rows_ok(rows, len(chunks_of(TYPED))) and bool(hands_rows(rows, tool="readback", outcome="done"))},
            {"name": "one JSON action per model turn, every turn fenced", "value": [(r.get("tool"), r.get("fenced")) for r in recs],
             "pass": bool(recs) and all(r.get("fenced") for r in recs)},
            {"name": "the sentinel, focused on the lab desktop, got no key, button or motion event", "value": ev,
             "pass": not ev},
            {"name": "every wtype run went to the separate desktop",
             "value": [r.get("env") for r in self.input_runs(t0)],
             "pass": bool(self.input_runs(t0)) and all((r.get("env") or {}).get("XDG_RUNTIME_DIR") == str(self.nest)
                                                       for r in self.input_runs(t0))},
            {"name": "the task ended done", "value": ina[1], "pass": (ina[1] or {}).get("end") == "done"},
        ]
        return {"checks": checks}, self.checks_pass(checks)

    def desktop_click15(self):
        checks = []
        sent = self.window(SENTINEL)
        if sent is None:
            raise RuntimeError("no sentinel window")
        # the lab's pointer rests on the sentinel
        c = guidekit.mid([sent["pos"][0], sent["pos"][1], sent["size"][0], sent["size"][1]])
        (self.aout / "desk.json").unlink(missing_ok=True)
        lm = self.ledger.mark()
        m, _, b, n, t0 = self.begin_nested(GOALS["desk"])
        # the lab's pointer rests on the sentinel once the Start click is done
        self.sh("input.sh", "move", round(c[0]), round(c[1]), check=False)
        time.sleep(1.0)
        s0 = time.time()
        deadline = time.monotonic() + 150
        st = None
        while time.monotonic() < deadline:
            st = self.target_state("desk.json") or {}
            if st.get("marks"):
                break
            time.sleep(0.5)
        self.ended(m, 120)
        s1 = time.time()
        ev = sentinel_events(self.sentinel(), s0, s1, ("button", "motion", "enter", "leave", "scroll", "key"))
        glow = [l for l in self.ledger.since(lm) if l.get("part") == PARTS["glow"] and l.get("ev") == "commit"]
        checks += [
            {"name": "a click on Mark landed on the separate desktop", "value": st, "pass": (st or {}).get("marks") == 1},
            {"name": "the sentinel under the lab's pointer got no event", "value": ev, "pass": not ev},
            {"name": "no glow on any live output for separate-desktop work", "value": len(glow), "pass": not glow},
        ]
        return {"checks": checks}, self.checks_pass(checks)

    # --- step 5: the window picker ---

    def pick15(self):
        checks = []
        self.hcli("stop")
        on, _ = self.stage("app")
        self.ensure_win("target")
        if self.wm == "niri":
            self.ensure_win("tiled")
        words, _ = self.picker_words()
        text = " ".join(w["text"] for w in words)
        target_rows = find_phrase(words, TARGET)
        app_rows = find_phrase(words, APP_ID)
        sentinel_rows = find_phrase(words, SENTINEL)
        st0 = self.hstatus()
        checks += [
            {"name": "the app stage is on", "value": on, "pass": on},
            {"name": "the picker lists the target", "value": target_rows, "pass": bool(target_rows)},
            {"name": "the picker never offers the Hyakki app itself", "value": app_rows, "pass": not app_rows},
            {"name": "before a pick, nothing is authorized and no task can start", "value": st0,
             "pass": st0.get("state") in ("closed", "open") and not st0.get("app")},
        ]
        if self.wm == "niri":
            tiled_rows = find_phrase(words, TILED)
            ko = find_phrase(words, LABELS["keys_only"])
            near = nearest_in_row(tiled_rows[0], ko, tol=60) if tiled_rows and ko else None
            checks.append({"name": "the tiled niri window is listed keys only", "value": {"row": tiled_rows, "keys_only": ko},
                           "pass": bool(tiled_rows) and (near is not None or any(abs(k[1] - tiled_rows[0][1]) < 80 for k in ko))})
        st = self.pick(TARGET)
        checks.append({"name": "Authorize this window binds the target's app id", "value": st,
                       "pass": st.get("app") == TARGET and st.get("state") == "authorized"})
        self.results["app"]["picker_words"] = len(text)
        self.limit(5, "Oni's surface", "Oni's layer surfaces are not toplevels, so no picker could list them; "
                      "discipline's self refusal is the unit tests'")
        if self.wm == "niri":
            self.close_win(TILED)
        return {"checks": checks}, self.checks_pass(checks)

    # --- step 6: typing into the picked window ---

    def window_type15(self):
        checks = []
        for goal, want in ((GOALS["type"], TYPED), (GOALS["long"], LONG), (GOALS["type_click"], TYPED)):
            self.sh("phase15.sh", "retitle", "Kansai notes", check=False)
            self.close_win(TARGET)
            self.ensure_win("target")
            self.pick(TARGET)
            m, lm, b, n, t0 = self.begin_window(goal)
            s0 = time.time()
            st = self.wait_text(want, 120)
            ina = self.ended(m, 120)
            st = self.target_state() or st
            rows = self.hrows_since(b)
            runs = self.input_runs(t0)
            ev = sentinel_events(self.sentinel(), s0, time.time())
            label = "type-and-click" if goal == GOALS["type_click"] else "long" if want == LONG else "short"
            checks += [
                {"name": f"{label}: the text landed in the target", "value": st, "pass": (st or {}).get("text") == want},
                {"name": f"{label}: {len(chunks_of(want))} chunk(s) of at most 40 runes, each between done checks; read back yes",
                 "value": hands_rows(rows, tool=("check", "type", "readback")),
                 "pass": chunk_rows_ok(rows, len(chunks_of(want))) and bool(hands_rows(rows, tool="readback", outcome="done"))},
                {"name": f"{label}: the only input program is the exact wtype argv on the lab display",
                 "value": [{"argv_len": len(r["argv"]), "ok": wtype_allowed(r["argv"]), "env": r.get("env")} for r in runs],
                 "pass": bool(runs) and all(wtype_allowed(r["argv"]) and self.on_lab_display(r) for r in runs)},
                {"name": f"{label}: the sentinel got nothing", "value": ev, "pass": not ev},
                {"name": f"{label}: ended done", "value": ina[1], "pass": (ina[1] or {}).get("end") == "done"},
            ]
            if goal == GOALS["type_click"]:
                checks.append({"name": "the ordinary Mark click landed once in the picked live fixture window",
                               "value": {"state": st, "rows": hands_rows(rows, tool="click")},
                               "pass": (st or {}).get("marks") == 1
                               and len(hands_rows(rows, tool="click", outcome="done")) == 1})
        lines = [l for l in self.ledger.since() if l.get("ev") == "hands" and l.get("scope") == WIRE["scope_window"]]
        target = self.window(TARGET)
        checks.append({"name": "the surface's hands lines name the window scope and the target's output",
                       "value": lines[-1:] if lines else None,
                       "pass": bool(lines) and (target is None or lines[-1].get("output") == target.get("output"))})
        return {"checks": checks}, self.checks_pass(checks)

    # --- step 7: theft pauses ---

    def theft15(self):
        checks, notes = [], {}
        cases = {}

        def case(word, act):
            self.close_win(TARGET)
            self.ensure_win("target")
            self.pick(TARGET)
            m, lm, b, n, t0 = self.begin_window(GOALS["long"], sub="slow", delay=2)
            t = time.time()
            act()
            p = self.pause_of(m, word, 6)
            time.sleep(5.0)
            acts = self.acts_since(b, p[0] or t)
            runs = [r for r in self.input_runs(p[0] or t) if r["first"] > (p[0] or t) + 0.3]
            cases[word] = {"latency_s": None if p[0] is None else round(p[0] - t, 3), "acts_after": acts,
                           "inputs_after": len(runs), "frame": p[1]}
            checks.append({"name": f"{word}: pauses at once, nothing typed after", "value": cases[word],
                           "pass": p[1] is not None and p[0] - t < 3.0 and not acts and not runs})
            return m, b

        case("title", lambda: self.sh("phase15.sh", "retitle", "Thieved title"))
        self.hcli("stop")
        tw = None

        def move():
            w = self.window(TARGET)
            self.sh("phase15.sh", "move", w["id"], round(w["pos"][0] + 60), round(w["pos"][1] + 40))
        case("geometry", move)
        self.hcli("stop")

        def other():
            self.focus_app_id(SENTINEL)
        m, b = case("focus", other)
        # the paused pill is the keep-going card: CLI and voice cannot resume
        rr = self.cli("oni", "hands", "resume", stdin="", timeout=60)
        o = self.hopen_say("keep_going")
        u = self.hutter("keep_going")
        time.sleep(2.0)
        st = self.hstatus()
        notes["voice_resume"] = {"cli": rr.returncode, "open": o.get("heard"), "chord": u.get("heard"), "status": st}
        checks.append({"name": "the CLI, open mic and the chord never resume", "value": notes["voice_resume"],
                       "pass": rr.returncode == 2 and st.get("state") in ("paused", "authorized")})
        if st.get("state") == "paused":
            self.focus_app_id(TARGET)
            rm = self.watch.mark()
            self.surface_button(PARTS["pill"], "resume")
            w = self.wait_phase(rm, ("working", "waiting"), 8)
            checks.append({"name": "the pill's Resume goes on", "value": w[1], "pass": w[1] is not None})
        self.hcli("stop")
        # a key the person presses: the any-key FIFO, never which key
        before_key = {}

        def press_key():
            frames = self.hframes(0)
            if frames:
                before_key.update(frames[-1][1])
            self.anyw.send("letter")

        m, b = case("key", press_key)
        frames = [d for _, d in self.hframes(m) if d.get("pause") == "key"]
        checks.append({"name": "the key pause names no key: the frame's pause word is key alone",
                       "value": {"before": before_key, "after": frames[-1:] if frames else None},
                       "pass": before_key.get("active") and before_key.get("phase") in ("working", "waiting")
                       and bool(frames) and all(set(d) <= set(before_key) | {"pause", "end"} for d in frames)})
        self.hcli("stop")
        # close ends it
        self.ensure_win("target")
        self.pick(TARGET)
        m, lm, b, n, t0 = self.begin_window(GOALS["long"], sub="slow", delay=2)
        self.close_win(TARGET)
        ina = self.ended(m, 15)
        checks.append({"name": "closing the target ends the task (closed)", "value": ina[1],
                       "pass": (ina[1] or {}).get("end") == "closed"})
        # the resume window ends a pause
        self.watch.pause()
        try:
            self.daemon_stop()
            self.daemon_start(f"{HOOKS['resume_ms']}=15000")
        finally:
            self.watch.resume()
        self.ensure_win("target")
        self.pick(TARGET)
        m, lm, b, n, t0 = self.begin_window(GOALS["long"], sub="slow", delay=2)
        self.focus_app_id(SENTINEL)
        p = self.pause_of(m, "focus", 6)
        ina = self.ended(m, 60)
        checks.append({"name": "a pause left alone ends at the resume window (away)",
                       "value": {"paused": p[1] is not None, "end": (ina[1] or {}).get("end")},
                       "pass": p[1] is not None and (ina[1] or {}).get("end") == "away"})
        self.watch.pause()
        try:
            self.daemon_stop()
            self.daemon_start()
        finally:
            self.watch.resume()
        return {"cases": cases, "notes": notes, "checks": checks}, self.checks_pass(checks)

    # --- step 8: private and self ---

    def private15(self):
        checks = []
        # the PIN window: a pick refused, or nothing typed into it
        self.hcli("stop")
        self.stage("app")
        self.close_win(TARGET)
        self.ensure_win("pin")
        words, _ = self.picker_words()
        rows = find_phrase(words, TARGET)
        auths = find_phrase(words, LABELS["authorize"])
        picked = None
        if rows and auths:
            b = nearest_in_row(rows[0], auths, tol=60)
            if b is not None:
                self.click_box(b)
                time.sleep(2.0)
                picked = self.hstatus()
        typed = None
        if picked and picked.get("app") == TARGET:
            before, t0 = self.hlog(), time.time()
            m, _ = self.start15("Type 4242", WIRE["scope_window"])
            self.focus_app_id(TARGET)
            time.sleep(10.0)
            self.hcli("stop")
            typed = {"rows": self.hrows_since(before), "inputs": len(self.input_runs(t0))}
        st = self.target_state() or {}
        checks.append({"name": "the PIN window is refused: not authorizable, or nothing typed into it",
                       "value": {"picked": picked, "typed": typed, "text": st.get("text")},
                       "pass": not st.get("text") and (typed is None or not typed["inputs"])})
        self.close_win(TARGET)
        # a secret-shaped text is refused before any input
        self.ensure_win("target")
        self.pick(TARGET)
        m, lm, b, n, t0 = self.begin_window(GOALS["type"], sub="secret")
        time.sleep(10.0)
        rows = self.hrows_since(b)
        st = self.target_state() or {}
        checks.append({"name": "a secret-shaped text is refused before any input",
                       "value": {"rows": hands_rows(rows, tool="type"), "inputs": len(self.input_runs(t0)), "text": st.get("text")},
                       "pass": bool(hands_rows(rows, tool="type", outcome="refused")) and not self.input_runs(t0)
                       and not st.get("text")})
        self.hcli("stop")
        # the Hyakki app taking focus mid-run pauses (self), and nothing types into it
        m, lm, b, n, t0 = self.begin_window(GOALS["long"], sub="slow", delay=2)
        aw = self.app_window()
        t = time.time()
        self.focus(aw)
        p = self.watch.wait("hands", lambda d: d.get("phase") == "paused" and d.get("pause") in ("self", "focus"), m, 6, None)
        time.sleep(4.0)
        runs = [r for r in self.input_runs(t) if r["first"] > (p[0] or t) + 0.3]
        checks.append({"name": "the Hyakki app taking focus mid-run pauses at once; nothing typed after",
                       "value": {"frame": p[1], "inputs_after": len(runs)}, "pass": p[1] is not None and not runs})
        self.hcli("stop")
        return {"checks": checks}, self.checks_pass(checks)

    # --- step 9: risky keys ---

    def keys15(self):
        checks, runs = [], {}
        # Safe keys need a positive action proof too; only testing the risky
        # handoffs could leave the entire press executor broken unnoticed.
        m, lm, b, n, t0 = self.begin_window('Press Tab')
        self.ended(m, 90)
        rows = self.hrows_since(b)
        pressed = [wtype_key(r["argv"]) for r in self.input_runs(t0) if wtype_key(r["argv"])]
        checks.append({"name": "safe Tab is pressed once in the authorized fixture window",
                       "value": {"rows": hands_rows(rows, tool="press"), "keys": pressed},
                       "pass": len(hands_rows(rows, tool="press", outcome="done")) == 1 and pressed == ["Tab"]})
        for chord in ("Return", "ctrl+w", "Delete", "alt+F4", "ctrl+v", "super+d"):
            m, lm, b, n, t0 = self.begin_window(GOALS["type"], sub="keys:" + chord)
            time.sleep(8.0)
            rows = self.hrows_since(b)
            pressed = [r["argv"] for r in self.input_runs(t0) if wtype_key(r["argv"])]
            runs[chord] = {"rows": hands_rows(rows, tool="press"), "pressed": pressed}
            checks.append({"name": f"press {chord}: handed off or refused, never pressed",
                           "value": runs[chord],
                           "pass": bool(hands_rows(rows, tool="press")) and not hands_rows(rows, tool="press", outcome="done")
                           and not pressed})
            if chord == "Return":
                card = self.consents(m)
                u = self.hutter("yes")
                o = self.hopen_say("yes")
                ok, msg = self.try_call("oni.hands", {"action": "yes"})
                time.sleep(2.0)
                pressed = [r["argv"] for r in self.input_runs(t0) if wtype_key(r["argv"])]
                checks.append({"name": "a spoken yes (chord and open mic) gets the needs-button line and no key; "
                                       "the CLI's yes is forbidden",
                               "value": {"card": bool(card), "chord": u.get("lines"), "open": o.get("lines"), "cli": ok},
                               "pass": not ok and not pressed and any(self.firm_line in (l or "") for l in
                                                                      (u.get("lines") or []) + (o.get("lines") or []))})
            self.hcli("stop")
        return {"runs": runs, "checks": checks}, self.checks_pass(checks)

    # --- step 10: stop ---

    def stop_case15(self, how, do, scope="window", typing=False):
        text_before = (self.target_state() or {}).get("text", "")
        typing_observation = None
        if scope == "window":
            m, lm, b, n, t0 = self.begin_window(GOALS["long"], sub="script" if typing else "slow",
                                                delay=None if typing else 20)
        else:
            m, lm, b, n, t0 = self.begin_nested(GOALS["type"], sub="slow", delay=20)
        if typing:
            deadline = time.monotonic() + 30
            while time.monotonic() < deadline:
                children = [r for r in self.vchildren.alive(("wtype",))
                            if r["first"] >= t0 and self.on_lab_display(r) and wtype_allowed(r["argv"])]
                changed = (self.target_state() or {}).get("text", "") != text_before
                if changed and children:
                    typing_observation = {"text_changed": True, "private_live_children": [r["pid"] for r in children]}
                    break
                time.sleep(0.01)
            if typing_observation is None:
                raise RuntimeError("no changed target with a live private typing child before Esc")
        else:
            self.wait_phase(m, ("working",), 10)
            time.sleep(3.0)
        t = time.time()
        do()
        ina = self.wait_inactive(m, 10)
        time.sleep(3.0)
        rows = self.hrows_since(b)
        h = stop_handler(self.ledger.since(lm))
        aborted = [r for r in self.app_records(n) if r.get("aborted")]
        after = [r for r in self.input_runs(t) if r["first"] > (ina[0] or t) + 0.3]
        return {"how": how, "latency_s": None if ina[0] is None else round(ina[0] - t, 3),
                "stop_rows": hands_rows(rows, tool="stop"), "handler": h, "aborted": len(aborted),
                "typing_observation": typing_observation,
                "inputs_after": len(after), "inactive": ina[1], "nest": bool(self.nest_procs())}

    def stop15(self):
        checks, cases = [], {}
        budget = self.hspec["stopBudgetMs"]

        def judge(c, reason, abort=True, glow=True):
            row = (c["stop_rows"] or [{}])[-1]
            h = c["handler"] or {}
            out = [
                {"name": f"{c['how']}: the inactive frame, nothing drawn after", "value": {"latency_s": c["latency_s"], "handler": h},
                 "pass": c["inactive"] is not None and (not glow or "glow" in (h.get("hides") or [])) and h.get("drawn_after") == 0},
                {"name": f"{c['how']}: a stop row ({reason}) within hands.stopBudgetMs", "value": row,
                 "pass": row.get("reason") == reason and isinstance(row.get("ms"), (int, float)) and row["ms"] <= budget},
                {"name": f"{c['how']}: no input after the stop" + (", the model request aborted" if abort else ""),
                 "value": {"aborted": c["aborted"], "inputs_after": c["inputs_after"]},
                 "pass": (c["aborted"] >= 1 or not abort) and not c["inputs_after"]},
            ]
            return out
        cases["esc_slow"] = self.stop_case15("Esc in a slow turn", lambda: self.stopw.send("press"))
        checks += judge(cases["esc_slow"], "esc")
        cases["esc_typing"] = self.stop_case15("Esc while typing", lambda: self.stopw.send("press"), typing=True)
        checks += judge(cases["esc_typing"], "esc", abort=False)
        cases["pill"] = self.stop_case15("the pill's Stop", lambda: self.surface_button(PARTS["pill"], "stop"))
        checks += judge(cases["pill"], "pill")
        cases["cli"] = self.stop_case15("hyakki oni hands stop", lambda: self.hcli("stop"))
        checks += judge(cases["cli"], "cli")
        cases["voice"] = self.stop_case15("open-mic \"stop\"", lambda: self.hopen_say("stop"))
        checks += judge(cases["voice"], "voice")
        cases["settings"] = self.stop_case15("inspect off", lambda: self.settings(inspect=False))
        self.settings(inspect=True)
        checks += judge(cases["settings"], "settings")
        # outside a tab a lost presenter pauses (presenter); only a button goes on
        m, lm, b, n, t0 = self.begin_window(GOALS["long"], sub="slow", delay=20)
        self.wait_phase(m, ("working",), 10)
        t = time.time()
        self.sh("ctl.sh", "surface", "kill")
        p = self.pause_of(m, "presenter", 8)
        time.sleep(4.0)
        after = [r for r in self.input_runs(t) if r["first"] > (p[0] or t) + 0.3]
        cases["presenter"] = {"frame": p[1], "latency_s": None if p[0] is None else round(p[0] - t, 3),
                              "inputs_after": len(after)}
        checks.append({"name": "a killed presenter pauses the window task (presenter), no input after",
                       "value": cases["presenter"], "pass": p[1] is not None and not after})
        self.hcli("stop")
        # a presenter again for the steps after (Hands starts only with one drawn)
        self.sh("ctl.sh", "surface", "restart", check=False, timeout=120)
        time.sleep(2.0)
        if self.keys:
            self.keys.close()
        fifo = self.sh("voice.sh", "keys").stdout.strip().splitlines()[-1]
        self.keys = KeyWriter(fifo)
        cases["nested_esc"] = self.stop_case15("Esc on the separate desktop", lambda: self.stopw.send("press"), scope="nested")
        checks += judge(cases["nested_esc"], "esc", glow=False)
        gone = self.wait_nest(False, self.dspec["stopGraceS"] + 10)
        checks.append({"name": "a stopped separate-desktop task ends its desktop", "value": not gone, "pass": gone})
        r1, r2 = self.hcli("stop"), self.hcli("stop")
        checks.append({"name": "stop twice is harmless", "value": [r1[0], r2[0]], "pass": r1[0] == 0 and r2[0] == 0})
        missed = [s for s in self.stopw.log if s.get("error")]
        checks.append({"name": "the Esc FIFO had a reader while a task ran", "value": self.stopw.log[-5:], "pass": not missed})
        return {"cases": cases, "checks": checks}, self.checks_pass(checks)

    # --- step 11: executables shown, never run ---

    def files15(self):
        checks = []
        (self.aout / "files.jsonl").unlink(missing_ok=True)
        m, lm, b, n, t0 = self.begin_nested(GOALS["files"])
        ina = self.ended(m, 300)
        rows = self.hrows_since(b)
        shows = self.files_log()
        inside = str(self.folder) + "/"
        ran = [r for r in self.vchildren.seen.values() if r["first"] >= t0 and any(a.startswith(inside) for a in r["argv"])]
        runnable = [f for f in self.files if f["runnable"]]
        checks += [
            {"name": "find_files found the fixture files and show_file ran once per file",
             "value": hands_rows(rows, tool=("find_files", "show_file")),
             "pass": bool(hands_rows(rows, tool="find_files", outcome="done"))
             and len(hands_rows(rows, tool="show_file", outcome="done")) == len(self.files)},
            {"name": "Lab Files was given the folder alone, every time", "value": shows,
             "pass": show_argvs_ok(shows, self.folder) and len(shows) == len(self.files)},
            {"name": "no fixture executable ran (no ran- marker)", "value": self.ran_markers(),
             "pass": not self.ran_markers()},
            {"name": "no daemon child named a file inside HandsFiles", "value": [r["argv"] for r in ran],
             "pass": not ran},
            {"name": f"the task ended ({len(runnable)} runnable kinds shown by their folder)", "value": ina[1],
             "pass": ina[1] is not None},
        ]
        # a traversal query is refused
        m, lm, b, n, t0 = self.begin_nested(GOALS["outside"])
        self.ended(m, 120)
        rows = self.hrows_since(b)
        checks.append({"name": "a find outside the chosen folders (traversal) is refused, nothing shown",
                       "value": hands_rows(rows, tool="find_files"),
                       "pass": not hands_rows(rows, tool="find_files", outcome="done") and len(self.files_log()) == len(shows)})
        return {"checks": checks}, self.checks_pass(checks)

    # --- step 12: restart hold ---

    def restart15(self):
        checks, notes = [], {}
        for scope in ("window", "desktop"):
            if scope == "window":
                m, lm, b, n, t0 = self.begin_window(GOALS["long"], sub="slow", delay=20)
            else:
                m, lm, b, n, t0 = self.begin_nested(GOALS["type"], sub="slow", delay=20)
                self.wait_nest(True, self.dspec["readyS"] + 10)
            task = self.hstatus().get("task")
            self.watch.pause()
            try:
                self.daemon_kill()
                gone = self.wait_nest(False, 15)
                t_restart = time.time()
                self.daemon_start()
                time.sleep(10.0)
            finally:
                self.watch.resume()
            st = self.hstatus()
            tasks = self.tasks()
            row = next((t for t in tasks if t.get("id") == task), None)
            started = [r for r in self.vchildren.seen.values() if r["first"] >= t_restart
                       and (r["comm"] in handskit.INPUT_PROGRAMS or nest_kind(r["argv"], self.nest, self.dspec))]
            notes[scope] = {"task": task, "row": row, "status": st, "started": [r["argv"][:2] for r in started]}
            checks += [
                {"name": f"{scope}: kill -9 mid-task; the separate desktop died with it", "value": gone, "pass": gone},
                {"name": f"{scope}: after the restart the task's row reads interrupted (restart)", "value": row,
                 "pass": bool(row) and row.get("state") == WIRE["interrupted"] and row.get("reason") == WIRE["restart_reason"]},
                {"name": f"{scope}: nothing is running and nothing resumed", "value": st,
                 "pass": st.get("state") not in ("working", "waiting", "paused")},
                {"name": f"{scope}: the restart started no input program and no separate desktop",
                 "value": notes[scope]["started"], "pass": not started},
                {"name": f"{scope}: the restarted daemon holds no /dev/input fd", "value": self.dev_input_fds(),
                 "pass": not self.dev_input_fds()},
            ]
        # voice and the CLI cannot start it; the app can
        n0 = time.time()
        u = self.hutter("keep_going")
        o = self.hopen_say("keep_going")
        rr = self.cli("oni", "hands", "start", stdin="", timeout=60)
        st = self.hstatus()
        checks.append({"name": "the chord, open mic and the CLI start nothing after a restart",
                       "value": {"chord": u.get("heard"), "open": o.get("heard"), "cli": rr.returncode, "status": st},
                       "pass": rr.returncode == 2 and st.get("state") not in ("working", "waiting", "paused")
                       and not self.input_runs(n0)})
        self.close_win(TARGET)
        self.ensure_win("target")
        m, lm, b, n, t0 = self.begin_window(GOALS["type"])
        stt = self.wait_text(TYPED, 120)
        self.ended(m, 120)
        checks.append({"name": "the app's pick and Start run a fresh task", "value": stt, "pass": (stt or {}).get("text") == TYPED})
        raw = self.journal_raw()
        checks += [
            {"name": "the journal is 0600 in a 0700 dir", "value": [file_mode(self.journal), file_mode(self.journal.parent)],
             "pass": file_mode(self.journal) == 0o600 and file_mode(self.journal.parent) == 0o700},
            {"name": "the journal holds no goal, title or typed word",
             "value": words_in(raw or "", SECRET_WORDS15 + tuple(GOALS.values()) + ("Kansai notes",)),
             "pass": raw is not None and not words_in(raw, SECRET_WORDS15 + tuple(GOALS.values()) + ("Kansai notes",))},
        ]
        return {"notes": notes, "checks": checks}, self.checks_pass(checks)

    # --- step 13: the queue ---

    def queue15(self):
        checks = []
        q = self.tspec["queueMax"]
        m, lm, b, n, t0 = self.begin_window(GOALS["long"], sub="slow", delay=10)
        queued = []
        for i in range(q + 1):
            mk, _ = self.start15(GOALS["type"], WIRE["scope_window"], queued=True)
            self.focus_app_id(TARGET)
            queued.append([t.get("state") for t in self.tasks()])
        states = [t.get("state") for t in self.tasks()]
        checks.append({"name": f"one running plus {q} up next; a further Start is refused",
                       "value": {"after_each": queued, "now": states},
                       "pass": states.count("upnext") == q and states.count("working") + states.count("checking") <= 1})
        self.hcli("stop")
        time.sleep(3.0)
        st = self.hstatus()
        checks.append({"name": "the next task does not start by itself after a stop", "value": st,
                       "pass": st.get("state") not in ("working", "waiting")})
        self.click_label(LABELS["clear"], "clear-queue")
        self.click_label(LABELS["clear_yes"], "clear-queue-confirm")
        deadline = time.monotonic() + 8
        while self.tasks():
            if time.monotonic() >= deadline:
                raise RuntimeError("the app did not clear the waiting task before the independent error case")
            time.sleep(0.1)
        # a Hands error is not retried
        self.hcli("stop")
        m, lm, b, n, t0 = self.begin_window(GOALS["type"], sub="fail")
        ina = self.ended(m, 60)
        recs = self.app_records(n)
        checks.append({"name": "a model 500 ends the task, asked once, not retried", "value": {"end": (ina[1] or {}).get("end"),
                                                                                              "turns": len(recs)},
                       "pass": ina[1] is not None and len(recs) == 1})
        self.amode("script")
        return {"checks": checks}, self.checks_pass(checks)

    # --- step 14: harness approvals ---

    def approvals15(self):
        checks = []
        self.hcli("stop")
        self.stage("app")
        caps = self.caps()
        tasks0, t0 = len(self.tasks()), time.time()
        for scope in ("window", "nested", "desktop"):
            ok, error = self.wire_call(APPROVAL["method"], {"session": "lab-forged", "token": "0" * 32,
                                                           "goal": "Type hello", "scope": scope})
            checks.append({"name": f"a teams.agent_* call ({scope}) on the main socket is forbidden",
                           "value": [ok, error], "pass": not ok and error.get("code") == "forbidden"})
        for pre in APPROVAL["id_prefixes"]:
            ok, error = self.wire_call("oni.consent.answer", {"id": pre + "1", "answer": "once"})
            checks.append({"name": f"a CLI answer to a {pre}N Hands approval is forbidden (the caller is checked first)",
                           "value": [ok, error], "pass": not ok and error.get("code") == "forbidden"})
        ok, msg = self.try_call("teams.approval_decide", {"id": "ta-1", "decision": "approve"})
        checks.append({"name": "teams.approval_decide from the CLI is refused (forbidden, or not_found for no such id)",
                       "value": [ok, str(msg)[-200:]], "pass": not ok})
        time.sleep(2.0)
        checks.append({"name": "none of that raised a task, a card or an input",
                       "value": {"tasks": len(self.tasks()) - tasks0, "inputs": len(self.input_runs(t0)),
                                 "cards": len(self.watch.frames(APPROVAL["frame"], 0))},
                       "pass": len(self.tasks()) == tasks0 and not self.input_runs(t0)
                       and not [d for _, d in self.watch.frames(APPROVAL["frame"], 0) if d.get("open")]})
        frames = self.watch.frames(APPROVAL["frame"], 0)
        checks.append({"name": "no approval frame ever names the desktop scope (a member cannot ask for it)",
                       "value": [d for _, d in frames if d.get("scope") == "desktop"],
                       "pass": not [d for _, d in frames if d.get("scope") == "desktop"]})
        self.limit(14, "a Yokai-hosted member", "teams.create and teams.task_create are the app's alone and the lab "
                       "does not script the app's Teams pages; a hosted member's request_hands, the card's once/no/app, "
                       "a desktop request refused, task B unable to use task A's grant, and the grant expiring "
                       "(approval.expire) after a restart are proven by the teams and daemon approval tests over the "
                       "socket with fakes. appmodel.py already answers a request_hands turn for when the lab can host one")
        return {"caps": {k: v for k, v in caps.items() if k.startswith("hands")}, "checks": checks}, self.checks_pass(checks)

    # --- step 15: exclusive stages ---

    def exclusive15(self):
        checks = []
        self.hcli("stop")
        self.stage("app")
        caps = self.caps()
        checks.append({"name": "the app stage: hands.desktop denied", "value": caps.get("hands.desktop"),
                       "pass": not (caps.get("hands.desktop") or {}).get("allow")})
        self.stage("desktop")
        caps = self.caps()
        checks.append({"name": "the desktop stage: hands.app denied", "value": caps.get("hands.app"),
                       "pass": not (caps.get("hands.app") or {}).get("allow")})
        self.stage("browser")
        caps = self.caps()
        checks.append({"name": "the browser stage: hands.app and hands.desktop both denied",
                       "value": {k: (caps.get(k) or {}).get("allow") for k in ("hands.app", "hands.desktop")},
                       "pass": not any((caps.get(k) or {}).get("allow") for k in ("hands.app", "hands.desktop"))})
        self.stage("app")
        checks.append({"name": "choosing the app stage never turns the desktop stage on",
                       "value": self.oni_settings().get("hands"), "pass": self.oni_settings().get("hands") == "app"})
        return {"checks": checks}, self.checks_pass(checks)

    # --- step 16: protocol ---

    def protocol15(self):
        checks, runs = [], {}
        rested = self.wait_provider_rested()
        if not rested["rested"]:
            raise RuntimeError(f"the lab provider is still resting: {rested}")
        expect = {"two": ("multiple",), "invented": ("invented",), "plan": ("invented", "shape"),
                  "permission": ("permission",), "badjson": ("json",), "foreign": ("invented",)}
        for sub, reasons in expect.items():
            self.model.reset()
            m, lm, b, n, t0 = self.begin_window(GOALS["type"], sub=sub)
            ina = self.ended(m, 90)
            rows = self.hrows_since(b)
            recs = self.app_records(0)
            runs[sub] = {"rows": rows, "turns": len(recs), "end": (ina[1] or {}).get("end")}
            checks.append({"name": f"{sub}: refused ({'/'.join(reasons)}), no input, at most one retry",
                           "value": runs[sub],
                           "pass": bool([r for r in rows if r.get("outcome") == "refused" and r.get("reason") in reasons])
                           and not self.input_runs(t0) and len(recs) <= 2})
        self.model.reset()
        m, lm, b, n, t0 = self.begin_window(GOALS["type"], sub="hostile")
        self.ended(m, 90)
        recs = self.app_records(0)
        rows = self.hrows_since(b)
        pressed = [r["argv"] for r in self.input_runs(t0) if wtype_key(r["argv"]) == "Return"]
        checks.append({"name": "hostile: Return is never pressed and the tool call runs nothing",
                       "value": {"rows": hands_rows(rows, tool="press"), "pressed": pressed}, "pass": not pressed})
        for sub, end in (("repeat", "repeat"), ("endless", "budget")):
            self.model.reset()
            m, lm, b, n, t0 = self.begin_window(GOALS["type"], sub=sub)
            ina = self.ended(m, 300)
            runs[sub] = {"end": (ina[1] or {}).get("end"), "turns": len(self.app_records(0))}
            if sub == "endless":
                rows = self.hrows_since(b)
                checks.append({"name": "the scrolling budget case exercised the real scroll action",
                               "value": hands_rows(rows, tool="scroll"),
                               "pass": bool(hands_rows(rows, tool="scroll", outcome="done"))})
            checks.append({"name": f"{sub}: ends {end}", "value": runs[sub], "pass": (ina[1] or {}).get("end") == end})
        self.amode("script")
        self.limit(16, "an unread typed text", "a done after typed text that read back no or unknown, and the "
                       "second unknown ending the task (unread), are the loop's and the driver's tests; the lab's "
                       "fixtures always read back")
        return {"provider_rest_before": rested, "runs": runs, "checks": checks}, self.checks_pass(checks)

    # --- step 17: boundaries ---

    def boundaries15(self):
        oni_hits = source_hits(APP / "internal/oni", labkit.INPUT_MARKS + ("input.dispatch",))
        hands_hits = source_hits(APP / "internal/hands", handskit.HANDS_INPUT_MARKS, skip=(APP / "internal/hands/input",))
        tests = {pkg: self.run_test(pkg, rel, run) for pkg, rel, run in TEST_PKGS15}
        seen = list(self.vchildren.seen.values())
        observed = {s["pid"]: s for s in seen}
        bad = [s for s in seen if not child_allowed15(s, self.ctx) and not handskit.child_allowed14(
            s, self.real, self.profile, self.starts, observed)]
        checks = [
            {"name": "no input-tool name in non-test internal/oni", "value": oni_hits, "pass": not oni_hits},
            {"name": "no input-tool name in internal/hands outside internal/hands/input", "value": hands_hits,
             "pass": not hands_hits},
        ]
        checks += [{"name": f"{pkg}.test ({rel})", "value": tests.get(pkg), "pass": bool(tests.get(pkg, {}).get("pass"))}
                   for pkg, rel, _ in TEST_PKGS15]
        checks.append({"name": "every program the daemon ran is on the Phase 15 allowlist",
                       "value": {"seen": len(seen), "outside": [{"argv": s["argv"][:6], "comm": s["comm"]} for s in bad[:20]],
                                 "exited_unread": len(self.vchildren.unread)},
                       "pass": bool(seen) and not bad})
        return {"tests": tests, "checks": checks}, self.checks_pass(checks)

    # --- step 18: disk ---

    def disk15(self):
        skip = [str(self.lab / p) for p in ("voice", "data/hyakki/oni/voice", "log", "keys", "build", "out", "home",
                                             "nest")]
        # The compositor's own diagnostic records its windows' titles.
        skip.extend(str(p) for p in (self.lab / "w" / "hypr").glob("*/hyprland.log"))
        words = SECRET_WORDS15 + tuple(GOALS.values()) + ("Thieved title",)
        hits, scanned = tree_words(str(self.lab), words, skip=tuple(skip), since=self.start_wall - 1)
        tmp = self.lab / "tmp"
        left = sorted(str(p.relative_to(tmp)) for p in tmp.rglob("*")) if tmp.is_dir() else None
        rows = self.hlog()
        tasks = self.tasks()
        checks = [
            {"name": "no picture, OCR text, title, goal or typed word in any Hyakki file (fixtures' own files aside)",
             "value": {"scanned": scanned, "hits": hits[:20]}, "pass": not hits},
            {"name": "the Hands ledger holds codes only", "value": words_in(rows, words), "pass": not words_in(rows, words)},
            {"name": "the task list holds codes only", "value": words_in(tasks, words), "pass": not words_in(tasks, words)},
            {"name": "the lab processes' temp dir is empty", "value": left, "pass": left == []},
        ]
        self.limit(18, "apps on the separate desktop", "an app Hands opens on the separate desktop keeps its own "
                       "state under the nested HOME; it is outside Hyakki's scrub claim and goes with the task")
        return {"checks": checks}, self.checks_pass(checks)

    # --- step 19: init and compositor ---

    def init15(self):
        stub_bins = {}
        for name in ("systemctl", "ryoku", "ryoku-host", "systemd-run"):
            p = self.lab / "bin" / name
            try:
                stub_bins[name] = p.is_file() and not p.is_symlink() and p.read_bytes()[:4] == b"\x7fELF"
            except OSError:
                stub_bins[name] = False
        argv = {k: r["argv"] for r, k in self.nest_procs()}
        weston = [r["argv"] for r in self.vchildren.seen.values() if nest_kind(r["argv"], self.nest, self.dspec) == "weston"]
        checks = [
            {"name": "the lab's refusal stubs (compiled, not links) stand first on every lab process's PATH",
             "value": {"bins": stub_bins, "refused": self.stubbed()[-10:]}, "pass": all(stub_bins.values())},
            {"name": f"the separate desktop was niri on weston under {self.wm}", "value": weston[:1],
             "pass": bool(weston)},
        ]
        self.limit(19, "runit", "the separate desktop is the daemon's own child under either init; the runit path "
                       "is proven by the launch and unit tests, not a booted runit")
        return {"argv_now": argv, "checks": checks}, self.checks_pass(checks)

    # --- step 20: settlement ---

    def settle15(self):
        th = labkit.THRESHOLDS

        def within(rec):
            rates = self.daemon_rates(rec)
            return all(v["cpu_percent"] is not None and v["cpu_percent"] < th["cpu_percent_max"]
                       and v["wakeups_per_s"] is not None and v["wakeups_per_s"] <= th["wakeups_per_s_max"]
                       for v in rates), rates
        checks = []
        anykey = str(self.anyw.path)
        # app stage, a window picked, no task
        self.hcli("stop")
        self.ensure_win("target")
        self.stage("app")
        if self.hstatus().get("app") != TARGET:
            self.pick(TARGET)
        self.wake()
        self.ledger.quiet(5.0, 120)
        # start the unchanged idle timer after the wake animation has settled
        self.call("oni.interact", {"kind": "wake"})
        idle = self.measure("settled", timeout=120, seconds=60, name="app-picked")
        ok, r = within(idle)
        checks.append({"name": "picked, no task: 0 commits, no any-key reader open, the daemon within budget",
                       "value": {"ledger": idle.get("ledger_window"), "daemon": r, "anykey_open": anykey in self.oni_fds()},
                       "pass": bool(idle.get("pass")) and ok and anykey not in self.oni_fds()})
        # paused
        m, lm, b, n, t0 = self.begin_window(GOALS["long"], sub="slow", delay=3)
        self.focus_app_id(SENTINEL)
        if self.pause_of(m, "focus", 6)[1] is None:
            raise RuntimeError("the paused settlement sample has no actual focus pause")
        self.ledger.quiet(5.0, 60)
        paused = self.measure("settled", timeout=120, seconds=30, name="app-paused")
        ok, r = within(paused)
        checks.append({"name": "paused: 0 commits (no glow tick), the daemon within budget",
                       "value": {"ledger": paused.get("ledger_window"), "daemon": r}, "pass": bool(paused.get("pass")) and ok})
        self.hcli("stop")
        time.sleep(2.0)
        ng = self.no_glow_layer()
        self.ledger.quiet(5.0, 120)
        after = self.measure("settled", timeout=120, seconds=60, name="app-stopped")
        ok, r = within(after)
        checks.append({"name": "after stop: no glow layer, no any-key reader, no separate desktop, 0 commits",
                       "value": {"no_glow_layer": ng, "anykey_open": anykey in self.oni_fds(), "nest": bool(self.nest_procs()),
                                 "ledger": after.get("ledger_window"), "daemon": r},
                       "pass": ng is not False and anykey not in self.oni_fds() and not self.nest_procs()
                       and bool(after.get("pass")) and ok})
        self.stage("off")
        time.sleep(2.0)
        self.ledger.quiet(5.0, 120)
        self.wake()
        self.ledger.quiet(5.0, 120)
        self.call("oni.interact", {"kind": "wake"})
        off = self.measure("settled", timeout=120, seconds=60, name="hands-off15")
        ok, r = within(off)
        checks.append({"name": "hands off: 0 commits, the daemon within budget",
                       "value": {"ledger": off.get("ledger_window"), "daemon": r}, "pass": bool(off.get("pass")) and ok})
        self.stage("app")
        # the masks: every glow strip the surface drew was registered with oni.overlay
        lines = self.ledger.since()
        glows = [l for l in lines if l.get("ev") == "commit" and l.get("part") == PARTS["glow"]]
        masks = [l for l in lines if l.get("ev") == PARTS["overlay_ev"]]
        unmasked = []
        for g in glows[-20:]:
            rects = g.get("rects") or []
            covered = [m2 for m2 in masks if m2.get("output") == g.get("output")]
            if rects and not any(all(r in (m2.get("rects") or []) for r in rects) for m2 in covered):
                unmasked.append({"output": g.get("output"), "t": g.get("t")})
        checks.append({"name": "every glow strip drawn was registered as a capture mask (ev=overlay)",
                       "value": {"glows": len(glows), "masks": len(masks), "unmasked": unmasked[:5]},
                       "pass": bool(glows) and not unmasked})
        return {"checks": checks}, self.checks_pass(checks)

    # --- step 21: reduced motion ---

    def reduced15(self):
        before = self.oni_settings().get("reduceMotion", "follow")
        self.settings(reduceMotion="on")
        try:
            m, lm, b, n, t0 = self.begin_window(GOALS["long"], sub="slow", delay=8)
            time.sleep(4.0)
            g = glow_stats(self.ledger.since(lm))
            sm = self.ledger.mark()
            self.hcli("stop")
            self.wait_inactive(m, 10)
            time.sleep(1.0)
            h = stop_handler(self.ledger.since(sm))
        finally:
            self.settings(reduceMotion=before)
            self.amode("script")
        checks = [
            {"name": "reduce motion: one glow commit per output while the phase holds", "value": g,
             "pass": bool(g) and all(v["commits"] <= 1 for v in g.values())},
            {"name": "reduce motion: stop still takes the glow down in one handler", "value": h,
             "pass": h is not None and "glow" in (h.get("hides") or []) and h.get("drawn_after") == 0},
        ]
        return {"checks": checks}, self.checks_pass(checks)

    def display15(self):
        details, ok = self.display_check()
        nested = [r for r in self.vchildren.seen.values() if (r.get("env") or {}).get("XDG_RUNTIME_DIR") == str(self.nest)]
        wd = sorted({(r.get("env") or {}).get("WAYLAND_DISPLAY", "") for r in nested})
        details["nested_displays"] = wd
        live = self.env.get("LAB_DISPLAY", "")
        nested_ok = bool(nested) and all(
            (bool(w := (r.get("env") or {}).get("WAYLAND_DISPLAY", ""))
             and w != live and not w.startswith(self.env.get("LAB_WRT", "//")))
            or (not w and nest_kind(r["argv"], str(self.nest), self.dspec) == "weston"
                and env_isolated(r.get("env") or {}, self.nest, self.live_dirs)[0])
            for r in nested)
        return details, ok and nested_ok

    def run(self):
        if self.model is None:
            self.results["steps"].append({"step": 0, "name": "lab model", "pass": False,
                                          "error": "no LAB_MODEL_URL: phase15 needs up.sh --model"})
            self.save()
            return 1
        self.vchildren.start()
        try:
            self.step(0, "census", self.census15)
            if self.keys is None:
                raise RuntimeError("no chord FIFO: the rest needs step 0")
            self.step(1, "refusals", self.refusals15)
            self.step(2, "separate desktop up and down", self.desktop_up15)
            self.step(3, "separate desktop type", self.desktop_type15)
            self.step(4, "separate desktop click", self.desktop_click15)
            self.step(5, "window picker", self.pick15)
            self.step(6, "window type", self.window_type15)
            self.step(7, "theft pauses", self.theft15)
            self.step(8, "private and self", self.private15)
            self.step(9, "risky keys", self.keys15)
            self.step(10, "stop", self.stop15)
            self.step(11, "executables shown, never run", self.files15)
            self.step(12, "restart hold", self.restart15)
            self.step(13, "queue", self.queue15)
            self.step(14, "harness approvals", self.approvals15)
            self.step(15, "exclusive stages", self.exclusive15)
            self.step(16, "protocol", self.protocol15)
            self.step(17, "boundaries", self.boundaries15)
            self.step(18, "disk", self.disk15)
            self.step(19, "init and compositor", self.init15)
            self.step(20, "settlement", self.settle15)
            self.step(21, "reduced motion", self.reduced15)
            self.step(22, "ledger display is the lab's", self.display15)
        except RuntimeError as e:
            print(f"phase15: stopped: {e}")
        finally:
            try:
                self.hcli("stop")
            except (OSError, RuntimeError, ValueError, subprocess.TimeoutExpired):
                pass
            self.vchildren.done.set()
            self.vchildren.join(timeout=5)
            self.watch.stop()
            if self.clock:
                self.clock.done.set()
            if self.keys:
                self.keys.close()
                self.results["keys_sent"] = self.keys.log
            self.results["stop_keys_sent"] = self.stopw.log
            self.results["anykey_sent"] = self.anyw.log
            self.results["daemon_programs"] = [dict(r, argv=r["argv"][:8]) for r in self.vchildren.seen.values()]
            with (self.out / "frames.jsonl").open("w") as fh:
                for l in self.watch.lines:
                    fh.write(json.dumps(l) + "\n")
            self.results["frame_watch_restarts"] = self.watch.restarts
            try:
                (self.out / "model" / "requests.json").write_text(json.dumps(self.model.requests(), indent=2))
            except OSError:
                pass
        self.limit(22, "fixtures", "the model and the windows are loopback fixtures: they prove the contract, the "
                       "window discipline, consent, isolation and the stop paths, not model quality")
        self.limit(22, "OCR read-back", "read-back is as good as tesseract on the lab's fonts")
        self.limit(22, "focus race", "a live action cannot activate a window: the race between a check and the act "
                       "is narrowed to the chunk, not closed")
        if self.wm == "niri":
            self.limit(22, "outputs", "nested niri has one output; the glow on the picked window's output alone is "
                           "proven on Hyprland's two")
        self.results["finished_at"] = now()
        self.save()
        for limit in self.results.get("limits", []):
            print(f"limit (step {limit['step']}): {limit['check']}: {limit['reason']}")
        return 0 if self.results["pass"] else 1


def main(argv):
    if not argv:
        raise SystemExit(__doc__)
    cmd, args = argv[0], argv[1:]
    if cmd == "selftest" and not args:
        return selftest()
    if cmd == "phase15" and len(args) == 2:
        return Phase15(*args).run()
    raise SystemExit(__doc__)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
