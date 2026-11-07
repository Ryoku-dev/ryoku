pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import "../.." as Pill
import shell.services
import "../../../../components"
import Ryoku.Ui.Singletons

// One quick-action row (contract 06 sec 2.11): a centred horizontal row of
// 48x48 surface tiles, 12px apart, one per configured action. Toggle actions
// highlight (primary) while on and flip their backing state without closing the
// menu; command actions run and close. Icons are 24px Material Symbols.
//
// Divergence recorded: the reference wraps Logout/Reboot/Shutdown in a
// ConfirmationDialog first; Ryoku fires them through the same immediate path its
// bar power actions already use (runBarAction), so behaviour matches the rest of
// the Ryoku shell. A ConfirmationDialog (contract 16 sec 2.2) is the follow-up.
Item {
    id: root

    property real s: 1
    property bool open: false
    property var actions: []
    signal requestClose()

    implicitHeight: 48

    function isToggle(id) { return id === "airplane" || id === "night-light"; }
    function isOn(id) {
        switch (id) {
        case "airplane": return !Toggles.wifiOn;
        case "night-light": return Toggles.nightOn;
        }
        return false;
    }
    function iconFor(id) {
        switch (id) {
        case "airplane": return "flight";
        case "night-light": return "bedtime";
        case "color": return "colorize";
        case "settings": return "settings";
        case "logout": return "logout";
        case "lock": return "lock";
        case "reboot": return "restart_alt";
        case "shutdown": return "power_settings_new";
        }
        return "circle";
    }
    function act(id) {
        switch (id) {
        case "airplane": Toggles.toggleWifi(); return;
        case "night-light": Toggles.toggleNight(); return;
        case "color": Quickshell.execDetached(["ryoku-cmd-color-picker"]); root.requestClose(); return;
        case "settings": Bus.run("hub open"); root.requestClose(); return;
        case "lock": Bus.run("lock"); root.requestClose(); return;
        case "logout": SessionActions.run("logout"); return;
        case "reboot": Quickshell.execDetached(["systemctl", "reboot"]); return;
        case "shutdown": Quickshell.execDetached(["systemctl", "poweroff"]); return;
        }
    }

    Row {
        anchors.horizontalCenter: parent.horizontalCenter
        spacing: 12

        Repeater {
            model: root.actions
            delegate: MenuButton {
                id: tile
                required property var modelData
                minW: 48
                minH: 48
                selected: root.isToggle(tile.modelData) && root.isOn(tile.modelData)
                onClicked: root.act(tile.modelData)
                MaterialIcon {
                    anchors.centerIn: parent
                    width: Theme.iconMd
                    height: Theme.iconMd
                    font.pixelSize: Theme.iconMd
                    text: root.iconFor(tile.modelData)
                    color: tile.contentColor
                }
            }
        }
    }
}
