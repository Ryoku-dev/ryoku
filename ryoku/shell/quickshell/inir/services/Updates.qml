pragma Singleton

import inir.modules.common
import inir.modules.common.functions
import QtQuick
import Quickshell
import Quickshell.Io

/*
 * System updates service for pacman/checkupdates, XBPS, and DNF.
 */
Singleton {
    id: root

    property bool available: false
    property int count: 0
    property string _backend: ""
    property string _availabilityOutput: ""
    
    readonly property bool updateAdvised: available && count > (Config.options?.updates?.adviseUpdateThreshold ?? 75)
    readonly property bool updateStronglyAdvised: available && count > (Config.options?.updates?.stronglyAdviseUpdateThreshold ?? 200)

    function load() {}
    function refresh() {
        if (!available) return;
        print("[Updates] Checking for system updates")
        if (checkUpdatesProc.running) return;
        root._checkOutput = null
        root._checkExit = -1
        if (root._backend === "xbps") {
            checkUpdatesProc.command = ["xbps-install", "-nu"]
        } else if (root._backend === "dnf") {
            checkUpdatesProc.command = ["/usr/bin/bash", "-c",
                "set -o pipefail; dnf_cmd=$(command -v dnf5 || command -v dnf) || exit 127; " +
                "\"$dnf_cmd\" -q repoquery --upgrades --qf '%{name}.%{arch}\\n' | sed '/^$/d' | sort -u"
            ]
        } else {
            checkUpdatesProc.command = ["checkupdates"]
        }
        checkUpdatesProc.running = true;
    }

    Timer {
        interval: (Config.options?.updates?.checkInterval ?? 120) * 60 * 1000
        repeat: true
        running: Config.ready
        onTriggered: {
            print("[Updates] Periodic update check due")
            root.refresh();
        }
    }

    Timer {
        id: availabilityDefer
        interval: 1500
        repeat: false
        onTriggered: {
            root._availabilityOutput = ""
            checkAvailabilityProc.running = true
        }
    }

    Connections {
        target: Config
        function onReadyChanged() {
            if (Config.ready) availabilityDefer.start()
        }
    }

    Component.onCompleted: if (Config.ready) availabilityDefer.start()

    Process {
        id: checkAvailabilityProc
        running: false
        command: ["/usr/bin/bash", "-c",
            "backend=$(ryoku-host pkgmgr 2>/dev/null) || exit 1; " +
            "case \"$backend\" in " +
                "pacman) command -v checkupdates &>/dev/null ;; " +
                "xbps) command -v xbps-install &>/dev/null ;; " +
                "dnf) command -v dnf5 &>/dev/null || command -v dnf &>/dev/null ;; " +
                "*) exit 1 ;; " +
            "esac && printf '%s\\n' \"$backend\""
        ]
        stdout: SplitParser {
            splitMarker: ""
            onRead: data => { root._availabilityOutput += data }
        }
        onExited: (exitCode, exitStatus) => {
            root._backend = exitCode === 0 ? root._availabilityOutput.trim() : ""
            root.available = root._backend.length > 0
            root.refresh();
        }
    }

    // The check's output and exit code arrive in either order; the count settles once both are in.
    property var _checkOutput: null
    property int _checkExit: -1
    function _settleCheck(): void {
        if (root._checkOutput === null || root._checkExit < 0) return
        const lines = String(root._checkOutput).trim()
        const listed = lines.length > 0 ? lines.split("\n").length : 0
        const code = root._checkExit
        root._checkOutput = null
        root._checkExit = -1
        // checkupdates: 0 lists updates, 2 means up to date, 1 could not check (offline, database locked, no
        // fakeroot). A check that could not run keeps the last count instead of announcing an up-to-date system.
        if (root._backend === "pacman") {
            if (code === 0 || code === 2) {
                root.count = code === 2 ? 0 : listed
            } else {
                const reason = (checkUpdatesErr.text ?? "").trim().split("\n").pop()
                console.info("[Updates] could not check for updates:", reason || `checkupdates exited ${code}`)
            }
            return
        }
        if (root._backend === "dnf") {
            if (code === 0) {
                root.count = listed
            } else {
                const reason = (checkUpdatesErr.text ?? "").trim().split("\n").pop()
                console.info("[Updates] could not check for DNF updates:", reason || `repoquery exited ${code}`)
            }
            return
        }
        root.count = listed
        if (code !== 0)
            console.error("[Updates] update check failed for", root._backend, code)
    }

    Process {
        id: checkUpdatesProc
        command: []
        stdout: StdioCollector {
            onStreamFinished: {
                root._checkOutput = text ?? ""
                root._settleCheck()
            }
        }
        stderr: StdioCollector {
            id: checkUpdatesErr
        }
        onExited: (exitCode, exitStatus) => {
            root._checkExit = exitCode
            root._settleCheck()
        }
    }
}
