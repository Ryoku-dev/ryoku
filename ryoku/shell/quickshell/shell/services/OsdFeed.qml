pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

// QML view of the daemon `osd` topic. The volume and mic OSDs read PipeWire
// directly (Ryoku owns the audio graph in QML), but brightness has no
// QML-observable source: the media keys and the brightness verb both drive the
// backlight directly, so the daemon watches the primary backlight and pushes
// {brightness:{seq,value}} here. The brightness OSD shows when `brightnessSeq`
// advances and binds `brightness` (0..1) to its bar. Contract 12 sec 3, sec 9.
Singleton {
    id: root

    property real brightness: 0
    property int brightnessSeq: 0
    property int previewSeq: 0
    property real expectedMicVolume: -1
    property double expectedMicVolumeAt: 0

    function expectMicVolume(value) {
        root.expectedMicVolume = value
        root.expectedMicVolumeAt = Date.now()
    }

    function acceptsMicVolume(value) {
        return root.expectedMicVolumeAt > 0
            && Date.now() - root.expectedMicVolumeAt < 1500
            && Math.abs(value - root.expectedMicVolume) < 0.015
    }


    signal volumeRequested()
    signal brightnessRequested()
    signal micRequested()
    signal keyboardRequested()
    signal mediaRequested(string action)
    signal hideRequested()

    IpcHandler {
        target: "osd"
        function preview(): void { root.previewSeq++; }
        function volume(): void { root.volumeRequested(); }
        function brightness(): void { root.brightnessRequested(); }
        function mic(): void { root.micRequested(); }
        function keyboard(): void { root.keyboardRequested(); }
        function media(action: string): string {
            const normalized = action.length === 0 ? "play" : action
            if (!["play", "pause", "next", "previous"].includes(normalized))
                return "Unknown action: play, pause, next or previous"
            root.mediaRequested(normalized)
            return normalized
        }
        function hide(): void { root.hideRequested(); }
    }
    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"

    function apply(line) {
        try {
            const b = JSON.parse(line).brightness;
            if (b) {
                if (typeof b.value === "number")
                    root.brightness = Math.max(0, Math.min(1, b.value));
                if (typeof b.seq === "number")
                    root.brightnessSeq = b.seq;
            }
        } catch (e) {
            // A malformed frame must never wedge the OSD; keep the last value.
        }
    }

    // Subscribe once for the whole shell; every screen's brightness OSD reads
    // this singleton rather than opening its own socket.
    Socket {
        id: sub
        path: root.sockPath
        parser: SplitParser { onRead: line => root.apply(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe osd\n");
                flush();
            } else {
                retry.restart();
            }
        }
    }

    // The daemon may be down at load or restart under the shell; retry quietly.
    Timer {
        id: retry
        interval: 2000
        onTriggered: if (!sub.connected) sub.connected = true
    }
}
