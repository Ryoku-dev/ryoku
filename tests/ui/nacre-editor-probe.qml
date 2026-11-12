import QtQuick
import Quickshell
import Ryoku.FrameBars
import Ryoku.Ui.Singletons
import "Singletons"

// The Bar page owns the Nacre editor: with Nacre in use, moving a widget
// between the lane editor's sections stores the expected nacre layout, taking
// one off the bar stores it unused, and each appearance row's control writes
// its own nacre key. Reads go through Settings, which holds every write the
// page made until the daemon answers, so the probe needs no daemon.
ShellRoot {
    id: probe

    property Item page: null
    property int failures: 0
    property int waited: 0

    function fail(message) {
        probe.failures++;
        console.log("NACRE-EDITOR-PROBE-FAIL " + message);
    }

    function require(condition, message) {
        if (!condition)
            probe.fail(message);
        return condition;
    }

    function finish() {
        console.log(probe.failures === 0 ? "NACRE-EDITOR-PROBE-PASS" : "NACRE-EDITOR-PROBE-FAILURES " + probe.failures);
        Qt.quit();
    }

    function walk(item, test, out) {
        if (!item)
            return out;
        if (test(item))
            out.push(item);
        const kids = item.children || [];
        for (let i = 0; i < kids.length; i++)
            probe.walk(kids[i], test, out);
        return out;
    }

    function laneEditor() {
        return probe.walk(probe.page, o => o.lanes !== undefined && o.spare !== undefined && typeof o.moved === "function", [])[0] || null;
    }

    function control(rowId) {
        return probe.walk(probe.page, o => typeof o.commit === "function" && o.row && o.row.id === rowId, [])[0] || null;
    }

    function same(a, b) {
        return JSON.stringify(a) === JSON.stringify(b);
    }

    function nacre() {
        return NacreConfig.normalize(Settings.get("nacre") || NacreConfig.defaultConfig());
    }

    // each appearance row: its control, the value to write, the nacre key it must land in
    readonly property var appearance: [
        ["bar-height", 48, "height"],
        ["section-opacity", 0.7, "opacity"],
        ["section-padding", 16, "padding"],
        ["widget-spacing", 10, "spacing"],
        ["section-gap", 20, "islandGap"],
        ["section-size", 0.8, "islandScale"],
        ["popup-size", 0.75, "osdScale"],
        ["workspace-markers", "kanji", "workspaceStyle"],
        ["used-workspaces", false, "occupiedWorkspaces"],
        ["mark-opens", "quicksettings", "brandClick"],
        ["nacre-frame", true, "frame"],
        ["nacre-frame-size", 14, "frameSize"],
        ["nacre-frame-corners", 18, "frameRoundness"],
        ["edge-softness", 20, "edgeMelt"]
    ]

    function checkLanes() {
        const lanes = probe.laneEditor();
        if (!probe.require(lanes, "the page shows no lane editor"))
            return;
        const start = probe.nacre();
        const id = start.islands.left[0];
        probe.require(!!id, "the left section starts with a widget");
        const want = NacreConfig.move(start, id, "left", "right", 1);
        lanes.moved(id, "left", 0, "right", 1);
        const moved = probe.nacre();
        probe.require(probe.same(moved.islands, want.islands), "moving " + id + " to the right stores " + JSON.stringify(want.islands) + ", got " + JSON.stringify(moved.islands));
        probe.require(moved.islands.left.indexOf(id) < 0 && moved.islands.right[1] === id, "the move leaves its source and lands at index 1");
        const right = lanes.lanes.find(l => l.id === "right");
        probe.require(right && right.items[1] === id, "the right lane shows the moved widget");

        lanes.moved(id, "right", 1, "", -1);
        const off = probe.nacre();
        probe.require(off.islands.left.concat(off.islands.center, off.islands.right).indexOf(id) < 0, "taking " + id + " off removes it from every section");
        probe.require(NacreConfig.unused(off).indexOf(id) >= 0 && lanes.spare.indexOf(id) >= 0, "a widget taken off shows under Not on the bar");
    }

    function checkAppearance() {
        for (const [rowId, value, key] of probe.appearance) {
            const c = probe.control(rowId);
            if (!probe.require(c, "no control for " + rowId))
                continue;
            c.commit(value);
            const got = Settings.get("nacre." + key);
            probe.require(probe.same(got, value), rowId + " writes nacre." + key + " = " + JSON.stringify(value) + ", got " + JSON.stringify(got));
        }
    }

    function openMore() {
        probe.walk(probe.page, o => o.open !== undefined && o.text === I18n.tr("More options"), []).forEach(d => d.open = true);
    }

    FloatingWindow {
        id: window
        implicitWidth: 1100
        implicitHeight: 800
        visible: true
        color: "black"

        Item { id: overlay; anchors.fill: parent; z: 10 }
    }

    // the Settings frame's members the Bar page and SettingsPage call (Hub.qml)
    QtObject {
        id: fakeHub
        property Item overlayItem: overlay
        property string sizeClass: "wide"
        property string lastSection: ""
        property real noticeInset: 0
        function navigate(page, row) {}
        function pageAvailable(key) { return true; }
        function rowAvailable(row) { return true; }
        function unhonoredReason(row) { return ""; }
        function providerRowsFor(page) { return []; }
        function confirm(options, yes, no) {}
    }

    // stages: wait for Nacre to be in use and its editor built, then lanes,
    // then the frame toggle and More options so every row is drawn, then rows
    Timer {
        id: stage
        property int step: 0
        interval: 100
        repeat: true
        running: true
        onTriggered: {
            if (++probe.waited > 150) {
                probe.fail("timed out at step " + stage.step + (probe.page ? " (style in use: " + probe.page.activeName + ", editor: " + probe.page.editor + ", lane editor: " + !!probe.laneEditor() + ", rows: " + probe.walk(probe.page, o => typeof o.commit === "function" && !!o.row, []).map(o => o.row.id).join(" ") + ", lanes-like: " + probe.walk(probe.page, o => o.lanes !== undefined, []).length + ")" : ""));
                probe.finish();
                return;
            }
            if (stage.step === 0) {
                // the daemon's first settings frame, empty: every nacre key reads its default
                Settings.apply({});
                const c = Qt.createComponent(Qt.resolvedUrl("pages/Bar.qml"));
                if (c.status !== Component.Ready) {
                    probe.fail("Bar does not load: " + c.errorString());
                    probe.finish();
                    return;
                }
                probe.page = c.createObject(window.contentItem, { hub: fakeHub, width: 1100, height: 800 });
                stage.step = 1;
            } else if (stage.step === 1) {
                if (probe.page.editor !== "nacre" || !probe.laneEditor())
                    return;
                probe.checkLanes();
                const frame = probe.control("nacre-frame");
                if (probe.require(frame, "no control for nacre-frame"))
                    frame.commit(true);
                probe.openMore();
                stage.step = 2;
            } else if (stage.step === 2) {
                if (!probe.control("nacre-frame-size") || !probe.control("edge-softness"))
                    return;
                probe.checkAppearance();
                probe.finish();
            }
        }
    }
}
