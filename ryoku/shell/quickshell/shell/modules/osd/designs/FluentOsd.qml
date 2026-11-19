pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import QtQuick.Shapes
import Lithic
import "OsdGlyph.js" as OsdGlyph

// Fluent's pop-up, after the Windows 11 volume flyout: a small layered card at the
// bottom centre, 28 above the edge, with an outer and an inner outline over a
// faintly tinted fill. The symbol, a slider with Fluent's ringed thumb and the
// value in one row; a subtle reveal light under the thumb follows the value. It
// slides up as it fades in. The keyboard layout shows the symbol and its name.
Item {
    id: root

    property string kind: "volume"
    property real u: 1
    property bool shown: false
    property real value: 0
    property real maxValue: 1
    property bool muted: false
    property string iconName: ""
    property string label: ""

    readonly property string placement: "bottom"
    readonly property real gap: Math.max(0, 28 * root.u - root.pad)
    readonly property real reveal: root.enter

    readonly property bool keyboard: root.kind === "keyboard"
    readonly property bool over: root.value > 1.005
    readonly property var planes: Lithic.planes("fluent")
    readonly property real pad: 16 * root.u
    readonly property real cardWidth: root.keyboard
        ? Math.round(Math.max(160 * root.u, Math.min(320 * root.u, layoutName.implicitWidth + 58 * root.u)))
        : Math.round(196 * root.u)
    readonly property real cardHeight: Math.round(48 * root.u)
    readonly property real hairline: Math.max(1, Math.round(root.u))
    readonly property color levelInk: root.muted ? Lithic.fg3 : Lithic.sel
    // the reveal light takes the colour of what the thumb stands on
    readonly property color lightInk: root.over ? Lithic.jasper : Lithic.sel

    implicitWidth: Math.ceil(root.cardWidth + root.pad * 2)
    implicitHeight: Math.ceil(root.cardHeight + root.pad * 2)

    property real enter: 0
    Behavior on enter {
        id: enterMotion
        NumberAnimation {
            duration: Lithic.duration(enterMotion.targetValue > 0.5 ? 250 : 150)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: enterMotion.targetValue > 0.5 ? [0, 0, 0, 1, 1, 1] : [0.7, 0, 1, 0.5, 1, 1]
        }
    }
    Component.onCompleted: root.enter = Qt.binding(() => root.shown ? 1 : 0)

    Item {
        id: card

        x: root.pad
        y: root.pad + 12 * root.u * (1 - root.enter)
        width: root.cardWidth
        height: root.cardHeight
        opacity: root.enter

        readonly property real radius: Lithic.cornerRadius(LithicTokens.cut.large, width / root.u,
            height / root.u, "fluent") * root.u

        RectangularShadow {
            anchors.fill: parent
            offset.y: 4 * root.u
            blur: 14 * root.u
            radius: card.radius
            color: Lithic.alpha(Lithic.scrim, Lithic.dark ? 0.7 : 0.28)
        }

        // the card's plane faintly tinted with the accent, then its two outlines
        Rectangle {
            anchors.fill: parent
            radius: card.radius
            color: Lithic.mix(root.planes.card, Lithic.sel, Lithic.dark ? 0.035 : 0.025)
            border.width: root.hairline
            border.color: root.planes.cardStroke
        }
        Rectangle {
            anchors.fill: parent
            anchors.margins: root.hairline
            radius: Math.max(0, card.radius - root.hairline)
            color: "transparent"
            border.width: root.hairline
            border.color: Lithic.alpha(Lithic.fg, Lithic.dark ? 0.06 : 0.05)
        }

        // the reveal light under the thumb, inside the card; a muted level casts none
        Shape {
            anchors.fill: parent
            visible: !root.keyboard
            preferredRendererType: Shape.CurveRenderer
            opacity: root.muted ? 0 : 1
            Behavior on opacity { NumberAnimation { duration: Lithic.duration(160) } }

            ShapePath {
                strokeColor: "transparent"
                fillGradient: RadialGradient {
                    centerX: slider.x + slider.thumbX
                    centerY: card.height / 2
                    centerRadius: 34 * root.u
                    focalX: centerX
                    focalY: centerY
                    GradientStop { position: 0; color: Lithic.alpha(root.lightInk, Lithic.dark ? 0.16 : 0.12) }
                    GradientStop { position: 1; color: "transparent" }
                }
                PathRectangle {
                    width: card.width
                    height: card.height
                    radius: card.radius
                }
            }
        }

        Text {
            id: glyph
            x: Math.round(14 * root.u)
            anchors.verticalCenter: parent.verticalCenter
            text: OsdGlyph.symbol(root.iconName)
            color: root.muted ? Lithic.fg3 : Lithic.fg
            font.family: "Material Symbols Outlined"
            font.variableAxes: ({ "wght": 300 })
            font.pixelSize: Math.round(16 * root.u)
            renderType: Text.CurveRendering
        }

        FluentSlider {
            id: slider
            visible: !root.keyboard
            anchors.left: glyph.right
            anchors.leftMargin: 10 * root.u
            anchors.right: figure.left
            anchors.rightMargin: 8 * root.u
            anchors.verticalCenter: parent.verticalCenter
            u: root.u
            value: root.value
            maxValue: root.maxValue
            ink: root.levelInk
            overInk: root.muted ? Lithic.fg3 : Lithic.jasper
            thumbPlane: Lithic.dark ? Lithic.mix(root.planes.card, Lithic.fg, 0.2) : Lithic.stone4
            thumbStroke: root.planes.cardStroke
        }

        Text {
            id: figure
            visible: !root.keyboard
            anchors.right: parent.right
            anchors.rightMargin: 14 * root.u
            anchors.verticalCenter: parent.verticalCenter
            width: Math.ceil(26 * root.u)
            horizontalAlignment: Text.AlignRight
            text: Math.round(root.value * 100)
            color: root.muted ? Lithic.fg3 : root.over ? Lithic.jasper : Lithic.fg
            font.family: Lithic.font.sans
            font.weight: Font.DemiBold
            font.pixelSize: Math.round(13 * root.u)
            font.features: ({ "tnum": 1 })
            renderType: Text.CurveRendering
        }

        Text {
            id: layoutName
            visible: root.keyboard
            anchors.left: glyph.right
            anchors.leftMargin: 12 * root.u
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, root.cardWidth - 58 * root.u)
            elide: Text.ElideRight
            text: root.label
            color: Lithic.fg
            font.family: Lithic.font.sans
            font.weight: Font.DemiBold
            font.pixelSize: Math.round(13 * root.u)
            renderType: Text.CurveRendering
        }
    }
}
