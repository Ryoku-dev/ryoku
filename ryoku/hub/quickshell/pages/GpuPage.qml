pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons

// The Machine page. Real hardware, not the shared config store, so it is
// full-bleed and draws its own head. Backed by `ryoku-hub gpu` (render mode,
// per-session GPU tuning, passthrough) and `ryoku-hub cpu` (persistent CPU
// power-profile definitions + battery), each capability-gated so a knob appears
// only where the machine exposes it.
//
// GPU tuning is per session: runtime sysfs / nvidia-smi state, gone on reboot.
// CPU profiles persist to power.json and ryoku-power re-applies them after
// power-profiles-daemon switches; boost and PPT/TDP are omitted because the
// firmware governs them on this hardware.
Item {
    id: pg

    property var hub
    readonly property bool fullBleed: true
    readonly property bool previewDirty: false

    // passthrough dossier + render mode (unchanged backend)
    property var caps: ({})
    property string mode: "hybrid"
    property string planText: ""
    property bool planning: false
    property bool enabling: false
    property bool showChecks: false
    property string actionError: ""
    property string capsError: ""
    property string modeWarn: ""
    // hardware display-routing switch (the MUX): which GPU the built-in panel
    // is physically wired to. Empty when the machine has no knob.
    property var mux: ({})
    property string muxError: ""

    // tuning + presets
    property var tune: []
    property var presets: []
    property bool showAdvanced: false
    property bool namingPreset: false
    property string tuneError: ""

    // cpu power-profile programmer + battery, fronted by `ryoku-hub cpu` (which
    // wraps ryoku-power). Empty when the backend or machine offers nothing, so
    // the sections hide rather than render an empty frame.
    property var cpu: []
    property var cpuProfiles: []
    property string cpuActive: ""
    property string cpuProfile: ""
    property string cpuError: ""

    // live telemetry for the render GPU (self-contained poll). liveAsleep is not
    // a failure: a runtime-suspended discrete GPU is reported rather than probed,
    // because nvidia-smi would wake it out of D3 (about 10 W on a hybrid laptop)
    // and this page polls every 2s, so a page left open would pin the card awake
    // and hide the very saving it is reporting on.
    property int liveTemp: 0
    property int liveUtil: 0
    property bool liveOk: false
    property bool liveAsleep: false

    readonly property var renderGpu: {
        var p = pg.caps.passthrough, h = pg.caps.host;
        if (p && p.drivesDisplay)
            return p;
        if (h && h.drivesDisplay)
            return h;
        return h || p || null;
    }
    readonly property string renderName: pg.renderGpu ? pg.renderGpu.model : I18n.tr("your GPU")
    readonly property string dgpuName: pg.caps.passthrough ? pg.caps.passthrough.model : I18n.tr("the discrete GPU")

    readonly property bool capsLoaded: pg.caps.verdict !== undefined
    readonly property bool ptPending: pg.capsError === "" && !pg.capsLoaded
    readonly property bool ptOk: pg.caps.verdict === "ready"

    readonly property var safeTune: (pg.tune || []).filter(t => t.risk === "safe")
    readonly property var advTune: (pg.tune || []).filter(t => t.risk === "advanced")
    readonly property var cpuTune: (pg.cpu || []).filter(t => t.gpu === "cpu")
    readonly property var batteryTune: (pg.cpu || []).filter(t => t.gpu === "battery")
    // idle policy, also fronted by `ryoku-hub cpu`: two toggles plus per-stage
    // minute steppers split into battery and AC. Neutral across compositors.
    readonly property var idleTune: (pg.cpu || []).filter(t => t.gpu === "idle")
    readonly property var idleGeneral: pg.idleTune.filter(t => t.id === "enabled" || t.id === "onDesktops")
    readonly property var idleBattery: pg.idleTune.filter(t => t.id.indexOf("battery.") === 0)
    readonly property var idleAc: pg.idleTune.filter(t => t.id.indexOf("ac.") === 0)
    readonly property bool idleOn: {
        var e = (pg.idleTune || []).find(t => t.id === "enabled");
        return e ? e.value === "on" : false;
    }
    readonly property string thermalNow: {
        var t = (pg.tune || []).find(x => x.id === "thermal");
        return t ? t.value : "";
    }
    readonly property string statusLine: {
        var s = I18n.tr("%1 renders here").arg(pg.renderName);
        if (pg.liveOk)
            s += "  ·  " + pg.liveTemp + "°C  ·  " + pg.liveUtil + I18n.tr("% GPU");
        else if (pg.liveAsleep)
            s += "  ·  " + I18n.tr("suspended, drawing no power");
        if (pg.thermalNow !== "")
            s += "  ·  " + I18n.tr(pg.thermalNow);
        return s;
    }

    readonly property string ptText: {
        switch (pg.caps.verdict) {
        case "ready": return I18n.tr("Ready. %1 is free for a VM to claim, and returns to the desktop when the VM stops.").arg(pg.dgpuName);
        case "needs-relogin": return I18n.tr("Set up. Log out and back in once, then it is ready.");
        case "needs-reboot": return I18n.tr("Your screen runs on %1. Switch to Hybrid GPU mode in the BIOS (look for GPU Mode, MUX, or Hybrid/Optimus) and reboot, so the built-in GPU drives the display and the discrete GPU is free.").arg(pg.dgpuName);
        case "needs-setup": return I18n.tr("Not set up yet. Review the changes, then enable it below.");
        case "incapable": return I18n.tr("This machine can't pass a GPU to a VM. Open the readiness checks below for why.");
        default: return pg.capsError !== "" ? I18n.tr("Couldn't read your graphics hardware.") : I18n.tr("Checking…");
        }
    }

    // short role tag for a gpu slot, so a tuning row reads "dGPU · Power limit".
    function tag(gpu) {
        if (gpu === "idle")
            return "";
        if (gpu === "cpu")
            return "CPU";
        if (gpu === "battery")
            return I18n.tr("Battery");
        if (gpu === "platform")
            return I18n.tr("Chassis");
        if (pg.caps.passthrough && gpu === pg.caps.passthrough.slot)
            return "dGPU";
        if (pg.caps.host && gpu === pg.caps.host.slot)
            return "iGPU";
        return "GPU";
    }

    function reload() {
        pg.capsError = "";
        capsProc.running = true;
        modeProc.running = true;
        tuneProc.running = true;
        presetProc.running = true;
        cpuActiveProc.running = true;
        muxProc.running = true;
    }
    function reloadTune() {
        tuneProc.running = true;
        presetProc.running = true;
    }
    function reloadCpu() {
        cpuCapsProc.command = ["ryoku-hub", "cpu", "caps", pg.cpuProfile];
        cpuCapsProc.running = true;
    }
    function editProfile(name) {
        pg.cpuProfile = name;
        pg.reloadCpu();
    }
    function cpuSet(scope, id, value) {
        pg.cpuError = "";
        cpuSetProc.command = ["ryoku-hub", "cpu", "set", scope, id, "" + value];
        cpuSetProc.running = true;
    }
    function act(cmd) {
        pg.actionError = "";
        runProc.command = cmd;
        runProc.running = true;
    }
    function setMode(m) {
        pg.modeWarn = "";
        modeSetProc.command = ["ryoku-hub", "gpu", "mode", "set", m];
        modeSetProc.running = true;
    }
    function setMux(m) {
        pg.muxError = "";
        muxSetProc.command = ["ryoku-hub", "gpu", "mux", "set", m];
        muxSetProc.running = true;
    }
    function cpuSwitch(name) {
        pg.cpuError = "";
        cpuSwitchProc.command = ["ryoku-hub", "cpu", "switch", name];
        cpuSwitchProc.running = true;
    }
    function tuneSet(gpu, id, value) {
        pg.tuneError = "";
        tuneSetProc.command = ["ryoku-hub", "gpu", "tune", "set", gpu, id, "" + value];
        tuneSetProc.running = true;
    }
    function tuneReset() {
        pg.tuneError = "";
        tuneSetProc.command = ["ryoku-hub", "gpu", "tune", "reset"];
        tuneSetProc.running = true;
    }
    function applyPreset(name) {
        pg.tuneError = "";
        tuneSetProc.command = ["ryoku-hub", "gpu", "tune", "preset", "apply", name];
        tuneSetProc.running = true;
    }
    function savePreset(name) {
        if (name.trim() === "")
            return;
        pg.namingPreset = false;
        presetSaveProc.command = ["ryoku-hub", "gpu", "tune", "preset", "save", name.trim()];
        presetSaveProc.running = true;
    }
    function deletePreset(name) {
        presetSaveProc.command = ["ryoku-hub", "gpu", "tune", "preset", "delete", name];
        presetSaveProc.running = true;
    }
    function reviewEnable() {
        planProc.command = ["ryoku-hub", "gpu", "apply", "enable", "--dry-run"];
        planProc.running = true;
    }
    function enableInTerminal() {
        Spawn.run(["kitty", "--class", "ryoku-gpu", "-e", "sh", "-c",
            "ryoku-hub gpu apply enable; echo; read -n1 -rsp 'Done. Press any key to close…'; echo"]);
        pg.planning = false;
        pg.planText = "";
        pg.enabling = true;
    }
    function recheck() {
        pg.enabling = false;
        pg.reload();
    }
    function modeLabel(m) { return m.length ? m.charAt(0).toUpperCase() + m.slice(1) : ""; }

    Component.onCompleted: pg.reload()

    // ── backends ─────────────────────────────────────────────────────────────
    Process {
        id: capsProc
        command: ["ryoku-hub", "gpu", "caps"]
        stdout: StdioCollector { id: capsOut }
        stderr: StdioCollector { id: capsErr }
        onExited: (code) => {
            if (code === 0) {
                try {
                    pg.caps = JSON.parse(capsOut.text);
                    pg.capsError = "";
                    return;
                } catch (e) {
                    console.log("gpu: caps parse failed: " + e);
                }
            }
            pg.capsError = capsErr.text.trim() || ("ryoku-hub gpu caps exited " + code);
        }
    }
    Process {
        id: modeProc
        command: ["ryoku-hub", "gpu", "mode", "get"]
        stdout: StdioCollector {
            onStreamFinished: {
                try { pg.mode = JSON.parse(this.text).mode; } catch (e) {}
            }
        }
    }
    Process {
        id: tuneProc
        command: ["ryoku-hub", "gpu", "tune", "caps"]
        stdout: StdioCollector {
            onStreamFinished: {
                try { pg.tune = JSON.parse(this.text) || []; } catch (e) { pg.tune = []; }
            }
        }
    }
    Process {
        id: presetProc
        command: ["ryoku-hub", "gpu", "tune", "preset", "list"]
        stdout: StdioCollector {
            onStreamFinished: {
                try { pg.presets = JSON.parse(this.text) || []; } catch (e) { pg.presets = []; }
            }
        }
    }
    Process {
        id: modeSetProc
        stdout: StdioCollector { onStreamFinished: pg.reload() }
        stderr: StdioCollector {
            onStreamFinished: {
                var e = this.text.trim();
                if (e.length > 0) pg.modeWarn = e;
            }
        }
    }
    Process {
        id: muxProc
        command: ["ryoku-hub", "gpu", "mux", "get"]
        stdout: StdioCollector {
            onStreamFinished: {
                try { pg.mux = JSON.parse(this.text) || {}; } catch (e) { pg.mux = {}; }
            }
        }
    }
    Process {
        id: muxSetProc
        stdout: StdioCollector {
            onStreamFinished: {
                try { pg.mux = JSON.parse(this.text) || pg.mux; } catch (e) {}
            }
        }
        stderr: StdioCollector {
            onStreamFinished: {
                var e = this.text.trim();
                if (e.length > 0) pg.muxError = e;
            }
        }
    }
    Process {
        id: tuneSetProc
        stdout: StdioCollector { onStreamFinished: pg.reloadTune() }
        stderr: StdioCollector {
            onStreamFinished: {
                var e = this.text.trim();
                if (e.length > 0) pg.tuneError = e;
            }
        }
    }
    Process {
        id: presetSaveProc
        stdout: StdioCollector { onStreamFinished: pg.reloadTune() }
        stderr: StdioCollector {
            onStreamFinished: {
                var e = this.text.trim();
                if (e.length > 0) pg.tuneError = e;
            }
        }
    }
    Process {
        id: cpuActiveProc
        command: ["ryoku-hub", "cpu", "active"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var j = JSON.parse(this.text);
                    pg.cpuActive = j.activeProfile || "";
                    pg.cpuProfiles = j.profiles || [];
                    if (pg.cpuProfile === "")
                        pg.cpuProfile = pg.cpuActive || (pg.cpuProfiles[0] || "");
                    pg.reloadCpu();
                } catch (e) {
                    pg.cpu = [];
                }
            }
        }
    }
    Process {
        id: cpuCapsProc
        stdout: StdioCollector {
            onStreamFinished: {
                try { pg.cpu = JSON.parse(this.text) || []; } catch (e) { pg.cpu = []; }
            }
        }
    }
    Process {
        id: cpuSetProc
        stdout: StdioCollector { onStreamFinished: pg.reloadCpu() }
        stderr: StdioCollector {
            onStreamFinished: {
                var e = this.text.trim();
                if (e.length > 0) pg.cpuError = e;
            }
        }
    }
    Process {
        id: cpuSwitchProc
        stdout: StdioCollector { onStreamFinished: cpuActiveProc.running = true }
        stderr: StdioCollector {
            onStreamFinished: {
                var e = this.text.trim();
                if (e.length > 0) pg.cpuError = e;
            }
        }
    }
    Process {
        id: runProc
        stdout: StdioCollector { onStreamFinished: pg.reload() }
        stderr: StdioCollector {
            onStreamFinished: {
                var e = this.text.trim();
                if (e.length > 0) pg.actionError = e;
            }
        }
    }
    Process {
        id: planProc
        stdout: StdioCollector {
            onStreamFinished: { pg.planText = this.text; pg.planning = true; }
        }
    }
    // live temperature + utilisation for the render GPU: nvidia-smi first, then
    // the first amdgpu card's sysfs, so it degrades on any hardware.
    Process {
        id: liveProc
        command: ["bash", "-c", `
for st in /sys/bus/pci/drivers/nvidia/*/power/runtime_status; do
  [ -r "$st" ] || continue
  IFS= read -r s < "$st"
  [ "$s" = suspended ] && { echo asleep; exit 0; }
  break
done
g=$(nvidia-smi --query-gpu=temperature.gpu,utilization.gpu --format=csv,noheader,nounits 2>/dev/null | head -1)
if [ -n "$g" ]; then echo "$g" | tr ',' ' '; exit 0; fi
for d in /sys/class/drm/card*/device; do
  [ -r "$d/gpu_busy_percent" ] || continue
  u=$(cat "$d/gpu_busy_percent" 2>/dev/null)
  t=$(cat "$d"/hwmon/hwmon*/temp1_input 2>/dev/null | head -1)
  echo "$((t/1000)) $u"; exit 0
done
`]
        stdout: StdioCollector {
            onStreamFinished: {
                var raw = this.text.trim();
                if (raw === "asleep") {
                    pg.liveAsleep = true;
                    pg.liveOk = false;
                    return;
                }
                var p = raw.split(/\s+/);
                if (p.length >= 2) {
                    pg.liveAsleep = false;
                    pg.liveTemp = parseInt(p[0]) || 0;
                    pg.liveUtil = parseInt(p[1]) || 0;
                    pg.liveOk = true;
                }
            }
        }
    }
    Timer {
        interval: 2000; repeat: true; running: pg.visible
        triggeredOnStart: true
        onTriggered: liveProc.running = true
    }
    Timer {
        interval: 4000; repeat: true; running: pg.enabling
        onTriggered: capsProc.running = true
    }

    // ── one tuning knob as a SettingRow, control chosen from its kind ──────────
    component TuneCell: SettingRow {
        id: tc
        property var tunable: ({})
        property string scope: ""      // non-empty routes writes through `ryoku-hub cpu set <scope>`
        readonly property string knd: tc.tunable.kind || ""
        readonly property int optCount: (tc.tunable.options || []).length

        // a cpu/battery row carries a scope and persists via `cpu set`; a gpu
        // row has none and writes live via `gpu tune`.
        function apply(v) {
            if (tc.scope !== "")
                pg.cpuSet(tc.scope, tc.tunable.id, v);
            else
                pg.tuneSet(tc.tunable.gpu, tc.tunable.id, v);
        }

        anchors.left: parent.left
        anchors.right: parent.right
        // A two-option segment normally sits inline, but long option words
        // ("performance"/"powersave") overflow that width and wrap into the next
        // row, so give any long-labelled segment its own band.
        readonly property bool wideOpts: (tc.tunable.options || []).some(o => String(o).length > 8)
        block: tc.knd === "segment" && (tc.optCount >= 3 || tc.wideOpts)
        controlWidth: tc.knd === "toggle" ? 54
            : tc.knd === "stepper" ? 58
            : (tc.knd === "slider" ? Math.min(240, Math.max(160, Math.round(tc.width * 0.34)))
            : Math.max(120, 62 * Math.max(2, tc.optCount)))
        label: (pg.tag(tc.tunable.gpu) !== "" ? pg.tag(tc.tunable.gpu) + " · " : "") + I18n.tr(tc.tunable.label || "")
        unit: tc.tunable.unit || ""
        value: (tc.knd === "slider" || tc.knd === "stepper") ? String(Math.round(tc.tunable.current || 0)) : ""
        desc: (tc.tunable.desc && tc.tunable.desc !== "") ? I18n.tr(tc.tunable.desc)
            : (tc.tunable.risk === "advanced" ? I18n.tr("Advanced · per session, can misbehave") : I18n.tr("Applies now, resets on reboot"))
        source: tc.tunable.src || ""
        changed: false

        Loader {
            anchors.fill: parent
            sourceComponent: tc.knd === "toggle" ? swC : tc.knd === "stepper" ? stepC : (tc.knd === "slider" ? slidC : segC)
        }
        Component {
            id: swC
            Sw {
                anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                on: tc.tunable.value === "on"
                onToggled: (v) => tc.apply(v ? "on" : "off")
            }
        }
        Component {
            id: slidC
            Slid {
                anchors.fill: parent
                from: tc.tunable.min || 0
                to: tc.tunable.max || 1
                value: tc.tunable.current || 0
                onModified: (v) => tc.apply(String(Math.round(v)))
            }
        }
        Component {
            id: segC
            Seg {
                anchors.left: parent.left; anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                options: tc.tunable.options || []
                current: tc.tunable.value
                onChose: (k) => tc.apply(k)
            }
        }
        Component {
            id: stepC
            Step {
                anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                from: tc.tunable.min || 0
                to: tc.tunable.max || 60
                stepBy: tc.tunable.stepBy || 1
                value: tc.tunable.current || 0
                onModified: (v) => tc.apply(String(Math.round(v)))
            }
        }
    }

    // ── head ───────────────────────────────────────────────────────────────────
    Column {
        id: head
        anchors { left: parent.left; right: parent.right; top: parent.top }
        anchors.leftMargin: Tokens.s6
        anchors.rightMargin: Tokens.s6
        anchors.topMargin: Tokens.s6
        // the register row sits off the title: a rule over a 32px
        // title needs more than the gap between two lines of body text
        spacing: Tokens.s3

        Row {
            // the register row holds a fixed box, so the rule and the seal keep
            // their distance from the title on every page
            height: Tokens.s5
            spacing: Tokens.s2
            Rectangle { width: 16; height: 1; color: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
            Text {
                text: "力"; color: Tokens.ink; font.family: Tokens.jp
                font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: I18n.tr("DEVICES"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Text {
            text: I18n.tr("Machine"); color: Tokens.ink
            font.family: Tokens.display; font.pixelSize: Tokens.fTitle
        }
        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("Power profiles, graphics tuning, and which GPU renders the desktop.")
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
        }
        // live status line: one glance at what is happening right now.
        Text {
            text: pg.statusLine
            color: Tokens.inkDim; font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall; font.weight: Font.Medium
        }
    }

    // ── content: one full-width scrolling column above the render hero ─────────
    Item {
        id: below
        anchors {
            left: parent.left; right: parent.right; top: head.bottom; bottom: parent.bottom
            leftMargin: Tokens.s6; rightMargin: Tokens.s6; topMargin: Tokens.s5; bottomMargin: Tokens.s6
        }

        Flickable {
            id: gfx
            anchors {
                left: parent.left; right: parent.right; top: parent.top; bottom: parent.bottom
                bottomMargin: Tokens.s5
            }
            contentWidth: width
            contentHeight: Math.max(gfxCol.height, height)
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
            WheelScroll { }

            CardColumns {
                id: gfxCol
                // a body of cards fills the measure and splits into balanced columns
                width: gfx.width - Tokens.s3
                spacing: Tokens.s5
                fillTo: gfx.height

                // gpu caps failed: surface it up top; the sections below still
                // render from whatever partial payload arrived.
                Column {
                    visible: pg.capsError !== ""
                    width: gfxCol.colWidth; spacing: Tokens.s3
                    Text {
                        width: parent.width; wrapMode: Text.WordWrap
                        text: I18n.tr("Couldn't read your graphics hardware.")
                        color: Tokens.ink; font.family: Tokens.ui
                        font.pixelSize: Tokens.fBody; font.weight: Font.DemiBold
                    }
                    Text {
                        width: parent.width; wrapMode: Text.WordWrap
                        text: pg.capsError
                        color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro
                    }
                    Btn { text: I18n.tr("Retry"); primary: true; onAct: pg.reload() }
                }

                // ── RYOKU RENDERS ON ──
                SettingCard {
                    width: gfxCol.colWidth
                    title: I18n.tr("RYOKU RENDERS ON")
                    // Hybrid/Performance/Passthrough only mean something with a
                    // second GPU to switch between; a single-GPU box always renders
                    // on its one GPU, so hide the switcher rather than lock it.
                    visible: !!(pg.caps && pg.caps.host)

                    SettingRow {
                        anchors.left: parent.left; anchors.right: parent.right
                        block: true
                        label: I18n.tr("Graphics mode")
                        desc: I18n.tr("Software choice: which GPU renders the desktop. Takes effect on your next login.")
                        Seg {
                            anchors.left: parent.left; anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            options: ["Hybrid", "Performance", "Passthrough"]
                            current: pg.modeLabel(pg.mode)
                            onChose: (label) => pg.setMode(label.toLowerCase())
                        }
                    }

                    // what the chosen mode means, spelled out beneath the control.
                    Text {
                        width: parent.width
                        leftPadding: Tokens.s4; rightPadding: Tokens.s4
                        topPadding: Tokens.s2; bottomPadding: Tokens.s3
                        wrapMode: Text.WordWrap
                        text: pg.mode === "hybrid"
                            ? I18n.tr("Hybrid keeps the built-in GPU primary for battery; apps can still use %1 on demand.").arg(pg.dgpuName)
                            : (pg.mode === "performance"
                                ? (pg.caps.chassis === "laptop" && pg.mux.capable
                                    ? I18n.tr("Performance pins %1 as primary, so video decode and GPU work leave the CPU's heat budget. This laptop has a display-routing switch: set it to Discrete below and reboot to move the screen to %1, for the full effect.").arg(pg.dgpuName)
                                    : I18n.tr("Performance pins %1 as primary: fastest, more power draw.").arg(pg.dgpuName))
                                : I18n.tr("Passthrough runs the desktop on the built-in GPU so %1 is free for a VM.").arg(pg.dgpuName))
                        color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                    }

                    // The hardware display-routing switch, where the firmware has
                    // one: which GPU the built-in panel is physically wired to.
                    SettingRow {
                        visible: !!(pg.mux.capable)
                        anchors.left: parent.left; anchors.right: parent.right
                        block: true
                        label: I18n.tr("Display wired to")
                        desc: I18n.tr("Hardware (GPU Mode / MUX / Optimus): which GPU drives the screen. Reboot to apply.")
                        Seg {
                            anchors.left: parent.left; anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            options: ["Hybrid", "Discrete"]
                            current: pg.modeLabel(pg.mux.mode || "")
                            onChose: (label) => pg.setMux(label.toLowerCase())
                        }
                    }
                    Text {
                        visible: !!(pg.mux.capable && pg.mux.reboot_pending)
                        width: parent.width
                        leftPadding: Tokens.s4; rightPadding: Tokens.s4
                        topPadding: Tokens.s2; bottomPadding: Tokens.s3
                        wrapMode: Text.WordWrap
                        text: I18n.tr("Reboot to move the display.");
                        color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                    }
                    Text {
                        visible: pg.muxError !== ""
                        width: parent.width
                        leftPadding: Tokens.s4; rightPadding: Tokens.s4
                        topPadding: Tokens.s2; bottomPadding: Tokens.s3
                        wrapMode: Text.WordWrap
                        text: pg.muxError
                        color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro
                    }
                    // mode-set warning: a bordered plate, only when the backend complains.
                    Item {
                        visible: pg.modeWarn !== ""
                        width: parent.width
                        height: visible ? modeWarnPlate.height + Tokens.s3 : 0
                        Rectangle {
                            id: modeWarnPlate
                            anchors { left: parent.left; right: parent.right; top: parent.top }
                            anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4
                            height: modeWarnText.implicitHeight + Tokens.s3 * 2
                            radius: Tokens.radius; color: "transparent"
                            border.width: Tokens.border; border.color: Tokens.lineStrong
                            Text {
                                id: modeWarnText
                                anchors {
                                    left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter
                                    leftMargin: Tokens.s3; rightMargin: Tokens.s3
                                }
                                text: pg.modeWarn
                                color: Tokens.ink; wrapMode: Text.WordWrap
                                font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                            }
                        }
                    }
                }

                // ── CPU POWER PROFILES ──
                SettingCard {
                    width: gfxCol.colWidth
                    visible: pg.cpuTune.length > 0
                    title: I18n.tr("CPU POWER PROFILES")

                    // switch which profile is LIVE, through the shell daemon (the
                    // one owner of the pick: it banks it across reboots and keeps
                    // game mode's stash intact).
                    SettingRow {
                        visible: pg.cpuProfiles.length > 0
                        anchors.left: parent.left; anchors.right: parent.right
                        block: true
                        label: I18n.tr("Live profile")
                        desc: I18n.tr("Changes the active profile now.")
                        Seg {
                            anchors.left: parent.left; anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            options: pg.cpuProfiles
                            current: pg.cpuActive
                            onChose: (k) => pg.cpuSwitch(k)
                        }
                    }

                    // pick which definition to edit; this never switches the live
                    // profile, so the note below names the one that is active.
                    SettingRow {
                        anchors.left: parent.left; anchors.right: parent.right
                        block: true
                        label: I18n.tr("Editing profile")
                        Seg {
                            anchors.left: parent.left; anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            options: pg.cpuProfiles.length ? pg.cpuProfiles : ["power-saver", "balanced", "performance"]
                            current: pg.cpuProfile
                            onChose: (k) => pg.editProfile(k)
                        }
                    }
                    Text {
                        width: parent.width
                        leftPadding: Tokens.s4; rightPadding: Tokens.s4
                        topPadding: Tokens.s2; bottomPadding: Tokens.s3
                        wrapMode: Text.WordWrap
                        text: pg.cpuActive !== ""
                            ? I18n.tr("This edits what the profile does, not which one is live. %1 is active now.").arg(pg.cpuActive)
                            : I18n.tr("This edits what the profile does, not which one is live.")
                        color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                    }

                    // one row per capability-gated knob for the edited profile.
                    Repeater {
                        model: pg.cpuTune
                        delegate: TuneCell {
                            required property var modelData
                            tunable: modelData
                            scope: pg.cpuProfile
                            divider: true
                        }
                    }

                    Text {
                        visible: pg.cpuError !== ""
                        width: parent.width
                        leftPadding: Tokens.s4; rightPadding: Tokens.s4
                        topPadding: Tokens.s1; bottomPadding: Tokens.s2
                        wrapMode: Text.WordWrap
                        text: pg.cpuError
                        color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                        font.weight: Font.Medium
                    }

                    // closing honesty: boost and PPT/TDP are firmware-governed
                    // here, so a control would report success and change nothing.
                    Text {
                        width: parent.width
                        leftPadding: Tokens.s4; rightPadding: Tokens.s4
                        topPadding: Tokens.s1; bottomPadding: Tokens.s3
                        wrapMode: Text.WordWrap
                        text: I18n.tr("CPU boost and PPT/TDP limits are left out on purpose. On this hardware the firmware governs them, so a slider would report success and change nothing (boost measured 95.1 vs 95.0 °C; PPT writes are accepted then ignored). See docs/power.md.")
                        color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: Tokens.fTiny
                    }
                }

                // ── TUNING · THIS SESSION ──
                SettingCard {
                    width: gfxCol.colWidth
                    title: I18n.tr("TUNING · THIS SESSION")

                    // the per-session promise, said plainly and kept in view.
                    Text {
                        width: parent.width
                        leftPadding: Tokens.s4; rightPadding: Tokens.s4
                        topPadding: Tokens.s3; bottomPadding: Tokens.s1
                        wrapMode: Text.WordWrap
                        text: I18n.tr("Tuning is live and per session. Everything resets on reboot; there is nothing to save and nothing to undo but a reboot.")
                        color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                    }

                    // presets: apply a bundle, or save the current knobs as your own.
                    Column {
                        anchors.left: parent.left; anchors.right: parent.right
                        anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4
                        topPadding: Tokens.s2; bottomPadding: Tokens.s2
                        spacing: Tokens.s2
                        visible: (pg.tune || []).length > 0
                        Text {
                            text: I18n.tr("PRESETS")
                            color: Tokens.inkMuted; font.family: Tokens.ui
                            font.pixelSize: Tokens.fTiny; font.weight: Font.Medium
                            font.letterSpacing: Tokens.trackLabel
                        }
                        Flow {
                            width: parent.width; spacing: Tokens.s2
                            Repeater {
                                model: pg.presets
                                delegate: Row {
                                    id: prow
                                    required property var modelData
                                    spacing: 0
                                    Btn {
                                        text: prow.modelData.name
                                        onAct: pg.applyPreset(prow.modelData.name)
                                    }
                                    Btn {
                                        visible: prow.modelData.builtin !== true
                                        text: "×"; compact: true
                                        onAct: pg.deletePreset(prow.modelData.name)
                                    }
                                }
                            }
                            Btn {
                                text: I18n.tr("Save current…")
                                onAct: pg.namingPreset = true
                            }
                            Btn { text: I18n.tr("Reset all"); onAct: pg.tuneReset() }
                        }
                        // inline name entry for a new custom preset.
                        Rectangle {
                            visible: pg.namingPreset
                            width: parent.width; height: 32
                            radius: Tokens.radius; color: "transparent"
                            border.width: nameIn.activeFocus ? 2 : Tokens.border
                            border.color: nameIn.activeFocus ? Tokens.ink : Tokens.line
                            TextInput {
                                id: nameIn
                                anchors.fill: parent
                                anchors.leftMargin: Tokens.s3; anchors.rightMargin: 90
                                verticalAlignment: Text.AlignVCenter
                                color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: 13
                                clip: true; selectByMouse: true
                                onAccepted: pg.savePreset(text)
                                Text {
                                    anchors.verticalCenter: parent.verticalCenter
                                    visible: nameIn.text === ""
                                    text: I18n.tr("Name this preset…")
                                    color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: 13
                                }
                            }
                            Btn {
                                anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                anchors.rightMargin: Tokens.s2
                                text: I18n.tr("Save"); primary: true; compact: true
                                onAct: pg.savePreset(nameIn.text)
                            }
                        }
                    }

                    // safe knobs, always visible; one row per probed tunable.
                    Repeater {
                        model: pg.safeTune
                        delegate: TuneCell {
                            required property var modelData
                            tunable: modelData
                            divider: true
                        }
                    }

                    // nothing writable on this hardware: say so, do not leave a void.
                    Text {
                        visible: (pg.tune || []).length === 0
                        width: parent.width
                        leftPadding: Tokens.s4; rightPadding: Tokens.s4
                        topPadding: Tokens.s3; bottomPadding: Tokens.s3
                        wrapMode: Text.WordWrap
                        text: I18n.tr("Your graphics driver exposes no tunable knobs on this session. Everything here is read-only on this hardware.")
                        color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                    }

                    // advanced disclosure: overclock, undervolt, clock-lock, fan.
                    Item {
                        visible: pg.advTune.length > 0
                        width: parent.width; height: visible ? 34 : 0
                        Row {
                            anchors.left: parent.left; anchors.leftMargin: Tokens.s4
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: Tokens.s2
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: ">"; rotation: pg.showAdvanced ? 90 : 0
                                color: advHov.hovered ? Tokens.ink : Tokens.inkMuted
                                font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                                Behavior on rotation { NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease } }
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: pg.showAdvanced ? I18n.tr("Hide advanced") : I18n.tr("Advanced · per session, can misbehave")
                                color: advHov.hovered ? Tokens.ink : Tokens.inkMuted
                                font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                                font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
                            }
                        }
                        HoverHandler { id: advHov; cursorShape: Qt.PointingHandCursor }
                        TapHandler { onTapped: pg.showAdvanced = !pg.showAdvanced }
                    }
                    // advanced knobs, folded behind the disclosure.
                    Column {
                        width: parent.width
                        visible: pg.showAdvanced && pg.advTune.length > 0
                        spacing: 0
                        // the warning is a bone plate and the words, never a colour.
                        Item {
                            width: parent.width
                            height: advWarnPlate.height + Tokens.s2 * 2
                            Rectangle {
                                id: advWarnPlate
                                anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter }
                                anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4
                                height: advWarn.implicitHeight + Tokens.s3 * 2
                                radius: Tokens.radius; color: Tokens.bone
                                Text {
                                    id: advWarn
                                    anchors {
                                        left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter
                                        leftMargin: Tokens.s3; rightMargin: Tokens.s3
                                    }
                                    text: I18n.tr("Overclocking, undervolting and fan control can freeze the GPU or the desktop. Every change is per session and clears on reboot; if the screen misbehaves, reboot to recover.")
                                    color: Tokens.inkOnBone; wrapMode: Text.WordWrap
                                    font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; font.weight: Font.Medium
                                }
                            }
                        }
                        Repeater {
                            model: pg.advTune
                            delegate: TuneCell {
                                required property var modelData
                                tunable: modelData
                                divider: true
                            }
                        }
                    }

                    Text {
                        visible: pg.tuneError !== ""
                        width: parent.width
                        leftPadding: Tokens.s4; rightPadding: Tokens.s4
                        topPadding: Tokens.s2; bottomPadding: Tokens.s2
                        wrapMode: Text.WordWrap
                        text: pg.tuneError
                        color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                        font.weight: Font.Medium
                    }

                    Text {
                        width: parent.width
                        leftPadding: Tokens.s4; rightPadding: Tokens.s4
                        topPadding: Tokens.s1; bottomPadding: Tokens.s3
                        wrapMode: Text.WordWrap
                        text: I18n.tr("Want deeper overclocking (voltage curves, fan curves)? Install LACT, a dedicated GPU control daemon.")
                        color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: Tokens.fTiny
                    }
                }

                // ── BATTERY ──
                SettingCard {
                    width: gfxCol.colWidth
                    visible: pg.batteryTune.length > 0
                    title: I18n.tr("BATTERY")

                    Repeater {
                        model: pg.batteryTune
                        delegate: TuneCell {
                            required property var modelData
                            tunable: modelData
                            scope: "battery"
                            divider: true
                        }
                    }
                }

                // ── IDLE ──
                SettingCard {
                    width: gfxCol.colWidth
                    visible: pg.idleGeneral.length > 0
                    title: I18n.tr("IDLE")

                    Repeater {
                        model: pg.idleGeneral
                        delegate: TuneCell {
                            required property var modelData
                            tunable: modelData
                            scope: "idle"
                            divider: true
                        }
                    }
                }

                // ── ON BATTERY ── (folds away when idle is off)
                SettingCard {
                    width: gfxCol.colWidth
                    visible: pg.idleOn && pg.idleBattery.length > 0
                    title: I18n.tr("ON BATTERY")

                    Repeater {
                        model: pg.idleBattery
                        delegate: TuneCell {
                            required property var modelData
                            tunable: modelData
                            scope: "idle"
                            divider: true
                        }
                    }
                }

                // ── PLUGGED IN ──
                SettingCard {
                    width: gfxCol.colWidth
                    visible: pg.idleOn && pg.idleAc.length > 0
                    title: I18n.tr("PLUGGED IN")

                    Repeater {
                        model: pg.idleAc
                        delegate: TuneCell {
                            required property var modelData
                            tunable: modelData
                            scope: "idle"
                            divider: true
                        }
                    }
                }

                // ── GPU PASSTHROUGH · ADVANCED ──
                SettingCard {
                    width: gfxCol.colWidth
                    title: I18n.tr("GPU PASSTHROUGH · ADVANCED")

                    Text {
                        width: parent.width
                        leftPadding: Tokens.s4; rightPadding: Tokens.s4
                        topPadding: Tokens.s3; bottomPadding: Tokens.s1
                        wrapMode: Text.WordWrap
                        text: I18n.tr("Free %1 from the desktop and bind it to vfio so a virtual machine can own it for near-native performance. This sets up the host only; you run the VM yourself (libvirt + Looking Glass). Everyday VMs in ryovm need none of this.").arg(pg.dgpuName)
                        color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                    }

                    // the passthrough surface: status, actions, plan, and readiness
                    // checks -- a bespoke console, kept intact and inset into the card.
                    Column {
                        anchors.left: parent.left; anchors.right: parent.right
                        anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4
                        topPadding: Tokens.s1; bottomPadding: Tokens.s4
                        spacing: Tokens.s3

                        Row {
                            width: parent.width; spacing: Tokens.s2
                            Rectangle {
                                width: 7; height: 7; radius: 3.5
                                anchors.top: parent.top; anchors.topMargin: 5
                                color: pg.ptPending ? Tokens.inkMuted : (pg.ptOk ? Tokens.ink : "transparent")
                                border.width: (!pg.ptPending && !pg.ptOk) ? Tokens.border : 0
                                border.color: Tokens.ink
                            }
                            Text {
                                width: parent.width - 7 - Tokens.s2
                                wrapMode: Text.WordWrap
                                text: pg.ptText
                                color: pg.ptPending ? Tokens.inkMuted : (pg.ptOk ? Tokens.inkDim : Tokens.ink)
                                font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; font.weight: Font.Medium
                            }
                        }

                        Btn {
                            visible: pg.caps.enabled === true
                            text: I18n.tr("Disable passthrough")
                            onAct: pg.act(["ryoku-hub", "gpu", "apply", "disable"])
                        }
                        Btn {
                            visible: pg.caps.enabled !== true && pg.caps.verdict !== "incapable" && !pg.planning && !pg.enabling
                            text: I18n.tr("Review changes")
                            onAct: pg.reviewEnable()
                        }

                        Rectangle {
                            visible: pg.planning
                            width: parent.width; height: 220
                            radius: Tokens.radius; color: Tokens.tint5
                            border.width: Tokens.border; border.color: Tokens.line
                            clip: true
                            Flickable {
                                id: planFlick
                                anchors.fill: parent; anchors.margins: Tokens.s3
                                contentWidth: width; contentHeight: planView.height
                                clip: true; boundsBehavior: Flickable.StopAtBounds
                                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                                WheelScroll { }
                                Text {
                                    id: planView
                                    width: planFlick.width
                                    text: pg.planText
                                    color: Tokens.inkDim; font.family: Tokens.mono
                                    font.pixelSize: Tokens.fMicro; wrapMode: Text.WrapAnywhere
                                }
                            }
                        }
                        Row {
                            visible: pg.planning
                            spacing: Tokens.s2
                            Btn { text: I18n.tr("Enable passthrough"); primary: true; onAct: pg.enableInTerminal() }
                            Btn { text: I18n.tr("Close"); onAct: { pg.planning = false; pg.planText = ""; } }
                        }

                        Text {
                            visible: pg.enabling
                            width: parent.width; wrapMode: Text.WordWrap
                            text: I18n.tr("Setting up in a terminal window (it builds a kernel module, so it can take a few minutes). Click Recheck when it finishes.")
                            color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                        }
                        Btn { visible: pg.enabling; text: I18n.tr("Recheck"); onAct: pg.recheck() }

                        Item {
                            width: parent.width; height: 22
                            Row {
                                anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                spacing: Tokens.s2
                                Text {
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: ">"; rotation: pg.showChecks ? 90 : 0
                                    color: chkHov.hovered ? Tokens.ink : Tokens.inkMuted
                                    font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                                    Behavior on rotation { NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease } }
                                }
                                Text {
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: pg.showChecks ? I18n.tr("Hide readiness checks") : I18n.tr("Readiness checks")
                                    color: chkHov.hovered ? Tokens.ink : Tokens.inkMuted
                                    font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                                    font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
                                }
                            }
                            HoverHandler { id: chkHov; cursorShape: Qt.PointingHandCursor }
                            TapHandler { onTapped: pg.showChecks = !pg.showChecks }
                        }
                        Item {
                            width: parent.width; clip: true
                            height: pg.showChecks ? checksCol.implicitHeight : 0
                            visible: height > 0.5
                            opacity: pg.showChecks ? 1 : 0
                            Behavior on height { NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease } }
                            Behavior on opacity { NumberAnimation { duration: Tokens.snap } }
                            Column {
                                id: checksCol
                                width: parent.width; spacing: 0
                                Repeater {
                                    model: pg.caps.checks || []
                                    delegate: Item {
                                        id: cr
                                        required property var modelData
                                        readonly property string lvl: cr.modelData ? cr.modelData.level : ""
                                        readonly property bool attn: cr.lvl === "warn" || cr.lvl === "bad" || cr.lvl === "fail"
                                        width: parent.width; height: 30
                                        Rectangle {
                                            id: crdot
                                            width: 7; height: 7; radius: 3.5
                                            anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                                            color: cr.attn ? "transparent" : Tokens.ink
                                            border.width: cr.attn ? Tokens.border : 0
                                            border.color: Tokens.ink
                                        }
                                        Text {
                                            anchors.left: crdot.right; anchors.leftMargin: Tokens.s3
                                            anchors.verticalCenter: parent.verticalCenter
                                            text: cr.modelData ? I18n.tr(cr.modelData.label) : ""
                                            color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                        }
                                        Text {
                                            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                            text: cr.modelData ? cr.modelData.value : ""
                                            color: cr.attn ? Tokens.ink : Tokens.inkMuted
                                            font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }

        // action error banner (passthrough)
        Rectangle {
            anchors.left: parent.left; anchors.right: parent.right; anchors.bottom: parent.bottom
            visible: pg.actionError !== ""
            height: Math.min(errText.implicitHeight + Tokens.s3 * 2, 110)
            radius: Tokens.radius; color: Tokens.paper
            border.width: Tokens.border; border.color: Tokens.lineStrong
            clip: true
            Rectangle {
                id: errTag
                anchors.left: parent.left; anchors.top: parent.top
                anchors.leftMargin: Tokens.s3; anchors.topMargin: Tokens.s3
                width: errTagLab.width + Tokens.s2 * 2; height: 18
                radius: Tokens.radius; color: Tokens.bone
                Text {
                    id: errTagLab
                    anchors.centerIn: parent; text: I18n.tr("ERROR")
                    color: Tokens.inkOnBone; font.family: Tokens.ui; font.pixelSize: Tokens.fTiny
                    font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
                }
            }
            Text {
                id: errText
                anchors {
                    left: errTag.right; right: parent.right; top: parent.top
                    leftMargin: Tokens.s3; rightMargin: Tokens.s3; topMargin: Tokens.s3
                }
                text: pg.actionError
                color: Tokens.ink; wrapMode: Text.WordWrap
                elide: Text.ElideRight; maximumLineCount: 4
                font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
            }
            TapHandler { onTapped: pg.actionError = "" }
        }
    }
}
