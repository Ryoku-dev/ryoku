#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$here/../.."
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/Ryoku" "$work/bin"
ln -s "$repo/ryoku/ui" "$work/Ryoku/Ui"
cp -a "$repo/ryoku/apps/ryostore/quickshell" "$work/ryostore"
cp "$here/ryostore-flow-probe.qml" "$work/probe.qml"
cat >"$work/bin/ryostore" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
state="${RYOSTORE_FIXTURE_STATE:?}"
case "${1:-}" in
catalog)
    installed=false
    [[ -f "$state" ]] && installed=true
    cat <<JSON
{"generatedAt":"2026-07-30T00:00:00Z","offline":false,"categories":[{"id":"lockscreens","name":"Lockscreens","group":"wear","description":"Lock themes","count":11,"installedCount":$([[ -f "$state" ]] && echo 1 || echo 0)}],"items":[{"id":"clock","category":"lockscreens","name":"Clock","summary":"Installed clock dossier","installed":$installed,"active":false,"enabled":false,"installedCount":0,"totalCount":0,"updateAvailable":false},{"id":"broken","category":"lockscreens","name":"Broken","summary":"Failure fixture","installed":false,"active":false,"enabled":false,"installedCount":0,"totalCount":0,"updateAvailable":false},{"id":"f1","category":"lockscreens","name":"Fixture 1","installed":false},{"id":"f2","category":"lockscreens","name":"Fixture 2","installed":false},{"id":"f3","category":"lockscreens","name":"Fixture 3","installed":false},{"id":"f4","category":"lockscreens","name":"Fixture 4","installed":false},{"id":"f5","category":"lockscreens","name":"Fixture 5","installed":false},{"id":"f6","category":"lockscreens","name":"Fixture 6","installed":false},{"id":"f7","category":"lockscreens","name":"Fixture 7","installed":false},{"id":"f8","category":"lockscreens","name":"Fixture 8","installed":false},{"id":"f9","category":"lockscreens","name":"Fixture 9","installed":false}]}
JSON
    ;;
install)
    if [[ ${3:-} == broken ]]; then
        echo "fixture install failed" >&2
        exit 9
    fi
    [[ ${2:-} == lockscreens && ${3:-} == clock ]] || { echo "unexpected install: $*" >&2; exit 2; }
    sleep 2
    : >"$state"
    ;;
*)
    echo "unexpected ryostore command: $*" >&2
    exit 2
    ;;
esac
FAKE
chmod +x "$work/bin/ryostore"

PATH="$work/bin:$PATH" \
RYOSTORE_FIXTURE_STATE="$work/installed" \
QML2_IMPORT_PATH="$work:${QML2_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}" \
    timeout 25 qs -p "$work/probe.qml" >"$work/log" 2>&1 || true
if ! grep -q RYOSTORE-FLOW-PROBE-PASS "$work/log"; then
    sed -n '1,200p' "$work/log"
    exit 1
fi
if grep -Eq ' ERROR|TypeError|ReferenceError' "$work/log"; then
    sed -n '1,200p' "$work/log"
    exit 1
fi
echo "ryostore-flow-probe: install, detail, search, and exact context restoration"
