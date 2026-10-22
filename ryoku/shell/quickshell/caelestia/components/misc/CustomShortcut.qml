pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons

// One shell keybind. When the compositor offers a global-shortcuts protocol
// (caps.globalShortcuts) the provider bridge dispatches straight to this
// instance; where it is absent the shortcut stays inert. `key` is kept for the
// callers that set it, the compositor owns the binding.
Item {
    id: root

    property string name
    property string description
    property string key

    signal pressed
    signal released

    property QtObject _bridge: null
    property var _component: null

    onNameChanged: if (root._bridge) root._bridge.name = root.name
    onDescriptionChanged: if (root._bridge) root._bridge.description = root.description

    readonly property bool _canBridge: Wm.caps.globalShortcuts === true
    on_CanBridgeChanged: root._build()
    Component.onCompleted: root._build()

    function _build() {
        if (root._bridge || !Wm.caps.globalShortcuts)
            return;
        if (!root._component)
            root._component = Qt.createComponent("Ryoku.Wm.Hyprland", "GlobalShortcutBridge");
        const c = root._component;
        if (c.status === Component.Loading) {
            c.statusChanged.connect(root._build);
            return;
        }
        if (c.status !== Component.Ready)
            return;
        const b = c.createObject(root, { name: root.name, description: root.description });
        if (!b)
            return;
        // shortcutPressed and shortcutReleased, because GlobalShortcut's `pressed`
        // property shadows its signal of the same name.
        b.shortcutPressed.connect(root.pressed);
        b.shortcutReleased.connect(root.released);
        root._bridge = b;
    }
}
