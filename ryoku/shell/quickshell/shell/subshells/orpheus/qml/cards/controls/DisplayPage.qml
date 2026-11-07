pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io
import shell.services
import "../../kit" as K

Item {
    id: page

    required property var scene
    property var card: null
    property bool shown: false
    property bool live: false
    property var ddcValues: ({})
    property var ddcQueue: []
    property string readingBus: ""

    implicitWidth: 320
    implicitHeight: content.implicitHeight

    readonly property var monitors: Devices.ddcMonitors || []
    readonly property bool hasBrightness: Devices.backlightAvailable || monitors.length > 0
    readonly property real temperatureValue: Math.max(0, Math.min(1,
        (Number(Nightlight.temperature || 4000) - 2500) / 4000))

    function ddcValue(bus) {
        const value = ddcValues[String(bus)];
        return value === undefined ? 0.5 : Math.max(0, Math.min(1, value / 100));
    }

    function setDdc(bus, value) {
        const percent = Math.max(1, Math.min(100, Math.round(value * 100)));
        const next = Object.assign({}, ddcValues);
        next[String(bus)] = percent;
        ddcValues = next;
        Devices.setBrightness(bus, percent);
    }

    function queueReads() {
        if (!shown)
            return;
        const queued = [];
        for (let index = 0; index < monitors.length; ++index) {
            const monitor = monitors[index];
            if (!monitor || monitor.bus === undefined || monitor.bus === null)
                continue;
            const bus = String(monitor.bus);
            if (ddcValues[bus] === undefined && queued.indexOf(bus) < 0)
                queued.push(bus);
        }
        ddcQueue = queued;
        runRead();
    }

    function runRead() {
        if (!shown || ddcRead.running || ddcQueue.length === 0)
            return;
        readingBus = ddcQueue[0];
        ddcRead.command = ["ddcutil", "getvcp", "10", "--bus", readingBus, "--brief"];
        ddcRead.running = true;
    }

    function setLive(enabled) {
        if (enabled === live)
            return;
        live = enabled;
        if (enabled) {
            Devices.startProbes(page);
            Qt.callLater(page.queueReads);
        } else {
            Devices.stopProbes(page);
            ddcQueue = [];
            readingBus = "";
            ddcRead.running = false;
        }
    }

    function setTemperature(value) {
        Nightlight.setTemperature(Math.round(2500
            + Math.max(0, Math.min(1, value)) * 4000));
    }

    onShownChanged: setLive(shown)
    Component.onCompleted: setLive(shown)
    Component.onDestruction: setLive(false)

    Connections {
        target: Devices
        function onDdcMonitorsChanged() { page.queueReads(); }
    }

    Process {
        id: ddcRead
        command: []
        stdout: StdioCollector { id: ddcOutput }
        onExited: {
            const bus = page.readingBus;
            const percent = Devices.parseBrightness(ddcOutput.text || "");
            if (page.shown && bus !== "" && percent >= 0) {
                const next = Object.assign({}, page.ddcValues);
                next[bus] = percent;
                page.ddcValues = next;
            }
            page.ddcQueue = page.ddcQueue.slice(1);
            page.readingBus = "";
            Qt.callLater(page.runRead);
        }
    }

    component MonitorBrightness: Item {
        id: monitorRow

        required property string label
        required property string bus
        property bool divider: false

        implicitHeight: monitorLabel.implicitHeight + monitorSlider.implicitHeight + K.Style.s1

        K.ListRow {
            id: monitorLabel
            width: parent.width
            icon: "monitor"
            title: monitorRow.label || "External display"
            subtitle: "External brightness"
            divider: monitorRow.divider
        }

        K.Slider {
            id: monitorSlider
            anchors.top: monitorLabel.bottom
            anchors.topMargin: K.Style.s1
            width: parent.width
            value: page.ddcValue(monitorRow.bus)
            icon: "light_mode"
            valueText: Math.round(value * 100) + "%"
            onMoved: value => page.setDdc(monitorRow.bus, value)
            onCommitted: value => page.setDdc(monitorRow.bus, value)
        }
    }

    Column {
        id: content

        width: parent.width
        spacing: K.Style.s4

        Column {
            width: parent.width
            spacing: K.Style.s2

            K.Label {
                width: parent.width
                text: "Brightness"
                role: "micro"
            }

            Item {
                visible: Devices.backlightAvailable
                width: parent.width
                implicitHeight: panelLabel.implicitHeight + panelSlider.implicitHeight + K.Style.s1

                K.ListRow {
                    id: panelLabel
                    width: parent.width
                    icon: "laptop"
                    title: "Built-in display"
                    subtitle: "Backlight"
                }

                K.Slider {
                    id: panelSlider
                    anchors.top: panelLabel.bottom
                    anchors.topMargin: K.Style.s1
                    width: parent.width
                    value: Devices.backlightAvailable ? Devices.backlightPct / 100 : 0
                    icon: "light_mode"
                    valueText: Math.round(value * 100) + "%"
                    enabled: Devices.backlightAvailable
                    onMoved: value => Devices.setBacklight(Math.round(value * 100))
                    onCommitted: value => Devices.setBacklight(Math.round(value * 100))
                }
            }

            Repeater {
                model: page.monitors
                delegate: MonitorBrightness {
                    required property var modelData
                    required property int index
                    width: parent.width
                    label: String(modelData && modelData.label || "External display")
                    bus: String(modelData && modelData.bus || "")
                    divider: index > 0 || Devices.backlightAvailable
                }
            }

            K.Label {
                visible: !page.hasBrightness
                width: parent.width
                text: page.shown ? "No brightness controls found." : "Reading displays…"
                role: "body"
                tone: "variant"
            }
        }

        Column {
            width: parent.width
            spacing: K.Style.s2

            K.ListRow {
                width: parent.width
                icon: "nightlight"
                title: "Night light"
                subtitle: Nightlight.on ? "On" : "Off"
                selected: Nightlight.on

                K.Label {
                    visible: Nightlight.on
                    text: Math.round(Nightlight.temperature) + " K"
                    role: "caption"
                    tone: "onBone"
                    numeric: true
                }

                K.Switch {
                    z: 2
                    checked: Nightlight.on
                    onToggled: value => Nightlight.setEnabled(value, Nightlight.temperature)
                }
            }

            K.Slider {
                width: parent.width
                value: page.temperatureValue
                stepSize: 0.025
                icon: "thermostat"
                valueText: Math.round(Nightlight.temperature) + " K"
                onMoved: value => page.setTemperature(value)
                onCommitted: value => page.setTemperature(value)
            }

            Row {
                width: parent.width
                spacing: K.Style.s3

                K.Label {
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.max(0, parent.width - schedule.width - parent.spacing)
                    text: "Schedule"
                    role: "body"
                }

                K.Segmented {
                    id: schedule
                    width: Math.min(implicitWidth, 210)
                    model: [
                        { value: "off", label: "Manual", icon: "touch_app" },
                        { value: "sun", label: "Sunset", icon: "wb_twilight" }
                    ]
                    value: Nightlight.schedule === "sun" ? "sun" : "off"
                    onPicked: value => Nightlight.setSchedule(value === "sun")
                }
            }
        }
    }
}
