pragma ComponentBehavior: Bound

import QtQuick
import shell.services

// The Kairos date wheel: a strip of days that scrolls like a phone's date
// picker. The centred day is the selected one; the days either side turn away in
// perspective. Every visual is interpolated from a day's distance to the centre,
// never switched, so the scroll has no steps in it, and glyph sizes are fixed so
// scrolling never re-rasterises text.

Item {
    id: wheel

    property date anchor: new Date()
    property int span: 14

    // Scroll position in item steps from the anchor; negative walks forward.
    property real offset: 0

    // 0..1 entrance: an extra step and a half of distance that settles out as the
    // island opens, so the strip rolls into place.
    property real reveal: 1

    property real itemWidth: 52
    property real itemGap: 8
    readonly property real step: wheel.itemWidth + wheel.itemGap

    property color ink: "#ffffff"
    property color accent: "#7fd4d4"
    property var locale: Qt.locale()
    property var labels: []

    function rebuildLabels() {
        var next = [];
        for (var i = 0; i <= 2 * wheel.span; i++) {
            var day = wheel.dayAt(i);
            var weekday = day.toLocaleDateString(wheel.locale, "ddd").toUpperCase();
            next.push({
                weekday: weekday,
                initial: weekday.charAt(0),
                date: String(day.getDate())
            });
        }
        wheel.labels = next;
    }
    onAnchorChanged: wheel.rebuildLabels()
    onSpanChanged: wheel.rebuildLabels()
    onLocaleChanged: wheel.rebuildLabels()
    Component.onCompleted: wheel.rebuildLabels()

    // The picked day, reported because this surface's state is out of the
    // launcher's reach.
    signal selectionChanged(date day, int index)

    readonly property int selectedIndex: Math.max(0, Math.min(2 * span, Math.round(span - offset)))
    readonly property date selected: wheel.dayAt(wheel.selectedIndex)
    onSelectedChanged: wheel.selectionChanged(wheel.selected, wheel.selectedIndex)

    function dayAt(index) {
        var d = new Date(wheel.anchor.getTime());
        d.setDate(d.getDate() + (index - wheel.span));
        return d;
    }
    function stepBy(n) {
        wheel.offset = Math.max(-wheel.span, Math.min(wheel.span, Math.round(wheel.offset) - n));
    }
    function centreOn(index) {
        wheel.offset = Math.max(-wheel.span, Math.min(wheel.span, wheel.span - index));
    }

    property bool dragging: false

    Behavior on offset {
        enabled: !wheel.dragging && !Motion.reduce
        NumberAnimation {
            duration: Motion.spatial
            easing.type: Easing.Bezier
            easing.bezierCurve: Motion.spatialCurve
        }
    }

    clip: true

    Repeater {
        model: wheel.labels

        delegate: Item {
            id: slot

            required property int index
            required property var modelData

            readonly property real dist: (index - wheel.span) + wheel.offset
                + (1 - wheel.reveal) * 1.6
            // Centre weight (labels and accent) and distance falloff (opacity),
            // both continuous, so a day brightens as it rolls in rather than
            // switching at a threshold.
            readonly property real near: Math.max(0, 1 - Math.abs(dist))
            readonly property real far: Math.max(0, 1 - Math.abs(dist) / 2.6)

            width: wheel.itemWidth
            height: wheel.height
            x: wheel.width / 2 - width / 2 + dist * wheel.step
            visible: Math.abs(dist) < 3.6
            opacity: far * Math.min(1, wheel.reveal * 2)
            scale: 0.72 + 0.28 * near
            // Rotate the day in 3D toward the centre. No layer here: flattening
            // each slot to an FBO per frame was the wheel's main cost, and the only
            // thing it bought -- grouped opacity for the faint highlight behind the
            // text -- is imperceptible.
            transform: Rotation {
                origin.x: slot.width / 2
                origin.y: slot.height / 2
                axis { x: 0; y: 1; z: 0 }
                angle: -slot.dist * 22
            }

            Rectangle {
                anchors.centerIn: parent
                width: 44
                height: 60
                radius: 12
                color: Qt.rgba(wheel.ink.r, wheel.ink.g, wheel.ink.b, 0.07 * slot.near)
            }

            Column {
                anchors.centerIn: parent
                spacing: 1

                // The weekday unfolds from one letter to three as its day
                // arrives: two labels crossfading, so nothing resizes.
                Item {
                    width: parent.width
                    height: 14
                    Text {
                        anchors.centerIn: parent
                        text: slot.modelData.initial
                        color: wheel.ink
                        opacity: 1 - slot.near
                        font.family: Theme.fontPrimary
                        font.pixelSize: 11
                    }
                    Text {
                        anchors.centerIn: parent
                        text: slot.modelData.weekday
                        color: wheel.ink
                        opacity: slot.near
                        font.family: Theme.fontPrimary
                        font.pixelSize: 11
                        font.weight: Font.DemiBold
                        font.letterSpacing: 1
                    }
                }
                Text {
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: slot.modelData.date
                    font.family: Theme.mono
                    font.pixelSize: 22
                    color: Qt.rgba(
                        wheel.ink.r + (wheel.accent.r - wheel.ink.r) * slot.near,
                        wheel.ink.g + (wheel.accent.g - wheel.ink.g) * slot.near,
                        wheel.ink.b + (wheel.accent.b - wheel.ink.b) * slot.near, 1)
                }
            }

            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: wheel.centreOn(slot.index)
            }
        }
    }

    DragHandler {
        id: drag
        target: null
        xAxis.enabled: true
        yAxis.enabled: false
        property real startOffset: 0

        onActiveChanged: {
            if (active) {
                startOffset = wheel.offset;
                wheel.dragging = true;
            } else {
                wheel.dragging = false;
                wheel.offset = Math.max(-wheel.span,
                    Math.min(wheel.span, Math.round(wheel.offset)));
            }
        }
        onTranslationChanged: wheel.offset = startOffset + translation.x / wheel.step
    }

    WheelHandler {
        onWheel: event => wheel.stepBy(event.angleDelta.y > 0 ? -1 : 1)
    }
}
