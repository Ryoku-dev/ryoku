pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

// Live view of the daemon "updates" topic (ryoku status --json). The background
// check lives in the daemon; this just parses the streamed frame so the bar glyph
// reflects it without polling.
Singleton {
    id: root

    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"

    property var frame: ({})
    property string notificationScope: "ryoku"
    readonly property bool notifyRyoku: root.notificationScope !== "system"
    readonly property bool notifySystem: root.notificationScope === "system" || root.notificationScope === "both"
    readonly property bool ryokuAvailable: root.frame.available === true
    readonly property int ryokuPending: root.frame.pendingUpdates || 0
    readonly property int systemPending: root.frame.systemUpdates || 0
    readonly property bool available: (root.notifyRyoku && root.ryokuAvailable) || (root.notifySystem && root.systemPending > 0)
    readonly property int pending: (root.notifyRyoku ? root.ryokuPending : 0) + (root.notifySystem ? root.systemPending : 0)
    readonly property string notificationSummary: {
        var lines = [];
        if (root.notifyRyoku && root.ryokuAvailable)
            lines.push(I18n.tr("Ryoku updates: %1").arg(root.ryokuPending));
        if (root.notifySystem && root.systemPending > 0)
            lines.push(I18n.tr("System packages: %1").arg(root.systemPending));
        return lines.join("\n");
    }
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


    function loadNotificationScope() {
        if (notificationConfig.running)
            notificationConfig.reloadPending = true;
        else
            notificationConfig.running = true;
    }

    FileView {
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/hub.toml"
        watchChanges: true
        printErrors: false
        onFileChanged: root.loadNotificationScope()
    }

    Process {
        id: notificationConfig
        property bool reloadPending: false
        command: ["ryoku-hub", "config", "get", "update_notifications"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                var scope = this.text.trim();
                root.notificationScope = scope === "system" || scope === "both" ? scope : "ryoku";
            }
        }
        onExited: {
            if (reloadPending) {
                reloadPending = false;
                Qt.callLater(() => root.loadNotificationScope());
            }
        }
    }

    function check() { root.send("updates.check", {}); }

    function apply(line) {
        try {
            root.frame = JSON.parse(line);
        } catch (e) {}
    }

    function send(method, args) {
        ctl.queued += "call " + method + " " + JSON.stringify(args) + "\n";
        if (ctl.connected)
            ctl.flushQueued();
        else
            ctl.connected = true;
    }

    Socket {
        id: sub
        path: root.sockPath
        parser: SplitParser { onRead: line => root.apply(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe updates\n");
                flush();
            } else {
                retry.restart();
            }
        }
    }

    Timer {
        id: retry
        interval: 2000
        onTriggered: if (!sub.connected) sub.connected = true
    }

    Socket {
        id: ctl
        path: root.sockPath
        property string queued: ""
        function flushQueued() {
            if (queued.length === 0)
                return;
            write(queued);
            flush();
            queued = "";
        }
        onConnectionStateChanged: if (connected) flushQueued()
    }
}
