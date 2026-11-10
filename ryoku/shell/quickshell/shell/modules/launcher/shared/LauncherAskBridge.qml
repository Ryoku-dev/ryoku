pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import "../../ask" as Ask

Scope {
    id: root

    property string pending: ""
    property string pendingQuestion: ""
    property bool fromHistory: false
    property string errorText: ""

    readonly property alias client: socketClient
    readonly property alias session: askSession
    readonly property string status: socketClient.state

    signal recentReady()

    Ask.RashinClient { id: socketClient }
    Ask.RashinSession { id: askSession; client: socketClient; kind: "ask" }

    function flush() {
        if (!socketClient.ready || pending.length === 0)
            return;
        const action = pending;
        pending = "";
        if (action === "ask") {
            fromHistory = false;
            askSession.ask(pendingQuestion, []);
        } else if (action === "recent") {
            askSession.list(20, function() { root.recentReady(); });
        }
    }

    function ask(question) {
        pendingQuestion = String(question || "");
        pending = "ask";
        flush();
    }

    function recent() {
        pending = "recent";
        flush();
    }

    function attach(id) {
        fromHistory = true;
        askSession.attach(id);
    }

    function cancel() {
        pending = "";
        askSession.cancel();
    }

    function handoff(callback) {
        askSession.handoff(callback);
    }

    Connections {
        target: socketClient
        function onBecameReady() { root.flush(); }
    }

    Connections {
        target: askSession
        function onFailed(message) { root.errorText = message; }
    }
}
