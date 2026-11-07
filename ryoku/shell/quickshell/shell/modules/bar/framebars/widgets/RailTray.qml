pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import shell.services
import "../.." as Pill

// System tray. State comes from the daemon `tray` topic (Tray singleton), never
// D-Bus directly. The root self-hides while there are no items; a toggle button
// reveals/collapses the item strip, and each item shows the server-resolved icon
// (iconPath file, else iconName theme lookup, else the generic fallback). Left
// click asks the item to act; right click opens the item's own dbusmenu in-shell
// (Pill.TrayMenu), flying out from the rail under the icon. Contract 04
// sec 2.1, 3.2 (system_tray, system_tray_item).
//
// Design: 4px rhythm gaps between cells, hover wash (on-surface 8%),
// press dip (scale 0.88 + opacity 0.72), smooth color behavior. The reveal
// strip sits behind an outlined container.
Item {
    id: root

    required property string edge
    required property real scale

    readonly property var items: Tray.items
    readonly property bool horizontal: edge === "top" || edge === "bottom"
    readonly property real cross: 48 * scale
    property bool revealed: false

    readonly property bool selfShown: items.length > 0
    visible: selfShown
    implicitWidth: selfShown ? (horizontal ? line.implicitWidth : cross) : 0
    implicitHeight: selfShown ? (horizontal ? cross : line.implicitHeight) : 0

    function itemSource(it) {
        if (it.iconPath && it.iconPath.length > 0)
            return it.iconPath.indexOf("/") === 0 ? ("file://" + it.iconPath) : it.iconPath;
        if (it.iconName && it.iconName.length > 0)
            return Icons.path(it.iconName, "application-x-executable-symbolic");
        return Icons.path("application-x-executable-symbolic", true);
    }

    Loader {
        id: line
        anchors.centerIn: parent
        sourceComponent: root.horizontal ? rowComp : colComp
    }

    Component {
        id: rowComp
        Row {
            spacing: 0
            RailButton {
                edge: root.edge
                scale: root.scale
                icon: "tray"
                onClicked: root.revealed = !root.revealed
            }
            // Outlined reveal strip: clipped so it stays anchored to the edge.
            Item {
                clip: true
                height: root.cross
                width: root.revealed ? (strip.implicitWidth + 2) : 0
                Behavior on width {
                    NumberAnimation {
                        duration: Motion.barReveal
                        easing.type: Motion.barRevealCurve
                    }
                }
                Rectangle {
                    anchors.fill: strip
                    anchors.margins: -2 * root.scale
                    radius: Theme.radiusWidget
                    color: "transparent"
                    border.width: root.revealed ? 1 : 0
                    border.color: Qt.rgba(Theme.outline.r, Theme.outline.g, Theme.outline.b, 0.28)
                }
                Row {
                    id: strip
                    anchors.centerIn: parent
                    spacing: 4 * root.scale
                    Repeater { model: root.items; delegate: itemComp }
                }
            }
        }
    }
    Component {
        id: colComp
        Column {
            spacing: 0
            RailButton {
                edge: root.edge
                scale: root.scale
                icon: "tray"
                onClicked: root.revealed = !root.revealed
            }
            // Outlined reveal strip for a vertical rail.
            Item {
                clip: true
                width: root.cross
                height: root.revealed ? (strip.implicitHeight + 2) : 0
                Behavior on height {
                    NumberAnimation {
                        duration: Motion.barReveal
                        easing.type: Motion.barRevealCurve
                    }
                }
                Rectangle {
                    anchors.fill: strip
                    anchors.margins: -2 * root.scale
                    radius: Theme.radiusWidget
                    color: "transparent"
                    border.width: root.revealed ? 1 : 0
                    border.color: Qt.rgba(Theme.outline.r, Theme.outline.g, Theme.outline.b, 0.28)
                }
                Column {
                    id: strip
                    anchors.centerIn: parent
                    spacing: 4 * root.scale
                    Repeater { model: root.items; delegate: itemComp }
                }
            }
        }
    }

    Component {
        id: itemComp
        Item {
            id: cell
            required property var modelData

            // Along-bar slot: minimum 36px; icon (iconSm=16) + 20px pad = 36px exact.
            // Using iconSm keeps tray icons compact relative to dock/status icons.
            readonly property real iconPx: Theme.iconSm * root.scale
            readonly property real along: 36 * root.scale
            width: root.horizontal ? along : root.cross
            height: root.horizontal ? root.cross : along

            // Smooth hover wash
            Rectangle {
                anchors.fill: parent
                anchors.margins: 2 * root.scale
                radius: Theme.radiusWidget
                color: area.containsMouse
                    ? Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b,
                              area.pressed ? 0.14 : 0.08)
                    : "transparent"
                Behavior on color {
                    ColorAnimation { duration: Motion.fast; easing.type: Motion.easeStandard }
                }
            }

            Image {
                anchors.centerIn: parent
                width: cell.iconPx
                height: cell.iconPx
                sourceSize.width: width
                sourceSize.height: height
                smooth: true
                asynchronous: true
                source: root.itemSource(cell.modelData)
                // Press dip: slight scale-down on click for tactile feel.
                scale: area.pressed ? 0.82 : 1.0
                Behavior on scale {
                    NumberAnimation { duration: 80; easing.type: Easing.OutCubic }
                }
                opacity: area.pressed ? 0.72 : 1.0
                Behavior on opacity {
                    NumberAnimation { duration: 80; easing.type: Easing.OutCubic }
                }
            }

            MouseArea {
                id: area
                anchors.fill: parent
                acceptedButtons: Qt.LeftButton | Qt.RightButton
                hoverEnabled: true
                cursorShape: Qt.PointingHandCursor
                onClicked: event => {
                    const it = cell.modelData;
                    if (event.button === Qt.LeftButton) {
                        const g = cell.mapToGlobal(0, cell.height);
                        Tray.activate(it.service, Math.round(g.x), Math.round(g.y));
                    } else if (it.menu) {
                        trayMenu.openFor(it, cell);
                    } else {
                        const g = cell.mapToGlobal(0, cell.height);
                        Tray.contextMenu(it.service, Math.round(g.x), Math.round(g.y));
                    }
                }
            }
        }
    }
    Pill.TrayMenu {
        id: trayMenu
        edge: root.edge
        scale: root.scale
    }
}
