pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import caelestia.utils

Singleton {
    id: root

    readonly property string home: Quickshell.env("HOME") || ""
    readonly property string envCheckout: Quickshell.env("CAELESTIA_DIR") || ""

    readonly property string shellConfigDir: {
        const config = Quickshell.env("CAELESTIA_SHELL_CONFIG");
        if (config)
            return config.slice(0, config.lastIndexOf("/"));
        return root.home + "/.config/quickshell/caelestia";
    }

    readonly property var packageManagers: [
        { tool: "pacman", command: "sudo pacman -Rns caelestia-kde" },
        { tool: "dnf", command: "sudo dnf remove caelestia-kde" },
        { tool: "apt-get", command: "sudo apt-get remove caelestia-kde" }
    ]

    readonly property string state: {
        if (!root.probed)
            return "probing";
        if (root.scriptPath !== "")
            return "script";
        if (root.manualCommand !== "")
            return "package";
        return "unknown";
    }

    readonly property bool scriptFound: root.state === "script"

    property bool probed: false
    property string scriptPath: ""
    property string manualCommand: ""

    function launch(): void {
        if (!root.scriptFound)
            return;

        Launch.launchInTerminal(["bash", root.scriptPath], "");
    }

    Process {
        id: probe

        running: true
        command: ["sh", "-c", `
home="$1"
caelestia_dir="$2"
config_dir="$3"
shift 3

report_uninstaller() {
    if [ -n "$1" ] && [ -f "$1/uninstall.sh" ]; then
        printf 'SCRIPT %s\n' "$1/uninstall.sh"
        exit 0
    fi
}

recorded="$(cat "$config_dir/.checkout" 2>/dev/null || true)"

report_uninstaller "$caelestia_dir"
report_uninstaller "$home/caelestia-kde"
report_uninstaller "$recorded"
report_uninstaller "$home/.config/caelestia-update/repo"
report_uninstaller "$home/.cache/caelestia-update-repo"

for manager in "$@"; do
    case "$manager" in
        pacman)
            pacman -Q caelestia-kde >/dev/null 2>&1 && printf 'PACKAGE pacman\n' && exit 0
            ;;
        dnf)
            dnf list installed caelestia-kde >/dev/null 2>&1 && printf 'PACKAGE dnf\n' && exit 0
            ;;
        apt-get)
            dpkg-query -W -f='\${Status}' caelestia-kde 2>/dev/null | grep -qx 'install ok installed' && printf 'PACKAGE apt-get\n' && exit 0
            ;;
    esac
done
echo UNKNOWN`, "--", root.home, root.envCheckout, root.shellConfigDir, ...root.packageManagers.map(m => m.tool)]
        stdout: StdioCollector {
            onStreamFinished: {
                const lines = text.split("\n");
                const script = lines.find(l => l.startsWith("SCRIPT "));
                const manager = lines.find(l => l.startsWith("PACKAGE "));
                if (script)
                    root.scriptPath = script.slice("SCRIPT ".length).trim();
                if (manager) {
                    const tool = manager.slice("PACKAGE ".length).trim();
                    const entry = root.packageManagers.find(m => m.tool === tool);
                    root.manualCommand = entry ? entry.command : "";
                }
                root.probed = true;
            }
        }
    }
}
