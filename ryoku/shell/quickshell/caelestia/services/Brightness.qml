pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Caelestia.Config
import Caelestia.Services
import caelestia.components.misc
import caelestia.services

Singleton {
    id: root

    property list<var> ddcMonitors: []
    readonly property var ddcMonitorMap: {
        const map = {};
        for (const m of ddcMonitors)
            map[m.connector] = m;
        return map;
    }
    readonly property list<Monitor> monitors: variants.instances // qmllint disable incompatible-type
    property bool appleDisplayPresent: false

    function getMonitorForScreen(screen: ShellScreen): var {
        return monitors.find(m => m.modelData === screen); // qmllint disable missing-property
    }

    function getMonitor(query: string): var {
        if (query === "active") {
            return monitors.find(m => Kwin.monitorFor(m.modelData)?.focused); // qmllint disable missing-property
        }

        if (query.startsWith("model:")) {
            const model = query.slice(6);
            return monitors.find(m => m.modelData.model === model); // qmllint disable missing-property
        }

        if (query.startsWith("serial:")) {
            const serial = query.slice(7);
            return monitors.find(m => m.modelData.serialNumber === serial); // qmllint disable missing-property
        }

        if (query.startsWith("id:")) {
            const id = parseInt(query.slice(3), 10);
            return monitors.find(m => Kwin.monitorFor(m.modelData)?.id === id); // qmllint disable missing-property
        }

        return monitors.find(m => m.modelData.name === query); // qmllint disable missing-property
    }

    function increaseBrightness(): void {
        getMonitor("active")?.stepBrightness(GlobalConfig.services.brightnessIncrement);
    }

    function decreaseBrightness(): void {
        getMonitor("active")?.stepBrightness(-GlobalConfig.services.brightnessIncrement);
    }

    onMonitorsChanged: {
        ddcMonitors = [];
        ddcProc.running = true;
    }

    Variants {
        id: variants

        model: Quickshell.screens

        Monitor {}
    }

    Process {
        running: true
        command: ["sh", "-c", "asdbctl get"]
        stdout: StdioCollector {
            onStreamFinished: root.appleDisplayPresent = text.trim().length > 0
        }
    }

    Process {
        id: ddcProc

        command: ["ddcutil", "detect", "--brief"]
        stdout: StdioCollector {
            onStreamFinished: root.ddcMonitors = text.trim().split("\n\n").filter(d => d.startsWith("Display ")).map(d => {
                        const busNum = d.match(/I2C bus:[ ]*\/dev\/i2c-([0-9]+)/)?.[1];
                        const connector = d.match(/DRM connector:\s+(.*)/)?.[1]?.replace(/^card\d+-/, "");
                        return busNum && connector ? { busNum, connector } : null;
                    }).filter(m => m)
        }
    }

    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "brightnessUp"
        description: qsTr("Increase brightness")
        onPressed: root.increaseBrightness()
    }

    // qmllint disable unresolved-type
    CustomShortcut {
        // qmllint enable unresolved-type
        name: "brightnessDown"
        description: qsTr("Decrease brightness")
        onPressed: root.decreaseBrightness()
    }

    IpcHandler {
        function get(): real {
            return getFor("active");
        }

        function getFor(query: string): real {
            return root.getMonitor(query)?.brightness ?? -1;
        }

        function getDimming(): real {
            return getDimmingFor("active");
        }

        function getDimmingFor(query: string): real {
            return root.getMonitor(query)?.dimming ?? -1;
        }

        function set(value: string): string {
            return setFor("active", value);
        }

        // Handles brightness value like brightnessctl: 0.1, +0.1, 0.1-, 10%, +10%, 10%-
        function setFor(query: string, value: string): string {
            const monitor = root.getMonitor(query);
            if (!monitor)
                return "Invalid monitor: " + query;

            const current = monitor.effectiveBrightness;
            let targetBrightness;
            if (value.endsWith("%-")) {
                const percent = parseFloat(value.slice(0, -2));
                targetBrightness = current - (percent / 100);
            } else if (value.startsWith("+") && value.endsWith("%")) {
                const percent = parseFloat(value.slice(1, -1));
                targetBrightness = current + (percent / 100);
            } else if (value.endsWith("%")) {
                const percent = parseFloat(value.slice(0, -1));
                targetBrightness = percent / 100;
            } else if (value.startsWith("+")) {
                const increment = parseFloat(value.slice(1));
                targetBrightness = current + increment;
            } else if (value.endsWith("-")) {
                const decrement = parseFloat(value.slice(0, -1));
                targetBrightness = current - decrement;
            } else if (value.includes("%") || value.includes("-") || value.includes("+")) {
                return `Invalid brightness format: ${value}\nExpected: 0.1, +0.1, 0.1-, 10%, +10%, 10%-`;
            } else {
                targetBrightness = parseFloat(value);
            }

            if (isNaN(targetBrightness))
                return `Failed to parse value: ${value}\nExpected: 0.1, +0.1, 0.1-, 10%, +10%, 10%-`;

            monitor.setBrightness(targetBrightness);

            return `Set monitor ${monitor.modelData.name} brightness to ${+monitor.brightness.toFixed(2)}`;
        }

        target: "brightness"
    }

    component Monitor: QtObject {
        id: monitor

        required property ShellScreen modelData
        readonly property var ddcInfo: root.ddcMonitorMap[modelData.name] ?? null
        readonly property bool isDdc: ddcInfo !== null
        readonly property string busNum: ddcInfo?.busNum ?? ""
        readonly property bool isAppleDisplay: root.appleDisplayPresent && modelData.model.startsWith("StudioDisplay")
        readonly property bool writesHardwareDirectly: isAppleDisplay || isDdc
        property real brightness: 1.0
        property real dimming: 1.0
        readonly property real effectiveBrightness: brightness * dimming
        property real queuedBrightness: NaN
        property bool verifying: false

        readonly property Process readProc: Process {
            stdout: StdioCollector {
                onStreamFinished: {
                    if (monitor.isAppleDisplay) {
                        const val = parseInt(text.trim());
                        monitor.brightness = val / 101;
                        return;
                    }

                    const [, , , cur, max] = text.split(" ");
                    const actual = parseInt(cur) / parseInt(max);

                    if (!monitor.verifying) {
                        monitor.brightness = actual;
                        return;
                    }

                    monitor.verifying = false;
                    const target = Math.round(monitor.brightness * 100);

                    if (isNaN(actual) || Math.round(actual * 100) === target)
                        return;

                    monitor.writeBrightness(target / 100);
                    monitor.brightness = target / 100;

                    if (isDdc)
                        timer.restart();
                }
            }
        }

        readonly property Timer timer: Timer {
            interval: 500
            onTriggered: {
                if (!isNaN(monitor.queuedBrightness)) {
                    const pending = monitor.queuedBrightness;
                    monitor.queuedBrightness = NaN;
                    monitor.brightness = pending;
                    monitor.writeBrightness(pending);
                    monitor.timer.restart();
                }
            }
        }

        function writeBrightness(value: real): void {
            const rounded = Math.round(value * 100);

            if (isAppleDisplay)
                Quickshell.execDetached(["asdbctl", "set", rounded]);
            else if (isDdc)
                Quickshell.execDetached(["ddcutil", "-b", busNum, "setvcp", "10", rounded]);
            else
                Quickshell.execDetached(["brightnessctl", "set", `${rounded}%`]);
        }

        // Steps are relative to what the screen renders, not to the value last commanded.
        function stepBrightness(delta: real): void {
            setBrightness(effectiveBrightness + delta);
        }

        function setBrightness(value: real): void {
            value = Math.max(0, Math.min(1, value));
            const rounded = Math.round(value * 100);

            if (dimming < 1)
                dimming = 1;

            if (Math.round(brightness * 100) === rounded) {
                if (isDdc && !timer.running && !readProc.running) {
                    verifying = true;
                    readProc.running = true;
                }
                return;
            }

            if (isDdc && timer.running) {
                queuedBrightness = value;
                brightness = value;
                return;
            }

            brightness = value;
            writeBrightness(value);

            if (isDdc)
                timer.restart();
        }

        function initBrightness(): void {
            if (isAppleDisplay)
                readProc.command = ["asdbctl", "get"];
            else if (isDdc)
                readProc.command = ["ddcutil", "-b", busNum, "getvcp", "10", "--brief"];
            else
                // Internal panel: shaped like ddcutil's brief reply so one parser reads both.
                readProc.command = ["sh", "-c", 'echo "VCP 10 C $(brightnessctl get) $(brightnessctl max)"'];

            readProc.running = true;
        }

        onBusNumChanged: initBrightness()
        Component.onCompleted: initBrightness()
    }
}
