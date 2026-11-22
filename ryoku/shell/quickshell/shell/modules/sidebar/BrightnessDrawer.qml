pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons
import shell.services
import "../ask/look"

Item {
    id: root

    required property real s
    required property var screen
    required property bool active
    property bool linked: false
    readonly property int displayCount: (Devices.backlightAvailable ? 1 : 0) + (Devices.ddcMonitors || []).length
    readonly property string internalOutput: {
        const outputs = Wm.outputs || [];
        for (let i = 0; i < outputs.length; ++i) {
            const name = outputs[i] && outputs[i].name ? String(outputs[i].name) : "";
            if (/eDP|LVDS|DSI/i.test(name))
                return name;
        }
        return I18n.tr("Built-in display");
    }
    readonly property real rowGap: Math.round((QsLook.lithic ? 6 : QsLook.material ? 12 : 10) * root.s)
    signal levelChanged(string outputName, int percent)

    implicitHeight: body.implicitHeight

    function applyLinked(source, percent): void {
        if (!root.linked)
            return;
        const clamped = Math.max(1, Math.min(100, Math.round(percent)));
        if (Devices.backlightAvailable && source !== "internal")
            internalRow.setLinkedPercent(clamped);
        for (let i = 0; i < ddcRepeater.count; ++i) {
            const row = ddcRepeater.itemAt(i);
            if (row && row.bus !== source)
                row.setLinkedPercent(clamped);
        }
    }

    onActiveChanged: {
        if (active) {
            Devices.startProbes(root);
        } else {
            internalRow.flush();
            for (let i = 0; i < ddcRepeater.count; ++i) {
                const row = ddcRepeater.itemAt(i);
                if (row)
                    row.flush();
            }
            Devices.stopProbes(root);
        }
    }
    Component.onCompleted: if (active) Devices.startProbes(root)
    Component.onDestruction: {
        internalRow.flush();
        for (let i = 0; i < ddcRepeater.count; ++i) {
            const row = ddcRepeater.itemAt(i);
            if (row)
                row.flush();
        }
        Devices.stopProbes(root);
    }

    component DisplayName: Text {
        id: displayName
        property real s: 1
        visible: !QsLook.lithic
        color: Lithic.fg2
        font.family: QsLook.sans
        font.pixelSize: Math.round(12 * displayName.s)
        font.weight: Font.Medium
        elide: Text.ElideRight
        renderType: Text.NativeRendering
    }

    Column {
        id: body
        width: parent.width
        spacing: root.rowGap

        Column {
            id: internalRow
            width: parent.width
            visible: Devices.backlightAvailable
            spacing: Math.round(2 * root.s)
            property int pending: -1

            function setPercent(percent, broadcast): void {
                const value = Math.max(1, Math.min(100, Math.round(percent)));
                Devices.backlightPct = value;
                pending = value;
                internalCommit.restart();
                root.levelChanged(root.internalOutput, value);
                if (broadcast)
                    root.applyLinked("internal", value);
            }
            function setLinkedPercent(percent): void { setPercent(percent, false); }
            function flush(): void {
                if (pending >= 0)
                    Devices.setBacklight(pending);
                pending = -1;
                internalCommit.stop();
            }

            DisplayName {
                width: parent.width
                s: root.s
                text: root.internalOutput
            }
            LevelSlider {
                width: parent.width
                s: root.s
                label: root.internalOutput
                glyph: value < 0.34 ? "brightness_low" : value < 0.67 ? "brightness_medium" : "brightness_high"
                value: Math.max(0, Devices.backlightPct) / 100
                from: 0.01
                available: Devices.backlightAvailable
                expandable: false
                onAdjusted: value => internalRow.setPercent(value * 100, true)
            }
            Timer {
                id: internalCommit
                interval: 90
                onTriggered: internalRow.flush()
            }
        }

        Repeater {
            id: ddcRepeater
            model: Devices.ddcMonitors || []
            delegate: Column {
                id: monitorRow
                required property var modelData
                readonly property string bus: String(modelData.bus)
                readonly property string outputName: String(modelData.label)
                property int percent: -1
                property int pending: -1
                width: body.width
                spacing: Math.round(2 * root.s)

                function readLevel(): void {
                    if (!root.active || readProcess.running)
                        return;
                    readProcess.command = ["timeout", "3", "ddcutil", "getvcp", "10", "--brief", "--bus", monitorRow.bus];
                    readProcess.running = true;
                }
                function setPercent(value, broadcast): void {
                    const next = Math.max(5, Math.min(100, Math.round(value)));
                    monitorRow.percent = next;
                    monitorRow.pending = next;
                    commit.restart();
                    root.levelChanged(monitorRow.outputName, next);
                    if (broadcast)
                        root.applyLinked(monitorRow.bus, next);
                }
                function setLinkedPercent(value): void { setPercent(value, false); }
                function flush(): void {
                    if (monitorRow.pending >= 0)
                        Devices.setBrightness(monitorRow.bus, monitorRow.pending);
                    monitorRow.pending = -1;
                    commit.stop();
                }
                function syncActive(): void {
                    if (root.active)
                        monitorRow.readLevel();
                    else {
                        readProcess.running = false;
                        monitorRow.flush();
                    }
                }

                Component.onCompleted: monitorRow.syncActive()
                Component.onDestruction: monitorRow.flush()
                Connections {
                    target: root
                    function onActiveChanged(): void { monitorRow.syncActive(); }
                }

                DisplayName {
                    width: parent.width
                    s: root.s
                    text: monitorRow.outputName
                }
                LevelSlider {
                    width: parent.width
                    s: root.s
                    label: monitorRow.outputName
                    glyph: value < 0.34 ? "brightness_low" : value < 0.67 ? "brightness_medium" : "brightness_high"
                    value: Math.max(0, monitorRow.percent) / 100
                    from: 0.05
                    available: monitorRow.percent >= 0
                    expandable: false
                    onAdjusted: value => monitorRow.setPercent(value * 100, true)
                }
                Timer {
                    id: commit
                    interval: 160
                    onTriggered: monitorRow.flush()
                }
                Process {
                    id: readProcess
                    stdout: StdioCollector {
                        onStreamFinished: {
                            const value = Devices.parseBrightness(text);
                            if (value >= 0) {
                                monitorRow.percent = value;
                                root.levelChanged(monitorRow.outputName, value);
                            }
                        }
                    }
                }
            }
        }

        SidebarToggle {
            width: parent.width
            visible: root.displayCount > 1
            s: root.s
            text: I18n.tr("Link displays")
            glyph: "link"
            on: root.linked
            onToggleRequested: value => root.linked = value
        }
    }
}
