#!/usr/bin/env python3
"""Ryoku i18n sync: keep the per-language catalogs current from English.

A developer only ever writes English, wrapped at the point it is displayed:
`I18n.tr("...")` in QML, a Hub schema label/desc/group, `i18n.T("...")` in Go,
`t "..."` in the installer's shell. This tool does the rest:

  langs    print the language codes this repo ships (one per line), so CI and
           the shell runtime read the same table this file does.
  extract  scan the tree for every English UI string -> catalog/en.json
  sync     for each target language, translate ONLY the strings it is missing
           (keeping what is already translated and any human overrides), so a
           normal update translates a handful of new strings, never the file.
           --engine google  keyless Google endpoint (default; free, no secret,
                            but context-blind: "Shell" may become a seashell).
           --engine llm     a configured LLM with a domain prompt + glossary,
                            so word senses ("Shell" = the desktop shell) and
                            length are honoured. Needs an API key.
           --force          re-translate every string, not just the missing
                            ones (a one-time pass to upgrade old translations),
                            overrides still win.
           --strict         exit non-zero if any missing string got no
                            translation (so CI surfaces the gap instead of
                            silently shipping English).
  check    report translations that are much longer than their English source
           (the cause of overflowing/overlapping UI). --strict to fail on them.
  cost     price the next full pass: source tokens, output tokens and dollars
           per language for the configured model. --model/--force to compare.
  llm      generate a full language into the layered config dir
           (~/.config/ryoku/i18n/<lang>.json), for a higher-quality pass or a
           language Ryoku doesn't ship. Uses the same prompt + glossary.
  ensure   create ~/.config/ryoku/i18n-llm.json from a template if absent, so
           the user has a key file to fill in.

The LLM is configured either by ~/.config/ryoku/i18n-llm.json or by environment
(RYOKU_I18N_PROVIDER / _KEY / _MODEL / _URL), env winning, so CI can drive it
from a repository secret with no committed key. OpenRouter is the recommended
backend (OpenAI-compatible, one key, cheap models); Anthropic and OpenAI work
too. Placeholders (%1, %2, ...) are shielded so they survive translation, and
overrides/<lang>.json always wins, so a human fix is never overwritten.

  python3 sync.py extract
  python3 sync.py sync                       # all targets, Google
  python3 sync.py sync --engine llm          # all targets, LLM
  python3 sync.py sync --engine llm --force  # full LLM re-translate
  python3 sync.py check --strict             # length guard
  python3 sync.py cost                       # price the next full pass
"""

import concurrent.futures
import json
import os
import random
import re
import sys
import threading
import time
import urllib.error
import urllib.parse
import urllib.request

# this file lives in ryoku/i18n/tools; the module root one level up holds the
# catalog and the language table, and is a Go module, so the tools stay out
# of it (a `go mod vendor` in a consumer would otherwise copy them).
HERE = os.path.dirname(os.path.abspath(__file__))
I18N = os.path.abspath(os.path.join(HERE, ".."))
REPO = os.path.abspath(os.path.join(I18N, "..", ".."))
TRANS = os.path.join(I18N, "catalog")
OVERRIDES = os.path.join(TRANS, "overrides")
LANGS_FILE = os.path.join(I18N, "langs.json")


def _langs():
    """The one language table (langs.json). Everything else derives from it."""
    with open(LANGS_FILE, encoding="utf-8") as fh:
        return json.load(fh)["languages"]


LANGS = _langs()
# file code -> Google target code. "pt" is Brazilian on that endpoint and
# "pt-PT" European, so the table carries the endpoint's spelling, not ours.
TARGETS = {l["code"]: l["google"] for l in LANGS if l["code"] != "en"}
# human names for the LLM prompt (an endpoint code is meaningless to a model).
LANG_NAMES = {l["code"]: l["name"] for l in LANGS}
RTL = {l["code"] for l in LANGS if l.get("dir") == "rtl"}

# Every tree that holds displayed copy. QML and the Hub's schema are the
# desktop; the Go and shell roots are the two installers and the CLI, which
# speak the same catalog through ryoku/i18n/i18n.go and ryoku/i18n/i18n.sh.
QML_ROOT = os.path.join(REPO, "ryoku")
SHIMA_ROOT = os.path.join(QML_ROOT, "shell", "quickshell", "inir")
SCHEMA_DIR = os.path.join(REPO, "ryoku", "hub", "quickshell", "schema")
GO_ROOTS = [os.path.join(REPO, p) for p in
            ("installation/tui", "ryoku-shell-installer", "ryoku/cli",
             "ryoku/shell/ipc", "ryoku/hub/backend", "ryoku/apps/ryostore/backend")]
SH_ROOTS = [os.path.join(REPO, p) for p in
            ("installation/backend", "ryoku-shell-installer")]

# QML/JS: the shared I18n lookup, Shima's Translation bridge and Qt's qsTr
# marker. Quickshell has no qsTr loader, but matching it still keeps a stray
# marker in the catalog.
TR_CALL = re.compile(r"""(?:I18n\.tr|Translation\.tr|qsTr)\(\s*(["'])((?:\\.|(?!\1).)*)\1""")
# Go: i18n.T("..."), i18n.Tf("...", a, b) and the bare T/Tf a program dot-imports.
GO_CALL = re.compile(r'\b(?:i18n\.)?Tf?\(\s*"((?:\\.|[^"\\])*)"')
# shell: the installer's own helpers. log/die take the message as their first
# argument and run it through tf (installation/backend/lib/common.sh), so their
# format string is the catalog key exactly as t/tf's is; `step` is a protocol
# sentinel the TUI parses and is deliberately absent.
SH_CALL = re.compile(
    r"""(?:^|[|(&;`$]|\s)(?:tf?|log|die)\s+(["'])((?:\\.|(?!\1).)*)\1""", re.M)
