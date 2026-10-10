pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

// SYSTEM info for the Profile section. base fields (operator, host, CPU/GPU,
// RAM, disk, ...) = the hub-side twin of the shell's SysInfo, read from the
// shared ryoku-sysinfo helper. extended fields for the dossier (load, temp,
// processes, swap, battery, network, packages, cursor, palette) come
// from ryoku-profile-stats. both = one field per line, re-polled every 30 s,
// each falls back to a placeholder when its source is absent.
Singleton {
    id: root

    readonly property string helper: "ryoku-sysinfo"
    readonly property string helperExtra: "ryoku-profile-stats"

    property string sysUser: "user"
    property string sysHost: "host"
    property string sysDistro: "Linux"
    property string sysArch: "x86_64"
    property string sysKernel: "-"
    // the session's own word for its compositor ("niri", "Hyprland"), set by the
    // session itself; a hardcoded name here lied on every non-Hyprland login.
    readonly property string sysWM: {
        const d = Quickshell.env("XDG_CURRENT_DESKTOP") || "";
        return d === "" ? "-" : d.charAt(0).toUpperCase() + d.slice(1);
    }
    property string sysShell: "sh"
    property string sysCpu: "-"
    property string sysCpuCores: "-"
    property string sysGpu: "-"
    property string sysGpu2: ""
    property string sysRam: "- / -"
    property int sysRamPct: 0
    property string sysDisk: "- / -"
    property int sysDiskPct: 0
    property string sysUptime: "-"
    property string sysPackages: "-"
    property string sysResolution: "-"
    property string sysRefresh: ""
    property string codename: "OPERATOR"

    property string sysPackageManager: ""
    // extended read-out for the dossier panel (ryoku-profile-stats).
    property string sysLoad: "-"
    property string sysTemp: "-"
    property string sysProcs: "-"
    property string sysSwap: "-"
    property string sysBattery: "-"
    property string sysWmVer: "-"
    property string sysMonitors: "-"
    property string sysPkgExplicit: "-"
    property string sysPkgAur: "-"
    property string sysCursor: "-"
    property string sysPalette: ""

    Process {
        id: poller
        running: true
        command: ["bash", root.helper]
        stdout: StdioCollector {
            onStreamFinished: {
                const l = text.split("\n");
                if (l.length < 17)
                    return;
                root.sysUser = (l[0] || "user").trim();
                root.sysHost = (l[1] || "host").trim();
                root.sysDistro = (l[2] || "Linux").trim();
                root.sysArch = (l[3] || "x86_64").trim();
                root.sysKernel = (l[4] || "-").trim();
                root.sysShell = (l[5] || "sh").trim();
                root.sysCpu = (l[6] || "-").trim();
                root.sysCpuCores = (l[7] || "-").trim();
                root.sysGpu = (l[8] || "-").trim();
                root.sysGpu2 = (l[9] || "").trim();
                root.sysRam = (l[10] || "- / -").trim();
                root.sysRamPct = parseInt(l[11]) || 0;
                root.sysDisk = (l[12] || "- / -").trim();
                root.sysDiskPct = parseInt(l[13]) || 0;
                root.sysUptime = (l[14] || "-").trim();
                root.sysPackages = (l[15] || "-").trim();
                root.sysResolution = (l[16] || "-").trim();
                root.sysRefresh = (l[17] || "").trim();
                const cn = (l[18] || "").trim();
                if (cn.length > 0)
                    root.codename = cn;
                root.sysPackageManager = (l[19] || "").trim();
            }
        }
    }

    Process {
        id: extra
        running: true
        command: ["bash", root.helperExtra]
        stdout: StdioCollector {
            onStreamFinished: {
                const l = text.split("\n");
                if (l.length < 11)
                    return;
                root.sysLoad = (l[0] || "-").trim();
                root.sysTemp = (l[1] || "-").trim();
                root.sysProcs = (l[2] || "-").trim();
                root.sysSwap = (l[3] || "-").trim();
                root.sysBattery = (l[4] || "-").trim();
                root.sysWmVer = (l[5] || "-").trim();
                root.sysMonitors = (l[6] || "-").trim();
                root.sysPkgExplicit = (l[7] || "-").trim();
                root.sysPkgAur = (l[8] || "-").trim();
                root.sysCursor = (l[9] || "-").trim();
                root.sysPalette = (l[10] || "").trim();
            }
        }
    }

    Timer {
        interval: 30000
        running: true
        repeat: true
        triggeredOnStart: true
        onTriggered: { poller.running = true; extra.running = true; }
    }
}
