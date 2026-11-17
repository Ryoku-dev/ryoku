#!/usr/bin/env bash
# Regression test for ryoku-cursor-material-recolor (#211). The tool recolours
# the Material Bibata pointer on every palette change; two bugs made every path
# a silent no-op: it read the retired hypr.json (so the theme gate never opened
# and a forced run reset the size to 24), and its accent() argv scan crashed on
# the six-character token "--full". The store moved to desktop.json under
# .desktop.cursor.*, and "DYNAMIC" (the Hub's "Follow the wallpaper" pick) is a
# role that means this theme. None of that needs the Bibata source: the gate,
# the size, and the accent scan are pure reads.
set -euo pipefail

here="$(cd "$(dirname "$0")/.." && pwd)"
tool="$here/release/packages/ryoku-cursor-material/ryoku-cursor-material-recolor"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

export HOME="$tmp"
mkdir -p "$tmp/.config/ryoku" "$tmp/.cache/ryoku"

probe() {
  python3 - "$tool" "$@" <<'PY'
import importlib.machinery
import importlib.util
import json
import sys
from pathlib import Path

tool_path, body = sys.argv[1], sys.argv[2]
cfg = json.loads(body)
Path.home().joinpath(".config/ryoku/desktop.json").write_text(
    json.dumps(cfg.get("desktop.json", {})))
if "hypr.json" in cfg:
    Path.home().joinpath(".config/ryoku/hypr.json").write_text(
        json.dumps(cfg["hypr.json"]))
colors = Path.home() / ".cache/ryoku/colors.json"
if colors.exists():
    colors.unlink()
if "colors.json" in cfg:
    colors.write_text(json.dumps(cfg["colors.json"]))

spec = importlib.util.spec_from_loader(
    "recolor", importlib.machinery.SourceFileLoader("recolor", tool_path))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)

sys.argv = [tool_path, *cfg.get("argv", [])]
print(json.dumps({
    "active_theme": m.active_theme(),
    "size": m.cursor_size(),
    "accent": m.accent(),
}))
PY
}

fail() { echo "FAIL: $*" >&2; exit 1; }

# 1. The gate opens on the concrete theme read from desktop.json's nested
#    shape, and the user's size survives (the hypr.json bug reset it to 24).
got="$(probe '{"desktop.json":{"desktop":{"cursor":{"theme":"Bibata-Material-Ryoku","size":18}}},"colors.json":{"primary":"#ff8800"}}')"
[[ $(jq -r .active_theme <<<"$got") == "Bibata-Material-Ryoku" ]] || fail "theme gate closed on a desktop.json pick: $got"
[[ $(jq -r .size <<<"$got") == 18 ]] || fail "size not read from desktop.json: $got"

# 2. "DYNAMIC" is the Hub's "Follow the wallpaper" role: it means this theme,
#    so the gate must open for it too.
got="$(probe '{"desktop.json":{"desktop":{"cursor":{"theme":"DYNAMIC","size":20}}},"colors.json":{"primary":"#ff8800"}}')"
[[ $(jq -r .active_theme <<<"$got") == "Bibata-Material-Ryoku" ]] || fail "DYNAMIC did not open the gate: $got"

# 3. A different theme keeps the tool a no-op.
got="$(probe '{"desktop.json":{"desktop":{"cursor":{"theme":"Vimix-cursors"}}},"colors.json":{"primary":"#ff8800"}}')"
[[ $(jq -r .active_theme <<<"$got") == "Vimix-cursors" ]] || fail "foreign theme leaked: $got"

# 4. The retired hypr.json must NOT be consulted any more: a theme only there
#    reads as unset, not as the material theme.
got="$(probe '{"desktop.json":{"desktop":{"cursor":{"theme":"phinger-cursors"}}},"hypr.json":{"cursor":{"theme":"Bibata-Material-Ryoku","size":42}},"colors.json":{"primary":"#ff8800"}}')"
[[ $(jq -r .active_theme <<<"$got") == "phinger-cursors" ]] || fail "still reading hypr.json: $got"
[[ $(jq -r .size <<<"$got") == 24 ]] || fail "hypr.json size leaked through: $got"

# 5. accent() must survive the flag scan: "--full" is six characters and used
#    to crash the hex parse before the palette was ever read.
got="$(probe '{"desktop.json":{"desktop":{"cursor":{"theme":"Bibata-Material-Ryoku"}}},"colors.json":{"primary":"#123456"},"argv":["--force","--full"]}')"
[[ $(jq -r .accent <<<"$got") == "#123456" ]] || fail "--full crashed or hijacked the accent: $got"

# 6. A real hex argument still overrides the palette.
got="$(probe '{"desktop.json":{"desktop":{"cursor":{"theme":"Bibata-Material-Ryoku"}}},"colors.json":{"primary":"#123456"},"argv":["--force","#abcdef"]}')"
[[ $(jq -r .accent <<<"$got") == "#abcdef" ]] || fail "hex override ignored: $got"

