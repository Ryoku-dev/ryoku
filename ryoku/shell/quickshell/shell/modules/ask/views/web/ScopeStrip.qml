pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import "../../look"
import "../../lib/web.js" as Web

// Where to search: All, then each scope the daemon offers. Under every name sits
// one tick per source in it, and one per engine of the web lane. A tick fills
// when its source has answered, turns jasper when it failed, amber while its
// budget holds it back, and splits in two (a pause mark) while it is blocked.
// So the strip is the search's real progress.
Item {
    id: root

    required property real s
    // [{id, label}]
    property var scopes: []
    property string current: "all"
    property var sources: []
    property var engines: []
    // per source id, as SearchClient holds them
    property var answers: ({})
    // a query is still out
    property bool flying: false

    readonly property int currentIndex: {
        for (var i = 0; i < root.scopes.length; i++) {
            if (root.scopes[i].id === root.current)
                return i;
        }
        return -1;
    }

    signal picked(string scope)

    implicitHeight: Math.round(40 * root.s)

    Shape {
        id: plate

        readonly property Item target: root.currentIndex >= 0 && segments.count > root.currentIndex ? segments.itemAt(root.currentIndex) : null
        property real restX: 0
        property real restW: 0
        // the first placement is not a move
        property bool placed: false

        x: row.x + plate.restX
        y: row.y
        width: plate.restW
        height: row.height
        visible: plate.target !== null
        s: root.s
        cutSize: Look.cutRow
        fill: Look.plate

        function settle() {
            if (!plate.target)
                return;
            plate.restX = plate.target.x;
            plate.restW = plate.target.width;
            if (!plate.placed && plate.restW > 0)
                Qt.callLater(() => plate.placed = true);
        }
        onTargetChanged: plate.settle()
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
    }

    Row {
        id: row
        x: Math.round(8 * root.s)
        y: Math.round(4 * root.s)
        height: Math.round(32 * root.s)
        spacing: Math.round(2 * root.s)

        Repeater {
            id: segments
            model: root.scopes

            delegate: Item {
                id: segment

                required property var modelData
                required property int index
                readonly property bool chosen: segment.index === root.currentIndex
                readonly property var progress: Web.progress(root.sources, root.answers, segment.modelData.id, root.engines)
                readonly property color ink: segment.chosen ? Look.plateInk : Lithic.fg

                width: Math.ceil(Math.max(label.implicitWidth, ticks.width) + 22 * root.s)
                height: row.height

                Text {
                    id: label
                    anchors.horizontalCenter: parent.horizontalCenter
                    y: Math.round(5 * root.s)
                    text: String(segment.modelData.label || "")
                    color: segment.chosen ? Look.plateInk : hover.hovered ? Lithic.fg : Lithic.fg2
                    font.family: Look.sans
                    font.pixelSize: Math.round(12 * root.s)
                    font.weight: segment.chosen ? Font.Medium : Font.Normal
                    renderType: Text.NativeRendering
                }

                Row {
                    id: ticks
                    anchors.horizontalCenter: parent.horizontalCenter
                    y: Math.round(23 * root.s)
                    spacing: Math.max(1, Math.round(2 * root.s))

                    Repeater {
                        model: segment.progress.ticks

                        delegate: Item {
                            id: tick

                            required property var modelData
                            readonly property string state: String(tick.modelData.state || "idle")
                            readonly property bool halted: tick.state === "paused"
                            readonly property real bar: Math.max(1, Math.round(tick.width / 3))
                            property color tone: tick.state === "error" ? Lithic.jasper
                                : tick.halted || tick.state === "waiting" ? Lithic.amber
                                : tick.state === "done" ? (root.flying ? Lithic.lapis : Lithic.alpha(segment.ink, 0.55))
                                : Lithic.alpha(segment.ink, 0.16)

                            width: Math.max(3, Math.round(5 * root.s))
                            height: Math.max(2, Math.round(2 * root.s))

                            Behavior on tone {
                                ColorAnimation { duration: Look.dur(Look.stateMs) }
                            }

                            Rectangle {
                                width: tick.halted ? tick.bar : parent.width
                                height: parent.height
                                radius: Look.lithic || tick.halted ? 0 : height / 2
                                color: tick.tone
                            }
                            Rectangle {
                                visible: tick.halted
                                x: parent.width - width
                                width: tick.bar
                                height: parent.height
                                color: tick.tone
                            }
                        }
                    }
                }

                HoverHandler { id: hover; cursorShape: Qt.PointingHandCursor }
                TapHandler { onTapped: root.picked(String(segment.modelData.id)) }
            }
        }
    }
}
