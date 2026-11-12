pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

// Settings' own colours. It opens in the signature, the ryoku.dev stock; following
// the wallpaper is the reader's choice, kept in the shell settings under `hub`
// (colours, contrast, tint). The file is read once here so the first frame already
// wears the choice; from then on the settings topic carries it.
Singleton {
    id: root

    readonly property var coloursChoices: ["ryoku", "light", "wallpaper"]
    readonly property var contrastChoices: ["standard", "high"]
    readonly property var tintChoices: ["off", "low", "medium"]

    readonly property var colours: ({ store: "settings", key: "hub.colours", fallback: "ryoku" })
    readonly property var contrast: ({ store: "settings", key: "hub.contrast", fallback: "standard" })
    readonly property var tint: ({ store: "settings", key: "hub.tint", fallback: "off" })

    // a harness or a gallery run names the mode itself; the reader's choice then waits
    readonly property bool forced: ["dark", "light", "follow"].indexOf(Quickshell.env("LITHIC_MODE") || "") >= 0

    property var atStart: ({})

    function chosen(key, choices) {
        var v = Settings.loaded ? Settings.get("hub." + key) : root.atStart[key];
        return choices.indexOf(v) >= 0 ? v : choices[0];
    }

    readonly property string coloursNow: root.chosen("colours", root.coloursChoices)
    readonly property string mode: root.coloursNow === "wallpaper" ? "follow" : root.coloursNow === "light" ? "light" : "dark"
    readonly property string contrastNow: root.chosen("contrast", root.contrastChoices)
    readonly property string tintNow: root.chosen("tint", root.tintChoices)

    FileView {
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/shell.json"
        blockLoading: true
        printErrors: false
        onLoaded: {
            try {
                var hub = JSON.parse(text()).hub;
                root.atStart = hub && typeof hub === "object" ? hub : ({});
            } catch (e) {
                root.atStart = ({});
            }
        }
    }
}
