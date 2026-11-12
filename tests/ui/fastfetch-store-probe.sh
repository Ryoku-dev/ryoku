#!/usr/bin/env bash
# Settings > Terminal: an installed RyoStore greeting style is listed and choosing it runs the apply command
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/Ryoku" "$work/ryoku/hub" "$work/bin" "$work/home" "$work/cfg" "$work/run"
chmod 700 "$work/run"
ln -s "$repo/ryoku/ui" "$work/Ryoku/Ui"
cp -a "$repo/ryoku/hub/quickshell" "$work/ryoku/hub"
cp "$here/fastfetch-store-probe.qml" "$work/ryoku/probe.qml"

# stand-ins: reads answer from fixtures, every other call is logged and does nothing
cat >"$work/bin/ryostore" <<SH
#!/bin/sh
echo "ryostore \$*" >>"$work/calls"
[ "\$1" = catalog ] && echo '{"items":[{"id":"probe-readout","category":"fastfetch","name":"Probe Readout","installed":true},{"id":"not-here","category":"fastfetch","name":"Not Installed","installed":false}]}'
exit 0
SH
cat >"$work/bin/ryoku-hub" <<SH
#!/bin/sh
echo "ryoku-hub \$*" >>"$work/calls"
case "\$1 \$2" in
"shell get") echo '{"current":"bash","choices":[{"key":"bash","label":"Bash","installed":true}],"zshPrompt":""}' ;;
"starship get") echo '{"layout":"pill","palette":"fixed"}' ;;
"starship list") echo '["pill"]' ;;
"fastfetch get") echo '{"logo":{"kind":"builtin","source":"","width":28,"height":14,"padding":3,"dither":false},"accent":"226;52;42","palette":"fixed","rows":[{"kind":"module","enabled":true,"label":"OS","module":"os","key":"OS"}]}' ;;
esac
exit 0
SH
chmod +x "$work/bin/ryostore" "$work/bin/ryoku-hub"

# the Ryoku QML modules a dev box installs live under the real home, not the probe's
user_qml="${QML2_IMPORT_PATH:-$HOME/.local/lib/qt6/qml}"
PATH="$work/bin:$PATH" HOME="$work/home" XDG_CONFIG_HOME="$work/cfg" XDG_RUNTIME_DIR="$work/run" \
    QML2_IMPORT_PATH="$work:$repo/docs/lithic/library/qml:$user_qml" \
    QT_QPA_PLATFORM=offscreen timeout 30 qs -p "$work/ryoku/probe.qml" >"$work/log" 2>&1 || true
fail() { echo "fastfetch-store-probe: $1"; sed -n '1,120p' "$work/log"; exit 1; }
grep -q FASTFETCH-STORE-PROBE-PASS "$work/log" || fail "no pass line"
grep -Eq ' ERROR|TypeError|ReferenceError|is not a type' "$work/log" && fail "errors logged"
grep -qx 'ryostore internal apply-fastfetch probe-readout' "$work/calls" || { cat "$work/calls"; fail "the apply command did not run"; }
echo "fastfetch-store-probe: installed style listed, choosing it applies it"
