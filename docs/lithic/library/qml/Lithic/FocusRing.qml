import QtQuick
import QtQuick.Shapes

Item {
    id: root

    property bool keyboardFocusVisible: false
    property real cut: 0
    readonly property real _scale: Lithic.scaleOf(root)
    property real gap: Lithic.focusGap * root._scale
    property real ringWidth: Lithic.focusWidth * root._scale
    property color color: Lithic.focusColor
    readonly property real outset: gap + ringWidth
    readonly property real _grownCut: (cut + LithicTokens.size.focusCutGrow) * root._scale
    visible: keyboardFocusVisible
    // the nearest DesignScope's design, else the desktop's
    property Item _scope: null
    onParentChanged: root._scope = Lithic.designScope(root)
    Component.onCompleted: root._scope = Lithic.designScope(root)
    readonly property string _design: root._scope ? root._scope.lithicDesign : ""
    readonly property bool _round: (root._design !== "" ? (Lithic.designs[root._design] || Lithic.shape) : Lithic.shape).corner === "round"

    // the ring is drawn only while it shows, so an idle control carries no shape for it; a
    // design that rounds its corners rounds the ring with them
    Rectangle {
        visible: root.visible && root._round
        anchors.fill: parent
        color: "transparent"
        border.width: root.ringWidth
        border.color: root.color
        radius: Lithic.cornerRadius(root._grownCut, root.width, root.height, root._design)
    }
    Loader {
        anchors.fill: parent
        active: root.visible && !root._round
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
