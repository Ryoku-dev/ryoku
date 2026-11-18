import QtQuick
import "../look"
import ".."

// Lithic's loupe: a square plate, cut at two corners, with a hairline; the
// magnified pixels inset inside it, and a mono strip under them with the
// colour's hex (a chip of it beside) or the point's coordinate.
Item {
    id: loupe

    required property var chrome
    property real px: 0
    property real py: 0
    property string value: ""
    property color swatch: "transparent"
    property bool showSwatch: false
    property color mark: Look.sel
    property bool shown: false

    readonly property real u: loupe.chrome.u
    readonly property real inset: loupe.chrome.snap(5 * loupe.u)
    readonly property real lensSize: loupe.chrome.snap(112 * loupe.u)
    readonly property real strip: loupe.chrome.snap(22 * loupe.u)

    implicitWidth: loupe.lensSize + loupe.inset * 2
    implicitHeight: loupe.lensSize + loupe.inset + loupe.strip

    visible: opacity > 0
    opacity: loupe.shown ? 1 : 0
    scale: loupe.shown ? 1 : 0.94
    Behavior on opacity { enabled: loupe.shown; NumberAnimation { duration: Look.dur(140); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.decelerate } }
    Behavior on scale { enabled: loupe.shown; NumberAnimation { duration: Look.dur(160); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.decelerate } }
    Surface {
        anchors.fill: parent
        u: loupe.u
        cut: Look.cutLg
        cutTopRight: 0
        cutBottomLeft: 0
        elevation: 2
        fill: Look.planes.pane
        stroke: Look.vein
    }

    Loupe {
        x: loupe.inset
        y: loupe.inset
        width: loupe.lensSize
        height: loupe.lensSize
        source: loupe.chrome.plate
        cx: loupe.px
        cy: loupe.py
        dpr: loupe.chrome.dpr
        u: loupe.u
        markColor: loupe.mark
    }

    Row {
        x: loupe.inset + loupe.chrome.snap(2 * loupe.u)
        y: loupe.inset + loupe.lensSize + loupe.chrome.snap((loupe.strip - loupe.inset - height) / 2)
        spacing: loupe.chrome.snap(6 * loupe.u)

        Rectangle {
            visible: loupe.showSwatch
            anchors.verticalCenter: parent.verticalCenter
            width: loupe.chrome.snap(10 * loupe.u)
            height: width
            color: loupe.swatch
            border.width: loupe.chrome.hair
            border.color: Look.vein
            antialiasing: false
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: loupe.value
            color: Look.fg
            font.family: Look.mono
            font.pixelSize: Math.round(10 * loupe.u)
            font.letterSpacing: 0.6 * loupe.u
            font.features: ({ "tnum": 1 })
            renderType: Text.NativeRendering
        }
    }
}
