#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Phase 13 helpers for the Oni lab: the quick commands' speech fixtures, the
pure checks the gate rests on, and the Phase 13 gate itself (quick commands,
reminders and timers, routines, notes, typed decisions). It builds on
labkit.py, voicekit.py and guidekit.py without changing them and starts lab
processes only through the lab's shell scripts (phase13.sh
daemon|fixtures|app|wakestub|scope|worker among them).

reflexkit.py fixtures DIR       the Phase 13 speech WAVs (espeak-ng -w, never a
                                device) and fixtures.json into DIR, under
                                /dev/shm
reflexkit.py selftest           the pure helpers against synthetic data; no
                                lab, no process
reflexkit.py phase13 LAB OUT    the gate (phase13.sh runs it)

The contract this gate relies on (CLI verbs, frames, ledger fields, env hooks,
fixture endpoints and allowlists) is the header of phase13.sh and the table
in the lab README's Phase 13 section."""
import datetime
import json
import os
import re
import signal
import stat
import subprocess
import sys
import threading
import time
import urllib.parse
from pathlib import Path

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import busfix  # noqa: E402
import guidekit  # noqa: E402
import labkit  # noqa: E402
import voicekit  # noqa: E402
from labkit import comm_of, lab_pids, now, scope_of  # noqa: E402
from voicekit import (LEAD_S, TAIL_S, WAKE_PHRASE, FrameWatch, Phase11, Rec, read_wav16,  # noqa: E402
                      write_wav16)

ROOT = HERE.parents[4]
APP = ROOT / "ryoku/hyakki"

# --- what the gate drives ----------------------------------------------------

# launchfix.py's catalogue
NOTES_APP = "dev.ryoku.OniNotes"
BROWSER_APP = "dev.ryoku.OniBrowser"
NOTES_TITLE = "Lindqvist notes"
# decidefix.py's model name, which up.sh --decisions seeds as localModel
DECISION_MODEL = "oni-fixture-decisions"
REFLEX_ROWS = ("reflex", "reflex.routines", "reflex.reminders", "reflex.notes", "reflex.decisions")
SWITCHES = ("routines", "reminders", "notes", "decisions")
# Complete compiled contract, including Phase 16. Phase 17's activity,
# privacy and removable-parts APIs are management methods, not feature IDs.
CAPABILITY_ROWS = {
    "vision.inspect": ("read-only", "disclosed", "disclosure"),
    "voice": ("read-only", "disclosed", "user-initiated"),
    "voice.dictation": ("reversible", "disclosed", "user-initiated"),
    "reflex": ("reversible", "disclosed", "announced"),
    "feeds.guard": ("reversible", "local", "per-action"),
    "reflex.routines": ("reversible", "local", "per-action"),
    "reflex.reminders": ("reversible", "local", "announced"),
    "reflex.notes": ("reversible", "local", "announced"),
    "reflex.decisions": ("reversible", "disclosed", "disclosure"),
    "guide": ("read-only", "disclosed", "disclosure"),
    "guide.keep": ("read-only", "local", "per-action"),
    "guide.hints": ("read-only", "local", "announced"),
    "memory.forget": ("reversible", "local", "per-action"),
    "recall": ("read-only", "disclosed", "disclosure"),
    "rewrite": ("reversible", "disclosed", "disclosure"),
    "assignment": ("read-only", "disclosed", "disclosure"),
    "hands.browser": ("guarded", "disclosed", "firm"),
    "hands.app": ("guarded", "disclosed", "firm"),
    "hands.desktop": ("guarded", "disclosed", "firm"),
}
# The character addressed on open mic: the shipped one's name, read from its
# profile so the fixture says what the router strips.
CHARACTER_NAME = json.loads((APP / "internal/oni/characters/oni/profile.json").read_text())["name"]
STORM_REPEATS = 43

# name: (text espeak-ng says, words per minute)
REFLEX_FIXTURES = {
    "open_notes": ("Open Lindqvist notes.", 150),
    "open_again": ("Open notes.", 170),
    "open_named": (f"Hey {CHARACTER_NAME}, please open notes.", 140),
    "new_tab": ("Open a new tab.", 150),
    "next_tab": ("Next tab.", 150),
    "close_tab": ("Close the tab.", 150),
    "snap_left": ("Snap the window to the left.", 150),
    "snap_right": ("Snap the window to the right.", 150),
    "maximize": ("Maximize the window.", 150),
    "minimize": ("Minimize the window.", 150),
    # Saved default-ASR PCM already recognizes "softer" and executes down.
    # Avoid the ambiguous sound/salad synthesis without altering the parser,
    # decoder, thresholds or the mandatory observed volume change.
    "vol_up": ("Raise the sound.", 150),
    "vol_down": ("Make it softer.", 140),
    "mute": ("Mute the sound.", 150),
    "unmute": ("Unmute the sound.", 150),
    "pause": ("Pause the music.", 150),
    "timer5": ("Set a timer for five seconds.", 160),
    "timer2m": ("Set a timer for two minutes.", 160),
    "cancel_timer": ("Cancel the timer.", 150),
    "remind10": ("Set a reminder in ten seconds to stretch.", 150),
    "remind_bare": ("Remind me.", 150),
    "remind8d": ("Remind me in eight days to call the bank.", 160),
    "remind20": ("Remind me in twenty minutes to drink water.", 160),
    "remind30": ("Remind me in thirty minutes to stand up.", 160),
    "remind40": ("Remind me in forty minutes to water the plants.", 160),
    "list": ("What are my reminders?", 150),
    # under voice.press.maxHoldS: 86 words at 300 wpm is about 17 s
    "storm": (" ".join(["open notes,"] * STORM_REPEATS), 300),
    "ten": ("Softer, louder, mute the audio, unmute the audio, pause music, play music, open notes, new tab, next tab, close the tab.", 170),
    "twice": ("Open Lindqvist notes and open Lindqvist notes.", 160),
    "open_only": ("Open.", 150),
    "notes_only": ("Notes.", 170),
    "careful": ("I insert a table.", 150),
    "salt": ("Search for salt and pepper.", 150),
    "send": ("Send the message.", 150),
    "buy": ("Buy this.", 150),
    "delete": ("Delete the file.", 150),
    "submit": ("Submit the form.", 150),
    "run_routine": ("Run my routine.", 150),
    "yes": ("Yes.", 150),
    "note": ("Remember that my exam is on Friday.", 160),
    "note_secret": ("Remember that my password is swordfish.", 160),
    # "boot up" is in decidefix.py's table and in none of the rules' verbs
    "decide": ("Boot up Lindqvist notes.", 150),
    "move_tl": ("Move to the top left.", 150),
    "bigger": ("Make yourself bigger.", 150),
    "home": ("Go back to your corner.", 150),
    "sounds_off": ("Turn off your sounds.", 150),
    "sounds_off_named": (f"Hey {CHARACTER_NAME}, please turn off your sounds.", 140),
    "on_openmic": ("Turn on open mic.", 150),
    "on_wake": ("Turn on the wake word.", 150),
    "on_hands": ("Turn on hands.", 150),
}
# what the forbidden step says, and words that may live nowhere on disk
FORBIDDEN = ("send", "buy", "delete", "submit")
SECRET = "swordfish"
# the refused note never becomes an Ask, so its words may live in no lab file
# (an Ask's words are the user's own conversation history)
TRANSCRIPT_MARKS = (SECRET,)
# the test binaries steps 9 and 15 run: (binary, path under app, -test.run)
TEST_PKGS = (
    ("feature", "internal/oni/feature", "(?i)(boundary|apart|import|insert|reflex|routine|remind|decision)"),
    ("reflex", "internal/oni/reflex", "."),
    ("remind", "internal/oni/remind", "."),
    ("routine", "internal/oni/routine", "."),
    ("act", "internal/reflex/act", "."),
)
REGISTRY_RUN = "(?i)(registry|forbid|invariant)"
# no user-facing reflex string may name another product or a model family
THIRD_PARTY = ("Nūs", "Nus", "Knot", "Nixi", "Coucou", "Jev", "TypeSafe", "Ollaya", "OpenAI", "ChatGPT", "GPT",
               "Claude", "Anthropic", "Gemini", "Llama", "Qwen", "Mistral", "DeepSeek", "Phi")
COPY_SOURCES = ("internal/oni/reflex", "internal/oni/remind", "internal/oni/routine", "internal/reflex",
                "internal/daemon/oni_reflex.go", "internal/daemon/oni_remind.go", "internal/daemon/oni_routine.go")
# the reflex executors' exact commands (internal/reflex/{audio,wm}): pactl on
# the default sink only, niri's own maximize
PACTL_ARGV = (
    ("get-sink-volume", "@DEFAULT_SINK@"),
    ("set-sink-volume", "@DEFAULT_SINK@", "+5%"),
    ("set-sink-volume", "@DEFAULT_SINK@", "-5%"),
    ("set-sink-volume", "@DEFAULT_SINK@", "100%"),
    ("set-sink-mute", "@DEFAULT_SINK@", "1"),
    ("set-sink-mute", "@DEFAULT_SINK@", "0"),
    ("set-sink-mute", "@DEFAULT_SINK@", "toggle"),
)
SCOPE_PREFIX = ("systemd-run", "--user", "--scope", "--collect", "--")
WAKE_UNITS = ("oni-routines.timer", "oni-routines")
# the hyakki.json keys the gate may rewrite while the daemon is stopped
CONFIG_KEYS = {"providers": ("decisions",), "memory": None, "enabled": None}
INPUT_PROGRAMS = voicekit.INPUT_PROGRAMS


# --- pure helpers (selftest covers each) -------------------------------------

def go_consts(src):
    """name -> text for every `name = "text"` and `name = i18n.T("text")`
    in Go source: the lines the gate expects, read from the code."""
    out = {}
    for m in re.finditer(r'\b(\w+)\s*:?=\s*(?:i18n\.T\()?"((?:[^"\\]|\\.)*)"', src):
        out.setdefault(m.group(1), m.group(2).replace('\\"', '"'))
    return out


def go_intents(src):
    """The registry's intent ids: every `Name Intent = "id"`."""
    return [m.group(1) for m in re.finditer(r'\b\w+\s+Intent\s*=\s*"([a-z_]+)"', src)]


def user_strings(src):
    """Every string Go source shows or says: i18n.T/Tf arguments and the
    line constants (`line... = "..."`, `Line... = "..."`)."""
    out = [m.group(1) for m in re.finditer(r'i18n\.Tf?\("((?:[^"\\]|\\.)*)"', src)]
    out += [m.group(1) for m in re.finditer(r'\b[lL]ine\w*\s*=\s*"((?:[^"\\]|\\.)*)"', src)]
    return out


def third_party_hits(strings, names=THIRD_PARTY):
    hits = []
    for s in strings:
        for n in names:
            if re.search(r"(?<![\w])" + re.escape(n) + r"(?![\w])", s, re.I):
                hits.append({"string": s, "name": n})
    return hits


def stub_lines(text, prefix="refused"):
    """The argv lists a refusal (or wake) stub logged: "PREFIX: NAME ARGS"."""
    out = []
    for line in (text or "").splitlines():
        head, sep, rest = line.partition(": ")
        if sep and head == prefix and rest.strip():
            out.append(rest.split(" "))
    return out


def wake_request(argv):
    """'enable' or 'disable' for the wake unit's own request through either
    bridge, else None."""
    a = list(argv)
    if a and os.path.basename(a[0]) == "ryoku-host" and len(a) == 6 and a[1:3] == ["svc", "--user"] \
            and a[3] in ("enable", "disable") and a[4] == "--now" and a[5] in WAKE_UNITS:
        return a[3]
    if a and os.path.basename(a[0]) == "systemctl" and len(a) == 5 and a[1] == "--user" \
            and a[2] in ("enable", "disable") and a[3] == "--now" and a[4] in WAKE_UNITS:
        return a[2]
    return None


def launch_argv(here, out):
    """The two catalogue entries' commands, field codes stripped, as the
    launcher runs them (launchfix.py's Exec lines)."""
    return {
        "notes": ["python3", str(Path(here) / "textwin.py"), NOTES_APP, NOTES_TITLE, str(Path(out) / "notes.txt")],
        "browser": ["python3", str(Path(here) / "tabwin.py"), "--out", str(out)],
    }


def allowed_url(u):
    if u == "about:blank":
        return True
    p = urllib.parse.urlsplit(u)
    return (p.scheme == "https" and bool(p.netloc) and "@" not in p.netloc and not u.startswith("-")
            and not any(c.isspace() or ord(c) < 32 for c in u))


def fixture_launch(argv, here, out):
    """(kind, scoped) for a catalogue start: kind 'notes' or 'browser', scoped
    whether it rode inside the systemd-run wrapper; None for anything else."""
    a = list(argv or [])
    scoped = tuple(a[:len(SCOPE_PREFIX)]) == SCOPE_PREFIX or (
        a and os.path.basename(a[0]) == "systemd-run" and tuple(a[1:len(SCOPE_PREFIX)]) == SCOPE_PREFIX[1:])
    if scoped:
        a = a[len(SCOPE_PREFIX):]
    want = launch_argv(here, out)
    if a == want["notes"]:
        return "notes", bool(scoped)
    if len(a) == len(want["browser"]) + 1 and a[:-1] == want["browser"] and allowed_url(a[-1]):
        return "browser", bool(scoped)
    return None


def child_allowed13(rec, here, out):
    """A program the daemon ran this run: the Phase 11/12 allowlist
    (guidekit.child_allowed), the reflex executors' exact commands, a
    catalogue start (bare or in the systemd-run wrapper), the refusal stubs'
    wake and seam reads. Never an input program."""
    argv = rec.get("argv") or []
    if not argv:
        return False
    prog = os.path.basename(argv[0])
    if prog in INPUT_PROGRAMS or rec.get("comm") in INPUT_PROGRAMS:
        return False
    if guidekit.child_allowed(rec):
        return True
    if prog == "pactl":
        return tuple(argv[1:]) in PACTL_ARGV
    if prog == "niri":
        return argv[1:] == ["msg", "action", "maximize-column"]
    if wake_request(argv):
        return True
    return fixture_launch(argv, here, out) is not None


def sink_level(graph, name):
    """(volume percent of the first channel, mute) of sink NAME in pactl's
    JSON, or (None, None)."""
    for s in graph.get("sinks") or []:
        if s.get("name") != name:
            continue
        vol = None
        for ch in (s.get("volume") or {}).values():
            if isinstance(ch, dict) and str(ch.get("value_percent", "")).rstrip("%").strip().isdigit():
                vol = int(str(ch["value_percent"]).rstrip("%"))
                break
        mute = s.get("mute")
        return vol, (mute if isinstance(mute, bool) else None)
    return None, None


def merge_patch(base, patch):
    """RFC 7386 JSON merge patch."""
    if not isinstance(patch, dict):
        return patch
    out = dict(base) if isinstance(base, dict) else {}
    for k, v in patch.items():
        if v is None:
            out.pop(k, None)
        else:
            out[k] = merge_patch(out.get(k), v)
    return out


def config_patch_allowed(patch):
    """Only providers.decisions, memory and enabled: what the app alone may
    set, which the gate seeds with the daemon stopped."""
    if not isinstance(patch, dict) or not patch:
        return False
    for k, v in patch.items():
        if k not in CONFIG_KEYS:
            return False
        sub = CONFIG_KEYS[k]
        if sub is not None and (not isinstance(v, dict) or not set(v) <= set(sub)):
            return False
    return True


def new_rows(before, after):
    """The ledger rows AFTER holds beyond BEFORE (the ledger drops its oldest
    rows past its bound, so the last row seen anchors it)."""
    if not before:
        return list(after)
    last = before[-1]
    for i in range(len(after) - 1, -1, -1):
        if after[i] == last:
            return list(after[i + 1:])
    return list(after[len(before):]) if len(after) > len(before) else list(after)


def at_seconds(stamp):
    """A Go RFC 3339 time (nanoseconds allowed) as epoch seconds, or None."""
    if not isinstance(stamp, str):
        return None
    m = re.match(r"^(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d)(\.\d+)?(Z|[+-]\d\d:\d\d)$", stamp)
    if not m:
        return None
    frac = float(m.group(2)) if m.group(2) else 0.0
    zone = "+00:00" if m.group(3) == "Z" else m.group(3)
    return datetime.datetime.fromisoformat(m.group(1) + zone).timestamp() + frac


def max_in_window(times, window=60.0):
    """The most TIMES inside any WINDOW seconds."""
    ts, best, j = sorted(t for t in times if t is not None), 0, 0
    for i, t in enumerate(ts):
        while ts[j] < t - window + 1e-9:
            j += 1
        best = max(best, i - j + 1)
    return best


def first_sentence_words(text):
    return len(voicekit.first_sentence(text or "").split())


