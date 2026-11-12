#!/usr/bin/env bash
# run the Ryoku shell straight from this repo on the live Hyprland session, no
# install. daemon launches each component with `qs -p <repo>/quickshell/...`
# and quickshell hot-reloads QML edits, so changes show live. your ~/.config
# stays untouched. dev-stop.sh to stop.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
bin="$here/ipc/ryoku-shell"

(cd "$here/ipc" && go build -o ryoku-shell .)

# the frame component imports Ryoku.Blobs from the QML path the daemon sets,
# so build + install it once = dev loop renders the frame like a real deploy.
"$here/plugin/build.sh" "$HOME/.local/lib/qt6/qml"
# Keep the shared pure-QML module in step with this checkout too; HeroCrop and
# the other design-system components are imported by the live launcher.
"$here/../ui/install.sh" "$HOME/.local/lib/qt6/qml"
"$here/../../docs/lithic/library/qml/install.sh" "$HOME/.local/lib/qt6/qml"

# the shared frame-bar schema + catalogs live in Ryoku.FrameBars, imported by
# every config root and the Hub Bar Studio; install it so the dev loop resolves
# the module like a real deploy.
"$here/framebars/install.sh" "$HOME/.local/lib/qt6/qml"

# the language table + catalogs live at ~/.local/share/ryoku/i18n; install them
# so the live shell and Hub resolve a real language list instead of falling back
# to an empty table (the Hub picker would otherwise show only Auto and en_US/en_GB).
"$here/../i18n/tools/install.sh"

export RYOKU_SHELL_DIR="$here"
echo "ryoku-shell dev  (RYOKU_SHELL_DIR=$here)"
echo "  edit anything under $here/quickshell and it reloads live"
echo "  test actions:  $bin <launcher|lock|wallpaper|status>"
echo "  add keybinds:  $here/../hyprland/dev-binds.sh on    (restore yours with: ryoku wm act config.reload)"
echo "  stop:          $here/dev-stop.sh"
exec "$bin" daemon
