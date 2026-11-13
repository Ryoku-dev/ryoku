pragma Singleton

import QtQuick
import Quickshell
import Ryoku.Ui

// QML view of the daemon's `dictation` topic: the state of Ryoku's voice typing,
// the microphone level history the card draws, and the last tap that could not
// dictate. The popup reads this; nothing in the desktop talks to the dictation
// daemon directly, and nothing here polls.
Singleton {
    id: root

    property var frame: ({})
    property var levels: []
    property real drift: 0
    // when the last tap could not dictate, and why: the card flashes this
    property int attentionAt: 0
    property string attentionReason: ""

    readonly property string state: frame && frame.state ? frame.state : "idle"
    readonly property bool live: state === "recording"
    readonly property bool busy: live || state === "transcribing" || state === "inserting"
    readonly property bool loading: state === "loading"
    readonly property bool failed: state === "error"
    readonly property bool showingOff: attentionReason !== "" && (Date.now() - attentionAt) < 2200

    // the card exists only while the engine has something to say: an idle
    // desktop maps no window and runs no timer
    readonly property bool active: busy || loading || failed || showingOff

    readonly property int historyLength: 88

    function push(level: real) {
        const next = root.levels.slice(-(root.historyLength - 1));
        next.push(Math.max(0, Math.min(1, level || 0)));
        root.levels = next;
    }

    function apply(data) {
        const wasRecording = root.live;
        root.frame = data || {};
        if (root.live && !wasRecording)
            root.levels = [];
        if (root.live)
            root.push(root.frame.level);
        const stamp = root.frame.attentionMs || 0;
        if (stamp > 0 && stamp !== root.attentionAt) {
            root.attentionAt = stamp;
            root.attentionReason = root.frame.notice || "off";
        }
        if (root.attentionReason !== "" && !root.showingOff)
            root.attentionReason = "";
    }

    BusTopic {
        id: topic
        topic: "dictation"
        onUpdated: root.apply(topic.data)
    }

    Timer {
        interval: 33
        repeat: true
        running: root.live
        onTriggered: {
            root.drift = (root.drift + 0.18) % 6.28318;
            root.push(root.frame.level);
        }
    }

    Component.onCompleted: root.apply(topic.data)
}