SCHEMA_FIELD = re.compile(r'"(?:tab|label|desc|group)"\s*:\s*"((?:\\.|[^"\\])*)"')
# the shortcut catalogue (ryoku/wm/binds.go) and each compositor's reason for a
# bind it cannot honour are Go data that Kit and the Hub show through I18n.tr.
WM_ROOT = os.path.join(REPO, "ryoku", "wm")
WM_BIND_FILES = ("binds.go", "config_binds.go", "legend.go")
BIND_FIELD = re.compile(r'\b(?:Category|Label|Hint|reason):\s*"((?:\\.|[^"\\])*)"')
UNHONORED_MAP = re.compile(r'Unhonored = map\[string\]string\{(.*?)\n\}', re.S)
MAP_VALUE = re.compile(r'"[^"\n]*":\s*"((?:\\.|[^"\\])*)"')



def _unescape(lit):
    """Turn a source string literal body into its runtime value (\\n, \\" ...)."""
    try:
        return json.loads('"' + lit.replace('"', '\\"') + '"')
    except Exception:
        return lit


# schema .js for full-bleed pages is documentation, not rendered copy: its
# label/desc/group hold engineering notes, not UI. These markers drop that noise
# so only real, displayed strings become translation keys.
NOISE = ("SettingSection", "PluginPlacementEditor", "disclosure)", "readout)",
         "Repeater", "bespoke", "transient page state", "(no ", "(none", "(header",
         "(action", "(install", "(bottom", "(plugin", "(embedded", "(field")


def _noise(s):
    return s.startswith("(") or any(n in s for n in NOISE)


def _brand(s):
    return any(ord(c) >= 0x3000 for c in s)          # CJK / kana / kanji

OPTS_ARR = re.compile(r'"opts"\s*:\s*\[([^\]]*)\]', re.S)
PAGE_OPTS = re.compile(r'\boptions\s*:\s*\[([^\]]*)\]', re.S)   # inline page option arrays
STR_LIT = re.compile(r'"((?:\\.|[^"\\])*)"')
# data-model display fields, key quoted ("label":) or not (label:).
MODEL_LABEL = re.compile(r'\b(?:label|name|desc|altLabel)"?\s*:\s*"((?:\\.|[^"\\])*)"')
# Shima data tables are translated where their values are rendered, so their
# English literals have no direct tr() call for the extractor to see.
QML_MODEL_FIELD = re.compile(
    r'\b(?:label|description|desc|group|title|subtitle|tip|detail|button|placeholder|'
    r'zeroLabel|emptyText|caption|heading|message|note|summary|blurb|eyebrow)\s*:\s*'
    r'"((?:\\.|[^"\\])*)"')


def _shebang(path):
    """A shell script with no extension (ryoku-install and install.sh's kin)."""
    try:
        with open(path, "rb") as fh:
            head = fh.read(64)
    except OSError:
        return False
    return head.startswith(b"#!") and b"sh" in head.split(b"\n", 1)[0]


# Units and symbols read as words but must never change language: a translated
# "GB" or "Hz" is wrong in every locale, and a two-letter column head is a
# coin toss a translator loses.
VERBATIM = {"GB", "MB", "KB", "TB", "KiB", "MiB", "GiB", "Hz", "kHz", "MHz",
            "GHz", "ms", "px", "dpi", "fps", "W", "mW", "V", "mV", "C", "F",
            "RX", "TX", "IP", "OS", "WM", "FG", "WE", "CPU", "GPU", "RAM",
            "Ryoku", "\u00b0C", "\u00b0F", "CR", "CW", "P1", "P2"}


def _copy(s):
    """Is this string displayed copy, rather than a variable, a flag, a path or
    a symbol? The Go and shell wrappers take an expression, so a helper that
    calls t() on its own argument would otherwise seed the catalog with "$f";
    and a lone letter is an axis mark or a keycap, never a sentence, so asking
    for a translation of it only invites a wrong one."""
    if not s or s in VERBATIM or len(s.strip()) < 2:
        return False
    if s.startswith(("$", "-", "/")) or "$(" in s or "${" in s:
        return False
    if re.fullmatch(r"%[-+#0-9.*]*[A-Za-z]", s):
        return False
    return any(c.isalpha() and ord(c) < 0x80 for c in s)



