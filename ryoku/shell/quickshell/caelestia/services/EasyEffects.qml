pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import caelestia.utils

Singleton {
    id: root

    property bool available: false
    property bool active: false

    function refresh(): void {
        activeProc.running = true;
    }

    function enable(): void {
        Launch.exec(["bash", "-c",
            "easyeffects --hide-window --service-mode >/dev/null 2>&1 || "
            + "flatpak run com.github.wwmm.easyeffects --hide-window --service-mode >/dev/null 2>&1"]);
        confirmTimer.restart();
    }

    function disable(): void {
        Launch.exec(["bash", "-c",
            "pkill -x easyeffects >/dev/null 2>&1 || "
            + "flatpak kill com.github.wwmm.easyeffects >/dev/null 2>&1"]);
        confirmTimer.restart();
    }

    function open(): void {
        Launch.exec(["bash", "-c",
            "easyeffects -q >/dev/null 2>&1; flatpak kill com.github.wwmm.easyeffects >/dev/null 2>&1; "
            + "easyeffects >/dev/null 2>&1 || flatpak run com.github.wwmm.easyeffects >/dev/null 2>&1"]);
        confirmTimer.restart();
    }

    function toggle(): void {
        if (root.active)
            root.disable();
        else
            root.enable();
    }

    Process {
        id: availableProc

        command: ["bash", "-c",
            "command -v easyeffects >/dev/null 2>&1 || flatpak info com.github.wwmm.easyeffects >/dev/null 2>&1"]
        running: true
        onExited: code => {
            root.available = code === 0;
            if (root.available)
                root.refresh();
        }
    }
    Process {
        id: activeProc

        command: ["bash", "-c",
            "pidof -q easyeffects || flatpak ps --columns=application 2>/dev/null | grep -qx com.github.wwmm.easyeffects"]
        onExited: code => root.active = code === 0
    }
    // Starting and stopping are fire-and-forget, so the state is read back
    // rather than assumed: a launch that fails, or a stop that does not take,
    // would otherwise leave the toggle showing something that is not true.
    Timer {
        id: confirmTimer

        interval: 1200
        onTriggered: root.refresh()
    }
}