def part_commits(lines, start_ev="bubble", span=3.0):
    """Commits per named part from the first START_EV line through SPAN
    seconds of ledger time: what one bubble cost."""
    start = next((l for l in lines if l.get("ev") == start_ev), None)
    if start is None:
        return None
    t0, counts = start.get("t", 0.0), {}
    for l in lines:
        if l.get("ev") == "commit" and l.get("part") and t0 <= l.get("t", 0.0) <= t0 + span:
            counts[l["part"]] = counts.get(l["part"], 0) + 1
    return counts


def decision_request_ok(rec, labels):
    """A decision request carried only the model, the utterance and the
    registry's labels: (ok, why)."""
    keys = set(rec.get("keys") or [])
    if not keys <= {"model", "state", "questions"}:
        return False, f"extra keys {sorted(keys - {'model', 'state', 'questions'})}"
    if not isinstance(rec.get("state"), str):
        return False, "the state is not the utterance as a JSON string"
    qs = rec.get("questions") or {}
    for qid, q in qs.items():
        offered = set((q or {}).get("labels") or [])
        extra = offered - set(labels) - {"none"}
        if extra:
            return False, f"question {qid} offers {sorted(extra)}"
    return (bool(qs), "no question" if not qs else "")


def decision_hardware(meminfo, cpuinfo):
    """The local decision route's hardware gate (gateway Decide, from
    localmodels' /proc scan): MemTotal >= 6 GiB and >= 4 physical cores.
    Returns (ok, MemTotal GiB, physical cores)."""
    m = re.search(r"^MemTotal:\s+(\d+)\s+kB", meminfo or "", re.M)
    gib = int(m.group(1)) / (1 << 20) if m else 0.0
    cores, phys = set(), None
    for line in (cpuinfo or "").splitlines():
        k, _, v = line.partition(":")
        k, v = k.strip(), v.strip()
        if k == "physical id":
            phys = v
        elif k == "core id":
            cores.add((phys, v))
    if not cores:
        cores = {(None, str(i)) for i, line in enumerate((cpuinfo or "").splitlines()) if line.startswith("processor")}
    return gib >= 6 and len(cores) >= 4, round(gib, 2), len(cores)


def query_of(url):
    return urllib.parse.parse_qs(urllib.parse.urlsplit(url).query).get("q", [None])[0]


def file_mode(path):
    try:
        return stat.S_IMODE(os.stat(path).st_mode)
    except OSError:
        return None


def selftest():
    checks = []

    def check(name, ok, value=None):
        checks.append({"name": name, "pass": bool(ok), "value": value})

    src = ('const (\n\tLineCantHere = "I can\'t do that here."\n\tlineMany = "On it."\n)\n'
           'x := i18n.T("Reminders are off. Turn them on in Oni\'s settings.")\n'
           'OpenApp Intent = "open_app" // Arg\n\tNewTab        Intent = "new_tab"\n')
    c = go_consts(src)
    check("go_consts reads constants and i18n.T lines", c.get("LineCantHere") == "I can't do that here."
          and c.get("lineMany") == "On it." and c.get("x") == "Reminders are off. Turn them on in Oni's settings.", c)
    check("go_intents reads the registry ids", go_intents(src) == ["open_app", "new_tab"], go_intents(src))
    us = user_strings(src + 'i18n.Tf("Skipped %s.", a)\n')
    check("user_strings finds lines and i18n arguments",
          "On it." in us and "I can't do that here." in us and "Skipped %s." in us, us)
    check("third_party_hits finds a model name as a word", third_party_hits(["Ask Claude for that."]) != []
          and third_party_hits(["Ryoku Ember sorts it.", "AskNothing", "phrase", "knots", "Nested"]) == [])
    log = ("refused: ryoku-host svc --user enable --now oni-routines.timer\n"
           "refused: systemd-run --user --scope --collect -- python3 /x/textwin.py\nnoise\nwake: ryoku-host a\n")
    rl = stub_lines(log)
    check("stub_lines parses refusal lines only", len(rl) == 2 and rl[0][0] == "ryoku-host", rl)
    check("stub_lines reads the wake log's own prefix", stub_lines(log, "wake") == [["ryoku-host", "a"]])
    check("wake_request: the bridge's and systemctl's enable and disable",
          wake_request(rl[0]) == "enable"
          and wake_request(["/l/bin/ryoku-host", "svc", "--user", "disable", "--now", "oni-routines"]) == "disable"
          and wake_request(["systemctl", "--user", "enable", "--now", "oni-routines.timer"]) == "enable")
    check("wake_request refuses any other unit or verb",
          wake_request(["ryoku-host", "svc", "--user", "enable", "--now", "oni.socket"]) is None
          and wake_request(["systemctl", "--user", "start", "--now", "oni-routines.timer"]) is None
          and wake_request(["ryoku", "svc", "--user", "enable", "--now", "oni-routines.timer"]) is None)
    here, out = "/r/release/lab/oni", "/dev/shm/oni-lab/out"
    want = launch_argv(here, out)
    check("fixture_launch: the notes entry bare is a fallback start",
          fixture_launch(want["notes"], here, out) == ("notes", False))
    check("fixture_launch: inside the systemd-run wrapper",
          fixture_launch(list(SCOPE_PREFIX) + want["notes"], here, out) == ("notes", True)
          and fixture_launch(["/dev/shm/oni-lab/bin/systemd-run"] + list(SCOPE_PREFIX[1:]) + want["notes"], here, out)
          == ("notes", True))
    check("fixture_launch: the browser with about:blank or an https address",
          fixture_launch(want["browser"] + ["about:blank"], here, out) == ("browser", False)
          and fixture_launch(want["browser"] + ["https://duckduckgo.com/?q=salt+and+pepper"], here, out)
          == ("browser", False))
    check("fixture_launch refuses other addresses, extra words and other programs",
          fixture_launch(want["browser"] + ["http://x"], here, out) is None
          and fixture_launch(want["browser"] + ["file:///etc/passwd"], here, out) is None
          and fixture_launch(want["browser"] + ["--new-tab", "about:blank"], here, out) is None
          and fixture_launch(want["notes"] + ["--x"], here, out) is None
          and fixture_launch(["sh", "-c", "python3"], here, out) is None
          and fixture_launch(["python3", "/elsewhere/textwin.py", NOTES_APP, NOTES_TITLE, out + "/notes.txt"],
                             here, out) is None)
    allowed = [
        {"argv": ["pactl", "set-sink-volume", "@DEFAULT_SINK@", "+5%"], "comm": "pactl"},
        {"argv": ["pactl", "set-sink-mute", "@DEFAULT_SINK@", "toggle"], "comm": "pactl"},
        {"argv": ["niri", "msg", "action", "maximize-column"], "comm": "niri"},
        {"argv": ["/dev/shm/oni-lab/bin/ryoku-host", "svc", "--user", "enable", "--now", "oni-routines.timer"],
         "comm": "ryoku-host"},
        {"argv": ["ryoku", "wm", "caps"], "comm": "ryoku"},
        {"argv": want["notes"], "comm": "python3"},
        {"argv": list(SCOPE_PREFIX) + want["browser"] + ["about:blank"], "comm": "systemd-run"},
        {"argv": ["pw-cat", "--record", "-"], "comm": "pw-cat"},
    ]
    refused = [
        {"argv": ["pactl", "set-sink-volume", "alsa_output.pci", "+5%"], "comm": "pactl"},
        {"argv": ["pactl", "load-module", "module-null-sink"], "comm": "pactl"},
        {"argv": ["niri", "msg", "action", "close-window"], "comm": "niri"},
        {"argv": ["wtype", "-M", "ctrl", "t"], "comm": "wtype"},
        {"argv": ["ryoku", "wm", "act", "window.place", "1"], "comm": "ryoku"},
        {"argv": ["systemd-run", "--user", "--scope", "--collect", "--", "firefox"], "comm": "systemd-run"},
        {"argv": ["xdg-open", "https://x"], "comm": "xdg-open"},
        {"argv": [], "comm": "x"},
    ]
    check("child_allowed13 accepts the executors, stubs, launches and the voice pipeline",
          all(child_allowed13(r, here, out) for r in allowed), [r["argv"] for r in allowed if not child_allowed13(r, here, out)])
    check("child_allowed13 refuses other devices, verbs, input programs and apps",
          not any(child_allowed13(r, here, out) for r in refused), [r["argv"] for r in refused if child_allowed13(r, here, out)])
    graph = {"sinks": [{"name": "oni_lab_mic", "volume": {"mono": {"value_percent": "100%"}}, "mute": False},
                       {"name": "oni_lab", "volume": {"front-left": {"value": 62259, "value_percent": "95%"},
                                                      "front-right": {"value_percent": "95%"}}, "mute": True}]}
    check("sink_level reads pactl's JSON", sink_level(graph, "oni_lab") == (95, True)
          and sink_level(graph, "nope") == (None, None), sink_level(graph, "oni_lab"))
    base = {"version": 2, "providers": {"local": {"llamacpp": "x"}, "decisions": {"route": "local", "allowHosted": True}}}
    m = merge_patch(base, {"providers": {"decisions": {"route": "ember", "allowHosted": None}}})
    check("merge_patch changes, removes and keeps", m == {"version": 2, "providers": {"local": {"llamacpp": "x"},
                                                                                     "decisions": {"route": "ember"}}}, m)
    check("config_patch_allowed: decisions, memory and enabled only",
          config_patch_allowed({"providers": {"decisions": {"route": "local"}}})
          and config_patch_allowed({"memory": {"use": False}, "enabled": True})
          and not config_patch_allowed({"providers": {"local": {"llamacpp": "http://elsewhere"}}})
          and not config_patch_allowed({"teams": {}}) and not config_patch_allowed({}))
    r1 = [{"at": "a", "intent": "x"}, {"at": "b", "intent": "y"}]
    r2 = r1[1:] + [{"at": "c", "intent": "z"}]
    check("new_rows survives a bounded ledger dropping its oldest row", new_rows(r1, r2) == [{"at": "c", "intent": "z"}]
          and new_rows([], r1) == r1 and new_rows(r1, r1) == [])
    t = at_seconds("2026-11-23T10:00:01.123456789+01:00")
    check("at_seconds reads Go's nanosecond times", t is not None and abs(t - (at_seconds("2026-11-23T09:00:01Z") + 0.123456789)) < 1e-6
          and at_seconds("nope") is None, t)
    check("max_in_window counts the busiest minute", max_in_window([0, 10, 59, 61, 62, 200]) == 4
          and max_in_window([]) == 0)
    check("first_sentence_words counts the first sentence", first_sentence_words("Opening Lindqvist Notes. More words here.") == 3)
    lines = [{"ev": "commit", "part": "bubble", "t": 0.5}, {"ev": "bubble", "t": 1.0},
             {"ev": "commit", "part": "bubble", "t": 1.1}, {"ev": "commit", "t": 1.2},
             {"ev": "commit", "part": "strand", "t": 1.3}, {"ev": "commit", "part": "bubble", "t": 9.0}]
    check("part_commits counts named parts inside the bubble's span",
          part_commits(lines) == {"bubble": 1, "strand": 1} and part_commits([]) is None, part_commits(lines))
    labels = ["open_app", "new_tab"]
    good = {"keys": ["model", "state", "questions"], "state": "boot up notes",
            "questions": {"intent": {"type": "choice", "labels": ["open_app", "new_tab", "none"]}}}
    check("decision_request_ok accepts the utterance and the registry's labels", decision_request_ok(good, labels)[0])
    check("decision_request_ok refuses extra keys, a structured state and foreign labels",
          not decision_request_ok(dict(good, keys=["model", "state", "questions", "screen"]), labels)[0]
          and not decision_request_ok(dict(good, state={"utterance": "x", "title": "y"}), labels)[0]
          and not decision_request_ok(dict(good, questions={"intent": {"labels": ["send_message"]}}), labels)[0])
    check("query_of decodes a search address", query_of("https://duckduckgo.com/?q=salt+and+pepper") == "salt and pepper")
    cpu4 = "".join(f"processor\t: {i}\nphysical id\t: 0\ncore id\t\t: {i // 2}\n\n" for i in range(8))
    cpu2 = "".join(f"processor\t: {i}\nphysical id\t: 0\ncore id\t\t: {i // 2}\n\n" for i in range(4))
    check("decision_hardware: 6 GiB and 4 physical cores (threads not counted)",
          decision_hardware("MemTotal:       16303104 kB\n", cpu4) == (True, 15.55, 4)
          and decision_hardware("MemTotal:        4000000 kB\n", cpu4)[0] is False
          and decision_hardware("MemTotal:       16303104 kB\n", cpu2)[0] is False,
          [decision_hardware("MemTotal:       16303104 kB\n", cpu4), decision_hardware("MemTotal:  1 kB\n", cpu2)])
    names = [n for n in ("child_allowed", "leaks") if not callable(getattr(guidekit, n, None))]
    check("guidekit still offers the helpers this kit uses", not names, names)
    check("every fixture text is set", all(t.strip() for t, _ in REFLEX_FIXTURES.values()))
    check("the storm fixture repeats its phrase 43 times", REFLEX_FIXTURES["storm"][0].count("open notes") == STORM_REPEATS)
    ok = all(c["pass"] for c in checks)
    print(json.dumps({"pass": ok, "checks": checks}, indent=2, default=str))
    return 0 if ok else 1


def render_fixtures(dst):
    dst = Path(dst)
    if not str(dst).startswith("/dev/shm/"):
        raise SystemExit("reflexkit: DIR must be under /dev/shm")
    dst.mkdir(parents=True, exist_ok=True)
    ver = subprocess.run(["espeak-ng", "--version"], capture_output=True, text=True, check=True).stdout.strip()
    meta = {"engine": ver, "lead_s": LEAD_S, "tail_s": TAIL_S, "fixtures": {}}
    for name, (text, wpm) in REFLEX_FIXTURES.items():
        raw = dst / f"{name}.raw.wav"
        # -w writes a file: espeak-ng never opens an audio device here
        subprocess.run(["espeak-ng", "-v", "en-us", "-s", str(wpm), "-w", str(raw), text],
                       check=True, capture_output=True, timeout=120, env={"PATH": "/usr/bin:/bin", "HOME": str(dst)})
        pcm, rate = read_wav16(raw)
        raw.unlink()
        loud = [i for i, v in enumerate(pcm) if abs(v) > 0.015]
        if not loud:
            raise RuntimeError(f"espeak-ng rendered silence for {name}")
        body = pcm[max(0, loud[0] - int(0.02 * rate)):loud[-1] + int(0.05 * rate)]
        pcm_out = [0.0] * int(LEAD_S * rate) + body + [0.0] * int(TAIL_S * rate)
        write_wav16(dst / f"{name}.wav", pcm_out, rate)
        meta["fixtures"][name] = {"file": str(dst / f"{name}.wav"), "text": text, "rate": rate,
                                  "seconds": round(len(pcm_out) / rate, 3), "speech_s": round(len(body) / rate, 3),
                                  "wpm": wpm}
    (dst / "fixtures.json").write_text(json.dumps(meta, indent=2) + "\n")
    print(f"rendered {len(REFLEX_FIXTURES)} quick-command speech fixtures into {dst} ({ver})")
    return 0


# --- the frame watch, pausable -------------------------------------------------

class PausableWatch(FrameWatch):
    """voicekit's FrameWatch that can be held off: `hyakki oni watch` starts
    the daemon when it is down, so idle-exit and restart checks pause it."""

    def __init__(self, lab_id):
        super().__init__(lab_id)
        self.running = threading.Event()
        self.running.set()

    def run(self):
        while not self.done.is_set():
            if not self.running.is_set():
                self.done.wait(0.2)
                continue
            self.proc = subprocess.Popen([str(HERE / "ctl.sh"), "hyakki", "oni", "watch"], stdout=subprocess.PIPE,
                                         stderr=subprocess.DEVNULL, stdin=subprocess.DEVNULL, text=True)
            for raw in self.proc.stdout:
                try:
                    self.lines.append({"wall": time.time(), "frame": json.loads(raw)})
                except ValueError:
                    continue
            self.proc.wait()
            if not self.done.is_set() and self.running.is_set():
                self.restarts += 1
                self.done.wait(0.5)

    def _kill(self):
        for pid in lab_pids(self.lab_id, "hyakki"):
            try:
                if b"watch" in open(f"/proc/{pid}/cmdline", "rb").read():
                    os.kill(pid, signal.SIGTERM)
            except OSError:
                pass

    def pause(self):
        self.running.clear()
        # twice: a watch the loop was starting as the flag fell
        for _ in range(2):
            self._kill()
            if self.proc:
                try:
                    self.proc.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    self.proc.kill()
            time.sleep(0.7)

    def resume(self):
        self.running.set()
        time.sleep(1.0)


# --- the gate ----------------------------------------------------------------

