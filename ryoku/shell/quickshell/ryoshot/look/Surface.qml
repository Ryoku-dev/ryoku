import QtQuick
import QtQuick.Effects
import Lithic

// A plane in the design's shape: Lithic's cut corners and hairline, Material's
// tonal plane with no line, Adwaita's card with its shade border, Fluent's
// acrylic with an outer and an inner outline. Cuts are logical px; u is the
// output's UI scale. A corner given 0 stays square in every design, so a bar
// that sits flush on an edge keeps that edge flat.
Item {
    id: root

    property real u: 1
    property real cut: Look.cutLg
    property real cutTopLeft: root.cut
    property real cutTopRight: root.cut
    property real cutBottomRight: root.cut
    property real cutBottomLeft: root.cut
    // a fixed corner radius in scaled px for the round designs, else derived from the cut
    property real radius: -1
    property color fill: Look.fluent ? Look.acrylic : Look.plane
    property color stroke: Look.planeStroke
    property bool shadow: true
    // the shadow's depth: 1 a tooltip, 2 a menu, 3 a floating bar
    property int elevation: 3
    // Fluent's acrylic grain and inner highlight
    property bool acrylic: Look.fluent

    default property alias content: inner.data

    readonly property real hairline: Math.max(1, Math.round(root.u))
    function _corner(c) {
        if (c <= 0)
            return 0;
        if (Look.lithic)
            return c * root.u;
        if (root.radius >= 0)
            return Math.min(root.radius, Math.min(root.width, root.height) / 2);
        return Look.cornerAt(c, root.width, root.height, root.u);
    }
    readonly property real rTL: root._corner(root.cutTopLeft)
    readonly property real rTR: root._corner(root.cutTopRight)
    readonly property real rBR: root._corner(root.cutBottomRight)
    readonly property real rBL: root._corner(root.cutBottomLeft)

    RectangularShadow {
        visible: root.shadow
        anchors.fill: parent
        offset.y: (root.elevation >= 3 ? 4 : root.elevation === 2 ? 3 : 2) * root.u
        blur: (root.elevation >= 3 ? 16 : root.elevation === 2 ? 12 : 6) * root.u
        radius: Math.max(root.rTL, root.rBR)
        color: Look.shadow
    }

    CutRect {
        visible: Look.lithic
        anchors.fill: parent
        design: "lithic"
        fill: root.fill
        strokeColor: root.stroke
        strokeWidth: root.hairline
        cutTopLeft: root.rTL
        cutTopRight: root.rTR
        cutBottomRight: root.rBR
        cutBottomLeft: root.rBL
    }

    Rectangle {
        visible: !Look.lithic
        anchors.fill: parent
        color: root.fill
        border.width: root.stroke.a > 0 ? root.hairline : 0
        border.color: root.stroke
        topLeftRadius: root.rTL
        topRightRadius: root.rTR
        bottomRightRadius: root.rBR
        bottomLeftRadius: root.rBL
        antialiasing: true
    }

    // the acrylic grain kept inside the corners as a cross of three tiles, so it
    // needs no offscreen layer
    Item {
        id: grain
        readonly property real r: Math.max(root.rTL, root.rTR, root.rBR, root.rBL)
        visible: !Look.lithic && root.acrylic
        anchors.fill: parent
        opacity: 0.55
        Image {
            x: grain.r
            y: root.hairline
            width: grain.width - grain.r * 2
            height: grain.r - root.hairline
            source: "noise.png"
            fillMode: Image.Tile
            smooth: false
        }
        Image {
            x: root.hairline
            y: grain.r
            width: grain.width - root.hairline * 2
            height: grain.height - grain.r * 2
            source: "noise.png"
            fillMode: Image.Tile
            smooth: false
        }
        Image {
            x: grain.r
            y: grain.height - grain.r
            width: grain.width - grain.r * 2
            height: grain.r - root.hairline
            source: "noise.png"
            fillMode: Image.Tile
            smooth: false
        }
    }
    Rectangle {
        visible: !Look.lithic && root.acrylic
        anchors.fill: parent
        anchors.margins: root.hairline
        color: "transparent"
        border.width: root.hairline
        border.color: Look.highlight
        topLeftRadius: Math.max(0, root.rTL - root.hairline)
        topRightRadius: Math.max(0, root.rTR - root.hairline)
        bottomRightRadius: Math.max(0, root.rBR - root.hairline)
        bottomLeftRadius: Math.max(0, root.rBL - root.hairline)
    }

    Item {
        id: inner
        anchors.fill: parent
    }
}
