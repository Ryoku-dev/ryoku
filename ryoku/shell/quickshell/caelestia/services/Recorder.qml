pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import caelestia.utils

Singleton {
    id: root

    readonly property string recordBin: Paths.bin("caelestia-record")

    readonly property alias running: props.running
    readonly property alias paused: props.paused
    readonly property alias elapsed: props.elapsed
    readonly property bool probing: running || needsStart || needsStop || needsPause
    property bool needsStart
    property list<string> startArgs
    property bool needsStop
    property bool needsPause

    function start(extraArgs = []): void {
        needsStart = true;
        startArgs = extraArgs;
        checkProc.running = true;
    }

    function startGif(): void {
        needsStart = true;
        startArgs = ["--gif"];
        checkProc.running = true;
    }

    function stop(): void {
        needsStop = true;
        checkProc.running = true;
    }

    function togglePause(): void {
        needsPause = true;
        checkProc.running = true;
    }

    function launchSpectacle(): void {
        Launch.exec(["spectacle", "-R", "r"]);
    }

    // Forces a fresh probe of gpu-screen-recorder; `running` updates on exit.
    function probeRecording(): void {
        if (!checkProc.running) checkProc.running = true;
    }

    PersistentProperties {
        id: props

        property bool running: false
        property bool paused: false
        property real elapsed: 0

        reloadableId: "recorder"
    }

    property bool _wasRunning: false

    Process {
        id: checkProc

        command: ["sh", "-c", "pidof gpu-screen-recorder >/dev/null && f=\"$(cat $HOME/.local/state/caelestia/record/current_recording_path 2>/dev/null)\" && [ -n \"$f\" ] && test -f \"$f\""]
        onExited: code => { // qmllint disable signal-handler-parameters
            let isRunning = (code === 0);

            if (isRunning && !root._wasRunning) {
                props.elapsed = 0;
                props.paused = false;
            }

            root._wasRunning = isRunning;
            props.running = isRunning;

            if (isRunning) {
                if (root.needsStop) {
                    Launch.exec([root.recordBin, "--stop"]);
                } else if (root.needsPause) {
                    Launch.exec([root.recordBin, "--pause"]);
                    props.paused = !props.paused;
                }
            } else if (root.needsStart) {
                Launch.exec([root.recordBin, ...root.startArgs]);
            }

            root.needsStart = false;
            root.needsStop = false;
            root.needsPause = false;
        }
    }

    Timer {
        interval: root.probing ? 500 : 2500
        repeat: true
        running: true
        onTriggered: {
            if (!checkProc.running) {
                checkProc.running = true;
            }
        }
    }

    Timer {
        interval: 1000
        repeat: true
        running: props.running && !props.paused

        onTriggered: {
            props.elapsed++;
        }
    }
}
