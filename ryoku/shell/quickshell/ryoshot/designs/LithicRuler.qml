import QtQuick
import "../look"

// A local ruler on one output edge: a ±240 strip of the pane centred on the
// pointer, so it reads over any frame (the bar included), with a tick every 10,
// a longer one every 50 and a figure every 100, fading toward its ends. The
// pointer's coordinate sits on a sel plaque on it. Along the top edge it
// measures x; down the left edge, y.
Item {
    id: ruler

    required property var chrome
    property bool vertical: false

    readonly property real u: ruler.chrome.u
    readonly property real at: ruler.vertical ? ruler.chrome.py : ruler.chrome.px
    readonly property real reach: 240
    readonly property real fadeLen: 90
    readonly property int first: Math.floor((ruler.at - ruler.reach) / 10) * 10
    readonly property real t: ruler.chrome.hair
    readonly property real depth: ruler.chrome.snap((ruler.vertical ? 42 : 27) * ruler.u)
    readonly property color plane: Look.planes.pane

    function fade(v) { return Math.max(0, Math.min(1, (ruler.reach - Math.abs(v - ruler.at)) / ruler.fadeLen)); }

    // the strip and its inner hairline, both fading out toward the ends
    Repeater {
        model: [
            { inset: 0, thick: -1, ink: Qt.rgba(ruler.plane.r, ruler.plane.g, ruler.plane.b, 0.92) },
            { inset: 1, thick: ruler.t, ink: Look.vein }
        ]
        Rectangle {
            required property var modelData
            readonly property real from: ruler.chrome.snap(ruler.at - ruler.reach)
            readonly property real span: ruler.chrome.snap(ruler.reach * 2)
            readonly property real across: modelData.thick < 0 ? ruler.depth : modelData.thick
            readonly property real offset: modelData.inset ? ruler.depth : 0
            x: ruler.vertical ? offset : from
            y: ruler.vertical ? from : offset
            width: ruler.vertical ? across : span
            height: ruler.vertical ? span : across
            antialiasing: false
            gradient: Gradient {
                orientation: ruler.vertical ? Gradient.Vertical : Gradient.Horizontal
                GradientStop { position: 0; color: "transparent" }
                GradientStop { position: ruler.fadeLen / (ruler.reach * 2); color: modelData.ink }
                GradientStop { position: 1 - ruler.fadeLen / (ruler.reach * 2); color: modelData.ink }
                GradientStop { position: 1; color: "transparent" }
            }
        }
    }

    Repeater {
        model: 50
        Rectangle {
            id: tick
            required property int index
            readonly property int value: ruler.first + tick.index * 10
            readonly property real len: (tick.value % 100 === 0 ? 11 : tick.value % 50 === 0 ? 7 : 4) * ruler.u
            readonly property real pos: ruler.chrome.snap(tick.value)
            visible: tick.value >= 0
            opacity: ruler.fade(tick.value) * (tick.value % 50 === 0 ? 1 : 0.6)
            x: ruler.vertical ? 0 : tick.pos
            y: ruler.vertical ? tick.pos : 0
            width: ruler.vertical ? ruler.chrome.snap(tick.len) : ruler.t
            height: ruler.vertical ? ruler.t : ruler.chrome.snap(tick.len)
            color: Look.fg2
            antialiasing: false
        }
    }

    Repeater {
        model: 6
        Text {
            id: figure
            required property int index
            readonly property int value: Math.ceil((ruler.at - ruler.reach) / 100) * 100 + figure.index * 100
            readonly property real near: Math.abs(figure.value - ruler.at)
            visible: figure.value >= 0 && figure.near <= ruler.reach
            // the plaque carries the exact figure, so a round one under it steps aside
            opacity: ruler.fade(figure.value) * Math.max(0, Math.min(1, (figure.near - 26 * ruler.u) / (16 * ruler.u)))
            x: ruler.chrome.snap(ruler.vertical ? 14 * ruler.u : figure.value + 3 * ruler.u)
            y: ruler.chrome.snap(ruler.vertical ? figure.value - height / 2 : 2 * ruler.u)
            text: figure.value
            color: Look.fg2
            font.family: Look.mono
            font.pixelSize: Math.round(8.5 * ruler.u)
            font.letterSpacing: 0.4 * ruler.u
            font.features: ({ "tnum": 1 })
            renderType: Text.NativeRendering
        }
    }

    // the pointer's own tick, in sel, from the edge to its plaque
    Rectangle {
        x: ruler.vertical ? 0 : ruler.chrome.snap(ruler.at)
        y: ruler.vertical ? ruler.chrome.snap(ruler.at) : 0
        width: ruler.vertical ? plaque.x : ruler.t
        height: ruler.vertical ? ruler.t : plaque.y
        color: Look.sel
        antialiasing: false
    }

    Item {
        id: plaque
        readonly property real w: Math.ceil(coord.implicitWidth + 10 * ruler.u)
        readonly property real h: ruler.chrome.snap(14 * ruler.u)
        x: ruler.chrome.snap(ruler.vertical ? 13 * ruler.u : Math.max(2 * ruler.u, ruler.at - plaque.w / 2))
        y: ruler.chrome.snap(ruler.vertical ? Math.max(2 * ruler.u, ruler.at - plaque.h / 2) : 11 * ruler.u)
        width: plaque.w
        height: plaque.h

        Surface {
            anchors.fill: parent
            u: ruler.u
            cut: Look.cutSm
            cutTopRight: 0
            cutBottomLeft: 0
            shadow: false
            fill: Look.sel
            stroke: "transparent"
        }
        Text {
            id: coord
            anchors.centerIn: parent
            text: Math.round(ruler.at)
            color: Look.onSel
            font.family: Look.mono
            font.pixelSize: Math.round(9 * ruler.u)
            font.weight: Font.Medium
            font.features: ({ "tnum": 1 })
            renderType: Text.NativeRendering
        }
    }
}
