#!/usr/bin/env bash

# Reports whether the workspace-tracker KWin effect the installer enabled is
# actually loaded. KWin promises binary effects no ABI stability, so after a KDE
# point release it refuses the binary built against the old libkwin: per-output
# desktops and the swipe offset stop arriving, and nothing on screen says why.
#
# Exit status: 0 when the effect is loaded or this install does not use it, 3
# when it is enabled but absent, so the caller can tell the user to rebuild.

set -euo pipefail

# Without these there is no way to ask, and a missing tool must not be reported
# as a missing effect.
command -v kreadconfig6 >/dev/null 2>&1 || exit 0
command -v qdbus6 >/dev/null 2>&1 || exit 0

enabled="$(kreadconfig6 --file kwinrc --group Plugins --key kwin_workspace_trackerEnabled 2>/dev/null || true)"
[[ "$enabled" == "true" ]] || exit 0

# The effect registers this object on KWin's bus as it loads.
qdbus6 org.kde.KWin /Caelestia/Workspaces org.freedesktop.DBus.Introspectable.Introspect >/dev/null 2>&1 || exit 3

exit 0
