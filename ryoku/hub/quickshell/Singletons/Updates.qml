pragma Singleton
import QtQuick
import Quickshell
import Ryoku.Ui.Singletons

// Updates view for the Hub, fed by the daemon "updates" topic (ryoku status
// --json). The daemon owns the background check; check() forces a fresh run.
Singleton {
    id: root

    property var frame: ({})

    property bool available: false
    property string currentVersion: ""
    property string latestVersion: ""
    // the release line's name ("Onogoro") for the box and for what the
    // channel serves; "" on a checkout or a box that predates naming.
    property string currentName: ""
    property string latestName: ""
    property string branch: "main"
    property int behind: 0
    readonly property bool snapshotsSupported: Host.supports("snapshots")
    readonly property string snapshotReason: Host.reason("snapshots")

    // Ryoku channel commits (when behind) and recent history (when current).
    property var updates: []
    property var recent: []
    // system packages the host distribution's full upgrade would pull: [{ name, old, new }].
    property var packages: []
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


    property var lastChecked: null
    property int tick: 0
    readonly property string checkedAgo: {
        root.tick;
        if (!root.lastChecked)
            return "not yet";
        var s = Math.floor((Date.now() - root.lastChecked.getTime()) / 1000);
        if (s < 10)
            return "just now";
        if (s < 60)
            return s + "s ago";
        var m = Math.floor(s / 60);
        if (m < 60)
            return m + "m ago";
        var h = Math.floor(m / 60);
        if (h < 24)
            return h + "h ago";
        return Math.floor(h / 24) + "d ago";
    }


    function check() { Bus.call("updates.check", {}); }

    // o is the bus's shared parsed frame: read it, never mutate it.
    function apply(o) {
        if (!o || typeof o !== "object")
            return;
        root.frame = o;

        // a packaged box on a named release: show the release names,
        // else the commit pair a checkout reports.
        var named = !!(o.release && o.channelRelease);
        root.currentVersion = named ? o.release : (o.installedVersion || "");
        root.latestVersion = named ? o.channelRelease : (o.latestVersion || "");
        root.currentName = o.releaseName || "";
        root.latestName = o.channelReleaseName || "";
        root.branch = o.channel || "main";
        root.behind = o.pendingUpdates || 0;
        root.updates = o.updates || [];
        root.recent = o.recent || [];
        root.packages = o.packages || [];
        root.available = o.available === true;
        root.lastChecked = new Date();
    }

    Timer {
        interval: 30000
        running: true
        repeat: true
        onTriggered: root.tick++
    }

    Component.onCompleted: {
        Bus.subscribe("updates", data => root.apply(data));
        root.check();
    }
}
