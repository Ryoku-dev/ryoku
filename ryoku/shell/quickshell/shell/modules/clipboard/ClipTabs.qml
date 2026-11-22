pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import "../ask/look"

Item {
    id: root

    required property real s
    property var tabs: []
    property string current: ""

    readonly property int currentIndex: {
        for (var i = 0; i < root.tabs.length; i++) {
            if (root.tabs[i].id === root.current)
                return i;
        }
        return -1;
    }
    onCurrentIndexChanged: plate.retarget()

    signal picked(string id)

    implicitWidth: row.width
    implicitHeight: Math.round(28 * root.s)

    Shape {
        id: plate

        // set when an item exists, not from count: an async load counts the tabs before making them
        property Item target: null
        property real restX: 0
        property real restW: 0
        // the first placement is not a move
        property bool placed: false

        x: plate.restX
        width: plate.restW
        height: root.height
        visible: plate.target !== null && plate.restW > 0
        s: root.s
        cutSize: Look.cutRow
        fill: Look.material ? Lithic.container : Look.fluent ? "transparent" : Look.plate
        strokeColor: Look.lithic ? Lithic.vein : "transparent"
        strokeWidth: Look.lithic ? 1 : 0

        function retarget() {
            plate.target = root.currentIndex >= 0 ? segments.itemAt(root.currentIndex) : null;
            plate.settle();
        }

        function settle() {
            if (!plate.target)
                return;
            plate.restX = plate.target.x;
            plate.restW = plate.target.width;
            if (!plate.placed && plate.restW > 0)
                Qt.callLater(() => plate.placed = true);
        }
        Connections {
            target: plate.target
            function onXChanged() { plate.settle(); }
            function onWidthChanged() { plate.settle(); }
        }
        Behavior on restX {
            enabled: plate.placed
            NumberAnimation { duration: Look.dur(Look.plateMs); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.arrive }
        }
        Behavior on restW {
            enabled: plate.placed
            NumberAnimation { duration: Look.dur(Look.plateMs); easing.type: Easing.OutCubic }
        }

        Rectangle {
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: parent.bottom
            width: Math.round(16 * root.s)
            height: Math.max(2, Math.round(3 * root.s))
            radius: height / 2
            visible: Look.fluent
            color: Lithic.sel
        }
    }

    Row {
        id: row
        height: root.height
        spacing: Math.round(2 * root.s)

        Repeater {
            id: segments
            model: root.tabs
            onItemAdded: plate.retarget()

            delegate: Item {
                id: tab

                required property var modelData
                required property int index
                readonly property bool chosen: tab.index === root.currentIndex
                readonly property int count: Number(tab.modelData.count) || 0
                readonly property color ink: tab.chosen ? (Look.material ? Lithic.containerInk : Look.plateInk)
                    : hover.hovered ? Lithic.fg : Lithic.fg2

                width: Math.ceil(content.implicitWidth + 20 * root.s)
                height: row.height

                Accessible.role: Accessible.PageTab
                Accessible.name: label.text
                Accessible.selected: tab.chosen

                Row {
                    id: content
                    anchors.centerIn: parent
                    spacing: Math.round(6 * root.s)

                    Text {
                        id: label
                        anchors.verticalCenter: parent.verticalCenter
                        text: String(tab.modelData.label || "")
                        color: tab.ink
                        font.family: Look.monoMicro ? Look.mono : Look.sans
                        font.pixelSize: Math.round((Look.monoMicro ? 10 : 12) * root.s)
                        font.weight: Look.monoMicro ? Font.Normal : tab.chosen ? Font.DemiBold : Font.Medium
                        font.capitalization: Look.monoMicro ? Font.AllUppercase : Font.MixedCase
                        font.letterSpacing: Look.tracking * root.s
                        renderType: Text.NativeRendering
                        Behavior on color {
                            ColorAnimation { duration: Look.dur(Look.stateMs) }
                        }
                    }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: tab.count > 0
                        text: String(tab.count)
                        color: tab.chosen && Look.material ? Lithic.alpha(Lithic.containerInk, 0.72) : Lithic.fg3
                        font.family: Look.figures
                        font.pixelSize: Math.round(11 * Look.figureScale * root.s)
                        font.features: ({ "tnum": 1 })
                        renderType: Text.NativeRendering
                    }
                }

                HoverHandler {
                    id: hover
                    cursorShape: Qt.PointingHandCursor
                }
                TapHandler {
                    onTapped: root.picked(String(tab.modelData.id))
                }
            }
        }
    }
}
