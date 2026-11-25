#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Phase 17 real-app acceptance, not a demo or a source-pattern test.

Run through phase17.sh on frozen production binaries. UI mutations use OCR
of private grim pictures and input.sh's nested-compositor virtual pointer.
Readback uses the real daemon. No app identity is forged on a raw socket.
No model, account, microphone, speakers or live desktop is used.

Required frozen test executables (go test -c -o BIN/NAME.test ./PACKAGE):
protocol internal/protocol; daemon internal/daemon; settings
internal/app/pages/settings; shell internal/app/shell; mcpserve internal/mcpserve.
The tests supplement, and cannot replace, any real runtime step below.
"""
import datetime
import hashlib
import json
import os
from pathlib import Path
import queue
import shutil
import subprocess
import sys
import threading
import time

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import labkit
import handskit
from voicekit import VoiceChildWatch

ROOT = HERE.parents[4]
APP_ID = "dev.ryoku.Hyakki"
CLIENT = "Phase17 local fixture"
NOTE = "I like amber lanterns beside my desk."
NOTICE = "The local lantern check is ready"
TESTS = {
    "protocol": ("internal/protocol", "(?i)(mcp|component|activity|onboard|oni)"),
    "daemon": ("internal/daemon", "(?i)(mcp|parts|component|activity|onboard|appalone|settings.*app)"),
    "settings": ("internal/app/pages/settings", "(?i)(section|duplicate|oni|parts|privacy|mcp|route)"),
    "shell": ("internal/app/shell", "(?i)(onboarding|desktop.*look|oni.*settings|settings.*(route|section|duplicate|unique))"),
    "mcpserve": ("internal/mcpserve", "."),
}
PAGES = {
    "oni": ("Presence", "Oni on the desktop"),
    "oni-voice": ("Voice and dictation", "Talk to Oni"),
    "oni-screen": ("Screen and guides", "Let Oni look at a window"),
    "oni-commands": ("Quick commands", "Reminders and timers"),
    "oni-writing": ("Writing and memory", "Recall what"),
    "oni-privacy": ("Privacy and tools", "Local activity"),
    "parts": ("Optional parts", "Parts on this computer"),
    "hands": ("Hands", "Hands in a browser"),
    "features": ("Features",), "providers": ("Providers",),
}
TOUR = ("Welcome to Hyakki", "Choose where your model runs", "A name, only if you want",
        "Your microphone, your choice", "Shortcuts without surprises", "Give Oni a place",
        "Try a guide safely", "You decide what leaves", "Make the first task yours")
TOOLS = {"reminders_list": "List reminders", "reminders_add": "Set reminders",
         "notes_search": "Search memory about you", "notes_add": "Suggest notes",
         "focus_status": "Read the focus guard", "notify": "Show a line on Oni",
         "screen_ask": "Ask about the perched window"}
ROW_AFTER = {
    "Oni on the desktop": ("Character",),
    "Recall what": ("Include the Ryoku notes vault",),
    "Quick commands": ("Reminders and timers",),
    "Reminders and timers": ("Routines",),
    "Use memory": ("Learn from conversations",),
    "Step aside for full screen": ("Sounds",),
    "Serve Oni's tools to other programs": ("List reminders",),
    **{label: (following,) for label, following in zip(tuple(TOOLS.values()), tuple(TOOLS.values())[1:])},
    "Ask about the perched window": (CLIENT, "Programs", "Connected programs"),
}


def digest(path):
    h = hashlib.sha256()
    with Path(path).open("rb") as f:
        for block in iter(lambda: f.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()


def snapshot_sources():
    paths = []
    for base in (ROOT / "ryoku/hyakki", ROOT / "docs/lithic/library/go", HERE):
        for directory, dirs, files in os.walk(base):
            dirs[:] = sorted(d for d in dirs if d not in (".git", "__pycache__", ".cache", "build", "node_modules"))
            for name in sorted(files):
                p = Path(directory) / name
                if p.suffix not in (".pyc", ".pyo"):
                    paths.append(p)
    return {str(p.relative_to(ROOT)): digest(p) for p in sorted(set(paths))}


def denied(reply):
    return bool(reply.get("error") or (reply.get("result") or {}).get("isError"))


def notice_phrase(words, phrase):
    # notices wrap, but words from different OCR readings never join
    want = [handskit.norm_word(word) for word in phrase.split()]
    readings = {}
    for word in words:
        readings.setdefault(word["line"][0], []).append(handskit.norm_word(word["text"]))
    return bool(want) and any(
        reading[i:i + len(want)] == want
        for reading in readings.values()
        for i in range(len(reading) - len(want) + 1))


class MCP:
    """A real hyakki mcp child; newline JSON-RPC, bounded read and teardown."""
    def __init__(self, gate, name=CLIENT):
        self.gate, self.name, self.seq = gate, name, 0
        self.rows, self.replies = [], queue.Queue()
        self.err = (gate.out / f"mcp-{len(gate.mcps)}.stderr").open("w")
        self.proc = subprocess.Popen([str(HERE / "phase17.sh"), "mcp"], stdin=subprocess.PIPE,
                                     stdout=subprocess.PIPE, stderr=self.err, text=True, bufsize=1)
        self.reader = threading.Thread(target=self.read, daemon=True)
        self.reader.start()
        gate.mcps.append(self)

    def read(self):
        try:
            for line in self.proc.stdout:
                try:
                    self.replies.put(json.loads(line))
                except ValueError:
                    self.replies.put({"invalid_stdout": line.rstrip()})
        finally:
            self.replies.put(None)

    def rpc(self, method, params=None, timeout=40):
        self.seq += 1
        request = {"jsonrpc": "2.0", "id": self.seq, "method": method, "params": params or {}}
        self.rows.append({"at": labkit.now(), "request": request})
        self.proc.stdin.write(json.dumps(request) + "\n")
        self.proc.stdin.flush()
        end = time.monotonic() + timeout
        while time.monotonic() < end:
            reply = self.replies.get(timeout=max(.01, end - time.monotonic()))
            self.rows.append({"at": labkit.now(), "reply": reply})
            if reply is None or "invalid_stdout" in reply:
                raise RuntimeError(f"MCP ended or emitted non-JSON stdout: {reply}")
            if reply.get("id") == self.seq:
                return reply
        raise RuntimeError(f"MCP timed out: {method}")

    def initialize(self):
        return self.rpc("initialize", {"protocolVersion": "2024-11-05", "capabilities": {},
                                       "clientInfo": {"name": self.name, "version": "lab"}})

    def tool(self, name, args=None):
        return self.rpc("tools/call", {"name": name, "arguments": args or {}})

    def close(self):
        if getattr(self, "closed", False):
            return
        self.closed = True
        if self.proc.poll() is None:
            self.proc.stdin.close()
            try:
                self.proc.wait(timeout=10)
            except subprocess.TimeoutExpired:
                self.proc.terminate()
                try:
                    self.proc.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    self.proc.kill()
                    self.proc.wait(timeout=5)
        self.err.close()
        self.gate.write("mcp-transcript.json", [m.rows for m in self.gate.mcps])


class Phase17(labkit.Phase8):
    REPORT = "phase17.json"
    # Reuse the existing avoidance geometry and pointer, not a second input path.
    avoid = handskit.Phase14.avoid
    part_line = handskit.Phase14.part_line
    click_box = handskit.Phase14.click_box

    def __init__(self, lab, out):
        super().__init__(lab, out)
        for name in ("crops", "tests", "fixtures"):
            (self.out / name).mkdir(parents=True, exist_ok=True)
        self.mcps, self.checks = [], []
        self.children = VoiceChildWatch(self.env["LAB_ID"])
        self.children.start()
        self.initial_sources = snapshot_sources()
        self.initial_bins = {}
        self.write("source-sha256.json", self.initial_sources)
        self.results["limits"] = ["Private nested desktop only; no packaged-box or booted-edition claim.",
                                  "No production provider, credential, microphone, speaker or live input.",
                                  "Desktop style propagation is driven by private Ryoku palette files; Appearance is read-only."]

    def write(self, name, value):
        (self.out / name).write_text(json.dumps(value, indent=2, default=labkit.jsonable) + "\n")

    def check(self, name, ok, value=None):
        self.checks.append({"name": name, "pass": bool(ok), "value": value})

    def done(self):
        checks, self.checks = self.checks, []
        return {"checks": checks}, bool(checks) and all(c["pass"] for c in checks)

    def step(self, number, name, fn):
        self.checks = []
        super().step(number, name, fn)
        if self.checks and "details" not in self.results["steps"][-1]:
            self.results["steps"][-1]["details"] = {"checks": self.checks}
            self.save()

    def settings_read(self):
        value = self.call("settings.get", {})
        return value.get("settings", value)

    def oni_settings(self):
        return self.settings_read().get("oni", {})

    def app_window(self, state=None):
        wins = [w for w in (state or self.state())["windows"] if w.get("app_id") == APP_ID]
        if len(wins) != 1:
            raise RuntimeError(f"expected one isolated Hyakki app, got {len(wins)}")
        return wins[0]

    def wait_until(self, read, pred, timeout=15):
        end = time.monotonic() + timeout
        while True:
            value = read()
            if pred(value):
                return value
            if time.monotonic() >= end:
                raise RuntimeError(f"readback never matched within {timeout}s: {value}")
            time.sleep(.2)

    def route(self, view):
        view = view.removeprefix("settings/")
        self.sh("phase17.sh", "ui", "route", view, timeout=60)
        self.current_view = view
        time.sleep(.8)
        return self.app_window()

    def app_words(self, name):
        win = self.app_window()
        if not win.get("pos") or not win.get("size"):
            raise RuntimeError("compositor did not expose the app geometry")
        rect = [*win["pos"], *win["size"]]
        return self.ocr(name, rect), rect

    def ocr(self, name, rect):
        rect = [int(round(v)) for v in rect]
        png = self.shoot(f"{name}-{time.monotonic_ns()}", rect)
        words = handskit.image_words(png, rect)
        self.write(f"crops/{png.stem}.words.json", words)
        return words

    def scroll(self, n):
        w = self.app_window()
        self.sh("input.sh", "scroll", round(w["pos"][0] + w["size"][0] * .72),
                round(w["pos"][1] + w["size"][1] * .65), n)
        time.sleep(.4)

    def find_label(self, label, name, scroll=True):
        for i in range(14 if scroll else 3):
            words, rect = self.app_words(f"{name}-{i}")
            hits = handskit.find_phrase(words, label)
            # Prefer content over the same wording in the sidebar.
            content = [h for h in hits if h[0] > rect[0] + 240]
            if content or hits:
                return content or hits, words, rect
            if scroll:
                if i == 0 and getattr(self, "current_view", None):
                    self.route(self.current_view)
                    self.scroll(-60)
                else:
                    self.scroll(4)
            else:
                time.sleep(.5)
        raise RuntimeError(f"the real app shows no {label!r}")

    def click_label(self, label, name=None, scroll=True):
        hits, _, _ = self.find_label(label, name or label.replace(" ", "-")[:40], scroll)
        point = self.click_box(hits[-1])
        time.sleep(.5)
        return point

    def row_button(self, row, button, name):
        hits, words, _ = self.find_label(row, name)
        candidates = handskit.find_phrase(words, button)
        box = handskit.nearest_in_row(hits[0], candidates)
        if box is None:
            raise RuntimeError(f"no {button!r} beside {row!r}")
        return self.click_box(box)

    def toggle(self, label, read, want):
        if read() == want:
            return
        _, words, rect = self.find_label(label, "switch")
        content = None
        for attempt in range(6):
            for polarity in ("original", "contrast", "contrast-block"):
                reading = [word for word in words if word["line"][0] == polarity]
                candidates = []
                for row in handskit.find_phrase(reading, label):
                    following = [hit for title in ROW_AFTER[label]
                                 for hit in handskit.find_phrase(reading, title)
                                 if hit[1] > row[1] + row[3]]
                    if following:
                        below = min(hit[1] for hit in following)
                        alignment = min(abs(row[0] - hit[0]) for hit in following if hit[1] == below)
                        candidates.append((row, below, alignment))
                if not candidates:
                    continue
                # Descriptions can repeat a label; row titles share a column.
                row, below, _ = min(candidates, key=lambda candidate: (candidate[2], candidate[0][1]))
                text = [word["box"] for word in reading
                        if row[1] - 2 <= word["box"][1] < below - 2
                        and row[0] - 2 <= word["box"][0] < rect[0] + rect[2] - 100]
                content = handskit.union(text)
                break
            if content is not None:
                break
            self.scroll(1)
            words, rect = self.app_words(f"switch-complete-row-{attempt}")
        if content is None:
            raise RuntimeError(f"the complete {label!r} row is not visible in one OCR reading")
        y = content[1] + content[3] / 2
        for dx in (44, 64, 88, 120, 160):
            self.click_box([rect[0] + rect[2] - dx - 3, y - 3, 6, 6])
            time.sleep(.7)
            if read() == want:
                self.check(f"actual app switch {label} reads back {want}", True,
                           {"offset": dx, "row_text": content, "centre_y": y})
                return
        raise RuntimeError(f"{label!r} did not read back {want!r}")

    def scratch_field(self, focus):
        hits, words, _ = self.find_label("Tour scratch note", "scratch-field", False)
        label = hits[0]
        below = [box[1] for box in handskit.find_phrase(words, "Practice guide")
                 if box[1] > label[1] + label[3]]
        if not below:
            raise RuntimeError("the tour scratch editor's visible text boundaries are incomplete")
        editor = [label[0], label[1] + label[3], label[2], min(below) - label[1] - label[3]]
        if focus:
            self.click_box(editor)
        return editor

    def reopen(self, view):
        self.sh("phase17.sh", "ui", "close")
        self.sh("phase17.sh", "ui", "open")
        self.wait_until(self.state, lambda s: any(w.get("app_id") == APP_ID for w in s["windows"]), 30)
        return self.route(view)

    def master(self, on):
        self.route("oni")
        self.toggle("Oni on the desktop", lambda: self.oni_settings().get("master"), on)
        if on:
            # The app may have started an ordinary surface. Replace only that
            # isolated lab process with the same binary plus its existing ledger hook.
            mark = self.ledger.mark()
            self.sh("ctl.sh", "surface", "restart")
            self.wait(mark, 15, ev="commit")
            self.settle(1.5)
            state = self.state()
            character = self.current()
            rect = labkit.global_rect(character, state)
            app = self.app_window()
            if rect[0] + rect[2] > app["pos"][0] + 240:
                output = self.output(character["output"], state)
                end = (output["x"] + 1, output["y"] + output["height"] - 1)
                windows = [[*window["pos"], *window["size"]] for window in state["windows"]
                           if window.get("pos") and window.get("size")]
                if any(x <= end[0] < x + width and y <= end[1] < y + height
                       for x, y, width, height in windows):
                    raise RuntimeError("no bare left desktop endpoint for the Settings character drag")
                mark = self.ledger.mark()
                self.sh("input.sh", "drag", *labkit.centre(rect), *end)
                self.wait(mark, 10, ev="drop")
                self.settle(1.5)
                parked = labkit.global_rect(self.current(), self.state())
                self.check("real pointer drag parks Oni outside Settings content",
                           parked[0] + parked[2] <= app["pos"][0] + 240,
                           {"before": rect, "endpoint": end, "after": parked})
        else:
            self.wait_until(lambda: labkit.lab_pids(self.env["LAB_ID"], "oni-surface"), lambda p: not p)

    def optional_off(self):
        oni = self.oni_settings()
        fields = [oni.get(k) for k in ("master", "inspect", "voice", "reflex", "sounds", "perchFollowsFocus")]
        for group, keys in {"guide": ("enabled", "keep", "hints"),
                            "reflexSettings": ("reminders", "routines", "notes", "decisions"),
                            "recall": ("enabled", "vault"), "rewrite": ("enabled", "voice"),
                            "assignment": ("enabled",), "feeds": ("guard",), "mcp": ("enabled",)}.items():
            fields += [(oni.get(group) or {}).get(k, False) for k in keys]
        vs = oni.get("voiceSettings") or {}
        fields += [vs.get(k, False) for k in ("speakReplies", "openMic")]
        fields += [(vs.get("wake") or {}).get("enabled", False),
                   (vs.get("dictation") or {}).get("enabled", False)]
        return not any(fields) and oni.get("hands", "off") == "off" and not (oni.get("mcp") or {}).get("tools")

    def census17(self):
        for tool in ("tesseract", "wtype", "grim", "systemd-run", "choom"):
            self.check(f"dependency {tool}", shutil.which(tool) is not None)
        import PIL.Image  # Missing Pillow is a failed step, not a skip.
        scope = labkit.scope_of(os.getpid())
        driver = {"scope": scope, "cpu_affinity": sorted(os.sched_getaffinity(0)),
                  "memory_max": labkit.systemctl_show(scope, "MemoryMax") if scope else "",
                  "memory_swap_max": labkit.systemctl_show(scope, "MemorySwapMax") if scope else "",
                  "oom_score_adj": Path("/proc/self/oom_score_adj").read_text().strip()}
        self.check("native driver and OCR have memory/no-swap/OOM limits",
                   driver["memory_max"].isdigit() and 0 < int(driver["memory_max"]) <= 2 * 1024 ** 3
                   and driver["memory_swap_max"] == "0" and driver["oom_score_adj"] == "1000", driver)
        bins = Path(self.env["HYAKKI_BIN"])
        for name in ("oni", "oni-surface", "hyakki", "hyakki-app", *(f"{n}.test" for n in TESTS)):
            p = bins / name
            self.check(f"frozen executable {name}", p.is_file() and os.access(p, os.X_OK))
            if p.is_file():
                self.initial_bins[name] = digest(p)
        self.write("binary-sha256.json", self.initial_bins)
        self.check("master and optional features start off", self.optional_off(), self.oni_settings())
        self.check("no resident surface on fresh launch", not labkit.lab_pids(self.env["LAB_ID"], "oni-surface"))
        isolation = []
        for pid in labkit.lab_pids(self.env["LAB_ID"]):
            if labkit.comm_of(pid) not in ("oni", "hyakki-app", "oni-surface"):
                continue
            try:
                raw = Path(f"/proc/{pid}/environ").read_bytes()
            except PermissionError:
                # oni deliberately disables process dumps. Its real scope and
                # private socket remain observable; never disable that protection.
                scoped = str(labkit.scope_of(pid) or "").startswith(f"oni-lab-{self.env['LAB_ID']}-oni-")
                socket = self.lab / "rt/hyakki/oni.sock"
                ok = labkit.comm_of(pid) == "oni" and scoped and socket.is_socket()
                isolation.append({"pid": pid, "comm": "oni", "pass": ok,
                                  "authority": "owned daemon scope and private socket; environ protected"})
                continue
            env = dict(v.decode(errors="replace").split("=", 1) for v in raw.split(b"\0") if b"=" in v)
            ok = all(env.get(k, "").startswith(str(self.lab) + "/") for k in
                     ("HOME", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "XDG_RUNTIME_DIR"))
            ok = ok and env.get("WAYLAND_DISPLAY") == self.env["LAB_DISPLAY"]
            isolation.append({"pid": pid, "comm": labkit.comm_of(pid), "pass": ok})
        self.check("app XDG/display and protected daemon scope isolated", bool(isolation) and all(p["pass"] for p in isolation), isolation)
        self.check("no account credential or hosted configuration seeded",
                   not any(p.name in ("vault.enc", "credentials.json") for p in (self.lab / "data").rglob("*")))
        return self.done()

    def onboarding(self):
        self.find_label(TOUR[0], "fresh-tour", False)
        self.click_label("Skip tour", scroll=False)
        state = self.settings_read()
        self.check("skip records completion", state.get("onboarding") == {"seen": True, "done": True}, state.get("onboarding"))
        self.check("skip enabled nothing", self.optional_off())
        self.reopen("rashin")
        words, _ = self.app_words("skip-reopen")
        self.check("fresh offer does not repeat on reopen", not handskit.find_phrase(words, TOUR[0]))
        self.click_label("Re-run the tour")
        before = self.settings_read()
        for i, title in enumerate(TOUR):
            self.find_label(title, f"tour-card-{i}", False)
            if title == "Try a guide safely":
                self.click_label("Mark practice step done", scroll=False)
                self.find_label("Type hello in the tour scratch note first.", "guide-empty-refused", False)
                self.scratch_field(True)
                self.sh("input.sh", "type", "hello")
                self.sh("input.sh", "key", "-k", "Tab")
                self.click_label("Mark practice step done", scroll=False)
                self.find_label("2. Clear the scratch note yourself", "guide-first-step", False)
                editor = self.scratch_field(False)
                words, _ = self.app_words("scratch-entered")
                self.check("hello visibly entered in the actual scratch editor",
                           any(editor[0] <= box[0] and box[0] + box[2] <= editor[0] + editor[2]
                               and editor[1] <= box[1] < editor[1] + editor[3]
                               for box in handskit.find_phrase(words, "hello")), editor)
                self.scratch_field(True)
                self.sh("input.sh", "key", "-M", "ctrl", "-k", "a", "-m", "ctrl", "-k", "BackSpace")
                self.click_label("Mark practice step done", scroll=False)
                self.find_label("Practice complete.", "guide-complete", False)
                self.check("scratch guide completed through real input", True)
            if title == "You decide what leaves":
                self.find_label("A hosted provider", "consent-destination", False)
                self.click_label("No, keep it here", scroll=False)
                self.find_label("Practice answered.", "consent-no", False)
                self.check("practice refusal enabled nothing", self.optional_off())
                self.click_label("Practice Allow once", scroll=False)
                self.find_label("Nothing was captured, sent or approved for later.", "consent-once", False)
                self.check("practice allow saved no permanent permission", self.optional_off())
            self.click_label("Start my first Ask" if i == len(TOUR) - 1 else "Continue", scroll=False)
        words, _ = self.app_words("first-task")
        self.check("first-task choice dismisses the real tour", not handskit.find_phrase(words, TOUR[-1]))
        self.check("first-task opens an unsent Ask",
                   bool(handskit.find_phrase(words, "Ask about this machine")) and
                   bool(handskit.find_phrase(words, "Ask anything about this machine")))
        after = self.settings_read()
        self.check("replayed tour completed", after.get("onboarding", {}).get("done"))
        self.check("tour preserved every other setting", {k: v for k, v in before.items() if k != "onboarding"} ==
                   {k: v for k, v in after.items() if k != "onboarding"})
        self.check("tour left all optional features off", self.optional_off())
        return self.done()

    def pages17(self):
        for route, labels in PAGES.items():
            w = self.route(route)
            self.check(f"{route}: normal window, not fullscreen", not w.get("fullscreen") and
                       900 <= w["size"][0] <= 1440 and 650 <= w["size"][1] <= 900, w["size"])
            for label in labels:
                self.find_label(label, f"route-{route}")
                self.check(f"{route}: real visible {label}", True)
        self.check("navigation activated no feature", self.optional_off())
        self.master(True)
        self.check("app switch actually mapped surface", bool(labkit.lab_pids(self.env["LAB_ID"], "oni-surface")) and self.current().get("ev") == "commit")
        self.route("oni-writing")
        self.toggle("Recall what", lambda: self.oni_settings().get("recall", {}).get("enabled"), True)
        self.route("oni-screen")
        self.reopen("oni-writing")
        self.find_label("Recall what", "recall-reopen")
        self.check("app-only setting survives route and app reopen", self.oni_settings().get("recall", {}).get("enabled"))
        # Actual local action uses the newly enabled setting; no model needed.
        export = self.lab / "home/conversations.json"
        export.write_text(json.dumps([{"uuid": "phase17-local", "name": "Lantern local fixture", "created_at": "2026-01-01T09:00:00Z",
                                      "chat_messages": [{"sender": "human", "text": "What colour is amber?"},
                                                        {"sender": "assistant", "text": "A warm yellow orange."}]}]))
        export.chmod(0o600)
        result = self.call("oni.recall.import", {"paths": [str(export)]})
        self.write("fixtures/recall-import.json", result)
        self.check("enabled Recall imports actual local export", result.get("chats") == 1 and not result.get("refused"), result)
        return self.done()

    def styles17(self):
        from PIL import Image, ImageChops
        shell = self.lab / "cfg/ryoku/shell.json"
        colors = self.lab / "cache/ryoku/colors.json"
        shell.parent.mkdir(parents=True, exist_ok=True)
        colors.parent.mkdir(parents=True, exist_ok=True)
        palette = {"surface": "#102840", "on_surface": "#daf0ff", "primary": "#64aaff", "on_primary": "#001530"}
        colors.write_text(json.dumps(palette))
        before = self.settings_read()
        shots = {}
        for source in ("wallpaper", "ryoku"):
            for style in ("lithic", "material", "adwaita", "fluent"):
                chosen = {"theme": {"design": style, "colours": source, "tint": "medium", "motion": {"reduce": True}}}
                shell.write_text(json.dumps(chosen))
                time.sleep(2)
                self.route("appearance")
                self.find_label(style.title(), f"style-{style}-{source}")
                self.find_label("Wallpaper" if source == "wallpaper" else "Ryoku", f"palette-{style}-{source}")
                self.check(f"{style}/{source}: desktop choice not rewritten", json.loads(shell.read_text()) == chosen)
                self.check(f"{style}/{source}: wallpaper palette preserved", json.loads(colors.read_text()) == palette)
                w = self.app_window()
                shots[style, source] = self.shoot(f"look-{style}-{source}", [*w["pos"], *w["size"]])
        self.check("style changes preserve app settings", before == self.settings_read())
        for style in ("lithic", "material", "adwaita", "fluent"):
            with Image.open(shots[style, "wallpaper"]) as a, Image.open(shots[style, "ryoku"]) as b:
                # Check a large content region, not only a changed label.
                box = (a.width // 3, a.height // 3, a.width - 20, a.height - 20)
                diff = ImageChops.difference(a.convert("RGB").crop(box), b.convert("RGB").crop(box))
                changed = sum(1 for px in diff.getdata() if max(px) > 8)
                self.check(f"{style}: palette replaced in rendered content", changed > 1000, changed)
        self.reopen("appearance")
        self.find_label("Fluent", "style-reopen")
        self.find_label("Ryoku", "palette-reopen")
        return self.done()

    def mcp17(self):
        off = MCP(self)
        self.check("real MCP initialize refused by default", denied(off.initialize()))
        off.close()
        self.master(True)
        self.route("oni-privacy")
        self.toggle("Serve Oni's tools to other programs", lambda: self.oni_settings().get("mcp", {}).get("enabled"), True)
        bridge = MCP(self)
        self.bridge = bridge
        self.check("stdio initialize negotiates without approval", not denied(bridge.initialize()))
        pending = bridge.rpc("tools/list")
        self.check("unapproved program has no offered tools", not pending.get("result", {}).get("tools"), pending)
        self.check("unapproved program tools/call refused", denied(bridge.tool("focus_status")))
        self.route("oni-privacy")
        # Entering the section reads connected clients from the daemon.
        self.row_button("Connected programs", "Refresh", "refresh-mcp-programs")
        self.row_button(CLIENT, "Approve", "approve-local-program")
        self.wait_until(self.oni_settings, lambda o: CLIENT in o.get("mcp", {}).get("clients", []))
        empty = bridge.rpc("tools/list")
        self.check("approved client still has no tool permission", not denied(empty) and empty.get("result", {}).get("tools") == [], empty)
        for tool, label in TOOLS.items():
            self.route("oni-privacy")
            self.toggle(label, lambda t=tool: bool(self.oni_settings().get("mcp", {}).get("tools", {}).get(t)), True)
        listed = bridge.rpc("tools/list")
        offered = listed.get("result", {}).get("tools", [])
        self.check("real tools/list exactly app-enabled seven tools", not denied(listed) and
                   len(offered) == len(TOOLS) and {t["name"] for t in offered} == set(TOOLS), listed)
        self.check("unknown tool refused", denied(bridge.tool("unknown_phase17_tool")))
        self.check("unknown JSON-RPC method refused", denied(bridge.rpc("phase17/unsupported")))
        self.check("focus_status actually executes", not denied(bridge.tool("focus_status")))
        self.check("screen ask without inspect/perch refused", denied(bridge.tool("screen_ask", {"question": "What is shown?"})))
        self.route("oni-commands")
        self.toggle("Quick commands", lambda: self.oni_settings().get("reflex"), True)
        self.toggle("Reminders and timers", lambda: self.oni_settings().get("reflexSettings", {}).get("reminders"), True)
        due = (datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(hours=1)).isoformat(timespec="seconds")
        added = bridge.tool("reminders_add", {"due": due, "text": "Check the local amber lantern"})
        self.check("MCP creates real pending reminder", not denied(added), added)
        reminders = self.call("oni.reminders", {"action": "list"})
        self.check("real reminders store readback", len(reminders.get("reminders", [])) == 1, reminders)
        self.check("MCP lists actual reminder", "amber lantern" in json.dumps(bridge.tool("reminders_list")))
        self.route("memory")
        self.toggle("Use memory", lambda: self.settings_read().get("memory", {}).get("use"), True)
        suggestion = bridge.tool("notes_add", {"text": NOTE})
        self.check("MCP suggestion executes", not denied(suggestion), suggestion)
        suggested = self.call("memory.list", {"scope": "you", "status": "suggested", "query": "amber lanterns"})
        active = self.call("memory.list", {"scope": "you", "status": "active", "query": "amber lanterns"})
        self.check("agent note is suggested, never a fact", suggested.get("total") == 1 and active.get("total") == 0, {"suggested": suggested, "active": active})
        searched = bridge.tool("notes_search", {"query": "amber lanterns"})
        self.check("MCP search executes without treating suggestion as a fact",
                   not denied(searched) and NOTE not in json.dumps(searched), searched)
        self.route("inbox")
        self.find_label("amber lanterns", "inbox-agent-suggestion")
        self.check("suggestion visible in actual Inbox", True)
        self.check("private card-shaped note refused", denied(bridge.tool("notes_add", {"text": "My card number is 4111111111111111"})))
        self.route("oni")
        self.settings(hidden=False)
        self.wake()
        mark = self.ledger.mark()
        notice = bridge.tool("notify", {"text": NOTICE})
        pill = self.ledger.wait(lambda l: l.get("part") == "pill" and l.get("ev") == "commit", mark, 8)
        self.check("notify executed and pill committed", not denied(notice) and pill is not None, notice)
        output = self.output(self.env["LAB_OUTPUT"])
        rect = (pill or {}).get("rect", [])
        in_output = (len(rect) == 4 and pill.get("output") == self.env["LAB_OUTPUT"]
                     and rect[0] >= 0 and rect[1] >= 0 and rect[2] > 0 and rect[3] > 0
                     and rect[0] + rect[2] <= output["width"] and rect[1] + rect[3] <= output["height"])
        self.check("notice geometry stays inside actual lab output", in_output, {"pill": pill, "output": output})
        words = self.ocr("mcp-visible-notice", [output["x"] + rect[0], output["y"] + rect[1], rect[2], rect[3]]) if in_output else []
        self.check("notify words visible on real pill", notice_phrase(words, NOTICE), words)
        self.check("notify identifies sending program", notice_phrase(words, CLIENT + ":"))
        self.route("oni-privacy")
        self.toggle(TOOLS["notify"], lambda: bool(self.oni_settings().get("mcp", {}).get("tools", {}).get("notify")), False)
        self.check("disabled tool refused immediately on existing stdio", denied(bridge.tool("notify", {"text": "This must be refused"})))
        self.toggle("Serve Oni's tools to other programs", lambda: self.oni_settings().get("mcp", {}).get("enabled"), False)
        revoked = bridge.rpc("tools/list")
        self.check("master MCP off removes tools from connected client", denied(revoked) or
                   revoked.get("result", {}).get("tools") == [], revoked)
        self.check("master MCP off refuses an actual tool call", denied(bridge.tool("focus_status")))
        bridge.close()
        # An approved open pipe must not hold oni resident. The child makes no
        # RPC while /proc observes the daemon's ordinary configured idle exit.
        self.toggle("Serve Oni's tools to other programs", lambda: self.oni_settings().get("mcp", {}).get("enabled"), True)
        idle_bridge = MCP(self)
        self.check("idle bridge initialized", not denied(idle_bridge.initialize()))
        self.master(False)
        self.sh("phase17.sh", "ui", "close")
        started = time.monotonic()
        self.wait_until(lambda: labkit.lab_pids(self.env["LAB_ID"], "oni"), lambda p: not p, 360)
        self.check("open MCP pipe does not prevent natural daemon idle exit",
                   idle_bridge.proc.poll() is None, {"idle_seconds": time.monotonic() - started})
        idle_bridge.close()
        self.sh("phase17.sh", "ui", "open")
        self.wait_until(self.state, lambda s: any(w.get("app_id") == APP_ID for w in s["windows"]), 30)
        self.route("oni-privacy")
        self.toggle("Serve Oni's tools to other programs", lambda: self.oni_settings().get("mcp", {}).get("enabled"), False)
        return self.done()

    def activity17(self):
        before = self.call("oni.activity", {})
        self.write("activity-before.json", before)
        self.check("activity populated by actual reminder and recall actions", before.get("counts", {}).get("reminders") == 1 and
                   before.get("counts", {}).get("recall_imports") == 1, before)
        self.check("aggregate API contains codes/counts, not local content", not any(s in json.dumps(before) for s in
                   (NOTE, "Check the local amber lantern", "Lantern local fixture", "warm yellow orange")))
        self.master(False)
        self.route("oni-privacy")
        self.row_button("Reminders and timers", "Clear", "select-reminders")
        self.find_label("Clear records", "clear-confirmation", False)
        unchanged = self.call("oni.activity", {})
        self.check("opening clear confirmation deletes nothing", unchanged == before)
        self.click_label("Clear records", scroll=False)
        after = self.wait_until(lambda: self.call("oni.activity", {}), lambda v: v.get("counts", {}).get("reminders") == 0)
        self.write("activity-after.json", after)
        self.check("only selected class cleared by app confirmation", after.get("counts", {}).get("recall_imports") == 1 and
                   all(after["counts"].get(k) == v for k, v in before["counts"].items() if k != "reminders"), after)
        self.reopen("oni-privacy")
        self.click_label("Refresh activity")
        self.check("selective clear survives app reopen", self.call("oni.activity", {}).get("counts", {}).get("reminders") == 0)
        active = self.call("memory.list", {"scope": "you", "status": "suggested", "query": "amber lanterns"})
        self.check("activity clear does not erase memory suggestions", active.get("total") == 1)
        return self.done()

    def parts17(self):
        fixture = self.lab / "cache/hyakki/oni/voice/phase17-owned.wav"
        fixture.parent.mkdir(parents=True, exist_ok=True)
        fixture.write_bytes(b"RIFF" + b"\0" * 4092)
        fixture.chmod(0o600)
        sentinel = self.lab / "home/do-not-remove.txt"
        sentinel.write_text("outside the component, still lab-owned\n")
        sentinel.chmod(0o600)
        expected = digest(sentinel)
        before = self.call("components.list", {})
        row = next((c for c in before.get("components", []) if c.get("id") == "cache:voice"), {})
        self.check("real owned fixture inventoried with size", row.get("bytes", 0) >= 4096 and
                   row.get("removable") and row.get("state") == "present" and row.get("path") == str(fixture.parent), row)
        self.route("parts")
        self.click_label("Measure again")
        self.row_button("Spoken sentence cache", "Remove", "remove-owned-cache")
        self.find_label("Remove it", "remove-confirmation", False)
        self.check("part exists until explicit confirmation", fixture.exists())
        self.click_label("Remove it", scroll=False)
        self.wait_until(fixture.exists, lambda exists: not exists)
        after = self.call("components.list", {})
        self.write("parts-before-after.json", {"before": before, "after": after})
        self.check("actual owned cache removed and inventory absent", any(c.get("id") == "cache:voice" and c.get("state") == "absent" for c in after.get("components", [])))
        self.check("unrelated owned sentinel untouched", sentinel.exists() and digest(sentinel) == expected)
        self.check("optional programs are never app-removable", all(not c.get("removable") for c in after.get("components", []) if c.get("kind") == "program"))
        return self.done()

    def courtesy17(self):
        self.master(True)
        self.route("oni")
        self.toggle("Step aside for full screen", lambda: self.oni_settings().get("courtesy"), True)
        out = self.app_output(self.state())
        self.call("oni.place", {"output": out, "fx": .5, "fy": .5, "corner": ""})
        self.settle()
        self.wake()
        mark = self.ledger.mark()
        self.sh("ctl.sh", "fullscreen", "on")
        hide = self.ledger.wait(lambda l: l.get("ev") == "hide" and l.get("reason") == "courtesy", mark, 10)
        self.check("actual fullscreen courtesy hide", hide is not None, hide)
        o = self.output(out)
        self.shoot("fullscreen-courtesy", [o["x"], o["y"], o["width"], o["height"]])
        measured = self.measure("covered", seconds=10, name="phase17-courtesy")
        self.check("courtesy has no idle drawing", measured.get("pass"), measured)
        mark = self.ledger.mark()
        self.sh("ctl.sh", "fullscreen", "off")
        show = self.ledger.wait(lambda l: l.get("ev") in ("show", "commit"), mark, 10)
        self.check("returns only after fullscreen ends", show is not None, show)
        self.shoot("after-fullscreen", [o["x"], o["y"], o["width"], o["height"]])
        return self.done()

    def settlement17(self):
        self.sh("ctl.sh", "fullscreen", "off", check=False)
        # clear dependent switches before their master disables them
        oni = self.oni_settings()
        if not oni.get("master") and (oni.get("reflex") or
                                      oni.get("reflexSettings", {}).get("reminders")):
            self.master(True)
        self.route("oni-commands")
        self.toggle("Reminders and timers", lambda: self.oni_settings().get("reflexSettings", {}).get("reminders"), False)
        self.toggle("Quick commands", lambda: self.oni_settings().get("reflex"), False)
        self.master(False)
        self.route("oni-writing")
        self.toggle("Recall what I kept", lambda: self.oni_settings().get("recall", {}).get("enabled"), False)
        self.route("oni-privacy")
        self.toggle("Serve Oni's tools to other programs", lambda: self.oni_settings().get("mcp", {}).get("enabled"), False)
        self.route("memory")
        self.toggle("Use memory", lambda: self.settings_read().get("memory", {}).get("use"), False)
        self.check("all enabled runtime features returned off", not self.oni_settings().get("master") and
                   not self.oni_settings().get("reflex") and not self.oni_settings().get("mcp", {}).get("enabled"))
        self.check("off leaves no renderer", not labkit.lab_pids(self.env["LAB_ID"], "oni-surface"))
        for m in self.mcps:
            m.close()
        self.sh("phase17.sh", "ui", "close")
        # Read only /proc while waiting for the real configured idle exit.
        # No explicit daemon stop is allowed to masquerade as zero residency.
        idle_started = time.monotonic()
        self.wait_until(lambda: labkit.lab_pids(self.env["LAB_ID"], "oni"), lambda p: not p, 360)
        self.check("daemon exits naturally after last UI/stdio client", True,
                   {"wait_seconds": time.monotonic() - idle_started})
        # Do not call status/watch/CLI during this window: they self-start oni.
        watch_started = time.monotonic()
        observations = []
        while True:
            pids = {name: labkit.lab_pids(self.env["LAB_ID"], name) for name in
                    ("oni", "hyakki-app", "oni-surface", "whisper-server", "espeak-ng", "pw-cat", "chromium")}
            observations.append({"elapsed": time.monotonic() - watch_started, "pids": pids})
            if observations[-1]["elapsed"] >= 300:
                break
            time.sleep(min(1, 300 - observations[-1]["elapsed"]))
        self.write("off-process-observations.json", observations)
        self.check("closed/off processes do not reactivate for five minutes",
                   not any(p for sample in observations for p in sample["pids"].values()),
                   {"seconds": observations[-1]["elapsed"], "samples": len(observations)})
        self.check("no user service enabled by UI flow", not list((self.lab / "cfg/systemd").rglob("*.wants/*")))
        children = self.children.procs()
        self.write("daemon-children.json", {"sampling_passes": self.children.passes,
                                           "children": children, "unread": self.children.unread})
        forbidden = {"wtype", "ydotool", "dotool", "pw-record", "pw-cat", "parec", "paplay",
                     "aplay", "espeak", "espeak-ng", "whisper-server", "chromium", "weston"}
        self.check("no optional input, audio, browser or separate-desktop child activated",
                   self.children.passes > 0 and not any(Path((r.get("argv") or [""])[0]).name in forbidden
                                                       for r in children), children)
        self.check("no child observation silently skipped", not self.children.unread, self.children.unread)
        final_sources = snapshot_sources()
        changed = [path for path in sorted(self.initial_sources.keys() | final_sources.keys())
                   if self.initial_sources.get(path) != final_sources.get(path)]
        self.check("frozen source unchanged during runtime", not changed, changed)
        bins = Path(self.env["HYAKKI_BIN"])
        self.check("frozen binaries unchanged during runtime", all(digest(bins / n) == h for n, h in self.initial_bins.items()))
        return self.done()

    def tests17(self):
        for name, (package, pattern) in TESTS.items():
            binary = Path(self.env["HYAKKI_BIN"]) / f"{name}.test"
            if not binary.is_file() or not os.access(binary, os.X_OK):
                self.check(f"{name} compiled contract tests", False, f"missing {binary}; go test -c ./{package}")
                continue
            private = self.lab / "tests" / name
            for directory in ("home", "tmp", "cfg", "state", "data", "cache", "rt"):
                (private / directory).mkdir(parents=True, mode=0o700, exist_ok=True)
            command = ["systemd-run", "--user", "--scope", "--quiet", "--collect",
                       f"--unit=oni-lab-{self.env['LAB_ID']}-p17-{name}", "-p", "MemoryMax=1G",
                       "-p", "MemorySwapMax=0", "--", "choom", "-n", "1000", "--", "env", "-i",
                       "PATH=/usr/bin:/bin", f"HOME={private}/home", f"TMPDIR={private}/tmp",
                       f"XDG_CONFIG_HOME={private}/cfg", f"XDG_STATE_HOME={private}/state",
                       f"XDG_DATA_HOME={private}/data", f"XDG_CACHE_HOME={private}/cache",
                       f"XDG_RUNTIME_DIR={private}/rt",
                       str(binary), "-test.v", "-test.count=1", "-test.run", pattern]
            r = subprocess.run(command, cwd=ROOT / "ryoku/hyakki" / package, text=True, capture_output=True, timeout=600)
            (self.out / "tests" / f"{name}.txt").write_text(r.stdout + r.stderr)
            self.check(f"{name}: real compiled tests executed, no empty match or skip", r.returncode == 0 and
                       "--- PASS:" in r.stdout and "--- SKIP:" not in r.stdout, {"command": command, "exit": r.returncode})
            required = {
                "shell": ("TestOniSettingsRoutesNavigateToTheirOwnControls",
                          "TestOniPhase16SettingsWriteOnlyTheirOwnSwitches",
                          "TestOniSettingsHaveNoDuplicateEditableControls",
                          "TestOnboardingFreshLaunchSkipAndRerunChangeOnlyTourState"),
                "settings": ("TestSettingsSectionsKeepExistingRoutesAndSearch",
                             "TestOniChangesCountIndependentSettings"),
            }
            for test in required.get(name, ()):
                self.check(f"consumer acceptance {test} actually passed",
                           f"--- PASS: {test} " in r.stdout)
        return self.done()

    def run(self):
        stages = (("fresh isolated census", self.census17), ("skip and replay real onboarding", self.onboarding),
                  ("normal-sized routes and persistent actual effects", self.pages17),
                  ("live styles and independent palette", self.styles17),
                  ("real approved stdio MCP and visible notifications", self.mcp17),
                  ("real activity and app-confirmed selective clear", self.activity17),
                  ("owned optional part inventory and confirmed removal", self.parts17),
                  ("fullscreen courtesy", self.courtesy17), ("compiled consumer contracts", self.tests17),
                  ("off residency and frozen-input fence", self.settlement17))
        try:
            for n, (name, fn) in enumerate(stages):
                self.step(n, name, fn)
        finally:
            self.children.done.set()
            self.children.join(timeout=5)
            for m in self.mcps:
                m.close()
            self.results["finished_at"] = labkit.now()
            self.save()
            self.log.close()
        return 0 if self.results["pass"] else 1


def finalize(out, status):
    out = Path(out)
    path = out / "phase17.json"
    result = json.loads(path.read_text()) if path.exists() else {"pass": False, "steps": [], "error": "lab startup or helper failed before acceptance"}
    down = out / "down.txt"
    result["teardown"] = {"exit_status": int(status), "log": "down.txt", "sha256": digest(down) if down.exists() else None}
    if int(status) != 0 or not result.get("pass"):
        result["pass"] = False
    path.write_text(json.dumps(result, indent=2) + "\n")
    manifest = {str(p.relative_to(out)): digest(p) for p in sorted(out.rglob("*")) if p.is_file() and not p.is_symlink() and p.name != "evidence-sha256.json"}
    (out / "evidence-sha256.json").write_text(json.dumps(manifest, indent=2) + "\n")
    return 0 if result["pass"] else 1


if __name__ == "__main__":
    if len(sys.argv) == 4 and sys.argv[1] == "phase17":
        raise SystemExit(Phase17(sys.argv[2], sys.argv[3]).run())
    if len(sys.argv) == 4 and sys.argv[1] == "finalize":
        raise SystemExit(finalize(sys.argv[2], sys.argv[3]))
    raise SystemExit("usage: settingskit.py phase17 LAB OUT | finalize OUT STATUS")
