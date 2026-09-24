pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import "../lib/providers.js" as Providers

// Reads the active workspace's tiled layout and applies a chosen one to it.
// Gated on caps.tiledLayout so a compositor without a tiled-layout concept shows
// no control. `active` mirrors the owning menu's open state; `stopped()` fires
// when it closes.
Item {
    id: root

    property bool active: false
    readonly property var layouts: Providers.layouts
    property string current: ""
    readonly property bool available: Wm.caps.tiledLayout === true
    signal stopped()

    onActiveChanged: {
        if (active)
            refresh();
        else
            stop();
    }
    Component.onCompleted: refresh()
    Component.onDestruction: stop()

    Connections {
        target: Wm
        function onFocusedWorkspaceChanged() { if (root.active) root.refresh(); }
    }

    function refresh() {
        if (active)
            root.current = Wm.focusedWorkspace ? (Wm.focusedWorkspace.layout || "") : "";
    }

    // tiled_layout does not update on an empty workspace, so set current
    // optimistically as well as through the live frame.
    function choose(layout) {
        if (!active || !available || !layouts.includes(layout))
            return;
        Wm.setWorkspaceLayout(Wm.focusedWorkspace ? Wm.focusedWorkspace.name : "", layout);
        root.current = layout;
    }

    function stop() {
        stopped();
    }
}