# 7. No palette file: the documented fallback, no crash.
got="$(probe '{"desktop.json":{"desktop":{"cursor":{"theme":"Bibata-Material-Ryoku"}}},"argv":["--force","--full"]}')"
[[ $(jq -r .accent <<<"$got") == "#e2342a" ]] || fail "missing palette did not fall back: $got"

# 8. The fast path never queues behind a full-ladder build. That queue is what
#    made a wallpaper change wait ~30 s: the fast build sat behind the previous
#    change's full build on one shared lock. With a full build holding its lock,
#    a new accent's fast build must still apply promptly on its own lock.
got="$(python3 - "$tool" <<'PY'
import importlib.machinery
import importlib.util
import json
import sys
import threading
import time
from pathlib import Path

tool_path = sys.argv[1]
spec = importlib.util.spec_from_loader(
    "recolor", importlib.machinery.SourceFileLoader("recolor", tool_path))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)

colors = Path.home() / ".cache/ryoku/colors.json"
colors.write_text(json.dumps({"primary": "#aaaaaa"}))
left_ptr = m.INSTALL / m.THEME / "cursors/left_ptr"
left_ptr.parent.mkdir(parents=True, exist_ok=True)
left_ptr.write_text("stub")
m.display_sizes = lambda: [32]
m.apply_live = lambda: None

def build_x11(body, primary, watch, sizes=None, abort=None, nice=False):
    # fast build ~0.3 s, full build ~3 s; honour the abort poll like the real one
    end = time.time() + (0.3 if sizes else 3.0)
    while time.time() < end:
        if abort and abort():
            raise m.BuildAborted()
        time.sleep(0.05)

m.build_x11 = build_x11

tf = threading.Thread(target=m._full_build)
tf.start()
time.sleep(0.2)
colors.write_text(json.dumps({"primary": "#bbbbbb"}))
t0 = time.time()
m._fast_build()
fast_elapsed = time.time() - t0
tf.join()
print(json.dumps({"fast_elapsed": fast_elapsed,
                  "fast": m.FAST.read_text().strip()}))
PY
)"
awk -v e="$(jq -r .fast_elapsed <<<"$got")" 'BEGIN{exit !(e<1.0)}' || fail "fast build waited on the full lock: $got"
[[ $(jq -r .fast <<<"$got") == "#bbbbbb" ]] || fail "fast marker does not carry the newest accent: $got"

# 8b. A full build aborts when a newer accent arrives mid-raster, then rebuilds
#     onto that accent, so a stale ladder never lands over the current palette.
got="$(python3 - "$tool" <<'PY'
import importlib.machinery
import importlib.util
import json
import sys
import threading
import time
from pathlib import Path

tool_path = sys.argv[1]
spec = importlib.util.spec_from_loader(
    "recolor", importlib.machinery.SourceFileLoader("recolor", tool_path))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)

colors = Path.home() / ".cache/ryoku/colors.json"
colors.write_text(json.dumps({"primary": "#111111"}))
left_ptr = m.INSTALL / m.THEME / "cursors/left_ptr"
left_ptr.parent.mkdir(parents=True, exist_ok=True)
left_ptr.write_text("stub")
m.display_sizes = lambda: [32]
m.apply_live = lambda: None

completed = []
def build_x11(body, primary, watch, sizes=None, abort=None, nice=False):
    end = time.time() + 3.0
    while time.time() < end:
        if abort and abort():
            raise m.BuildAborted()
        time.sleep(0.05)
    completed.append(primary)

m.build_x11 = build_x11
tf = threading.Thread(target=m._full_build)
tf.start()
time.sleep(0.3)
colors.write_text(json.dumps({"primary": "#222222"}))
tf.join()
print(json.dumps({"completed": completed,
                  "built": m.BUILT.read_text().strip()}))
PY
)"
[[ $(jq -c .completed <<<"$got") == '["#222222"]' ]] || fail "stale full build was not aborted: $got"
[[ $(jq -r .built <<<"$got") == "#222222" ]] || fail "built marker does not carry the newest accent: $got"

# 9. A pass whose accent already landed skips the raster pass entirely.
got="$(python3 - "$tool" <<'PY'
import importlib.machinery
import importlib.util
import json
import sys
from pathlib import Path

tool_path = sys.argv[1]
spec = importlib.util.spec_from_loader(
    "recolor", importlib.machinery.SourceFileLoader("recolor", tool_path))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)

Path.home().joinpath(".cache/ryoku/colors.json").write_text(
    json.dumps({"primary": "#123456"}))
left_ptr = m.INSTALL / m.THEME / "cursors/left_ptr"
left_ptr.parent.mkdir(parents=True, exist_ok=True)
left_ptr.write_text("stub")
m.BUILT.write_text("#123456")
m.FAST.write_text("#123456")

builds = []
m.build_x11 = lambda *a, **k: builds.append(1)
m.apply_live = lambda: builds.append("apply")
m.x11_pass()
print(json.dumps({"builds": builds}))
PY
)"
[[ $(jq -c .builds <<<"$got") == '[]' ]] || fail "already-built accent rasterised again: $got"

echo "cursor-recolor: store shape, DYNAMIC role, flag scan, X11 coalescing all hold"
