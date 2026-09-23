import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../../../../../utils/fuzzy.js" as Fuzzy
import ".."
import shell.services as Svc

// Open-window switcher: lists open windows, fuzzy-matched by title and class,
// and focuses the picked one. Default-ranked just below apps so "fire" surfaces
// a running Firefox window under the app entry.
Provider {
    id: windows

    providerId: "windows"

    // The window list is eager, so a query never refreshes it. The signature
    // check below suppresses redundant notifications so the launcher does not
    // redraw while the user types.
    property bool notifyQueued: false
    property string publishedSignature: ""

    function queueLiveUpdate() {
        if (notifyQueued)
            return;
        notifyQueued = true;
        Qt.callLater(function () {
            notifyQueued = false;
            var signature = windows.windowSignature();
            if (signature === publishedSignature)
                return;
            publishedSignature = signature;
            Dispatcher.notifyAsync();
        });
    }

    Connections {
        target: Wm
        function onWindowsChanged() { windows.queueLiveUpdate(); }
    }

    // Focusing must wait until the palette's close morph unmaps its exclusive-
    // focus layer: focusing earlier gets overridden when the unmap hands focus to
    // the window under the cursor on mouse refocus.
    property var pendingFocus: null
    Timer {
        id: focusDelay
        interval: Motion.close + 110
        repeat: false
        onTriggered: {
            var e = windows.pendingFocus;
            windows.pendingFocus = null;
            if (!e)
                return;
            var w = e.toplevel;
            if (w)
                w.activate();
            else
                windows.focusWindow(e.address);
        }
    }

    function focusWindow(addr) {
        Wm.focusWindow(addr);
    }

    function entries() {
        var out = [];
        var wins = Wm.windows;
        for (var i = 0; i < wins.length; i++) {
            var w = wins[i];
            if (!w || !w.id)
                continue;
            var ws = Wm.workspaceByName(w.workspace);
            if (ws && ws.special)
                continue;
            out.push({
                address: w.id,
                toplevel: w.toplevel,
                title: w.title || w.appId || I18n.tr("Window"),
                cls: w.appId || "",
                workspace: w.workspace || "",
                keywords: [w.appId || ""]
            });
        }
        return out;
    }

    function windowSignature() {
        var list = windows.entries();
        var parts = [];
        for (var index = 0; index < list.length; index++) {
            var entry = list[index];
            parts.push([entry.address, entry.title, entry.cls, entry.workspace]
                .join("\u001f"));
        }
        parts.sort();
        return parts.join("\u001e");
    }

    function rowFor(e) {
        return {
            id: "window:" + e.address,
            windowAddress: e.address,
            appId: e.cls,
            title: e.title,
            subtitle: e.cls,
            icon: e.cls ? Svc.Icons.path(e.cls, "application-x-executable") : "",
            type: "Window",
            score: 5,
            actions: [{
                id: "focus",
                name: I18n.tr("Focus"),
                icon: "",
                execute: function () {
                    windows.pendingFocus = e;
                    focusDelay.restart();
                }
            }]
        };
    }

    function query(text) {
        var list = windows.entries();
        var q = (text || "").trim().toLowerCase();
        var rows = [];
        for (var i = 0; i < list.length; i++)
            if (q.length === 0 || Fuzzy.score({ name: list[i].title, keywords: list[i].keywords }, q) < 99)
                rows.push(windows.rowFor(list[i]));
        return rows;
    }

    Component.onCompleted: {
        windows.publishedSignature = windows.windowSignature();
        Dispatcher.register(windows);
    }
}
