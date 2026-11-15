pragma Singleton

import QtQuick
import Quickshell
import Ryoku.Ui

// QML view of the daemon `osd` and `audio` topics. Brightness has no
// QML-observable source: the media keys and the brightness verb both drive the
// backlight directly, so the daemon watches the primary backlight and pushes
// {brightness:{seq,value}} here. The brightness OSD shows when
// `brightnessSeq` advances and binds `brightness` (0..1) to its bar. Default
// sink/source levels arrive on the `audio` topic, read from PipeWire by the
// daemon, so the volume and mic OSDs need no Pipewire object either.
// Contract 12 sec 3, sec 9.
Singleton {
    id: root

    property real brightness: 0
    property int brightnessSeq: 0
    property int previewSeq: 0
    property real expectedMicVolume: -1
    property double expectedMicVolumeAt: 0

    // Default-device levels from the daemon `audio` topic. A -1 in the frame
    // means the server was unreadable at read time; the last real value stays.
    property real volume: 0
    property bool muted: false
    property real micVolume: 0
    property bool micMuted: false
    property string sinkName: ""
    property string sourceName: ""

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

    BusIpcHandler {
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

    function apply(frame) {
        const b = frame ? frame.brightness : null;
        if (b) {
            if (typeof b.value === "number")
                root.brightness = Math.max(0, Math.min(1, b.value));
            if (typeof b.seq === "number")
                root.brightnessSeq = b.seq;
        }
    }

    function applyAudio(frame) {
        if (!frame)
            return;
        if (typeof frame.volume === "number" && frame.volume >= 0)
            root.volume = frame.volume;
        root.muted = frame.muted === true;
        if (typeof frame.mic_volume === "number" && frame.mic_volume >= 0)
            root.micVolume = frame.mic_volume;
        root.micMuted = frame.mic_muted === true;
        if (typeof frame.sink === "string")
            root.sinkName = frame.sink;
        if (typeof frame.source === "string")
            root.sourceName = frame.source;
    }

    // Subscribe once for the whole shell; every screen's brightness OSD reads
    // this singleton rather than subscribing itself.
    BusTopic {
        id: topicFeed
        topic: "osd"
        onUpdated: root.apply(topicFeed.data)
    }

    BusTopic {
        id: audioFeed
        topic: "audio"
        onUpdated: root.applyAudio(audioFeed.data)
    }
}
