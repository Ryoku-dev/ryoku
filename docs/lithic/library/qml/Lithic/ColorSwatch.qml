import QtQuick

Item {
    id: root

    property color color: "transparent"
    property real cut: LithicTokens.cut.token

    implicitWidth: LithicTokens.size.textFieldHeight
    implicitHeight: implicitWidth

    CutRect {
        anchors.fill: parent
        fill: Lithic.field
        cut: root.cut
    }

    CutClip {
        anchors.fill: parent
        cut: root.cut

        Hatch {
            width: root.width
            height: root.height
        }
    }

    CutRect {
        anchors.fill: parent
        fill: root.color
        strokeColor: Lithic.edge
        strokeWidth: LithicTokens.size.hairline
        cut: root.cut
    }
}
