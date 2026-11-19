import QtQuick
import QtQuick.Shapes
import "../look"

// Fluent's capture chrome, after the Windows 11 Snipping Tool: a thin light
// outline with small accent handles while editing, the size on a tooltip card
// beside the corner being placed, an accent outline around the window under the
// pointer with a reveal light that follows the pointer along it, and a small
// rounded loupe. A drag that will record turns the outline red.
Item {
    id: look

    required property var chrome

    readonly property real u: look.chrome.u
    property color frameInk: look.chrome.record ? Look.alert : look.chrome.lightInk
    Behavior on frameInk { ColorAnimation { duration: Look.dur(167) } }
    readonly property color accent: look.chrome.record ? Look.alert : Look.sel

    RoundOutline {
        id: hover
        anchors.fill: parent
        chrome: look.chrome
        radius: 8
        strokeWidth: 2
        stroke: look.accent
        fillAlpha: 0.08

        // the reveal: the outline brightens where the pointer is nearest
        Shape {
            id: reveal
            readonly property real sw: look.chrome.px2(2)
            anchors.fill: parent
            visible: look.chrome.pointerIn
            preferredRendererType: Shape.CurveRenderer
            ShapePath {
                strokeColor: "transparent"
                fillRule: ShapePath.OddEvenFill
                fillGradient: RadialGradient {
                    centerX: look.chrome.px
                    centerY: look.chrome.py
                    centerRadius: 120 * look.u
                    focalX: centerX
                    focalY: centerY
                    GradientStop { position: 0; color: Qt.rgba(look.chrome.lightInk.r, look.chrome.lightInk.g, look.chrome.lightInk.b, 0.95) }
                    GradientStop { position: 1; color: "transparent" }
                }
                PathRectangle {
                    x: look.chrome.snap(hover.rx) - reveal.sw
                    y: look.chrome.snap(hover.ry) - reveal.sw
                    width: look.chrome.snap(hover.rw) + reveal.sw * 2
                    height: look.chrome.snap(hover.rh) + reveal.sw * 2
                    radius: 8 * look.u + reveal.sw
                }
                PathRectangle {
                    x: look.chrome.snap(hover.rx)
                    y: look.chrome.snap(hover.ry)
                    width: look.chrome.snap(hover.rw)
                    height: look.chrome.snap(hover.rh)
                    radius: 8 * look.u
                }
            }
        }

        ValueChip {
            readonly property real inset: 12 * look.u
            visible: look.chrome.hoverApp !== ""
            x: look.chrome.snap(Math.max(hover.rx, 0) + inset)
            y: look.chrome.snap(Math.max(hover.ry, 0) + inset)
            u: look.u
            text: look.chrome.hoverApp + "  \u00b7  " + look.chrome.hoverSizeText
            fill: Look.planes.card
            ink: Look.fg
            stroke: Look.planeStroke
            chipHeight: 28
            padX: 10
            radius: 4
            fontSize: 12
            fontWeight: Font.Normal
            elevation: 1
        }
    }

    Item {
        id: region
        anchors.fill: parent
        visible: look.chrome.hasArea
        readonly property var s: look.chrome.sel

        Rectangle {
            x: region.s ? look.chrome.snap(region.s.x) - look.chrome.hair : 0
            y: region.s ? look.chrome.snap(region.s.y) - look.chrome.hair : 0
            width: region.s ? look.chrome.snap(region.s.w) + look.chrome.hair * 2 : 0
            height: region.s ? look.chrome.snap(region.s.h) + look.chrome.hair * 2 : 0
            color: "transparent"
            border.width: look.chrome.hair
            border.color: look.frameInk
            antialiasing: false
        }

        RoundHandles {
            anchors.fill: parent
            visible: look.chrome.editing
            chrome: look.chrome
            rect: region.s
            cornerSize: 8
            edgeLong: 0
            fill: look.accent
            ring: look.chrome.lightInk
            ringWidth: 1.5
            hotRole: look.chrome.hotGrip
            downRole: look.chrome.gripRole
        }
    }

    Item {
        readonly property var b: look.chrome.annBox
        anchors.fill: parent
        visible: b !== null
        Rectangle {
            x: parent.b ? look.chrome.snap(parent.b.x) : 0
            y: parent.b ? look.chrome.snap(parent.b.y) : 0
            width: parent.b ? look.chrome.snap(parent.b.w) : 0
            height: parent.b ? look.chrome.snap(parent.b.h) : 0
            radius: 4 * look.u
            color: "transparent"
            border.width: look.chrome.hair
            border.color: Look.sel
            antialiasing: true
        }
        RoundHandles {
            anchors.fill: parent
            chrome: look.chrome
            rect: parent.b
            edges: false
            cornerSize: 7
            fill: Look.sel
            ring: look.chrome.lightInk
            ringWidth: 1.5
        }
    }

    // the size tooltip and the loupe beside the corner being placed
    Item {
        id: corner
        anchors.fill: parent
        readonly property var p: look.chrome.loupePoint
        readonly property bool showSize: look.chrome.corner !== null && look.chrome.hasArea
        readonly property var place: corner.p
            ? look.chrome.placeAround(corner.p,
                corner.showSize ? size.width : 0, corner.showSize ? size.height : 0,
                lens.implicitWidth, lens.implicitHeight)
            : null

        ValueChip {
            id: size
            shown: corner.showSize && corner.place !== null
            x: corner.place && corner.place.readout ? look.chrome.snap(corner.place.readout.x) : 0
            y: corner.place && corner.place.readout ? look.chrome.snap(corner.place.readout.y) : 0
            u: look.u
            lead: look.chrome.record ? "\u25cf" : ""
            leadInk: Look.alert
            text: look.chrome.selSizeText
            fill: Look.planes.card
            ink: Look.fg
            stroke: look.chrome.record ? Look.alert : Look.planeStroke
            chipHeight: 28
            padX: 10
            radius: 4
            fontSize: 12
            fontWeight: Font.Normal
            elevation: 1
        }

        FluentLoupe {
            id: lens
            chrome: look.chrome
            shown: corner.p !== null
            x: corner.place ? look.chrome.snap(corner.place.loupe.x) : 0
            y: corner.place ? look.chrome.snap(corner.place.loupe.y) : 0
            px: corner.p ? corner.p.x : 0
            py: corner.p ? corner.p.y : 0
            mark: look.accent
            value: look.chrome.picking ? (look.chrome.pickHex || "#\u2026") : look.chrome.pointText
        }
    }
}
