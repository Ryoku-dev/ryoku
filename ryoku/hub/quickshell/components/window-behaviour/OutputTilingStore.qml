import QtQuick
import Ryoku.Ui.Singletons
import "../../Singletons"

// per-display tiling as a Changes store, so undo works from any page: the link and
// the layouts ryoku-output-tiling remembers, and the displays and each one's live
// layout from the window manager topic, which follows a display being plugged in or out
QtObject {
    id: store

    property bool linked: true
    property bool known: false
    // output -> layout the helper saved
    property var remembered: ({})
    // output -> layout just written, until the topic shows it
    property var pending: ({})
    property string selected: ""

    readonly property var frame: Live.topic("wm") || ({})
    readonly property var outputs: (store.frame.outputs || []).filter(o => o && o.name && !o.disabled).map(o => o.name)
    readonly property string main: store.outputs[0] || ""
    readonly property string active: store.outputs.indexOf(store.selected) >= 0 ? store.selected : store.main
    // a display other than the main one is chosen while the link is off
    readonly property bool scoped: !store.linked && store.active !== "" && store.active !== store.main

    onFrameChanged: Changes.touch()

    function liveLayout(output) {
        var ws = store.frame.workspaces || [];
        var first = "";
        for (var i = 0; i < ws.length; i++) {
            if (ws[i].output !== output || ws[i].special === true || !ws[i].layout)
                continue;
            if (ws[i].active === true)
                return ws[i].layout;
            if (first === "")
                first = ws[i].layout;
        }
        return first;
    }

    function select(output) {
        store.selected = output;
        Changes.touch();
    }

    function read(key) {
        if (key === "linked")
            return store.known ? store.linked : undefined;
        if (key.indexOf("layout:") === 0) {
            var output = key.slice(7);
            return store.pending[output] || store.liveLayout(output) || store.remembered[output] || undefined;
        }
        return undefined;
    }

    function write(key, v, done) {
        // an undo back to a layout nothing ever read has no value to apply
        if (key !== "linked" && ["dwindle", "master", "scrolling"].indexOf(v) < 0) {
            done(true, "");
            return;
        }
        var argv, undo;
        if (key === "linked") {
            argv = ["ryoku-output-tiling", "link", v === true ? "on" : "off"];
            if (v !== true && store.selected === "")
                store.selected = store.outputs[1] || "";
            undo = store.linked;
            store.linked = v === true;
            store.known = true;
        } else {
            var output = key.slice(7);
            argv = ["ryoku-output-tiling", "set", output, String(v)];
            var next = Object.assign({}, store.pending);
            next[output] = String(v);
            store.pending = next;
        }
        Changes.touch();
        Changes.run(argv, function (ok, out, err, code) {
            if (key === "linked" && !ok)
                store.linked = undo;
            if (key !== "linked" && !ok) {
                var rest = Object.assign({}, store.pending);
                delete rest[key.slice(7)];
                store.pending = rest;
            } else if (key !== "linked") {
                settle.restart();
            }
            Changes.touch();
            // code -1 is a missing program, or a write stopped after 20 s
            done(ok, ok ? "" : code === -1
                ? (err === Changes.missingText(argv[0])
                    ? I18n.tr("The layout could not be changed because a Ryoku tool is missing. Update Ryoku, then try again.")
                    : I18n.tr("Changing the layout took too long, so Settings stopped waiting. Press Retry."))
                : I18n.tr("The layout could not be changed. Make sure your desktop is running, then press Retry."));
            store.load();
        }, { timeout: 20000 });
    }

    // the window manager topic takes a moment to show a layout just applied
    property Timer settleTimer: Timer {
        id: settle
        interval: 3000
        onTriggered: {
            store.pending = ({});
            Changes.touch();
        }
    }

    // what the helper saved, so the switch shows what was really applied; a failed read keeps the last state
    function load() {
        Changes.run(["ryoku-output-tiling", "get"], function (ok, out) {
            if (!ok)
                return;
            try {
                var st = JSON.parse(out);
                store.linked = st.linked !== false;
                store.remembered = st.outputs || ({});
                store.known = true;
            } catch (e) {
            }
            Changes.touch();
        }, { timeout: 10000 });
    }

    Component.onCompleted: {
        store.load();
        Changes.registerStore("outputtiling", store);
    }
}
