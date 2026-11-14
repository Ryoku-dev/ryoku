pragma Singleton

import QtQuick
import Quickshell
import Ryoku.Ui

Singleton {
    id: root

    property var frame: ({})
    property var levels: []
    property var microphoneFloor: []
    property real activity: 0
    property int calibrationSamples: 0
    property real attentionAt: 0
    property string attentionReason: ""
    property bool showingOff: false
    property bool melting: false

    readonly property string state: frame.state || "idle"
    readonly property bool live: state === "recording"
    readonly property bool busy: live || state === "transcribing" || state === "inserting"
    readonly property bool loading: state === "loading"
    readonly property bool failed: state === "error"
    readonly property bool present: busy || loading || failed || showingOff
    readonly property bool active: present || melting
    readonly property string modelName: String(frame.model || "").split("/").pop()
    readonly property real decibels: 20 * Math.log10(Math.max(0.000001, Number(frame.level || 0)))

    // This trace shows speech variation, not absolute input gain. Calibrate
    // against the recent microphone floor so a hot input starts at rest too.
    function measure(rms: real): real {
        if (!isFinite(rms) || rms <= 0) {
            root.activity *= 0.65;
            return root.activity;
        }
        const db = 20 * Math.log10(Math.max(0.000001, rms));
        const recent = root.microphoneFloor.slice(-63);
        recent.push(db);
        root.microphoneFloor = recent;
        const sorted = recent.slice().sort((a, b) => a - b);
        // PipeWire may ramp up over its first buffers. A short median estimate
        // keeps that ramp from becoming the floor for the whole recording.
        if (root.calibrationSamples < 8) {
            root.calibrationSamples++;
            if (root.calibrationSamples === 8) {
                root.microphoneFloor = [sorted[Math.floor(sorted.length / 2)]];
            }
            return 0;
        }
        // A low percentile ignores isolated buffer dropouts without letting
        // ordinary speech raise the baseline immediately.
        const floor = sorted[Math.floor((sorted.length - 1) * 0.2)];
        const excess = Math.max(0, db - floor - 0.75);
        const target = Math.sqrt(Math.min(1, excess / 6));
        root.activity += (target - root.activity) * (target > root.activity ? 0.8 : 0.35);
        return root.activity;
    }

    onPresentChanged: {
        if (present) {
            melt.stop();
            melting = false;
        } else {
            melting = true;
            melt.restart();
        }
    }

    Timer {
        id: melt
        interval: 240
        onTriggered: root.melting = false
    }
    Timer {
        id: refusal
        interval: 2200
        onTriggered: {
            root.showingOff = false;
            root.attentionReason = "";
        }
    }

    function apply(data) {
        const wasRecording = root.live;
        root.frame = data || {};
        if (root.live) {
            if (!wasRecording) {
                root.microphoneFloor = [];
                root.activity = 0;
                root.calibrationSamples = 0;
                root.levels = Array(64).fill(0);
            }
            const sample = measure(Number(root.frame.level || 0));
            const next = root.levels.slice(-63);
            next.push(sample);
            root.levels = next;
        }
        const stamp = Number(root.frame.attentionMs || 0);
        if (root.frame.enabled === true) {
            refusal.stop();
            root.showingOff = false;
            root.attentionReason = "";
        } else if (stamp > 0 && stamp !== root.attentionAt) {
            root.attentionAt = stamp;
            root.attentionReason = root.frame.notice || "off";
            root.showingOff = true;
            refusal.restart();
        }
    }

    BusTopic {
        id: topic
        topic: "dictation"
        onUpdated: root.apply(topic.data)
    }
    Component.onCompleted: root.apply(topic.data)
}
