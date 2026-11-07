import QtQuick
import "../kit" as K

Item {
    id: root

    required property var scene
    required property string lane
    property bool present: true

    implicitWidth: K.Style.barIcon + 2 * K.Style.hitPad
    implicitHeight: K.Style.barH
    height: K.Style.barH

    K.Glyph {
        anchors.centerIn: parent
        scale: press.pressScale
        name: "power_settings_new"
        size: K.Style.barIcon
        tone: "dim"
    }

    K.Press {
        id: press
        radius: K.Style.radius
        onClicked: root.scene.toggleCard("session", root)
        onHoveredChanged: root.scene.hoverCard("session", root, hovered)
    }

    K.Hint {
        target: root
        text: "Session actions"
        shown: press.hovered
    }
}
