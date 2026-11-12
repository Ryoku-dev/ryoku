import QtQuick
import Quickshell
import Quickshell.Wayland
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/backup-import"
import "../schema/backup-import.js" as Schema

// import another desktop's configuration, carry your setup, reset everything
Item {
    id: root

    required property var hub
    readonly property string home: Quickshell.env("HOME") || ""
    readonly property var setupBind: ({ store: "setup", key: "applied" })
    readonly property var resetBind: ({ store: "reset", key: "state" })
    property string pendingRow: ""
    // the import steps, while drawn
    property ImportSteps steps: null

    // the setup job: export (capture, then write to a folder) or import (read a folder)
    property string job: ""
    property string jobError: ""
    property string exportedTo: ""
    property string exportFolder: ""
    property string readFolder: ""
    // the step that failed last, and whether its Retry asks for another folder
    property string failedJob: ""
    property bool retryPicks: false

    function tilde(p) {
        p = String(p || "");
        return root.home.length > 0 && p.indexOf(root.home) === 0 ? "~" + p.slice(root.home.length) : p;
    }

    function localPath(url) {
        var s = String(url || "");
        return decodeURIComponent(s.indexOf("file://") === 0 ? s.slice(7) : s);
    }

    readonly property var schema: {
        var rows = Schema.rows.filter(r => {
            if (r.template === true)
                return false;
            if (r.section === "import" && !flow.supported)
                return false;
            if (r.id === "import-detected" && !(flow.found && flow.step === "source"))
                return false;
            if ((r.id === "import-folder" || r.id === "import-git") && flow.step !== "source")
                return false;
            return true;
        }).map(r => {
            if (r.id === "import-detected")
                return Object.assign({}, r, { detail: I18n.tr("Your %1 configuration in %2").replace("%1", flow.providerName).replace("%2", root.tilde(flow.detected.path)) });
            if (r.id === "import-setup")
                return Object.assign({}, r, { bind: root.setupBind });
            if (r.id === "reset-all")
                return Object.assign({}, r, { bind: root.resetBind });
            return r;
        });
        return { title: Schema.title, lead: Schema.lead, sections: Schema.sections, related: Schema.related, rows: rows };
    }

    readonly property string stateLine: flow.step === "done" && !flow.undone ? I18n.tr("An import was applied; it can be undone below.")
        : Changes.value(root.resetBind) === "defaults" ? I18n.tr("All settings were reset this session.")
        : ""

    function sectionIndex(id) {
        for (var i = 0; i < page.layout.length; i++)
            if (page.layout[i].id === id)
                return i;
        return -1;
    }

    // a row the page draws itself, or one hidden by the step, lands on its section
    function focusRow(rowId) {
        if (!rowId)
            return;
        // the found row exists only once the import has looked; a landing on it waits for that
        if (page.layout.length === 0 || (rowId === "import-detected" && !flow.detectDone)) {
            root.pendingRow = rowId;
            return;
        }
        if (page.sectionOfRow(rowId)) {
            page.focusRow(rowId);
            return;
        }
        // nothing was found on this computer: the import starts from a folder
        if (rowId === "import-detected" && page.sectionOfRow("import-folder")) {
            page.focusRow("import-folder");
            return;
        }
        // a source asked for once an import is done: the way back to choosing one, after the
        // landing hands the keyboard to the page
        var another = root.steps ? root.steps.another : null;
        if (["import-detected", "import-folder", "import-git"].indexOf(rowId) >= 0 && flow.step === "done" && another && another.visible) {
            page.reveal(another);
            Qt.callLater(() => another.forceActiveFocus(Qt.TabFocusReason));
            return;
        }
        var row = Schema.rows.find(r => r.id === rowId);
        // a deep link may name a section rather than a row
        var at = root.sectionIndex(row ? row.section : rowId);
        if (at >= 0)
            page.chooseSection(at);
    }

    function scan(source) {
        flow.choose(source);
        var at = root.sectionIndex("import");
        if (at >= 0)
            page.chooseSection(at);
    }

    function exportSetup(folder) {
        root.jobError = "";
        root.exportedTo = "";
        root.exportFolder = folder;
        var name = I18n.tr("My setup %1").replace("%1", Qt.formatDate(new Date(), "yyyy-MM-dd"));
        root.runJob("export", ["ryoku-hub", "rice", "capture", name, "all"]);
    }

    function readSetup(folder) {
        root.jobError = "";
        root.exportedTo = "";
        root.readFolder = folder;
        root.runJob("read", ["ryoku-hub", "rice", "import", folder]);
    }

    // Retry runs the failed step again on the same folder, or asks for another folder when
    // the one chosen was not a setup; the error stays up while the folder dialog is open, so
    // closing the dialog leaves the keys on Retry
    function retryJob() {
        if (root.retryPicks) {
            folderPick.choose(root.failedJob === "read" ? "setup" : "export");
            return;
        }
        if (root.failedJob === "read")
            root.readSetup(root.readFolder);
        else
            root.exportSetup(root.exportFolder);
    }

    // a minute is ample for either step; a hang ends as an error with Retry
    function runJob(job, argv) {
        root.job = job;
        Changes.run(argv, function (ok, out, err, code) {
            if (code === -1)
                root.jobStopped(argv, err);
            else
                root.jobDone(code, out, err);
        }, { timeout: 60000 });
    }

    function jobStopped(argv, err) {
        var job = root.job;
        root.job = "";
        root.failedJob = job;
        root.retryPicks = false;
        root.jobError = err === Changes.missingText(argv[0])
            ? I18n.tr("The setup tool is missing from this Ryoku install. Update Ryoku, then try again.")
            : job === "read" ? I18n.tr("Opening the setup took too long, so Settings stopped waiting. Try again.")
            : I18n.tr("Exporting your setup took too long, so Settings stopped waiting. Try again.");
    }

    function useSetup(rice) {
        var name = rice.name || rice.slug;
        root.hub.confirm({
            title: I18n.tr("Use %1?").replace("%1", name),
            text: I18n.tr("Its look and settings replace yours: the bar, panels, launcher, widgets, window style, colours and wallpaper. Your setup now is kept, so Undo puts it back."),
            actionText: I18n.tr("Use it"),
            destructive: false
        }, () => Changes.set(root.setupBind, rice.slug, I18n.tr("Setup"), "backup-import", I18n.tr("Now using %1").replace("%1", name)));
    }

    function confirmReset() {
        root.hub.confirm({
            title: I18n.tr("Reset all settings?"),
            text: I18n.tr("Everything you set for the desktop goes back to how Ryoku ships it: the bar and its style, panels, launcher, notifications, sounds, fonts and motion. The window manager's settings go back too: window style and behaviour, your keyboard shortcuts, startup apps, app rules, keyboard and mouse. Your wallpaper and colours, language and formats, displays, location and time zone, accounts and files stay as they are. Undo is offered right after."),
            actionText: I18n.tr("Reset"),
            destructive: true
        }, () => Changes.set(root.resetBind, "defaults", I18n.tr("All settings"), "backup-import", I18n.tr("All settings are back to how Ryoku ships them")));
    }

    function jobDone(code, out, err) {
        var job = root.job;
        root.failedJob = job;
        root.retryPicks = false;
        if (code !== 0) {
            root.job = "";
            root.retryPicks = job === "read";
            var why = String(err).trim().split("\n").pop().replace(/^ryoku-hub:\s*/, "").replace(/\.+$/, "");
            root.jobError = (job === "read" ? I18n.tr("That folder could not be opened as a setup: %1. Choose the folder an export made, then try again.")
                                            : I18n.tr("Your setup could not be exported: %1. Check that the folder can be written to, then try again."))
                .replace("%1", why || I18n.tr("it stopped early"));
            return;
        }
        var d = null;
        try { d = JSON.parse(out); } catch (e) { d = null; }
        if (job === "export" && d && d.slug) {
            root.runJob("write", ["ryoku-hub", "rice", "export", d.slug, root.exportFolder]);
            return;
        }
        root.job = "";
        if (job === "write")
            root.exportedTo = d && d.path ? String(d.path) : root.exportFolder;
        else if (job === "read" && d && d.slug)
            root.useSetup(d);
        else
            root.jobError = I18n.tr("The setup tool answered with something Settings could not read. Try again.");
    }

    Component.onCompleted: {
        Changes.ensureStore("setup", Qt.resolvedUrl("../components/backup-import/SetupStore.qml"));
        Changes.ensureStore("reset", Qt.resolvedUrl("../components/backup-import/ResetStore.qml"));
    }

    ImportFlow {
        id: flow
        onDetectDoneChanged: {
            if (flow.detectDone && root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }
    readonly property var importer: flow

    FolderChooser {
        id: folderPick
        property string purpose: ""
        overlayParent: root.hub.overlayItem
        title: purpose === "export" ? I18n.tr("Export your setup into") : purpose === "setup" ? I18n.tr("Open a saved setup") : I18n.tr("Choose a configuration folder")
        currentFolder: "file://" + root.home + (purpose === "config" ? "/.config" : "")
        onAccepted: {
            var path = root.localPath(folderPick.selectedFolder);
            if (purpose === "export")
                root.exportSetup(path);
            else if (purpose === "setup")
                root.readSetup(path);
            else
                root.scan(path);
        }
        function choose(what) {
            folderPick.purpose = what;
            folderPick.open();
        }
    }

    // a field moving a clashing shortcut holds the window manager's keys
    property Item recordingField: null
    function noteRecording(field, on) {
        if (on) {
            if (root.recordingField === null && Settings.supports("submap"))
                Settings.send("wm.act", { action: "submap.enter", args: ["record"] });
            root.recordingField = field;
            recordTimeout.restart();
        } else if (root.recordingField === field) {
            root.recordingField = null;
            recordTimeout.stop();
            if (Settings.supports("submap"))
                Settings.send("wm.act", { action: "submap.reset", args: [] });
        }
    }
    Timer {
        id: recordTimeout
        interval: 15000
        onTriggered: if (root.recordingField) root.recordingField.cancelRecording()
    }
    ShortcutInhibitor {
        window: root.QsWindow ? root.QsWindow.window : null
        enabled: root.recordingField !== null
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "backup-import"
        schema: root.schema
        stateLine: root.stateLine
        controls: ({
            "import-detected": detectedControl,
            "import-folder": folderControl,
            "import-git": gitControl,
            "export-setup": exportControl,
            "import-setup": setupControl,
            "reset-all": resetControl
        })
        footers: ({ import: importFooter, setup: setupFooter })
        onLayoutChanged: {
            if (root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    // a folder dropped anywhere on the page is looked through
    DropArea {
        anchors.fill: parent
        enabled: flow.supported && flow.busy === ""
        onDropped: drop => {
            if (drop.hasUrls && drop.urls.length > 0)
                root.scan(root.localPath(drop.urls[0]));
        }
    }

    Component {
        id: detectedControl
        Plaque {
            function activate() { root.scan(flow.detected.path); }
            kind: "line"
            small: true
            glyph: "search"
            text: I18n.tr("Look through it")
            enabled: flow.busy === ""
            onClicked: activate()
        }
    }

    Component {
        id: folderControl
        Plaque {
            kind: "quiet"
            small: true
            glyph: "folder_open"
            text: I18n.tr("Choose a folder")
            enabled: flow.busy === ""
            onClicked: folderPick.choose("config")
        }
    }

    Component {
        id: gitControl
        TextField {
            width: LithicTokens.size.settingsControlMax
            mono: true
            placeholderText: "https://github.com/you/dotfiles.git"
            enabled: flow.busy === ""
            Accessible.name: I18n.tr("Git repository address")
            onAccepted: root.scan(text)
        }
    }

    Component {
        id: exportControl
        Plaque {
            function activate() { if (root.job === "") folderPick.choose("export"); }
            kind: "quiet"
            small: true
            glyph: "upload"
            text: I18n.tr("Export")
            busy: root.job === "export" || root.job === "write"
            enabled: root.job === ""
            onClicked: activate()
        }
    }

    Component {
        id: setupControl
        Plaque {
            function activate() { if (root.job === "") folderPick.choose("setup"); }
            kind: "quiet"
            small: true
            glyph: "download"
            text: I18n.tr("Choose a folder")
            busy: root.job === "read"
            enabled: root.job === ""
            onClicked: activate()
        }
    }

    Component {
        id: resetControl
        Plaque {
            function activate() { root.confirmReset(); }
            kind: "dangerLine"
            small: true
            text: I18n.tr("Reset")
            enabled: Changes.value(root.resetBind) !== "defaults"
            onClicked: activate()
        }
    }

    Component {
        id: importFooter
        Column {
            spacing: LithicTokens.space.s3
            Label {
                width: parent.width
                visible: !flow.supported
                text: I18n.tr("No reader for other desktops' configuration is installed. A window manager that can import one adds it.")
                typeRole: "detail"
                inkRole: "fg3"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            WorkLine {
                width: parent.width
                active: flow.busy !== ""
                text: flow.busy === "scan" ? I18n.tr("Looking through %1").replace("%1", root.tilde(flow.lastSource))
                    : flow.busy === "apply" ? I18n.tr("Backing up and applying")
                    : I18n.tr("Putting the backup back")
                cancellable: flow.busy === "scan"
                error: flow.error
                onCancelled: flow.cancel()
                onRetried: flow.retry()
            }
            ImportSteps {
                id: importSteps
                width: parent.width
                visible: flow.step !== "source"
                flow: root.importer
                hub: root.hub
                onRecording: (field, on) => root.noteRecording(field, on)
                onLeftSteps: page.focusRow("import-folder")
                Component.onCompleted: root.steps = importSteps
                Component.onDestruction: {
                    if (root.steps === importSteps)
                        root.steps = null;
                }
            }
        }
    }

    Component {
        id: setupFooter
        Column {
            spacing: LithicTokens.space.s2
            WorkLine {
                width: parent.width
                active: root.job !== ""
                text: root.job === "read" ? I18n.tr("Opening the setup") : I18n.tr("Exporting your setup")
                error: root.jobError
                onRetried: root.retryJob()
            }
            Row {
                visible: root.exportedTo.length > 0
                width: parent.width
                spacing: LithicTokens.space.s3
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.min(implicitWidth, parent.width - showFolder.width - parent.spacing)
                    text: I18n.tr("Exported to %1").replace("%1", root.tilde(root.exportedTo))
                    typeRole: "detail"
                    elide: Text.ElideMiddle
                }
                Plaque {
                    id: showFolder
                    anchors.verticalCenter: parent.verticalCenter
                    kind: "quiet"
                    small: true
                    glyph: "open_in_new"
                    text: I18n.tr("Show the folder")
                    onClicked: Spawn.runApp(["xdg-open", root.exportedTo], { label: "files" })
                }
            }
        }
    }
}