def extract_keys():
    keys = set()
    for root, _, files in os.walk(QML_ROOT):
        for f in files:
            if not f.endswith(".qml"):
                continue
            # vendored Qt imports are symlinks into /usr/lib/qt6; on a runner
            # without Qt they dangle, so skip anything that won't open.
            path = os.path.join(root, f)
            try:
                text = open(path, encoding="utf-8", errors="ignore").read()
            except OSError:
                continue
            for _, body in TR_CALL.findall(text):
                s = _unescape(body).strip()
                if s:
                    keys.add(s)          # explicit tr() calls are always kept
            if os.path.commonpath((path, SHIMA_ROOT)) == SHIMA_ROOT:
                for body in QML_MODEL_FIELD.findall(text):
                    s = _unescape(body).strip()
                    if s and not _noise(s) and not _brand(s):
                        keys.add(s)
    if os.path.isdir(SCHEMA_DIR):
        for f in os.listdir(SCHEMA_DIR):
            if not f.endswith(".js"):
                continue
            text = open(os.path.join(SCHEMA_DIR, f), encoding="utf-8", errors="ignore").read()
            for body in SCHEMA_FIELD.findall(text):
                s = _unescape(body).strip()
                if s and not _noise(s):
                    keys.add(s)
            # seg/chips option values (the controls translate their display)
            for arr in OPTS_ARR.findall(text):
                for body in STR_LIT.findall(arr):
                    s = _unescape(body).strip()
                    if s and not _noise(s) and not _brand(s):
                        keys.add(s)
    # the Hub rail's nav + group names are data-driven, so I18n.tr() wraps them by
    # variable, not literal; pull them from Hub.qml's groups array (unquoted
    # `name: "..."`, so the quoted-key kanji jpName map is not matched).
    hub = os.path.join(REPO, "ryoku", "hub", "quickshell", "Hub.qml")
    if os.path.isfile(hub):
        text = open(hub, encoding="utf-8", errors="ignore").read()
        for body in re.findall(r'\bname:\s*"([^"]+)"', text):
            s = body.strip()
            if s and not _noise(s):
                keys.add(s)
    # each page declares its title/eyebrow/blurb as string properties, wrapped by
    # variable at render, so pull the literals from the page files.
    pages = os.path.join(REPO, "ryoku", "hub", "quickshell", "pages")
    if os.path.isdir(pages):
        for f in os.listdir(pages):
            if not f.endswith(".qml"):
                continue
            text = open(os.path.join(pages, f), encoding="utf-8", errors="ignore").read()
            for body in re.findall(r'\bp(?:Title|Eyebrow|Blurb)\s*:\s*"((?:\\.|[^"\\])*)"', text):
                s = _unescape(body).strip()
                if s and not _noise(s):
                    keys.add(s)
            # data-model display labels ({key, label} arrays, FnCard names, ...);
            # the controls / render sites translate them, brand kana excluded.
            for body in MODEL_LABEL.findall(text):
                s = _unescape(body).strip()
                if s and not _noise(s) and not _brand(s):
                    keys.add(s)
            # inline option arrays in a page (options: ["FOLLOW","LIGHT",...]);
            # a control translates the display, the value stays the source string.
            for arr in PAGE_OPTS.findall(text):
                for body in STR_LIT.findall(arr):
                    s = _unescape(body).strip()
                    if s and not _noise(s) and not _brand(s):
                        keys.add(s)
    # the two installers and the CLI: Go and shell wrap at the display site the
    # same way QML does, so the same catalog serves them. vendor/ is other
    # people's code and _test.go is not shipped copy. i18n.sh is skipped
    # because it calls t() on its own argument, which is a variable, not copy.
    for root_dir in GO_ROOTS:
        for root, dirs, files in os.walk(root_dir):
            dirs[:] = [d for d in dirs if d not in ("vendor", "testdata", ".git")]
            for f in files:
                if not f.endswith(".go") or f.endswith("_test.go"):
                    continue
                text = open(os.path.join(root, f), encoding="utf-8", errors="ignore").read()
                for body in GO_CALL.findall(text):
                    s = _unescape(body).strip()
                    if _copy(s):
                        keys.add(s)
    for root, dirs, files in os.walk(WM_ROOT):
        dirs[:] = [d for d in dirs if d not in ("vendor", "testdata", ".git")]
        for f in files:
            if f not in WM_BIND_FILES:
                continue
            text = open(os.path.join(root, f), encoding="utf-8", errors="ignore").read()
            bodies = BIND_FIELD.findall(text)
            for block in UNHONORED_MAP.findall(text):
                bodies += MAP_VALUE.findall(block)
            for body in bodies:
                s = _unescape(body).strip()
                if _copy(s):
                    keys.add(s)
    for root_dir in SH_ROOTS:
        for root, dirs, files in os.walk(root_dir):
            dirs[:] = [d for d in dirs if d not in ("vendor", "testdata", ".git")]
            for f in files:
                p = os.path.join(root, f)
                if f in ("i18n.sh", "common.sh"):
                    continue
                if not (f.endswith((".sh", ".bash")) or _shebang(p)):
                    continue
                text = open(p, encoding="utf-8", errors="ignore").read()
                for _, body in SH_CALL.findall(text):
                    s = _unescape(body).strip()
                    if _copy(s):
                        keys.add(s)
    # the installers' browser/shell/compositor choices are data, translated by
    # variable at render, so pull their display columns from the one manifest.
    choices = os.path.join(REPO, "installation", "installerplan", "choices.tsv")
    if os.path.isfile(choices):
        with open(choices, encoding="utf-8", errors="ignore") as fh:
            rows = [line.rstrip("\n").split("\t") for line in fh if line.strip()]
        header = rows[0] if rows else []
        wanted = [header.index(c) for c in ("label", "tui_hint", "shell_detail") if c in header]
        for row in rows[1:]:
            if not row or row[0] != "choice":
                continue
            for i in wanted:
                s = row[i].strip() if i < len(row) else ""
                if s and s != "-" and _copy(s):
                    keys.add(s)
    # a last pass over everything, whatever wrapped it: a string with no ASCII
    # letter is a number, a glyph or a symbol, and asking a translator for one
    # invites localized digits in a numeric readout.
    return {k for k in keys if _copy(k)}


