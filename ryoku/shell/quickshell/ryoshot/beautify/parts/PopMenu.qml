pragma ComponentBehavior: Bound

import QtQuick
import "../../look"

// The panel's one drop-down: a combo row's popover, a ComboBox's list, the
// "For" menu. It covers the output while open, so a click anywhere else closes
// it, and tells the studio, so Esc closes it instead of leaving.
Item {
    id: host

    required property var studio
    property real u: 1
    property bool open: false
    property Item anchorItem: null
    // [{ kind: "item" | "heading", k, l }]
    property var entries: []
    property string current: ""
    property real minWidth: 0
    property var pick: null

    function show(anchor, entries, current, minWidth, pick) {
        host.anchorItem = anchor;
        host.entries = entries;
        host.current = current;
        host.minWidth = minWidth;
        host.pick = pick;
        host.open = true;
        host.studio.popupOpen = true;
    }
    function close() {
        host.open = false;
        if (host.studio)
            host.studio.popupOpen = false;
    }

    Connections {
        target: host.studio
        ignoreUnknownSignals: true
        function onClosePopupRequested() {
            host.close();
        }
    }

    MouseArea {
        anchors.fill: parent
        enabled: host.open
        acceptedButtons: Qt.AllButtons
        onPressed: host.close()
        onWheel: (w) => host.close()
    }

    Menu {
        id: menu
        u: host.u
        open: host.open
        anchor: host.anchorItem
        bounds: host
        preferAbove: false
        minWidth: Math.max(168, host.minWidth / host.u)
        onDismissed: host.close()

        Repeater {
            model: host.entries
            delegate: MenuItem {
                required property var modelData
                u: host.u
                kind: modelData.kind || "item"
                text: modelData.l
                checkable: Look.adwaita || Look.fluent ? "check" : "radio"
                checked: modelData.k === host.current
                onTriggered: if (host.pick) host.pick(modelData.k)
            }
        }
    }
}
