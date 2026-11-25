#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Phase 12 helpers for the Oni lab: the guide's speech fixtures, the pure
checks the gate rests on, and the Phase 12 gate itself (guides, Keep,
model-free replay, select-and-ask). It builds on labkit.py and voicekit.py
without changing them and starts lab processes only through the lab's shell
scripts (phase12.sh guidewin|daemon among them).

guidekit.py fixtures DIR        the guide's speech WAVs (espeak-ng -w, never a
                                device) and fixtures.json into DIR, under
                                /dev/shm
guidekit.py selftest            the pure helpers against synthetic data; no
                                lab, no process
guidekit.py phase12 LAB OUT     the gate (phase12.sh runs it)

The contract this gate relies on (CLI verbs, frames, ledger fields, env hooks
and model routes) is the header of phase12.sh and the table in the lab
README's Phase 12 section."""
import json
import math
import os
import re
import signal
import stat
import subprocess
import sys
import time
from pathlib import Path

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import labkit  # noqa: E402
import voicekit  # noqa: E402
from labkit import (CAPTURE_PATHS, close_box, comm_of, consent_of, global_rect, grow, inside,  # noqa: E402
                    lab_pids, now, summary)
from voicekit import (CAPTURE_PROGRAMS, ENGINE_PROGRAMS, LEAD_S, PLAYER_PROGRAMS, TAIL_S, WAKE_PHRASE,  # noqa: E402
                      Phase11, read_wav16, write_wav16)

ROOT = HERE.parents[4]
APP = ROOT / "ryoku/hyakki"

# --- what the gate drives ----------------------------------------------------

# phase12.sh starts guidewin.py with these; the title is a canary for the keeps
GUIDE_APP = "dev.ryoku.OniGuide"
GUIDE_TITLE = "Oni guide Lindqvist ledger"
GOAL_SAVE = "how do I save this file"
GOAL_SAVE2 = "how do I export a copy"
GOAL_DEAD = "how do I press dead"
GOAL_PIN = "where do I type my PIN"
PICK_QUESTION = "What does this do?"
# typed into the window's PIN entry; it may appear nowhere but the window
TYPED_PIN = "4817"
# guidewin.py's document text, read by OCR: window text the lab never writes
WINDOW_WORDS = ("Quarterly", "errands", "printer")
# model.py's hostile plan carries this word
CANARY = "Zephyr"
HOSTILE_PIN = "4321"
PICK_NS = "hyakki-oni-pick"
MODE_PERCH = 3
# the daemon's lab-only read-timeout hook (HYAKKI_ONI_LAB=1), agreed with S8
TIMEOUT_HOOK = "HYAKKI_ONI_GUIDE_READ_TIMEOUT_MS"
TIMEOUT_HOOK_MS = 5000
# the guide's own speech, played into the virtual microphone; "yes" and the
# wake phrase are voice.sh's fixtures
GUIDE_FIXTURES = {
    "guide": ("How do I save this file?", 150),
    # one word ("Next.", "Stop.") reached the engine through the lab's graph
    # as "Yeah." and "So": whole phrases the router knows carry over
    "next": ("Okay, next step.", 140),
    "stop": ("Stop talking.", 140),
    "yes": ("Yes.", 150),
}
# the test binaries step 13 runs: (package, path under app, -test.run)
TEST_PKGS = (
    ("feature", "internal/oni/feature", "(?i)(boundary|apart|import|insert|guide)"),
    ("window", "../wm/oniwindow",
     "^(TestAllowlistIsReadVerbsOnly|TestProvidersStayInsideTheAllowlist|TestPackageSourceHasNoOtherExecPath)$"),
    ("ocr", "internal/oni/ocr", "."),
    ("keep", "internal/oni/keep", "."),
    ("guide", "internal/oni/guide", "."),
)
GUIDE_PKGS = ("guide", "keep", "ocr", "settle")
INPUT_PROGRAMS = voicekit.INPUT_PROGRAMS
LANG_RE = re.compile(r"[a-z][a-z_]{0,30}(\+[a-z][a-z_]{0,30})*")
IMAGE_MAGICS = (b"\x89PNG", b"\xff\xd8\xff", b"GIF8")
# WebP is RIFF with its form type; a WAV is RIFF too and no picture
WEBP_RE = re.compile(rb"RIFF.{4}WEBP", re.S)
INLINE_MARKS = (b"data:image/", b"/9j/4", b"iVBORw0KGgo")
# voice/audio's pactl reads (audio.Allowed): the source list and default
PACTL_READS = (["-f", "json", "list", "sources"], ["get-default-source"])


# --- pure helpers (selftest covers each) -------------------------------------

def go_lines(src):
    """name -> text for every `name = i18n.T("text")` in Go source: the
    guide's closing lines and kickers, read from the code, not copied."""
    return {m.group(1): m.group(2).replace('\\"', '"')
            for m in re.finditer(r'(\w+)\s*=\s*i18n\.T\("((?:[^"\\]|\\.)*)"\)', src)}


def tesseract_allowed(argv):
    """ocr.Allowed: tesseract stdin stdout -l LANG --psm 11 tsv, nothing else."""
    if len(argv) != 8 or os.path.basename(argv[0]) != "tesseract":
        return False
    a = argv[1:]
    return (a[:3] == ["stdin", "stdout", "-l"] and len(a[3]) <= 64 and bool(LANG_RE.fullmatch(a[3]))
            and a[4:] == ["--psm", "11", "tsv"])


VOICE_PROGRAMS = set(CAPTURE_PROGRAMS + ENGINE_PROGRAMS + PLAYER_PROGRAMS)


def child_allowed(rec):
    """A program the daemon ran this run: the window and capture read verbs
    (labkit.argv_allowed), tesseract on its allowlist, voice/audio's pactl
    reads, or the Phase 11 voice pipeline (by name; its own gate holds the
    argv)."""
    argv = rec.get("argv") or []
    if not argv:
        return False
    if os.path.basename(argv[0]) in INPUT_PROGRAMS or rec.get("comm") in INPUT_PROGRAMS:
        return False
    if os.path.basename(argv[0]) == "pactl":
        return argv[1:] in [list(a) for a in PACTL_READS]
    return labkit.argv_allowed(argv) or tesseract_allowed(argv) or rec.get("comm") in VOICE_PROGRAMS


def mid(box):
    return (box[0] + box[2] / 2, box[1] + box[3] / 2)


def target_box(line, out):
    """A strand-target line's box, global: the ledger's is output-local."""
    b = ((line or {}).get("target") or {}).get("box")
    if not b or out is None:
        return None
    return [out["x"] + b["x"], out["y"] + b["y"], b["w"], b["h"]]


def target_mode(line):
    return ((line or {}).get("target") or {}).get("mode")


def click_point(box, avoid=(), step=3):
    """A point inside BOX that no AVOID rect covers: the centre when free,
    else the free grid point nearest it, 2 px inside the edges."""
    c = mid(box)
    if not any(inside(c, r) for r in avoid):
        return c
    best = None
    x0, y0, x1, y1 = box[0] + 2, box[1] + 2, box[0] + box[2] - 2, box[1] + box[3] - 2
    y = y0
    while y <= y1:
        x = x0
        while x <= x1:
            if not any(inside((x, y), r) for r in avoid):
                d = math.hypot(x - c[0], y - c[1])
                if best is None or d < best[0]:
                    best = (d, (x, y))
            x += step
        y += step
    return None if best is None else best[1]


def card_pill_h(spec):
    ch = spec["perch"]["chip"]
    return math.ceil(ch["textPx"] * 1.3) + 2 * ch["padYPx"]


def card_next_point(rect, spec):
    """Where the card's Next pill is, global: render.LayoutCard puts the
    pills right-aligned on the card's last row, Next (the primary) last,
    bubble.textPx inside the card's edge; 12 px in from its right end is
    inside any pill at least as wide as its own padding."""
    pad = spec["strand"]["bubble"]["textPx"]
    return (rect[0] + rect[2] - pad - 12, rect[1] + rect[3] - pad - card_pill_h(spec) / 2)


def lasso_sides(points, box, inner=2.0, outer=26.0, least=4):
    """Whether drawn pixels (global points) ring BOX: on each of its four
    sides at least LEAST of them lie in a band INNER..OUTER px outside the
    box and across that side's own span."""
    x, y, w, h = box
    n = {"left": 0, "right": 0, "top": 0, "bottom": 0}
    for px, py in points:
        if y <= py <= y + h:
            if x - outer <= px <= x - inner:
                n["left"] += 1
            elif x + w + inner <= px <= x + w + outer:
                n["right"] += 1
        if x <= px <= x + w:
            if y - outer <= py <= y - inner:
                n["top"] += 1
            elif y + h + inner <= py <= y + h + outer:
                n["bottom"] += 1
    return all(v >= least for v in n.values()), n


def box_distance(p, box):
    dx = max(box[0] - p[0], 0.0, p[0] - (box[0] + box[2]))
    dy = max(box[1] - p[1], 0.0, p[1] - (box[1] + box[3]))
    return math.hypot(dx, dy)


def nearest_to_box(points, box):
    return min((box_distance(p, box) for p in points), default=math.inf)


def is_picture(data):
    """Whether DATA starts as a PNG, JPEG, GIF or WebP file does."""
    return data.startswith(IMAGE_MAGICS) or bool(WEBP_RE.match(data[:12]))


def leaks(data, words):
    """What a stored file must never hold: picture bytes, an inline
    picture, or any of WORDS (a window title, typed words, window text) as
    a whole word: a typed number inside a longer number (a float in a
    ledger line) is not it."""
    out = []
    if any(m in data for m in IMAGE_MAGICS) or WEBP_RE.search(data):
        out.append("picture bytes")
    out += [f"inline picture {m.decode()!r}" for m in INLINE_MARKS if m in data]
    low = data.lower()
    for w in words:
        if w and re.search(rb"(?<![0-9a-z])" + re.escape(w.lower().encode()) + rb"(?![0-9a-z])", low):
            out.append(f"word {w!r}")
    return out


def request_texts(rec):
    return "\n".join(t for m in rec.get("messages") or [] for t in m.get("texts") or [])


def canary_after(requests, first_n, word=CANARY):
    """The numbers of requests from FIRST_N on whose text carries WORD."""
    return [r.get("n") for r in requests if (r.get("n") or 0) >= first_n and word in request_texts(r)]


def arrival_violations(frames, arrivals):
    """Arrival-gated order: a guide frame reading step k >= 2 comes only after
    the strand of step k-1's latest target arrived. FRAMES are (wall, guide
    frame data) in order, ARRIVALS strand seq -> wall of its arrival."""
    last_target, bad = {}, []
    for wall, d in frames:
        step, phase = d.get("step") or 0, d.get("phase")
        if d.get("target") and step:
            last_target[step] = d["target"]
        if phase == "reading" and step >= 2:
            seq = last_target.get(step - 1)
            at = arrivals.get(seq) if seq is not None else None
            if seq is None or at is None or at > wall:
                bad.append({"step": step, "reading_at": wall, "previous_target": seq, "arrived_at": at})
    return bad


def plan_texts(frames):
    out = []
    for _, d in frames:
        p = d.get("plan") or {}
        out += list(p.get("done") or []) + ([p["current"]] if p.get("current") else []) + list(p.get("next") or [])
    return out


def per_frame_commits(lines, part):
    """Commits of PART between each guide ledger line and the next one."""
    counts, cur = [], None
    for l in lines:
        if l.get("ev") == "guide":
            if cur is not None:
                counts.append(cur)
            cur = {"seq": l.get("seq"), "phase": l.get("phase"), "commits": 0}
        elif cur is not None and l.get("ev") == "commit" and l.get("part") == part:
            cur["commits"] += 1
    if cur is not None:
        counts.append(cur)
    return counts


def pick_layers(state):
    return [s for s in state.get("layers") or [] if (s.get("namespace") or "") == PICK_NS]


