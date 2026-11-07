pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import shell.services
import shell.barkit as Pill
import Ryoku.Ui.Singletons
import "../kit" as K

Column {
    id: page

    property var host

    spacing: 12

    // This page drives the displays through ryoku-monitor, the Hyprland
    // provider's engine; outputMirror is the capability only that provider
    // reports, so the surface hides itself where it would be dead rather than
    // offering controls that cannot apply.
    visible: Wm.caps.outputMirror === true

    readonly property color fill: Theme.shadow
    readonly property color ink: Theme.ink(fill, 7)
    readonly property color accent: Theme.primary
    function dim(a) { return Qt.rgba(page.ink.r, page.ink.g, page.ink.b, a) }

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
        return r[1] + "\u00D7" + r[2] + "  @  " + Math.round(parseFloat(r[3])) + " Hz";
    }
    function ddcBusFor(name) {
        var mons = Devices.ddcMonitors || [];
        for (var i = 0; i < mons.length; i++)
            if (mons[i] && mons[i].label === name) return mons[i].bus;
        return "";
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
    function initialize() {
        if (page.initialized || !page.host)
            return;
        page.initialized = true;
        if (page.host.monitorSnapshotValid) {
            page.mons = page.host.monitorSnapshot || [];
            page.loading = false;
            page.failed = false;
            page.queueDdcReads();
            return;
        }
        listProc.command = page.monitorCmd(["list"]);
        listProc.running = true;
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

    Process {
        id: listProc
        command: []
        stdout: StdioCollector { id: listOut }
        onExited: {
            var parsed = null;
            try { parsed = JSON.parse(listOut.text); } catch (e) { parsed = null; }
            if (parsed instanceof Array) {
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
        onTriggered: if (!listProc.running) {
            listProc.command = page.monitorCmd(["list"]);
            listProc.running = true;
        }
    }
    Connections {
        target: Devices
        function onDdcMonitorsChanged() { Qt.callLater(page.queueDdcReads); }
    }

    onHostChanged: page.initialize()
    Component.onCompleted: page.initialize()

    Item {
        width: parent.width
        height: 36

        K.QsBack {
            id: back
            anchors { left: parent.left; verticalCenter: parent.verticalCenter }
            onClicked: page.host.back()
        }
        Text {
            anchors { left: back.right; leftMargin: 12; verticalCenter: parent.verticalCenter }
            text: I18n.tr("Display")
            color: page.ink
            font.family: Theme.fontPrimary
            font.pixelSize: 18
            font.weight: Font.DemiBold
        }
    }

    Text {
        visible: page.loading || page.failed
        text: page.failed ? I18n.tr("Could not read the displays") : I18n.tr("Reading displays\u2026")
        color: page.dim(0.5)
        font.family: Theme.fontPrimary
        font.pixelSize: 13
    }

    Repeater {
        model: page.mons
        delegate: Rectangle {
            id: monCard
            required property var modelData
            width: parent.width
            radius: 20
            color: page.dim(0.06)
            height: cardColumn.implicitHeight + 28

            Column {
                id: cardColumn
                anchors { left: parent.left; right: parent.right; top: parent.top; margins: 14 }
                spacing: 10

                Item {
                    width: parent.width
                    height: 42

                    Rectangle {
                        id: monDisc
                        anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                        width: 38
                        height: 38
                        radius: 19
                        color: page.dim(0.12)
                        Pill.GlyphIcon {
                            anchors.centerIn: parent
                            width: 20
                            height: 20
                            name: "monitor"
                            color: page.dim(0.85)
                        }
                    }
                    Column {
                        anchors {
                            left: monDisc.right; leftMargin: 10
                            right: monFocus.left; rightMargin: 8
                            verticalCenter: parent.verticalCenter
                        }
                        spacing: 1
                        Text {
                            width: parent.width
                            text: monCard.modelData.name
                            color: page.ink
                            font.family: Theme.fontPrimary
                            font.pixelSize: 15
                            font.weight: Font.DemiBold
                            elide: Text.ElideRight
                        }
                        Text {
                            width: parent.width
                            text: monCard.modelData.width + "\u00D7" + monCard.modelData.height
                                + "  \u00B7  " + Math.round(monCard.modelData.refresh) + " Hz"
                                + "  \u00B7  " + page.fmtScale(monCard.modelData.scale)
                            color: page.dim(0.6)
                            font.family: Theme.fontPrimary
                            font.pixelSize: 11
                            elide: Text.ElideRight
                        }
                    }
                    Text {
                        id: monFocus
                        visible: monCard.modelData.focused === true
                        anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                        text: I18n.tr("focused")
                        color: page.accent
                        font.family: Theme.fontPrimary
                        font.pixelSize: 10
                        font.weight: Font.DemiBold
                    }
                }

                K.QsFader {
                    width: parent.width
                    glyph: "sun"
                    value: page.brightOf(monCard.modelData)
                    onMoved: (v) => page.setBright(monCard.modelData, v)
                }

                Row {
                    spacing: 8

                    Repeater {
                        model: page.chipsFor(monCard.modelData)
                        delegate: K.QsChip {
                            required property var modelData
                            readonly property var mon: monCard.modelData
                            text: page.fmtScale(Number(modelData))
                            active: Math.abs(Number(modelData) - mon.scale) < 0.005
                            onClicked: page.setScale(mon.name, Number(modelData))
                        }
                    }
                }

                Item {
                    id: resRow
                    width: parent.width
                    height: 34

                    Text {
                        anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                        text: I18n.tr("Resolution")
                        color: page.dim(0.6)
                        font.family: Theme.fontPrimary
                        font.pixelSize: 12
                    }
                    Row {
                        anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                        spacing: 2
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: page.modeLabel(page.modeOf(monCard.modelData))
                            color: page.dim(0.85)
                            font.family: Theme.fontPrimary
                            font.pixelSize: 12
                        }
                        Pill.MaterialIcon {
                            anchors.verticalCenter: parent.verticalCenter
                            text: page.expanded === monCard.modelData.name ? "expand_less" : "chevron_right"
                            color: page.dim(0.6)
                            font.pixelSize: 19
                        }
                    }
                    MouseArea {
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        onClicked: page.expanded = page.expanded === monCard.modelData.name
                            ? "" : monCard.modelData.name
                    }
                }

                Column {
                    visible: page.expanded === monCard.modelData.name
                    width: parent.width
                    spacing: 4

                    Repeater {
                        model: monCard.modelData.modes || []
                        delegate: Rectangle {
                            id: modeRow
                            required property var modelData
                            readonly property var mon: monCard.modelData
                            readonly property bool current: String(modelData) === page.modeOf(mon)
                            width: parent.width
                            height: 34
                            radius: 12
                            color: modeRow.current
                                ? Qt.rgba(page.accent.r, page.accent.g, page.accent.b, 0.16)
                                : (modeHover.hovered ? page.dim(0.10) : "transparent")

                            Text {
                                anchors { left: parent.left; leftMargin: 12; verticalCenter: parent.verticalCenter }
                                text: page.modeLabel(String(modeRow.modelData))
                                color: modeRow.current ? page.ink : page.dim(0.75)
                                font.family: Theme.fontPrimary
                                font.pixelSize: 12
                            }
                            Pill.MaterialIcon {
                                visible: modeRow.current
                                anchors { right: parent.right; rightMargin: 10; verticalCenter: parent.verticalCenter }
                                text: "check"
                                color: page.accent
                                font.pixelSize: 17
                            }
                            HoverHandler { id: modeHover }
                            MouseArea {
                                anchors.fill: parent
                                cursorShape: Qt.PointingHandCursor
                                onClicked: page.setMode(modeRow.mon.name, String(modeRow.modelData))
                            }
                        }
                    }
                }
            }
        }
    }

    Rectangle {
        visible: page.mons.length > 0
        width: parent.width
        height: 64
        radius: 20
        color: page.dim(0.06)

        Rectangle {
            id: nightDisc
            anchors { left: parent.left; leftMargin: 10; verticalCenter: parent.verticalCenter }
            width: 38
            height: 38
            radius: 19
            color: page.dim(0.12)
            Pill.GlyphIcon {
                anchors.centerIn: parent
                width: 19
                height: 19
                name: "moon"
                color: page.dim(0.85)
            }
        }
        Column {
            anchors {
                left: nightDisc.right; leftMargin: 10
                right: nightSwitch.left; rightMargin: 10
                verticalCenter: parent.verticalCenter
            }
            spacing: 1
            Text {
                text: I18n.tr("Night Light")
                color: page.ink
                font.family: Theme.fontPrimary
                font.pixelSize: 14
                font.weight: Font.DemiBold
            }
            Text {
                text: Toggles.nightOn ? I18n.tr("On") : I18n.tr("Off")
                color: page.dim(0.6)
                font.family: Theme.fontPrimary
                font.pixelSize: 11
            }
        }
        K.QsSwitch {
            id: nightSwitch
            anchors { right: parent.right; rightMargin: 12; verticalCenter: parent.verticalCenter }
            value: Toggles.nightOn
            onToggled: Toggles.toggleNight()
        }
    }
}