class Phase13(Phase11):
    REPORT = "phase13.json"

    def __init__(self, lab, out):
        super().__init__(lab, out)
        for d in ("crops", "model", "fixtures"):
            (self.out / d).mkdir(parents=True, exist_ok=True)
        self.watch = PausableWatch(self.env["LAB_ID"])
        self.rspec = self.spec["reflex"]
        self.rfx = json.loads((self.lab / "voice/reflex/fixtures.json").read_text())
        self.labout = self.lab / "out"
        self.cfg_path = self.lab / "cfg/hyakki/hyakki.json"
        self.reminders_path = self.lab / "state/hyakki/oni-reminders.json"
        self.routines_path = self.lab / "state/hyakki/oni-reflex-routines.json"
        self.refused_log = self.lab / "log/refused.log"
        self.wake_log = self.lab / "log/wake.log"
        src = {p: (APP / p).read_text() for p in ("internal/reflex/act/act.go", "internal/daemon/oni_reflex.go",
                                                   "internal/oni/reflex/rules.go", "internal/daemon/oni_remind.go",
                                                   "internal/oni/voice/route/route.go")}
        self.lines13 = {}
        for text in src.values():
            for k, v in go_consts(text).items():
                self.lines13.setdefault(k, v)
        self.labels = go_intents((APP / "internal/oni/reflex/intent.go").read_text())
        self.fixture_pids = set()
        self.results["reflex"] = {
            "spec": self.rspec, "registry": self.labels, "character_name": CHARACTER_NAME,
            "fixtures": {k: v["text"] for k, v in self.rfx["fixtures"].items()},
            "lines": {k: v for k, v in self.lines13.items() if k.lower().startswith("line")},
            "catalogue": {"notes": NOTES_APP, "browser": BROWSER_APP, "argv": launch_argv(HERE, self.labout)},
            "model_note": "model.py and decidefix.py are loopback fixtures: they prove routing, thresholds, "
                          "fall-through and model-free replay, not model quality or calibration"}

    def line(self, name):
        return self.lines13.get(name, "")

    # --- plumbing ---

    def step(self, number, name, fn):
        # ONI_LAB_P13_STEPS=6,7 runs step 0 and the listed steps only, to
        # retry them after a fix; every other step is recorded as skipped and
        # failed, so such a run never passes the gate
        only = os.environ.get("ONI_LAB_P13_STEPS", "")
        if only and number != 0 and str(number) not in only.split(","):
            self.results["steps"].append({"step": number, "name": name, "pass": False,
                                          "error": "skipped: not in ONI_LAB_P13_STEPS"})
            return None
        try:
            return super().step(number, name, fn)
        finally:
            # a failed step may leave a mode, open mic, wake, a hidden Oni, a
            # paused watch or the systemd marker behind
            for undo in (lambda: self.watch.resume() if not self.watch.running.is_set() else None,
                         lambda: self.sh("phase13.sh", "scope", "off", check=False),
                         lambda: self.decide_mode("keyword"),
                         lambda: self.vset(openMic=False, wake={"enabled": False, "phrase": WAKE_PHRASE}),
                         lambda: self.settings(hidden=False)):
                try:
                    undo()
                except (OSError, RuntimeError, ValueError, subprocess.TimeoutExpired):
                    pass

    def jcli(self, *args, timeout=120):
        r = self.cli(*args, stdin="", timeout=timeout)
        try:
            value = json.loads(r.stdout) if r.stdout.strip() else {}
        except ValueError:
            value = {"raw": r.stdout.strip()}
        return r.returncode, value, r.stderr.strip()

    def rset(self, reflex=None, **switches):
        patch = {}
        if reflex is not None:
            patch["reflex"] = reflex
        if switches:
            patch["reflexSettings"] = switches
        return self.settings(**patch)

    def rlog(self):
        """The reflex ledger oldest first: oni.reflex.log serves it newest
        first (protocol README), and new_rows anchors on the last row seen."""
        rc, v, _ = self.jcli("oni", "reflex", "log", "--json")
        return list(reversed(v.get("rows") or [])) if rc == 0 else None

    def rows_since(self, before):
        after = self.rlog()
        return new_rows(before or [], after or [])

    def reminders(self):
        rc, v, err = self.jcli("oni", "reminders", "--json")
        return rc, v, err

    def routines(self):
        rc, v, _ = self.jcli("oni", "routines", "--json")
        return (v.get("routines") or []) if rc == 0 else None

    def caps(self):
        st = self.call("oni.status", {})
        return {f.get("id"): f for f in ((st.get("capabilities") or {}).get("features") or [])}

    def capability_checks(self):
        st = self.call("oni.status", {})
        features = ((st.get("capabilities") or {}).get("features") or [])
        caps = {f.get("id"): f for f in features}
        checks = [{"name": "the full capability inventory (including Phase 16, no duplicate IDs)",
                   "value": list(caps), "pass": set(caps) == set(CAPABILITY_ROWS) and len(features) == len(caps)}]
        for name, want in CAPABILITY_ROWS.items():
            row = caps.get(name) or {}
            got = tuple(row.get(k) for k in ("stage", "egress", "consent"))
            checks.append({"name": f"{name}: compiled stage, egress and consent", "value": got, "pass": got == want})
        return checks

    def close_ui(self):
        """Remove only this lab's app client before idle or shutdown proofs.
        The caller must pause the watch too; neither may reconnect oni."""
        self.sh("phase13.sh", "ui", "close", timeout=60)
        if lab_pids(self.env["LAB_ID"], "hyakki-app"):
            raise RuntimeError("the isolated app still holds a daemon connection")

    def open_ui(self):
        self.sh("phase13.sh", "ui", "open", timeout=60)

    def decisions(self):
        try:
            return json.loads(self.model._req("/_lab/decisions"))
        except (OSError, ValueError):
            return []

    def decisions_reset(self):
        self.model._req("/_lab/decisions/reset", {})

    def decide_mode(self, mode, delay=None):
        body = {"mode": mode}
        if delay is not None:
            body["delay"] = delay
        self.model._req("/_lab/decide", body)

    def jsonl(self, path):
        out = []
        try:
            for line in Path(path).read_text().splitlines():
                try:
                    out.append(json.loads(line))
                except ValueError:
                    continue
        except OSError:
            pass
        return out

    def mpris(self):
        return self.jsonl(self.lab / "log/mpris.jsonl")

    def notices(self):
        return [n for n in self.jsonl(self.lab / "log/notify.jsonl") if n.get("member") == "Notify"]

    def tabs(self):
        try:
            return json.loads((self.labout / "tabs.json").read_text())
        except (OSError, ValueError):
            return None

    def tab_requests(self):
        return self.jsonl(self.labout / "tabs-requests.jsonl")

    def stubbed(self, prefix="refused"):
        path = self.wake_log if prefix == "wake" else self.refused_log
        try:
            return stub_lines(path.read_text(), prefix)
        except OSError:
            return []

    def cfg(self):
        return json.loads(self.cfg_path.read_text())

    def graph(self):
        return json.loads(self.sh("audio.sh", "graph").stdout)

    def daemon_pids(self):
        return lab_pids(self.env["LAB_ID"], "oni")

    def wait_daemon(self, up, timeout):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if bool(self.daemon_pids()) == up:
                return True
            time.sleep(0.2)
        return bool(self.daemon_pids()) == up

    def process_start_ticks(self, pid):
        """Linux process identity, not just a PID that may have been reused."""
        try:
            fields = Path(f"/proc/{pid}/stat").read_text().rsplit(")", 1)[1].split()
            return int(fields[19])  # field 22; suffix begins with field 3
        except (OSError, ValueError, IndexError):
            return None

    def file_decoders(self):
        """Live isolated decoders holding the private WAV their adapter owns.
        Residual files with no such process remain privacy failures."""
        workers = []
        for pid in lab_pids(self.env["LAB_ID"]):
            argv = [a.decode(errors="replace") for a in self._cmdline(pid).split(b"\0") if a]
            files = [a for a in argv if a.startswith(str(self.lab) + "/")
                     and "/hyakki-stt-" in a and a.endswith("/speech.wav")]
            if files:
                workers.append({"pid": pid, "start_ticks": self.process_start_ticks(pid),
                                "comm": comm_of(pid), "argv": argv, "files": files})
        return workers

    def drain_file_decoders(self):
        """Wait for real decoder completion, never remove or exempt its WAV."""
        before = self.file_decoders()
        deadline = time.monotonic() + self.vspec["budgets"]["finalS"] + 12
        workers, quiet_since = before, None
        while time.monotonic() < deadline:
            workers = self.file_decoders()
            now = time.monotonic()
            if workers:
                quiet_since = None
            elif quiet_since is None:
                quiet_since = now
            elif now - quiet_since >= 0.3:
                return {"idle": True, "before": before, "remaining": []}
            time.sleep(0.1)
        return {"idle": False, "before": before, "remaining": workers}

    def daemon_stop(self):
        self.sh("phase13.sh", "daemon", "stop", timeout=120)

    def daemon_start(self):
        self.sh("phase13.sh", "daemon", "start", timeout=120)
        self.call("oni.status", {})
        time.sleep(2.0)

    def restart(self):
        """The daemon again with the watch held off between, so nothing
        starts one in the gap."""
        self.watch.pause()
        try:
            self.daemon_stop()
            self.daemon_start()
        finally:
            self.watch.resume()

    def write_config(self, patch):
        """Merge PATCH (only what config_patch_allowed admits) into the lab's
        hyakki.json; the caller has stopped the daemon and paused the watch,
        or the running daemon would write its own copy over it."""
        if not config_patch_allowed(patch):
            raise RuntimeError(f"the gate may not rewrite {sorted(patch)}")
        if self.daemon_pids():
            raise RuntimeError("hyakki.json is rewritten only with the daemon stopped")
        mode = file_mode(self.cfg_path) or 0o600
        new = merge_patch(self.cfg(), patch)
        part = self.cfg_path.with_name(self.cfg_path.name + ".part")
        part.write_text(json.dumps(new, indent=2) + "\n")
        os.chmod(part, mode)
        os.replace(part, self.cfg_path)
        return new

    def reconfigure(self, patch):
        """Write what only the app may set (providers.decisions, memory,
        enabled) into the lab's hyakki.json with the daemon stopped, then
        start it again."""
        if not config_patch_allowed(patch):
            raise RuntimeError(f"the gate may not rewrite {sorted(patch)}")
        self.watch.pause()
        try:
            self.daemon_stop()
            if not self.wait_daemon(False, 15):
                raise RuntimeError("the daemon did not stop for a settings rewrite")
            self.write_config(patch)
            self.daemon_start()
        finally:
            self.watch.resume()
        return self.cfg()

    # --- speech ---

    def rinject(self, name):
        t0 = time.time()
        self.sh("audio.sh", "inject", self.rfx["fixtures"][name]["file"], timeout=120)
        return t0, time.time()

    def rhold(self, name, lead=0.3, tail=0.4):
        r = {"fixture": name, "mark": self.watch.mark()}
        r["down"] = self.keys.send("down")
        w, d, _ = self.wait_voice(r["mark"], lambda d: d.get("phase") == "listening", 3.0, "listening")
        r["listening"] = w
        time.sleep(lead)
        r["inject"] = self.rinject(name)
        time.sleep(tail)
        r["up"] = self.keys.send("up")
        return r

    def spoken(self, mark, until=None):
        return [d.get("text") for _, d in self.voice_since(mark, until) if d.get("phase") == "speaking" and d.get("text")]

    def bubbles(self, mark):
        return [(w, d.get("text")) for w, d in self.watch.frames("bubble", mark) if d.get("text")]

    def settle_voice(self, mark, timeout=15.0, quiet=1.5):
        """Wait until the voice frames after MARK have gone quiet (idle or
        armed for QUIET seconds) or TIMEOUT passed; the reflex run's actions
        finish in the background, so a short rest follows."""
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            frames = self.voice_since(mark)
            if frames and frames[-1][1].get("phase") in ("idle", "armed") and time.time() - frames[-1][0] >= quiet:
                break
            time.sleep(0.2)
        time.sleep(0.8)

    def utter(self, name, timeout=15.0):
        """Hold the chord, say fixture NAME, let go, and wait for Oni's
        answer: the final transcript and every line it said."""
        r = self.rhold(name)
        self.final_of(r)
        self.settle_voice(r["mark"], timeout)
        final = r.get("final")
        return {"fixture": name, "said": self.rfx["fixtures"][name]["text"], "heard": final[1] if final else None,
                "final_at": final[0] if final else None,
                "lines": self.spoken(r["mark"]), "mark": r["mark"], "down": r["down"], "up": r["up"]}

    def wake_say(self, name, timeout=15.0):
        """The wake phrase, then fixture NAME inside the wake session."""
        m = self.watch.mark()
        self.vset(wake={"enabled": True, "phrase": WAKE_PHRASE})
        self.wait_voice(m, lambda d: d.get("phase") == "armed", 8, None)
        time.sleep(1.0)
        m2 = self.watch.mark()
        self.inject("wake")
        opened = self.wait_voice(m2, lambda d: d.get("phase") == "listening", 10, None)
        self.rinject(name)
        self.settle_voice(m2, timeout)
        heard = [d.get("text") for _, d in self.voice_since(m2) if d.get("phase") == "transcribing" and d.get("text")]
        self.vset(wake={"enabled": False, "phrase": WAKE_PHRASE})
        return {"fixture": name, "source": "wake", "opened": opened[1] is not None,
                "heard": heard[-1] if heard else None, "lines": self.spoken(m2), "mark": m2}

    def open_say(self, name, timeout=15.0):
        """Play real PCM into a newly armed open microphone, then await its
        actual final transcript. The initial armed frame is not completion."""
        m = self.watch.mark()
        self.vset(openMic=True)
        opened = self.wait_voice(m, lambda d: d.get("source") == "open" and d.get("phase") == "armed",
                                 5, "open microphone armed")
        try:
            time.sleep(0.4)
            injected = self.rinject(name)
            final = self.wait_voice(opened[2], lambda d: d.get("source") == "open"
                                    and d.get("phase") == "transcribing" and bool(d.get("text")),
                                    timeout, None)
            if final[1] is not None:
                self.settle_voice(final[2] - 1, timeout)
            heard = [d.get("text") for _, d in self.voice_since(opened[2])
                     if d.get("source") == "open" and d.get("phase") == "transcribing" and d.get("text")]
            return {"fixture": name, "source": "open", "opened": True,
                    "heard": heard[-1] if heard else None, "lines": self.spoken(m), "mark": m,
                    "injected": injected, "transcribed_at": final[0]}
        finally:
            self.vset(openMic=False)

    def new_run(self):
        """A fresh voice run (a routine is offered once per run)."""
        self.settings(voice=False)
        time.sleep(1.0)
        self.settings(voice=True)
        time.sleep(2.0)

    # --- launched apps ---

    def launches(self, kind=None, since=0.0):
        """Every catalogue start the daemon ran since SINCE: (record, kind,
        scoped)."""
        out = []
        for rec in self.vchildren.procs(since=since):
            if rec["first"] < since:
                continue
            fl = fixture_launch(rec["argv"], HERE, self.labout)
            if fl and (kind is None or fl[0] == kind):
                out.append((rec, fl[0], fl[1]))
                self.fixture_pids.add(rec["pid"])
        return out

    def windows(self, app_id):
        return [w for w in self.state()["windows"] if w.get("app_id") == app_id]

    def wait_window(self, app_id, timeout=15.0, gone=False):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            ws = self.windows(app_id)
            if bool(ws) != gone:
                return ws
            time.sleep(0.2)
        return self.windows(app_id)

    def close_apps(self):
        """End every catalogue app the daemon started, by pid, once its scope
        shows it is the lab's."""
        tag = f"oni-lab-{self.env['LAB_ID']}-"
        self.launches()
        for pid in list(self.fixture_pids):
            try:
                if (scope_of(pid) or "").startswith(tag) and comm_of(pid) == "python3":
                    os.kill(pid, signal.SIGTERM)
            except OSError:
                pass
        for app in (NOTES_APP, BROWSER_APP):
            self.wait_window(app, 10, gone=True)
        for name in ("tabs.json", "tabs-requests.jsonl"):
            # the browser's record starts over with its next window
            try:
                (self.labout / name).rename(self.out / "fixtures" / f"{name}.{int(time.time() * 1000)}")
            except OSError:
                pass

    def perch_fixture(self):
        """Oni perched on window.sh's notes fixture (dev.ryoku.OniFixture,
        not a catalogue app): a reminder's bubble needs Oni on screen, not
        hidden, and perched, since a bubble hangs from the strand."""
        self.settings(hidden=False)
        if self.fixture("notes") is None:
            self.fixture_open("notes")
        self.perch_on("notes")

    def cooldown(self):
        time.sleep(self.rspec["launchCooldownS"] + 0.6)

    def requests_n(self):
        return len(self.model_requests())

    # --- step 0: census ---

    def census13(self):
        voice, _ = self.census11()
        checks = [dict(c, name="voice: " + c["name"]) for c in voice["checks"]]
        checks += self.capability_checks()
        o = self.oni_settings()
        rs = o.get("reflexSettings") or {}
        checks.append({"name": "quick commands and their four switches are off by default",
                       "value": {"reflex": o.get("reflex"), "reflexSettings": rs},
                       "pass": not o.get("reflex") and set(rs) >= set(SWITCHES) and not any(rs.get(k) for k in SWITCHES)})
        checks.append({"name": "no reminders or routines file yet",
                       "value": [str(p) for p in (self.reminders_path, self.routines_path) if p.exists()],
                       "pass": not self.reminders_path.exists() and not self.routines_path.exists()})
        refused = self.stubbed()
        checks.append({"name": "bin/systemd-run is the lab's refusal stub (its census line in refused.log)",
                       "value": refused[-3:], "pass": ["systemd-run", "--lab-census"] in refused})
        bins = {n: (self.lab / "bin" / n).is_file() for n in ("systemctl", "ryoku", "ryoku-shell", "ryoku-host", "systemd-run")}
        checks.append({"name": "every refusal stub is in the lab's bin", "value": bins, "pass": all(bins.values())})
        entries = {a: (self.lab / "data/applications" / f"{a}.desktop").is_file() for a in (NOTES_APP, BROWSER_APP)}
        try:
            mime = (self.lab / "cfg/mimeapps.list").read_text()
        except OSError:
            mime = ""
        checks.append({"name": "the catalogue's two entries, the lab browser the https default",
                       "value": {"entries": entries, "mimeapps": mime.splitlines()[:4]},
                       "pass": all(entries.values()) and f"x-scheme-handler/https={BROWSER_APP}.desktop" in mime})
        ready = {n: "ready" in (self.lab / "log" / f"{n}.log").read_text(errors="replace").split()
                 for n in ("mpris", "notify") if (self.lab / "log" / f"{n}.log").exists()}
        checks.append({"name": "the MPRIS player and the notification server own their names on the lab bus",
                       "value": ready, "pass": ready.get("mpris") is True and ready.get("notify") is True})
        dec = ((self.cfg().get("providers") or {}).get("decisions") or {})
        addr = urllib.parse.urlsplit(dec.get("localAddress") or "")
        checks.append({"name": "the decision route is seeded local, loopback, the fixture's model, oni.reflex local",
                       "value": dec,
                       "pass": dec.get("route") == "local" and addr.hostname == "127.0.0.1"
                       and dec.get("localModel") == DECISION_MODEL and (dec.get("features") or {}).get("oni.reflex") == "local"
                       and not dec.get("allowHosted")})
        r = self.rspec
        checks.append({"name": "the spec's reflex numbers (to be ratified)",
                       "value": {k: r.get(k) for k in ("maxActions", "maxActionsPerMinute", "launchCooldownS", "danglingVerbS")},
                       "pass": r.get("maxActions") == 6 and r.get("launchCooldownS") == 4 and r.get("danglingVerbS") == 6
                       and (r.get("maxActionsPerMinute") or 0) > 0 and r["reminders"]["maxDays"] == 7
                       and r["reminders"]["maxTimerH"] == 24 and r["reminders"]["wakeHorizonMin"] == 30
                       and r["reminders"]["residencyHorizonMin"] == 35 and r["routines"]["offerMinSteps"] == 3
                       and r["routines"]["gapS"] == 20 and r["routines"]["maxRoutines"] == 20
                       and r["routines"]["maxSteps"] == 8 and r["routines"]["chipS"] == 12})
        units = sorted(p.name for p in list((APP / "systemd").glob("*")) + list((APP / "runit").glob("*")))
        checks.append({"name": "no unit or runit service for reflexes, reminders or routines of their own",
                       "value": units, "pass": not [u for u in units if re.search(r"reflex|remind", u)]})
        at_rest = [(p, comm_of(p)) for p in lab_pids(self.env["LAB_ID"])
                   if comm_of(p) in INPUT_PROGRAMS or b"textwin.py" in self._cmdline(p) or b"tabwin.py" in self._cmdline(p)]
        checks.append({"name": "no catalogue app or input program in the lab at rest", "value": at_rest, "pass": not at_rest})
        checks.append({"name": "the decision fixture has heard nothing", "value": len(self.decisions()),
                       "pass": self.decisions() == []})
        checks.append({"name": "the registry offers no send, buy, pay, delete, submit, post, install, remove or erase intent",
                       "value": self.labels,
                       "pass": bool(self.labels) and not [i for i in self.labels if re.search(
                           r"send|buy|pay|delete|submit|post|install|remove|erase|format|reboot|shutdown|sudo", i)]})
        return {"checks": checks}, self.checks_pass(checks)

    def _cmdline(self, pid):
        try:
            return open(f"/proc/{pid}/cmdline", "rb").read()
        except OSError:
            return b""

    # --- step 1: off is zero ---

    def off13(self):
        self.rset(reflex=False, **{k: False for k in SWITCHES})
        caps = self.caps()
        self.model.reset()
        t0 = time.time()
        u = self.utter("open_notes")
        time.sleep(2.0)
        reqs = self.model_requests()
        asked = [voicekit.asked_text(q) for q in reqs]
        log_rc, _, log_err = self.jcli("oni", "reflex", "log", "--json")
        launched = self.launches(since=t0)
        checks = [
            {"name": "the five reflex rows are denied with reflex off", "value": {k: caps.get(k) for k in REFLEX_ROWS},
             "pass": all(k in caps and caps[k].get("allow") is False for k in REFLEX_ROWS)},
            {"name": "the words became an ordinary Ask (one model request)", "value": asked,
             "pass": len(reqs) == 1},
            {"name": "nothing launched, no notes window", "value": [l[0]["argv"] for l in launched],
             "pass": not launched and not self.windows(NOTES_APP)},
            {"name": "oni reflex log answers disabled", "value": [log_rc, log_err[-200:]], "pass": log_rc != 0},
            {"name": "no reminders or routines file", "value": [self.reminders_path.exists(), self.routines_path.exists()],
             "pass": not self.reminders_path.exists() and not self.routines_path.exists()},
        ]
        return {"utterance": u, "checks": checks}, self.checks_pass(checks)

    # --- step 2: launch ---

    def launch13(self):
        checks, rows_all = [], {}
        self.rset(reflex=True)
        rows0 = self.rlog() or []
        refused0 = len(self.stubbed())
        t0 = time.time()
        u = self.utter("open_notes")
        ws = self.wait_window(NOTES_APP, 15)
        time.sleep(1.0)
        launched = self.launches("notes", t0)
        rows = self.rows_since(rows0)
        rows_all["press"] = rows
        new_refused = self.stubbed()[refused0:]
        acks = [l for l in u["lines"] if l]
        child = launched[0][0] if launched else None
        env = {}
        if child:
            try:
                for entry in open(f"/proc/{child['pid']}/environ", "rb").read().split(b"\0"):
                    k, _, v = entry.decode(errors="replace").partition("=")
                    if k in ("WAYLAND_DISPLAY", "DBUS_SESSION_BUS_ADDRESS", "XDG_RUNTIME_DIR") or k.startswith("HYAKKI_"):
                        env[k] = v
            except OSError:
                pass
        checks += [
            {"name": "a Press opens the notes window, listed by the compositor", "value": ws, "pass": bool(ws)},
            {"name": "exactly one start, the catalogue's argv bare (scope=fallback: no user manager in the lab)",
             "value": [(l[0]["argv"], l[2]) for l in launched],
             "pass": len(launched) == 1 and launched[0][2] is False},
            {"name": "no systemd-run was asked without a user manager", "value": new_refused,
             "pass": not [a for a in new_refused if os.path.basename(a[0]) == "systemd-run"]},
            {"name": "one ack, its first sentence at most five words", "value": acks,
             "pass": len(acks) == 1 and first_sentence_words(acks[0]) <= 5},
            {"name": "one done row for open_app (intent, route, ms, confidence; no words)", "value": rows,
             "pass": len([r for r in rows if r.get("intent") == "open_app" and r.get("outcome") == "done"]) == 1
             and all(set(r) <= {"at", "intent", "route", "ms", "confidence", "outcome"} for r in rows)},
            {"name": "the app keeps the lab's display and bus, and no HYAKKI_ variable", "value": env,
             "pass": env.get("WAYLAND_DISPLAY") == self.env.get("LAB_DISPLAY")
             and env.get("DBUS_SESSION_BUS_ADDRESS") == f"unix:path={self.lab / 'rt/bus'}"
             and not [k for k in env if k.startswith("HYAKKI_")]},
        ]
        # the app outlives the daemon's TERM
        survived = None
        if child:
            self.watch.pause()
            try:
                self.daemon_stop()
                time.sleep(1.0)
                survived = os.path.exists(f"/proc/{child['pid']}") and b"textwin.py" in self._cmdline(child["pid"])
                still = bool(self.windows(NOTES_APP))
                self.daemon_start()
            finally:
                self.watch.resume()
            checks.append({"name": "the app outlives oni's TERM (and its window stays)", "value": {"alive": survived, "window": still},
                           "pass": survived is True and still})
        else:
            checks.append({"name": "the app outlives oni's TERM", "value": None, "pass": False})
        # with the user manager's marker the launcher asks systemd-run first;
        # the lab's stub refuses, and the same argv starts bare
        self.close_apps()
        self.cooldown()
        self.sh("phase13.sh", "scope", "on")
        try:
            refused1 = len(self.stubbed())
            t1 = time.time()
            us = self.utter("open_notes")
            self.wait_window(NOTES_APP, 15)
            time.sleep(1.0)
            scoped = self.stubbed()[refused1:]
            launched2 = self.launches("notes", t1)
        finally:
            self.sh("phase13.sh", "scope", "off")
        wrapped = [a for a in scoped if os.path.basename(a[0]) == "systemd-run"]
        want = launch_argv(HERE, self.labout)["notes"]
        checks += [
            {"name": "with a user manager the start is asked of systemd-run --user --scope --collect -- EXEC (refused)",
             "value": wrapped, "pass": len(wrapped) == 1
             and " ".join(wrapped[0][1:]) == " ".join(list(SCOPE_PREFIX[1:]) + want)},
            {"name": "then the same argv starts bare, once (scope=fallback, reason scope unavailable)",
             "value": [(l[0]["argv"], l[2]) for l in launched2],
             "pass": len([l for l in launched2 if not l[2]]) == 1},
        ]
        # open mic: only when Oni is named
        self.close_apps()
        self.cooldown()
        t2 = time.time()
        plain = self.open_say("open_notes")
        time.sleep(1.5)
        plain_launch = self.launches("notes", t2)
        self.cooldown()
        named_before = self.rlog() or []
        t3 = time.time()
        named = self.open_say("open_named")
        time.sleep(1.5)
        named_launch = self.launches("notes", t3)
        named_rows = self.rows_since(named_before)
        heard_name = bool(re.search(r"\b" + re.escape(CHARACTER_NAME) + r"\b", named.get("heard") or "", re.I))
        checks.append({"name": "recognized open mic without the character's name opens nothing", "value": plain,
                       "pass": plain["opened"] and bool(plain["heard"]) and not plain_launch})
        checks.append({"name": f"real open-mic PCM naming {CHARACTER_NAME} is transcribed and opens it exactly once",
                       "value": {"utterance": named, "launches": len(named_launch), "rows": named_rows},
                       "pass": named["opened"] and heard_name and len(named_launch) == 1
                       and len([r for r in named_rows if r.get("intent") == "open_app"
                                and r.get("outcome") == "done"]) == 1})
        self.close_apps()
        return {"utterances": [u, us, plain, named], "rows": rows_all, "checks": checks}, self.checks_pass(checks)

    # --- step 3: tabs ---

    def tabs13(self):
        self.rset(reflex=True)
        self.close_apps()
        self.cooldown()
        t0 = time.time()
        a = self.utter("new_tab")
        first = self._wait_tabs(lambda s: s and s.get("count", 0) >= 1, 15)
        self.cooldown()
        b = self.utter("new_tab")
        second = self._wait_tabs(lambda s: s and s.get("count", 0) >= (first or {}).get("count", 0) + 1, 15)
        before = self.tabs()
        n = self.utter("next_tab")
        c = self.utter("close_tab")
        time.sleep(1.0)
        after = self.tabs()
        launched = self.launches("browser", t0)
        inputs = self.input_programs(t0)
        cant = self.line("lineCantHere")
        checks = [
            {"name": "the first new tab starts the lab browser through its launcher (about:blank)",
             "value": [l[0]["argv"][-1] for l in launched], "pass": bool(first) and first.get("count") == 1
             and bool(launched) and launched[0][0]["argv"][-1] == "about:blank"},
            {"name": "\"open a new tab\" raises tabs.json's count by one", "value": [first, second],
             "pass": bool(first) and bool(second) and second.get("count") == first.get("count") + 1},
            {"name": "every browser start is the catalogue argv with about:blank (the second hands its tab to the first)",
             "value": {"seen": [l[0]["argv"] for l in launched], "requests": self.tab_requests()},
             "pass": 1 <= len(launched) <= 2 and all(l[0]["argv"][-1] == "about:blank" for l in launched)
             and [q.get("action") for q in self.tab_requests()][:2] == ["start", "new_tab"]},
            {"name": f"next tab and close the tab say {cant!r}", "value": [n["lines"], c["lines"]],
             "pass": cant in n["lines"] and cant in c["lines"]},
            {"name": "and change nothing in the browser", "value": [before, after],
             "pass": bool(before) and bool(after) and before.get("count") == after.get("count")
             and before.get("active") == after.get("active")},
            {"name": "no input program ever ran (no key chord)", "value": inputs, "pass": not inputs},
        ]
        self.close_apps()
        return {"utterances": [a, b, n, c], "checks": checks}, self.checks_pass(checks)

    def _wait_tabs(self, pred, timeout):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            s = self.tabs()
            if pred(s):
                return s
            time.sleep(0.2)
        return self.tabs()

    # --- step 4: window verbs ---

    def wm13(self):
        self.rset(reflex=True)
        self.close_apps()
        self.cooldown()
        self.utter("open_notes")
        self.wait_window(NOTES_APP, 15)
        focused = False
        ws = self.windows(NOTES_APP)
        if ws:
            self.sh("phase14.sh", "focus", ws[0]["id"])
            deadline = time.monotonic() + 5
            while time.monotonic() < deadline:
                ws = self.windows(NOTES_APP)
                if ws and ws[0].get("focused"):
                    focused = True
                    break
                time.sleep(0.1)
        g0 = ws[0] if ws else None
        said, rows = {}, {}
        for name in ("snap_left", "snap_right", "minimize", "maximize"):
            r0 = self.rlog() or []
            said[name] = self.utter(name)
            time.sleep(1.0)
            rows[name] = self.rows_since(r0)
        g1 = (self.windows(NOTES_APP) or [None])[0]
        cant = self.line("LineCantHere")
        expect_done = {"maximize"} if self.wm == "niri" else set()
        checks = [{"name": "the notes window is open and focused for the verbs", "value": g0,
                   "pass": bool(g0) and focused}]
        for name in ("snap_left", "snap_right", "minimize", "maximize"):
            intent = name
            done = name in expect_done
            want_outcome = "done" if done else "unsupported"
            outs = [r.get("outcome") for r in rows[name] if r.get("intent") == intent]
            checks.append({"name": f"{name}: " + ("done" if done else f"says {cant!r}, an unsupported row"),
                           "value": {"lines": said[name]["lines"], "rows": rows[name]},
                           "pass": outs == [want_outcome] and (done or cant in said[name]["lines"])})
        if self.wm == "niri":
            checks.append({"name": "niri: maximize widens the focused column (compositor geometry)", "value": [g0, g1],
                           "pass": bool(g0) and bool(g1) and g1["size"][0] > g0["size"][0]})
        else:
            checks.append({"name": "Hyprland: no verb moved the window", "value": [g0, g1],
                           "pass": bool(g0) and bool(g1) and g0.get("pos") == g1.get("pos") and g0.get("size") == g1.get("size")})
        self.limit(4, "window verbs", "snap needs the Ryoku seam (windowFloat, focusHistory), which the lab's refusal "
                                      "stub withholds; niri offers maximize only, Hyprland none, minimize nowhere")
        self.close_apps()
        return {"said": said, "rows": rows, "checks": checks}, self.checks_pass(checks)

    # --- step 5: audio ---

    def audio13(self):
        self.rset(reflex=True)
        step = 5
        g0 = self.graph()
        v0, m0 = sink_level(g0, "oni_lab")
        said = {}
        said["down"] = self.utter("vol_down")
        v1, _ = sink_level(self.graph(), "oni_lab")
        said["up"] = self.utter("vol_up")
        v2, _ = sink_level(self.graph(), "oni_lab")
        said["full"] = self.utter("vol_up")
        v3, _ = sink_level(self.graph(), "oni_lab")
        said["mute"] = self.utter("mute")
        _, m1 = sink_level(self.graph(), "oni_lab")
        said["unmute"] = self.utter("unmute")
        _, m2 = sink_level(self.graph(), "oni_lab")
        p0 = [e for e in self.mpris() if e.get("member") == "PlayPause"]
        said["pause"] = self.utter("pause")
        time.sleep(1.0)
        p1 = [e for e in self.mpris() if e.get("member") == "PlayPause"]
        mods = json.loads(self.sh("audio.sh", "modules").stdout)
        mic = voicekit.mic_check(self.graph(), mods, str(self.lab))
        # a hidden Oni announces through the notification server
        n0 = len(self.notices())
        self.settings(hidden=True)
        try:
            said["hidden"] = self.utter("vol_down")
            time.sleep(1.5)
            n1 = self.notices()[n0:]
        finally:
            self.settings(hidden=False)
        self.utter("vol_up")
        full = self.line("LineVolumeFull")
        checks = [
            {"name": "volume down lowers the lab sink by the 5 % step", "value": [v0, v1],
             "pass": v0 is not None and v1 == max(0, v0 - 5)},
            {"name": "volume up raises it by the step", "value": [v1, v2], "pass": v1 is not None and v2 == min(100, v1 + 5)},
            {"name": f"at 100 % volume up says {full!r} and changes nothing",
             "value": {"level": [v2, v3], "lines": (said["full"] or {}).get("lines")},
             "pass": v2 == 100 and v3 == 100 and full in said["full"]["lines"]},
            {"name": "mute and unmute set the lab sink's mute", "value": [m0, m1, m2], "pass": m1 is True and m2 is False},
            {"name": "\"pause the music\" is one MPRIS PlayPause, Playing to Paused", "value": p1[len(p0):],
             "pass": len(p1) - len(p0) == 1 and p1[-1].get("before") == "Playing" and p1[-1].get("after") == "Paused"},
            {"name": "the lab graph still has no device, foreign default or module", "value": mic.get("problems"),
             "pass": not mic.get("problems")},
            {"name": "a hidden Oni still announces: a notification on the lab's server carrying its line", "value": n1,
             "pass": any(n.get("summary") == CHARACTER_NAME and n.get("body") in said["hidden"]["lines"]
                         for n in n1)},
        ]
        self.limit(step, "audio targets", "the sink is the lab's null sink and the player mprisfix.py; no real device "
                                          "or player is reached")
        return {"said": said, "checks": checks}, self.checks_pass(checks)

    # --- step 6: timers and reminders ---

    def fired(self, mark, word, timeout):
        """The first bubble or spoken line after MARK holding WORD: (wall, text)."""
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            for w, t in self.bubbles(mark):
                if word.lower() in (t or "").lower():
                    return w, t
            for w, d in self.voice_since(mark):
                if d.get("phase") == "speaking" and word.lower() in (d.get("text") or "").lower():
                    return w, d.get("text")
            time.sleep(0.1)
        return None, None

    def timers13(self):
        checks, said = [], {}
        # a timer rings on voice alone, reflex off
        self.rset(reflex=False, reminders=False)
        # a bubble needs a strand: Oni perched on window.sh's notes fixture
        self.perch_fixture()
        rec = Rec(self, "timer")
        lm = self.ledger.mark()
        said["timer"] = self.utter("timer5")
        fire_w, fire_t = self.fired(said["timer"]["mark"], "Time's up", 20)
        png, stats = None, None
        bubble_line = self.ledger.wait(lambda l: l.get("ev") == "bubble", lm, 5)
        try:
            c = self.current()
            out = self.output(c.get("output"))
            if out:
                x, y, w, h = c["rect"]
                png = self.shoot("timer-bubble", [max(out["x"], out["x"] + x - 260), max(out["y"], out["y"] + y - 200), w + 520, h + 400])
                stats = labkit.image_stats(png)
        except (RuntimeError, OSError, KeyError, ValueError):
            pass
        time.sleep(1.5)
        rec.stop()
        after = rec.to_rec(fire_w) if fire_w else None
        segs = None if after is None else rec.segments(after=after - 0.5)
        checks += [
            {"name": "with reflex off a five second timer is still set (voice alone)", "value": said["timer"]["lines"],
             "pass": bool(said["timer"]["lines"])},
            {"name": "it rings: a \"Time's up.\" bubble or line after about five seconds",
             "value": {"text": fire_t, "after_s": None if not fire_w else round(fire_w - said["timer"]["up"], 2)},
             "pass": fire_w is not None and 4.0 <= fire_w - said["timer"]["up"] <= 15.0},
            {"name": "the surface drew it: a ledger bubble line and a non-blank crop", "value": {"bubble": bubble_line, "crop": str(png) if png else None,
                                                                                            "stats": stats},
             "pass": bubble_line is not None and bool(stats) and bool(stats.get("nonblank", True))},
            {"name": "and said it: sound in the recorded PCM after the fire", "value": segs[:4] if segs else segs,
             "pass": bool(segs)},
        ]
        # reminders need the switch
        self.rset(reflex=True, reminders=False)
        said["off"] = self.utter("remind10")
        rc_off, _, _ = self.reminders()
        off_line = "Reminders are off."
        checks.append({"name": "with the reminders switch off a reminder is refused and nothing kept",
                       "value": {"lines": said["off"]["lines"], "cli_rc": rc_off},
                       "pass": any(off_line in (l or "") for l in said["off"]["lines"]) and rc_off != 0})
        self.rset(reminders=True)
        said["remind"] = self.utter("remind10")
        rw, rt = self.fired(said["remind"]["mark"], "stretch", 25)
        checks.append({"name": "\"remind me in ten seconds to stretch\" rings with its words",
                       "value": {"confirm": said["remind"]["lines"], "fired": rt,
                                 "after_s": None if not rw else round(rw - said["remind"]["up"], 2)},
                       "pass": rw is not None and 8.0 <= rw - said["remind"]["up"] <= 30.0})
        n0 = self.requests_n()
        _, l0, _ = self.reminders()
        said["bare"] = self.utter("remind_bare")
        time.sleep(1.5)
        _, l1, _ = self.reminders()
        checks.append({"name": "bare \"remind me\" is not claimed: an ordinary Ask, nothing kept",
                       "value": {"requests": self.requests_n() - n0, "kept": len(l1.get("reminders") or [])},
                       "pass": self.requests_n() - n0 == 1 and len(l1.get("reminders") or []) == len(l0.get("reminders") or [])})
        said["timer2m"] = self.utter("timer2m")
        said["list"] = self.utter("list")
        _, lt, _ = self.reminders()
        timers = [r for r in lt.get("reminders") or [] if r.get("kind") == "timer"]
        mode = file_mode(self.reminders_path)
        try:
            data = self.reminders_path.read_bytes()
        except OSError:
            data = b""
        words = [w for w in ("remind me", "two minutes", "set a timer") if w.encode() in data.lower()]
        said["cancel"] = self.utter("cancel_timer")
        _, lc, _ = self.reminders()
        said["8d"] = self.utter("remind8d")
        _, l8, _ = self.reminders()
        checks += [
            {"name": "a two minute timer is kept and listed by voice", "value": {"cli": timers, "said": said["list"]["lines"]},
             "pass": len(timers) == 1 and bool(said["list"]["lines"])},
            {"name": "the reminders file is 0600 and holds no transcript", "value": {"mode": oct(mode) if mode else None,
                                                                                  "transcript": words},
             "pass": mode == 0o600 and not words},
            {"name": "\"cancel the timer\" takes it away", "value": {"lines": said["cancel"]["lines"],
                                                                   "left": lc.get("reminders")},
             "pass": "Timer cancelled." in said["cancel"]["lines"]
             and not [r for r in lc.get("reminders") or [] if r.get("kind") == "timer"]},
            {"name": "eight days is refused with a line and nothing kept",
             "value": {"lines": said["8d"]["lines"], "kept": l8.get("reminders")},
             "pass": bool(said["8d"]["lines"]) and len(l8.get("reminders") or []) == len(lc.get("reminders") or [])},
        ]
        # one due while the daemon was down rings once, late, then is done.
        # The restarted daemon fires it while starting, before the paused
        # watch is back (and a surface only redials when touched), so Oni is
        # hidden for this one: the fire is then also a desktop notification,
        # which notifyfix.py records whatever is subscribed (remindShow)
        said["late"] = self.utter("remind10")
        n_before = len(self.notices())
        m_before = self.watch.mark()
        self.settings(hidden=True)
        try:
            self.close_ui()
            self.watch.pause()
            try:
                self.daemon_stop()
                # a fire is late only past busyTries x busyWaitS + 1 min after due
                r = self.rspec["reminders"]
                time.sleep(10 + r["busyTries"] * r["busyWaitS"] + 60 + 20)
                self.daemon_start()
            finally:
                self.watch.resume()
            lw, lt_ = self.fired(m_before, "late", 25)
            time.sleep(8.0)
            late_notes = [n for n in self.notices()[n_before:] if "late" in (n.get("summary") or "").lower()]
            rings = len([t for _, t in self.bubbles(m_before) if "stretch" in (t or "").lower()]) + len(
                [n for n in self.notices()[n_before:] if "stretch" in (n.get("body") or "").lower()])
            _, ll, _ = self.reminders()
        finally:
            self.open_ui()
            self.settings(hidden=False)
        checks += [
            {"name": "one that fell due while the daemon was down rings after the restart, marked late",
             "value": {"frame": lt_, "notifications": late_notes}, "pass": lw is not None or bool(late_notes)},
            {"name": "it rings once (bubble and notification together count once each)", "value": rings,
             "pass": 1 <= rings <= 2},
            {"name": "then it is done: not listed", "value": ll.get("reminders"),
             "pass": not [r for r in ll.get("reminders") or [] if "stretch" in (r.get("text") or "")]},
        ]
        self.ensure_detached()
        return {"said": said, "recording": str(rec.wav), "checks": checks}, self.checks_pass(checks)

    # --- step 7: zero at rest, both inits ---

    def no_daemon_for(self, seconds):
        deadline = time.monotonic() + seconds
        while time.monotonic() < deadline:
            if self.daemon_pids():
                return False
            time.sleep(0.2)
        return True

    def rest13(self):
        checks, notes = [], {}
        self.rset(reflex=True, reminders=True)
        self.vset(openMic=False, wake={"enabled": False, "phrase": WAKE_PHRASE})
        self.call("settings.set", {"patch": {"daemon": {"idleExitMinutes": 1}}})
        self.jcli("oni", "reminders", "--all", "--json")
        try:
            self.close_ui()
            # a: the wake unit refused (the lab's stubs): the daemon stays up
            r0 = len(self.stubbed())
            notes["refused_set"] = self.utter("remind40")
            time.sleep(1.5)
            rc, lst, err = self.reminders()
            asked = [a for a in self.stubbed()[r0:] if wake_request(a) == "enable"]
            checks += [
                {"name": "a reminder 40 minutes off is kept", "value": lst.get("reminders"),
                 "pass": rc == 0 and len(lst.get("reminders") or []) == 1},
                {"name": "the daemon asked for the wake unit through the bridge (logged by the refusal stub)",
                 "value": asked, "pass": bool(asked)},
                {"name": "refused, oni reminders reports noWake and the CLI says Oni stays running",
                 "value": {"noWake": lst.get("noWake"), "stderr": err[-300:]}, "pass": lst.get("noWake") is True},
            ]
            self.watch.pause()
            try:
                pid0 = self.daemon_pids()
                stayed = not self.wait_daemon(False, 100)
            finally:
                self.watch.resume()
            checks.append({"name": "with no wake unit the daemon stays up past the one-minute idle exit while a reminder is pending",
                           "value": {"before": pid0, "after": self.daemon_pids()}, "pass": bool(pid0) and stayed})
            # b: the accepting stub: enable and disable are logged, the daemon idles out
            self.sh("phase13.sh", "wakestub", "accept")
            w1 = len(self.stubbed("wake"))
            self.jcli("oni", "reminders", "--all", "--json")
            time.sleep(1.0)
            notes["accepted_set"] = self.utter("remind40")
            time.sleep(1.5)
            _, lst2, _ = self.reminders()
            wakes = self.stubbed("wake")[w1:]
            checks += [
                {"name": "the accepting stub logs the enable when a reminder is kept", "value": wakes,
                 "pass": [wake_request(a) for a in wakes][-1:] == ["enable"]},
                {"name": "armed: noWake is absent or false", "value": lst2.get("noWake"), "pass": not lst2.get("noWake")},
            ]
            self.watch.pause()
            try:
                exited = self.wait_daemon(False, 120)
                stayed_down = exited and self.no_daemon_for(20)
                checks.append({"name": "a reminder past the 35 minute residency horizon lets the daemon idle out, and it stays down",
                               "value": {"exited": exited, "stayed_down": stayed_down}, "pass": stayed_down})
                rw = self.cli("routines", "wake", stdin="", timeout=60)
                far = self.no_daemon_for(4)
                checks.append({"name": "hyakki routines wake does not start it for a reminder 40 minutes off",
                               "value": rw.returncode, "pass": rw.returncode == 0 and far})
            finally:
                self.watch.resume()
            # a reminder due within the 30 minute horizon, the daemon down
            # Watch may reconnect lazily after an idle exit. Start a known
            # live daemon and presenter before sending another microphone turn.
            self.restart()
            self.wake()
            notes["near_set"] = self.utter("remind20")
            rc_near, near, _ = self.reminders()
            decoder_idle = self.drain_file_decoders()
            near_rows = [r for r in near.get("reminders") or []
                         if r.get("kind") == "reminder" and "drink water" in (r.get("text") or "").lower()
                         and 0 < (at_seconds(r.get("due")) or 0) - time.time()
                         <= self.rspec["reminders"]["wakeHorizonMin"] * 60]
            prepared = bool(notes["near_set"]["heard"]) and rc_near == 0 and bool(near_rows) and decoder_idle["idle"]
            checks.append({"name": "the real near reminder is transcribed, persisted and decoded before shutdown",
                           "value": {"utterance": notes["near_set"], "reminders": near_rows, "decoder": decoder_idle},
                           "pass": prepared})
            if not prepared:
                raise RuntimeError("near-reminder PCM did not produce a completed, persisted reminder: "
                                   + json.dumps({"utterance": notes["near_set"], "result": near,
                                                 "decoder": decoder_idle}))
            self.watch.pause()
            try:
                self.daemon_stop()
                down = self.wait_daemon(False, 15)
                rw = self.cli("routines", "wake", stdin="", timeout=60)
                woke = self.wait_daemon(True, 10)
                checks.append({"name": "hyakki routines wake starts it for a reminder due within the horizon",
                               "value": {"down": down, "rc": rw.returncode, "up": woke}, "pass": down and rw.returncode == 0 and woke})
            finally:
                self.watch.resume()
            # not with reflex off
            self.rset(reflex=False)
            self.watch.pause()
            try:
                self.daemon_stop()
                self.wait_daemon(False, 15)
                rw = self.cli("routines", "wake", stdin="", timeout=60)
                quiet = self.no_daemon_for(4)
                checks.append({"name": "not with reflex off", "value": rw.returncode, "pass": rw.returncode == 0 and quiet})
            finally:
                self.watch.resume()
            self.rset(reflex=True)
            # not with Hyakki off (the switch only the app and hyakki on|off set)
            self.watch.pause()
            try:
                self.daemon_stop()
                self.wait_daemon(False, 15)
                self.write_config({"enabled": False})
                rw = self.cli("routines", "wake", stdin="", timeout=60)
                quiet = self.no_daemon_for(4)
                checks.append({"name": "not with Hyakki off", "value": rw.returncode, "pass": rw.returncode == 0 and quiet})
            finally:
                self.daemon_stop()
                self.write_config({"enabled": True})
                self.watch.resume()
            # the runit worker runs the same command
            self.watch.pause()
            try:
                self.daemon_stop()
                self.wait_daemon(False, 15)
                self.sh("phase13.sh", "worker", "start")
                woke = self.wait_daemon(True, 80)
                self.sh("phase13.sh", "worker", "stop")
                checks.append({"name": "the runit worker (hyakki service routines) starts it through the same wake command",
                               "value": woke, "pass": woke})
            finally:
                self.watch.resume()
            self.daemon_start()
            # clearing the last reminder lets the unit go
            w2 = len(self.stubbed("wake"))
            self.jcli("oni", "reminders", "--all", "--json")
            time.sleep(1.5)
            gone = self.stubbed("wake")[w2:]
            checks.append({"name": "clearing the last reminder logs the disable", "value": gone,
                           "pass": "disable" in [wake_request(a) for a in gone]})
            # hyakki off reaches only the stubs
            r3 = len(self.stubbed())
            self.watch.pause()
            try:
                off = self.cli("off", stdin="", timeout=60)
                down = self.wait_daemon(False, 15)
                stubbed = self.stubbed()[r3:] + self.stubbed("wake")[w2:]
                enabled = self.cfg().get("enabled")
            finally:
                self.watch.resume()
            self.watch.pause()
            try:
                self.daemon_stop()
                self.write_config({"enabled": True})
                self.daemon_start()
            finally:
                self.watch.resume()
            checks.append({"name": "hyakki off: Hyakki off, the daemon down, and only the lab's stubs were asked",
                           "value": {"rc": off.returncode, "enabled": enabled, "stubs": stubbed},
                           "pass": enabled is False and down})
        finally:
            self.sh("phase13.sh", "wakestub", "refuse", check=False)
            self.sh("phase13.sh", "worker", "stop", check=False)
            if self.cfg().get("enabled") is not True:
                self.watch.pause()
                try:
                    self.daemon_stop()
                    self.write_config({"enabled": True})
                finally:
                    self.watch.resume()
            if not self.daemon_pids():
                self.daemon_start()
            self.call("settings.set", {"patch": {"daemon": {"idleExitMinutes": 5}}})
            self.jcli("oni", "reminders", "--all", "--json")
            self.open_ui()
        self.limit(7, "wake unit", "the host has /run/systemd/system and the lab's bin has ryoku-host, so the "
                                   "bridge's systemd form is the one exercised; the bare systemctl and runit-bridge "
                                   "forms are the teams package's unit tests")
        return {"said": notes, "checks": checks}, self.checks_pass(checks)

    # --- step 8: storm ---

    def storm13(self):
        checks, said = [], {}
        self.rset(reflex=True)
        self.close_apps()
        self.cooldown()
        from gi.repository import Gio
        address = busfix.lab_address({"DBUS_SESSION_BUS_ADDRESS": "unix:path=" + str(self.lab / "rt/bus")})
        bus = busfix.connect(address)
        try:
            bus.call_sync("org.mpris.MediaPlayer2.onilab", "/org/mpris/MediaPlayer2",
                          "org.mpris.MediaPlayer2.Player", "Play", None, None,
                          Gio.DBusCallFlags.NONE, 5000, None)
        finally:
            bus.close_sync(None)
        media_before = self.mpris()[-1]
        checks.append({"name": "the private media fixture starts Playing so both pause and play act",
                       "value": media_before, "pass": media_before.get("after") == "Playing"})
        cap = self.rspec["maxActions"]
        t0 = time.time()
        r0 = self.rlog() or []
        said["storm"] = self.utter("storm", timeout=25)
        time.sleep(2.0)
        storm_launch = self.launches("notes", t0)
        rows = self.rows_since(r0)
        checks.append({"name": f"the {STORM_REPEATS}-repeat utterance launches at most once",
                       "value": {"heard": (said["storm"]["heard"] or "")[:200], "launches": len(storm_launch), "rows": rows},
                       "pass": len(storm_launch) <= 1 and len([r for r in rows if r.get("outcome") == "done"]) <= cap})
        self.close_apps()
        r1 = self.rlog() or []
        said["ten"] = self.utter("ten", timeout=20)
        time.sleep(2.0)
        ten_rows = self.rows_since(r1)
        done = [r for r in ten_rows if r.get("outcome") == "done"]
        heard_clauses = [part for part in re.split(r"[,;.!?]+", said["ten"]["heard"] or "") if part.strip()]
        checks.append({"name": f"ten recognized clauses reach, but cannot exceed, the {cap}-action turn limit",
                       "value": {"heard": said["ten"]["heard"], "clauses": len(heard_clauses), "rows": ten_rows},
                       "pass": len(heard_clauses) == 10 and len(done) == cap})
        # Reuse the recognized corpus so only the inter-turn budget changes.
        per_min = self.rspec["maxActionsPerMinute"]
        for k in range(3):
            said[f"burst{k}"] = self.utter("ten", timeout=20)
        rows_min = self.rows_since(r0)
        times = [at_seconds(r.get("at")) for r in rows_min if r.get("outcome") == "done"]
        enough = self.line("lineEnough")
        lines_min = [line for k in range(3) for line in said[f"burst{k}"]["lines"]]
        checks.append({"name": f"the {per_min}-action minute limit is reached and Oni refuses further actions aloud",
                       "value": {"busiest_minute": max_in_window(times), "lines": lines_min},
                       "pass": max_in_window(times) == per_min
                       and (enough in lines_min or self.line("lineReflexEnough") in lines_min)})
        time.sleep(61.0)
        # one utterance naming the app twice: the duplicate is dropped
        t1 = time.time()
        r2 = self.rlog() or []
        said["twice"] = self.utter("twice")
        time.sleep(1.5)
        tw = self.launches("notes", t1)
        rows2 = self.rows_since(r2)
        checks.append({"name": "\"open X and open X\" in one utterance starts it once, one done row",
                       "value": {"launches": len(tw), "rows": rows2, "lines": said["twice"]["lines"]},
                       "pass": len(tw) == 1 and len([r for r in rows2 if r.get("intent") == "open_app"
                                                     and r.get("outcome") == "done"]) == 1})
        # a second launch inside the cooldown: two presses back to back; the
        # session holds the second back before the executor (no row)
        self.close_apps()
        self.cooldown()
        t1b = time.time()
        r2b = self.rlog() or []
        first = self.rhold("open_notes")
        # a press while the first turn is still thinking cancels it (the new
        # press is the person's newer intent), so the second press waits for
        # the first plan's line; the cooldown runs from final transcript to
        # final transcript, so the second hold is kept short
        self.wait_voice(first["mark"], lambda d: d.get("phase") == "speaking", 10.0, None)
        second = self.rhold("open_again", lead=0.1, tail=0.25)
        self.final_of(first)
        self.final_of(second)
        self.settle_voice(first["mark"], 15)
        time.sleep(1.0)
        lines2 = self.spoken(first["mark"])
        tw2 = self.launches("notes", t1b)
        rows2b = self.rows_since(r2b)
        already = self.line("lineAlready")
        first_final, second_final = first.get("final"), second.get("final")
        final_gap = second_final[0] - first_final[0] if first_final and second_final else None
        said["again"] = {"lines": lines2, "release_gap_s": round(second["up"] - first["up"], 2),
                         "final_gap_s": round(final_gap, 3) if final_gap is not None else None,
                         "first_final": first_final, "second_final": second_final}
        checks.append({"name": f"a second recognized launch inside {self.rspec['launchCooldownS']} s is held back with {already!r}, one start",
                       "value": {"launches": len(tw2), "rows": rows2b, **said["again"]},
                       "pass": final_gap is not None and 0 < final_gap < self.rspec["launchCooldownS"]
                       and len(tw2) == 1 and already in lines2
                       and len([r for r in rows2b if r.get("intent") == "open_app"]) == 1})
        # a dangling verb: completes within 6 s, not after
        self.close_apps()
        self.cooldown()
        t2 = time.time()
        said["open"] = self.utter("open_only", timeout=0)
        said["object"] = self.utter("notes_only")
        time.sleep(1.5)
        quick = self.launches("notes", t2)
        gap = (said["object"]["final_at"] - said["open"]["final_at"]
               if said["object"]["final_at"] is not None and said["open"]["final_at"] is not None else None)
        self.close_apps()
        self.cooldown()
        t3 = time.time()
        said["open2"] = self.utter("open_only", timeout=4)
        if said["open2"]["final_at"] is None:
            raise RuntimeError("the dangling-verb expiry proof received no final verb transcript")
        wait_s = self.rspec["danglingVerbS"] + 2.0 - (time.time() - said["open2"]["final_at"])
        if wait_s > 0:
            time.sleep(wait_s)
        said["object2"] = self.utter("notes_only")
        time.sleep(1.5)
        late = self.launches("notes", t3)
        gap2 = (said["object2"]["final_at"] - said["open2"]["final_at"]
                if said["object2"]["final_at"] is not None else None)
        checks += [
            {"name": "a dangling \"open\" completes with the next words inside 6 s", "value": {"gap_s": gap, "launches": len(quick)},
             "pass": gap is not None and 0 < gap <= self.rspec["danglingVerbS"] and len(quick) == 1},
            {"name": "and not after 6 s", "value": {"gap_s": gap2, "launches": len(late)},
             "pass": gap2 is not None and gap2 > self.rspec["danglingVerbS"] and not late},
        ]
        self.close_apps()
        # careful words and a joined search
        t4 = time.time()
        r4 = self.rlog() or []
        said["careful"] = self.utter("careful")
        time.sleep(1.5)
        cr = self.rows_since(r4)
        checks.append({"name": "\"I insert a table\" opens nothing and runs nothing",
                       "value": {"rows": cr, "launches": [l[0]["argv"] for l in self.launches(since=t4)]},
                       "pass": not [r for r in cr if r.get("outcome") == "done"] and not self.launches(since=t4)})
        self.cooldown()
        t5 = time.time()
        said["salt"] = self.utter("salt")
        time.sleep(2.0)
        searches = [l for l in self.launches("browser", t5) if l[0]["argv"][-1] != "about:blank"]
        queries = [query_of(l[0]["argv"][-1]) for l in searches]
        checks.append({"name": "\"search for salt and pepper\" is one search for the whole phrase",
                       "value": queries, "pass": queries == ["salt and pepper"]})
        self.close_apps()
        return {"said": said, "checks": checks}, self.checks_pass(checks)

    # --- step 9: forbidden ---

    def run_test(self, pkg, rel, run):
        binary = os.environ.get(f"ONI_LAB_{pkg.upper()}_TEST") or str(Path(self.env.get("HYAKKI_BIN", "/")) / f"{pkg}.test")
        if not os.access(binary, os.X_OK):
            return {"ran": False, "pass": False, "error": f"no {pkg}.test (go test -c of {rel}) in HYAKKI_BIN"}
        r = subprocess.run(["systemd-run", "--user", "--scope", "--quiet", "--collect",
                            f"--unit=oni-lab-{self.env['LAB_ID']}-{pkg}test-{os.urandom(4).hex()}",
                            "-p", "MemoryMax=1G", "-p", "MemorySwapMax=0", "--",
                            "env", "-i", "PATH=/usr/bin:/bin", binary, "-test.run", run, "-test.v", "-test.count=1"],
                           capture_output=True, text=True, timeout=600, cwd=APP / rel)
        outp = (r.stdout + r.stderr).strip()
        return {"ran": True, "rc": r.returncode, "output": outp[-1500:],
                "pass": r.returncode == 0 and "--- PASS" in outp and "--- FAIL" not in outp}

    def forbidden13(self):
        checks, said = [], {}
        self.rset(reflex=True)
        for name in FORBIDDEN:
            t0 = time.time()
            r0 = self.rlog() or []
            n0 = self.requests_n()
            said[name] = self.utter(name)
            time.sleep(1.5)
            rows = self.rows_since(r0)
            launched = self.launches(since=t0)
            inputs = self.input_programs(t0)
            checks.append({"name": f"\"{self.rfx['fixtures'][name]['text']}\": no action, launch or key; an ordinary Ask",
                           "value": {"heard": said[name]["heard"], "rows": rows, "launches": [l[0]["argv"] for l in launched],
                                     "inputs": inputs, "requests": self.requests_n() - n0},
                           "pass": not [r for r in rows if r.get("outcome") == "done"] and not launched and not inputs
                           and self.requests_n() - n0 == 1})
        test = self.run_test("reflex", "internal/oni/reflex", REGISTRY_RUN)
        checks.append({"name": "reflex.test: the registry invariant (no send, buy or delete intent)", "value": test,
                       "pass": bool(test.get("pass"))})
        return {"said": said, "checks": checks}, self.checks_pass(checks)

    # --- step 10: routines ---

    def offer_after(self, mark, timeout=20):
        w, d, _ = self.watch.wait("consent", lambda d: d.get("feature") == "reflex.routines", mark, timeout, None)
        return w, d

    def three_steps(self):
        # Kill old catalogue fixtures before the three-command sequence.
        # Waiting for stale windows here used to exceed routines.gapS.
        self.close_apps()
        self.cooldown()
        out = [self.utter("vol_down"), self.utter("vol_up"), self.utter("open_notes")]
        return out

    def routines13(self):
        checks, said = [], {}
        chip = self.rspec["routines"]["chipS"]
        self.rset(reflex=True, routines=True)
        self.jcli("oni", "routines", "--all", "--json")
        texts = [self.rfx["fixtures"][n]["text"].lower().rstrip(".") for n in ("vol_down", "vol_up", "open_notes")]
        # A: open-mic "yes" saves nothing
        self.new_run()
        m = self.watch.mark()
        said["A"] = self.three_steps()
        wa, offer_a = self.offer_after(m)
        said["A_yes"] = self.open_say("yes", timeout=6)
        time.sleep(max(0.0, chip + 2 - (time.time() - (wa or time.time()))))
        checks.append({"name": "three commands within 20 s bring one routine offer (per-action, local)", "value": offer_a,
                       "pass": bool(offer_a) and offer_a.get("class") and str(offer_a.get("id", "")).startswith("r-")})
        checks.append({"name": "open-mic \"yes\" saves nothing", "value": {"heard": said["A_yes"]["heard"], "routines": self.routines()},
                       "pass": bool(offer_a) and (said["A_yes"]["heard"] or "").strip().rstrip(".!?").casefold() == "yes"
                       and not self.routines() and not self.routines_path.exists()})
        # B: wake "yes" saves nothing
        self.new_run()
        m = self.watch.mark()
        said["B"] = self.three_steps()
        wb, offer_b = self.offer_after(m)
        said["B_yes"] = self.wake_say("yes", timeout=6)
        time.sleep(max(0.0, chip + 2 - (time.time() - (wb or time.time()))))
        checks.append({"name": "wake \"yes\" saves nothing", "value": {"offer": bool(offer_b), "heard": said["B_yes"]["heard"],
                                                                        "routines": self.routines()},
                       "pass": bool(offer_b) and (said["B_yes"]["heard"] or "").strip().rstrip(".!?").casefold() == "yes"
                       and not self.routines()})
        # C: the offer lapses
        self.new_run()
        m = self.watch.mark()
        said["C"] = self.three_steps()
        wc, offer_c = self.offer_after(m)
        time.sleep(chip + 3)
        checks.append({"name": f"left alone, the offer lapses after {chip} s and saves nothing",
                       "value": {"offer": bool(offer_c), "routines": self.routines()},
                       "pass": bool(offer_c) and not self.routines()})
        # D: a pressed "yes" saves
        self.new_run()
        m = self.watch.mark()
        said["D"] = self.three_steps()
        wd, offer_d = self.offer_after(m)
        said["D_yes"] = self.utter("yes")
        time.sleep(1.0)
        saved = self.routines() or []
        mode = file_mode(self.routines_path)
        try:
            data = self.routines_path.read_text().lower()
        except OSError:
            data = ""
        spoken = [t for t in texts if t in data]
        m2 = self.watch.mark()
        said["D_more"] = self.three_steps()
        time.sleep(3.0)
        again = [d for _, d in self.watch.frames("consent", m2) if d.get("feature") == "reflex.routines"]
        checks += [
            {"name": "a pressed \"yes\" saves one routine of three steps", "value": {"offer": bool(offer_d), "routines": saved},
             "pass": bool(offer_d) and len(saved) == 1 and saved[0].get("steps") == 3},
            {"name": "the routines file is 0600, within 20 x 8, and holds no spoken words",
             "value": {"mode": oct(mode) if mode else None, "spoken": spoken},
             "pass": mode == 0o600 and not spoken and len(saved) <= self.rspec["routines"]["maxRoutines"]
             and all((r.get("steps") or 0) <= self.rspec["routines"]["maxSteps"] for r in saved)},
            {"name": "no second offer in the run", "value": again, "pass": not again},
        ]
        # replay: model-free and decision-free; the per-minute cap counts
        # voice and replay together, so the minute of recorded steps passes first
        self.close_apps()
        time.sleep(61.0)
        n0, d0 = self.requests_n(), len(self.decisions())
        r0 = self.rlog() or []
        t0 = time.time()
        said["run"] = self.utter("run_routine")
        time.sleep(3.0)
        rows = self.rows_since(r0)
        replayed = self.launches("notes", t0)
        steps = [r for r in rows if r.get("intent") != "run_routine"]
        checks.append({"name": "\"run my routine\" replays the three steps with zero model and decision requests",
                       "value": {"rows": rows, "launches": len(replayed), "model": self.requests_n() - n0,
                                 "decisions": len(self.decisions()) - d0},
                       "pass": [r.get("outcome") for r in rows if r.get("intent") == "run_routine"] == ["done"]
                       and len([r for r in steps if r.get("outcome") == "done"]) == 3 and len(replayed) == 1
                       and self.requests_n() == n0 and len(self.decisions()) == d0})
        # a removed app is skipped with its line
        self.close_apps()
        self.sh("phase13.sh", "app", "remove")
        try:
            self.restart()
            self.cooldown()
            r1 = self.rlog() or []
            t1 = time.time()
            said["skip"] = self.utter("run_routine")
            time.sleep(3.0)
            rows1 = self.rows_since(r1)
            gone = self.launches("notes", t1)
        finally:
            self.sh("phase13.sh", "app", "install")
            self.restart()
        checks.append({"name": "with the app removed, replay skips it, says so and logs a skipped row",
                       "value": {"lines": said["skip"]["lines"], "rows": rows1, "launches": len(gone)},
                       "pass": not gone and "skipped" in [r.get("outcome") for r in rows1]
                       and any("Skipped a missing app." in (l or "") for l in said["skip"]["lines"])})
        self.close_apps()
        return {"said": said, "checks": checks}, self.checks_pass(checks)

    # --- step 11: notes ---

    def notes13(self):
        checks, said = [], {}
        t_step = time.time()
        self.reconfigure({"memory": {"asked": True, "use": True}})
        self.rset(reflex=True, notes=True)
        said["note"] = self.utter("note")
        time.sleep(1.0)
        ml = self.cli("memory", "list", "--format", "json", stdin="", timeout=60)
        said["secret"] = self.utter("note_secret")
        time.sleep(1.0)
        ml2 = self.cli("memory", "list", "--format", "json", stdin="", timeout=60)
        kept, refused_line = self.line("lineNoteKept"), self.line("lineNoteRefused")
        checks += [
            {"name": f"memory on: the note is kept ({kept!r}) and memory lists it",
             "value": {"lines": said["note"]["lines"], "listed": "exam" in ml.stdout.lower()},
             "pass": kept in said["note"]["lines"] and "exam" in ml.stdout.lower()},
            {"name": "a password is refused with the research copy and never reaches memory",
             "value": {"lines": said["secret"]["lines"], "in_memory": SECRET in ml2.stdout.lower()},
             "pass": any(refused_line.split(".")[0] in (l or "") for l in said["secret"]["lines"])
             and SECRET not in ml2.stdout.lower()},
        ]
        self.reconfigure({"memory": {"use": False}})
        self.rset(reflex=True, notes=True)
        said["off"] = self.utter("note")
        off = self.line("lineNoteOff")
        checks.append({"name": f"memory off: an honest {off!r}", "value": said["off"]["lines"], "pass": off in said["off"]["lines"]})
        # notes reach memory's own store (you.md under data/hyakki/memory,
        # memory.db under state/hyakki/memory) and nothing else: no notes.md,
        # no other Markdown written during the step, and the note's words in
        # no other lab file (the skills' shipped docs predate it)
        memdirs = [str(self.lab / "data" / "hyakki" / "memory"), str(self.lab / "state" / "hyakki" / "memory")]
        md = [str(p) for p in self.lab.rglob("*") if p.is_file() and not str(p).startswith(str(self.lab / "voice"))
              and not any(str(p).startswith(m + os.sep) for m in memdirs)
              and ("note" in p.name.lower() and p.suffix == ".md" or p.suffix == ".md" and p.stat().st_mtime >= t_step)]
        elsewhere, _ = self.word_counts(["exam is on"], t_step, exclude=memdirs)
        counts, _ = self.word_counts([SECRET], 0.0)
        checks += [
            {"name": "no notes file anywhere in the lab: the note is only in memory's store",
             "value": {"markdown": md, "note_words_elsewhere": elsewhere}, "pass": not md and not elsewhere},
            {"name": "the refused secret is in no lab file", "value": counts, "pass": not counts},
        ]
        return {"said": said, "checks": checks}, self.checks_pass(checks)

    # --- step 12: decisions, local ---

    def decide_try(self, mode, delay=None):
        self.decide_mode(mode, delay)
        self.close_apps()
        self.cooldown()
        d0, n0 = len(self.decisions()), self.requests_n()
        r0 = self.rlog() or []
        t0 = time.time()
        u = self.utter("decide", timeout=20)
        time.sleep(2.0)
        reqs = self.model_requests()[n0:]
        ask_s = None if not reqs or not isinstance(reqs[0].get("t"), (int, float)) else round(reqs[0]["t"] - u["up"], 2)
        return {"mode": mode, "said": u, "decisions": self.decisions()[d0:], "asks": len(reqs), "ask_after_s": ask_s,
                "rows": self.rows_since(r0), "launches": len(self.launches("notes", t0))}

    def decisions13(self):
        checks = []
        try:
            hw = decision_hardware(Path("/proc/meminfo").read_text(), Path("/proc/cpuinfo").read_text())
        except OSError:
            hw = (False, 0.0, 0)
        self.results["reflex"]["decision_hardware"] = {"pass": hw[0], "mem_gib": hw[1], "physical_cores": hw[2]}
        checks.append({"name": "this machine passes the local decision route's hardware gate (6 GiB, 4 physical cores)",
                       "value": self.results["reflex"]["decision_hardware"], "pass": hw[0]})
        self.rset(reflex=True, decisions=False)
        pre = self.decide_try("keyword")
        checks.append({"name": "the rules miss the phrase: with decisions off nothing runs and it is an Ask",
                       "value": {"rows": pre["rows"], "launches": pre["launches"], "asks": pre["asks"],
                                 "decisions": len(pre["decisions"])},
                       "pass": pre["launches"] == 0 and not pre["decisions"] and pre["asks"] == 1})
        self.rset(decisions=True)
        self.decisions_reset()
        ok = self.decide_try("keyword")
        req_ok = [decision_request_ok(d, self.labels) for d in ok["decisions"]]
        budget = self.rspec["decisions"]["budgetMs"] / 1000.0
        checks += [
            {"name": "the local route classifies it and it runs: one decision, a local done row, one start",
             "value": {"rows": ok["rows"], "launches": ok["launches"], "decisions": len(ok["decisions"])},
             "pass": len(ok["decisions"]) == 1 and ok["launches"] == 1
             and [r.get("route") for r in ok["rows"] if r.get("intent") == "open_app" and r.get("outcome") == "done"] == ["local"]},
            {"name": "the request held only the model, the utterance and the registry's labels",
             "value": {"checks": req_ok, "first": (ok["decisions"] or [None])[0]},
             "pass": bool(req_ok) and all(x[0] for x in req_ok)
             and all(d.get("model") == DECISION_MODEL
                     and (d.get("utterance") or "").strip().rstrip(".!?").casefold()
                     == (ok["said"]["heard"] or "").strip().rstrip(".!?").casefold()
                     for d in ok["decisions"])},
        ]
        tries = {}
        for mode, delay in (("unsure", None), ("fail", None), ("slow", 3)):
            tries[mode] = t = self.decide_try(mode, delay)
            checks.append({"name": f"{mode}: nothing runs and the words fall through to the Ask within the budget",
                           "value": {k: t[k] for k in ("decisions", "asks", "ask_after_s", "rows", "launches")},
                           "pass": t["launches"] == 0 and t["asks"] == 1
                           and not [r for r in t["rows"] if r.get("outcome") == "done"]
                           and (t["ask_after_s"] is None or t["ask_after_s"] <= budget + self.vspec["budgets"]["finalS"] + 6)})
        tries["hostile"] = h = self.decide_try("hostile")
        checks.append({"name": "hostile (a label never offered): refused, nothing runs",
                       "value": {k: h[k] for k in ("decisions", "asks", "rows", "launches")},
                       "pass": h["launches"] == 0 and not [r for r in h["rows"] if r.get("outcome") == "done"]})
        self.decide_mode("keyword")
        dec = (self.cfg().get("providers") or {}).get("decisions") or {}
        checks.append({"name": "the decision route is loopback only", "value": dec.get("localAddress"),
                       "pass": urllib.parse.urlsplit(dec.get("localAddress") or "").hostname == "127.0.0.1"})
        (self.out / "model" / "decisions.json").write_text(json.dumps(self.decisions(), indent=2))
        self.close_apps()
        self.limit(12, "decision quality", "decidefix.py is a keyword table: thresholds, fall-through and refusal are "
                                           "proven, not a model's accuracy or calibration; the bench is S15's")
        return {"tries": dict(tries, keyword=ok, rules_only=pre), "checks": checks}, self.checks_pass(checks)

    # --- step 13: decisions, hosted ---

    def hosted13(self):
        checks, tries = [], {}
        self.rset(reflex=True, decisions=True)
        try:
            # localModel cleared: the gateway's Ember and automatic routes
            # would otherwise fall back to the local fixture
            for label, patch in (("ember", {"route": "ember", "allowHosted": None, "localModel": None,
                                            "features": {"oni.reflex": "ember"}}),
                                 ("auto", {"route": "auto", "allowHosted": True, "localModel": None,
                                           "features": {"oni.reflex": "auto"}})):
                self.reconfigure({"providers": {"decisions": patch}})
                self.rset(reflex=True, decisions=True)
                self.decisions_reset()
                m = self.watch.mark()
                t = self.decide_try("keyword")
                cards = [d for _, d in self.watch.frames("consent", m) if d.get("feature") == "reflex.decisions"]
                tries[label] = dict(t, cards=cards)
                checks.append({"name": f"{label}: no account, zero hosted decisions, no disclosure card, and the local "
                                       "fixture never asked",
                               "value": {"rows": t["rows"], "cards": cards, "local_decisions": len(t["decisions"])},
                               "pass": not cards and not [r for r in t["rows"] if r.get("route") == "ember"]
                               and not t["decisions"] and t["launches"] == 0})
        finally:
            self.reconfigure({"providers": {"decisions": {"route": "local", "allowHosted": None,
                                                          "localModel": DECISION_MODEL,
                                                          "features": {"oni.reflex": "local"}}}})
        self.close_apps()
        self.limit(13, "hosted decisions", "the lab has no Ryoku account: the card's account binding, credit "
                                           "exhaustion and the per-minute cap are the daemon wire tests' (fakeHosted)")
        return {"tries": tries, "checks": checks}, self.checks_pass(checks)

    # --- step 14: self-commands ---

    def self13(self):
        checks, said = [], {}
        self.rset(reflex=True)
        self.settings(sounds=True)
        p0 = self.placement() or {}
        s0 = self.oni_settings()
        said["move"] = self.utter("move_tl")
        time.sleep(1.5)
        p1 = self.placement() or {}
        said["bigger"] = self.utter("bigger")
        s1 = self.oni_settings()
        said["home"] = self.utter("home")
        time.sleep(1.5)
        p2 = self.placement() or {}
        said["sounds"] = self.utter("sounds_off")
        s2 = self.oni_settings()
        checks += [
            {"name": "\"move to the top left\" pins Oni top left", "value": [p0.get("corner"), p1.get("corner")],
             "pass": p1.get("corner") == "tl"},
            {"name": "\"make yourself bigger\" changes the size setting", "value": [s0.get("size"), s1.get("size")],
             "pass": s1.get("size") != s0.get("size") or self.line("lineReflexBiggest") in said["bigger"]["lines"]},
            {"name": "\"go home\" leaves the top-left pin", "value": [p1.get("corner"), p2.get("corner"), said["home"]["lines"]],
             "pass": p2 != p1 and self.line("lineReflexHome") in said["home"]["lines"]},
            {"name": "a pressed \"turn off your sounds\" turns them off", "value": s2.get("sounds"), "pass": s2.get("sounds") is False},
        ]
        self.settings(sounds=True)
        cfg0 = self.cfg_path.read_bytes()
        refuse = self.line("lineInSettings")
        for name in ("on_openmic", "on_wake", "on_hands"):
            said[name] = self.utter(name)
        o = self.oni_settings()
        vs = o.get("voiceSettings") or {}
        checks += [
            {"name": f"turning open mic, wake or hands on by voice is refused ({refuse!r})",
             "value": {n: said[n]["lines"] for n in ("on_openmic", "on_wake", "on_hands")},
             "pass": all(refuse in said[n]["lines"] for n in ("on_openmic", "on_wake", "on_hands"))},
            {"name": "the settings file is unchanged and none of them is on", "value": {"openMic": vs.get("openMic"),
                                                                                     "wake": vs.get("wake"), "hands": o.get("hands")},
             "pass": self.cfg_path.read_bytes() == cfg0 and not vs.get("openMic")
             and not (vs.get("wake") or {}).get("enabled") and o.get("hands") in (None, "", "off")},
        ]
        said["wake_flip"] = self.wake_say("sounds_off")
        after_wake = self.oni_settings().get("sounds")
        said["open_flip"] = self.open_say("sounds_off_named")
        after_open = self.oni_settings().get("sounds")
        press = self.line("ReflexNeedsPressLine")
        checks.append({"name": "wake and open mic never flip a setting", "value": {"wake": [said["wake_flip"]["heard"], after_wake],
                                                                                   "open": [said["open_flip"]["heard"], after_open]},
                       "pass": after_wake is True and after_open is True})
        checks.append({"name": f"both recognized commands say {press!r}, including real addressed open-mic PCM",
                       "value": {"wake": said["wake_flip"], "open": said["open_flip"]},
                       "pass": bool(said["wake_flip"]["heard"])
                       and bool(re.search(r"\b" + re.escape(CHARACTER_NAME) + r"\b", said["open_flip"]["heard"] or "", re.I))
                       and press in said["wake_flip"]["lines"] and press in said["open_flip"]["lines"]})
        # oni.place takes the whole spot (output, fx, fy, corner), as labkit sends it
        outs = p0.get("outputs") or []
        spot = next((o for o in outs if o.get("id") == p0.get("active")), outs[0] if outs else None)
        if spot:
            self.call("oni.place", {"output": spot["id"], "fx": spot.get("fx", 0.5), "fy": spot.get("fy", 0.5),
                                    "corner": p0.get("corner") or ""})
        if s0.get("size"):
            self.settings(size=s0["size"])
        return {"said": said, "checks": checks}, self.checks_pass(checks)

    # --- step 15: read-only and boundaries ---

    def readonly13(self):
        hits = []
        for root in (APP / "internal/oni", APP / "internal/reflex"):
            for path in sorted(root.rglob("*.go")):
                if path.name.endswith("_test.go"):
                    continue
                for n, line in enumerate(path.read_text(errors="replace").splitlines(), 1):
                    low = line.lower()
                    for mark in labkit.INPUT_MARKS:
                        if mark in low:
                            hits.append({"file": str(path.relative_to(ROOT)), "line": n, "mark": mark})
        strings = []
        for rel in COPY_SOURCES:
            p = APP / rel
            files = [p] if p.is_file() else sorted(q for q in p.rglob("*.go") if not q.name.endswith("_test.go"))
            for f in files:
                strings += user_strings(f.read_text(errors="replace"))
        tp = third_party_hits(strings)
        tests = {pkg: self.run_test(pkg, rel, run) for pkg, rel, run in TEST_PKGS}
        seen = list(self.vchildren.seen.values())
        bad = [s for s in seen if not child_allowed13(s, HERE, self.labout)]
        inputs = [s for s in seen if s["comm"] in INPUT_PROGRAMS]
        inputs += [{"pid": p, "comm": c} for p, c in self.vchildren.unread.items() if c in INPUT_PROGRAMS]
        starts = [(s["argv"], fixture_launch(s["argv"], HERE, self.labout)) for s in seen
                  if fixture_launch(s["argv"], HERE, self.labout)]
        scoped = [a for a, fl in starts if fl[1]]
        refused_wrapped = [a for a in self.stubbed() if os.path.basename(a[0]) == "systemd-run" and a[1:2] == ["--user"]]
        bare = [a for a, fl in starts if not fl[1]]
        other_apps = [s for s in bad if s["comm"] == "python3" or "systemd-run" in os.path.basename((s["argv"] or [""])[0])]
        caps = self.caps()
        hands = {k: v for k, v in caps.items() if k.startswith("hands")}
        hframes = self.hands_frames_off(0)
        checks = [
            {"name": "no input-executor name under internal/oni or internal/reflex", "value": hits, "pass": not hits},
            {"name": "no third-party product or model name in reflex, reminder or routine copy",
             "value": {"strings": len(strings), "hits": tp}, "pass": bool(strings) and not tp},
        ]
        checks += [{"name": f"{pkg}.test ({rel})", "value": tests.get(pkg), "pass": bool(tests.get(pkg, {}).get("pass"))}
                   for pkg, rel, _ in TEST_PKGS]
        checks += [
            {"name": "every program the daemon ran is on its allowlist (argv read before it exited)",
             "value": {"seen": len(seen), "outside": bad, "exited_unread": len(self.vchildren.unread)},
             "pass": bool(seen) and not bad},
            {"name": "every launched app went through the lab launcher: bare (scope=fallback), or wrapped and refused first",
             "value": {"bare": len(bare), "scoped_seen": scoped, "scoped_refused": len(refused_wrapped), "others": other_apps},
             "pass": bool(bare) and not other_apps and len(scoped) <= len(refused_wrapped)},
            {"name": "the daemon started no input program", "value": inputs, "pass": not inputs},
            {"name": "Hands stays off: all three compiled stages guarded and denied",
             "value": {"rows": hands, "frames": hframes},
             "pass": set(hands) == {"hands.browser", "hands.app", "hands.desktop"}
             and all(v.get("stage") == "guarded" and not v.get("allow") for v in hands.values()) and not hframes},
        ]
        return {"tests": tests, "checks": checks}, self.checks_pass(checks)

    # --- step 16: disk ---

    def shutdown_privacy13(self):
        """Interrupt real default-ASR work; observe cleanup at daemon exit.
        No scratch file is removed here, including on a failed probe."""
        probe = {"fixture": "storm", "stt_overrides": self.stt,
                 "default_asr": not any(self.stt.values()), "workers": [], "exit_observed": False}
        before_oni = self.oni_settings()
        was_voice = before_oni.get("voice", False)
        before_vs = before_oni.get("voiceSettings") or {}
        was_open = before_vs.get("openMic", False)
        was_wake = before_vs.get("wake") or {"enabled": False, "phrase": WAKE_PHRASE}
        was_ui = bool(lab_pids(self.env["LAB_ID"], "hyakki-app"))
        was_surface = bool(lab_pids(self.env["LAB_ID"], "oni-surface"))
        play = None
        pressed = False
        since = time.time()
        try:
            self.close_ui()
            self.vset(openMic=False, wake={"enabled": False, "phrase": WAKE_PHRASE})
            self.settings(voice=True)
            probe["prior_decoders"] = self.drain_file_decoders()
            if not probe["prior_decoders"]["idle"]:
                raise RuntimeError("earlier decodes did not finish before the shutdown probe")
            mark = self.watch.mark()
            pressed = self.keys.send("down") is not None
            self.wait_voice(mark, lambda d: d.get("phase") == "listening" and d.get("source") == "chord",
                            5, "real chord capture before shutdown")
            # Do this before playback, not after finding a short-lived worker:
            # the watch must neither restart oni nor consume the interruption.
            self.watch.pause()
            since = time.time()
            play = subprocess.Popen([str(HERE / "audio.sh"), "inject", self.rfx["fixtures"]["storm"]["file"]],
                                    stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
            deadline = time.monotonic() + min(self.rfx["fixtures"]["storm"]["seconds"],
                                              self.vspec["budgets"]["finalS"] + 12)
            while time.monotonic() < deadline:
                workers = [w for w in self.file_decoders()
                           if self._cmdline(w["pid"]) and all(Path(p).is_file() for p in w["files"])]
                if workers:
                    probe["workers"] = workers
                    break
                time.sleep(0.01)
            if not probe["workers"]:
                # A real alternate adapter is evidence, not a fabricated pass
                # for the file-only adapter whose cleanup this case exercises.
                probe["adapter_evidence"] = [
                    {"pid": r["pid"], "comm": r["comm"], "argv": r["argv"]}
                    for r in self.vchildren.procs(since=since)
                    if r["comm"] in voicekit.ENGINE_PROGRAMS]
                raise RuntimeError("no live private-WAV decoder was exercised; inspect adapter_evidence")
            daemons = self.daemon_pids()
            probe["daemons"] = daemons
            if len(daemons) != 1:
                raise RuntimeError("shutdown probe requires exactly one isolated oni process")
            daemon_ticks = self.process_start_ticks(daemons[0])
            probe["daemon_identity"] = {"pid": daemons[0], "start_ticks": daemon_ticks}
            probe["live_before_term"] = all(w.get("start_ticks") is not None
                                            and self.process_start_ticks(w["pid"]) == w["start_ticks"]
                                            and self._cmdline(w["pid"]) and all(Path(p).is_file() for p in w["files"])
                                            for w in probe["workers"])
            if not probe["live_before_term"] or daemon_ticks is None or self.process_start_ticks(daemons[0]) != daemon_ticks:
                raise RuntimeError("a captured process identity or active WAV changed before TERM")
            probe["term_at"] = time.time()
            os.kill(daemons[0], signal.SIGTERM)
            deadline = time.monotonic() + 15
            while time.monotonic() < deadline:
                if not self.daemon_pids():
                    probe["exit_observed"] = True
                    probe["exit_at"] = time.time()
                    # Capture this immediately. Waiting for cleanup after an
                    # observed daemon exit would conceal the regression.
                    probe["pids_at_exit"] = [w["pid"] for w in probe["workers"]
                                             if self.process_start_ticks(w["pid"]) == w["start_ticks"]]
                    files = sorted({p for w in probe["workers"] for p in w["files"]})
                    probe["files_at_exit"] = [p for p in files if Path(p).exists()]
                    probe["dirs_at_exit"] = [p for p in sorted({str(Path(p).parent) for p in files}) if Path(p).exists()]
                    probe["decoders_at_exit"] = self.file_decoders()
                    break
                time.sleep(0.01)
        except (OSError, RuntimeError, ValueError, subprocess.TimeoutExpired) as e:
            probe["error"] = f"{type(e).__name__}: {e}"
        finally:
            self.watch.pause()
            if pressed:
                self.keys.send("up")
            if play is not None:
                try:
                    output, _ = play.communicate(timeout=self.rfx["fixtures"]["storm"]["seconds"] + 10)
                except subprocess.TimeoutExpired:
                    # Reap only the private injector, never a device/player on
                    # the user's session. No product scratch is touched.
                    labkit.stop_role(self.env["LAB_ID"], "audio-play-", "TERM")
                    output, _ = play.communicate(timeout=10)
                probe["playback"] = {"exit": play.returncode, "output": output[-2000:]}
            if self.daemon_pids():
                self.daemon_stop()
            self.daemon_start()
            self.watch.resume()
            self.settings(voice=was_voice)
            self.vset(openMic=was_open, wake=was_wake)
            if was_surface:
                self.sh("voice.sh", "keys", timeout=60)
            if was_ui:
                self.open_ui()
            probe["restarted"] = bool(self.daemon_pids())
            probe["presenter_restored"] = bool(lab_pids(self.env["LAB_ID"], "oni-surface")) == was_surface
            after_oni = self.oni_settings()
            after_vs = after_oni.get("voiceSettings") or {}
            probe["state_restored"] = (after_oni.get("voice") == was_voice
                                       and after_vs.get("openMic", False) == was_open
                                       and after_vs.get("wake") == was_wake
                                       and bool(lab_pids(self.env["LAB_ID"], "hyakki-app")) == was_ui
                                       and probe["presenter_restored"])
        checks = [
            {"name": "shutdown privacy uses real PCM and the default ASR, with a live file decoder before TERM",
             "value": probe,
             "pass": probe["default_asr"] and pressed and bool(probe["workers"]) and bool(probe.get("live_before_term"))
             and not probe.get("error")},
            {"name": "at actual oni SIGTERM exit, its decoder is reaped and the private WAV and directory are already gone",
             "value": {k: probe.get(k) for k in ("exit_observed", "term_at", "exit_at", "pids_at_exit",
                                                "files_at_exit", "dirs_at_exit", "decoders_at_exit")},
             "pass": probe["exit_observed"] and probe.get("pids_at_exit") == []
             and probe.get("files_at_exit") == [] and probe.get("dirs_at_exit") == []
             and probe.get("decoders_at_exit") == []},
            {"name": "the private PCM injector finishes and the daemon, presenter and UI are restored for remaining proofs",
             "value": {"playback": probe.get("playback"), "restarted": probe.get("restarted"),
                       "presenter_restored": probe.get("presenter_restored"), "state_restored": probe.get("state_restored")},
             "pass": (probe.get("playback") or {}).get("exit") == 0 and probe.get("restarted") is True
             and probe.get("state_restored") is True},
        ]
        return probe, checks

    def disk13(self):
        shutdown, shutdown_checks = self.shutdown_privacy13()
        decoder_idle = self.drain_file_decoders()
        start = self.start_wall - 1
        audio, images, words, scanned = [], [], [], 0
        allow = [str(self.lab / "voice"), str(self.lab / "data/hyakki/oni/voice"), str(self.lab / "log/model-requests.jsonl"),
                 str(self.lab / "log/model-decisions.jsonl"), str(self.lab / "log/model.log")]
        for dirpath, _, filenames in os.walk(self.lab):
            for name in filenames:
                path = os.path.join(dirpath, name)
                if any(path == s or path.startswith(s + os.sep) for s in allow):
                    continue
                try:
                    st = os.lstat(path)
                    if not stat.S_ISREG(st.st_mode) or st.st_mtime < start or st.st_size == 0 or st.st_size > 32 << 20:
                        continue
                    data = Path(path).read_bytes()
                    scanned += 1
                except OSError:
                    continue
                if data[:4] in (b"RIFF", b"OggS", b"fLaC") or data[:3] == b"ID3":
                    audio.append(path)
                if guidekit.is_picture(data) if hasattr(guidekit, "is_picture") else data.startswith(guidekit.IMAGE_MAGICS):
                    images.append(path)
                found = guidekit.leaks(data, list(TRANSCRIPT_MARKS))
                words += [{"file": path, "found": f} for f in found if f.startswith("word")]
        own = {}
        for p in (self.reminders_path, self.routines_path):
            try:
                own[str(p)] = {"mode": oct(file_mode(p) or 0), "keys": sorted(json.loads(p.read_text()) or {})}
            except (OSError, ValueError, TypeError):
                own[str(p)] = None
        tmp = self.lab / "tmp"
        left = sorted(str(p.relative_to(tmp)) for p in tmp.rglob("*")) if tmp.is_dir() else None
        checks = shutdown_checks + [
            {"name": "all real private-WAV decoder workers completed before the privacy scan",
             "value": decoder_idle, "pass": decoder_idle["idle"]},
            {"name": "no audio file written in the lab outside the speech fixtures and models", "value": audio, "pass": not audio},
            {"name": "no picture file written in the lab", "value": images, "pass": not images},
            {"name": "no transcript words in any lab file (the model's own log aside)", "value": words, "pass": not words},
            {"name": "the reminders and routines files are 0600 where present", "value": own,
             "pass": all(v is None or v["mode"] == oct(0o600) for v in own.values())},
            {"name": "the lab processes' temp dir is empty", "value": left, "pass": left == []},
        ]
        return {"files_scanned": scanned, "shutdown": shutdown, "decoder": decoder_idle,
                "checks": checks}, self.checks_pass(checks)

    # --- step 17: reduced motion ---

    def reduced13(self):
        before = self.oni_settings().get("reduceMotion", "follow")
        self.settings(reduceMotion="on")
        try:
            self.rset(reflex=True, reminders=True)
            self.perch_fixture()
            u = self.utter("remind10")
            lm = self.ledger.mark()
            w, t = self.fired(u["mark"], "stretch", 25)
            time.sleep(4.0)
            counts = part_commits(self.ledger.since(lm))
        finally:
            self.settings(reduceMotion=before)
            self.ensure_detached()
        checks = [
            {"name": "the reminder rang", "value": t, "pass": w is not None},
            {"name": "its bubble commits at most once per part", "value": counts,
             "pass": counts is not None and all(v <= 1 for v in counts.values())},
        ]
        return {"checks": checks}, self.checks_pass(checks)

    # --- step 18: settlement ---

    def settle13(self):
        th = labkit.THRESHOLDS
        self.vset(openMic=False, wake={"enabled": False, "phrase": WAKE_PHRASE})
        self.rset(reflex=True, reminders=True, routines=True, notes=True, decisions=False)
        self.jcli("oni", "reminders", "--all", "--json")
        self.close_apps()
        self.ledger.quiet(5.0, 120)
        self.wake()
        idle = self.measure("settled", timeout=120, seconds=60, name="reflex-idle")
        self.utter("remind30")
        _, lst, _ = self.reminders()
        self.ledger.quiet(5.0, 120)
        self.wake()
        pending = self.measure("settled", timeout=120, seconds=60, name="reminder-pending")
        self.jcli("oni", "reminders", "--all", "--json")
        self.rset(reflex=False)
        time.sleep(3.0)
        left = [s for s in self.vchildren.alive(("pactl", "niri", "python3", "systemd-run"))]
        self.ledger.quiet(5.0, 120)
        self.wake()
        off = self.measure("settled", timeout=120, seconds=60, name="reflex-off")

        def within(rec):
            rates = self.daemon_rates(rec)
            return all(v["cpu_percent"] is not None and v["cpu_percent"] < th["cpu_percent_max"]
                       and v["wakeups_per_s"] is not None and v["wakeups_per_s"] <= th["wakeups_per_s_max"] for v in rates), rates
        ok_idle, r_idle = within(idle)
        ok_pend, r_pend = within(pending)
        ok_off, r_off = within(off)
        checks = [
            {"name": "reflex on, nothing pending: 0 commits, 0 armed callbacks", "value": idle.get("ledger_window"),
             "pass": bool(idle.get("pass"))},
            {"name": "reflex on, nothing pending: the daemon within budget", "value": r_idle, "pass": ok_idle},
            {"name": "one reminder pending (within the residency horizon)", "value": lst.get("reminders"),
             "pass": len(lst.get("reminders") or []) == 1},
            {"name": "one reminder pending: 0 commits, the resident daemon within budget (one timer, near-zero CPU)",
             "value": {"ledger": pending.get("ledger_window"), "daemon": r_pend}, "pass": bool(pending.get("pass")) and ok_pend},
            {"name": "reflex off: nothing of it left running", "value": [(p["comm"], p["argv"][:3]) for p in left], "pass": not left},
            {"name": "reflex off: 0 commits, the daemon within budget (or gone)", "value": {"ledger": off.get("ledger_window"),
                                                                                         "daemon": r_off},
             "pass": bool(off.get("pass")) and ok_off},
        ]
        self.limit(18, "one timer armed", "the daemon's armed-timer count is its own test (clock.Armed() <= 1); the lab "
                                          "measures the resident daemon's CPU and wakeups instead")
        return {"checks": checks}, self.checks_pass(checks)

    # --- step 19: display ---

    def display13(self):
        details, ok = self.display_check()
        envs = [s["env"].get("WAYLAND_DISPLAY") for s in self.vchildren.seen.values()
                if fixture_launch(s["argv"], HERE, self.labout)]
        check = {"name": "every launched app got the lab's display", "value": sorted(set(e for e in envs if e)),
                 "pass": bool(envs) and all(e == self.env.get("LAB_DISPLAY") for e in envs)}
        if isinstance(details, dict):
            details.setdefault("checks", []).append(check)
        return details, ok and check["pass"]

    def run(self):
        if self.model is None:
            self.results["steps"].append({"step": 0, "name": "lab model", "pass": False,
                                          "error": "no LAB_MODEL_URL: phase13 needs up.sh --model"})
            self.save()
            return 1
        self.vchildren.start()
        try:
            self.step(0, "census", self.census13)
            if self.keys is None:
                raise RuntimeError("no chord FIFO: the rest needs step 0")
            self.step(1, "off is zero", self.off13)
            self.step(2, "launch", self.launch13)
            self.step(3, "tabs", self.tabs13)
            self.step(4, "window verbs", self.wm13)
            self.step(5, "audio", self.audio13)
            self.step(6, "timers and reminders", self.timers13)
            self.step(7, "zero at rest, both inits", self.rest13)
            self.step(8, "storm", self.storm13)
            self.step(9, "forbidden", self.forbidden13)
            self.step(10, "routines", self.routines13)
            self.step(11, "notes", self.notes13)
            self.step(12, "decisions, local", self.decisions13)
            self.step(13, "decisions, hosted", self.hosted13)
            self.step(14, "self-commands", self.self13)
            self.step(15, "read-only and boundaries", self.readonly13)
            self.step(16, "disk", self.disk13)
            self.step(17, "reduced motion", self.reduced13)
            self.step(18, "settlement", self.settle13)
            self.step(19, "ledger display is the lab's", self.display13)
        except RuntimeError as e:
            print(f"phase13: stopped: {e}")
        finally:
            try:
                self.close_apps()
            except (OSError, RuntimeError, ValueError):
                pass
            self.vchildren.done.set()
            self.vchildren.join(timeout=5)
            self.watch.stop()
            if self.clock:
                self.clock.done.set()
            if self.keys:
                self.keys.close()
                self.results["keys_sent"] = self.keys.log
            self.results["daemon_programs"] = list(self.vchildren.seen.values())
            with (self.out / "frames.jsonl").open("w") as fh:
                for l in self.watch.lines:
                    fh.write(json.dumps(l) + "\n")
            self.results["frame_watch_restarts"] = self.watch.restarts
            try:
                (self.out / "model" / "requests.json").write_text(json.dumps(self.model.requests(), indent=2))
                (self.out / "model" / "decisions.json").write_text(json.dumps(self.decisions(), indent=2))
            except OSError:
                pass
        self.results["finished_at"] = now()
        self.save()
        for limit in self.results.get("limits", []):
            print(f"limit (step {limit['step']}): {limit['check']}: {limit['reason']}")
        return 0 if self.results["pass"] else 1


# --- command line ------------------------------------------------------------

def main(argv):
    if not argv:
        raise SystemExit(__doc__)
    cmd, args = argv[0], argv[1:]
    if cmd == "fixtures" and len(args) == 1:
        return render_fixtures(args[0])
    if cmd == "selftest" and not args:
        return selftest()
    if cmd == "phase13" and len(args) == 2:
        return Phase13(*args).run()
    raise SystemExit(__doc__)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
