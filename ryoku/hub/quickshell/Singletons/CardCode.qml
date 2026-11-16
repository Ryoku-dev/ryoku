pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../registry/rarity.js" as Rarity

// The rulers carry the owner's machine as well as their words and figure.
Singleton {
    id: root

    property var code: null
    property string distro: ""
    // os-release has been read: a code read before it says the distro is unknown
    property bool distroRead: false
    readonly property bool ready: root.distroRead && root.code !== null && !reader.running && !settle.running
    function num(re, s) { var m = re.exec(String(s || "")); return m ? parseInt(m[1]) : 0; }
    readonly property int threads: root.num(/(\d+)\s*T/, SysInfo.sysCpuCores) || root.num(/(\d+)/, SysInfo.sysCpuCores)
    readonly property int ramGb: root.num(/\/\s*(\d+)/, SysInfo.sysRam) || Math.round(SysInfo.memoryKb / 1048576)
    readonly property int refreshRate: root.num(/(\d+)/, SysInfo.sysRefresh)
    readonly property int displayWidth: root.num(/^(\d+)/, SysInfo.sysResolution)
    readonly property int graphicsClass: Math.max(Rarity.gpuClass(SysInfo.sysGpu), Rarity.gpuClass(SysInfo.sysGpu2))
    readonly property string gpu: String(Rarity.gpuClass(SysInfo.sysGpu2) > Rarity.gpuClass(SysInfo.sysGpu) ? SysInfo.sysGpu2 : SysInfo.sysGpu)
        .replace(/NVIDIA |GeForce |AMD |Radeon |Intel | Laptop GPU| Max-Q/gi, "").trim()
    readonly property var argv: ["ryoku-hub", "profile", "code", "--threads", String(root.threads), "--ram", String(root.ramGb),
        "--refresh", String(root.refreshRate), "--width", String(root.displayWidth), "--gpu-class", String(root.graphicsClass),
        "--gpu", root.gpu, "--distro", root.distro, "--wm", Settings.provider || ""]
    onArgvChanged: { root.refresh(); Badge.scheduleSync(); }

    function refresh() { settle.restart(); }
    Connections {
        target: ProfileStore
        function onDataChanged() { root.refresh(); }
    }
    Timer {
        id: settle
        interval: 400
        onTriggered: {
            if (reader.running)
                settle.restart();
            else
                reader.running = true;
        }
    }
    Process {
        running: true
        command: ["cat", "/etc/os-release"]
        stdout: StdioCollector {
            onStreamFinished: {
                var match = /^ID=(.*)$/m.exec(text);
                root.distro = match ? match[1].replace(/^["']|["']$/g, "").trim() : "";
                root.distroRead = true;
                root.refresh();
            }
        }
    }
    Process {
        id: reader
        command: root.argv
        stdout: StdioCollector {
            onStreamFinished: {
                try { root.code = JSON.parse(text); }
                catch (e) { root.code = null; }
            }
        }
        onExited: (exitCode, exitStatus) => { if (exitCode !== 0) root.code = null; }
    }
}
