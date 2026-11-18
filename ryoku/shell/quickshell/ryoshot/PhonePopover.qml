pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import "look"

// The phone to send to, when more than one is connected.
Menu {
    id: pop

    property var devices: []

    signal picked(string deviceId, string deviceName)

    MenuItem {
        u: pop.u
        kind: "heading"
        text: I18n.tr("Send to phone")
    }
    Repeater {
        model: pop.devices
        MenuItem {
            required property var modelData
            u: pop.u
            icon: "phone"
            text: modelData.name
            onTriggered: pop.picked(String(modelData.deviceId), String(modelData.name))
        }
    }
}
