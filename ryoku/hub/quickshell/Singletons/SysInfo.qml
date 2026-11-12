pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

// SYSTEM info for the Profile section. base fields (operator, host, CPU/GPU,
// RAM, disk, ...) = the hub-side twin of the shell's SysInfo, read from the
// shared ryoku-sysinfo helper. extended fields for the dossier (load, temp,
// processes, swap, battery, network, packages, cursor, palette) come
// from ryoku-profile-stats. both = one field per line, read once by load()
// (About asks); at start only what Home shows is read, from cheap sources.
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

    // the full read-out: both helpers count packages through the package manager, so they run
    // only when a page that shows them opens, and again on each visit
    property bool loading: false
    // a full read ended at least once; failed: the last one left a helper without an answer
    property bool loaded: false
    property bool failed: false
    property var waiting: ({})
    function load() {
        if (root.loading)
            return;
        root.loading = true;
        root.waiting = { poller: true, extra: true };
        root.failed = false;
        poller.running = true;
        extra.running = true;
        readLimit.restart();
    }
    // a helper whose answer landed owns its fields from then on
    property bool baseLanded: false
    property bool extraLanded: false
    function ended(name, ok) {
        if (ok && name === "poller")
            root.baseLanded = true;
        if (ok && name === "extra")
            root.extraLanded = true;
        if (!root.waiting[name])
            return;
        var w = Object.assign({}, root.waiting);
        delete w[name];
        root.waiting = w;
        if (!ok)
            root.failed = true;
        if (Object.keys(w).length === 0) {
            readLimit.stop();
            root.loading = false;
            root.loaded = true;
        }
    }
    // a helper that hangs ends the read as failed after 10 s
    Timer {
        id: readLimit
        interval: 10000
        onTriggered: {
            poller.running = false;
            extra.running = false;
            root.ended("poller", false);
            root.ended("extra", false);
        }
    }

    // Home's facts at start: distro, first graphics card, battery, active displays, the host and
    // user names, seconds up and memory, read the same way the helpers read them, without the
    // package manager
    property int upSeconds: 0
    property real memoryKb: 0
    Process {
        running: true
        command: ["sh", "-c", "grep -m1 'PRETTY_NAME' /etc/os-release 2>/dev/null | cut -d'\"' -f2; "
            + "lspci 2>/dev/null | grep -iE 'vga compatible|3d controller|display controller' | sed -n '1p' "
            + "| sed -E 's/^[^ ]+ [^:]*: //; s/ \\(rev [0-9a-f]+\\)//; s/NVIDIA Corporation //; s/Advanced Micro Devices, Inc\\. //; s/Intel Corporation //; s/\\[AMD\\/ATI\\] //; s/.*\\[([^]]+)\\].*/\\1/' | xargs "
            + "| awk '{ s = $0; if (length(s) > 30) { s = substr(s, 1, 31); if (s ~ /[ \\/]/) sub(/[ \\/][^ \\/]*$/, \"\", s); else s = substr(s, 1, 30); sub(/[ \\/,-]+$/, \"\", s) } print s }'; "
            + "bat=-; for b in /sys/class/power_supply/BAT*; do [ -e \"$b/capacity\" ] || continue; bat=\"$(cat \"$b/capacity\")%\"; break; done; echo \"$bat\"; "
            + "n=0; for e in /sys/class/drm/card*-*/enabled; do [ \"$(cat \"$e\" 2>/dev/null)\" = enabled ] && n=$((n + 1)); done; echo \"$n\"; "
            + "h=; read -r h < /proc/sys/kernel/hostname 2>/dev/null; echo \"$h\"; echo \"${USER:-$(id -un 2>/dev/null)}\"; "
            + "up=; read -r up _ < /proc/uptime 2>/dev/null; echo \"${up%%.*}\"; "
            + "m=0; while read -r k v _; do [ \"$k\" = MemTotal: ] && { m=$v; break; }; done < /proc/meminfo 2>/dev/null; echo \"$m\""]
        stdout: StdioCollector {
            onStreamFinished: {
                const l = text.split("\n");
                if (l.length < 4)
                    return;
                // a helper's full answer, when it already landed, stays
                if (!root.baseLanded) {
                    root.sysDistro = (l[0] || "Linux").trim();
                    root.sysGpu = (l[1] || "").trim() || "-";
                    if ((l[4] || "").trim().length > 0)
                        root.sysHost = l[4].trim();
                    if ((l[5] || "").trim().length > 0)
                        root.sysUser = l[5].trim();
                }
                if (!root.extraLanded) {
                    root.sysBattery = (l[2] || "-").trim();
                    root.sysMonitors = (l[3] || "-").trim();
                }
                root.upSeconds = parseInt(l[6]) || 0;
                root.memoryKb = parseInt(l[7]) || 0;
            }
        }
    }

    Process {
        id: poller
        command: ["bash", root.helper]
        stdout: StdioCollector {
            onStreamFinished: {
                const l = text.split("\n");
                if (l.length < 17) {
                    root.ended("poller", false);
                    return;
                }
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
                root.ended("poller", true);
            }
        }
    }

    Process {
        id: extra
        command: ["bash", root.helperExtra]
        stdout: StdioCollector {
            onStreamFinished: {
                const l = text.split("\n");
                if (l.length < 11) {
                    root.ended("extra", false);
                    return;
                }
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
                root.ended("extra", true);
            }
        }
    }
}
