pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io
import shell.services
import "../../components"
import Ryoku.Ui.Singletons

// Brightness controls for the Super+Escape home module: one fader for the
// internal backlight (brightnessctl) plus one per external ddc monitor
// (ddcutil detect), so a multi-head rig gets a labelled fader each while a lone
// laptop panel gets one clean fader. `active` gates the probes so reads run only
// while the module is open, and writes debounce so a drag never floods i2c or
// brightnessctl.
Column {
    id: root

    property real s: 1
    property bool active: false

    spacing: 8 * s

    // total displays decides whether faders wear a name: a lone laptop panel
    // needs none, a multi-head rig does.
    readonly property int displayCount: (Devices.backlightAvailable ? 1 : 0) + Devices.ddcMonitors.length
    readonly property bool labelled: root.displayCount > 1

    property int pendingBl: -1

    // on open: enumerate external monitors and read the panel level once.
    onActiveChanged: {
        if (active) {
            Devices.startProbes(root);
        } else {
            Devices.stopProbes(root);
            blDebounce.stop();
            root.pendingBl = -1;
        }
    }
    Component.onDestruction: Devices.stopProbes(root)

    Timer {
        id: blDebounce
        interval: 40
        onTriggered: {
            if (root.active && root.pendingBl >= 0)
                Devices.setBacklight(root.pendingBl);
            root.pendingBl = -1;
        }
    }

    // internal backlight (the laptop panel); hidden when there is no backlight.
    Column {
        width: parent.width
        spacing: 3 * root.s
        visible: Devices.backlightAvailable
        MicroLabel {
            label: I18n.tr("Built-in")
            s: root.s
            visible: root.labelled
        }
        HFader {
            width: parent.width
            s: root.s
            icon: "sun"
            lit: blHov.hovered
            value: Math.max(0, Devices.backlightPct) / 100
            valueLabel: Math.max(0, Devices.backlightPct) + "%"
            onMoved: (v) => Devices.backlightPct = Math.max(1, Math.round(v * 100))
            onCommitted: (v) => {
                root.pendingBl = Math.max(1, Math.round(v * 100));
                blDebounce.restart();
            }
            HoverHandler { id: blHov }
        }
    }

    // external ddc monitors, one fader each, read once per open and on demand.
    Repeater {
        model: Devices.ddcMonitors
        delegate: Column {
            id: mon
            required property var modelData
            width: root.width
            spacing: 3 * root.s

            property int pct: 75
            property real pendingPct: -1

            MicroLabel {
                label: I18n.tr(mon.modelData.label)
                s: root.s
                visible: root.labelled
            }
            HFader {
                width: parent.width
                s: root.s
                icon: "sun"
                lit: monHov.hovered
                value: mon.pct / 100
                valueLabel: mon.pct + "%"
                onMoved: (v) => mon.pct = Math.max(5, Math.min(100, Math.round(v * 100)))
                onCommitted: (v) => {
                    mon.pendingPct = Math.max(5, Math.min(100, Math.round(v * 100)));
                    monCommit.restart();
                }
                HoverHandler { id: monHov }
            }
            Timer {
                id: monCommit
                interval: 160
                onTriggered: {
                    if (root.active && mon.pendingPct >= 0)
                        Devices.setBrightness(mon.modelData.bus, mon.pendingPct);
                    mon.pendingPct = -1;
                }
            }
            Process {
                command: ["timeout", "3", "ddcutil", "getvcp", "10", "--bus", mon.modelData.bus, "--brief"]
                running: root.active
                stdout: StdioCollector {
                    onStreamFinished: {
                        var v = Devices.parseBrightness(this.text);
                        if (v >= 0)
                            mon.pct = v;
                    }
                }
            }
        }
    }
}
