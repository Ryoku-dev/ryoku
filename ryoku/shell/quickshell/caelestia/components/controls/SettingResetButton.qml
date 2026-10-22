pragma ComponentBehavior: Bound

import QtQuick
import caelestia.components.controls

IconButton {
    id: root

    property var options

    readonly property bool active: root.options !== null && root.options !== undefined
    readonly property bool isCustom: root.active && root.options.customGet !== undefined
    readonly property var current: {
        if (!root.active)
            return null;
        if (root.isCustom)
            return root.options.customGet();
        return root.options.node[root.options.setting];
    }
    readonly property var fallback: {
        if (!root.active)
            return null;
        if (root.isCustom)
            return root.options.customDef;
        return root.options.node.descriptorFor(root.options.setting).defaultValue;
    }
    readonly property bool dirty: {
        if (!root.active)
            return false;
        if (typeof root.current === "number" && typeof root.fallback === "number")
            return Math.abs(root.current - root.fallback) > 0.005;
        return root.current !== root.fallback;
    }

    property bool latched: false

    function reset(): void {
        if (!root.active)
            return;
        if (root.isCustom) {
            root.options.customSet(root.options.customDef);
        } else {
            root.options.node.resetOption(root.options.setting);
        }
        root.latched = false;
    }

    onDirtyChanged: {
        if (root.dirty)
            root.latched = true;
    }

    // Always occupy its slot at the right edge so rows never shift when it
    // appears; dimmed and inert until the value differs from the default.
    visible: root.active
    disabled: !(root.dirty || root.latched)
    icon: "restart_alt"
    type: IconButton.Text
    onClicked: root.reset()
}