def load_json(path):
    try:
        return json.load(open(path, encoding="utf-8"))
    except Exception:
        return {}


def write_json(path, obj):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        json.dump(dict(sorted(obj.items())), fh, ensure_ascii=False, indent=2)
        fh.write("\n")


def cmd_extract():
    keys = extract_keys()
    write_json(os.path.join(TRANS, "en.json"), {k: k for k in keys})
    print(f"extract: {len(keys)} strings -> catalog/en.json")


# Placeholders differ per runtime: QML uses .arg() with %1..%9, Go and the
# installer's printf use %s/%d/%v (and %[1]s). All of them are substituted at
# runtime, so a translator must return them untouched and in a usable order.
PLACEHOLDER = re.compile(r"%(?:%|\d|\[\d+\][a-zA-Z]|[-+#0-9.]*[a-zA-Z])")


def _ph(s):
    """The placeholder multiset of a string, order-independent."""
    return sorted(PLACEHOLDER.findall(s))


def shield(s):
    """Swap placeholders for stable ASCII tokens before machine translation."""
    toks = []

    def take(m):
        toks.append(m.group(0))
        return f"__RYOKU_PH_{len(toks) - 1}__"
    return PLACEHOLDER.sub(take, s), toks


def unshield(s, toks):
    for i, token in enumerate(toks):
        s = s.replace(f"__RYOKU_PH_{i}__", token)
    return s


def _sanitize(s):
    # the repo's pre-commit forbids em-dashes in text files (and en-dashes read
    # as machine-styled); translators emit both, so normalise them to a hyphen.
    return s.replace("\u2014", "-").replace("\u2013", "-")


def google_translate(text, tl, tries=4):
    q, toks = shield(text)
    url = "https://translate.googleapis.com/translate_a/single?" + urllib.parse.urlencode(
        {"client": "gtx", "sl": "en", "tl": tl, "dt": "t", "q": q})
    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
    for attempt in range(tries):
        try:
            _pace()
            with urllib.request.urlopen(req, timeout=15) as resp:
                data = json.loads(resp.read().decode("utf-8"))
            out = "".join(seg[0] for seg in data[0] if seg and seg[0])
            return _sanitize(unshield(out, toks))
        except Exception as e:
            if attempt == tries - 1:
                print(f"  ! translate failed ({tl}): {e}", file=sys.stderr)
                return None
            # exponential backoff + jitter: datacenter IPs (CI) get rate-limited
            # by the keyless endpoint, and the block clears if we back off.
            time.sleep(1.5 * (attempt + 1) + random.random())
    return None


# One throttle for every request to the keyless endpoint, whatever thread makes
# it. The block that matters is per IP per window, so what keeps a full catalog
# run alive is a steady rate, not a worker count: threads only cover latency.
# RYOKU_I18N_RATE tunes it (requests per second).
_pace_lock = threading.Lock()
_pace_last = [0.0]
_PACE_MIN = 1.0 / float(os.environ.get("RYOKU_I18N_RATE") or 2.0)


def _pace():
    with _pace_lock:
        wait = _pace_last[0] + _PACE_MIN - time.monotonic()
        if wait > 0:
            time.sleep(wait)
        _pace_last[0] = time.monotonic()


def _google_get(payload, tl, tries=7):
    url = "https://translate.googleapis.com/translate_a/single?" + urllib.parse.urlencode(
        {"client": "gtx", "sl": "en", "tl": tl, "dt": "t", "q": payload})
    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
    for attempt in range(tries):
        try:
            _pace()
            with urllib.request.urlopen(req, timeout=20) as resp:
                return json.loads(resp.read().decode("utf-8"))
        except Exception as e:
            if attempt == tries - 1:
                print(f"  ! translate failed ({tl}): {e}", file=sys.stderr)
                return None
            # exponential backoff + jitter. A 429 is the endpoint asking for a
            # real pause, not a hiccup: back off hard on it, because retrying
            # inside a second only deepens the block and loses the whole
            # language. Everything else is a transient network fault.
            rate_limited = "429" in str(e)
            base = 8.0 if rate_limited else 1.5
            time.sleep(base * (2 ** attempt) + random.random() * 2)
    return None


# The endpoint segments a newline-joined payload and echoes each source line
# beside its translation, so many strings travel in one request and every result
# is matched by its source rather than by position. One string per request over
# a whole catalog is hours of round trips; this is the difference between the
# keyless engine being a real fallback and being unusable.
GOOGLE_BUDGET = 1500        # payload chars: a GET URL has to stay well short of 8k


def google_batch(texts, tl):
    """Translate a list of newline-free strings. Returns {source: translation}
    for what came back; a string the endpoint did not echo is simply absent, so
    the caller falls back or ships English."""
    out = {}
    shielded = {}
    for t in texts:
        q, toks = shield(t)
        shielded[q] = (t, toks)
    batch, size = [], 0
    def flush():
        if not batch:
            return
        data = _google_get("\n".join(batch), tl)
        if not data or not data[0]:
            return
        for seg in data[0]:
            if not seg or len(seg) < 2 or not seg[0] or not seg[1]:
                continue
            src = seg[1].strip("\n")
            got = shielded.get(src)
            if not got:
                continue
            orig, toks = got
            out[orig] = _sanitize(unshield(seg[0].strip("\n"), toks))
    for q in shielded:
        if size + len(q) + 1 > GOOGLE_BUDGET and batch:
            flush()
            batch, size = [], 0
        batch.append(q)
        size += len(q) + 1
    flush()
    return out


