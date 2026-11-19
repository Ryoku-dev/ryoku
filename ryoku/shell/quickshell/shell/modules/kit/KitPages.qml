pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import "../ask/look"

// A section's own pages, drawn by the frame: plain labels with a blade under the
// current one that glides between them. A strip longer than the page scrolls
// sideways and fades at the side that holds more. Sizes: kit-layouts SPEC 3.
Item {
    id: root

    required property real s
    property bool active: false
    // [{id, label, count?}]
    property var pages: []
    property string current: ""

    readonly property real lead: Math.round(22 * root.s)
    readonly property bool more: flick.contentWidth > flick.width + 1

    signal picked(string id)

    implicitHeight: Math.round(34 * root.s)

    function reveal() {
        for (var i = 0; i < repeater.count; i++) {
            const item = repeater.itemAt(i);
            if (!item || String(item.modelData.id) !== root.current)
                continue;
            const pad = Math.round(40 * root.s);
            const most = Math.max(0, flick.contentWidth - flick.width);
            if (item.x - pad < flick.contentX)
                flick.contentX = Math.max(0, item.x - pad);
            else if (item.x + item.width + pad > flick.contentX + flick.width)
                flick.contentX = Math.min(most, item.x + item.width + pad - flick.width);
            blade.target = item;
            return;
        }
        blade.target = null;
    }
    onCurrentChanged: Qt.callLater(root.reveal)
    onPagesChanged: Qt.callLater(root.reveal)
    onWidthChanged: Qt.callLater(root.reveal)

    Flickable {
        id: flick
        x: root.lead
        width: parent.width - root.lead - hint.width - Math.round(28 * root.s)
        height: parent.height
        contentWidth: row.width
        contentHeight: height
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        flickableDirection: Flickable.HorizontalFlick
        interactive: root.more

        Behavior on contentX {
            enabled: root.active
            NumberAnimation { duration: Look.dur(Look.plateMs); easing.type: Easing.OutCubic }
        }

        Row {
            id: row
            height: Math.round(30 * root.s)
            spacing: Math.round(18 * root.s)

            Repeater {
                id: repeater
                model: root.pages

                delegate: Item {
                    id: page

                    required property var modelData
                    readonly property bool chosen: String(page.modelData.id) === root.current
                    readonly property bool counted: page.modelData.count !== undefined && page.modelData.count !== null && Number(page.modelData.count) >= 0

                    width: label.implicitWidth
                    height: row.height

                    Row {
                        id: label
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: Math.round(5 * root.s)

                        Text {
                            id: name
                            text: String(page.modelData.label || "")
                            color: page.chosen || tap.containsMouse ? Lithic.fg : Lithic.fg2
                            font.family: Look.sans
                            font.pixelSize: Math.round(13 * root.s)
                            font.weight: page.chosen ? Font.Medium : Font.Normal
                            renderType: Text.NativeRendering
                        }
                        Text {
                            anchors.baseline: name.baseline
                            visible: page.counted
                            text: page.counted ? Number(page.modelData.count).toLocaleString(Qt.locale(), "f", 0) : ""
                            color: Lithic.fg3
                            font.family: Look.figures
                            font.pixelSize: Math.round(11.5 * Look.figureScale * root.s)
                            font.features: ({ "tnum": 1 })
                            renderType: Text.NativeRendering
                        }
                    }
                    MouseArea {
                        id: tap
                        anchors.fill: parent
                        anchors.margins: -Math.round(6 * root.s)
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: root.picked(String(page.modelData.id))
                    }
                }
            }
        }

        Rectangle {
            id: blade

            property Item target: null
            // landing on a strip that just appeared is not a glide
            property bool placed: false

            visible: blade.target !== null
            x: blade.target ? blade.target.x : 0
            y: row.height - height
            width: blade.target ? blade.target.width : 0
            height: Math.max(2, Math.round(2 * root.s))
            radius: Look.lithic ? 0 : height / 2
            color: Look.material ? Lithic.sel : Lithic.fg

            onTargetChanged: if (blade.target && !blade.placed) Qt.callLater(() => blade.placed = true)
            Behavior on x {
                enabled: root.active && blade.placed
                NumberAnimation { duration: Look.dur(Look.viewMs); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.arrive }
            }
            Behavior on width {
                enabled: root.active && blade.placed
                NumberAnimation { duration: Look.dur(Look.viewMs); easing.type: Easing.OutCubic }
            }
        }
    }

    // more to the left, more to the right
    Rectangle {
        x: flick.x
        width: Math.round(28 * root.s)
        height: row.height
        visible: root.more && flick.contentX > 1
        gradient: Gradient {
            orientation: Gradient.Horizontal
            GradientStop { position: 0; color: Look.frame }
            GradientStop { position: 1; color: Lithic.alpha(Look.frame, 0) }
        }
    }
    Rectangle {
        x: flick.x + flick.width - width
        width: Math.round(28 * root.s)
        height: row.height
        visible: root.more && flick.contentX < flick.contentWidth - flick.width - 1
        gradient: Gradient {
            orientation: Gradient.Horizontal
            GradientStop { position: 0; color: Lithic.alpha(Look.frame, 0) }
            GradientStop { position: 1; color: Look.frame }
        }
    }

    KeyHint {
        id: hint
        anchors.right: parent.right
        anchors.rightMargin: Math.round(16 * root.s)
        y: Math.round((row.height - height) / 2)
        s: root.s
        hint: "Ctrl+PageDown"
    }
}
