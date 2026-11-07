#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$here/../.."
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/Ryoku" "$work/bin"
ln -s "$repo/ryoku/ui" "$work/Ryoku/Ui"
cp -a "$repo/ryoku/apps/ryostore/quickshell" "$work/ryostore"
cp "$here/ryostore-shell-probe.qml" "$work/probe.qml"
cat >"$work/bin/ryostore" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
if [[ ${1:-} != catalog ]]; then
    echo "unexpected ryostore command: $*" >&2
    exit 2
fi
cat <<'JSON'
{"generatedAt":"2026-07-30T00:00:00Z","offline":false,"categories":[{"id":"rices","name":"Rices","group":"wear","description":"Desktop looks","count":1,"installedCount":1},{"id":"lockscreens","name":"Lockscreens","group":"wear","description":"Lock themes","count":1,"installedCount":1},{"id":"barstyles","name":"Bar styles","group":"wear","description":"Shell bars","count":3,"installedCount":3},{"id":"fastfetch","name":"Fastfetch","group":"wear","description":"Terminal dossiers","count":0,"installedCount":0},{"id":"plugins","name":"Plugins","group":"extend","description":"Shell extensions","count":1,"installedCount":1},{"id":"bundles","name":"Bundles","group":"extend","description":"Curated tool sets","count":1,"installedCount":1}],"items":[{"id":"paper","category":"rices","name":"Paper","summary":"Warm paper","installed":true,"active":true,"enabled":false,"installedCount":0,"totalCount":0,"updateAvailable":false},{"id":"market","category":"plugins","name":"Market","summary":"Widget source","installed":true,"active":false,"enabled":true,"installedCount":0,"totalCount":0,"updateAvailable":true},{"id":"creator","category":"bundles","name":"Creator","summary":"Creative tools","installed":false,"active":false,"enabled":false,"installedCount":2,"totalCount":4,"updateAvailable":false},{"id":"clock","category":"lockscreens","name":"Clockwork","summary":"Mechanical lock","installed":true,"active":false,"enabled":false,"installedCount":0,"totalCount":0,"updateAvailable":false},{"id":"sample-one","category":"barstyles","name":"Sample One","installed":true,"active":false,"enabled":false,"installedCount":0,"totalCount":0,"updateAvailable":false},{"id":"sample-two","category":"barstyles","name":"Sample Two","installed":true,"active":false,"enabled":false,"installedCount":0,"totalCount":0,"updateAvailable":false},{"id":"sample-three","category":"barstyles","name":"Sample Three","installed":true,"active":true,"enabled":false,"installedCount":0,"totalCount":0,"updateAvailable":false}]}
JSON
FAKE
chmod +x "$work/bin/ryostore"

run_probe() {
    local size="$1"
    PATH="$work/bin:$PATH" \
    RYOSTORE_PROBE_SIZE="$size" \
    QML2_IMPORT_PATH="$work:${QML2_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}" \
        timeout 20 qs -p "$work/probe.qml" >"$work/log" 2>&1 || true
    if ! grep -q RYOSTORE-SHELL-PROBE-PASS "$work/log"; then
        sed -n '1,160p' "$work/log"
        exit 1
    fi
    if grep -Eq ' ERROR|TypeError|ReferenceError' "$work/log"; then
        sed -n '1,160p' "$work/log"
        exit 1
    fi
}

run_probe 1180x760
run_probe 980x640
echo "ryostore-shell-probe: Discover and Library showroom states"
