import QtQuick
import Quickshell
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/screen-capture"
import "../schema/screen-capture.js" as Schema

// recording, screenshots, the key press overlay and whether apps can share the screen
Item {
    id: root

    required property var hub
    property string pendingRow: ""

    readonly property string recStatus: Changes.value({ store: "recording", key: "status" }) || "loading"
    readonly property string recError: Changes.value({ store: "recording", key: "error" }) || ""

    // what the recorder can do on this machine, asked once on open
    property bool probeDone: false
    property bool recorderAvailable: true
    property var codecs: ["h264", "hevc", "av1"]
    property var encoders: ["gpu", "cpu"]
    property string infoBackend: ""
    property string infoVersion: ""
    property string infoEncoder: ""

    // whether an app asking to share the screen gets a picker
    property bool shareKnown: false
    property bool shareReady: false
    property bool repairing: false
    property bool repairCancelled: false
    property string repairError: ""

    readonly property string home: Quickshell.env("HOME") || ""
    readonly property string videosRoot: Quickshell.env("XDG_VIDEOS_DIR") || root.home + "/Videos"
    readonly property string picturesRoot: Quickshell.env("XDG_PICTURES_DIR") || root.home + "/Pictures"
    readonly property string videosDir: root.videosRoot + "/Recordings"
    readonly property string shotsDir: root.picturesRoot + "/Screenshots"

    readonly property bool recordingShown: root.recStatus === "ready" && !(root.probeDone && !root.recorderAvailable)

    function prepared(row) {
        var out = Object.assign({}, row);
        switch (row.id) {
        case "rec-codec":
            out.options = row.options.filter(o => root.codecs.indexOf(o.value) >= 0);
            break;
        case "rec-encoder":
            out.options = row.options.filter(o => root.encoders.indexOf(o.value) >= 0);
            break;
        case "rec-audio-bitrate":
            // a rate set by hand elsewhere stays choosable
            var now = Changes.value(row.bind);
            if (typeof now === "number" && !row.options.some(o => o.value === now))
                out.options = row.options.concat([{ label: I18n.tr("%1 kbps").arg(now), value: now }]);
            break;
        case "rec-engine":
            out.detail = I18n.tr("%1, encoding with %2")
                .arg(root.infoBackend === "gsr" ? I18n.tr("GPU Screen Recorder") + (root.infoVersion ? " " + root.infoVersion : "") : root.infoBackend)
                .arg(root.infoEncoder || I18n.tr("the graphics card"));
            break;
        case "key-sample":
            out.detail = root.keypress.showing ? I18n.tr("Drag the sample keycaps on the desktop to where they should appear, then hide it")
                                               : I18n.tr("Show a sample on the desktop and drag it where it should appear");
            break;
        case "share-picker":
            out.detail = root.repairing ? I18n.tr("Running the system check that repairs screen sharing")
                : root.shareReady ? I18n.tr("Apps can ask which screen or window to share")
                : I18n.tr("Apps cannot offer a screen picker, so sharing fails without a message. Repair runs the system check that fixes it");
            break;
        }
        return out;
    }

    function shown(row) {
        var store = row.bind ? row.bind.store : "";
        if (row.section === "recording" && (store === "recording" || store === "ryoshot" || row.id === "rec-engine"))
            return root.recordingShown && (row.id !== "rec-engine" || root.infoBackend !== "");
        if (row.id === "share-picker")
            return root.shareKnown;
        return true;
    }

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: Schema.rows.filter(r => root.shown(r)).map(r => root.prepared(r))
    })

    readonly property var keypress: Changes.stores["keypress"] || ({ showing: false, backend: "disabled", backendError: "", placement: null })

    function sectionIndex(id) {
        for (var i = 0; i < page.layout.length; i++)
            if (page.layout[i].id === id)
                return i;
        return -1;
    }

    // a row hidden for now lands on its section; recording rows wait for their first read
    function focusRow(rowId) {
        if (!rowId)
            return;
        // a section id is the frame's to land on
        if (Schema.sections.some(s => s.id === rowId)) {
            page.focusRow(rowId);
            return;
        }
        var target = Schema.rows.find(r => r.id === rowId);
        var waiting = target && target.section === "recording" && (root.recStatus === "loading" || !root.probeDone);
        var undrawn = root.schema.rows.some(r => r.id === rowId) && page.landingRow(rowId) === "";
        if (page.layout.length === 0 || waiting || undrawn) {
            root.pendingRow = rowId;
            return;
        }
        if (page.landingRow(rowId) !== "") {
            page.focusRow(rowId);
            return;
        }
        var row = Schema.rows.find(r => r.id === rowId);
        var at = root.sectionIndex(row ? row.section : "");
        if (at >= 0)
            page.chooseSection(at);
    }

    function retryPending() {
        if (root.pendingRow === "")
            return;
        var id = root.pendingRow;
        root.pendingRow = "";
        Qt.callLater(root.focusRow, id);
    }
    onProbeDoneChanged: root.retryPending()
    onRecStatusChanged: root.retryPending()

    // the folder dialog fills whichever folder row asked for it
    property var choosing: null

    function chooseFolder(row, current) {
        root.choosing = row;
        folderDialog.title = I18n.tr(row.label);
        var at = String(current || "").trim().replace(/^~(?=\/|$)/, root.home);
        folderDialog.currentFolder = "file://" + (at.length > 0 ? at : root.home);
        folderDialog.open();
    }

    function setPath(row, path) {
        Changes.set(row.bind, String(path || "").trim(), row.label, "screen-capture");
    }

    function repair() {
        if (root.repairing)
            return;
        root.repairError = "";
        root.repairCancelled = false;
        root.repairing = true;
        repairProc.didStart = false;
        repairProc.running = true;
    }

    function cancelRepair() {
        if (!root.repairing)
            return;
        root.repairCancelled = true;
        repairProc.signal(15);
    }

    Component.onCompleted: {
        Changes.ensureStore("recording", Qt.resolvedUrl("../components/screen-capture/RecordingStore.qml"));
        Changes.ensureStore("ryoshot", Qt.resolvedUrl("../components/screen-capture/RyoshotStore.qml"));
        Changes.ensureStore("keypress", Qt.resolvedUrl("../components/screen-capture/KeypressStore.qml"));
        var rec = Changes.stores["recording"];
        if (rec && rec.status !== "loading")
            rec.reload();
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "screen-capture"
        schema: root.schema
        bodies: ({ recording: root.recordingShown ? null : recordingBody })
        footers: ({
            keypress: root.keypress.backend === "error" ? keypressFooter : null,
            privacy: root.repairError !== "" ? repairFooter : null
        })
        controls: ({
            "rec-folder": recFolderControl,
            "shot-folder": shotFolderControl,
            "key-sample": sampleControl,
            "share-picker": pickerControl
        })
        onLayoutChanged: {
            if (root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    Component {
        id: recordingBody
        Column {
            spacing: LithicTokens.space.s2
            Label {
                width: parent.width
                text: root.recStatus === "loading" ? I18n.tr("Reading your recording settings.")
                    : root.recStatus === "unavailable"
                        ? (root.recError !== "" ? I18n.tr("Your recording settings could not be read. %1").arg(root.recError)
                                                : I18n.tr("Your recording settings could not be read because the desktop is not running."))
                    : I18n.tr("Recording needs GPU Screen Recorder, which is missing or found no video encoder on this computer. Install it from your package manager to record.")
                typeRole: "body"
                ink: root.recStatus === "unavailable" ? Lithic.jasper : Lithic.fg2
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                visible: root.recStatus === "unavailable"
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: {
                    var rec = Changes.stores["recording"];
                    if (rec)
                        rec.reload();
                }
            }
        }
    }

    Component {
        id: recFolderControl
        PathField {
            property var row: null
            width: LithicTokens.size.settingsControlMax
            path: row ? (Changes.value(row.bind) || "") : ""
            placeholder: root.videosDir
            accessibleName: I18n.tr("Save recordings to")
            // a landing focuses the field itself, which is no Tab stop; its text box is
            onActiveFocusChanged: if (activeFocus) nextItemInFocusChain(true).forceActiveFocus(Qt.OtherFocusReason)
            onChooseRequested: root.chooseFolder(row, path || root.videosDir)
            onEdited: p => root.setPath(row, p)
        }
    }

    Component {
        id: shotFolderControl
        PathField {
            property var row: null
            width: LithicTokens.size.settingsControlMax
            path: row ? (Changes.value(row.bind) || "") : ""
            placeholder: root.shotsDir
            accessibleName: I18n.tr("Save screenshots to")
            // a landing focuses the field itself, which is no Tab stop; its text box is
            onActiveFocusChanged: if (activeFocus) nextItemInFocusChain(true).forceActiveFocus(Qt.OtherFocusReason)
            onChooseRequested: root.chooseFolder(row, path || root.shotsDir)
            onEdited: p => root.setPath(row, p)
        }
    }

    Component {
        id: sampleControl
        Row {
            id: sample
            property var row: null
            function activate() {
                root.keypress.showSample(!root.keypress.showing);
            }
            spacing: LithicTokens.space.s2
            // only a dragged overlay has a position to put back; Undo returns it to that spot
            Plaque {
                enabled: root.keypress.placement !== null && root.keypress.placement !== undefined
                kind: "quiet"
                small: true
                glyph: "restart_alt"
                text: I18n.tr("Reset position")
                onClicked: Changes.set(sample.row.bind, null, I18n.tr("Key press overlay position"), "screen-capture", I18n.tr("Key press overlay back in its usual place"))
            }
            Plaque {
                kind: root.keypress.showing ? "on" : "line"
                small: true
                glyph: root.keypress.showing ? "visibility_off" : "visibility"
                text: root.keypress.showing ? I18n.tr("Hide") : I18n.tr("Show on the desktop")
                onClicked: sample.activate()
            }
        }
    }

    Component {
        id: pickerControl
        Row {
            property var row: null
            spacing: LithicTokens.space.s2
            Working {
                anchors.verticalCenter: parent.verticalCenter
                visible: root.repairing && repairClock.working
                active: visible
            }
            BladeProgress {
                anchors.verticalCenter: parent.verticalCenter
                visible: root.repairing && repairClock.slow
                width: 96
                active: visible
                indeterminate: true
            }
            Plaque {
                visible: root.repairing && repairClock.slow
                enabled: !root.repairCancelled
                kind: "quiet"
                small: true
                text: root.repairCancelled ? I18n.tr("Stopping") : I18n.tr("Cancel")
                onClicked: root.cancelRepair()
            }
            Label {
                anchors.verticalCenter: parent.verticalCenter
                visible: !root.repairing && root.shareReady
                text: I18n.tr("Ready")
                typeRole: "label"
                inkRole: "fg2"
            }
            Plaque {
                visible: !root.repairing && !root.shareReady
                kind: "line"
                small: true
                glyph: "build"
                text: I18n.tr("Repair")
                onClicked: root.repair()
            }
        }
    }

    Component {
        id: keypressFooter
        Row {
            spacing: LithicTokens.space.s3
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, parent.width - retryOverlay.width - LithicTokens.space.s3)
                text: I18n.tr("The overlay could not start: %1. Press Retry to start it again.").arg(root.keypress.backendError.replace(/\.+$/, ""))
                typeRole: "detail"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                id: retryOverlay
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: root.keypress.showSample(true)
            }
        }
    }

    Component {
        id: repairFooter
        Row {
            id: repairNote
            spacing: LithicTokens.space.s3
            readonly property string note: root.repairError
            // the note sits below the section's last row, so a new one is brought into view once placed
            onNoteChanged: if (repairNote.note !== "") revealRepair.restart()
            Component.onCompleted: if (repairNote.note !== "") revealRepair.restart()
            Timer {
                id: revealRepair
                interval: 120
                onTriggered: page.reveal(repairNote)
            }
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, parent.width - retryRepair.width - LithicTokens.space.s3)
                text: root.repairError
                typeRole: "detail"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                id: retryRepair
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: root.repair()
            }
        }
    }

    FolderChooser {
        id: folderDialog
        overlayParent: root.hub.overlayItem
        onAccepted: {
            if (!root.choosing)
                return;
            var path = decodeURIComponent(String(folderDialog.selectedFolder).replace(/^file:\/\//, ""));
            root.setPath(root.choosing, path);
            root.choosing = null;
        }
        onRejected: root.choosing = null
    }

    BusyClock {
        id: repairClock
        running: root.repairing
    }

    Process {
        id: shareInfo
        command: ["ryoku-hub", "share", "status"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var j = JSON.parse(this.text);
                    root.shareKnown = j.probeable === true;
                    root.shareReady = j.available === true;
                } catch (e) {
                    root.shareKnown = false;
                }
            }
        }
    }

    Process {
        id: repairProc
        property bool didStart: false
        command: ["ryoku", "doctor"]
        stdout: StdioCollector {}
        stderr: StdioCollector { id: repairErr }
        onStarted: repairProc.didStart = true
        onExited: (code, status) => {
            root.repairing = false;
            if (root.repairCancelled)
                root.repairError = I18n.tr("Repair stopped before it finished. Screen sharing may still be broken.");
            else if (code !== 0)
                root.repairError = I18n.tr("The repair did not finish: %1. Run ryoku doctor in a terminal to see every step.")
                    .arg((repairErr.text.trim().split("\n").pop() || I18n.tr("it stopped with code %1").arg(code)).replace(/\.+$/, ""));
            shareInfo.running = true;
        }
        // a program that cannot start reports no exit, only that it stopped running
        onRunningChanged: {
            if (!repairProc.running && !repairProc.didStart && root.repairing) {
                root.repairing = false;
                root.repairError = I18n.tr("The repair could not start. %1").arg(Changes.missingText("ryoku"));
            }
        }
    }

    Process {
        id: info
        command: ["sh", "-c", "command -v ryoku-cmd-record >/dev/null && exec ryoku-cmd-record --info"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var j = JSON.parse(this.text);
                    root.infoBackend = j.backend || "";
                    root.infoEncoder = j.encoder || "";
                    root.infoVersion = j.version || "";
                } catch (e) {}
            }
        }
    }

    Process {
        id: recorderProbe
        command: ["sh", "-c", "command -v gpu-screen-recorder >/dev/null && exec gpu-screen-recorder --info"]
        running: true
        stdout: StdioCollector { id: probeOut }
        stderr: StdioCollector {}
        onExited: code => {
            var codecs = [], encoders = [];
            function add(list, value) {
                if (list.indexOf(value) < 0)
                    list.push(value);
            }
            var lines = probeOut.text.split("\n").map(l => l.trim());
            for (var i = 0; i < lines.length; i++) {
                var v = lines[i];
                if (v === "h264" || v === "h264_software")
                    add(codecs, "h264");
                else if (v === "hevc" || v === "av1")
                    add(codecs, v);
                if (v === "h264_software")
                    add(encoders, "cpu");
                else if (["h264", "hevc", "av1", "vp8", "vp9"].indexOf(v) >= 0)
                    add(encoders, "gpu");
            }
            root.codecs = codecs.length > 0 ? codecs : ["h264", "hevc", "av1"];
            root.encoders = encoders.length > 0 ? encoders : ["gpu", "cpu"];
            root.recorderAvailable = code === 0 && codecs.length > 0 && encoders.length > 0;
            root.probeDone = true;
        }
    }
}
