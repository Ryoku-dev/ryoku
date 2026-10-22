pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Caelestia
import Caelestia.Config

/// Night light, backed by the ryoku-shell `nightlight` topic. The daemon holds one
/// warm temperature and a schedule, so the day temperature is fixed and the
/// schedule maps to auto mode.
Singleton {
    id: root

    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"

    property bool _on: false
    property int _temperature: 4000
    property string _schedule: "off"
    property var _beforePreview: null

    readonly property int currentTemperature: root._on ? root._temperature : root.dayTemperature
    // shortcut: the daemon has no day temperature, so it stays at neutral.
    readonly property int dayTemperature: 6500
    readonly property int nightTemperature: root._temperature
    readonly property bool active: root._on
    readonly property bool available: sub.connected
    readonly property bool autoMode: root._schedule !== "off"

    function nightLightToast(message: string): void {
        if (GlobalConfig.utilities.toasts.nightLightChanged)
            Toaster.toast(qsTr("Night Light"), qsTr(message), "dark_mode");
    }

    function setDayTemperature(temp: var): void {}

    function setNightTemperature(temp: var): void {
        if (temp === undefined || temp === null)
            return;
        root._temperature = temp;
        if (root._on)
            root._call("nightlight.set", { on: true, temperature: temp });
    }

    // A preview warms the screen now and remembers what to put back.
    function previewTemperature(temp: var): void {
        if (temp === undefined || temp === null)
            return;
        if (root._beforePreview === null)
            root._beforePreview = { on: root._on, temperature: root._temperature };
        root._call("nightlight.set", { on: true, temperature: temp });
    }

    function stopPreview(): void {
        const before = root._beforePreview;
        if (before === null)
            return;
        root._beforePreview = null;
        root._call("nightlight.set", { on: before.on, temperature: before.temperature });
    }

    function toggleNightLight(): void {
        root._call("nightlight.toggle", {});
    }

    function toggleAutoMode(): void {
        root._call("nightlight.set", { schedule: root.autoMode ? "off" : "sun" });
    }

    function _apply(line: string): void {
        try {
            const f = JSON.parse(line);
            root._on = f.on === true;
            if (typeof f.temperature === "number" && f.temperature > 0)
                root._temperature = f.temperature;
            if (f.schedule === "off" || f.schedule === "sun" || f.schedule === "clock")
                root._schedule = f.schedule;
        } catch (e) {
            // A malformed frame keeps the last good state.
        }
    }

    function _call(method: string, args: var): void {
        ctl.queued += "call " + method + " " + JSON.stringify(args) + "\n";
        if (ctl.connected)
            ctl.flushQueued();
        else
            ctl.connected = true;
    }

    Timer {
        id: retry

        interval: 2000
        onTriggered: if (!sub.connected) sub.connected = true
    }

    Socket {
        id: sub

        path: root.sockPath
        parser: SplitParser { onRead: line => root._apply(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe nightlight\n");
                flush();
            } else {
                retry.restart();
            }
        }
    }

    Socket {
        id: ctl

        property string queued: ""

        path: root.sockPath

        function flushQueued(): void {
            if (queued.length === 0)
                return;
            write(queued);
            flush();
            queued = "";
        }

        onConnectionStateChanged: if (connected) flushQueued()
    }
}
