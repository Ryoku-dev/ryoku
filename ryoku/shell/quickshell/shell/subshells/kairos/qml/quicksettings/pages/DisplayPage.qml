pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import shell.services
import Ryoku.Ui.Singletons
import "../../kit"

// scale and resolution go through ryoku-monitor, which only the hyprland provider (outputMirror) has
Column {
    id: page

    property var host

    spacing: 8

    readonly property bool engine: Wm.caps.outputMirror === true
    // the screens stand in until ryoku-monitor answers, and stay if it never does
    property bool fromEngine: false
    property var mons: []
    property bool loading: true
    property bool failed: false
    readonly property var ddcPct: page.host ? page.host.ddcBrightness : ({})
    property string expanded: ""
    property var ddcQueue: []
    property string ddcBus: ""
    property bool initialized: false
    readonly property int ddcCacheMs: 10000

    readonly property var scaleTargets: [1, 1.25, 1.5, 1.75, 2]
    readonly property int coolK: 6500
    readonly property int warmK: 2500

    function monitorCmd(args) {
        var script = 'if command -v ryoku-monitor >/dev/null 2>&1; then exec ryoku-monitor "$@"; '
            + 'else exec "$HOME/.local/bin/ryoku-monitor" "$@"; fi';
        return ["sh", "-c", script, "ryoku-monitor"].concat(args);
    }
    function isInternal(name) { return /eDP|LVDS/i.test(String(name || "")); }
    function ladderOf(m) {
        var l = m && m.scaleLadders ? m.scaleLadders[m.width + "x" + m.height] : null;
        return (l && l.length) ? l : [1];
    }
    function nearestScale(l, s) {
        var best = l[0];
        for (var i = 1; i < l.length; i++)
            if (Math.abs(l[i] - s) < Math.abs(best - s)) best = l[i];
        return best;
    }
    function chipsFor(m) {
        var l = page.ladderOf(m);
        var out = [];
        for (var i = 0; i < page.scaleTargets.length; i++) {
            var v = page.nearestScale(l, page.scaleTargets[i]);
            if (out.indexOf(v) < 0) out.push(v);
        }
        return out;
    }
    function fmtScale(v) { return Number(v).toFixed(2).replace(/0$/, "") + "x"; }
    function modeOf(m) { return m.width + "x" + m.height + "@" + Number(m.refresh).toFixed(2) + "Hz"; }
    function modeLabel(mode) {
        var r = /^(\d+)x(\d+)@([\d.]+)Hz$/.exec(String(mode || ""));
        if (!r) return String(mode || "");
        return I18n.tr("%1 at %2 Hz").replace("%1", r[1] + "\u00D7" + r[2]).replace("%2", Math.round(parseFloat(r[3])));
    }
    function describe(m) {
        if (page.fromEngine) return page.modeLabel(page.modeOf(m));
        if (m.model) return m.model;
        return page.isInternal(m.name) ? I18n.tr("Built-in display") : I18n.tr("External display");
    }
    function ddcBusFor(name) {
        var mons = Devices.ddcMonitors || [];
        for (var i = 0; i < mons.length; i++)
            if (mons[i] && mons[i].label === name) return mons[i].bus;
        return "";
    }
    function canDim(m) {
        return page.isInternal(m.name) ? Devices.backlightAvailable : page.ddcBusFor(m.name) !== "";
    }
    function cacheDdc(bus, pct) {
        if (!page.host || bus === "")
            return;
        var next = Object.assign({}, page.host.ddcBrightness || {});
        var updated = Object.assign({}, page.host.ddcBrightnessAt || {});
        next[bus] = pct;
        updated[bus] = Date.now();
        page.host.ddcBrightness = next;
        page.host.ddcBrightnessAt = updated;
    }
    function cacheMonitors(monitors) {
        if (!page.host)
            return;
        page.host.monitorSnapshot = monitors;
        page.host.monitorSnapshotValid = true;
    }
    function screenSnapshot() {
        var out = [];
        var list = Quickshell.screens;
        for (var i = 0; i < list.length; i++)
            out.push({ name: list[i].name, model: list[i].model || "" });
        return out;
    }
    function useScreens() {
        page.fromEngine = false;
        page.mons = page.screenSnapshot();
        page.queueDdcReads();
    }
    function readMonitors() {
        if (listProc.running) return;
        listProc.command = page.monitorCmd(["list"]);
        listProc.running = true;
    }
    function initialize() {
        if (page.initialized || !page.host)
            return;
        page.initialized = true;
        page.failed = false;
        if (!page.engine) {
            page.loading = false;
            page.useScreens();
            return;
        }
        if (page.host.monitorSnapshotValid) {
            page.fromEngine = true;
            page.mons = page.host.monitorSnapshot || [];
            page.loading = false;
            page.queueDdcReads();
            return;
        }
        page.loading = true;
        page.useScreens();
        page.readMonitors();
    }
    function brightOf(m) {
        if (page.isInternal(m.name))
            return Devices.backlightAvailable ? Devices.backlightPct / 100 : 0.5;
        var bus = page.ddcBusFor(m.name);
        if (bus !== "" && page.ddcPct[bus] !== undefined) return page.ddcPct[bus] / 100;
        return 0.5;
    }
    function setBright(m, v) {
        var pct = Math.max(1, Math.min(100, Math.round(v * 100)));
        if (page.isInternal(m.name)) {
            Devices.setBacklight(pct);
            return;
        }
        var bus = page.ddcBusFor(m.name);
        if (bus === "") return;
        page.cacheDdc(bus, pct);
        Devices.setBrightness(bus, pct);
    }
    function queueDdcReads() {
        var q = [];
        var now = Date.now();
        var updated = page.host ? (page.host.ddcBrightnessAt || {}) : {};
        for (var i = 0; i < page.mons.length; i++) {
            var m = page.mons[i];
            if (page.isInternal(m.name)) continue;
            var bus = page.ddcBusFor(m.name);
            var fresh = bus !== "" && page.ddcPct[bus] !== undefined
                && now - Number(updated[bus] || 0) < page.ddcCacheMs;
            if (bus !== "" && !fresh && q.indexOf(bus) < 0)
                q.push(bus);
        }
        page.ddcQueue = q;
        Qt.callLater(page.runDdc);
    }
    function runDdc() {
        if (ddcProc.running || page.ddcQueue.length === 0) return;
        page.ddcBus = page.ddcQueue[0];
        ddcProc.command = ["ddcutil", "getvcp", "10", "--bus", page.ddcBus, "--brief"];
        ddcProc.running = false;
        ddcProc.running = true;
    }
    function specsAll() {
        var out = [];
        for (var i = 0; i < page.mons.length; i++) {
            var m = page.mons[i];
            out.push({
                "id": m.id, "output": m.name, "mode": page.modeOf(m),
                "position": m.x + "x" + m.y, "scale": m.scale,
                "transform": m.transform || 0, "vrr": m.vrr,
                "mirror": (m.mirror && m.mirror !== "none") ? m.mirror : "",
                "disabled": m.disabled === true,
                "cm": m.cm || "srgb", "sdrbrightness": m.sdrbrightness || 1.0
            });
        }
        return out;
    }
    function apply() {
        if (page.host)
            page.host.monitorSnapshotValid = false;
        applyProc.command = page.monitorCmd(["apply", JSON.stringify(page.specsAll())]);
        applyProc.running = false;
        applyProc.running = true;
    }
    function setScale(name, v) {
        var next = page.mons.slice();
        for (var i = 0; i < next.length; i++)
            if (next[i].name === name) next[i] = Object.assign({}, next[i], { scale: v });
        page.mons = next;
        page.apply();
    }
    function setMode(name, mode) {
        var r = /^(\d+)x(\d+)@([\d.]+)Hz$/.exec(mode);
        if (!r) return;
        var next = page.mons.slice();
        for (var i = 0; i < next.length; i++) {
            if (next[i].name !== name) continue;
            var upd = Object.assign({}, next[i], {
                width: parseInt(r[1], 10), height: parseInt(r[2], 10), refresh: parseFloat(r[3])
            });
            upd.scale = page.nearestScale(page.ladderOf(upd), next[i].scale);
            next[i] = upd;
        }
        page.mons = next;
        page.apply();
        page.expanded = "";
    }
    function warmthOf(kelvin) {
        return Math.max(0, Math.min(1, (page.coolK - Number(kelvin || 4000)) / (page.coolK - page.warmK)));
    }
    function kelvinAt(warmth) {
        return Math.round((page.coolK - Math.max(0, Math.min(1, warmth)) * (page.coolK - page.warmK)) / 50) * 50;
    }

    Process {
        id: listProc
        command: []
        stdout: StdioCollector { id: listOut }
        onExited: {
            var parsed = null;
            try { parsed = JSON.parse(listOut.text); } catch (e) { parsed = null; }
            if (parsed instanceof Array) {
                page.fromEngine = true;
                page.mons = parsed;
                page.cacheMonitors(parsed);
                page.failed = false;
            } else {
                page.failed = true;
            }
            page.loading = false;
            page.queueDdcReads();
        }
    }
    Process {
        id: applyProc
        command: []
        onExited: refreshTimer.restart()
    }
    Process {
        id: ddcProc
        command: []
        stdout: StdioCollector { id: ddcOut }
        onExited: {
            var bus = page.ddcBus;
            var v = Devices.parseBrightness(ddcOut.text || "");
            if (bus !== "" && v >= 0)
                page.cacheDdc(bus, v);
            page.ddcQueue = page.ddcQueue.slice(1);
            Qt.callLater(page.runDdc);
        }
    }
    Timer {
        id: refreshTimer
        interval: 900
        onTriggered: page.readMonitors()
    }
    Connections {
        target: Devices
        function onDdcMonitorsChanged() { Qt.callLater(page.queueDdcReads); }
    }
    Connections {
        target: Quickshell
        enabled: !page.fromEngine
        function onScreensChanged() {
            page.mons = page.screenSnapshot();
            page.queueDdcReads();
        }
    }

    onHostChanged: page.initialize()
    onEngineChanged: {
        page.initialized = false;
        page.initialize();
    }
    Component.onCompleted: page.initialize()

    TopBar {
        width: parent.width
        title: I18n.tr("Display")
        onBack: page.host.back()
    }

    Column {
        width: parent.width
        visible: page.engine && (page.loading || page.failed)
        topPadding: 20
        bottomPadding: 12
        spacing: 12

        Label {
            width: parent.width
            horizontalAlignment: Text.AlignHCenter
            role: "body"
            color: Md.inkMuted
            wrapMode: Text.Wrap
            elide: Text.ElideNone
            text: page.failed ? I18n.tr("Could not read the display layout, so scale and resolution are missing. Try again in a moment.")
                : I18n.tr("Reading the display layout\u2026")
        }
        Button {
            anchors.horizontalCenter: parent.horizontalCenter
            visible: page.failed
            variant: "tonal"
            icon: "refresh"
            text: I18n.tr("Try again")
            onClicked: {
                page.loading = true;
                page.failed = false;
                page.readMonitors();
            }
        }
    }

    SectionLabel {
        visible: page.mons.length > 0
        text: page.mons.length > 1 ? I18n.tr("Displays") : page.fromEngine ? I18n.tr("Screen") : I18n.tr("Brightness")
    }

    Repeater {
        model: page.mons
        delegate: Column {
            id: mon
            required property var modelData
            readonly property bool dims: page.canDim(mon.modelData)
            readonly property bool open: page.expanded === mon.modelData.name
            readonly property var modes: page.fromEngine ? (mon.modelData.modes || []) : []
            width: parent.width
            spacing: Md.seam

            ListItem {
                width: parent.width
                position: mon.dims || page.fromEngine ? "first" : "only"
                clickable: false
                icon: page.isInternal(mon.modelData.name) ? "laptop" : "monitor"
                headline: mon.modelData.name
                supporting: mon.dims ? page.describe(mon.modelData)
                    : I18n.tr("Set its brightness with the buttons on the display")
                wrapSupporting: !mon.dims

                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    visible: page.fromEngine && mon.modelData.focused === true && page.mons.length > 1
                    role: "caption"
                    color: Md.accent
                    text: I18n.tr("Focused")
                }
            }

            Segment {
                visible: mon.dims
                position: page.fromEngine ? "middle" : "last"
                height: 72

                Slider {
                    id: bright
                    anchors {
                        left: parent.left; leftMargin: 14
                        right: brightText.left; rightMargin: 12
                        verticalCenter: parent.verticalCenter
                    }
                    icon: "light_mode"
                    value: page.brightOf(mon.modelData)
                    // ddc writes are slow, so an external display only takes where the drag lands
                    onMoved: v => { if (page.isInternal(mon.modelData.name)) page.setBright(mon.modelData, v); }
                    onCommitted: v => page.setBright(mon.modelData, v)
                }
                Label {
                    id: brightText
                    anchors { right: parent.right; rightMargin: 18; verticalCenter: parent.verticalCenter }
                    width: 40
                    horizontalAlignment: Text.AlignRight
                    role: "label"
                    font.family: Md.mono
                    color: Md.inkMuted
                    text: Math.round((bright.dragging ? bright.live : bright.value) * 100) + "%"
                }
            }

            Segment {
                visible: page.fromEngine
                position: "middle"
                height: scaleColumn.implicitHeight + 28

                Column {
                    id: scaleColumn
                    anchors { left: parent.left; right: parent.right; top: parent.top; margins: 14; leftMargin: 18 }
                    spacing: 10

                    Label {
                        role: "title"
                        font.pixelSize: 15
                        text: I18n.tr("Scale")
                    }
                    Flow {
                        width: parent.width
                        spacing: 8

                        Repeater {
                            model: page.fromEngine ? page.chipsFor(mon.modelData) : []
                            delegate: Chip {
                                required property var modelData
                                text: page.fmtScale(Number(modelData))
                                selected: Math.abs(Number(modelData) - mon.modelData.scale) < 0.005
                                onClicked: page.setScale(mon.modelData.name, Number(modelData))
                            }
                        }
                    }
                }
            }

            ListItem {
                width: parent.width
                visible: page.fromEngine
                position: mon.open ? "middle" : "last"
                headline: I18n.tr("Resolution")
                supporting: page.fromEngine ? page.modeLabel(page.modeOf(mon.modelData)) : ""
                onClicked: page.expanded = mon.open ? "" : mon.modelData.name

                Icon {
                    anchors.verticalCenter: parent.verticalCenter
                    name: mon.open ? "expand_less" : "expand_more"
                    size: 22
                    color: Md.inkMuted
                }
            }

            Repeater {
                model: mon.open ? mon.modes : []
                delegate: ListItem {
                    id: modeRow
                    required property var modelData
                    required property int index
                    readonly property bool current: String(modeRow.modelData) === page.modeOf(mon.modelData)
                    width: parent.width
                    position: modeRow.index === mon.modes.length - 1 ? "last" : "middle"
                    selected: modeRow.current
                    headline: page.modeLabel(String(modeRow.modelData))
                    onClicked: page.setMode(mon.modelData.name, String(modeRow.modelData))

                    Icon {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: modeRow.current
                        name: "check"
                        size: 20
                        color: modeRow.text
                    }
                }
            }
        }
    }

    Column {
        width: parent.width
        topPadding: 8
        spacing: Md.seam

        ListItem {
            width: parent.width
            position: "first"
            selected: Nightlight.on
            icon: "nightlight"
            headline: I18n.tr("Night light")
            supporting: Nightlight.on ? I18n.tr("On")
                : Nightlight.schedule === "sun" ? I18n.tr("Turns on at sunset")
                : Nightlight.schedule === "clock" ? I18n.tr("Turns on at the hours you set")
                : I18n.tr("Off")
            onClicked: Nightlight.setEnabled(!Nightlight.on, Nightlight.temperature)

            Switch {
                anchors.verticalCenter: parent.verticalCenter
                checked: Nightlight.on
                onToggled: v => Nightlight.setEnabled(v, Nightlight.temperature)
            }
        }

        Segment {
            position: "middle"
            height: 72

            Slider {
                id: warmth
                anchors {
                    left: parent.left; leftMargin: 14
                    right: warmthText.left; rightMargin: 12
                    verticalCenter: parent.verticalCenter
                }
                icon: "thermostat"
                step: 0.025
                value: page.warmthOf(Nightlight.temperature)
                // every daemon call reruns the backend
                onCommitted: v => Nightlight.setTemperature(page.kelvinAt(v))
            }
            Label {
                id: warmthText
                anchors { right: parent.right; rightMargin: 18; verticalCenter: parent.verticalCenter }
                width: 56
                horizontalAlignment: Text.AlignRight
                role: "label"
                font.family: Md.mono
                color: Md.inkMuted
                text: (warmth.dragging ? page.kelvinAt(warmth.live) : Nightlight.temperature) + " K"
            }
        }

        Segment {
            position: "last"
            height: scheduleColumn.implicitHeight + 28

            Column {
                id: scheduleColumn
                anchors { left: parent.left; right: parent.right; top: parent.top; margins: 14; leftMargin: 18 }
                spacing: 10

                Label {
                    role: "title"
                    font.pixelSize: 15
                    text: I18n.tr("Schedule")
                }
                Flow {
                    width: parent.width
                    spacing: 8

                    Chip {
                        text: I18n.tr("Manual")
                        selected: Nightlight.schedule === "off"
                        onClicked: Nightlight.setSchedule(false)
                    }
                    Chip {
                        icon: "wb_twilight"
                        text: I18n.tr("Sunset to sunrise")
                        selected: Nightlight.schedule === "sun"
                        onClicked: Nightlight.setSchedule(true)
                    }
                    // custom hours are set in the hub
                    Chip {
                        visible: Nightlight.schedule === "clock"
                        icon: "schedule"
                        text: I18n.tr("Custom hours")
                        selected: true
                    }
                }
            }
        }
    }

    Item { width: 1; height: 4 }
}
