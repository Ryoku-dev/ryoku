pragma Singleton
import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons

// Live view of the daemon "updates" topic (ryoku status --json). The background
// check lives in the daemon; this just keeps the streamed frame so the bar glyph
// reflects it without polling.
Singleton {
    id: root

    property var frame: ({})
    readonly property bool available: root.frame.available === true
    readonly property int pending: root.frame.pendingUpdates || 0
    readonly property string channel: root.frame.channel || ""
    // a packaged box on a named release reports the release it runs and the
    // one its channel serves; show those, and fall back to the commit pair a
    // checkout (or a box that predates release naming) reports.
    readonly property bool named: !!(root.frame.release && root.frame.channelRelease)
    readonly property string installed: root.named ? root.frame.release : (root.frame.installedVersion || "")
    readonly property string latest: root.named ? root.frame.channelRelease : (root.frame.latestVersion || "")
    // the line's name ("Onogoro"); the latest name differs only when the
    // channel serves the next line, which is worth saying out loud.
    readonly property string installedName: root.frame.releaseName || ""
    readonly property string latestName: root.frame.channelReleaseName || ""
    readonly property var commits: root.frame.updates || []
    readonly property var packages: root.frame.packages || []
    readonly property string buildState: root.frame.channelBuild ? (root.frame.channelBuild.state || "") : ""
    readonly property string buildCommit: root.frame.channelBuild ? String(root.frame.channelBuild.commit || "").slice(0, 7) : ""
    readonly property string buildRun: root.frame.channelBuild ? (root.frame.channelBuild.run || "") : ""
    readonly property string buildStarted: root.frame.channelBuild ? (root.frame.channelBuild.started || "") : ""
    readonly property int buildMinutes: {
        root.frame;
        var started = Date.parse(root.buildStarted);
        return root.buildStarted === "" || isNaN(started)
            ? 0
            : Math.max(0, Math.floor((Date.now() - started) / 60000));
    }
    readonly property string checkState: root.frame.channelCheck ? (root.frame.channelCheck.state || "") : ""
    readonly property string checkCommit: root.frame.channelCheck ? String(root.frame.channelCheck.commit || "").slice(0, 7) : ""
    readonly property string checkRun: root.frame.channelCheck ? (root.frame.channelCheck.run || "") : ""


    function check() { root.send("updates.check", {}); }

    function apply(frame) {
        if (frame && typeof frame === "object")
            root.frame = frame;
    }

    function send(method, args) {
        Bus.call(method, args);
    }

    BusTopic {
        id: topicFeed
        topic: "updates"
        onUpdated: root.apply(topicFeed.data)
    }
}
