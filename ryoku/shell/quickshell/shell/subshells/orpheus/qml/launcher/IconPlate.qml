import QtQuick
import QtQuick.Effects
import QtQuick.Shapes
import "../kit" as K
import "lib/rows.js" as Rows

// The lead's icon on its plate, per design: Lithic a cut plate ruled by a
// hairline over a giant faint rank numeral, Material a tonal circle, Adwaita the
// bare icon a size up, Fluent an outlined small-corner tile. The round designs
// set a soft halo behind the icon instead of the numeral.
Item {
    id: root

    // an image url or path (app and window icons), else the symbol is drawn
    property string source: ""
    property string glyph: "apps"
    // the selected row's index, -1 for none
    property int rank: -1
    // the selection-change rise, in px, applied to the icon only
    property real rise: 0

    readonly property real iconSize: K.Look.adwaita ? 72 : 64
    readonly property real plateSize: K.Look.adwaita ? 72 : K.Look.material ? 96 : 88
    readonly property bool hasImage: root.source.length > 0 && image.status !== Image.Error

    implicitWidth: root.plateSize
    implicitHeight: root.plateSize

    // printed beside the plate like a folio, never under it, so both digits read
    K.Label {
        id: rankNumeral
        visible: K.Look.lithic && text.length > 0
        x: root.plateSize + K.Style.s3
        anchors.verticalCenter: plate.verticalCenter
        text: root.rank >= 0 ? Rows.indexLabel(root.rank, true) : ""
        color: K.Style.alpha(K.Style.inkFaint, 0.6)
        font.family: K.Style.mono
        font.pixelSize: Math.round(root.plateSize * 0.86)
        font.weight: Font.Light
        font.features: K.Style.tabular
        font.letterSpacing: -2
        renderType: Text.QtRendering
        Accessible.ignored: true
    }

    K.Shape {
        id: plate
        visible: K.Look.lithic || K.Look.fluent
        width: root.plateSize
        height: root.plateSize
        cutSize: K.Look.lithic ? K.Look.cutLg : K.Look.cutSm
        fill: K.Look.lithic ? K.Style.base : K.Style.tint5
        strokeWidth: K.Style.border
        strokeColor: K.Look.lithic ? K.Style.line : K.Style.lineSoft
    }

    Shape {
        visible: K.Look.material
        width: root.plateSize
        height: root.plateSize
        preferredRendererType: Shape.CurveRenderer

        ShapePath {
            fillColor: K.Style.tint10
            strokeColor: "transparent"
            strokeWidth: 0
            startX: root.plateSize
            startY: root.plateSize / 2

            PathAngleArc {
                centerX: root.plateSize / 2
                centerY: root.plateSize / 2
                radiusX: root.plateSize / 2
                radiusY: root.plateSize / 2
                startAngle: 0
                sweepAngle: 360
            }
        }
    }

    Item {
        id: art
        width: root.iconSize
        height: root.iconSize
        x: (root.plateSize - width) / 2
        y: (root.plateSize - height) / 2 + root.rise

        MultiEffect {
            visible: !K.Look.lithic
            anchors.fill: parent
            source: root.hasImage ? image : symbol
            scale: 1.5
            opacity: 0.28
            blurEnabled: true
            blur: 1
            blurMax: 40
            autoPaddingEnabled: true
        }

        Image {
            id: image
            anchors.fill: parent
            visible: root.hasImage && status === Image.Ready
            source: root.source
            sourceSize: Qt.size(Math.round(root.iconSize * 2), Math.round(root.iconSize * 2))
            fillMode: Image.PreserveAspectFit
            asynchronous: true
            smooth: true
            mipmap: true
        }

        K.Glyph {
            id: symbol
            anchors.centerIn: parent
            visible: !root.hasImage
            name: root.glyph
            size: Math.round(root.iconSize * 0.62)
            tone: "ink"
        }
    }
}
