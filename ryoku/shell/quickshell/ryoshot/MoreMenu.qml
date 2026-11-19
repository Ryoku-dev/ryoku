import QtQuick
import Ryoku.Ui.Singletons
import "look"

// The dock's overflow: the exports that leave the desktop and the help, each
// with the key that does the same. Send to phone shows only where KDE Connect
// is installed and names why it cannot send when it cannot.
Menu {
    id: menu

    property bool phoneAvailable: false
    property bool phoneReady: false
    property bool phoneBusy: false
    property var phoneDevices: []

    signal pinRequested()
    signal uploadRequested()
    signal phoneRequested()
    signal helpRequested()
    signal settingsRequested()

    readonly property bool phoneSendable: menu.phoneReady && !menu.phoneBusy && menu.phoneDevices.length > 0

    minWidth: 208

    MenuItem {
        u: menu.u
        icon: "pin"
        text: I18n.tr("Pin to desktop")
        keyHint: Look.keyHint("Ctrl+P")
        onTriggered: menu.pinRequested()
    }
    MenuItem {
        u: menu.u
        icon: "upload"
        text: I18n.tr("Upload")
        keyHint: Look.keyHint("Ctrl+U")
        onTriggered: menu.uploadRequested()
    }
    MenuItem {
        u: menu.u
        visible: menu.phoneAvailable
        enabled: menu.phoneSendable
        icon: "phone"
        text: menu.phoneBusy ? I18n.tr("Sending screenshot")
            : !menu.phoneReady ? I18n.tr("Phone transfer is unavailable")
            : menu.phoneDevices.length === 0 ? I18n.tr("No connected phone")
            : menu.phoneDevices.length === 1 ? I18n.tr("Send to %1").arg(menu.phoneDevices[0].name)
            : I18n.tr("Choose a phone")
        keyHint: menu.phoneSendable ? Look.keyHint("Ctrl+K") : ""
        onTriggered: menu.phoneRequested()
    }
    MenuItem {
        u: menu.u
        kind: "separator"
    }
    MenuItem {
        u: menu.u
        icon: "keyboard"
        text: I18n.tr("Keyboard shortcuts")
        keyHint: "?"
        onTriggered: menu.helpRequested()
    }
    MenuItem {
        u: menu.u
        icon: "gear"
        text: I18n.tr("Settings")
        onTriggered: menu.settingsRequested()
    }
}
