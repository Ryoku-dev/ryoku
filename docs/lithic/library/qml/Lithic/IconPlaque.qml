import QtQuick

Plaque {
    id: root

    property string notchTone: ""

    readonly property real _scale: Lithic.scaleOf(root)
    readonly property real _side: (small ? LithicTokens.size.iconPlaqueSmall : LithicTokens.size.iconPlaque) * root._scale

    text: ""
    leftPadding: 0
    rightPadding: 0
    implicitWidth: root._side
    implicitHeight: implicitWidth
    Accessible.name: accessibleName.length > 0 ? accessibleName : glyph

    contentItem: Item {
        readonly property real icon: (root.small ? LithicTokens.size.iconPlaqueIconSmall : LithicTokens.size.iconPlaqueIcon) * root._scale

        implicitWidth: icon
        implicitHeight: icon

        Working {
            anchors.centerIn: parent
            visible: root.busy
            active: root.busy
            size: parent.icon
        }

        Glyph {
            anchors.centerIn: parent
            visible: !root.busy
            width: parent.icon
            height: width
            name: root.glyph
            ink: root.ink
            size: root.small ? LithicTokens.size.iconPlaqueIconSmall : LithicTokens.size.iconPlaqueIcon
        }
    }

    Notch {
        anchors.left: parent.left
        anchors.top: parent.top
        visible: root.notchTone.length > 0
        size: LithicTokens.cut.notch * root._scale
        fill: Lithic.stone(root.notchTone)
        corner: "topLeft"
    }
}
