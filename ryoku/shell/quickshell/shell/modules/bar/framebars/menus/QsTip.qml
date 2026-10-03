import QtQuick
import shell.services

// Hover bubble naming an icon-only control. Shows above (default), below
// (below: true), or to the RIGHT (side: true) after a short dwell. `align`
// pins the bubble to the button's "left"/"right" edge so an edge button opens
// its bubble inward instead of overflowing the panel. Never takes input.
Item {
    id: root

    property string text: ""
    property bool hovered: false
    property bool below: false
    // side: true opens the bubble to the RIGHT, vertically centred; wins over below.
    property bool side: false
    // "center" (default), "left" or "right": the button edge the bubble pins to.
    property string align: "center"

    readonly property bool showing: root.hovered && root.text.length > 0 && dwell.done
    anchors.fill: parent

    Timer {
        id: dwell
        property bool done: false
        interval: 320
        running: root.hovered
        onTriggered: dwell.done = true
    }
    onHoveredChanged: if (!root.hovered) dwell.done = false

    Rectangle {
        id: bubble

        // Above every sibling subtree in the hosting panel.
        z: 1000

        readonly property real gap: root.showing ? 8 : 4

        x: root.side ? parent.width + gap
            : root.align === "right" ? parent.width - width
            : root.align === "left" ? 0
            : (parent.width - width) / 2

        y: root.side
            ? (parent.height - height) / 2
            : root.below
                ? parent.height + gap
                : -height - gap

        width: cap.implicitWidth + 16
        height: cap.implicitHeight + 10
        radius: height / 2
        color: Theme.surfaceContainerHigh ? Theme.surfaceContainerHigh : Theme.surface
        border.width: 1
        border.color: Qt.rgba(Theme.outline.r, Theme.outline.g, Theme.outline.b, 0.4)
        opacity: root.showing ? 1 : 0
        scale: root.showing ? 1 : 0.92
        visible: opacity > 0.004

        Behavior on x { NumberAnimation { duration: Motion.fast; easing.type: Motion.easeStandard } }
        Behavior on y { NumberAnimation { duration: Motion.fast; easing.type: Motion.easeStandard } }
        Behavior on opacity { NumberAnimation { duration: Motion.fast; easing.type: Motion.easeStandard } }
        Behavior on scale { NumberAnimation { duration: Motion.fast; easing.type: Easing.OutBack; easing.overshoot: 1.2 } }

        Text {
            id: cap
            anchors.centerIn: parent
            text: root.text
            color: Theme.inkOn(bubble.color, Theme.onSurface)
            font.family: Theme.fontPrimary
            font.pixelSize: Theme.fontSm - 2
            font.weight: Font.DemiBold
        }
    }
}
