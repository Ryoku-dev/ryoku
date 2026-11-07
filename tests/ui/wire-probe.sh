#!/usr/bin/env bash
# Does the Hub still write what the UI says it wrote?
#
# A setting that renders but does not persist is worse than one that is
# missing: it lies. This drives the real FileView + JsonAdapter contract
# against a copy of the live shell.json, edits a real, a nested object and a
# set, flushes, and reads the file back.
#
#   tests/ui/wire-probe.sh
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

src="${XDG_CONFIG_HOME:-$HOME/.config}/ryoku/shell.json"
[ -f "$src" ] || { echo "no $src to probe against"; exit 77; }

mkdir -p "$work/cfg" "$work/qs"
cp "$src" "$work/cfg/shell.json"
cp "$here/wire-probe.qml" "$work/qs/shell.qml"

RYOKU_TEST_CFG="$work/cfg" \
QML_IMPORT_PATH="${QML_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}" \
  timeout 20 qs -p "$work/qs" >"$work/log" 2>&1 || true

grep -q FLUSHED "$work/log" || { echo "FAIL: never flushed"; sed -n '1,20p' "$work/log"; exit 1; }

python3 - "$work/cfg/shell.json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))
frame_bars = d.get("frameBars")
want = {
    "frameBorder": 88,
    "barStyle": "kairos",
    "frameBars.rails.top.size": 38,
    "frameBars.rails.left.center": ["dock"],
    "sidebarRightPanes": ["notifications", "calendar"],
}
got = {
    "frameBorder": d.get("frameBorder"),
    "barStyle": d.get("barStyle"),
    "frameBars.rails.top.size": frame_bars.get("rails", {}).get("top", {}).get("size") if isinstance(frame_bars, dict) else None,
    "frameBars.rails.left.center": frame_bars.get("rails", {}).get("left", {}).get("center") if isinstance(frame_bars, dict) else None,
    "sidebarRightPanes": d.get("sidebarRightPanes"),
}
bad = [k for k, v in want.items() if got[k] != v]
for k, v in want.items():
    print("  %-29s %-34s %s" % (k, json.dumps(got[k]), "ok" if got[k] == v else "MISMATCH, wanted " + json.dumps(v)))
sys.exit(1 if bad else 0)
PY
echo "wire-probe: the adapter writes what the UI set"
