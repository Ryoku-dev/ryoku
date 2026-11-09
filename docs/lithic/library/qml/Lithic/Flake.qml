import QtQuick
import QtQuick.Shapes

Item {
    id: root

    property string tone: "idle"
    property real fillFraction: tone === "idle" ? 0 : 1
    property alias progress: root.fillFraction
    property real size: LithicTokens.size.flakeNominal
    property color outline: Lithic.vein
    property real outlineWidth: LithicTokens.size.flakeOutline
    property color fill: Lithic.stone(tone)
    readonly property real _halfOutline: outlineWidth / 2
    implicitWidth: size * LithicTokens.size.flakeWidthRatio
    implicitHeight: size * LithicTokens.size.flakeHeightRatio

    Shape {
        anchors.fill: parent
        antialiasing: true
        ShapePath {
            fillColor: "transparent"
            strokeColor: root.outline
            strokeWidth: root.outlineWidth
            joinStyle: ShapePath.MiterJoin
            startX: root.height
            startY: root._halfOutline
            PathLine { x: root.width - root._halfOutline; y: root._halfOutline }
            PathLine { x: root.width - root.height; y: root.height - root._halfOutline }
            PathLine { x: root._halfOutline; y: root.height - root._halfOutline }
            PathLine { x: root.height; y: root._halfOutline }
        }
    }

    Item {
        width: Math.max(0, Math.min(1, root.fillFraction)) * root.width
        height: root.height
        clip: true
        visible: width > 0

        Shape {
            width: root.width
            height: root.height
            antialiasing: true
            ShapePath {
                fillColor: root.fill
                strokeColor: "transparent"
                startX: root.height
                startY: 0
                PathLine { x: root.width; y: 0 }
                PathLine { x: root.width - root.height; y: root.height }
                PathLine { x: 0; y: root.height }
                PathLine { x: root.height; y: 0 }
            }
        }
    }
}
