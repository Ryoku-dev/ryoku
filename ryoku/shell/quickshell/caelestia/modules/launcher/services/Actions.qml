pragma Singleton

import ".."
import QtQuick
import Quickshell
import Caelestia.Config
import Caelestia.Services
import caelestia.services
import caelestia.utils

Searcher {
    id: root

    readonly property var labels: ({
        "Calculator": qsTr("Calculator"),
        "Scheme": qsTr("Scheme"),
        "Wallpaper": qsTr("Wallpaper"),
        "Variant": qsTr("Variant"),
        "Random": qsTr("Random"),
        "Light": qsTr("Light"),
        "Dark": qsTr("Dark"),
        "Shutdown": qsTr("Shutdown"),
        "Reboot": qsTr("Reboot"),
        "Logout": qsTr("Logout"),
        "Lock": qsTr("Lock"),
        "Sleep": qsTr("Sleep"),
        "Settings": qsTr("Settings"),
        "Emoji": qsTr("Emoji"),
        "Clipboard": qsTr("Clipboard"),
        "Windows": qsTr("Windows"),
        "Keybinds": qsTr("Keybinds"),
        "Animations": qsTr("Animations"),
        "Do simple math equations (powered by Qalc)": qsTr("Do simple math equations (powered by Qalc)"),
        "Change the current color scheme": qsTr("Change the current color scheme"),
        "Change the current wallpaper": qsTr("Change the current wallpaper"),
        "Change the current scheme variant": qsTr("Change the current scheme variant"),
        "Switch to a random wallpaper": qsTr("Switch to a random wallpaper"),
        "Change the scheme to light mode": qsTr("Change the scheme to light mode"),
        "Change the scheme to dark mode": qsTr("Change the scheme to dark mode"),
        "Shutdown the system": qsTr("Shutdown the system"),
        "Reboot the system": qsTr("Reboot the system"),
        "Log out of the current session": qsTr("Log out of the current session"),
        "Lock the current session": qsTr("Lock the current session"),
        "Suspend then hibernate": qsTr("Suspend then hibernate"),
        "Configure the shell": qsTr("Configure the shell"),
        "Pick an emoji to copy": qsTr("Pick an emoji to copy"),
        "View clipboard history": qsTr("View clipboard history"),
        "Switch to another window": qsTr("Switch to another window"),
        "View all keybinds": qsTr("View all keybinds"),
        "Switch your animation style": qsTr("Switch your animation style"),
    })

    function transformSearch(search: string): string {
        return search.slice(GlobalConfig.launcher.actionPrefix.length);
    }

    list: variants.instances
    useFuzzy: GlobalConfig.launcher.useFuzzy.actions

    Variants {
        id: variants

        model: {
            const enableDangerous = GlobalConfig.launcher.enableDangerousActions;
            return GlobalConfig.launcher.actions.values.filter(a => a.enabled && (enableDangerous || !a.dangerous));
        }

        Action {}
    }

    component Action: QtObject {
        required property var modelData
        readonly property string name: root.labels[modelData.name] ?? modelData.name ?? qsTr("Unnamed")
        readonly property string desc: root.labels[modelData.description] ?? modelData.description ?? qsTr("No description")
        readonly property string icon: modelData.icon ?? "help_outline"
        readonly property list<string> command: modelData.command ?? []
        readonly property bool enabled: modelData.enabled ?? true
        readonly property bool dangerous: modelData.dangerous ?? false

        function onClicked(list: AppList): void {
            if (command.length === 0)
                return;

            if (command[0] === "autocomplete" && command.length > 1) {
                list.search.text = `${GlobalConfig.launcher.actionPrefix}${command[1]} `;
            } else if (command[0] === "setMode" && command.length > 1) {
                list.visibilities.launcher = false;
                Colours.setMode(command[1]);
            } else {
                list.visibilities.launcher = false;
                if (!SessionManager.exec(command))
                    Quickshell.execDetached(command);
            }
        }
    }
}
