pragma ComponentBehavior: Bound
import QtQuick
import Ryoku.Ui.Singletons
import QtQuick.Shapes
import "../Singletons"

// Day Progress: a ring that fills with the fraction of the day elapsed, the
// current time and percent at its centre, the date beneath. Ported from the
// iRiS desktop-widget family into Ryoku's paper-and-ink: the track is a
// hairline of the widget's own ink, the fill is the accent, and both follow the
// wallpaper luminance under the widget (or a pinned colour) like every face
// here. Time-only, so it costs nothing but the shared one-second tick.
Item {
    id: face

    property real underL: Scheme.wallLstar
    property string inkColorA: ""
    property real s: 1

    readonly property color ink:     Theme.inkOn2(face.underL, face.inkColorA)
    readonly property color inkDim:  Theme.inkDimOn2(face.underL, face.inkColorA)
    readonly property color accent:  Theme.accentOn2(face.underL, face.inkColorA)

    // ring | arc: the arc drops the background track for a lighter mark.
    readonly property string style: Config.dayprogressStyle
    readonly property bool showDate: Config.dayprogressShowDate

    readonly property real fraction: {
        const d = Now.date;
        return (d.getHours() * 3600 + d.getMinutes() * 60 + d.getSeconds()) / 86400;
    }
    readonly property string timeText: TimeFormat.format(Now.date)

    readonly property real dim: Math.round(200 * face.s)
    implicitWidth: face.dim
    implicitHeight: col.implicitHeight

    Column {
        id: col
        width: face.dim
        spacing: Math.round(10 * face.s)

        Item {
            id: dial
            width: face.dim
            height: face.dim
            readonly property real stroke: Math.round(9 * face.s)
            readonly property real r: (face.dim - dial.stroke) / 2 - Math.round(2 * face.s)

            Shape {
                anchors.fill: parent
                preferredRendererType: Shape.CurveRenderer

                ShapePath {
                    fillColor: "transparent"
                    // ShapePath is not an Item, so the track is dropped by going
                    // transparent in arc style rather than toggling visibility.
                    strokeColor: face.style === "ring"
                        ? Qt.rgba(face.ink.r, face.ink.g, face.ink.b, 0.16) : "transparent"
                    strokeWidth: dial.stroke
                    capStyle: ShapePath.RoundCap
                    PathAngleArc {
                        centerX: dial.width / 2
                        centerY: dial.height / 2
                        radiusX: dial.r
                        radiusY: dial.r
                        startAngle: -90
                        sweepAngle: 360
                    }
                }
                ShapePath {
                    fillColor: "transparent"
                    strokeColor: face.accent
                    strokeWidth: dial.stroke
                    capStyle: ShapePath.RoundCap
                    PathAngleArc {
                        centerX: dial.width / 2
                        centerY: dial.height / 2
                        radiusX: dial.r
                        radiusY: dial.r
                        startAngle: -90
                        sweepAngle: Math.max(0.001, 360 * face.fraction)
                    }
                }
            }

            Column {
                anchors.centerIn: parent
                spacing: 0
                Text {
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: face.timeText
                    color: face.ink
                    font.family: Theme.font
                    font.pixelSize: Math.round(38 * face.s)
                    font.weight: Font.DemiBold
                }
                Text {
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: Math.round(face.fraction * 100) + "% \u00b7 of the day"
                    color: face.inkDim
                    font.family: Theme.font
                    font.pixelSize: Math.round(12 * face.s)
                    font.weight: Font.Medium
                    font.letterSpacing: Math.round(0.6 * face.s)
                }
            }
        }

        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            visible: face.showDate
            text: Qt.formatDate(Now.date, "ddd, MMM d")
            color: face.inkDim
            font.family: Theme.font
            font.pixelSize: Math.round(13 * face.s)
            font.weight: Font.Medium
            font.letterSpacing: Math.round(0.6 * face.s)
        }
    }
}
