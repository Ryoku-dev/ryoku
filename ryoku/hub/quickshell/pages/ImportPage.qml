pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"
import "../Combos.js" as Combos

// Import config (TOOLS, ADVANCED). The drop-and-go migration wizard: bring an
// existing setup (a folder, a drop, an existing ~/.config, or a git URL) onto
// Ryoku, layer it over the defaults, resolve the keybind clashes in place, and
// keep it reversible. The engine is `ryoku-hub import` (scan/apply/undo); this
// page is one of its two front doors. It never parses config itself: scan hands
// back the model and the conflicts (each with a stable `norm` the UI echoes and
// never computes), apply takes the decisions and reports the exact change set,
// and undo restores the backup. The keybind conflict rules and the chord reader
// are the Keybinds page's, shared through Combos.js so both classify a clash and
// record a chord the same way. A full-bleed page: it draws its own head, wizard
// body and footer, since the shell hides the global action bar here.
Item {
    id: pg

    property var hub
    readonly property bool fullBleed: true

    // ── wizard state ─────────────────────────────────────────────────────────
    // one of source | review | resolve | preview | done. busy gates a spinner
    // over the whole page while scan/apply/undo run (they touch the disk and can
    // take a second). errorMsg surfaces a failed shell-out.
    property string step: "source"
    property bool busy: false
    property string busyLabel: ""
    property string errorMsg: ""

    // the parsed ScanResult { source, apps: [ App ] } and the two decision maps.
    // appInclude and conflictDecision are reassigned whole on every edit so the
    // bindings that read them refresh, the way the Keybinds editor swaps arrays.
    property var scan: null
    property var appInclude: ({})        // app id -> bool (absent = included)
    property var conflictDecision: ({})  // norm -> "ryoku" | "mine" | { remap }

    // the ApplyResult / UndoResult reported back, and the decisions we posted.
    property var applyResult: null
    property var undoResult: null
    property bool undone: false
    property string pendingDecisions: ""

    // Provider import metadata comes from the backend, which asks each installed
    // provider for its native reader and default source. The page treats provider
    // ids as opaque and never infers syntax from filenames.
    property var importSource: ({})
    property bool importProbeReady: false
    property bool autoDetected: false
    property string urlText: ""
    readonly property string home: Quickshell.env("HOME") || ""
    readonly property bool importSupported: !pg.importProbeReady || pg.importSource.supported === true
    readonly property string providerName: pg.importSource.name || ""
    readonly property string providerDefaultPath: pg.importSource.path || ""

    function providerCased() {
        var p = pg.providerName;
        return p.length ? p.charAt(0).toUpperCase() + p.slice(1) : I18n.tr("desktop config");
    }
    function runDetect() {
        if (!detectProc.running)
            detectProc.running = true;
    }

    readonly property var stepDefs: [
        { key: "source", label: I18n.tr("Source") }, { key: "review", label: I18n.tr("Review") },
        { key: "resolve", label: I18n.tr("Resolve") }, { key: "preview", label: I18n.tr("Preview") },
        { key: "done", label: I18n.tr("Done") }
    ]
    function stepIndex(k) {
        for (var i = 0; i < pg.stepDefs.length; i++)
            if (pg.stepDefs[i].key === k)
                return i;
        return 0;
    }

    // ── derived model ─────────────────────────────────────────────────────────
    readonly property var apps: (pg.scan && pg.scan.apps) ? pg.scan.apps : []
    function includeOf(id) {
        var v = pg.appInclude[id];
        return v === undefined ? true : v;
    }
    function setInclude(id, v) {
        var m = {};
        for (var k in pg.appInclude)
            m[k] = pg.appInclude[k];
        m[id] = v;
        pg.appInclude = m;
    }
    // Every conflict of an included, present app, flattened with keybinds first.
    readonly property var conflicts: {
        var out = [];
        for (var i = 0; i < pg.apps.length; i++) {
            var a = pg.apps[i];
            if (!a || a.present === false || !pg.includeOf(a.id))
                continue;
            var cs = a.conflicts || [];
            for (var j = 0; j < cs.length; j++) {
                var c = cs[j];
                out.push({ app: a.name, norm: c.norm, combo: c.combo,
                    ryoku: c.ryoku || ({}), mine: c.mine || ({}), kind: c.kind });
            }
        }
        return out;
    }
    readonly property int conflictTotal: pg.conflicts.length
    readonly property int resolvedCount: {
        var n = 0;
        for (var i = 0; i < pg.conflicts.length; i++) {
            var d = pg.conflictDecision[pg.conflicts[i].norm];
            if (d !== undefined && d !== null && d !== "")
                n++;
        }
        return n;
    }
    readonly property int includedCount: {
        var n = 0;
        for (var i = 0; i < pg.apps.length; i++) {
            var a = pg.apps[i];
            if (a && a.present !== false && pg.includeOf(a.id))
                n++;
        }
        return n;
    }
    function conflictKind(norm) {
        for (var i = 0; i < pg.conflicts.length; i++)
            if (pg.conflicts[i].norm === norm)
                return pg.conflicts[i].kind;
        return "";
    }

    // the segment a conflict's current decision maps to ("" = still on the safe
    // Ryoku default, nothing highlighted).
    function segCurrent(norm) {
        var d = pg.conflictDecision[norm];
        if (d === "ryoku") return "Keep Ryoku's";
        if (d === "mine") return "Use mine";
        if (d && d.remap) return "Remap";
        return "";
    }
    function decisionOf(norm) { return pg.conflictDecision[norm]; }
    function setDecision(norm, val) {
        var m = {};
        for (var k in pg.conflictDecision)
            m[k] = pg.conflictDecision[k];
        m[norm] = val;
        pg.conflictDecision = m;
    }
    function chooseSegment(norm, label) {
        if (label === "Keep Ryoku's") pg.setDecision(norm, "ryoku");
        else if (label === "Use mine") pg.setDecision(norm, "mine");
        else if (label === "Remap") pg.startRemap(norm);
    }
    // a remap that lands on another conflicting combo still overlaps: normalise
    // the captured chord (the one place the UI touches norm math) and compare.
    function remapClashes(norm) {
        var d = pg.conflictDecision[norm];
        if (!d || !d.remap)
            return false;
        var nk = Combos.normKeys(d.remap);
        for (var i = 0; i < pg.conflicts.length; i++)
            if (pg.conflicts[i].norm !== norm && pg.conflicts[i].norm === nk)
                return true;
        return false;
    }

    // the items an included app layers verbatim (everything that is not an
    // ingested keybind), shown read-only under "these layer on top and win".
    readonly property var layeredItems: {
        var out = [];
        for (var i = 0; i < pg.apps.length; i++) {
            var a = pg.apps[i];
            if (!a || a.present === false || !pg.includeOf(a.id))
                continue;
            var items = a.items || [];
            for (var j = 0; j < items.length; j++) {
                var it = items[j];
                if (it.kind !== "bind")
                    out.push({ app: a.name, raw: it.raw || "", kind: it.kind });
            }
        }
        return out;
    }

    function tierLabel(t) {
        return t === "deep" ? I18n.tr("Deep ingest") : (t === "layer" ? I18n.tr("Layer on top") : I18n.tr("Drop in"));
    }
    function tierNote(t) {
        if (t === "deep")
            return I18n.tr("Keybinds, rules and supported settings become Ryoku settings; anything without a typed mapping stays in the provider's user include.");
        if (t === "layer")
            return I18n.tr("Layered into this app's override file, which already wins over the shipped config.");
        return I18n.tr("Dropped into this app's override slot, clearly labelled, applied as-is.");
    }

    // ── the forecast shown on Preview, built from the scan model + decisions ───
    // the authoritative numbers come back in the ApplyResult on Done; this is the
    // before-you-commit estimate the wizard promises.
    readonly property var forecast: {
        var binds = 0, rules = 0, unbinds = 0;
        for (var i = 0; i < pg.apps.length; i++) {
            var a = pg.apps[i];
            if (!a || a.present === false || !pg.includeOf(a.id))
                continue;
            var items = a.items || [];
            for (var j = 0; j < items.length; j++) {
                var it = items[j];
                if (it.kind === "bind" && it.ingestable) binds++;
                else if (it.kind === "windowrule" && it.ingestable) rules++;
            }
        }
        for (var norm in pg.conflictDecision)
            if (pg.conflictDecision[norm] === "mine" && pg.conflictKind(norm) === "shipped")
                unbinds++;
        return { binds: binds, rules: rules, unbinds: unbinds };
    }

    // ── driving the engine ─────────────────────────────────────────────────────
    function stripFile(u) {
        var s = "" + u;
        if (s.indexOf("file://") === 0)
            s = s.slice(7);
        return decodeURIComponent(s);
    }
    function tildePath(p) {
        return (pg.home && ("" + p).indexOf(pg.home) === 0) ? "~" + ("" + p).substring(pg.home.length) : ("" + p);
    }
    function chooseSource(arg) {
        arg = ("" + (arg || "")).trim();
        if (!arg)
            return;
        pg.errorMsg = "";
        pg.scan = null;
        pg.applyResult = null;
        pg.undone = false;
        pg.appInclude = ({});
        pg.conflictDecision = ({});
        pg.busy = true;
        pg.busyLabel = I18n.tr("Scanning your config");
        scanProc.command = ["ryoku-hub", "import", "scan", arg];
        scanProc.running = true;
    }
    function buildDecisions() {
        var apps = {};
        for (var i = 0; i < pg.apps.length; i++) {
            var a = pg.apps[i];
            if (a && a.present !== false)
                apps[a.id] = { include: pg.includeOf(a.id) };
        }
        var conflicts = {};
        for (var norm in pg.conflictDecision) {
            var d = pg.conflictDecision[norm];
            if (d !== undefined && d !== null && d !== "")
                conflicts[norm] = d;
        }
        return JSON.stringify({ source: pg.scan ? pg.scan.source : "", apps: apps, conflicts: conflicts });
    }
    function apply() {
        pg.errorMsg = "";
        pg.pendingDecisions = pg.buildDecisions();
        pg.busy = true;
        pg.busyLabel = I18n.tr("Applying and backing up");
        applyProc.stdinEnabled = true;
        applyProc.command = ["ryoku-hub", "import", "apply", "-"];
        applyProc.running = true;
    }
    function undo() {
        pg.errorMsg = "";
        pg.busy = true;
        pg.busyLabel = I18n.tr("Undoing the import");
        undoProc.command = ["ryoku-hub", "import", "undo"];
        undoProc.running = true;
    }
    function reloadDesktop() { if (pg.hub) pg.hub.wmAct("config.reload"); }
    function restart() {
        pg.scan = null;
        pg.applyResult = null;
        pg.undoResult = null;
        pg.undone = false;
        pg.errorMsg = "";
        pg.appInclude = ({});
        pg.conflictDecision = ({});
        pg.step = "source";
    }

    function goNext() {
        if (pg.step === "review") pg.step = (pg.conflictTotal > 0 ? "resolve" : "preview");
        else if (pg.step === "resolve") pg.step = "preview";
        else if (pg.step === "preview") pg.apply();
    }
    function goBack() {
        if (pg.step === "review") pg.step = "source";
        else if (pg.step === "resolve") pg.step = "review";
        else if (pg.step === "preview") pg.step = (pg.conflictTotal > 0 ? "resolve" : "review");
    }
    function primaryLabel() {
        if (pg.step === "review") return I18n.tr("Continue");
        if (pg.step === "resolve") return I18n.tr("Preview changes");
        if (pg.step === "preview") return I18n.tr("Apply import");
        return "";
    }
    readonly property bool footerVisible: pg.step === "review" || pg.step === "resolve" || pg.step === "preview"

    Component.onCompleted: pg.runDetect()

    Process {
        id: detectProc
        running: false
        command: ["ryoku-hub", "import", "detect"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    pg.importSource = JSON.parse(this.text.trim() || "{}");
                } catch (e) {
                    pg.importSource = ({});
                }
                pg.autoDetected = pg.importSource.found === true;
                pg.importProbeReady = true;
            }
        }
        onExited: (code) => {
            if (code !== 0)
                pg.importProbeReady = true;
        }
    }
    Process {
        id: scanProc
        running: false
        stdout: StdioCollector {
            id: scanOut
            onStreamFinished: {
                var t = this.text.trim();
                if (t.length === 0)
                    return;
                try {
                    pg.scan = JSON.parse(t);
                    pg.step = "review";
                } catch (e) {
                    pg.errorMsg = I18n.tr("Could not read the scan result.");
                }
                pg.busy = false;
            }
        }
        stderr: StdioCollector { id: scanErr }
        onExited: (code) => {
            if (code !== 0) {
                pg.errorMsg = scanErr.text.trim() || I18n.tr("Scan failed (exit %1).").arg(code);
                pg.busy = false;
            }
        }
    }
    Process {
        id: applyProc
        running: false
        stdinEnabled: true
        stdout: StdioCollector {
            id: applyOut
            onStreamFinished: {
                var t = this.text.trim();
                if (t.length === 0)
                    return;
                try {
                    pg.applyResult = JSON.parse(t);
                    pg.step = "done";
                } catch (e) {
                    pg.errorMsg = I18n.tr("Could not read the apply result.");
                }
                pg.busy = false;
            }
        }
        stderr: StdioCollector { id: applyErr }
        onStarted: {
            write(pg.pendingDecisions);
            stdinEnabled = false;
        }
        onExited: (code) => {
            if (code !== 0) {
                pg.errorMsg = applyErr.text.trim() || I18n.tr("Import failed (exit %1).").arg(code);
                pg.busy = false;
            }
        }
    }
    Process {
        id: undoProc
        running: false
        stdout: StdioCollector {
            id: undoOut
            onStreamFinished: {
                var t = this.text.trim();
                if (t.length === 0)
                    return;
                try {
                    pg.undoResult = JSON.parse(t);
                    pg.undone = true;
                } catch (e) {
                    pg.errorMsg = I18n.tr("Could not read the undo result.");
                }
                pg.busy = false;
            }
        }
        stderr: StdioCollector { id: undoErr }
        onExited: (code) => {
            if (code !== 0) {
                pg.errorMsg = undoErr.text.trim() || I18n.tr("Undo failed (exit %1).").arg(code);
                pg.busy = false;
            }
        }
    }

    // ── reusable poster parts ─────────────────────────────────────────────────
    // a hairline status chip: mono caps, tinted to its meaning.
    component Pill: Rectangle {
        id: pill
        property string label: ""
        property color tint: Tokens.inkMuted
        implicitWidth: pillT.implicitWidth + Tokens.s3
        implicitHeight: 20
        radius: Tokens.radius
        color: "transparent"
        border.width: Tokens.border
        border.color: Qt.rgba(pill.tint.r, pill.tint.g, pill.tint.b, 0.5)
        Text {
            id: pillT
            anchors.centerIn: parent
            text: I18n.tr(pill.label)
            color: pill.tint
            font.family: Tokens.mono
            font.pixelSize: Tokens.fTiny
            font.letterSpacing: 1
        }
    }
    // a bordered plate the wizard cards are built on.
    component Plate: Rectangle {
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.line
        radius: Tokens.radius
    }

    // ── head: eyebrow, Fraunces title, blurb (matches every settings page) ─────
    Column {
        id: head
        anchors { left: parent.left; right: parent.right; top: parent.top }
        anchors.topMargin: Tokens.s6
        // the register row sits off the title: a rule over a 32px
        // title needs more than the gap between two lines of body text
        spacing: Tokens.s3

        Row {
            // the register row holds a fixed box, so the rule and the seal keep
            // their distance from the title on every page
            height: Tokens.s5
            spacing: Tokens.s2
            Rectangle {
                width: 16; height: 1; color: Tokens.ink
                anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: "力"; color: Tokens.ink; font.family: Tokens.jp
                font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: I18n.tr("TOOLS"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Text {
            text: I18n.tr("Import config"); color: Tokens.ink
            font.family: Tokens.display; font.pixelSize: Tokens.fTitle
        }
        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("Bring another setup onto Ryoku. Everything it touches is backed up.")
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
        }
    }

    // ── the step rail: where you are in the five-step wizard ───────────────────
    Row {
        id: rail
        visible: pg.importSupported
        anchors.left: parent.left
        anchors.leftMargin: Tokens.s6
        anchors.top: head.bottom
        anchors.topMargin: Tokens.s5
        spacing: Tokens.s4

        Repeater {
            model: pg.stepDefs
            delegate: Row {
                id: stepPip
                required property var modelData
                required property int index
                readonly property bool active: pg.step === stepPip.modelData.key
                readonly property bool done: pg.stepIndex(pg.step) > stepPip.index
                // a little air around the hairline connectors, so the numbers and
                // labels do not sit on the line that joins them
                spacing: Tokens.s3
                // a hairline between the steps, so the row reads as one path
                // walked left to right rather than five loose labels
                Rectangle {
                    visible: stepPip.index > 0
                    width: 20; height: 1
                    color: stepPip.active || stepPip.done ? Tokens.line : Tokens.lineSoft
                    anchors.verticalCenter: parent.verticalCenter
                }
                Text {
                    text: (stepPip.index + 1)
                    color: stepPip.active ? Tokens.sun : (stepPip.done ? Tokens.inkDim : Tokens.inkFaint)
                    font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                    anchors.verticalCenter: parent.verticalCenter
                }
                Text {
                    text: I18n.tr(stepPip.modelData.label)
                    color: stepPip.active ? Tokens.ink : (stepPip.done ? Tokens.inkDim : Tokens.inkMuted)
                    font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                    font.weight: stepPip.active ? Font.Medium : Font.Normal
                    font.letterSpacing: Tokens.trackLabel
                    anchors.verticalCenter: parent.verticalCenter
                }
            }
        }
    }

    // ── the wizard body: a Loader swaps the step subtree ───────────────────────
    Loader {
        id: body
        anchors {
            left: parent.left; right: parent.right
            top: rail.bottom; bottom: pg.footerVisible ? bar.top : parent.bottom
            leftMargin: Tokens.s6; rightMargin: Tokens.s6
            topMargin: Tokens.s4; bottomMargin: Tokens.s5
        }
        sourceComponent: !pg.importSupported ? notSupportedComp
            : (pg.step === "source" ? sourceComp
            : (pg.step === "review" ? reviewComp
            : (pg.step === "resolve" ? resolveComp
            : (pg.step === "preview" ? previewComp : doneComp))))
    }

    // A provider package may omit foreign-config import. Keep that absence
    // explicit instead of offering a wizard that cannot map the desktop tree.
    Component {
        id: notSupportedComp
        Item {
            Column {
                anchors.centerIn: parent
                width: Math.min(parent.width - Tokens.s6 * 2, 520)
                spacing: Tokens.s3
                Text {
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: "\uf0ee"
                    color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: 30
                }
                Text {
                    width: parent.width
                    horizontalAlignment: Text.AlignHCenter
                    text: I18n.tr("No desktop config reader is installed")
                    color: Tokens.ink; font.family: Tokens.ui
                    font.pixelSize: Tokens.fRow; font.weight: Font.Medium; wrapMode: Text.WordWrap
                }
                Text {
                    width: parent.width
                    horizontalAlignment: Text.AlignHCenter
                    text: I18n.tr("Install or enable a window-manager provider that advertises config import, then reopen this page.")
                    color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                }
            }
        }
    }

    // ── Source: four affordances, no dead ends ─────────────────────────────────
    Component {
        id: sourceComp
        Flickable {
            contentWidth: width
            contentHeight: srcCol.height
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
            WheelScroll { }

            Column {
                id: srcCol
                width: parent.width - Tokens.s3
                spacing: Tokens.s4

                // What Ryoku can bring over, on the same row rhythm as every other
                // sheet in the Hub: a card with value rows parted by hairlines,
                // rather than a bespoke plate with its own tighter spacing.
                SettingCard {
                    width: srcCol.width
                    collapsible: false
                    title: I18n.tr("WHAT IT BRINGS OVER")

                    Repeater {
                        model: [
                            { app: pg.providerCased(), note: "keybinds, rules and supported settings become Ryoku settings; unknown native nodes stay in the provider user include" },
                            { app: "Kitty", note: "kitty.conf, layered into kitty/user.conf" },
                            { app: "Fish", note: "config.fish, functions and conf.d, layered into fish/user.fish" },
                            { app: "Fastfetch", note: "config.jsonc, layered into fastfetch/user.jsonc" },
                            { app: "Other apps", note: "any other config folder, dropped into its own override slot" }
                        ]
                        delegate: SettingRow {
                            required property var modelData
                            required property int index
                            anchors.left: parent.left
                            anchors.right: parent.right
                            divider: index > 0
                            label: I18n.tr(modelData.app)
                            desc: I18n.tr(modelData.note)
                        }
                    }
                }

                // the auto-detect banner: an existing non-Ryoku setup in ~/.config.
                Plate {
                    width: srcCol.width
                    height: Math.max(72, banRow.implicitHeight + Tokens.s4 * 2)
                    visible: pg.autoDetected
                    border.color: Tokens.lineStrong
                    Row {
                        id: banRow
                        anchors.fill: parent
                        anchors.margins: Tokens.s4
                        spacing: Tokens.s4
                        Column {
                            width: parent.width - detectBtn.width - Tokens.s4
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: Tokens.s1
                            Text {
                                text: I18n.tr("An existing %1 source was detected").arg(pg.providerCased())
                                color: Tokens.ink; font.family: Tokens.ui
                                font.pixelSize: Tokens.fRow; font.weight: Font.Medium
                            }
                            Text {
                                width: parent.width
                                text: I18n.tr("Scan the detected source to bring its keybinds, rules and app configs onto Ryoku.")
                                color: Tokens.inkMuted; font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                            }
                        }
                        Btn {
                            id: detectBtn
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("SCAN %1").arg(pg.providerCased().toUpperCase())
                            primary: true
                            onAct: pg.chooseSource(pg.providerDefaultPath)
                        }
                    }
                }

                // the drop / pick zone.
                Plate {
                    id: dropPlate
                    width: srcCol.width
                    height: 196
                    border.color: dropArea.containsDrag ? Tokens.sun : Tokens.line
                    Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

                    Column {
                        anchors.centerIn: parent
                        width: parent.width - Tokens.s6 * 2
                        spacing: Tokens.s3
                        Text {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: "\uf0ee"
                            color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: 30
                        }
                        Text {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: I18n.tr("Drop a config folder here")
                            color: Tokens.ink; font.family: Tokens.ui
                            font.pixelSize: Tokens.fRow; font.weight: Font.Medium
                        }
                        Text {
                            width: parent.width
                            horizontalAlignment: Text.AlignHCenter
                            text: I18n.tr("A whole ~/.config, a single app folder, or a dotfiles checkout all work.")
                            color: Tokens.inkMuted; font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                        }
                        Btn {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: I18n.tr("CHOOSE A FOLDER")
                            onAct: picker.open()
                        }
                    }

                    DropArea {
                        id: dropArea
                        anchors.fill: parent
                        onDropped: (drop) => {
                            if (drop.hasUrls && drop.urls.length > 0)
                                pg.chooseSource(pg.stripFile(drop.urls[0]));
                        }
                    }
                }

                // the git URL row.
                Column {
                    width: srcCol.width
                    spacing: Tokens.s2
                    Text {
                        text: I18n.tr("OR A GIT URL")
                        color: Tokens.inkMuted; font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                        font.letterSpacing: Tokens.trackMark
                    }
                    Row {
                        width: parent.width
                        spacing: Tokens.s2
                        Field {
                            id: urlField
                            width: parent.width - fetchBtn.width - Tokens.s2
                            toolbar: true
                            tabular: true
                            placeholder: I18n.tr("https://github.com/you/dotfiles.git")
                            onEdited: (v) => pg.urlText = v
                            onAccepted: pg.chooseSource(pg.urlText)
                        }
                        Btn {
                            id: fetchBtn
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("FETCH")
                            armed: pg.urlText.trim().length > 0
                            onAct: pg.chooseSource(pg.urlText)
                        }
                    }
                    Text {
                        width: parent.width
                        text: I18n.tr("Cloned to a temporary folder, scanned, and layered in only when you apply.")
                        color: Tokens.inkFaint; font.family: Tokens.ui
                        font.pixelSize: Tokens.fTiny; wrapMode: Text.WordWrap
                    }
                }
            }
        }
    }

    // ── Review: one card per detected app ──────────────────────────────────────
    Component {
        id: reviewComp
        Flickable {
            contentWidth: width
            contentHeight: revCol.height
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
            WheelScroll { }

            Column {
                id: revCol
                width: parent.width - Tokens.s3
                spacing: Tokens.s4

                Text {
                    text: pg.includedCount + " " + (pg.includedCount === 1 ? I18n.tr("app") : I18n.tr("apps")) + ", "
                        + pg.conflictTotal + " " + (pg.conflictTotal === 1 ? I18n.tr("conflict") : I18n.tr("conflicts"))
                    color: Tokens.ink; font.family: Tokens.ui
                    font.pixelSize: Tokens.fRow; font.weight: Font.Medium
                }

                Empty {
                    width: revCol.width
                    visible: pg.apps.length === 0
                    caption: I18n.tr("Nothing importable was found there. Try another folder, an app config, or a git URL.")
                }

                Repeater {
                    model: pg.apps
                    delegate: Plate {
                        id: appCard
                        required property var modelData
                        readonly property bool present: appCard.modelData.present !== false
                        readonly property int nConflicts: (appCard.modelData.conflicts || []).length
                        readonly property int nLosses: (appCard.modelData.losses || []).length
                        readonly property string scanError: appCard.modelData.error || ""
                        visible: appCard.present
                        width: revCol.width
                        height: appCard.present ? (appBody.implicitHeight + Tokens.s4 * 2) : 0

                        Column {
                            id: appBody
                            anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s4 }
                            spacing: Tokens.s2

                            Item {
                                width: parent.width
                                height: Math.max(headRow.implicitHeight, incSw.height)
                                Row {
                                    id: headRow
                                    anchors.left: parent.left
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: Tokens.s2
                                    Text {
                                        text: appCard.modelData.name || appCard.modelData.id
                                        color: Tokens.ink; font.family: Tokens.ui
                                        font.pixelSize: Tokens.fRow; font.weight: Font.Medium
                                        anchors.verticalCenter: parent.verticalCenter
                                    }
                                    Pill {
                                        anchors.verticalCenter: parent.verticalCenter
                                        label: pg.tierLabel(appCard.modelData.tier || "drop")
                                        tint: (appCard.modelData.tier === "deep") ? Tokens.sun : Tokens.inkMuted
                                    }
                                    Pill {
                                        anchors.verticalCenter: parent.verticalCenter
                                        visible: appCard.nConflicts > 0
                                        label: appCard.nConflicts + " " + (appCard.nConflicts === 1 ? I18n.tr("conflict") : I18n.tr("conflicts"))
                                        tint: Tokens.alert
                                    }
                                    Pill {
                                        anchors.verticalCenter: parent.verticalCenter
                                        visible: appCard.scanError.length > 0
                                        label: I18n.tr("PARSE ERROR")
                                        tint: Tokens.alert
                                    }
                                    Pill {
                                        anchors.verticalCenter: parent.verticalCenter
                                        visible: appCard.nLosses > 0
                                        label: appCard.nLosses + " " + (appCard.nLosses === 1 ? I18n.tr("preserved item") : I18n.tr("preserved items"))
                                        tint: Tokens.inkMuted
                                    }
                                }
                                Sw {
                                    id: incSw
                                    anchors.right: parent.right
                                    anchors.verticalCenter: parent.verticalCenter
                                    on: pg.includeOf(appCard.modelData.id)
                                    onToggled: (v) => pg.setInclude(appCard.modelData.id, v)
                                }
                            }
                            Text {
                                width: parent.width
                                text: appCard.modelData.summary || ""
                                color: appCard.scanError.length > 0 ? Tokens.alert : Tokens.inkDim
                                font.family: Tokens.mono
                                font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                            }
                            Text {
                                width: parent.width
                                text: pg.tierNote(appCard.modelData.tier || "drop")
                                color: Tokens.inkFaint; font.family: Tokens.ui
                                font.pixelSize: Tokens.fTiny; wrapMode: Text.WordWrap
                            }
                            Repeater {
                                model: appCard.modelData.losses || []
                                delegate: Text {
                                    required property var modelData
                                    width: appBody.width
                                    text: I18n.tr("%1 Preserved as native config: %2").arg(modelData.reason || "").arg(modelData.raw || "")
                                    color: Tokens.inkMuted
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fTiny
                                    wrapMode: Text.WrapAnywhere
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    // ── Resolve: the conflict table, keybinds first ────────────────────────────
    Component {
        id: resolveComp
        Flickable {
            contentWidth: width
            contentHeight: resCol.height
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
            WheelScroll { }

            Column {
                id: resCol
                width: parent.width - Tokens.s3
                spacing: Tokens.s4

                Row {
                    width: resCol.width
                    spacing: Tokens.s2
                    Text {
                        text: I18n.tr("%1 / %2 resolved").arg(pg.resolvedCount).arg(pg.conflictTotal)
                        color: Tokens.ink; font.family: Tokens.ui
                        font.pixelSize: Tokens.fRow; font.weight: Font.Medium
                    }
                    Pill {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: pg.conflictTotal - pg.resolvedCount > 0
                        label: I18n.tr("%1 on the default").arg(pg.conflictTotal - pg.resolvedCount)
                        tint: Tokens.inkMuted
                    }
                }
                Text {
                    width: resCol.width
                    visible: pg.conflictTotal > 0
                    text: I18n.tr("Each combo your config wants that Ryoku already uses. Keep Ryoku's shortcut, use yours (Ryoku unbinds its own so yours actually wins), or remap yours to a free chord. Untouched rows keep Ryoku's.")
                    color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap; lineHeight: 1.3
                }

                Empty {
                    width: resCol.width
                    visible: pg.conflictTotal === 0
                    caption: I18n.tr("No keybind conflicts. Your binds layer in cleanly.")
                }

                Repeater {
                    model: pg.conflicts
                    delegate: Plate {
                        id: confRow
                        required property var modelData
                        readonly property bool remapped: {
                            var d = pg.decisionOf(confRow.modelData.norm);
                            return d && d.remap ? true : false;
                        }
                        width: resCol.width
                        height: confBody.implicitHeight + Tokens.s4 * 2

                        Column {
                            id: confBody
                            anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s4 }
                            spacing: Tokens.s2

                            Item {
                                width: parent.width
                                height: Math.max(comboText.implicitHeight, seg.height)
                                Row {
                                    anchors.left: parent.left
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: Tokens.s2
                                    Text {
                                        id: comboText
                                        text: confRow.modelData.combo || ""
                                        color: Tokens.ink; font.family: Tokens.mono
                                        font.pixelSize: Tokens.fRow; font.weight: Font.Medium
                                        anchors.verticalCenter: parent.verticalCenter
                                    }
                                    Pill {
                                        anchors.verticalCenter: parent.verticalCenter
                                        visible: confRow.modelData.kind === "duplicate"
                                        label: I18n.tr("DUPLICATE")
                                        tint: Tokens.inkMuted
                                    }
                                }
                                Seg {
                                    id: seg
                                    anchors.right: parent.right
                                    anchors.verticalCenter: parent.verticalCenter
                                    options: ["Keep Ryoku's", "Use mine", "Remap"]
                                    current: pg.segCurrent(confRow.modelData.norm)
                                    onChose: (label) => pg.chooseSegment(confRow.modelData.norm, label)
                                }
                            }

                            Text {
                                width: parent.width
                                text: "Ryoku" + ": " + (confRow.modelData.ryoku.desc || I18n.tr("a shipped shortcut"))
                                color: Tokens.inkDim; font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                            }
                            Text {
                                width: parent.width
                                text: I18n.tr("Yours") + ": " + (confRow.modelData.mine.desc || confRow.modelData.mine.raw || "")
                                color: Tokens.inkDim; font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                            }
                            Row {
                                width: parent.width
                                spacing: Tokens.s2
                                visible: confRow.remapped
                                Text {
                                    text: "\u2192 " + ((pg.decisionOf(confRow.modelData.norm) || ({})).remap || "")
                                    color: Tokens.sun; font.family: Tokens.mono; font.pixelSize: Tokens.fSmall
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                                Text {
                                    visible: pg.remapClashes(confRow.modelData.norm)
                                    text: I18n.tr("still overlaps another combo")
                                    color: Tokens.alert; font.family: Tokens.ui; font.pixelSize: Tokens.fTiny
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                            }
                        }
                    }
                }

                // non-keybind items collapse here: they layer on top and win.
                Column {
                    id: layeredSection
                    width: resCol.width
                    spacing: Tokens.s2
                    visible: pg.layeredItems.length > 0
                    property bool expanded: false
                    Row {
                        spacing: Tokens.s2
                        Text {
                            text: (layeredSection.expanded ? "\u25be " : "\u25b8 ")
                                + I18n.tr("%1 settings layer on top and win").arg(pg.layeredItems.length)
                            color: Tokens.inkDim; font.family: Tokens.ui
                            font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                            font.letterSpacing: Tokens.trackLabel
                        }
                        HoverHandler { cursorShape: Qt.PointingHandCursor }
                        TapHandler { onTapped: layeredSection.expanded = !layeredSection.expanded }
                    }
                    Plate {
                        width: parent.width
                        visible: layeredSection.expanded
                        height: rawCol.implicitHeight + Tokens.s3 * 2
                        Column {
                            id: rawCol
                            anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s3 }
                            spacing: Tokens.s1
                            Repeater {
                                model: pg.layeredItems
                                delegate: Text {
                                    required property var modelData
                                    width: rawCol.width
                                    text: modelData.raw
                                    color: Tokens.inkFaint; font.family: Tokens.mono
                                    font.pixelSize: Tokens.fTiny; elide: Text.ElideRight
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    // ── Preview: the change set before you commit ──────────────────────────────
    Component {
        id: previewComp
        Flickable {
            contentWidth: width
            contentHeight: preCol.height
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
            WheelScroll { }

            Column {
                id: preCol
                width: parent.width - Tokens.s3
                spacing: Tokens.s4

                Text {
                    text: I18n.tr("What applying will do")
                    color: Tokens.ink; font.family: Tokens.ui
                    font.pixelSize: Tokens.fRow; font.weight: Font.Medium
                }

                // per-app handling.
                Repeater {
                    model: pg.apps
                    delegate: Plate {
                        id: preApp
                        required property var modelData
                        readonly property bool shown: preApp.modelData.present !== false && pg.includeOf(preApp.modelData.id)
                        visible: preApp.shown
                        width: preCol.width
                        height: preApp.shown ? (preBody.implicitHeight + Tokens.s3 * 2) : 0
                        Column {
                            id: preBody
                            anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s3 }
                            spacing: Tokens.s1
                            Text {
                                text: preApp.modelData.name || preApp.modelData.id
                                color: Tokens.ink; font.family: Tokens.ui
                                font.pixelSize: Tokens.fBody; font.weight: Font.Medium
                            }
                            Text {
                                width: parent.width
                                text: pg.tierNote(preApp.modelData.tier || "drop")
                                color: Tokens.inkMuted; font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                            }
                        }
                    }
                }

                // the counts + the backup promise.
                Plate {
                    width: preCol.width
                    height: countsCol.implicitHeight + Tokens.s4 * 2
                    Column {
                        id: countsCol
                        anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s4 }
                        spacing: Tokens.s2
                        Text {
                            text: I18n.tr("%1 keybinds ingested into the GUI").arg(pg.forecast.binds)
                            color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                        }
                        Text {
                            text: I18n.tr("%1 window rules ingested").arg(pg.forecast.rules)
                            color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                        }
                        Text {
                            text: I18n.tr("%1 unbinds added so your shortcuts win").arg(pg.forecast.unbinds)
                            color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                        }
                        Rectangle { width: parent.width; height: 1; color: Tokens.lineSoft }
                        Text {
                            width: parent.width
                            text: I18n.tr("Every file this touches is copied into ~/.config/ryoku/import-backups first, so the whole import can be undone from the next step.")
                            color: Tokens.inkMuted; font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap; lineHeight: 1.3
                        }
                    }
                }
            }
        }
    }

    // ── Done: the peak-end, with reload and undo ───────────────────────────────
    Component {
        id: doneComp
        Flickable {
            contentWidth: width
            contentHeight: doneCol.height
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
            WheelScroll { }

            Column {
                id: doneCol
                width: parent.width - Tokens.s3
                spacing: Tokens.s4

                Row {
                    spacing: Tokens.s2
                    Text {
                        text: pg.undone ? "空" : "力"
                        color: pg.undone ? Tokens.inkDim : Tokens.sun
                        font.family: Tokens.jp; font.pixelSize: Tokens.fHero
                        anchors.verticalCenter: parent.verticalCenter
                    }
                    Text {
                        text: pg.undone ? I18n.tr("Import undone") : I18n.tr("Your config is in")
                        color: Tokens.ink; font.family: Tokens.display; font.pixelSize: Tokens.fHero
                        anchors.verticalCenter: parent.verticalCenter
                    }
                }

                Text {
                    width: doneCol.width
                    visible: !pg.undone
                    text: I18n.tr("Your keybinds are in the Keybinds page and conflict-checked from now on; the rest of your config is layered in and live. Reload the desktop to see it, or undo the whole import.")
                    color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap; lineHeight: 1.3
                }
                Text {
                    width: doneCol.width
                    visible: pg.undone
                    text: I18n.tr("Everything this import touched has been restored from the backup. You are back where you started.")
                    color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap; lineHeight: 1.3
                }

                // the exact change set the engine reported.
                Plate {
                    width: doneCol.width
                    visible: !pg.undone && pg.applyResult
                    height: resCol2.implicitHeight + Tokens.s4 * 2
                    Column {
                        id: resCol2
                        anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s4 }
                        spacing: Tokens.s2
                        Text {
                            text: I18n.tr("%1 keybinds, %2 rules, %3 unbinds")
                                .arg((pg.applyResult ? pg.applyResult.bindsIngested : 0) || 0)
                                .arg((pg.applyResult ? pg.applyResult.rulesIngested : 0) || 0)
                                .arg((pg.applyResult ? pg.applyResult.unbinds : 0) || 0)
                            color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; font.weight: Font.Medium
                        }
                        Text {
                            text: I18n.tr("FILES WRITTEN")
                            color: Tokens.inkMuted; font.family: Tokens.ui
                            font.pixelSize: Tokens.fTiny; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                        }
                        Repeater {
                            model: (pg.applyResult && pg.applyResult.filesWritten) ? pg.applyResult.filesWritten : []
                            delegate: Text {
                                required property var modelData
                                width: resCol2.width
                                text: modelData
                                color: Tokens.inkDim; font.family: Tokens.mono
                                font.pixelSize: Tokens.fSmall; elide: Text.ElideMiddle
                            }
                        }
                        Rectangle { width: parent.width; height: 1; color: Tokens.lineSoft }
                        Text {
                            width: parent.width
                            text: I18n.tr("Backup") + ": " + pg.tildePath((pg.applyResult && pg.applyResult.backupDir) ? pg.applyResult.backupDir : "")
                            color: Tokens.inkFaint; font.family: Tokens.mono
                            font.pixelSize: Tokens.fTiny; wrapMode: Text.WrapAnywhere
                        }
                    }
                }

                // restored files after an undo.
                Plate {
                    width: doneCol.width
                    visible: pg.undone && pg.undoResult
                    height: undCol.implicitHeight + Tokens.s4 * 2
                    Column {
                        id: undCol
                        anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s4 }
                        spacing: Tokens.s2
                        Text {
                            text: I18n.tr("FILES RESTORED")
                            color: Tokens.inkMuted; font.family: Tokens.ui
                            font.pixelSize: Tokens.fTiny; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                        }
                        Repeater {
                            model: (pg.undoResult && pg.undoResult.restored) ? pg.undoResult.restored : []
                            delegate: Text {
                                required property var modelData
                                width: undCol.width
                                text: modelData
                                color: Tokens.inkDim; font.family: Tokens.mono
                                font.pixelSize: Tokens.fSmall; elide: Text.ElideMiddle
                            }
                        }
                    }
                }

                Row {
                    spacing: Tokens.s2
                    Btn {
                        visible: !pg.undone
                        text: I18n.tr("RELOAD NOW")
                        primary: true
                        onAct: pg.reloadDesktop()
                    }
                    Btn {
                        visible: !pg.undone
                        text: I18n.tr("UNDO THIS IMPORT")
                        onAct: pg.undo()
                    }
                    Btn {
                        text: I18n.tr("IMPORT ANOTHER")
                        onAct: pg.restart()
                    }
                }
            }
        }
    }

    // ── footer: back + the step's primary action ───────────────────────────────
    Rectangle {
        id: bar
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6; anchors.bottomMargin: Tokens.s5
        height: 60
        color: "transparent"
        visible: pg.importSupported && pg.footerVisible
        Rectangle {
            anchors { left: parent.left; right: parent.right; top: parent.top }
            height: 1; color: Tokens.line
        }
        Btn {
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("BACK")
            onAct: pg.goBack()
        }
        Btn {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr(pg.primaryLabel())
            primary: true
            onAct: pg.goNext()
        }
    }

    // ── the error line: pinned low, never shifts the body layout ───────────────
    Text {
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6; anchors.bottomMargin: Tokens.s2
        visible: pg.errorMsg.length > 0 && !pg.busy
        text: pg.errorMsg
        color: Tokens.alert; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
        wrapMode: Text.WordWrap
    }

    // ── the folder picker ──────────────────────────────────────────────────────
    PickFile {
        id: picker
        foldersOnly: true
        title: I18n.tr("Choose a config folder")
        startFolder: "file://" + pg.home + "/.config"
        onPicked: (path) => { picker.active = false; pg.chooseSource(pg.stripFile(path)); }
        onCanceled: picker.active = false
    }

    // ── busy overlay: a spinner while scan/apply/undo touch the disk ───────────
    MouseArea {
        anchors.fill: parent
        visible: pg.busy
        z: 100
        hoverEnabled: true
        Rectangle {
            anchors.fill: parent
            color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.7)
        }
        Column {
            anchors.centerIn: parent
            spacing: Tokens.s3
            Rectangle {
                id: spin
                anchors.horizontalCenter: parent.horizontalCenter
                width: 26; height: 26
                color: "transparent"
                radius: 3
                border.width: 2
                border.color: Tokens.sun
                NumberAnimation on rotation {
                    running: pg.busy
                    from: 0; to: 360
                    duration: 1100
                    loops: Animation.Infinite
                }
            }
            Text {
                anchors.horizontalCenter: parent.horizontalCenter
                text: I18n.tr(pg.busyLabel) + "\u2026"
                color: Tokens.inkDim; font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall; font.letterSpacing: Tokens.trackLabel
            }
        }
    }

    // ── chord recorder overlay: shared reader, page-owned capture ──────────────
    property string remapNorm: ""
    readonly property bool recording: pg.remapNorm.length > 0

    function enterRecordSubmap() {
        if (Settings.supports("submap") && pg.hub)
            pg.hub.wmAct("submap.enter", ["record"]);
    }
    function exitRecordSubmap() {
        if (Settings.supports("submap") && pg.hub)
            pg.hub.wmAct("submap.reset");
    }
    function startRemap(norm) {
        if (!norm)
            return;
        pg.remapNorm = norm;
        pg.enterRecordSubmap();
        remapTimeout.restart();
    }
    function stopRemap(commit, chord) {
        remapTimeout.stop();
        pg.exitRecordSubmap();
        var norm = pg.remapNorm;
        pg.remapNorm = "";
        if (!commit || !chord || !norm)
            return;
        pg.setDecision(norm, { remap: chord });
    }
    Timer {
        id: remapTimeout
        interval: 15000
        onTriggered: pg.stopRemap(false, "")
    }
    MouseArea {
        id: recScrim
        anchors.fill: parent
        visible: pg.recording
        z: 200
        onClicked: pg.stopRemap(false, "")
        onVisibleChanged: if (visible) capture.forceActiveFocus()

        Rectangle {
            anchors.fill: parent
            color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.72)
        }
        Item {
            id: capture
            anchors.fill: parent
            focus: true
            Keys.onPressed: (event) => {
                event.accepted = true;
                if (event.isAutoRepeat)
                    return;
                if (event.key === Qt.Key_Escape) {
                    pg.stopRemap(false, "");
                    return;
                }
                var chord = Combos.chordFrom(event);
                if (chord !== "")
                    pg.stopRemap(true, chord);
            }
        }
        Rectangle {
            anchors.centerIn: parent
            width: 340; height: 156
            radius: Tokens.radius
            color: Tokens.paper
            border.width: Tokens.border
            border.color: Tokens.lineStrong
            Column {
                anchors.centerIn: parent
                width: parent.width - Tokens.s4 * 2
                spacing: Tokens.s3
                Text {
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: I18n.tr("PRESS YOUR SHORTCUT")
                    color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                    font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                }
                Text {
                    width: parent.width
                    horizontalAlignment: Text.AlignHCenter
                    wrapMode: Text.WordWrap
                    text: I18n.tr("Hold your modifiers and tap the key. Esc cancels.")
                    color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                }
                Btn {
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: I18n.tr("CANCEL")
                    onAct: pg.stopRemap(false, "")
                }
            }
            MouseArea { anchors.fill: parent; z: -1 }
        }
    }
}