def google_translate_all(texts, tl, workers=4):
    """Every string for one language. Multi-line sources cannot ride a
    newline-joined payload, so they go one at a time; the rest batch. Requests
    run a few at a time, which is what makes a 4000-string language minutes
    rather than hours, while staying gentle enough not to trip the block."""
    multiline = [t for t in texts if "\n" in t]
    flat = [t for t in texts if "\n" not in t]
    out = {}

    chunks, batch, size = [], [], 0
    for t in flat:
        if size + len(t) + 1 > GOOGLE_BUDGET and batch:
            chunks.append(batch)
            batch, size = [], 0
        batch.append(t)
        size += len(t) + 1
    if batch:
        chunks.append(batch)

    with concurrent.futures.ThreadPoolExecutor(max_workers=workers) as pool:
        for got in pool.map(lambda c: google_batch(c, tl), chunks):
            out.update(got)
        for t, got in zip(multiline, pool.map(lambda s: google_translate(s, tl), multiline)):
            if got:
                out[t] = got

    # anything the batch did not echo back, retried on its own.
    missed = [t for t in texts if t not in out]
    if missed:
        with concurrent.futures.ThreadPoolExecutor(max_workers=workers) as pool:
            for t, got in zip(missed, pool.map(lambda s: google_translate(s, tl), missed)):
                if got:
                    out[t] = got
    return out


# ── glossary: the domain knowledge Google cannot have and an LLM can ──────────
# Ryoku is a Linux/Wayland desktop shell, so its words carry software senses, not
# everyday ones. KEEP stays verbatim in every language; SENSE disambiguates the
# words a generic translator gets wrong ("Shell" -> command-line shell, never a
# seashell). Both are injected into the LLM prompt.
KEEP = [
    "Ryoku", "Wi-Fi", "Bluetooth", "GPU", "CPU", "RAM", "VPN", "SSID", "DNS",
    "IP", "MAC", "USB", "HDMI", "RGB", "PID", "OSD", "QR", "PipeWire",
    "PulseAudio", "Wayland", "Hyprland", "Niri", "Sway", "systemd",
    "opencode", "codex", "Whisper", "gpu-screen-recorder",
]
SENSE = {
    "Shell": "the desktop shell / Unix command-line shell software, never a seashell",
    "Bar": "the desktop top panel / status bar, not a place that serves drinks",
    "Dock": "the application dock / taskbar",
    "Tray": "the system tray (notification area)",
    "Idle": "the session's idle / inactivity state",
    "Lock": "locking the screen",
    "Sink": "an audio output device",
    "Source": "an audio input device",
    "Mount": "mounting a filesystem / drive",
    "Window": "an application window (window manager)",
    "Workspace": "a virtual desktop / workspace",
    "Tile": "a tiling window layout",
    "Key": "a keyboard key or a config key, not a door key",
    "Launcher": "the application launcher",
    "Hero": "the large feature widget / image / clock area of a page, not a person or superhero",
    "Deck": "a stacked panel of results (e.g. the launcher result deck), not a card deck or a ship deck",
    "Frost": "a frosted-glass blur effect (also frosts / frosted), not weather or ice",
    "Passthrough": "GPU passthrough (a VM gets direct GPU access), not a keyboard shortcut",
    "Dictation": "voice dictation / speech to text, never dictatorship",
    "Clockwork": "a clockwork gears mechanism, not a watchmaker",
    "X-ray": "a see-through blur that reveals the wallpaper, not medical imaging",
    "Snap": "snapping / aligning a window to a screen edge, not attaching",
    "Reflection": "a visual mirror reflection, not contemplation",
    "Passes": "rendering / blur passes (a count), not passages or walkways",
    "Fade-in": "content appearing as opacity rises; fade-out is the reverse, never swap them",
}


def build_prompt(lang_name, chunk):
    senses = "".join(f'    - "{t}": {d}\n' for t, d in SENSE.items())
    return (
        f"You translate UI strings for Ryoku, a Linux/Wayland desktop shell "
        f"(status bars, launcher, control center, notifications, settings). "
        f"Translate from English into {lang_name}.\n"
        "Rules:\n"
        "- Return ONLY a JSON object mapping each English source string to its "
        "translation. No prose, no code fences.\n"
        "- Match the terse tone of a settings app. Keep each translation as short "
        "as the English, and never more than ~1.3x its character length, so the "
        "UI does not overflow.\n"
        "- Copy every placeholder through untouched and keep each one's meaning: "
        "%1 %2 (QML), %s %d %v (Go and shell printf), %% for a literal percent. "
        "Never translate, reorder into nonsense, add or drop one.\n"
        "- Do not use em dashes or en dashes; use a comma, colon, or parentheses.\n"
        "- Write the target language's own script, never a Latin transliteration.\n"
        f"- Keep these terms untranslated: {', '.join(KEEP)}.\n"
        "- These words are desktop-software terms, not everyday language:\n"
        f"{senses}"
        "\nStrings to translate:\n"
        + json.dumps({k: k for k in chunk}, ensure_ascii=False)
    )


def _cfg_home():
    return os.environ.get("XDG_CONFIG_HOME") or os.path.join(os.path.expanduser("~"), ".config")


