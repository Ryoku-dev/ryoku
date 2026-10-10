pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import shell.services
import "../../../../components"

// The dock. Order is pinned classes first (a pinned app may have zero clients),
// then running unpinned classes by pid (the host hands clients pre-sorted). Each
// item is a 48x44 button with a 24px glyph resolved from the shell icon set.
// The running indicator overlays the outer edge: zero clients none,
// one to three clients that many 4x4 dots, four or more a single line (16x4 on a
// horizontal bar, 4x16 on a vertical bar). Left click focuses/cycles/launches
// (host-driven); right click toggles pin. No middle click, no hover preview, no
// tooltip. Contract 03 sec 2.1/2.2/3.3/4.1.
Item {
    id: root

    required property var clients        // [{ className, address, pid }], pid-sorted
    required property string activeClass
    required property string edge
    required property real scale
    signal activate(string className)

    readonly property bool horizontal: edge === "top" || edge === "bottom"
    readonly property real cross: 48 * scale
    readonly property var sharedPins: Dock.pinnedOrStarter()
    readonly property var classes: Dock.resolve(sharedPins, clients)
    // space the zone can give the dock; -1 = unbounded. When the natural strip
    // would not fit, every item shrinks together, macOS style.
    // ponytail: floored at 0.55 (a 24px tile); past that the strip overlaps
    // its neighbour zones and the zone's z-order keeps the dock on top.
    property real maxExtent: -1
    readonly property real itemNatural: Math.max(36 * scale, Math.min(Theme.iconMd, cross - 8) + 20 * scale)
    readonly property real fit: maxExtent > 0 && classes.length > 0
        ? Math.max(0.55, Math.min(1, maxExtent / (classes.length * itemNatural))) : 1
    readonly property var counts: {
        const c = {};
        const list = Array.isArray(clients) ? clients : [];
        for (let i = 0; i < list.length; ++i) {
            const cl = list[i] && list[i].className;
            if (cl)
                c[cl] = (c[cl] || 0) + 1;
        }
        return c;
    }

    implicitWidth: horizontal ? dock.implicitWidth : Math.max(cross, dock.implicitWidth)
    implicitHeight: horizontal ? Math.max(cross, dock.implicitHeight) : dock.implicitHeight

    Loader {
        id: dock
        anchors.centerIn: parent
        sourceComponent: root.horizontal ? rowComp : colComp
    }

    Component {
        id: rowComp
        Row {
            spacing: 0
            Repeater { model: root.classes; delegate: itemComp }
        }
    }
    Component {
        id: colComp
        Column {
            spacing: 0
            Repeater { model: root.classes; delegate: itemComp }
        }
    }

    Component {
        id: itemComp
        Item {
            id: item
            required property string modelData
            readonly property string className: modelData
            readonly property int count: root.counts[className] || 0
            readonly property color fg: Theme.onSurface
            // Same rule the rail glyphs follow: the tile scales with the band,
            // the app icon inside it only shrinks once the band is too thin to
            // hold it, so a 32px dock does not render visibly smaller icons than
            // a 48px one.
            readonly property real iconPx: Math.min(Theme.iconMd, root.cross - 8) * root.fit

            width: root.horizontal ? root.itemNatural * root.fit : root.cross
            height: root.horizontal ? root.cross : root.itemNatural * root.fit

            // Docker-style items: the real app icon, in colour, resolved from
            // the desktop entry and the system icon themes (user decision). The
            // app-grid glyph is only the last resort when nothing resolves.
            // Shared with the first-class dock (services/Dock.qml): resolves the
            // desktop-entry icon then the class name, and re-resolves reactively so a
            // transient miss at boot or after an app update heals without a reload.
            readonly property string iconSource: Dock.iconFor(className)

            Rectangle {
                anchors.fill: parent
                radius: Theme.radiusWidget
                color: area.containsMouse
                    ? Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.08)
                    : "transparent"
            }

            Image {
                id: dockIcon
                anchors.centerIn: parent
                width: item.iconPx
                height: item.iconPx
                visible: item.iconSource !== "" && status === Image.Ready
                source: item.iconSource
                sourceSize.width: width
                sourceSize.height: height
                smooth: true
                asynchronous: true
            }

            SymbolIcon {
                anchors.centerIn: parent
                visible: !dockIcon.visible
                name: "view-app-grid"
                size: item.iconPx
                color: item.fg
            }

            // Running indicator on the bar's outer edge (contract 03 sec 2.2):
            // its orientation follows the bar, dots for 1..3 clients, one line for
            // 4 or more. Non-interactive.
            Item {
                anchors.margins: 4 * root.scale
                anchors.left: root.edge === "left" ? parent.left : undefined
                anchors.right: root.edge === "right" ? parent.right : undefined
                anchors.top: root.edge === "top" ? parent.top : undefined
                anchors.bottom: root.edge === "bottom" ? parent.bottom : undefined
                anchors.horizontalCenter: root.horizontal ? parent.horizontalCenter : undefined
                anchors.verticalCenter: !root.horizontal ? parent.verticalCenter : undefined
                width: childrenRect.width
                height: childrenRect.height

                Loader {
                    active: item.count >= 1 && item.count <= 3
                    sourceComponent: root.horizontal ? dotRow : dotColumn
                }
                Rectangle {
                    visible: item.count >= 4
                    width: root.horizontal ? 16 * root.scale : 4 * root.scale
                    height: root.horizontal ? 4 * root.scale : 16 * root.scale
                    radius: 2 * root.scale
                    color: item.fg
                }

                Component {
                    id: dotRow
                    Row {
                        spacing: 2 * root.scale
                        Repeater {
                            model: item.count
                            delegate: Rectangle {
                                width: 4 * root.scale; height: 4 * root.scale
                                radius: width / 2; color: item.fg
                            }
                        }
                    }
                }
                Component {
                    id: dotColumn
                    Column {
                        spacing: 2 * root.scale
                        Repeater {
                            model: item.count
                            delegate: Rectangle {
                                width: 4 * root.scale; height: 4 * root.scale
                                radius: width / 2; color: item.fg
                            }
                        }
                    }
                }
            }

            MouseArea {
                id: area
                anchors.fill: parent
                acceptedButtons: Qt.LeftButton | Qt.RightButton
                hoverEnabled: true
                onClicked: event => {
                    if (event.button === Qt.LeftButton)
                        root.activate(item.className);
                    else
                        Dock.togglePin(item.className);
                }
                onEntered: {
                    if (item.count > 0) {
                        const c = item.mapToGlobal(item.width / 2, item.height / 2);
                        DockPreview.gx = c.x;
                        DockPreview.gy = c.y;
                        DockPreview.edge = root.edge;
                        DockPreview.hoveredClass = item.className;
                    }
                }
                onExited: if (DockPreview.hoveredClass === item.className) DockPreview.hoveredClass = "";
            }
        }
    }
}
