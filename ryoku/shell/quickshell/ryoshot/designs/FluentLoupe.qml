import QtQuick
import "../look"
import ".."

// Fluent's loupe: a small rounded card with a thin stroke and a flyout shadow,
// the pixels inset inside it, and the value on a tooltip card below.
Item {
    id: loupe

    required property var chrome
    property real px: 0
    property real py: 0
    property color mark: Look.sel
    property string value: ""
    property bool shown: false

    readonly property real u: loupe.chrome.u
    readonly property real side: loupe.chrome.snap(112 * loupe.u)
    readonly property real inset: loupe.chrome.snap(3 * loupe.u)
    readonly property real gap: loupe.chrome.snap(6 * loupe.u)

    implicitWidth: Math.max(loupe.side, tip.implicitWidth)
    implicitHeight: loupe.side + loupe.gap + tip.implicitHeight

    visible: opacity > 0
    opacity: loupe.shown ? 1 : 0
    Behavior on opacity { enabled: loupe.shown; NumberAnimation { duration: Look.dur(167); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.fluentIn } }
    transform: Translate {
        y: loupe.shown ? 0 : 6 * loupe.u
        Behavior on y { enabled: loupe.shown; NumberAnimation { duration: Look.dur(250); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.fluentIn } }
    }

    Surface {
        id: card
        anchors.horizontalCenter: parent.horizontalCenter
        width: loupe.side
        height: loupe.side
        u: loupe.u
        radius: 8 * loupe.u
        elevation: 2
        acrylic: false
        fill: Look.planes.card
        stroke: Look.planeStroke
    }
    Loupe {
        x: card.x + loupe.inset
        y: loupe.inset
        width: loupe.side - loupe.inset * 2
        height: loupe.side - loupe.inset * 2
        source: loupe.chrome.plate
        cx: loupe.px
        cy: loupe.py
        dpr: loupe.chrome.dpr
        u: loupe.u
        markColor: loupe.mark
    }

    ValueChip {
        id: tip
        anchors.horizontalCenter: parent.horizontalCenter
        y: loupe.side + loupe.gap
        u: loupe.u
        text: loupe.value
        fill: Look.planes.card
        ink: Look.fg
        stroke: Look.planeStroke
        chipHeight: 26
        padX: 8
        radius: 4
        fontSize: 12
        fontWeight: Font.Normal
    }
}