LLM_CFG = os.path.join(_cfg_home(), "ryoku", "i18n-llm.json")
GEN_DIR = os.path.join(_cfg_home(), "ryoku", "i18n")

LLM_CFG_TEMPLATE = {
    "_help": ("Paste your API key into \"key\". The default is OpenRouter "
              "(OpenAI-compatible, one key for every model, cheap): get a key at "
              "https://openrouter.ai/keys and pick a model at "
              "https://openrouter.ai/models . For vanilla OpenAI set provider "
              "\"openai\" and url \"\"; for Anthropic set provider \"anthropic\". "
              "Any field can also be set via env: RYOKU_I18N_PROVIDER / _KEY / "
              "_MODEL / _URL (env wins, used by CI)."),
    "provider": "openai",
    "url": "https://openrouter.ai/api/v1/chat/completions",
    "model": "google/gemini-2.5-flash",
    "key": "",
}


def load_llm_cfg():
    """File config overlaid with environment (env wins), so CI drives it from a
    secret with nothing committed."""
    cfg = load_json(LLM_CFG)
    env = os.environ
    for field, var in (("provider", "RYOKU_I18N_PROVIDER"), ("key", "RYOKU_I18N_KEY"),
                       ("model", "RYOKU_I18N_MODEL"), ("url", "RYOKU_I18N_URL")):
        val = env.get(var)
        if val:
            cfg[field] = val
    return cfg


def seed_llm_cfg():
    """Create the LLM key file from the template if absent. Idempotent."""
    if os.path.exists(LLM_CFG):
        return False
    os.makedirs(os.path.dirname(LLM_CFG), exist_ok=True)
    with open(LLM_CFG, "w", encoding="utf-8") as fh:
        json.dump(LLM_CFG_TEMPLATE, fh, ensure_ascii=False, indent=2)
        fh.write("\n")
    os.chmod(LLM_CFG, 0o600)
    return True


def cmd_ensure():
    seed_llm_cfg()
    return 0


class LLMUnavailable(Exception):
    """The configured translation provider cannot currently serve requests."""


def llm_call(cfg, prompt):
    provider = cfg.get("provider", "openai")
    key = cfg.get("key", "")
    if provider == "anthropic":
        model = cfg.get("model") or "claude-3-5-haiku-latest"
        url = "https://api.anthropic.com/v1/messages"
        body = {"model": model, "max_tokens": 4096,
                "messages": [{"role": "user", "content": prompt}]}
        headers = {"x-api-key": key, "anthropic-version": "2023-06-01", "content-type": "application/json"}
    else:  # openai-compatible (OpenRouter by default, or vanilla OpenAI)
        model = cfg.get("model") or "gpt-4o-mini"
        url = cfg.get("url") or "https://api.openai.com/v1/chat/completions"
        body = {"model": model, "messages": [{"role": "user", "content": prompt}]}
        headers = {"Authorization": "Bearer " + key, "content-type": "application/json",
                   "X-Title": "Ryoku i18n",
                   "HTTP-Referer": "https://github.com/noctalia-dev"}
    req = urllib.request.Request(url, data=json.dumps(body).encode(), headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=90) as resp:
            data = json.loads(resp.read().decode())
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        unavailable = ("credit", "quota", "insufficient", "billing",
                       "unauthorized", "invalid api key")
        if exc.code in (401, 402, 403, 429) and any(
                marker in body.lower() for marker in unavailable):
            snippet = " ".join(body.split())[:240]
            raise LLMUnavailable(f"HTTP {exc.code}: {snippet}") from exc
        raise
    except urllib.error.URLError as exc:
        reason = exc.reason
        reason_text = str(reason).lower()
        if isinstance(reason, TimeoutError) or "timeout" in reason_text or "timed out" in reason_text:
            raise
        raise LLMUnavailable(str(reason)) from exc
    if provider == "anthropic":
        return data["content"][0]["text"]
    return data["choices"][0]["message"]["content"]


def _extract_json(text):
    a, b = text.find("{"), text.rfind("}")
    return json.loads(text[a:b + 1]) if a >= 0 and b > a else {}


def llm_translate(cfg, lang, keys, batch=40):
    """Translate `keys` into `lang` (a file code or name) via the model, in
    batches with the domain prompt + glossary. A batch whose reply is not valid
    JSON is bisected and retried, so one malformed string never sinks a whole
    batch. Returns {key: translation} for what came back; caller picks fallback."""
    name = LANG_NAMES.get(lang, lang)
    out = {}

    def run(chunk, depth):
        if not chunk:
            return
        try:
            got = _extract_json(llm_call(cfg, build_prompt(name, chunk)))
            # a translation that lost, gained or translated a placeholder would
            # print a literal "%s" (or drop an argument) at runtime, so it is
            # dropped here and the English source is shipped instead.
            out.update({k: _sanitize(v) for k, v in got.items()
                        if k in chunk and isinstance(v, str) and v
                        and _ph(v) == _ph(k)})
        except LLMUnavailable:
            raise
        except Exception as e:
            if len(chunk) > 1 and depth < 6:
                mid = len(chunk) // 2
                run(chunk[:mid], depth + 1)
                run(chunk[mid:], depth + 1)
            else:
                print(f"  ! llm failed ({lang}) on {chunk!r}: {str(e)[:80]}", file=sys.stderr)

    for i in range(0, len(keys), batch):
        run(keys[i:i + batch], 0)
        print(f"  {lang}: {len(out)}/{len(keys)}")
    return out


