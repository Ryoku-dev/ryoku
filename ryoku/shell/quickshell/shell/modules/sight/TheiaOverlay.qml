pragma ComponentBehavior: Bound

import QtQuick
import Lithic

Item {
    id: root
    property var lines: []
    property rect plateRect: Qt.rect(0, 0, 0, 0)
    property real s: 1
    readonly property bool interactive: false
    readonly property var planes: Lithic.planes(Lithic.design)
    readonly property color plane: Lithic.design === "lithic" ? planes.pane : Lithic.design === "fluent" ? Lithic.mix(planes.card, Lithic.sel, 0.025) : Lithic.stone4
    x: plateRect.x
    y: plateRect.y
    width: plateRect.width
    height: plateRect.height
    clip: true

    function unit(value) {
        const n = Number(value);
        return isFinite(n) ? Math.max(0, Math.min(1, n)) : 0;
    }

    Repeater {
        model: root.lines
        delegate: Item {
            id: box
            required property var modelData
            readonly property real nx: root.unit(modelData.x)
            readonly property real ny: root.unit(modelData.y)
            property real reveal: 0
            x: nx * root.width
            y: ny * root.height
            width: Math.min(root.unit(modelData.width), 1 - nx) * root.width
            height: Math.min(root.unit(modelData.height), 1 - ny) * root.height
            visible: width > 0 && height > 0 && String(modelData.text || "").length > 0
            opacity: reveal
            Component.onCompleted: reveal = 1
            Behavior on reveal { NumberAnimation { duration: Lithic.duration(160); easing.type: Easing.OutCubic } }
            CutRect {
                anchors.fill: parent
                cut: Math.min(3 * root.s, box.height / 4)
                fill: Lithic.alpha(root.plane, 0.92)
            }
            Text {
                anchors.fill: parent
                anchors.margins: Math.min(2 * root.s, box.height * 0.06)
                text: String(box.modelData.text || "")
                textFormat: Text.PlainText
                color: Lithic.fg
                font.family: Lithic.type.body.family
                font.pixelSize: Math.max(1, box.height * 0.85)
                fontSizeMode: Text.Fit
                minimumPixelSize: 1
                wrapMode: Text.Wrap
                horizontalAlignment: Text.AlignHCenter
                verticalAlignment: Text.AlignVCenter
                renderType: Text.QtRendering
            }
        }
    }
}