def daemon_hooks(src_dir):
    """The HYAKKI_ONI_GUIDE_* lab hooks the daemon's non-test source names."""
    found = set()
    for path in sorted(Path(src_dir).glob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        found |= set(re.findall(r"HYAKKI_ONI_GUIDE_[A-Z_]+", path.read_text(errors="replace")))
    return sorted(found)


def selftest():
    checks = []

    def check(name, ok, value=None):
        checks.append({"name": name, "pass": bool(ok), "value": value})

    src = ('\tlineDone       = i18n.T("That is it. You are done.")\n'
           '\tkickerReplay   = i18n.T("Kept from before")\n\tx := i18n.Tf("Step %d", n)\n'
           '\tq = i18n.T("say \\"hi\\"")\n')
    lines = go_lines(src)
    check("go_lines reads i18n.T assignments and skips Tf", lines == {"lineDone": "That is it. You are done.",
                                                                      "kickerReplay": "Kept from before",
                                                                      "q": 'say "hi"'}, lines)
    ok = ["tesseract", "stdin", "stdout", "-l", "eng", "--psm", "11", "tsv"]
    check("tesseract: the exact argv passes, also by path and with eng+deu",
          tesseract_allowed(ok) and tesseract_allowed(["/usr/bin/tesseract"] + ok[1:])
          and tesseract_allowed(ok[:4] + ["eng+deu"] + ok[5:]), None)
    bad = {"file input": ["tesseract", "/tmp/a.png"] + ok[2:], "hocr": ok[:-1] + ["hocr"], "extra": ok + ["pdf"],
           "flag as language": ok[:4] + ["--tessdata-dir"] + ok[5:], "path language": ok[:4] + ["../eng"] + ok[5:],
           "psm 3": ok[:6] + ["3"] + ok[7:], "other program": ["tess"] + ok[1:], "shell": ok[:4] + ["eng;rm"] + ok[5:]}
    check("tesseract: every other argv is refused", not any(tesseract_allowed(v) for v in bad.values()),
          [k for k, v in bad.items() if tesseract_allowed(v)])
    check("child_allowed: grim to stdout, tesseract, niri read verbs, whisper-server yes; wtype, sh, grim to a file no",
          child_allowed({"comm": "grim", "argv": ["grim", "-g", "1,2 3x4", "-"]})
          and child_allowed({"comm": "tesseract", "argv": ok})
          and child_allowed({"comm": "niri", "argv": ["niri", "msg", "--json", "windows"]})
          and child_allowed({"comm": "whisper-server", "argv": ["whisper-server", "-m", "x"]})
          and not child_allowed({"comm": "wtype", "argv": ["wtype", "-s", "50", "--", "x"]})
          and not child_allowed({"comm": "sh", "argv": ["sh", "-c", "rm -rf /"]})
          and not child_allowed({"comm": "grim", "argv": ["grim", "/tmp/shot.png"]})
          and not child_allowed({"comm": "grim", "argv": []}), None)
    check("child_allowed: pactl's two source reads only",
          child_allowed({"comm": "pactl", "argv": ["pactl", "-f", "json", "list", "sources"]})
          and child_allowed({"comm": "pactl", "argv": ["/usr/bin/pactl", "get-default-source"]})
          and not child_allowed({"comm": "pactl", "argv": ["pactl", "set-default-source", "x"]})
          and not child_allowed({"comm": "pactl", "argv": ["pactl", "load-module", "module-null-sink"]}), None)
    out = {"x": 1920, "y": 0, "width": 1920, "height": 1080}
    line = {"ev": "strand-target", "seq": 4, "target": {"seq": 4, "box": {"x": 10, "y": 20, "w": 30, "h": 40}, "mode": 1}}
    check("target_box adds the output origin; target_mode reads the int mode",
          target_box(line, out) == [1930, 20, 30, 40] and target_mode(line) == 1 and target_box({}, out) is None, None)
    box = [100, 100, 60, 30]
    check("click_point: the centre when free", click_point(box) == (130.0, 115.0), click_point(box))
    p = click_point(box, [[120, 90, 80, 60]])
    check("click_point: a free point inside the box when the centre is covered",
          p is not None and inside(p, box) and not inside(p, [120, 90, 80, 60]), p)
    check("click_point: None when the box is covered", click_point(box, [[0, 0, 500, 500]]) is None, None)
    spec = {"strand": {"bubble": {"textPx": 13.5}}, "perch": {"chip": {"textPx": 11, "padYPx": 4}}}
    nx = card_next_point([500, 300, 288, 120], spec)
    check("card_next_point: right end of the last row, inside the card", nx == (500 + 288 - 13.5 - 12, 300 + 120 - 13.5 - 11.5)
          and card_pill_h(spec) == 23, nx)
    ring = [(100 + 30 + 40 * math.cos(a / 20), 100 + 15 + 26 * math.sin(a / 20)) for a in range(0, 126)]
    okr, n = lasso_sides(ring, box, outer=26)
    check("lasso_sides: an ellipse round the box rings it", okr, n)
    half = [p for p in ring if p[0] >= 130]
    okh, nh = lasso_sides(half, box, outer=26)
    check("lasso_sides: a half ring does not", not okh, nh)
    check("nearest_to_box: inside 0, outside by distance",
          nearest_to_box([(110, 110)], box) == 0 and abs(nearest_to_box([(163, 100)], box) - 3) < 1e-9
          and nearest_to_box([], box) == math.inf, None)
    data = json.dumps({"key": "dev.ryoku.OniGuide::save-file", "steps": [{"label": "File"}]}).encode()
    check("leaks: a clean keeps file holds none", leaks(data, [GUIDE_TITLE, TYPED_PIN] + list(WINDOW_WORDS)) == [], None)
    dirty = data + b' "Oni guide Lindqvist ledger" 4817 data:image/png;base64,iVBORw0KGgo'
    lk = leaks(dirty, [GUIDE_TITLE, TYPED_PIN])
    check("leaks: title, typed PIN and an inline PNG are each named",
          any("Lindqvist" in x for x in lk) and any("4817" in x for x in lk)
          and any("data:image" in x for x in lk) and any("iVBOR" in x for x in lk), lk)
    check("leaks: raw JPEG bytes", leaks(b"\xff\xd8\xff\xe0rest", []) == ["picture bytes"], None)
    check("leaks: a typed number inside a longer number is not it, alone it is",
          leaks(b'{"scale":0.8599431679481773}', [TYPED_PIN]) == []
          and leaks(b'{"typed":"4817"}', [TYPED_PIN]) == [f"word {TYPED_PIN!r}"], None)
    wav = b"RIFF\x24\x00\x00\x00WAVEfmt "
    check("is_picture: PNG, JPEG, GIF, WebP yes; a WAV no",
          all(is_picture(d) for d in (b"\x89PNG\r\n", b"\xff\xd8\xff\xe0", b"GIF89a", b"RIFF\x10\x00\x00\x00WEBPVP8 "))
          and not is_picture(wav) and leaks(wav, []) == [], None)
    reqs = [{"n": 0, "messages": [{"role": "user", "texts": ["Steps already done: none yet"]}]},
            {"n": 1, "messages": [{"role": "user", "texts": [f"plan was {CANARY} step"]}]},
            {"n": 2, "messages": [{"role": "user", "texts": ["Steps already done, oldest first:\n1. Open File"]}]}]
    check("canary_after: only requests from n on, by their text", canary_after(reqs, 1) == [1] and canary_after(reqs, 2) == [],
          canary_after(reqs, 1))
    frames = [(1.0, {"phase": "reading", "step": 1}), (2.0, {"phase": "pointing", "step": 1, "target": 7}),
              (2.5, {"phase": "waiting", "step": 1, "target": 9}), (4.0, {"phase": "reading", "step": 2})]
    check("arrival_violations: none when step 1's latest strand arrived first",
          arrival_violations(frames, {7: 2.2, 9: 3.0}) == [], None)
    v = arrival_violations(frames, {7: 2.2, 9: 4.5})
    check("arrival_violations: a read before the latest strand arrived is named (the older arrival does not count)",
          len(v) == 1 and v[0]["previous_target"] == 9, v)
    check("arrival_violations: a read with no arrival at all is named", len(arrival_violations(frames, {})) == 1, None)
    check("plan_texts: done, current and next in order",
          plan_texts([(0, {"plan": {"done": ["a"], "current": "b", "next": ["c", "d"]}})]) == ["a", "b", "c", "d"], None)
    led = [{"ev": "guide", "seq": 1, "phase": "reading"}, {"ev": "commit", "part": "card"},
           {"ev": "commit", "part": "strand"}, {"ev": "guide", "seq": 2, "phase": "pointing"},
           {"ev": "commit", "part": "card"}, {"ev": "commit", "part": "card"}]
    pf = per_frame_commits(led, "card")
    check("per_frame_commits counts each guide frame's card commits", [c["commits"] for c in pf] == [1, 2], pf)
    st = {"layers": [{"namespace": "hyakki-oni"}, {"namespace": PICK_NS}, {"namespace": PICK_NS}]}
    check("pick_layers counts only the pick namespace", len(pick_layers(st)) == 2 and pick_layers({"layers": None}) == [], None)
    tmp = Path("/dev/shm") / f"guidekit-selftest-{os.getpid()}"
    tmp.mkdir()
    try:
        (tmp / "a.go").write_text('x := os.Getenv("HYAKKI_ONI_GUIDE_READ_TIMEOUT_MS")\n')
        (tmp / "a_test.go").write_text('"HYAKKI_ONI_GUIDE_TEST_ONLY"\n')
        check("daemon_hooks reads non-test sources only", daemon_hooks(tmp) == [TIMEOUT_HOOK], daemon_hooks(tmp))
    finally:
        for f in tmp.iterdir():
            f.unlink()
        tmp.rmdir()
    okall = all(c["pass"] for c in checks)
    print(json.dumps({"pass": okall, "checks": checks}, indent=2, default=str))
    return 0 if okall else 1


# --- speech fixtures ---------------------------------------------------------

def render_fixtures(dst):
    dst = Path(dst)
    if not str(dst).startswith("/dev/shm/"):
        raise SystemExit("guidekit: DIR must be under /dev/shm")
    dst.mkdir(parents=True, exist_ok=True)
    ver = subprocess.run(["espeak-ng", "--version"], capture_output=True, text=True, check=True).stdout.strip()
    meta = {"engine": ver, "lead_s": LEAD_S, "tail_s": TAIL_S, "fixtures": {}}
    for name, (text, wpm) in GUIDE_FIXTURES.items():
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
    print(f"rendered {len(GUIDE_FIXTURES)} guide speech fixtures into {dst} ({ver})")
    return 0


# --- the gate ----------------------------------------------------------------

class Phase12(Phase11):
    REPORT = "phase12.json"

    def __init__(self, lab, out):
        super().__init__(lab, out)
        for d in ("guidewin", "crops", "answers", "model"):
            (self.out / d).mkdir(parents=True, exist_ok=True)
        import model as lab_model
        self.lab_model = lab_model
        self.gspec = self.spec["guide"]
        self.glines = go_lines((APP / "internal/oni/guide/guide.go").read_text())
        m = re.search(r'RefusalLine\s*=\s*i18n\.T\("([^"]+)"\)', (APP / "internal/oni/screenask/point.go").read_text())
        self.refusal = m.group(1) if m else labkit.PRIVATE_REFUSAL
        self.gfx = json.loads((self.lab / "voice/guide/fixtures.json").read_text())
        self.keeps_path = self.lab / "state/hyakki/oni-keeps.json"
        self.hooks = daemon_hooks(APP / "internal/daemon")
        self.gw_n, self.gwin, self.pick_n = 0, {}, 0
        self.results["guide"] = {
            "spec": self.gspec, "lines": {k: v for k, v in self.glines.items() if k.startswith(("line", "kicker"))},
            "refusal": self.refusal, "daemon_hooks": self.hooks, "window": {"app_id": GUIDE_APP, "title": GUIDE_TITLE},
            "fixtures": {k: v["text"] for k, v in self.gfx["fixtures"].items()},
            "model_note": "model.py's guide mode is scripted on the steps already done: File, Save, then [DONE]; it "
                          "proves routing, consent, OCR-verified trust, strand landing and model-free replay, not "
                          "model quality"}

    def line(self, name):
        return self.glines.get(name, "")

    # --- plumbing ---

    def step(self, number, name, fn):
        try:
            return super().step(number, name, fn)
        finally:
            # a failed step may leave a guide, a pick or a model mode behind
            self.cli("oni", "guide", "stop", "--json", stdin="", timeout=60)
            self.cli("oni", "pick", "cancel", "--json", stdin="", timeout=60)
            try:
                self.model.mode("point")
            except OSError:
                pass

    def gw(self, *args, check=True, timeout=120):
        return self.sh("phase12.sh", "guidewin", *args, check=check, timeout=timeout)

    def model_guide(self, path):
        self.model._req("/_lab/guide", {"state": None if path is None else str(path)})

    def gw_open(self, variant="normal"):
        self.gw_n += 1
        d = self.out / "guidewin" / f"{self.gw_n:02d}-{variant}"
        rec = json.loads(self.gw("open", variant, d).stdout.strip().splitlines()[-1])
        self.gwin = {"variant": variant, "dir": d, "id": rec["id"]}
        self.model_guide(d / "state.json")
        return rec

    def gw_rec(self):
        r = self.gw("get", check=False)
        try:
            return json.loads(r.stdout) if r.returncode == 0 and r.stdout.strip() else None
        except ValueError:
            return None

    def gw_close(self):
        self.gw("close")
        self.gwin = {}

    def gw_state(self, timeout=5.0):
        deadline = time.monotonic() + timeout
        while True:
            try:
                return json.loads((self.gwin["dir"] / "state.json").read_text())
            except (OSError, ValueError, KeyError):
                if time.monotonic() > deadline:
                    raise RuntimeError("the guide window's state.json is unreadable")
                time.sleep(0.1)

    def saved(self):
        try:
            return json.loads((self.gwin["dir"] / "saved.json").read_text())
        except (OSError, ValueError, KeyError):
            return None

    def wait_state(self, pred, timeout=5.0):
        deadline = time.monotonic() + timeout
        while True:
            s = self.gw_state()
            if pred(s):
                return s
            if time.monotonic() > deadline:
                return None
            time.sleep(0.1)

    def ensure_gw(self, variant="normal"):
        """The guide window open in VARIANT on its document page."""
        rec = self.gw_rec()
        if rec is not None and self.gwin.get("variant") != variant:
            self.gw_close()
            rec = None
        if rec is None:
            self.gw_open(variant)
        if self.gw_state()["page"] != "doc":
            self.gw_click("back")
            if self.wait_state(lambda s: s["page"] == "doc") is None:
                raise RuntimeError("the guide window did not return to its document page")
        return self.gw_rec()

    def gbox(self, cid, rec=None, st=None):
        """Control CID's global box, from the compositor's window place and
        the window's own state.json; None when it is not shown."""
        rec, st = rec or self.gw_rec(), st or self.gw_state()
        for c in st["controls"]:
            if c["id"] == cid:
                x, y, w, h = c["box"]
                off = st.get("surface_offset") or [0, 0]
                return [rec["pos"][0] + x - off[0], rec["pos"][1] + y - off[1], w, h]
        return None

    def part_rect(self, part, state=None):
        """An overlay part's (card, chip) global rect while it is shown."""
        last = None
        for l in self.lines():
            if l.get("part") == part and l.get("ev") in ("commit", "hide"):
                last = l
        if last is None or last.get("ev") == "hide" or not last.get("rect"):
            return None
        return global_rect(last, state or self.state())

    def card_rect(self, state=None):
        """The guide card's global rect while it is shown, else None."""
        return self.part_rect("card", state)

    def avoid_rects(self):
        """Oni's sprite, its chip and its card, global, grown 2 px: what a
        click must not land on and a pixel mask must leave out."""
        state = self.state()
        out = []
        try:
            out.append(grow(global_rect(self.current(), state), 2))
        except (RuntimeError, KeyError, TypeError):
            pass
        for part in ("card", "chip"):
            r = self.part_rect(part, state)
            if r:
                out.append(grow(r, 2))
        return out

    def gw_click(self, cid):
        self.gw("focus")
        box = self.gbox(cid)
        if box is None:
            raise RuntimeError(f"the guide window shows no {cid}")
        p = click_point(box, self.avoid_rects())
        if p is None:
            raise RuntimeError(f"{cid} is covered by Oni or its card")
        self.sh("input.sh", "click", round(p[0]), round(p[1]))
        return p

    def click_and_wait(self, cid, pred, timeout=5.0):
        seq = self.gw_state()["seq"]
        self.gw_click(cid)
        return self.wait_state(lambda s: s["seq"] > seq and pred(s), timeout)

    def perch_gw(self):
        rec = self.gw_rec()
        if rec is None:
            raise RuntimeError("the guide window is not open")
        if str(summary(self.lines())["perch"]) == str(rec["id"]) and self.perch_frame():
            return rec
        self.gw("focus")
        mark = self.ledger.mark()
        ok, _, err = self.perch_cli(rec["id"])
        if not ok:
            raise RuntimeError(f"perch on the guide window refused: {err}")
        self.perch_line(mark, 15, rec["id"])
        self.quiet_parts(1.0, 20, ("strand",))
        return rec

    def perch_corner_rect(self, state, r=30):
        """A square round the perch corner, where the curl leaves when the
        strand goes out to a step: kept out of pixel masks."""
        for l in reversed(self.lines()):
            if l.get("ev") == "perch" and l.get("perch"):
                p = l["perch"]
                out = self.output(l.get("output") or (self.gw_rec() or {}).get("output"), state)
                if out is None:
                    return None
                b = p["box"]
                cx = b["x"] + (b["w"] if p.get("corner") == "top-right" else 0)
                return [out["x"] + cx - r, out["y"] + b["y"] - r, 2 * r, 2 * r]
        return None

    def guide(self, action, goal=None, answer="", preview=None, timeout=240, park=True):
        """hyakki oni guide ACTION [GOAL] --json on one connection, answering a
        consent prompt from ANSWER; the exit code, result, consent prompt,
        stderr and wall times."""
        if park:
            self.park_pointer()
        args = ["oni", "guide", action] + ([goal] if goal else []) + (["--preview", str(preview)] if preview else []) + ["--json"]
        t0 = time.time()
        r = self.cli(*args, stdin=answer, timeout=timeout)
        result = None
        if r.returncode == 0:
            try:
                result = json.loads(r.stdout)
            except ValueError:
                result = None
        return {"action": action, "rc": r.returncode, "result": result, "consent": consent_of(r.stderr),
                "stderr": r.stderr.strip()[-1500:], "t0": t0, "t1": time.time()}

    def guide_popen(self, action, goal=None):
        args = [str(HERE / "ctl.sh"), "hyakki", "oni", "guide", action] + ([goal] if goal else []) + ["--json"]
        return subprocess.Popen(args, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)

    def kill_cli(self, word):
        for pid in lab_pids(self.env["LAB_ID"], "hyakki"):
            try:
                if word.encode() in open(f"/proc/{pid}/cmdline", "rb").read():
                    os.kill(pid, signal.SIGKILL)
            except OSError:
                pass

    def keep_cli(self, kid, verb="keep"):
        r = self.cli("oni", verb, kid, "--json", stdin="", timeout=60)
        try:
            value = json.loads(r.stdout) if r.returncode == 0 and r.stdout.strip() else None
        except ValueError:
            value = None
        return {"rc": r.returncode, "result": value, "stderr": r.stderr.strip()[-500:]}

    def keeps(self):
        r = self.cli("oni", "keeps", "--json", stdin="", timeout=60)
        try:
            return (json.loads(r.stdout) or {}).get("keeps") or [] if r.returncode == 0 else None
        except ValueError:
            return None

    def forget_all(self):
        self.cli("oni", "keeps", "--all", "--json", stdin="", timeout=60)

    def pick(self, text=None, cancel=False):
        args = ["oni", "pick"] + (["cancel"] if cancel else []) + (["--text", text] if text else []) + ["--json"]
        r = self.cli(*args, stdin="", timeout=60)
        try:
            value = json.loads(r.stdout) if r.returncode == 0 and r.stdout.strip() else None
        except ValueError:
            value = None
        return {"rc": r.returncode, "result": value, "stderr": r.stderr.strip()[-500:], "t": time.time()}

    def gsettings(self, **guide):
        return self.settings(guide=guide)

    def snapshot_guide(self):
        for frame in self.call("oni.open", {"renderer": "observer"}).get("frames", []):
            if frame.get("kind") == "guide":
                return frame.get("data") or {}
        return None

    def requests(self):
        return self.model_requests()

    def targets(self, mark, perch=False):
        return [l for l in self.ledger.since(mark)
                if l.get("ev") == "strand-target" and (perch or target_mode(l) != MODE_PERCH)]

    def wait_target(self, mark, timeout=30):
        return self.ledger.wait(lambda l: l.get("ev") == "strand-target" and target_mode(l) != MODE_PERCH, mark, timeout)

    def arrivals(self):
        out = {}
        for l in self.lines():
            if l.get("ev") in ("strand-arrive", "strand-skip") and l.get("seq") is not None:
                w = self.clock.to_wall(l.get("t", 0)) if self.clock else None
                out.setdefault(l["seq"], w)
        return out

    def gframes(self, mark):
        return self.watch.frames("guide", mark)

    def wait_gframe(self, mark, pred, timeout):
        return self.watch.wait("guide", pred, mark, timeout, None)

    def wall_of(self, line):
        return self.clock.to_wall(line.get("t", 0)) if self.clock and line else None

    def restart_surface(self):
        """The presenter again, with the chord FIFO (voice.sh keys restarts
        oni-surface with HYAKKI_ONI_KEYS_FIFO, keeping the FIFO)."""
        mark = self.ledger.mark()
        self.sh("voice.sh", "keys")
        self.ledger.wait(lambda l: l.get("ev") == "commit", mark, 15)
        self.wake()
        time.sleep(1.0)

    def wait_idle(self, wm, timeout=None):
        """The guide's idle frame after its closing line: the daemon holds a
        winding or done frame for screenask.Dwell(words), 10 to 60 s."""
        return self.wait_gframe(wm, lambda d: d.get("phase") == "idle", timeout or 70)

    def ended_cleanly(self, lm, wm, t_trigger, name, surface=True, frames=True):
        """After an unwind trigger: the guide frames end, nothing points
        again, the closing card goes once its line was read, and no running
        guide is in the snapshot. With Oni off no frame can go out (FRAMES
        false): the guide is gone with presence."""
        if frames:
            end = self.wait_gframe(wm, lambda d: d.get("phase") in ("idle", "winding"), 20)
            idle = end if (end[1] or {}).get("phase") == "idle" else self.wait_idle(wm)
        else:
            end = idle = (None, None, 0)
        time.sleep(3.0)
        after = [l for l in self.ledger.since(lm) if (self.wall_of(l) or 0) >= t_trigger]
        new_targets = [l for l in after if l.get("ev") == "strand-target" and target_mode(l) != MODE_PERCH]
        card_commits = [l for l in after if l.get("ev") == "commit" and l.get("part") == "card"]
        card_down = any(l.get("ev") == "hide" and l.get("part") == "card" for l in after) or self.card_rect() is None
        snap = self.snapshot_guide()
        gone = snap is None or snap.get("phase") in ("idle", None)
        checks = [
            {"name": f"{name}: the guide frames end (winding, then idle)" if frames
             else f"{name}: the guide went with presence (no frame can go out)",
             "value": [end[1], idle[1]] if frames else snap,
             "pass": end[1] is not None and idle[1] is not None if frames else gone},
            {"name": f"{name}: nothing points again afterwards", "value": new_targets, "pass": not new_targets},
            {"name": f"{name}: no running guide in the observer snapshot", "value": snap, "pass": gone},
        ]
        if surface:
            # the closing frame itself may commit the card; nothing after it
            checks.append({"name": f"{name}: the card goes after the closing line, never committed again",
                           "value": len(card_commits),
                           "pass": card_down and all((self.wall_of(l) or 0) < (end[0] or math.inf) + 0.5 for l in card_commits)})
        return checks

    # --- one walkthrough ---

    def walk(self, goal, name, answer="y\n", controls=("file", "save"), pixels=False, preview=True):
        """Start a guide for GOAL, check each step's landing on CONTROLS,
        click it and press Next; return the record."""
        rec = {"name": name, "goal": goal, "steps": [], "calls": []}
        wm, n0 = self.watch.mark(), len(self.requests())
        rec["saves_before"], rec["wm"] = (self.saved() or {}).get("saves", 0), wm
        base = crop = None
        before = []
        if pixels:
            self.gw("focus")
            self.park_pointer()
            out = self.output(self.gw_rec()["output"], self.state())
            crop = self.region(self.gbox(controls[0]), pad=60, out=out)
            base = self.shoot(f"{name}-base", crop)
            # Oni may fly beside the target: where it and its overlays sat
            # at the baseline is left out of the mask as well
            before = self.avoid_rects()
        lm = lm0 = self.ledger.mark()
        call = self.guide("start", goal, answer=answer,
                          preview=(self.out / "answers" / f"{name}-preview.jpg") if preview else None)
        rec["calls"].append(call)
        for i, cid in enumerate(controls):
            res = call["result"] or {}
            if res.get("status") != "answered" or (res.get("answer") or {}).get("refused"):
                break
            tline = self.wait_target(lm, 30)
            rec["steps"].append(self.landing(cid, tline, lm, f"{name}-{i + 1}", base if i == 0 else None, crop, before))
            page = self.gw_state()["page"]
            saves = (self.saved() or {}).get("saves", 0)
            rec["steps"][-1]["clicked"] = self.click_and_wait(
                cid, lambda s: s["page"] != page or s.get("saves", 0) > saves) is not None
            lm = self.ledger.mark()
            call = self.guide("next", answer=answer)
            rec["calls"].append(call)
        rec["end"] = call
        rec["saved"] = self.saved()
        rec["requests"] = self.requests()[n0:]
        rec["frames"] = self.gframes(wm)
        rec["consents"] = self.watch.frames("consent", wm)
        self.keep_evidence(name, lm0, rec["requests"])
        (self.out / "answers" / f"{name}.json").write_text(json.dumps(rec, indent=2, default=str))
        return rec

    def keep_evidence(self, name, mark, reqs):
        """The surface ledger since MARK as ledgers/NAME.jsonl, and each
        request's picture as the model got it as model/NAME-I.jpg: what a
        step's masks and landing are judged from afterwards."""
        lines = self.ledger.since(mark)
        (self.out / "ledgers" / f"{name}.jsonl").write_text("".join(json.dumps(l) + "\n" for l in lines))
        for i, r in enumerate(reqs):
            if r.get("images"):
                try:
                    (self.out / "model" / f"{name}-{i}.jpg").write_bytes(self.model.image(r["n"], 0))
                except OSError:
                    pass

    def landing(self, cid, tline, mark, name, base=None, crop=None, before=()):
        """Where step CID's strand went: the ledger box on the control (the
        window's own state.json), its arrival, and with a baseline the drawn
        ring or tip in the pixels."""
        state = self.state()
        rec = self.gw_rec()
        out = self.output(rec["output"], state)
        want = self.gbox(cid, rec)
        tb = target_box(tline, out)
        arrive = None
        if tline:
            arrive = self.ledger.wait(lambda l: l.get("ev") in ("strand-arrive", "strand-skip")
                                      and l.get("seq") == tline.get("seq"), mark, 20)
        row = {"control": cid, "want": want, "target": tb, "mode": target_mode(tline), "seq": (tline or {}).get("seq"),
               "arrived": (arrive or {}).get("ev"), "skipped": (arrive or {}).get("skipped")}
        row["geometry"] = bool(tb and want and inside(mid(tb), grow(want, 3)) and close_box(tb, want, 6))
        if base is not None and tline:
            time.sleep(0.6)
            self.park_pointer()
            after = self.shoot(f"{name}-landed", crop)
            exclude = [r for r in [*self.avoid_rects(), *before, self.perch_corner_rect(state)] if r]
            try:
                _, _, pts, _ = self.strand_mask(after, base, crop, exclude=exclude)
                pts = [(float(p[0]), float(p[1])) for p in pts]
            except RuntimeError:
                pts = []
            lasso = self.spec["strand"]["lasso"]
            if row["mode"] in (1, 2):
                outer = lasso["gapPx"] + self.spec["strand"]["stroke"]["widthPx"] + lasso["thrownPx"] + 8
                row["pixels"], row["sides"] = lasso_sides(pts, want, 2.0, outer)
            else:
                bp = self.spec["strand"]["bowedPath"]
                row["nearest_px"] = nearest_to_box(pts, want)
                row["pixels"] = row["nearest_px"] <= bp["tipGapPx"] + bp["tipStopPx"] + 3
            row["crops"] = [str(base), str(after)]
        return row

    def keep_workflow(self, goal=GOAL_SAVE, name="rekeep"):
        """A finished, trusted guide for GOAL kept with oni keep."""
        self.ensure_gw("normal")
        self.perch_gw()
        self.model.mode("point")
        rec = self.walk(goal, name)
        offer = (rec["end"]["result"] or {}).get("keep") or {}
        k = self.keep_cli(offer["id"]) if offer.get("id") else {"rc": None, "result": None}
        rec["keep"] = k
        return rec, bool((k.get("result") or {}).get("kept"))

    def entry(self, goal=GOAL_SAVE):
        for k in self.keeps() or []:
            if k.get("goal") == goal and k.get("app") == GUIDE_APP:
                return k
        return None

    # --- step 0: census ---

    def census12(self):
        checks = []
        g0 = self.oni_settings().get("guide") or {}
        checks.append({"name": "guide, keep and hints are off by default", "value": g0,
                       "pass": not g0.get("enabled") and not g0.get("keep") and not g0.get("hints")})
        voice, _ = self.census11()
        checks += [dict(c, name="voice: " + c["name"]) for c in voice["checks"]]
        bound = json.loads(self.sh("ctl.sh", "globals").stdout)
        (self.out / "globals.json").write_text(json.dumps(bound, indent=2))
        adv = json.loads(self.sh("window.sh", "globals").stdout)["globals"]
        toplevel = bool(adv.get("ext_image_copy_capture_manager_v1") and adv.get("ext_foreign_toplevel_image_capture_source_manager_v1"))
        output = bool(adv.get("zwlr_screencopy_manager_v1") or (adv.get("ext_image_copy_capture_manager_v1")
                                                               and adv.get("ext_output_image_capture_source_manager_v1")))
        langs = subprocess.run(["env", "-i", "PATH=/usr/bin:/bin", "tesseract", "--list-langs"], capture_output=True,
                               text=True, timeout=60)
        gtk = subprocess.run([sys.executable, "-c", 'import gi; gi.require_version("Gtk", "4.0"); from gi.repository import Gtk'],
                             capture_output=True, text=True, timeout=60)
        at_rest = [(p, comm_of(p)) for p in lab_pids(self.env["LAB_ID"])
                   if comm_of(p) in ("grim", "tesseract") + INPUT_PROGRAMS]
        g = self.gspec
        checks += [
            {"name": "the surface binds layer-shell", "value": sorted(bound.get("bound", {})),
             "pass": "zwlr_layer_shell_v1" in bound.get("bound", {})},
            {"name": "a capture path is offered (toplevel or output)", "value": {"toplevel": toplevel, "output": output},
             "pass": toplevel or output},
            {"name": "tesseract with eng on the daemon's PATH (/usr/bin)", "value": langs.stdout.split()[-6:],
             "pass": langs.returncode == 0 and "eng" in langs.stdout.split()},
            {"name": "grim and GTK 4 present", "value": gtk.stderr.strip()[-200:] or None,
             "pass": os.access("/usr/bin/grim", os.X_OK) and gtk.returncode == 0},
            {"name": "no keeps file yet", "value": str(self.keeps_path), "pass": not self.keeps_path.exists()},
            {"name": "no capture, OCR or input program in the lab at rest", "value": at_rest, "pass": not at_rest},
            {"name": "the spec's guide numbers (to be ratified)", "value": {k: g.get(k) for k in ("maxSteps", "readTimeoutS",
                                                                                                    "pickRepeatGuardMs")},
             "pass": g.get("maxSteps") == 12 and g.get("readTimeoutS") == 75 and g["keep"]["chipS"] == 12
             and g["keep"]["maxEntries"] == 80 and g["settle"]["maxFrames"] > 0},
        ]
        # the guide window: the compositor's place, its own state, a capture
        self.forget_all()
        rec = self.gw_open("normal")
        st = self.gw_state()
        self.gw("focus")
        self.park_pointer()
        gb = [round(v) for v in (rec["pos"] + rec["size"])]
        png = self.shoot("census-guidewin", gb)
        data = png.read_bytes()
        from PIL import Image
        pw, ph = Image.open(png).size
        rows = []
        for c in st["controls"]:
            loc = self.lab_model.locate_guide(data, c["id"])
            px = None if loc is None else [loc[0] * gb[2], loc[1] * gb[3], loc[2] * gb[2], loc[3] * gb[3]]
            rows.append({"id": c["id"], "state": c["box"], "capture": px, "pass": close_box(px, c["box"], 2)})
        tsv = subprocess.run(["env", "-i", "PATH=/usr/bin:/bin", "tesseract", "stdin", "stdout", "-l", "eng", "--psm", "11", "tsv"],
                             input=data, capture_output=True, timeout=60)
        words = {ln.split("\t")[-1].strip() for ln in tsv.stdout.decode(errors="replace").splitlines()[1:] if "\t" in ln}
        W, H = st["window"]["width"], st["window"]["height"]
        checks += [
            {"name": "the guide window floats where up.sh's rule puts it", "value": rec,
             "pass": bool(rec.get("floating")) and rec.get("pos") is not None},
            {"name": "its app id and title", "value": [rec.get("app_id"), rec.get("title")],
             "pass": rec.get("app_id") == GUIDE_APP and rec.get("title") == GUIDE_TITLE},
            {"name": "the compositor's size is the window's own (state.json), no surface offset",
             "value": {"compositor": rec.get("size"), "state": [W, H], "offset": st.get("surface_offset")},
             "pass": close_box(rec.get("size"), [W, H], 1) and list(st.get("surface_offset") or [0, 0]) == [0, 0]},
            {"name": "every control box lies inside the window", "value": st["controls"],
             "pass": all(0 <= c["box"][0] and 0 <= c["box"][1] and c["box"][0] + c["box"][2] <= W
                         and c["box"][1] + c["box"][3] <= H for c in st["controls"])},
            {"name": "a grim capture of the compositor's box shows each marker where state.json says (2 px)",
             "value": rows, "pass": bool(rows) and all(r["pass"] for r in rows) and [pw, ph] == gb[2:]},
            {"name": "the text reader reads File and Dead in the capture (OCR-verified steps are possible)",
             "value": sorted(w for w in words if w)[:40], "pass": {"File", "Dead"} <= words},
        ]
        self.settings(inspect=True, disclosure="session", guide={"enabled": True, "keep": False, "hints": False})
        stored = self.oni_settings().get("guide") or {}
        checks.append({"name": "guide on for the run (keep and hints off)", "value": stored,
                       "pass": stored.get("enabled") is True and not stored.get("keep") and not stored.get("hints")})
        return {"window": rec, "state": st, "capture": str(png), "checks": checks}, self.checks_pass(checks)

    # --- step 1: consent first ---

    def consent12(self):
        checks = []
        self.ensure_gw("normal")
        self.perch_gw()
        self.gsettings(enabled=True, keep=False)
        self.settings(inspect=True, disclosure="always")
        self.model.reset()
        self.model.mode("point")
        wm, lm = self.watch.mark(), self.ledger.mark()
        preview = self.out / "answers" / "guide-consent-preview.jpg"
        a = self.guide("start", GOAL_SAVE, answer="n\n", preview=preview)
        time.sleep(1.0)
        frames = self.watch.frames("consent", wm)
        pdata = preview.read_bytes() if preview.exists() else b""
        pw = ph = None
        if pdata:
            import io
            from PIL import Image
            pw, ph = Image.open(io.BytesIO(pdata)).size
        c = a["consent"]
        checks += [
            {"name": "a guide's first picture asks before anything is sent", "value": c, "pass": c["prompted"]},
            {"name": "the prompt names the capture path, and the picture stays on this computer (lab route)",
             "value": [c["path"], c["destination"]], "pass": c["path"] in CAPTURE_PATHS and c["destination"] == "local"},
            {"name": "the ConsentFrame went out with the lab route", "value": frames[-1][1] if frames else None,
             "pass": bool(frames) and frames[-1][1].get("destination") == "local"
             and "oni-fixture-vision" in (frames[-1][1].get("route") or "")},
            {"name": "the preview is a thumbnail of the window's picture (at most 320 px, its shape)",
             "value": [pw, ph, c["width"], c["height"]],
             "pass": bool(pdata) and pdata[:3] == b"\xff\xd8\xff" and bool(c["width"]) and max(pw, ph) <= 320
             and abs(pw / ph - c["width"] / c["height"]) <= 0.03},
            {"name": "no: nothing sent, nothing pointed", "value": [a["rc"], len(self.requests()), len(self.targets(lm))],
             "pass": a["rc"] != 0 and not self.requests() and not self.targets(lm)},
        ]
        # the asker leaves while the question waits
        wm = self.watch.mark()
        p = self.guide_popen("start", GOAL_SAVE)
        card = self.watch.wait("consent", lambda d: True, wm, 60, None)
        self.kill_cli("guide")
        try:
            p.communicate(timeout=20)
        except subprocess.TimeoutExpired:
            p.kill()
        # the guide ends: its closing line is held, then the idle frame
        idle = self.wait_idle(wm)
        time.sleep(1.0)
        snap = self.snapshot_guide()
        checks += [
            {"name": "the asker killed while its card waits: nothing sent", "value": {"card": card[1] is not None,
                                                                                     "requests": len(self.requests())},
             "pass": card[1] is not None and not self.requests()},
            {"name": "and the guide ends: its idle frame, no guide left running in the snapshot",
             "value": {"idle": idle[1], "snapshot": snap},
             "pass": idle[1] is not None and (snap is None or snap.get("phase") == "idle")},
        ]
        # the 120 s expiry, stdin open and silent
        t0 = time.monotonic()
        p = self.guide_popen("start", GOAL_SAVE)
        try:
            p.wait(timeout=200)
        except subprocess.TimeoutExpired:
            p.kill()
            p.wait()
        waited = time.monotonic() - t0
        p.stdin.close()
        said = p.stderr.read().strip()[-200:]
        checks.append({"name": "after the 120 s expiry: dropped, nothing sent",
                       "value": {"waited_s": round(waited, 1), "said": said, "requests": len(self.requests())},
                       "pass": p.returncode != 0 and 115 <= waited <= 190 and not self.requests()})
        # disclosure always: the next step's picture asks again
        self.ensure_gw("normal")
        self.perch_gw()
        s = self.guide("start", GOAL_SAVE, answer="y\n")
        self.click_and_wait("file", lambda st: st["page"] == "menu")
        n = self.guide("next", answer="y\n")
        reqs = self.requests()
        checks += [
            {"name": "disclosure always: step 1 asked and pointed", "value": [s["rc"], s["consent"]["prompted"]],
             "pass": s["rc"] == 0 and s["consent"]["prompted"]},
            {"name": "disclosure always: step 2's picture asked again before it was sent",
             "value": {"prompted": n["consent"]["prompted"], "requests": [r.get("kind") for r in reqs]},
             "pass": n["rc"] == 0 and n["consent"]["prompted"] and len(reqs) == 2},
        ]
        self.guide("stop")
        self.settings(disclosure="session")
        return {"calls": [a, s, n], "checks": checks}, self.checks_pass(checks)

    # --- step 2: the workflow ---

    def workflow(self):
        self.ensure_gw("normal")
        self.perch_gw()
        self.gsettings(enabled=True, keep=False)
        self.settings(inspect=True, disclosure="session")
        self.model.reset()
        self.model.mode("point")
        rec = self.walk(GOAL_SAVE, "workflow", pixels=True)
        reqs = rec["requests"]
        st = self.gw_state()
        W, H = st["window"]["width"], st["window"]["height"]
        imgs = [self.model.image(r["n"], 0) if r.get("images") else b"" for r in reqs]
        lm = self.lab_model
        changed = len(imgs) > 1 and bool(imgs[1]) and lm.locate_guide(imgs[1], "save") is not None \
            and lm.locate_guide(imgs[1], "file") is None
        start, end = rec["calls"][0], rec["end"]
        ans0, endres = (start["result"] or {}).get("answer") or {}, end["result"] or {}
        # the closing card holds its line for screenask.Dwell, then goes
        idle = self.wait_idle(rec["wm"])
        time.sleep(1.0)
        snap_after = self.snapshot_guide()
        frames = rec["frames"]
        phases = [(d.get("phase"), d.get("step")) for _, d in frames]
        seqs = [d.get("seq") for _, d in frames]
        viol = arrival_violations(frames, self.arrivals())
        ledger_guides = [l for l in self.lines() if l.get("ev") == "guide"]
        steps = rec["steps"]
        checks = [
            {"name": "step 1 answered after one yes", "value": [start["rc"], ans0.get("step"), ans0.get("text")],
             "pass": start["rc"] == 0 and ans0.get("step") == 1 and bool(ans0.get("text"))},
            {"name": "three model requests (File, Save, done), each one JPEG of the window's size and no tools",
             "value": [{"kind": r.get("kind"), "images": [(i.get("mime"), i.get("width"), i.get("height")) for i in r["images"]],
                        "tools": r["tools"]} for r in reqs],
             "pass": len(reqs) == 3 and all(len(r["images"]) == 1 and r["images"][0]["mime"] == "image/jpeg"
                                            and close_box([r["images"][0]["width"], r["images"][0]["height"]], [W, H], 1)
                                            and not r["tools"] for r in reqs)},
            {"name": "the window's words reached the model fenced", "value": [r.get("fenced") for r in reqs],
             "pass": bool(reqs) and all(r.get("fenced") is True for r in reqs)},
            {"name": "step 1's strand lands on File (ledger box within 6 px, arrived)", "value": steps[:1],
             "pass": len(steps) >= 1 and steps[0]["geometry"] and steps[0]["arrived"] == "strand-arrive"},
            {"name": "pixel proof: the drawn strand marks File", "value": {k: steps[0].get(k) for k in ("mode", "sides", "nearest_px", "crops")} if steps else None,
             "pass": len(steps) >= 1 and bool(steps[0].get("pixels"))},
            {"name": "the click changed the screen and step 2 read the changed screen (Save shown, File gone)",
             "value": changed, "pass": changed},
            {"name": "step 2's strand lands on Save", "value": steps[1:2],
             "pass": len(steps) >= 2 and steps[1]["geometry"] and steps[1]["arrived"] == "strand-arrive"},
            {"name": "Save clicked: saved.json written by Save", "value": rec["saved"],
             "pass": (rec["saved"] or {}).get("control") == "save" and (rec["saved"] or {}).get("saves", 0) == rec["saves_before"] + 1},
            {"name": "the done line ends the guide: the model's closing sentence (lineDone when it gives none)",
             "value": [endres.get("status"), (endres.get("answer") or {}).get("text")],
             "pass": endres.get("status") == "ended" and (endres.get("answer") or {}).get("text") == self.lab_model.GUIDE_DONE},
            {"name": "keep off: no keep offer", "value": endres.get("keep"), "pass": not endres.get("keep")},
            {"name": "guide frames: rising seqs, reading → pointing → … for steps 1 and 2", "value": phases,
             "pass": seqs == sorted(seqs) and len(set(seqs)) == len(seqs) and ("reading", 1) in phases
             and ("reading", 2) in phases and any(p in ("pointing", "waiting") and s == 2 for p, s in phases)},
            {"name": "arrival-gated: step 2 read only after step 1's strand arrived", "value": viol, "pass": not viol},
            {"name": "the surface took the guide frames (ledger guide lines, words never)",
             "value": len(ledger_guides), "pass": bool(ledger_guides) and not any("text" in l for l in ledger_guides)},
            {"name": "after the closing line: the idle frame, no running guide in the snapshot, the card down",
             "value": {"idle": idle[1], "snapshot": snap_after, "card": self.card_rect()},
             "pass": idle[1] is not None and (snap_after or {}).get("phase") in (None, "idle") and self.card_rect() is None},
        ]
        self.limit(2, "advancing", "a click is not observed (no evdev pointer, no click guess, decision 3): the lab "
                                   "clicks, then presses Next, as the card says")
        return {"record": str(self.out / "answers" / "workflow.json"), "checks": checks}, self.checks_pass(checks)

    # --- step 3: next on an unchanged screen ---

    def unchanged(self):
        self.ensure_gw("normal")
        self.perch_gw()
        self.gsettings(enabled=True, keep=False)
        self.settings(inspect=True, disclosure="session")
        self.model.reset()
        self.model.mode("point")
        lm = self.ledger.mark()
        a = self.guide("start", GOAL_DEAD, answer="y\n")
        t1 = self.wait_target(lm, 30)
        land = self.landing("dead", t1, lm, "dead-1")
        n0 = len(self.requests())
        wm, lm2 = self.watch.mark(), self.ledger.mark()
        b = self.guide("next")
        f1 = self.gframes(wm)
        again = [d for _, d in f1 if (d.get("kicker") or "").endswith("· again")]
        reread = len(self.requests()) - n0
        advanced = [d for _, d in f1 if (d.get("step") or 0) >= 2]
        wm2 = self.watch.mark()
        c = self.guide("next")
        exp = self.wait_gframe(wm2, lambda d: d.get("phase") == "explaining", 20)
        if exp[1] is None and not again:
            # an untrusted step is read again once before it explains
            c = self.guide("next")
            exp = self.wait_gframe(wm2, lambda d: d.get("phase") == "explaining", 20)
        time.sleep(1.0)
        after = self.ledger.since(lm2)
        retracted = [l for l in after if l.get("ev") == "strand-retract" or (l.get("ev") == "hide" and l.get("part") in ("strand", "card"))]
        # the newest strand gates the advance: circle (a new seq), then Next at once
        lm3, wm3 = self.ledger.mark(), self.watch.mark()
        ci = self.guide("circle", park=False)
        t2 = self.wait_target(lm3, 20)
        nx = self.guide("next", park=False)
        time.sleep(1.0)
        arr = self.arrivals()
        new_seq = (t2 or {}).get("seq")
        first_after = next(((w, d) for w, d in self.gframes(wm3) if w >= nx["t0"]), (None, None))
        gated = first_after[0] is None or (arr.get(new_seq) is not None and arr[new_seq] <= first_after[0] + 0.05)
        self.guide("stop")
        checks = [
            {"name": "Dead pointed (step 1)", "value": land, "pass": a["rc"] == 0 and land["geometry"]},
            {"name": "Next on an unchanged screen: re-pointed \"Step 1 · again\" (trusted) or read again (untrusted)",
             "value": {"again": again, "model_reads": reread}, "pass": bool(again) and reread == 0 or (not again and reread == 1)},
            {"name": "it did not advance to a step 2", "value": advanced, "pass": not advanced and b["rc"] == 0},
            {"name": "unchanged twice: the explain line", "value": exp[1],
             "pass": exp[1] is not None and exp[1].get("text") == self.line("lineUnchanged")},
            {"name": "the strand and the card stay out while it explains", "value": retracted, "pass": not retracted},
            {"name": "circle throws a new strand", "value": {"before": (t1 or {}).get("seq"), "after": new_seq},
             "pass": ci["rc"] == 0 and t2 is not None},
            {"name": "an advance waits for the newest strand's arrival, not the superseded one's",
             "value": {"next_at": nx["t0"], "first_frame_after": first_after[0], "arrived": arr.get(new_seq)},
             "pass": gated},
        ]
        if again:
            self.results["guide"]["dead_trusted"] = True
        return {"calls": [a, b, c, ci, nx], "checks": checks}, self.checks_pass(checks)

    # --- step 4: keep ---

    def keep12(self):
        checks = []
        self.forget_all()
        self.ensure_gw("normal")
        self.perch_gw()
        self.gsettings(enabled=True, keep=True)
        self.settings(inspect=True, disclosure="session")
        self.model.reset()
        self.model.mode("point")
        # a typed secret, which must stay in the window
        self.click_and_wait("pin", lambda s: True, 1.0)
        self.sh("input.sh", "type", TYPED_PIN)
        r1 = self.walk(GOAL_SAVE, "keep-lapse")
        end1 = r1["end"]["result"] or {}
        offer = end1.get("keep") or {}
        keeping = [d for _, d in r1["frames"] if d.get("phase") == "keeping"]
        offer_frames = [d for _, d in r1["consents"] if d.get("feature") == "guide.keep"]
        chip = self.gspec["keep"]["chipS"]
        wm = self.watch.mark()
        # the lapse ends keeping: the guide shows its closing line (done), which
        # it then holds for screenask.Dwell before the idle frame
        lapse = self.wait_gframe(wm, lambda d: d.get("phase") in ("done", "idle", "winding"), chip + 8)
        lapsed_at = lapse[0] - r1["end"]["t1"] if lapse[0] else None
        late = self.keep_cli(offer["id"]) if offer.get("id") else {"rc": None}
        checks += [
            {"name": "a trusted finish offers Keep (offer id, question, chip time)", "value": offer,
             "pass": bool(offer.get("id")) and bool(offer.get("question")) and offer.get("expiresMs") == chip * 1000},
            {"name": "the guide shows keeping, and the offer is a local consent frame (guide.keep)",
             "value": {"keeping": len(keeping), "offer": offer_frames[-1] if offer_frames else None},
             "pass": bool(keeping) and bool(offer_frames) and offer_frames[-1].get("destination") == "local"},
            {"name": f"no answer: the offer lapses after the chip's {chip} s", "value": lapsed_at,
             "pass": lapsed_at is not None and lapsed_at <= chip + 4},
            {"name": "the lapse wrote nothing, and a late keep is refused", "value": {"file": self.keeps_path.exists(),
                                                                                       "late_rc": late.get("rc")},
             "pass": not self.keeps_path.exists() and late.get("rc") not in (0, None)},
        ]
        self.ensure_gw("normal")
        r2 = self.walk(GOAL_SAVE, "keep-yes")
        offer2 = (r2["end"]["result"] or {}).get("keep") or {}
        k = self.keep_cli(offer2["id"]) if offer2.get("id") else {"rc": None, "result": None}
        kept = self.keeps() or []
        mode = dmode = None
        data = b""
        if self.keeps_path.exists():
            mode = stat.S_IMODE(self.keeps_path.stat().st_mode)
            dmode = stat.S_IMODE(self.keeps_path.parent.stat().st_mode)
            data = self.keeps_path.read_bytes()
        lk = leaks(data, [GUIDE_TITLE, TYPED_PIN] + list(WINDOW_WORDS))
        e = kept[0] if kept else {}
        checks += [
            {"name": "the second run: oni keep within the chip keeps it", "value": k,
             "pass": k["rc"] == 0 and (k.get("result") or {}).get("kept") is True and (k.get("result") or {}).get("entries") == 1},
            {"name": "one entry for the window's app and the goal, two steps", "value": kept,
             "pass": len(kept) == 1 and e.get("app") == GUIDE_APP and e.get("goal") == GOAL_SAVE and e.get("steps") == 2},
            {"name": "the keeps file is 0600 in a 0700 folder", "value": [oct(mode or 0), oct(dmode or 0)],
             "pass": mode == 0o600 and dmode == 0o700},
            {"name": "it holds no window title, typed words, window text or pixels", "value": lk,
             "pass": bool(data) and not lk},
        ]
        self.limit(4, "the presenter cannot keep", "Keep is the app's, the CLI's or a Press's (decision 4); the card "
                                                    "has no Keep pill, which the surface tests prove")
        return {"runs": [r1["name"], r2["name"]], "checks": checks}, self.checks_pass(checks)

    # --- step 5: replay ---

    def replay12(self):
        self.ensure_gw("normal")
        self.perch_gw()
        self.gsettings(enabled=True, keep=True)
        # always: any picture that would leave would have to ask first
        self.settings(inspect=True, disclosure="always")
        self.model.reset()
        seen0 = set(self.vchildren.seen)
        before = self.entry() or {}
        rec = self.walk(GOAL_SAVE, "replay", answer="", preview=False)
        start, end = rec["calls"][0], rec["end"]
        ans = (start["result"] or {}).get("answer") or {}
        first = next((d for _, d in rec["frames"] if d.get("phase") in ("pointing", "waiting")), {})
        spawned = [v for k, v in self.vchildren.seen.items() if k not in seen0]
        local = sorted({s["comm"] for s in spawned if s["comm"] in ("grim", "tesseract")})
        steps = rec["steps"]
        after = self.entry() or {}
        lat = {"start_s": round(start["t1"] - start["t0"], 3),
               "next_s": [round(c["t1"] - c["t0"], 3) for c in rec["calls"][1:]]}
        self.results["guide"]["replay_latency"] = lat
        checks = [
            {"name": "zero model requests", "value": len(rec["requests"]), "pass": not rec["requests"]},
            {"name": "no consent asked (nothing leaves), disclosure always notwithstanding",
             "value": [c["consent"]["prompted"] for c in rec["calls"]] + [len(rec["consents"])],
             "pass": not any(c["consent"]["prompted"] for c in rec["calls"]) and not [d for _, d in rec["consents"]
                                                                                   if d.get("feature") != "guide.keep"]},
            {"name": "the answer is a replay of step 1", "value": ans, "pass": start["rc"] == 0 and ans.get("replay") is True
             and ans.get("step") == 1},
            {"name": "the replay kicker (kickerReplay) on a replay frame", "value": first,
             "pass": first.get("kicker") == self.line("kickerReplay") and first.get("replay") is True},
            {"name": "the strand lands on File, then Save", "value": steps,
             "pass": len(steps) == 2 and all(s["geometry"] and s["arrived"] == "strand-arrive" for s in steps)},
            {"name": "the replay's closing line (lineReplayDone)", "value": (end["result"] or {}).get("answer"),
             "pass": ((end["result"] or {}).get("answer") or {}).get("text") == self.line("lineReplayDone")},
            {"name": "the window was saved", "value": rec["saved"], "pass": (rec["saved"] or {}).get("saves", 0) == rec["saves_before"] + 1},
            {"name": "the window was read on this computer (capture and the text reader under the daemon)",
             "value": local, "pass": "tesseract" in local or "grim" in local},
            {"name": "the entry was used", "value": {"before": before.get("used"), "after": after.get("used")},
             "pass": bool(after) and after.get("used") != before.get("used")},
        ]
        if lat["start_s"] > 1.5 or any(v > 1.5 for v in lat["next_s"]):
            self.limit(5, "replay latency", f"the persona target is 1.5 s a step; the lab measured {lat}")
        self.settings(disclosure="session")
        return {"latency": lat, "checks": checks}, self.checks_pass(checks)

    # --- step 6: Save against Save As ---

    def saveas(self):
        rows, checks = [], []
        self.ensure_gw("saveas")
        self.perch_gw()
        # replaying a kept guide needs guide.keep; the fall-through is stopped
        # before it finishes, so no new offer comes
        self.gsettings(enabled=True, keep=True)
        self.settings(inspect=True, disclosure="always")
        for attempt in (1, 2):
            self.model.reset()
            self.ensure_gw("saveas")
            lm = self.ledger.mark()
            a = self.guide("start", GOAL_SAVE, answer="")
            ans = (a["result"] or {}).get("answer") or {}
            t1 = self.wait_target(lm, 20)
            land = self.landing("file", t1, lm, f"saveas-{attempt}")
            self.click_and_wait("file", lambda s: s["page"] == "menu")
            lm2 = self.ledger.mark()
            n = self.guide("next", answer="y\n")
            reqs = self.requests()
            first_t = reqs[0]["t"] if reqs else math.inf
            early = [l for l in self.targets(lm2) if (self.wall_of(l) or 0) < first_t]
            e = self.entry()
            self.guide("stop")
            rows.append({"attempt": attempt, "start": ans, "landing": land, "next_prompted": n["consent"]["prompted"],
                         "requests": [{"kind": r.get("kind"), "step": r.get("step"), "label": r.get("label")} for r in reqs],
                         "early_targets": early, "entry": e})
            checks += [
                {"name": f"run {attempt}: step 1 replays onto File", "value": [ans.get("replay"), land["geometry"]],
                 "pass": ans.get("replay") is True and land["geometry"]},
                {"name": f"run {attempt}: Save As is not Save: no strand before the fall-through's model call",
                 "value": early, "pass": not early},
                {"name": f"run {attempt}: the fall-through is a normal consented model read of step 2",
                 "value": rows[-1]["requests"], "pass": n["consent"]["prompted"] and len(reqs) == 1
                 and reqs[0].get("kind") == "guide-step"},
            ]
        checks += [
            {"name": "the first miss is recorded", "value": rows[0]["entry"],
             "pass": bool(rows[0]["entry"]) and rows[0]["entry"].get("misses") == 1},
            {"name": "the second miss retires the entry", "value": rows[1]["entry"], "pass": rows[1]["entry"] is None},
        ]
        self.settings(disclosure="session")
        return {"rows": rows, "checks": checks}, self.checks_pass(checks)

    # --- step 7: sensitive targets ---

    def sensitive12(self):
        self.ensure_gw("normal")
        self.perch_gw()
        self.gsettings(enabled=True, keep=True)
        self.settings(inspect=True, disclosure="session")
        self.model.reset()
        self.model.mode("point")
        lm = self.ledger.mark()
        a = self.guide("start", GOAL_PIN, answer="y\n")
        time.sleep(2.0)
        ans = (a["result"] or {}).get("answer") or {}
        new_reqs = self.requests()
        new_targets = self.targets(lm)
        # the replay path: keep a walkthrough, then open the pin variant
        kept_rec, kept = self.keep_workflow(GOAL_SAVE, "sensitive-keep")
        self.ensure_gw("pin")
        self.perch_gw()
        self.model.reset()
        lm2 = self.ledger.mark()
        b = self.guide("start", GOAL_SAVE, answer="")
        bans = (b["result"] or {}).get("answer") or {}
        self.click_and_wait("file", lambda s: s["page"] == "menu")
        lm3 = self.ledger.mark()
        c = self.guide("next", answer="")
        cres = c["result"] or {}
        cans = cres.get("answer") or {}
        time.sleep(1.5)
        refused_targets = self.targets(lm3)
        replay_reqs = self.requests()
        e = self.entry()
        self.guide("stop")
        checks = [
            {"name": "new path: a PIN goal is refused with the private-field line",
             "value": ans, "pass": bool(ans.get("refused")) or self.refusal in (ans.get("text") or "")},
            {"name": "new path: no strand toward the field", "value": new_targets, "pass": not new_targets},
            {"name": "replay path: a walkthrough was kept first", "value": kept_rec.get("keep"), "pass": kept},
            {"name": "replay path: step 1 replays on the pin variant", "value": bans,
             "pass": bans.get("replay") is True and len(self.targets(lm2)) >= 1},
            {"name": "replay path: the control now labelled PIN is refused with the private-field line",
             "value": cans, "pass": bool(cans.get("refused")) or self.refusal in (cans.get("text") or "")},
            {"name": "replay path: no strand toward it, no model request", "value": {"targets": refused_targets,
                                                                                    "requests": len(replay_reqs)},
             "pass": not refused_targets and not replay_reqs},
            {"name": "replay path: the entry is retired", "value": e, "pass": e is None},
        ]
        self.limit(7, "new-path refusal and the model", f"on the new path the step's label comes from the model's reply "
                                                        f"and the window's words; {len(new_reqs)} model request(s) ran "
                                                        f"before the code refused (the refusal before any model call is "
                                                        f"select-and-ask's, step 10)")
        return {"calls": [a, b, c], "checks": checks}, self.checks_pass(checks)

    # --- step 8: unwind ---

    def start_pointing(self, goal=GOAL_SAVE):
        self.ensure_gw("normal")
        self.perch_gw()
        lm = self.ledger.mark()
        a = self.guide("start", goal, answer="y\n")
        t = self.wait_target(lm, 30)
        if a["rc"] != 0 or t is None:
            raise RuntimeError(f"the guide did not point: {a['stderr']}")
        self.ledger.wait(lambda l: l.get("ev") in ("strand-arrive", "strand-skip") and l.get("seq") == t.get("seq"), lm, 20)
        return a

    def unwind(self):
        checks, rows = [], {}
        self.gsettings(enabled=True, keep=False)
        self.settings(inspect=True, disclosure="session")
        self.model.reset()
        # stop while the step is being read: the late reply changes nothing
        self.ensure_gw("normal")
        self.perch_gw()
        self.model.mode("slow", delay=8)
        lm, wm = self.ledger.mark(), self.watch.mark()
        n0 = len(self.requests())
        p = self.guide_popen("start", GOAL_SAVE)
        p.stdin.write("y\n")
        p.stdin.flush()
        deadline = time.monotonic() + 60
        while len(self.requests()) <= n0 and time.monotonic() < deadline:
            time.sleep(0.1)
        read_seen = len(self.requests()) > n0
        t_stop = time.time()
        s = self.guide("stop")
        try:
            out, err = p.communicate(timeout=60)
        except subprocess.TimeoutExpired:
            p.kill()
            out, err = p.communicate()
        time.sleep(10.0)
        targets = self.targets(lm)
        bubbles = [l for l in self.ledger.since(lm) if l.get("ev") == "bubble" and (self.wall_of(l) or 0) > t_stop]
        checks += [
            {"name": "stop mid-read: the read was under way", "value": read_seen, "pass": read_seen},
            {"name": "stop mid-read: stopped, and the late reply pointed at nothing",
             "value": {"stop": s["rc"], "targets": targets, "bubbles_after": len(bubbles)},
             "pass": s["rc"] == 0 and not targets and not bubbles},
            {"name": "stop mid-read: the asker's call returned (it is not left waiting on the dropped read)",
             "value": {"rc": p.returncode, "out": out[-300:]}, "pass": p.returncode is not None},
        ]
        checks += self.ended_cleanly(lm, wm, t_stop, "stop mid-read")
        self.model.mode("point")
        rows["stop"] = {"stop": s, "asker": {"rc": p.returncode, "stderr": err[-300:]}}

        def case(name, trigger, restore, surface=True, frames=True):
            self.start_pointing()
            lm, wm = self.ledger.mark(), self.watch.mark()
            t = time.time()
            trigger()
            c = self.ended_cleanly(lm, wm, t, name, surface, frames)
            restore()
            return c

        checks += case("detach", self.detach_cli, lambda: None)
        checks += case("window closed", self.gw_close, lambda: None)

        def oni_off():
            mark = self.ledger.mark()
            self.settings(master=False)
            self.ledger.wait(lambda l: l.get("ev") == "exit", mark, 15)

        def oni_on():
            self.settings(master=True)
            self.restart_surface()

        checks += case("Oni off", oni_off, oni_on, surface=False, frames=False)
        checks += case("presenter lost", lambda: self.sh("ctl.sh", "surface", "kill"), self.restart_surface, surface=False)
        # the step ceiling, on a model that never finishes
        self.ensure_gw("normal")
        self.perch_gw()
        self.model.reset()
        self.model.mode("endless")
        lm = lm0 = self.ledger.mark()
        call = self.guide("start", GOAL_SAVE, answer="y\n")
        calls = [call]
        for _ in range(self.gspec["maxSteps"] + 2):
            res = call["result"] or {}
            if res.get("status") != "answered":
                break
            self.wait_target(lm, 20)
            page = self.gw_state()["page"]
            self.click_and_wait("file" if page == "doc" else "back", lambda st: st["page"] != page)
            lm = self.ledger.mark()
            call = self.guide("next", answer="y\n")
            calls.append(call)
        res = call["result"] or {}
        reqs = self.requests()
        self.keep_evidence("ceiling", lm0, reqs)
        self.model.mode("point")
        checks += [
            {"name": f"never-finishing model: the guide ends at {self.gspec['maxSteps']} steps with the ceiling line",
             "value": {"status": res.get("status"), "text": (res.get("answer") or {}).get("text"), "calls": len(calls)},
             "pass": res.get("status") == "ended" and (res.get("answer") or {}).get("text") == self.line("lineCeiling")},
            # one read a step: a second read of a step is an "again" re-point
            # after a Next the settle watch saw change nothing
            {"name": "exactly maxSteps model reads, one a step, no keep offer",
             "value": [len(reqs), res.get("keep"), [r.get("step") for r in reqs]],
             "pass": len(reqs) == self.gspec["maxSteps"] and not res.get("keep")},
        ]
        # the read timeout, through the daemon's lab hook when it serves one
        if TIMEOUT_HOOK in self.hooks:
            self.sh("phase12.sh", "daemon", f"{TIMEOUT_HOOK}={TIMEOUT_HOOK_MS}")
            self.restart_surface()
            try:
                self.ensure_gw("normal")
                self.perch_gw()
                self.model.reset()
                self.model.mode("slow", delay=TIMEOUT_HOOK_MS / 1000 + 10)
                lm, wm = self.ledger.mark(), self.watch.mark()
                a = self.guide("start", GOAL_SAVE, answer="y\n", timeout=120)
                took = a["t1"] - a["t0"]
                time.sleep(TIMEOUT_HOOK_MS / 1000 + 12)
                late = self.targets(lm)
                ares = a["result"] or {}
                checks += [
                    {"name": f"read timeout ({TIMEOUT_HOOK}={TIMEOUT_HOOK_MS}): the timeout line",
                     "value": {"status": ares.get("status"), "text": (ares.get("answer") or {}).get("text"),
                               "rc": a["rc"], "stderr": a["stderr"][-200:], "took_s": round(took, 1)},
                     "pass": (ares.get("answer") or {}).get("text") == self.line("lineTimeout")
                     or self.line("lineTimeout") in a["stderr"]},
                    {"name": "read timeout: the late reply points at nothing", "value": late, "pass": not late},
                ]
            finally:
                self.model.mode("point")
                self.sh("phase12.sh", "daemon")
                self.restart_surface()
        else:
            self.limit(8, "read timeout", f"the daemon serves no {TIMEOUT_HOOK} lab hook (found {self.hooks}); the "
                                          f"{self.gspec['readTimeoutS']} s timeout rests on the guide and daemon unit "
                                          f"tests (fake clock)")
        # a stale consent: the guide stopped elsewhere, then the asker says yes
        self.ensure_gw("normal")
        self.perch_gw()
        self.settings(disclosure="always")
        self.model.reset()
        wm = self.watch.mark()
        p = self.guide_popen("start", GOAL_SAVE)
        card = self.watch.wait("consent", lambda d: True, wm, 60, None)
        stop = self.guide("stop")
        try:
            out, err = p.communicate("y\n", timeout=60)
        except subprocess.TimeoutExpired:
            p.kill()
            out, err = p.communicate()
        time.sleep(1.0)
        checks.append({"name": "a stale consent answer: refused (not_found), nothing sent",
                       "value": {"card": card[1] is not None, "stop": stop["rc"], "rc": p.returncode,
                                 "stderr": err.strip()[-300:], "requests": len(self.requests())},
                       "pass": card[1] is not None and p.returncode != 0 and not self.requests()})
        self.settings(disclosure="session")
        self.limit(8, "Esc", "a layer surface without keyboard focus never hears Esc (decision 11): the card's Stop, "
                             "voice stop, the CLI and detaching wind the guide back")
        return {"rows": rows, "checks": checks}, self.checks_pass(checks)

    # --- step 9: voice first ---

    def ghold(self, name, lead=0.3, tail=0.4):
        """Hold the chord, play guide fixture NAME into the microphone, let go."""
        r = {"fixture": name, "mark": self.watch.mark()}
        r["down"] = self.keys.send("down")
        w, d, _ = self.wait_voice(r["mark"], lambda d: d.get("phase") == "listening", 3.0, "listening")
        r["listening"] = w
        time.sleep(lead)
        t0 = time.time()
        self.sh("audio.sh", "inject", self.gfx["fixtures"][name]["file"], timeout=120)
        r["inject"] = (t0, time.time())
        time.sleep(tail)
        r["up"] = self.keys.send("up")
        return r

    def ginject(self, name):
        self.sh("audio.sh", "inject", self.gfx["fixtures"][name]["file"], timeout=120)

    def voice12(self):
        checks, heard = [], {}
        self.ensure_gw("normal")
        self.perch_gw()
        self.gsettings(enabled=True, keep=False)
        self.settings(inspect=True, disclosure="always", voice=True)
        self.vset(openMic=False, wake={"enabled": False, "phrase": WAKE_PHRASE})
        self.model.reset()
        self.model.mode("point")
        wm, lm = self.watch.mark(), self.ledger.mark()
        r = self.final_of(self.ghold("guide"))
        heard["guide"] = r.get("final")
        card = self.watch.wait("consent", lambda d: True, wm, 60, None)
        started = [d for _, d in self.gframes(wm)]
        # open mic "yes" never answers the card
        mo = self.watch.mark()
        self.vset(openMic=True)
        self.wait_voice(mo, lambda d: d.get("source") == "open", 5, None)
        time.sleep(0.4)
        self.inject("yes")
        time.sleep(3.0)
        open_yes = len(self.requests())
        self.vset(openMic=False)
        self.wait_voice(mo, lambda d: d.get("phase") == "idle", 15, None)
        # a Press's yes answers it
        wy = self.watch.mark()
        heard["yes"] = self.final_of(self.ghold("yes")).get("final")
        p1 = self.wait_gframe(wy, lambda d: d.get("phase") in ("pointing", "waiting") and d.get("step") == 1, 60)
        reqs1 = len(self.requests())
        self.wait_target(lm, 20)
        self.click_and_wait("file", lambda s: s["page"] == "menu")
        # wake and open mic "next" never advance
        ww = self.watch.mark()
        self.vset(wake={"enabled": True, "phrase": WAKE_PHRASE})
        self.wait_voice(ww, lambda d: d.get("phase") == "armed", 8, None)
        time.sleep(1.0)
        self.inject("wake")
        self.wait_voice(ww, lambda d: d.get("phase") == "listening" and d.get("source") == "wake", 10, None)
        self.ginject("next")
        time.sleep(4.0)
        self.vset(wake={"enabled": False, "phrase": WAKE_PHRASE})
        self.wait_voice(ww, lambda d: d.get("phase") == "idle", 30, None)
        mo2 = self.watch.mark()
        self.vset(openMic=True)
        self.wait_voice(mo2, lambda d: d.get("source") == "open", 5, None)
        time.sleep(0.4)
        self.ginject("next")
        time.sleep(4.0)
        self.vset(openMic=False)
        self.wait_voice(mo2, lambda d: d.get("phase") == "idle", 15, None)
        not_advanced = [d for _, d in self.gframes(ww) if (d.get("step") or 0) >= 2]
        reqs_wake = len(self.requests())
        # a Press's next advances; step 2's picture asks again (always), a Press answers
        wn = self.watch.mark()
        heard["next"] = self.final_of(self.ghold("next")).get("final")
        card2 = self.watch.wait("consent", lambda d: True, wn, 60, None)
        if card2[1] is not None:
            heard["yes2"] = self.final_of(self.ghold("yes")).get("final")
        p2 = self.wait_gframe(wn, lambda d: d.get("phase") in ("pointing", "waiting", "reading") and d.get("step") == 2, 60)
        # a Press's stop dismisses
        ws, ls = self.watch.mark(), self.ledger.mark()
        t_stop = time.time()
        heard["stop"] = self.final_of(self.ghold("stop")).get("final")
        checks += [
            {"name": "a Press asking how to save starts a guide on the perched window, consent first",
             "value": {"heard": heard["guide"], "card": card[1], "guide_frames": len(started)},
             "pass": card[1] is not None and bool(started)},
            {"name": "open mic 'yes' never answers the card", "value": open_yes, "pass": open_yes == 0},
            {"name": "a Press's yes answers it: step 1 points", "value": {"heard": heard["yes"], "frame": p1[1], "requests": reqs1},
             "pass": p1[1] is not None and reqs1 == 1},
            {"name": "wake and open mic 'next' never advance", "value": {"step2_frames": not_advanced, "requests": reqs_wake},
             "pass": not not_advanced and reqs_wake == reqs1},
            {"name": "a Press's next advances to step 2", "value": {"heard": heard["next"], "frame": p2[1]},
             "pass": p2[1] is not None},
        ]
        checks += self.ended_cleanly(ls, ws, t_stop, "a Press's stop")
        self.settings(disclosure="session")
        self.limit(9, "speech", "espeak-ng speech into the virtual microphone proves the routing, not recognition of "
                                f"human speech; heard {heard}")
        return {"heard": heard, "checks": checks}, self.checks_pass(checks)

    # --- step 10: select-and-ask ---

    def pick_start(self, text=PICK_QUESTION, answer="y\n"):
        """hyakki oni pick [--text TEXT] --json, which keeps its connection
        after arming and answers the click's consent from ANSWER on stdin as
        oni ask does; returns once it says it is armed."""
        args = [str(HERE / "ctl.sh"), "hyakki", "oni", "pick"] + (["--text", text] if text else []) + ["--json"]
        p = subprocess.Popen(args, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        p.stdin.write(answer)
        p.stdin.close()
        head, deadline = [], time.time() + 30
        while time.time() < deadline:
            line = p.stderr.readline()
            if not line:
                break
            head.append(line)
            if "Click the spot to ask about." in line:
                break
        return {"proc": p, "head": "".join(head), "args": args[2:], "t": time.time()}

    def pick_end(self, run, timeout=150):
        """What the waiting pick came to: exit code, result and consent."""
        p = run["proc"]
        try:
            out, err = p.communicate(timeout=timeout)
        except subprocess.TimeoutExpired:
            p.kill()
            out, err = p.communicate()
        err = run["head"] + err
        self.log.write(f"{now()} {' '.join(run['args'])} -> {p.returncode}\n{out}{err}\n")
        try:
            value = json.loads(out) if p.returncode == 0 and out.strip() else None
        except ValueError:
            value = None
        return {"rc": p.returncode, "result": value, "consent": consent_of(err), "stderr": err.strip()[-1500:]}

    def pick_point(self, cid, text=PICK_QUESTION, answer="y\n"):
        """Arm with a waiting oni pick, click CID, and return what came of
        it, the pick's own consent answered from ANSWER."""
        lm, wm, n0 = self.ledger.mark(), self.watch.mark(), len(self.requests())
        run = self.pick_start(text, answer)
        armed = self.ledger.wait(lambda l: l.get("ev") == "pick" and l.get("reason") == "armed", lm, 10)
        mapped = len(pick_layers(self.state()))
        p = self.gw_click(cid)
        point = self.ledger.wait(lambda l: l.get("ev") == "pick" and l.get("reason") == "point", lm, 10)
        cli = self.pick_end(run)
        bubble = self.watch.wait("bubble", lambda d: bool(d.get("text")), wm, 90, None)
        time.sleep(1.0)
        reqs = self.requests()[n0:]
        self.pick_n += 1
        self.keep_evidence(f"pick-{self.pick_n}-{cid}", lm, reqs)
        return {"cli": cli, "armed": armed, "mapped": mapped, "click": p, "point": point, "bubble": bubble[1],
                "requests": [{"kind": r.get("kind"), "hit": r.get("hit"), "located": r.get("located")} for r in reqs],
                "targets": self.targets(lm), "mapped_after": len(pick_layers(self.state())), "lm": lm, "wm": wm}

    def select(self):
        checks, rows = [], {}
        self.ensure_gw("normal")
        self.perch_gw()
        self.gsettings(enabled=True, keep=False)
        # disclosure every time: each pick asks about its picture as oni.ask
        # does, and the waiting `hyakki oni pick` answers on its own stdin
        self.settings(inspect=True, disclosure="always")
        self.model.reset()
        self.model.mode("point")
        outs = [o["name"] for o in self.state()["outputs"] if o["enabled"]]
        rest = len(pick_layers(self.state()))
        self.click_and_wait("file", lambda s: s["page"] == "menu")
        r = self.pick_point("export")
        rows["export"] = {k: v for k, v in r.items() if k not in ("lm", "wm")}
        out = self.output(self.gw_rec()["output"], self.state())
        exp = self.gbox("export")
        tb = target_box(r["targets"][0], out) if r["targets"] else None
        cli = r["cli"]
        checks += [
            {"name": "nothing is mapped for picking at rest", "value": rest, "pass": rest == 0},
            {"name": "oni pick arms the overlay: one surface per output", "value": {"ledger": r["armed"], "layers": r["mapped"],
                                                                                    "outputs": outs},
             "pass": r["armed"] is not None
             and sorted(r["armed"].get("outputs") or []) == sorted(outs) and r["mapped"] == len(outs)},
            {"name": "the pick asks its picture's consent on the waiting oni pick's stdin, which answers it",
             "value": {"rc": cli["rc"], "consent": cli["consent"], "status": (cli["result"] or {}).get("status"),
                       "answer": ((cli["result"] or {}).get("answer") or {}).get("text")},
             "pass": cli["rc"] == 0 and cli["consent"]["prompted"] and cli["consent"]["path"] is not None
             and (cli["result"] or {}).get("status") == "answered"
             and (((cli["result"] or {}).get("answer") or {}).get("text") or "").startswith("That is Export")},
            {"name": "the click is the pick: the overlay comes down at once", "value": [r["point"], r["mapped_after"]],
             "pass": r["point"] is not None and not (r["point"].get("outputs") or []) and r["mapped_after"] == 0},
            {"name": "one select request, on Export", "value": r["requests"],
             "pass": len(r["requests"]) == 1 and r["requests"][0]["kind"] == "select" and r["requests"][0]["hit"] == "export"},
            {"name": "the answer shows beside Oni (bubble)", "value": r["bubble"],
             "pass": bool(r["bubble"]) and (r["bubble"].get("text") or "").startswith("That is Export")},
            {"name": "the strand marks Export", "value": {"target": tb, "export": exp},
             "pass": tb is not None and exp is not None and inside(mid(tb), grow(exp, 3))},
        ]
        # Walk me through it: the nudge's Next starts a guide on the question
        nudge = self.wait_gframe(r["wm"], lambda d: d.get("phase") == "nudging" and "next" in (d.get("actions") or []), 10)
        n0 = len(self.requests())
        wk = self.guide("next", answer="y\n") if nudge[1] is not None else None
        walk_reqs = self.requests()[n0:]
        checks.append({"name": "Walk me through it: the nudge's Next starts a guide on the pick question",
                       "value": {"nudge": nudge[1], "next": (wk or {}).get("result"),
                                 "requests": [(q.get("kind"), q.get("request")) for q in walk_reqs]},
                       "pass": nudge[1] is not None and any((q.get("kind") or "").startswith("guide")
                                                           and PICK_QUESTION.lower() in (q.get("request") or "").lower()
                                                           for q in walk_reqs)})
        self.guide("stop")
        # the other output (Hyprland)
        if self.wm == "hyprland" and len(outs) > 1:
            home = self.gw_rec()["output"]
            other = next(o for o in outs if o != home)
            self.gw("output", other)
            time.sleep(0.5)
            self.perch_gw()
            if self.gw_state()["page"] != "menu":
                self.click_and_wait("file", lambda s: s["page"] == "menu")
            self.model.reset()
            r2 = self.pick_point("export")
            rows["other_output"] = {k: v for k, v in r2.items() if k not in ("lm", "wm")}
            checks.append({"name": f"a pick on the second output ({other}) answers about Export there",
                           "value": {"requests": r2["requests"], "bubble": r2["bubble"]},
                           "pass": len(r2["requests"]) == 1 and r2["requests"][0]["hit"] == "export" and bool(r2["bubble"])})
            self.guide("stop")
            self.gw("output", home)
            time.sleep(0.5)
            self.perch_gw()
        else:
            self.limit(10, "a pick on another output", "nested niri has one output; Hyprland's run picks on both")
        # the PIN: refused before any model call
        self.ensure_gw("normal")
        self.model.reset()
        rp = self.pick_point("pin")
        rows["pin"] = {k: v for k, v in rp.items() if k not in ("lm", "wm")}
        checks.append({"name": "a pick on the PIN entry: the private-field line, no model request, no strand",
                       "value": {"bubble": rp["bubble"], "requests": rp["requests"], "targets": rp["targets"]},
                       "pass": not rp["requests"] and not rp["targets"] and self.refusal in ((rp["bubble"] or {}).get("text") or "")})
        # the repeat guard
        self.click_and_wait("file", lambda s: s["page"] == "menu")
        self.model.reset()
        lm = self.ledger.mark()
        # the second arm and click as fast as the lab's tools go: one click
        # point, no focus round trip
        spot = click_point(self.gbox("export"), self.avoid_rects())
        first = self.pick_start(PICK_QUESTION)
        self.sh("input.sh", "click", round(spot[0]), round(spot[1]))
        second = self.pick_start(PICK_QUESTION)
        self.sh("input.sh", "click", round(spot[0]), round(spot[1]))
        time.sleep(8.0)
        ends = [self.pick_end(first), self.pick_end(second)]
        points = [l for l in self.ledger.since(lm) if l.get("ev") == "pick" and l.get("reason") == "point"]
        gap = points[1]["t"] - points[0]["t"] if len(points) >= 2 else None
        sel = [q for q in self.requests() if q.get("kind") == "select"]
        guard = self.gspec["pickRepeatGuardMs"] / 1000
        if gap is not None and gap < guard:
            checks.append({"name": f"a second pick {gap:.2f} s after the first (guard {guard} s) is ignored",
                           "value": {"points": len(points), "selects": len(sel), "rcs": [e["rc"] for e in ends]},
                           "pass": len(sel) == 1 and ends[1]["rc"] == 1 and "No spot was picked" in ends[1]["stderr"]})
        else:
            self.limit(10, "repeat guard", f"the lab could not land a second pick inside {guard} s (gap {gap}); "
                                           f"the guard rests on the daemon's pick tests")
        self.cli("oni", "pick", "cancel", "--json", stdin="")
        # cancel unmaps; the CLI's cancel reaches the presenter as the
        # daemon's disarm (the surface's own cancel reason is its right click)
        lm = self.ledger.mark()
        waiting = self.pick_start(PICK_QUESTION, answer="")
        self.ledger.wait(lambda l: l.get("ev") == "pick" and l.get("reason") == "armed", lm, 10)
        c = self.pick(cancel=True)
        cl = self.ledger.wait(lambda l: l.get("ev") == "pick" and l.get("reason") == "daemon", lm, 10)
        ended = self.pick_end(waiting, timeout=30)
        time.sleep(0.5)
        left = len(pick_layers(self.state()))
        checks.append({"name": "cancel disarms: nothing mapped, nothing asked, the waiting pick ends with nothing sent",
                       "value": {"rc": c["rc"], "line": cl, "layers": left, "waiting": {"rc": ended["rc"], "stderr": ended["stderr"][-200:]}},
                       "pass": c["rc"] == 0 and cl is not None and left == 0 and ended["rc"] == 1
                       and not ended["consent"]["prompted"] and "No spot was picked" in ended["stderr"]})
        self.settings(disclosure="session")
        self.limit(10, "pick cursor", "the pick overlay sets no cursor shape; the compositor keeps its own")
        return {"rows": rows, "checks": checks}, self.checks_pass(checks)

    # --- step 11: hints ---

    def hints(self):
        checks = []
        self.forget_all()
        self.gsettings(enabled=True, keep=True, hints=False)
        self.settings(inspect=True, disclosure="session")
        self.model.reset()
        self.model.mode("point")
        _, k1 = self.keep_workflow(GOAL_SAVE, "hints-keep-1")
        _, k2 = self.keep_workflow(GOAL_SAVE2, "hints-keep-2")
        checks.append({"name": "two walkthroughs kept for the window's app", "value": self.keeps(), "pass": k1 and k2})

        def reperch(wait):
            self.ensure_detached()
            wm = self.watch.mark()
            self.perch_gw()
            nudge = self.wait_gframe(wm, lambda d: d.get("phase") == "nudging", wait)
            hint = self.watch.wait("consent", lambda d: d.get("feature") == "guide.hints", wm, 2, None)
            return wm, nudge, hint

        _, off, offh = reperch(8)
        caps = [p["comm"] for p in self.vchildren.alive(("grim", "tesseract"))]
        checks.append({"name": "hints off: no hint on perching, nothing running", "value": {"nudge": off[1], "hint": offh[1], "running": caps},
                       "pass": off[1] is None and offh[1] is None and not caps})
        self.gsettings(hints=True)
        wm1, n1, h1 = reperch(15)
        key1 = self.hinted_last()
        checks.append({"name": "hints on: perching a window with kept goals shows one hint",
                       "value": {"nudge": n1[1], "hint": h1[1], "key": key1}, "pass": n1[1] is not None and bool(key1)})
        if h1[1] is not None:
            sk = self.keep_cli(h1[1]["id"], verb="skip")
        else:
            sk = self.guide("stop")
        snooze = self.wait_gframe(wm1, lambda d: d.get("phase") in ("idle", "winding"), 10)
        checks.append({"name": "Not now (Skip) puts the hint away", "value": {"skip": sk.get("rc"), "end": snooze[1]},
                       "pass": sk.get("rc") == 0 and snooze[1] is not None})
        # the cooldown and the day are not waited out: the lab's own hint
        # record stands in for them (cooldown: its last-hint time dropped; a
        # new day: the record removed), and every rule is read from it
        self.hints_edit(cooldown=True)
        _, n2, _ = reperch(10)
        checks.append({"name": "Not now holds for the day: past the cooldown, still no hint for the app",
                       "value": n2[1], "pass": n2[1] is None})
        self.hints_edit(new_day=True)
        wm3, n3, _ = reperch(15)
        key3 = self.hinted_last()
        retract_s = self.gspec["hints"]["retractS"]
        gone = self.wait_gframe(wm3, lambda d: d.get("phase") in ("idle", "winding"), retract_s + 15) if n3[1] is not None else (None, None, 0)
        held = None if not (n3[0] and gone[0]) else round(gone[0] - n3[0], 1)
        checks.append({"name": f"a new day: a hint again, untouched, retracts at {retract_s} s",
                       "value": {"nudge": n3[1], "key": key3, "held_s": held},
                       "pass": n3[1] is not None and held is not None and retract_s - 2 <= held <= retract_s + 5})
        self.hints_edit(cooldown=True)
        _, n4, _ = reperch(15)
        key4 = self.hinted_last()
        if n4[1] is not None:
            self.guide("stop")
        checks.append({"name": "past the cooldown: the other kept goal's hint",
                       "value": {"nudge": n4[1], "keys": [key3, key4]},
                       "pass": n4[1] is not None and bool(key4) and key4 != key3})
        self.hints_edit(cooldown=True)
        _, n5, _ = reperch(10)
        checks.append({"name": "once per key per day: past the cooldown again, no third hint", "value": n5[1],
                       "pass": n5[1] is None})
        self.gsettings(hints=False)
        self.limit(11, "hint cooldown and day", f"the {self.gspec['hints']['cooldownS']} s cooldown and the calendar "
                                                "day are not waited out and no clock hook is served: the lab drops the "
                                                "hint record's last-hint time or removes the record to stand in for "
                                                "them; the timing itself rests on the daemon's hint tests")
        return {"checks": checks}, self.checks_pass(checks)

    @property
    def hints_path(self):
        return self.keeps_path.with_name("oni-hints.json")

    def hinted_last(self):
        """The key the lab's hint record says was hinted last, or None."""
        try:
            shown = json.loads(self.hints_path.read_text()).get("shown") or []
        except (OSError, ValueError):
            return None
        return max(shown, key=lambda m: m.get("at") or "").get("key") if shown else None

    def hints_edit(self, cooldown=False, new_day=False):
        """Stand in for time passing in the lab's hint record: COOLDOWN drops
        the last-hint time, NEW_DAY removes the record."""
        p = self.hints_path
        if new_day:
            p.unlink(missing_ok=True)
            return
        try:
            rec = json.loads(p.read_text())
        except (OSError, ValueError):
            return
        if cooldown:
            rec.pop("last", None)
        tmp = p.with_name(p.name + ".lab")
        tmp.write_text(json.dumps(rec))
        os.chmod(tmp, 0o600)
        os.replace(tmp, p)

    # --- step 12: injection ---

    def injection12(self):
        self.ensure_gw("normal")
        self.perch_gw()
        self.gsettings(enabled=True, keep=False, hints=False)
        self.settings(inspect=True, disclosure="session")
        self.model.reset()
        self.model.mode("hostile")
        seen0 = set(self.vchildren.seen)
        wm, lm = self.watch.mark(), self.ledger.mark()
        a = self.guide("start", GOAL_SAVE, answer="y\n")
        time.sleep(2.0)
        r1 = self.requests()
        plan = plan_texts(self.gframes(wm))
        self.model.mode("point")
        self.click_and_wait("file", lambda s: s["page"] == "menu")
        b = self.guide("next", answer="y\n")
        reqs = self.requests()
        fed = canary_after(reqs, 1)
        self.guide("stop")
        # a hostile select answer carries an action tag: refused whole
        self.model.reset()
        self.model.mode("hostile")
        rs = self.pick_point("export")
        self.model.mode("point")
        spawned = [v for k, v in self.vchildren.seen.items() if k not in seen0]
        inputs = [s for s in spawned if s["comm"] in INPUT_PROGRAMS]
        btext = (rs["bubble"] or {}).get("text") or ""
        (self.out / "answers" / "injection.json").write_text(json.dumps({"start": a, "next": b, "requests": reqs,
                                                                         "select": {k: v for k, v in rs.items() if k not in ("lm", "wm")}},
                                                                        indent=2, default=str))
        checks = [
            {"name": "one model call for the hostile step, no tool offered", "value": [(r.get("kind"), r["tools"]) for r in r1],
             "pass": len(r1) == 1 and not r1[0]["tools"]},
            {"name": "the plan never names the private field or the typed number", "value": plan,
             "pass": not any(HOSTILE_PIN in t or "PIN" in t for t in plan)},
            {"name": f"the plan never reaches a later request (canary {CANARY})", "value": {"requests": len(reqs), "carrying": fed},
             "pass": len(reqs) >= 2 and not fed},
            {"name": "a hostile select answer with an action tag is refused whole", "value": {"bubble": btext, "targets": rs["targets"]},
             "pass": "[CLICK" not in btext and "PWNED" not in btext and not rs["targets"]},
            {"name": "nothing but capture, the text reader and compositor reads ran", "value": spawned,
             "pass": all(child_allowed(s) for s in spawned) and not inputs},
        ]
        return {"checks": checks}, self.checks_pass(checks)

    # --- step 13: read-only, boundaries, disk ---

    def readonly12(self):
        hits, by_pkg = [], {p: [] for p in GUIDE_PKGS}
        for root in (APP / "internal/oni", APP / "internal/dictate", APP.parent / "wm/oniwindow"):
            for path in sorted(root.rglob("*.go")):
                if path.name.endswith("_test.go") or (APP / "internal/dictate/insert") in path.parents:
                    continue
                rel = path.relative_to(APP / "internal/oni").parts[0] if (APP / "internal/oni") in path.parents else None
                for n, line in enumerate(path.read_text(errors="replace").splitlines(), 1):
                    low = line.lower()
                    for mark in labkit.INPUT_MARKS:
                        if mark in low:
                            hit = {"file": str(path.relative_to(ROOT)), "line": n, "mark": mark}
                            hits.append(hit)
                            if rel in by_pkg:
                                by_pkg[rel].append(hit)
        tests = {}
        for pkg, rel, run in TEST_PKGS:
            binary = os.environ.get(f"ONI_LAB_{pkg.upper()}_TEST") or str(Path(self.env.get("HYAKKI_BIN", "/")) / f"{pkg}.test")
            if not os.access(binary, os.X_OK):
                tests[pkg] = {"ran": False, "pass": False, "error": f"no {pkg}.test (go test -c of {rel}) in HYAKKI_BIN"}
                continue
            r = subprocess.run(["systemd-run", "--user", "--scope", "--quiet", "--collect",
                                f"--unit=oni-lab-{self.env['LAB_ID']}-{pkg}test-{os.urandom(4).hex()}",
                                "-p", "MemoryMax=1G", "-p", "MemorySwapMax=0", "--",
                                "env", "-i", "PATH=/usr/bin:/bin", binary, "-test.run", run, "-test.v", "-test.count=1"],
                               capture_output=True, text=True, timeout=600, cwd=APP / rel)
            outp = (r.stdout + r.stderr).strip()
            tests[pkg] = {"ran": True, "rc": r.returncode, "output": outp[-1500:],
                          "pass": r.returncode == 0 and "--- PASS" in outp and "--- FAIL" not in outp}
        seen = list(self.vchildren.seen.values())
        bad = [s for s in seen if not child_allowed(s)]
        tess = [s for s in seen if s["comm"] == "tesseract"]
        inputs = [s for s in seen if s["comm"] in INPUT_PROGRAMS]
        inputs += [{"pid": p, "comm": c} for p, c in self.vchildren.unread.items() if c in INPUT_PROGRAMS]
        # pictures and window text never land in the lab
        start = self.start_wall - 1
        images, inline, words, scanned = [], [], [], 0
        skip = [str(self.lab / "voice"), str(self.lab / "log/model-requests.jsonl"), str(self.lab / "log/model.log"),
                str(self.lab / "log/guidewin.log")]
        for dirpath, _, filenames in os.walk(self.lab):
            for name in filenames:
                path = os.path.join(dirpath, name)
                if any(path == s or path.startswith(s + os.sep) for s in skip):
                    continue
                try:
                    st = os.lstat(path)
                    if not stat.S_ISREG(st.st_mode) or st.st_mtime < start or st.st_size == 0 or st.st_size > 32 << 20:
                        continue
                    scanned += 1
                    data = Path(path).read_bytes()
                except OSError:
                    continue
                found = leaks(data, list(WINDOW_WORDS) + [TYPED_PIN])
                if is_picture(data):
                    images.append(path)
                inline += [path for f in found if f.startswith("inline")][:1]
                words += [{"file": path, "found": f} for f in found if f.startswith("word")]
        tmp = self.lab / "tmp"
        left = sorted(str(p.relative_to(tmp)) for p in tmp.rglob("*")) if tmp.is_dir() else None
        checks = [
            {"name": "no input-executor name in internal/oni or internal/dictate (outside insert)", "value": hits, "pass": not hits},
            {"name": "guide, keep, ocr and settle name none", "value": by_pkg, "pass": not any(by_pkg.values())},
        ]
        checks += [{"name": f"{pkg}.test ({rel})", "value": tests.get(pkg), "pass": bool(tests.get(pkg, {}).get("pass"))}
                   for pkg, rel, _ in TEST_PKGS]
        checks += [
            {"name": "every program the daemon ran is on its allowlist (argv read before it exited)",
             "value": {"seen": len(seen), "outside": bad, "exited_unread": len(self.vchildren.unread)},
             "pass": bool(seen) and not bad},
            {"name": "the text reader ran, always as `tesseract stdin stdout -l LANG --psm 11 tsv`",
             "value": [s["argv"] for s in tess][:5], "pass": bool(tess) and all(tesseract_allowed(s["argv"]) for s in tess)},
            {"name": "the daemon started no input program", "value": inputs, "pass": not inputs},
            {"name": "no picture file written in the lab", "value": images, "pass": not images},
            {"name": "no inline picture in the lab's files", "value": inline, "pass": not inline},
            {"name": "no window text or typed words in the lab's files (the model's own log aside)", "value": words,
             "pass": not words},
            {"name": "the lab processes' temp dir is empty", "value": left, "pass": left == []},
        ]
        return {"files_scanned": scanned, "tests": tests, "checks": checks}, self.checks_pass(checks)

    # --- step 14: reduced motion ---

    def reduced12(self):
        before = self.oni_settings().get("reduceMotion", "follow")
        self.settings(reduceMotion="on")
        try:
            self.ensure_gw("normal")
            self.perch_gw()
            self.gsettings(enabled=True, keep=False)
            self.settings(inspect=True, disclosure="session")
            self.model.reset()
            self.model.mode("point")
            lm = self.ledger.mark()
            a = self.guide("start", GOAL_SAVE, answer="y\n")
            time.sleep(2.5)
            lines = self.ledger.since(lm)
            target = next((l for l in lines if l.get("ev") == "strand-target" and target_mode(l) != MODE_PERCH), None)
            arrive = next((l for l in lines if l.get("ev") == "strand-arrive" and target
                           and l.get("seq") == target.get("seq")), None)
            card = per_frame_commits(lines, "card")
            strands = [l for l in lines if l.get("ev") == "commit" and l.get("part") == "strand"
                       and target and (l.get("strand") or {}).get("seq") == target.get("seq")]
            self.guide("stop")
        finally:
            self.settings(reduceMotion=before)
        checks = [
            {"name": "the guide pointed", "value": a["rc"], "pass": a["rc"] == 0 and target is not None},
            {"name": "the strand arrives at once", "value": None if not (target and arrive) else round(arrive["t"] - target["t"], 4),
             "pass": target is not None and arrive is not None and arrive["t"] - target["t"] <= 0.05},
            {"name": "at most one card commit per guide frame", "value": card,
             "pass": bool(card) and all(c["commits"] <= 1 for c in card)},
            {"name": "one strand commit for the step's target", "value": len(strands), "pass": len(strands) <= 1},
        ]
        return {"checks": checks}, self.checks_pass(checks)

    # --- step 15: settlement ---

    def settlement12(self):
        self.ensure_gw("normal")
        self.perch_gw()
        self.gsettings(enabled=True, keep=False, hints=False)
        self.settings(inspect=True, disclosure="session")
        sampler = None
        if self.wm == "hyprland":
            # Hyprland publishes no move event, so while Oni is awake the
            # floating guide window's perch is sampled with backoff (Phase
            # 10's recorded limit). The sampler's cost is measured here, with
            # no guide and the same settling, so the guide rows gate the
            # guide's own daemon wakeups over it.
            self.wake()
            self.quiet_parts(2.0, 40)
            self.silent(2.0, 60)
            sampler = self.sampler_cost(self.measure("perched", timeout=150, seconds=30, name="perched-floating", gate=False))
        self.model.reset()
        self.model.mode("point")
        self.start_pointing()
        self.quiet_parts(2.0, 40)
        self.silent(2.0, 60)
        t0 = time.time()
        waiting = self.own_cost("guide-waiting", sampler)
        t1 = time.time()
        snap = self.snapshot_guide()
        caps = [p["comm"] for p in self.vchildren.procs(("grim", "tesseract"), since=t0, until=t1)]
        # the settle watch after Next is bounded
        self.click_and_wait("file", lambda s: s["page"] == "menu")
        tn0 = time.time()
        n = self.guide("next", answer="y\n")
        grims = [p for p in self.vchildren.procs(("grim",)) if tn0 <= p["first"] <= n["t1"]]
        frames_max = self.gspec["settle"]["maxFrames"]
        self.guide("stop")
        self.quiet_parts(2.0, 40)
        self.silent(2.0, 60)
        e0 = time.time()
        ended = self.own_cost("guide-ended", sampler)
        e1 = time.time()
        caps_e = [p["comm"] for p in self.vchildren.procs(("grim", "tesseract"), since=e0, until=e1)]
        snap_e = self.snapshot_guide()
        checks = [
            {"name": "guide waiting (a step out, the user not acting): 0 commits, 0 armed callbacks, within budget",
             "value": waiting.get("ledger_window"), "pass": bool(waiting.get("pass"))},
            {"name": "the guide was waiting through the window", "value": snap,
             "pass": bool(snap) and snap.get("phase") in ("pointing", "waiting")},
            {"name": "no capture or text reader while it waits", "value": caps, "pass": not caps},
            {"name": f"after Next, the settle watch is bounded (at most maxFrames {frames_max} captures and the read)",
             "value": {"grims_seen": len(grims), "next_s": round(n["t1"] - n["t0"], 2)},
             "pass": n["rc"] == 0 and len(grims) <= frames_max + 1},
            {"name": "guide ended: 0 commits, 0 armed callbacks, within budget", "value": ended.get("ledger_window"),
             "pass": bool(ended.get("pass"))},
            {"name": "guide ended: no guide in the snapshot, no capture or text reader", "value": {"snapshot": snap_e, "running": caps_e},
             "pass": (snap_e is None or snap_e.get("phase") == "idle") and not caps_e},
        ]
        self.limit(15, "settle captures seen", "a capture shorter than the 50 ms watch period can be missed, so the "
                                               "settle count is a lower bound held to the spec's upper bound")
        if sampler is not None:
            self.limit(15, "the floating perch's sampler (Hyprland)",
                       f"Hyprland publishes no move event, so while Oni is awake the floating guide window's perch is "
                       f"sampled with one batched monitors-and-clients request, every 250 ms after a change, backing "
                       f"off to every 2 s while the window holds still (Phase 10's limit); measured with no guide: "
                       f"{sampler}; the guide rows gate the daemon's wakeups over it, everything else as measured")
        return {"checks": checks}, self.checks_pass(checks)

    @staticmethod
    def sampler_cost(base):
        """The floating perch's daemon wakeups per second with no guide, or
        None when the record did not hold the scenario for its window."""
        held = {c["name"]: c["pass"] for c in base.get("checks", [])}
        if not all(held.get(n) for n in ("precondition", "settled before sampling", "no character event during the window")):
            return {"error": "the sampler's measurement did not hold the perched scenario", "checks": base.get("checks")}
        wakes = [c["value"] for c in base["checks"]
                 if c["name"].startswith("daemon ") and c["name"].endswith(" wakeups per second")]
        return {"daemon_wakeups_per_s": wakes[0]} if len(wakes) == 1 and wakes[0] is not None else \
            {"error": "no single daemon wakeup rate", "values": wakes}

    def own_cost(self, name, sampler):
        """measure.sh perched as NAME. With a sampler cost (Hyprland's
        floating perch), the raw record is kept as recorded and the gated
        record holds every check as measured except the daemon's wakeups,
        which count over the sampler's."""
        if sampler is None:
            return self.measure("perched", timeout=150, seconds=30, name=name)
        raw = self.measure("perched", timeout=150, seconds=30, name=name, gate=False)
        rec = json.loads(json.dumps(raw))
        base = sampler.get("daemon_wakeups_per_s")
        checks = []
        for c in rec.get("checks", []):
            if c["name"].startswith("daemon ") and c["name"].endswith(" wakeups per second"):
                own = None if c["value"] is None or base is None else round(c["value"] - base, 3)
                c = {"name": c["name"] + " over the floating perch's sampler", "value": own,
                     "measured": c["value"], "sampler": sampler, "threshold": c.get("threshold"),
                     "pass": own is not None and own <= labkit.THRESHOLDS["wakeups_per_s_max"]}
            checks.append(c)
        rec["checks"] = checks
        rec["sampler"] = sampler
        rec["pass"] = bool(checks) and base is not None and all(c["pass"] for c in checks)
        self.results["measurements"][name] = rec
        self.save()
        print(f"measure {name} over the sampler: {'pass' if rec['pass'] else 'FAIL'}")
        return rec

    def run(self):
        if self.model is None:
            self.results["steps"].append({"step": 0, "name": "lab model", "pass": False,
                                          "error": "no LAB_MODEL_URL: phase12 needs up.sh --model"})
            self.save()
            return 1
        self.vchildren.start()
        try:
            self.step(0, "census", self.census12)
            if self.keys is None:
                raise RuntimeError("no chord FIFO: the rest needs step 0")
            self.step(1, "consent first", self.consent12)
            self.step(2, "workflow", self.workflow)
            self.step(3, "next on an unchanged screen", self.unchanged)
            self.step(4, "keep", self.keep12)
            self.step(5, "replay", self.replay12)
            self.step(6, "Save against Save As", self.saveas)
            self.step(7, "sensitive targets", self.sensitive12)
            self.step(8, "unwind", self.unwind)
            self.step(9, "voice first", self.voice12)
            self.step(10, "select-and-ask", self.select)
            self.step(11, "hints", self.hints)
            self.step(12, "injection", self.injection12)
            self.step(13, "read-only, boundaries, disk", self.readonly12)
            self.step(14, "reduced motion", self.reduced12)
            self.step(15, "settlement", self.settlement12)
            self.step(16, "ledger display is the lab's", self.display_check)
        except RuntimeError as e:
            print(f"phase12: stopped: {e}")
        finally:
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
    if cmd == "phase12" and len(args) == 2:
        return Phase12(*args).run()
    raise SystemExit(__doc__)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