def _parse_sync_args(argv):
    engine, force, strict, langs = "google", False, False, []
    i = 0
    while i < len(argv):
        a = argv[i]
        if a == "--engine":
            engine = argv[i + 1]
            i += 2
        elif a == "--force":
            force = True
            i += 1
        elif a == "--strict":
            strict = True
            i += 1
        else:
            langs.append(a)
            i += 1
    return engine, force, strict, langs


def cmd_sync(argv):
    engine, force, strict, langs = _parse_sync_args(argv)
    if engine not in ("google", "llm"):
        print(f"sync: unknown engine {engine} (google|llm)", file=sys.stderr)
        return 2
    en = load_json(os.path.join(TRANS, "en.json"))
    if not en:
        print("sync: run extract first (catalog/en.json is empty)", file=sys.stderr)
        return 1
    langs = langs or list(TARGETS)

    cfg = None
    if engine == "llm":
        cfg = load_llm_cfg()
        if not cfg.get("key"):
            print("sync --engine llm: no API key. Set RYOKU_I18N_KEY (CI) or edit "
                  f"{LLM_CFG} (run `ensure` to create it).", file=sys.stderr)
            return 1

    unresolved = 0
    try:
        for lang in langs:
            tl = TARGETS.get(lang)
            if not tl:
                print(f"sync: unknown language {lang}", file=sys.stderr)
                continue
            existing = load_json(os.path.join(TRANS, f"{lang}.json"))
            overrides = load_json(os.path.join(OVERRIDES, f"{lang}.json"))

            # a string needs translating unless a human override covers it, or (when
            # not forcing) it is already translated (present and not equal to English).
            def done(k):
                return k in overrides or (not force and k in existing and existing[k] != k)
            missing = [k for k in en if not done(k)]

            translated = {}
            if missing:
                if engine == "llm":
                    translated = llm_translate(cfg, lang, missing)
                else:
                    got = google_translate_all(missing, tl)
                    # a translation that lost, gained or translated a placeholder
                    # would print a literal "%s" at runtime, so it is dropped and
                    # the English source ships for that string instead.
                    translated = {k: v for k, v in got.items() if _ph(v) == _ph(k)}

            out, failed = {}, []
            for k in en:
                if k in overrides:
                    out[k] = overrides[k]
                elif not force and k in existing and existing[k] != k:
                    out[k] = existing[k]                  # keep prior translation
                elif k in translated:
                    out[k] = translated[k]
                else:
                    out[k] = _sanitize(k)                  # unresolved -> English source
                    if k in missing:
                        failed.append(k)                  # a real miss, not a kept string
            write_json(os.path.join(TRANS, f"{lang}.json"), out)
            unresolved += len(failed)
            print(f"sync[{engine}] {lang}: {len(out)} strings "
                  f"({len(translated)} newly translated, {len(overrides)} overrides, "
                  f"{len(failed)} unresolved)")
            # a whole catalog is thousands of requests; running the next language
            # straight after is what turns the keyless endpoint's rate limiter on.
            # the LLM engine is paid for and needs no such pause.
            if engine == "google" and lang != langs[-1]:
                sys.stdout.flush()
                time.sleep(10)
    except LLMUnavailable as exc:
        # Keep completed catalogs so a later retry does not discard useful work.
        print(f"i18n: translation provider unavailable: {exc}; nothing was translated, "
              "run the workflow again when credits are back", file=sys.stderr)
        return 3

    if strict and unresolved:
        print(f"strict: {unresolved} string(s) left untranslated", file=sys.stderr)
        return 1
    return 0


def cmd_check(argv):
    """Flag translations far longer than their English source: the cause of
    overflowing/overlapping UI. Advisory by default; --strict fails."""
    strict = "--strict" in argv
    factor, min_src = 1.5, 12
    for a in argv:
        if a.startswith("--factor="):
            factor = float(a.split("=", 1)[1])
    en = load_json(os.path.join(TRANS, "en.json"))
    if not en:
        print("check: run extract first (catalog/en.json is empty)", file=sys.stderr)
        return 1
    offenders = []
    for lang in TARGETS:
        m = load_json(os.path.join(TRANS, f"{lang}.json"))
        for k, src in en.items():
            tr = m.get(k)
            if not tr or tr == src or len(src) < min_src:
                continue
            if len(tr) > len(src) * factor:
                offenders.append((len(tr) / len(src), lang, src, tr))
    offenders.sort(reverse=True)
    for ratio, lang, src, tr in offenders:
        print(f"  {lang} {ratio:.2f}x  {src!r} -> {tr!r}")
    print(f"check: {len(offenders)} translation(s) exceed {factor:.2f}x the "
          f"English length (source >= {min_src} chars)")
    return 1 if strict and offenders else 0


def cmd_llm(langs):
    """Full LLM generation into the layered config overlay (~/.config/ryoku/i18n)."""
    seed_llm_cfg()
    cfg = load_llm_cfg()
    if not cfg.get("key"):
        print(f"llm: no API key set. Edit {LLM_CFG} and paste your key into the "
              "\"key\" field (or set RYOKU_I18N_KEY), then try again.", file=sys.stderr)
        return 1
    en = load_json(os.path.join(TRANS, "en.json"))
    if not en:
        cmd_extract()
        en = load_json(os.path.join(TRANS, "en.json"))
    keys = list(en)
    for lang in (langs or [cfg.get("target", "es")]):
        out = llm_translate(cfg, lang, keys)
        os.makedirs(GEN_DIR, exist_ok=True)
        with open(os.path.join(GEN_DIR, f"{lang}.json"), "w", encoding="utf-8") as fh:
            json.dump(dict(sorted(out.items())), fh, ensure_ascii=False, indent=2)
        print(f"llm {lang}: wrote {len(out)} strings -> {GEN_DIR}/{lang}.json")
    return 0


