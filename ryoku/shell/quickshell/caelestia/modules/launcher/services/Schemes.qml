pragma Singleton
pragma ComponentBehavior: Bound

import ".."
import QtQuick
import Quickshell
import Quickshell.Io
import Caelestia.Config
import caelestia.utils

Searcher {
    id: root

    // The desktop's colour themes (`ryoku-shell theme catalog`), and the theme and
    // matugen variant in use, read from shell.json.
    property var catalog: []
    readonly property var themeSettings: {
        try {
            return JSON.parse(shellJson.text()).theme || {};
        } catch (e) {
            return {};
        }
    }
    readonly property string currentScheme: root.themeSettings.theme || ""
    readonly property string currentVariant: ((root.themeSettings.matugen || {}).scheme_type || "").toLowerCase()

    FileView {
        id: shellJson
        path: `${Quickshell.env("XDG_CONFIG_HOME") || `${Quickshell.env("HOME")}/.config`}/ryoku/shell.json`
        watchChanges: true
        onFileChanged: reload()
    }

    Process {
        id: catalogProc
        running: true
        command: ["ryoku-shell", "theme", "catalog"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    root.catalog = JSON.parse(text);
                } catch (e) {
                    console.warn("schemes: could not read the theme catalog:", e);
                }
            }
        }
    }

    function transformSearch(search: string): string {
        return search.slice(`${GlobalConfig.launcher.actionPrefix}scheme `.length);
    }

    function selector(item: var): string {
        return item.name;
    }

    function reload(): void {
        catalogProc.running = true;
    }

    list: schemes.instances
    useFuzzy: GlobalConfig.launcher.useFuzzy.schemes
    keys: ["name", "flavour"]
    weights: [0.9, 0.1]

    Variants {
        id: schemes

        model: root.catalog
        
        Scheme {}
    }

    component Scheme: QtObject {
        required property var modelData
        readonly property string name: modelData.label || modelData.id
        readonly property string flavour: ""
        // a fixed theme's swatches start with background, foreground and accent;
        // the dynamic ones (Default, Wallpaper) carry none and draw blank
        readonly property var colours: {
            const sw = (modelData.sw || []).map(c => String(c).replace(/^#/, ""));
            return { surface: sw[0] || "", outline: sw[1] || "", primary: sw[2] || "" };
        }

        function onClicked(list: var): void {
            if (list) {
                list.visibilities.launcher = false;
            }
            Quickshell.execDetached(["ryoku-shell", "theme", modelData.id]);
        }
    }
}
