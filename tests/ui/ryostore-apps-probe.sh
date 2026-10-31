#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$here/../.."
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/Ryoku" "$work/bin"
ln -s "$repo/ryoku/ui" "$work/Ryoku/Ui"
cp -a "$repo/ryoku/apps/ryostore/quickshell" "$work/ryostore"
cp "$here/ryostore-apps-probe.qml" "$work/probe.qml"

cat >"$work/bin/ryostore" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
catalog)
    cat <<JSON
{"generatedAt":"2026-10-28T00:00:00Z","offline":false,
 "categories":[{"id":"flatpak","name":"Apps","group":"extend","description":"Applications from Flathub, installed with Flatpak.","count":5,"installedCount":0}],
 "items":[
  {"id":"org.example.Gamma","category":"flatpak","name":"Gamma","summary":"Trending first","kind":"flatpak","verificationStatus":"verified","tags":["game"],"listedAt":"2026-01-01T00:00:00Z","metadata":{"provider":"Flathub","trending":9,"installsLastMonth":100,"updatedAt":"2026-01-05T00:00:00Z"}},
  {"id":"org.example.Alpha","category":"flatpak","name":"Alpha","summary":"Middle of the road","kind":"flatpak","verificationStatus":"unverified","tags":["network"],"listedAt":"2026-01-03T00:00:00Z","metadata":{"provider":"Flathub","trending":7,"installsLastMonth":500,"updatedAt":"2026-01-02T00:00:00Z"}},
  {"id":"org.example.Beta","category":"flatpak","name":"Beta","summary":"Most installed","kind":"flatpak","verificationStatus":"verified","tags":["graphics"],"listedAt":"2026-01-02T00:00:00Z","metadata":{"provider":"Flathub","trending":5,"installsLastMonth":900,"updatedAt":"2026-01-04T00:00:00Z"}},
  {"id":"org.example.Epsilon","category":"flatpak","name":"Epsilon","summary":"Newest listing","kind":"flatpak","verificationStatus":"unverified","tags":["game","audiovideo"],"listedAt":"2026-01-04T00:00:00Z","metadata":{"provider":"Flathub","trending":3,"installsLastMonth":300,"updatedAt":"2026-01-03T00:00:00Z"}},
  {"id":"org.example.Delta","category":"flatpak","name":"Delta","summary":"Recently added","kind":"flatpak","verificationStatus":"unverified","listedAt":"2026-01-05T00:00:00Z","metadata":{"provider":"Flathub","trending":1,"installsLastMonth":700,"updatedAt":"2026-01-01T00:00:00Z"}}
 ]}
JSON
    ;;
check)
    exit 1
    ;;
*)
    echo "unexpected ryostore command: $*" >&2
    exit 2
    ;;
esac
FAKE
chmod +x "$work/bin/ryostore"

PATH="$work/bin:$PATH" \
QML2_IMPORT_PATH="$work:${QML2_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}" \
    timeout 30 qs -p "$work/probe.qml" >"$work/log" 2>&1 || true
if ! grep -q RYOSTORE-APPS-PROBE-PASS "$work/log"; then
    sed -n '1,200p' "$work/log"
    exit 1
fi
if grep -Eq '(^|[[:space:]])ERROR|TypeError|ReferenceError' "$work/log"; then
    sed -n '1,200p' "$work/log"
    exit 1
fi
echo "ryostore-apps-probe: hero, sort strip, compact grid, detail restore"
