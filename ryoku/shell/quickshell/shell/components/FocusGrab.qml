pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons

// Neutral focus grab. Loads the provider bridge when the compositor offers a
// focus-grab protocol (caps.focusGrab); otherwise stays inert and the caller
// supplies its own scrim fallback (see `available`). Set active/windows and
// handle the cleared signal.
Item {
    id: root

    property bool active: false
    property var windows: []
    signal cleared

    readonly property bool available: Wm.caps.focusGrab === true

    property QtObject _bridge: null
    property var _component: null

    onActiveChanged: if (root._bridge) root._bridge.active = root.active
    onWindowsChanged: if (root._bridge) root._bridge.windows = root.windows

    // A property binding re-evaluates whenever Wm.caps arrives, so this cannot
    // miss the capability the way a one-shot ready handler did.
    readonly property bool _canBridge: Wm.caps.focusGrab === true
    on_CanBridgeChanged: root._build()
    Component.onCompleted: root._build()

    function _build() {
        if (root._bridge || !Wm.caps.focusGrab)
            return;
        if (!root._component)
            root._component = Qt.createComponent("Ryoku.Wm.Hyprland", "FocusGrabBridge");
        const c = root._component;
        if (c.status === Component.Loading) {
            c.statusChanged.connect(root._build);
            return;
        }
        if (c.status !== Component.Ready)
            return;
        const b = c.createObject(root, { active: root.active, windows: root.windows });
        if (!b)
            return;
        b.cleared.connect(root.cleared);
        root._bridge = b;
    }
}
