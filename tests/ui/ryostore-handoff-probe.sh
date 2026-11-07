#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$here/../.."
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/Ryoku" "$work/bin" "$work/shell/quickshell/plugins" "$work/cfg/ryoku"
ln -s "$repo/ryoku/ui" "$work/Ryoku/Ui"
ln -s "$repo/ryoku/shell/framebars" "$work/Ryoku/FrameBars"
cp -a "$repo/ryoku/hub/quickshell" "$work/hub"
cp "$here/ryostore-handoff-probe.qml" "$work/probe.qml"
printf '%s\n' '{"barStyle":"orpheus"}' >"$work/cfg/ryoku/shell.json"
printf '%s\n' '[]' >"$work/shell/quickshell/plugins/discover-data.json"
cat >"$work/shell/quickshell/plugins/discover.sh" <<'DISCOVER'
#!/usr/bin/env bash
cat <<'JSON'
[{"id":"market","manifest":{"name":"Market","version":"1.0.0","metadata":{"settings":[]}},"placement":{"enabled":true,"host":"framePopout"}}]
JSON
DISCOVER
chmod +x "$work/shell/quickshell/plugins/discover.sh"

cat >"$work/bin/ryostore" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
printf 'ryostore %s\n' "$*" >>"${RYOSTORE_HANDOFF_LOG:?}"
if [[ ${1:-} == open ]]; then exit 0; fi
if [[ ${1:-} != catalog || ${2:-} != --category ]]; then
    echo "unexpected ryostore command: $*" >&2
    exit 2
fi
case "${3:-}" in
lockscreens) cat <<'JSON'
{"items":[{"id":"clockwork","category":"lockscreens","name":"Clockwork","art":"","installed":true,"active":true,"metadata":{"theme":"Clockwork"}}]}
JSON
;;
plugins) cat <<'JSON'
{"items":[{"id":"market","category":"plugins","name":"Market","version":"2.0.0","installed":true,"enabled":true,"updateAvailable":true}]}
JSON
;;
bundles) cat <<'JSON'
{"items":[{"id":"creator","category":"bundles","name":"Creator","installed":false,"installedCount":1,"totalCount":2,"metadata":{"items":[{"type":"package","name":"editor","installed":true},{"type":"package","name":"paint","installed":false}]}}]}
JSON
;;
barstyles) cat <<'JSON'
{"items":[{"id":"sample-one","category":"barstyles","name":"Sample One","summary":"First sample","installed":true,"active":true},{"id":"sample-two","category":"barstyles","name":"Sample Two","summary":"Second sample","installed":true},{"id":"sample-three","category":"barstyles","name":"Sample Three","summary":"Third sample","installed":true}]}
JSON
;;
*) printf '%s\n' '{"items":[]}' ;;
esac
FAKE

cat >"$work/bin/ryoku-hub" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
printf 'ryoku-hub %s\n' "$*" >>"${RYOSTORE_HANDOFF_LOG:?}"
if [[ ${1:-} == extras || ${1:-} == lock && ${2:-} =~ ^(catalog|install|cache)$ || ${1:-} == rice && ${2:-} =~ ^(catalog|install)$ ]]; then
    printf 'forbidden %s\n' "$*" >>"${RYOSTORE_HANDOFF_LOG:?}"
    exit 9
fi
case "${1:-} ${2:-} ${3:-}" in
"rice list ") printf '%s\n' '[]' ;;
"rice preflight ") printf '%s\n' '{}' ;;
"hypr matugen get") printf '%s\n' '{}' ;;
"fastfetch get ") printf '%s\n' '{"logo":{"kind":"none","source":"","width":28,"height":14,"padding":3},"accent":"226;52;42","rows":[]}' ;;
*) printf '%s\n' '{}' ;;
esac
FAKE

cat >"$work/bin/ryoku" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' '{"mode":"ask","daemon":false,"default":{"name":"","format":"absent"},"notes":[]}'
FAKE
cat >"$work/bin/brightnessctl" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' 'backlight,amdgpu_bl1,100,50,50%'
FAKE
cat >"$work/bin/ryoku-cmd-nightlight" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' 'off 4500'
FAKE
cat >"$work/bin/kitty" <<'FAKE'
#!/usr/bin/env bash
printf 'kitty %s\n' "$*" >>"${RYOSTORE_HANDOFF_LOG:?}"
FAKE
chmod +x "$work/bin/"*

: >"$work/commands"
PATH="$work/bin:$PATH" \
RYOSTORE_HANDOFF_LOG="$work/commands" \
RYOKU_SHELL_DIR="$work/shell" \
RYOKU_SCRIPTS_DIR="$work/bin/" \
XDG_CONFIG_HOME="$work/cfg" \
QML2_IMPORT_PATH="$work:${QML2_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}" \
    timeout 25 qs -p "$work/probe.qml" >"$work/log" 2>&1 || true
if ! grep -q RYOSTORE-HANDOFF-PROBE-PASS "$work/log"; then
    sed -n '1,220p' "$work/log"
    exit 1
fi
if grep -q '^forbidden ' "$work/commands"; then
    cat "$work/commands"
    exit 1
fi
for category in rices lockscreens plugins bundles barstyles fastfetch; do
    grep -q "^ryostore open $category$" "$work/commands" || { echo "missing RyoStore handoff: $category"; cat "$work/commands"; exit 1; }
done
grep -q '^kitty --class ryostore -e ryostore-install remove item creator editor$' "$work/commands" || { cat "$work/commands"; exit 1; }
grep -q '^kitty --class ryostore -e ryostore-install remove bundle creator$' "$work/commands" || { cat "$work/commands"; exit 1; }
echo "ryostore-handoff-probe: Settings manages installed state through RyoStore"
