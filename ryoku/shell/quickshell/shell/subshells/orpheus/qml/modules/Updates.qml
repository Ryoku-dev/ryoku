import QtQuick
import shell.services
import "../kit" as K

Item {
    id: root

    required property var scene
    required property string lane

    property bool present: Updates.pending > 0
    readonly property string hintText: {
        const count = Updates.pending;
        const base = count + (count === 1 ? " update" : " updates");
        return Updates.channel.length > 0 ? base + ", " + Updates.channel : base;
    }

    implicitWidth: present ? body.implicitWidth + 2 * K.Style.hitPad : 0
    implicitHeight: K.Style.barH
    height: K.Style.barH

    Row {
        id: body
        anchors.centerIn: parent
        spacing: K.Style.s1
        scale: press.pressScale

        K.Glyph {
            anchors.verticalCenter: parent.verticalCenter
            name: "system_update_alt"
            size: K.Style.barIcon
            tone: "dim"
        }

        K.Badge {
            anchors.verticalCenter: parent.verticalCenter
            count: Updates.pending
        }
    }

    K.Press {
        id: press
        radius: K.Style.radius
        onClicked: root.scene.openHub("updates")
        onRightClicked: Updates.check()
    }

    K.Hint {
        target: root
        text: root.hintText
        shown: root.present && press.hovered
    }
}