import QtQuick
import QtQuick.Shapes

Item {
    id: root

    property bool keyboardFocusVisible: false
    property real cut: 0
    property real gap: Lithic.focusGap
    property real ringWidth: Lithic.focusWidth
    property color color: Lithic.focusColor
    readonly property real outset: gap + ringWidth
    readonly property real _grownCut: cut + LithicTokens.size.focusCutGrow
    visible: keyboardFocusVisible

    // the ring is drawn only while it shows, so an idle control carries no shape for it
    Loader {
        anchors.fill: parent
        active: root.visible
        sourceComponent: Component {
            Shape {
                anchors.fill: parent
                antialiasing: true
                ShapePath {
                    fillColor: "transparent"
                    strokeColor: root.color
                    strokeWidth: root.ringWidth
                    joinStyle: ShapePath.MiterJoin
                    startX: root._grownCut
                    startY: root.ringWidth / 2
                    PathLine { x: root.width - root._grownCut; y: root.ringWidth / 2 }
                    PathLine { x: root.width - root.ringWidth / 2; y: root._grownCut }
                    PathLine { x: root.width - root.ringWidth / 2; y: root.height - root._grownCut }
                    PathLine { x: root.width - root._grownCut; y: root.height - root.ringWidth / 2 }
                    PathLine { x: root._grownCut; y: root.height - root.ringWidth / 2 }
                    PathLine { x: root.ringWidth / 2; y: root.height - root._grownCut }
                    PathLine { x: root.ringWidth / 2; y: root._grownCut }
                    PathLine { x: root._grownCut; y: root.ringWidth / 2 }
                }
            }
        }
    }
}
