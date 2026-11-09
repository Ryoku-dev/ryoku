import QtQuick

Plaque {
    id: root

    property string notchTone: ""

    text: ""
    leftPadding: 0
    rightPadding: 0
    implicitWidth: small ? LithicTokens.size.iconPlaqueSmall : LithicTokens.size.iconPlaque
    implicitHeight: implicitWidth
    Accessible.name: accessibleName.length > 0 ? accessibleName : glyph

    contentItem: Item {
        implicitWidth: root.small ? LithicTokens.size.iconPlaqueIconSmall : LithicTokens.size.iconPlaqueIcon
        implicitHeight: implicitWidth

        Working {
            anchors.centerIn: parent
            visible: root.busy
            active: root.busy
            size: root.small ? LithicTokens.size.iconPlaqueIconSmall : LithicTokens.size.iconPlaqueIcon
        }

        Glyph {
            anchors.centerIn: parent
            visible: !root.busy
            width: root.small ? LithicTokens.size.iconPlaqueIconSmall : LithicTokens.size.iconPlaqueIcon
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
        fill: Lithic.stone(root.notchTone)
        corner: "topLeft"
    }
}
