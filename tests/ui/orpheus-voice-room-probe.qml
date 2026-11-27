import QtQuick
import Quickshell
import shell.services

// Checks the voice faces of the Orpheus bar style against the feedback loop
// that froze the shell (#985): the notice face clamped its text width to
// scene.textRoom, which the bar's own overflow sets, so the two values chased
// each other forever. The faces must therefore size from their own text alone.
//
// The probe builds each face against a stub scene whose textRoom moves, and
// fails when a face's text width follows it. It runs headless: the voice faces
// need no Wayland surface, only the shell's QML import path.
ShellRoot {
    id: probe

    readonly property string repo: Quickshell.env("RYOKU_REPO") || "/repo"
    // Assigning Media.player below makes the Music service push the fake track
    // to ryoku-shell, so the probe refuses to run unless the wrapper has put it
    // in a runtime directory with no daemon socket.
    readonly property bool isolated: Quickshell.env("RYOKU_PROBE_ISOLATED") === "1"
    readonly property url noticeUrl: probe.repo
        + "/ryoku/shell/quickshell/shell/modules/bar/barstyles/orpheus/voice/VoiceNotice.qml"
    readonly property url mediaUrl: probe.repo
        + "/ryoku/shell/quickshell/shell/modules/bar/barstyles/orpheus/voice/VoiceMedia.qml"

    // The scene contract the faces used to read from.
    QtObject {
        id: stubScene
        property real textRoom: 64
        function registerTextModule(key, item) { }
        function unregisterTextModule(key, item) { }
    }

    // An empty summary falls back to "Notification", an empty body leaves the
    // second line out, and both are measured here.
    // VoiceMedia takes its text from Media.player instead, so the media checks
    // hand it a fixed player. Leaving the service alone would tie the test to
    // whatever happens to be playing, and a box with no player at all would
    // measure an empty title and never see the loop.
    QtObject {
        id: stubPlayer
        property string trackTitle: "A track title long enough to exceed every cap we could set"
        property var trackArtists: ["An artist name that is also long"]
        property string trackArtUrl: ""
        property bool isPlaying: false
        property real length: 0
        property real position: 0
        property var metadata: ({})
    }
    readonly property var notices: [
        { id: "empty", summary: "", body: "" },
        { id: "summary-only", summary: "Download complete", body: "" },
        { id: "typical", summary: "System update", body: "3 packages upgraded" },
        { id: "long", summary: "A summary long enough to exceed any cap we could set here", body: "A body line that is also extremely long, far beyond any reasonable bar width" },
        { id: "markup", summary: "<b>Bold</b> summary", body: "<i>Body</i> with an anchor" },
        { id: "newlines", summary: "Summary", body: "first line\nsecond line\nthird line" }
    ]
    // Every textRoom the scene can hand a face, from the floor to far past the
    // preference caps.
    readonly property var rooms: [64, 190, 240, 1000]

    property var steps: []
    property int step: 0
    property int failures: 0
    property var pending: null
    property var last: null

    function fail(what) {
        probe.failures += 1;
        console.log("ORPHEUS-VOICE-ROOM-PROBE-FAIL: " + what);
    }

    function buildSteps() {
        const s = [];
        const notices = probe.notices;
        ["notice", "media"].forEach(function (kind) {
            for (let i = 0; i < notices.length; ++i) {
                s.push(function () { probe.last = null; });
                for (let r = 0; r < probe.rooms.length; ++r) {
                    s.push(function () { probe.create(kind, notices[i], probe.rooms[r]); });
                    s.push(function () { probe.readBack(); });
                }
            }
        });
        s.push(function () { Media.player = null; });
        return s;
    }

    function create(kind, notice, room) {
        const component = Qt.createComponent(kind === "notice"
            ? probe.noticeUrl : probe.mediaUrl);
        if (component.status !== Component.Ready) {
            fail(kind + " face did not compile: " + component.errorString());
            return;
        }
        stubScene.textRoom = room;
        const properties = { scene: stubScene };
        if (kind === "notice")
            properties.notification = notice;
        else
            Media.player = stubPlayer;
        const created = component.createObject(probe, properties);
        if (!created) {
            fail(kind + "[" + notice.id + "] could not be created");
            return;
        }
        probe.pending = { kind: kind, notice: notice.id, room: room, item: created };
    }

    function readBack() {
        const held = probe.pending;
        probe.pending = null;
        if (!held)
            return;
        const item = held.item;
        const label = held.kind + "[" + held.notice + "]@scene.textRoom=" + held.room;
        const name = held.kind === "notice" ? "textWidth" : "titleWidth";
        const width = item[name];
        if (!Number.isFinite(width) || width < 1)
            fail(label + " " + name + " is " + width);
        if (!Number.isFinite(item.implicitWidth))
            fail(label + " implicitWidth is " + item.implicitWidth);
        if (probe.last && Math.abs(probe.last - width) > 0.001)
            fail(label + " " + name + " followed scene.textRoom: " + probe.last
                + " then " + width);
        probe.last = width;
        item.destroy();
    }

    Component.onCompleted: {
        if (!probe.isolated) {
            console.log("ORPHEUS-VOICE-ROOM-PROBE-FAIL: run orpheus-voice-room-probe.sh,"
                + " which isolates the runtime directory");
            Qt.exit(2);
            return;
        }
        probe.steps = probe.buildSteps();
        runner.start();
    }

    Timer {
        id: runner
        interval: 60
        repeat: true
        onTriggered: {
            if (probe.step < probe.steps.length) {
                probe.steps[probe.step]();
                probe.step += 1;
                return;
            }
            runner.stop();
            if (probe.failures === 0) {
                console.log("ORPHEUS-VOICE-ROOM-PROBE-PASS");
            } else {
                console.log("ORPHEUS-VOICE-ROOM-PROBE-FAIL: " + probe.failures
                    + " failure(s)");
            }
            Qt.quit();
        }
    }

    Timer {
        interval: 30000
        running: true
        onTriggered: {
            console.log("ORPHEUS-VOICE-ROOM-PROBE-FAIL: timed out after "
                + probe.step + " of " + probe.steps.length + " steps");
            Qt.exit(2);
        }
    }
}
