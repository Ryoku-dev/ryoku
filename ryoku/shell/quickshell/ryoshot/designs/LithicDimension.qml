import QtQuick
import "../look"

// A drafting dimension line: a hairline between two end ticks with the length
// on a small cut plaque at its middle. Horizontal it spans x0..x1 at y;
// vertical it spans y0..y1 at x, its figure turned to read up the line.
Item {
    id: dim

    required property var chrome
    property bool vertical: false
    property real from: 0
    property real to: 0
    property real at: 0
    property color ink: Look.sel
    // the figure on the plaque, when it is not the line's own length
    property string label: ""

    readonly property real u: dim.chrome.u
    readonly property real t: dim.chrome.hair
    readonly property real a: dim.chrome.snap(dim.from)
    readonly property real b: dim.chrome.snap(dim.to)
    readonly property real line: dim.chrome.snap(dim.at)
    readonly property real tick: dim.chrome.snap(5 * dim.u)
    readonly property real mid: (dim.a + dim.b) / 2
    readonly property real plateLong: Math.ceil(label.implicitWidth + 10 * dim.u)
    readonly property real plateShort: dim.chrome.snap(14 * dim.u)
    readonly property real gap: dim.chrome.snap(4 * dim.u)

    Rectangle {
        x: dim.vertical ? dim.line : dim.a
        y: dim.vertical ? dim.a : dim.line
        width: dim.vertical ? dim.t : dim.chrome.snap(dim.mid - dim.plateLong / 2 - dim.gap) - dim.a
        height: dim.vertical ? dim.chrome.snap(dim.mid - dim.plateLong / 2 - dim.gap) - dim.a : dim.t
        color: dim.ink
        antialiasing: false
    }
    Rectangle {
        readonly property real start: dim.chrome.snap(dim.mid + dim.plateLong / 2 + dim.gap)
        x: dim.vertical ? dim.line : start
        y: dim.vertical ? start : dim.line
        width: dim.vertical ? dim.t : dim.b - start + dim.t
        height: dim.vertical ? dim.b - start + dim.t : dim.t
        color: dim.ink
        antialiasing: false
    }
    Repeater {
        model: [dim.a, dim.b]
        Rectangle {
            required property real modelData
            x: dim.vertical ? dim.line - dim.chrome.snap(dim.tick / 2) : modelData
            y: dim.vertical ? modelData : dim.line - dim.chrome.snap(dim.tick / 2)
            width: dim.vertical ? dim.tick : dim.t
            height: dim.vertical ? dim.t : dim.tick
            color: dim.ink
            antialiasing: false
        }
    }

    Item {
        x: dim.vertical ? dim.line - dim.chrome.snap(dim.plateShort / 2) : dim.chrome.snap(dim.mid - dim.plateLong / 2)
        y: dim.vertical ? dim.chrome.snap(dim.mid - dim.plateLong / 2) : dim.line - dim.chrome.snap(dim.plateShort / 2)
        width: dim.vertical ? dim.plateShort : dim.plateLong
        height: dim.vertical ? dim.plateLong : dim.plateShort

        Surface {
            anchors.fill: parent
            u: dim.u
            cut: Look.cutSm
            cutTopRight: 0
            cutBottomLeft: 0
            shadow: false
            fill: Look.planes.pane
            stroke: Qt.rgba(dim.ink.r, dim.ink.g, dim.ink.b, 0.6)
        }
        Text {
            id: label
            anchors.centerIn: parent
            rotation: dim.vertical ? -90 : 0
            text: dim.label.length > 0 ? dim.label : Math.round(Math.abs(dim.to - dim.from))
            color: Look.fg
            font.family: Look.mono
            font.pixelSize: Math.round(9 * dim.u)
            font.letterSpacing: 0.4 * dim.u
            font.features: ({ "tnum": 1 })
            renderType: Text.NativeRendering
        }
    }
}