# ── cost: what the next pass costs, so a model choice is a number not a guess ─
# USD per million tokens (input, output) at the OpenRouter list price. A model
# missing here is priced from --in/--out on the command line.
PRICES = {
    "google/gemini-2.5-flash": (0.30, 2.50),
    "google/gemini-2.5-flash-lite": (0.10, 0.40),
    "google/gemini-2.5-pro": (1.25, 10.00),
    "openai/gpt-5-mini": (0.25, 2.00),
    "openai/gpt-5": (1.25, 10.00),
    "anthropic/claude-haiku-4.5": (1.00, 5.00),
    "anthropic/claude-sonnet-4.5": (3.00, 15.00),
    "deepseek/deepseek-v3.2": (0.27, 0.41),
    "qwen/qwen3-235b-a22b": (0.13, 0.60),
}


def _toks(s):
    """Token estimate. Latin prose runs ~4 chars/token; a script with no BPE
    merges to lean on (CJK, Devanagari, Arabic) runs closer to 1.5, so a
    per-language output estimate has to weight by script, not by character."""
    return max(1, round(len(s) / 4))


def cmd_cost(argv):
    model = None
    force = "--force" in argv
    price_in = price_out = None
    for a in argv:
        if a.startswith("--model="):
            model = a.split("=", 1)[1]
        elif a.startswith("--in="):
            price_in = float(a.split("=", 1)[1])
        elif a.startswith("--out="):
            price_out = float(a.split("=", 1)[1])
    if model is None:
        model = load_llm_cfg().get("model") or LLM_CFG_TEMPLATE["model"]
    if price_in is None or price_out is None:
        got = PRICES.get(model)
        if not got:
            print(f"cost: no list price for {model}; pass --in=<usd/Mtok> "
                  "--out=<usd/Mtok>", file=sys.stderr)
            return 1
        price_in, price_out = got

    en = load_json(os.path.join(TRANS, "en.json"))
    if not en:
        print("cost: run extract first (catalog/en.json is empty)", file=sys.stderr)
        return 1
    # the prompt is rebuilt per batch, so its glossary is paid for once per
    # batch, not once per string: that overhead dominates on small deltas.
    batch = 40
    overhead = _toks(build_prompt("English", []))
    total_in = total_out = 0.0
    rows = []
    for lang in TARGETS:
        existing = load_json(os.path.join(TRANS, f"{lang}.json"))
        overrides = load_json(os.path.join(OVERRIDES, f"{lang}.json"))
        missing = [k for k in en if k not in overrides
                   and (force or k not in existing or existing[k] == k)]
        if not missing:
            rows.append((lang, 0, 0.0, 0.0, 0.0))
            continue
        src = sum(_toks(k) for k in missing)
        batches = (len(missing) + batch - 1) // batch
        tin = src + overhead * batches
        # a translation is about as long as its source in Latin script, and
        # tokenizes far worse outside it; JSON echoes the English key too, so
        # output carries the source string as well as the translation.
        expand = 2.4 if lang in ("ar", "bn", "el", "fa", "he", "hi", "ja", "ko",
                                 "ml", "mr", "ru", "ta", "te", "th", "uk",
                                 "zh_CN", "zh_TW") else 1.35
        tout = src * (1 + expand)
        cin, cout = tin / 1e6 * price_in, tout / 1e6 * price_out
        total_in += cin
        total_out += cout
        rows.append((lang, len(missing), tin, tout, cin + cout))
    print(f"cost[{model}]  ${price_in:.2f}/Mtok in, ${price_out:.2f}/Mtok out"
          f"   ({'full re-translate' if force else 'missing strings only'})")
    print(f"  {'lang':7s} {'strings':>8s} {'in tok':>10s} {'out tok':>10s} {'USD':>9s}")
    for lang, n, tin, tout, usd in rows:
        print(f"  {lang:7s} {n:8d} {tin:10.0f} {tout:10.0f} {usd:9.4f}")
    print(f"  {'TOTAL':7s} {sum(r[1] for r in rows):8d} "
          f"{sum(r[2] for r in rows):10.0f} {sum(r[3] for r in rows):10.0f} "
          f"{total_in + total_out:9.4f}")
    return 0


def cmd_langs():
    for l in LANGS:
        print(l["code"])
    return 0


def main():
    args = sys.argv[1:]
    cmds = ("extract", "sync", "check", "llm", "ensure", "cost", "langs")
    if not args or args[0] not in cmds:
        print(__doc__)
        return 2
    if args[0] == "langs":
        return cmd_langs()
    if args[0] == "ensure":
        return cmd_ensure()
    if args[0] == "extract":
        cmd_extract()
        return 0
    if args[0] == "check":
        return cmd_check(args[1:])
    if args[0] == "cost":
        return cmd_cost(args[1:])
    if args[0] == "llm":
        return cmd_llm(args[1:])
    return cmd_sync(args[1:])


if __name__ == "__main__":
    sys.exit(main())
