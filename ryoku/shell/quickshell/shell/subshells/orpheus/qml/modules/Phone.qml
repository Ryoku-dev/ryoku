import QtQuick
import Ryoku.Ui.Singletons
import shell.modules.phone
import "../kit" as K

Item {
    id: root

    required property var scene
    required property string lane

    readonly property bool hasPairedDevice: KdeConnectService.devices.some(device => {
        return device.paired === true;
    })
    readonly property bool reachable: KdeConnectService.activeDeviceId !== ""
        && KdeConnectService.activeReachable
    readonly property int charge: KdeConnectService.activeDevice?.charge ?? -1
    readonly property int unread: KdeConnectService.notificationCount

    property bool present: KdeConnectService.serviceEnabled && root.hasPairedDevice

    readonly property string hintText: {
        if (!root.reachable)
            return "Phone not connected";
        const name = KdeConnectService.activeDeviceDisplayName || "Phone";
        return root.charge >= 0 ? name + ", " + root.charge + "%" : name;
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
            name: root.reachable ? "smartphone" : "phonelink_off"
            size: K.Style.barIcon
            tone: root.reachable ? "dim" : "faint"
        }

        K.Label {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.reachable && root.charge >= 0
            text: root.charge + "%"
            role: "bar"
            numeric: true
        }

        K.Badge {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.reachable && root.unread > 0
            count: root.unread
        }
    }

    K.Press {
        id: press
        radius: K.Style.radius
        onClicked: Spawn.runApp(["ryoku-kit", "phone"], { label: "ryoku-kit" })
    }

    K.Hint {
        target: root
        text: root.hintText
        shown: root.present && press.hovered
    }
}
