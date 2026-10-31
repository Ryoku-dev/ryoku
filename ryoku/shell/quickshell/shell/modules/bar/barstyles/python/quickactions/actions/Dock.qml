import QtQuick
import QtQuick.Layouts
import Quickshell
import "../../"
import "../../reusables"
import shell.services as Ryoku

Item {
    id: root

    property var requestedLayoutTemplate: []
    property bool isActiveTab: typeof isCurrentTarget !== "undefined" ? isCurrentTarget : true
    property string safeActiveEdge: typeof activeEdge !== "undefined" ? activeEdge : "left"

    function s(val) {
        return typeof scaleFunc === "function" ? scaleFunc(val) : val;
    }

    property int maxApps: 10
    property real iconSize: s(42)
    property real itemSpacing: s(6)
    property real dockPadding: s(8)

    readonly property var dockApps: Ryoku.Dock.pinnedOrStarter().slice(0, maxApps)
    property real dockSpan: dockApps.length > 0 ? (dockApps.length * iconSize) + ((dockApps.length - 1) * itemSpacing) + (dockPadding * 2) : s(90)

    property real preferredWidth: dockThickness + s(6)
    property real preferredExtraLength: typeof floatingWidget !== "undefined" ? Math.max(0, dockSpan - floatingWidget.baseSidebarH + s(12)) : dockSpan

    property real counterRotation: {
        if (root.safeActiveEdge === "right") return 180;
        if (root.safeActiveEdge === "bottom") return 90;
        if (root.safeActiveEdge === "top") return -90;
        return 0;
    }

    property bool isHorizontal: root.safeActiveEdge === "bottom" || root.safeActiveEdge === "top"

    property color cBase: ThemeBackend.base
    property color cSurface0: ThemeBackend.surface0
    property color cSurface1: ThemeBackend.surface1
    property color cText: ThemeBackend.text

    function alpha(color, a) { return Qt.rgba(color.r, color.g, color.b, a); }


    Item {
        id: orientedRoot
        anchors.centerIn: parent
        width: root.isHorizontal ? root.dockSpan : root.dockThickness
        height: root.isHorizontal ? root.dockThickness : root.dockSpan
        rotation: root.counterRotation
        clip: false

        Rectangle {
            anchors.fill: parent
            radius: ThemeBackend.borderRadius
            color: dockGlass.active ? "transparent" : root.alpha(root.cSurface0, 0.85)

            GlassFill {
                id: dockGlass
                anchors.fill: parent
                radius: parent.radius
                tintColor: root.cSurface0
                tintAmount: 0.3
                shadow: 1
            }

            ListView {
                id: dockList
                anchors.fill: parent
                anchors.margins: root.dockPadding
                orientation: root.isHorizontal ? ListView.Horizontal : ListView.Vertical
                spacing: root.itemSpacing
                model: root.dockApps
                interactive: false
                boundsBehavior: Flickable.StopAtBounds

                delegate: Item {
                    required property string modelData
                    width: root.iconSize
                    height: root.iconSize

                    Rectangle {
                        anchors.fill: parent
                        radius: ThemeBackend.borderRadius
                        color: (itemGlass.active || !itemMouseArea.containsMouse) ? "transparent" : root.alpha(root.cSurface1, 0.4)
                        Behavior on color { ColorAnimation { duration: 250 } }

                        GlassFill {
                            id: itemGlass
                            anchors.fill: parent
                            radius: parent.radius
                            tintColor: root.cSurface1
                            tintAmount: 0.2
                            allowed: itemMouseArea.containsMouse
                            press: itemMouseArea.pressed ? 1 : 0.3
                        }
                    }

                    Item {
                        anchors.centerIn: parent
                        width: parent.width * 0.8
                        height: parent.height * 0.8

                        property real hoverScale: itemMouseArea.pressed ? 0.92 : (itemMouseArea.containsMouse ? 1.08 : 1.0)
                        scale: hoverScale
                        Behavior on scale { NumberAnimation { duration: 400; easing.type: Easing.OutCubic } }

                        Image {
                            anchors.fill: parent
                            source: Ryoku.Dock.iconFor(modelData)
                            sourceSize: Qt.size(64, 64)
                            fillMode: Image.PreserveAspectFit
                            smooth: true
                            mipmap: true
                        }
                    }

                    MouseArea {
                        id: itemMouseArea
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: {
                            Ryoku.Dock.activate(modelData);
                            Quickshell.execDetached(["bash", Caching.scriptsPath + "/qs_manager.sh", "close"]);
                        }
                    }
                }
            }
        }
    }
}
