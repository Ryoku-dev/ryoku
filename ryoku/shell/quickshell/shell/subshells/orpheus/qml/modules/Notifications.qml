import QtQuick
import shell.services
import "../kit" as K

Item {
    id: root

    required property var scene
    required property string lane
    property bool present: true

    readonly property int count: (Notifs.tracked || []).length
    readonly property string hintText: {
        const amount = count + (count === 1 ? " notification" : " notifications");
        return Notifs.dnd ? "Do not disturb, " + amount : amount;
    }

    implicitWidth: body.implicitWidth + 2 * K.Style.hitPad
    implicitHeight: K.Style.barH
    height: K.Style.barH

    Item {
        id: body
        anchors.centerIn: parent
        implicitWidth: K.Style.barIcon
            + (badge.visible ? Math.max(0, badge.implicitWidth - K.Style.s1) : 0)
        implicitHeight: Math.max(K.Style.barIcon, badge.implicitHeight)
        width: implicitWidth
        height: implicitHeight
        scale: press.pressScale

        K.Glyph {
            id: bell
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            name: Notifs.dnd ? "notifications_off" : "notifications"
            size: K.Style.barIcon
            tone: Notifs.dnd ? "faint" : "dim"
            filled: root.count > 0 && !Notifs.dnd
        }

        K.Badge {
            id: badge
            anchors.left: bell.right
            anchors.leftMargin: -K.Style.s1
            anchors.verticalCenter: parent.verticalCenter
            count: root.count
        }
    }

    K.Press {
        id: press
        radius: K.Style.radius
        onClicked: root.scene.toggleCard("notifications", root)
        onRightClicked: Toggles.toggleDnd()
        onHoveredChanged: root.scene.hoverCard("notifications", root, hovered)
    }

    K.Hint {
        target: root
        text: root.hintText
        shown: press.hovered
    }
}
