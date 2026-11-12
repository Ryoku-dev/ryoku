import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../schema/phone.js" as Schema

// the phone link: setup, what the phone brings here, and the phones paired with this computer
Item {
    id: root

    required property var hub

    property var status: ({ installed: false, configured: false, supported: false, reason: "", devices: [] })
    property bool statusKnown: false
    property string statusError: ""
    // the running install or removal: "on", "off", "webcam" or ""
    property string job: ""
    // the job started last, so a stopped job's note shows in the section it was started from
    property string lastJob: ""
    property string jobError: ""
    property string jobErrorKind: ""
    property bool jobCancelled: false
    property string jobNote: ""
    property bool jobSlow: false
    property bool jobLong: false
    property string deviceBusy: ""
    property var deviceErrors: ({})
    property string pendingRow: ""

    readonly property var enabledBind: ({ store: "settings", key: "phone.enabled", fallback: false })
    readonly property var clockBind: ({ store: "settings", key: "phone.alarm.clockApps", fallback: [] })
    readonly property var ignoreBind: ({ store: "settings", key: "phone.alarm.ignorePackages", fallback: [] })
    readonly property var folderBind: ({ store: "settings", key: "phone.scrcpy.recording.folder", fallback: "" })

    readonly property bool enabledSetting: Changes.value(root.enabledBind) === true
    readonly property bool installed: root.status.installed === true
    readonly property bool linkOn: root.enabledSetting && root.installed
    // until the status is read, the setting alone decides what shows
    readonly property bool active: root.enabledSetting && (!root.statusKnown || root.installed)
    readonly property bool canSwitch: root.statusKnown && root.status.supported === true && root.job === ""
    readonly property var paired: (root.status.devices || []).filter(d => d.paired === true)
    readonly property var primary: root.status.primaryDevice || (root.paired.length > 0 ? root.paired[0] : null)

    readonly property string stateLine: {
        if (!root.statusKnown || !root.linkOn)
            return "";
        if (!root.primary)
            return I18n.tr("No phone paired yet");
        var name = root.primary.name || root.primary.id;
        if (!root.primary.reachable)
            return I18n.tr("Paired with %1, not connected").arg(name);
        if (root.primary.battery >= 0)
            return I18n.tr("Connected to %1, battery at %2%").arg(name).arg(root.primary.battery);
        return I18n.tr("Connected to %1").arg(name);
    }

    readonly property string linkDetail: {
        if (root.job === "on")
            return root.jobSlow ? I18n.tr("Installing the phone tools. This can take a few minutes.") : I18n.tr("Asking for your password");
        if (root.job === "off")
            return root.jobSlow ? I18n.tr("Removing the phone tools") : I18n.tr("Asking for your password");
        if (!root.statusKnown)
            return I18n.tr("Checking whether this computer can link a phone");
        if (root.status.supported !== true)
            return root.status.reason ? String(root.status.reason) : I18n.tr("Phone link is not available on this system.");
        if (root.linkOn)
            return I18n.tr("On. Phones you pair can share their notifications, screen, camera and microphone here.");
        if (root.enabledSetting)
            return I18n.tr("The phone tools are missing. Turn phone link on to install them again.");
        if (root.installed)
            return I18n.tr("Off. The phone tools are installed.");
        return I18n.tr("Off. Turning it on installs the tools a phone link needs and asks for your password.");
    }

    readonly property string webcamDetail: root.job === "webcam"
        ? (root.jobSlow ? I18n.tr("Setting up the camera device. This can take a few minutes.") : I18n.tr("Asking for your password"))
        : I18n.tr("The camera device video apps use is not set up yet. Setting it up asks for your password.")

    function alsoMet(row) {
        if (!row.also)
            return true;
        var v = Changes.value(row.also.bind);
        return row.also.is.some(x => JSON.stringify(x) === JSON.stringify(v));
    }

    function rateText(rate) {
        var r = String(rate);
        if (/^[0-9.]+M$/i.test(r))
            return I18n.tr("%1 Mbit/s").arg(r.slice(0, -1));
        if (/^[0-9.]+K$/i.test(r))
            return I18n.tr("%1 kbit/s").arg(r.slice(0, -1));
        return I18n.tr("%1 bit/s").arg(r);
    }

    // a value set outside these choices shows as its own option, so the control tells the truth
    function prepared(row) {
        if (row.control !== "segmented")
            return row;
        var v = Changes.value(row.bind);
        if (v === undefined || v === null || row.options.some(o => JSON.stringify(o.value) === JSON.stringify(v)))
            return row;
        var label = "", detail = row.detail;
        if (row.id === "quality") {
            label = "Custom";
            var size = Number(v.maxSize) > 0 ? I18n.tr("up to %1 px").arg(v.maxSize) : I18n.tr("full size");
            detail = I18n.tr("Now %1 at %2 fps, %3").arg(size).arg(v.maxFps).arg(root.rateText(v.videoBitRate));
        } else if (row.id === "webcam-size") {
            label = v.width + " × " + v.height;
        } else if (row.id === "webcam-fps") {
            label = I18n.tr("%1 fps").arg(v);
        } else {
            return row;
        }
        return Object.assign({}, row, { options: row.options.concat([{ label: label, value: v }]), detail: detail });
    }

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: root.active
            ? Schema.rows.filter(r => root.alsoMet(r)).map(r => root.prepared(r))
            : Schema.rows.filter(r => r.section === "link")
    })

    // the row search should land on: a hidden row leads to the row it depends on
    function landing(rowId) {
        var id = rowId;
        for (var i = 0; i < 4; i++) {
            if (page.sectionOfRow(id) || page.rowItems[id])
                return id;
            var row = Schema.row(id);
            var cond = row ? (!root.alsoMet(row) ? row.also : row.when) : null;
            if (!cond)
                return "";
            var key = Changes.keyOf(cond.bind);
            var parent = Schema.rows.find(r => r.bind && Changes.keyOf(r.bind) === key);
            if (!parent)
                return "";
            id = parent.id;
        }
        return "";
    }

    function focusRow(rowId) {
        if (!rowId)
            return;
        if (page.layout.length === 0) {
            root.pendingRow = rowId;
            return;
        }
        var row = Schema.row(rowId);
        if (!row) {
            // a section: with the link off only its own section is drawn
            if (!root.active && Schema.sections.some(s => s.id === rowId) && rowId !== "link")
                page.land("phone-link");
            else
                page.focusRow(rowId);
            return;
        }
        if (!root.active && row.section !== "link") {
            page.land("phone-link");
            return;
        }
        var id = root.landing(rowId);
        if (id !== "" && page.rowItems[id] && !page.sectionOfRow(id)) {
            var target = Schema.row(id);
            var sec = page.sectionItems[target.section];
            if (target.more === true && sec && !sec.moreOpen) {
                sec.openMore();
                bespokeLand.rowId = id;
                bespokeLand.restart();
                return;
            }
            page.land(id);
            return;
        }
        page.focusRow(id !== "" ? id : rowId);
    }

    // the reason a tool gave, without its name: its "error:" line, else its last line
    function errorOf(text, fallback) {
        var lines = String(text || "").trim().split("\n").map(l => l.trim().replace(/^ryoku-hub:\s*/, "")).filter(l => l.length > 0);
        for (var i = 0; i < lines.length; i++)
            if (lines[i].indexOf("error:") === 0)
                return lines[i].slice(6).trim();
        return lines.length > 0 ? lines[lines.length - 1] : fallback;
    }

    // a status read is running, so a second one waits for it
    property bool statusReading: false
    // the running install or removal; each start gets a number, so a stopped one's late answer is ignored
    property var jobRun: null
    property int jobSeq: 0

    function refresh() {
        if (root.statusReading)
            return;
        root.statusReading = true;
        root.statusError = "";
        Changes.run(["ryoku-hub", "phone", "status"], function (ok, out, err) {
            root.statusReading = false;
            root.statusRead(ok, out, err);
        }, { timeout: 20000 });
    }

    function statusRead(ok, out, err) {
        if (!ok) {
            root.statusError = root.errorOf(err, I18n.tr("the status check stopped"));
            root.statusKnown = true;
            return;
        }
        try {
            var s = JSON.parse(out || "{}");
            s.devices = Array.isArray(s.devices) ? s.devices : [];
            root.status = s;
        } catch (e) {
            root.statusError = I18n.tr("the status check returned something unreadable");
        }
        // after an action the page list's summary reads the status again too
        if (root.statusKnown)
            Changes.readCommand({ store: "command", get: ["ryoku-hub", "phone", "status"] });
        root.statusKnown = true;
    }

    // the removal plan names the packages the Confirm lists; without it the Confirm still asks
    function planOff() {
        Changes.run(["ryoku-hub", "phone", "plan-disable"], function (ok, out) {
            var packages = [];
            if (ok) {
                try {
                    packages = JSON.parse(out || "{}").packages || [];
                } catch (e) {
                    packages = [];
                }
            }
            root.confirmOff(packages);
        }, { timeout: 15000 });
    }

    function requestLink(on) {
        if (!root.canSwitch || on === root.linkOn)
            return;
        root.jobError = "";
        root.jobNote = "";
        if (on) {
            // the tools are already in place: no password needed
            if (root.installed && root.status.configured === true) {
                root.enable();
                return;
            }
            root.startJob("on");
            return;
        }
        root.planOff();
    }

    function enable() {
        Settings.patch("phone.enabled", true, function (ok, why) {
            if (!ok) {
                root.jobErrorKind = "on";
                root.jobError = I18n.tr("Phone link did not turn on: %1").arg(why || I18n.tr("the settings service did not answer"));
            }
            root.refresh();
        });
    }

    function confirmOff(packages) {
        var list = (packages || []).join(", ");
        root.hub.confirm({
            title: I18n.tr("Turn off phone link?"),
            text: list.length > 0
                ? I18n.tr("Ryoku removes the phone tools it installed: %1. Your phone settings stay, and you can turn phone link on again.").arg(list)
                : I18n.tr("Your phone stops connecting to this computer. Your phone settings stay, and you can turn phone link on again."),
            actionText: I18n.tr("Turn off")
        }, function () {
            Settings.patch("phone.enabled", false, function (ok, why) {
                if (!ok) {
                    root.jobErrorKind = "off";
                    root.jobError = I18n.tr("Phone link did not turn off: %1").arg(why || I18n.tr("the settings service did not answer"));
                }
            });
            root.startJob("off");
        });
    }

    function startJob(kind) {
        root.job = kind;
        root.lastJob = kind;
        root.jobError = "";
        root.jobNote = "";
        root.jobCancelled = false;
        root.jobSlow = false;
        root.jobLong = false;
        slowTimer.restart();
        longTimer.restart();
        // installing can take minutes, so no deadline: Cancel shows past ten seconds
        var seq = ++root.jobSeq;
        var run = Changes.run(["ryoku-hub", "phone", kind === "off" ? "off" : "on"], function (ok, out, err, code) {
            if (seq !== root.jobSeq)
                return;
            root.jobRun = null;
            if (root.job !== "")
                root.jobDone(ok ? 0 : (code === 0 ? 1 : code), err);
        });
        if (seq === root.jobSeq && root.job !== "")
            root.jobRun = run;
    }

    function cancelJob() {
        if (root.job === "")
            return;
        root.jobCancelled = true;
        root.jobSeq++;
        var run = root.jobRun;
        root.jobRun = null;
        if (run)
            run.running = false;
        Qt.callLater(function () {
            if (root.job !== "")
                root.jobDone(-1, "");
        });
    }

    function jobDone(code, err) {
        var kind = root.job;
        slowTimer.stop();
        longTimer.stop();
        root.job = "";
        root.jobSlow = false;
        root.jobLong = false;
        if (root.jobCancelled) {
            root.jobCancelled = false;
            root.jobNote = I18n.tr("Stopped waiting. If the system had already started, it finishes by itself; check again in a minute.");
            root.refresh();
            return;
        }
        if (code !== 0) {
            root.jobErrorKind = kind;
            var why = root.errorOf(err, I18n.tr("the system did not say why"));
            root.jobError = kind === "off" ? I18n.tr("The phone tools were not removed: %1").arg(why)
                : kind === "webcam" ? I18n.tr("The webcam was not set up: %1").arg(why)
                : I18n.tr("Phone link did not turn on: %1").arg(why);
            root.refresh();
            return;
        }
        if (kind === "on" || kind === "webcam") {
            root.enable();
            return;
        }
        root.refresh();
    }

    function retryJob() {
        var kind = root.jobErrorKind;
        root.jobError = "";
        if (kind === "off")
            root.startJob("off");
        else if (kind === "webcam")
            root.startJob("webcam");
        else if (root.installed && root.status.configured === true)
            root.enable();
        else
            root.startJob("on");
    }

    function deviceLine(d) {
        if (!d.reachable)
            return I18n.tr("Not connected");
        if (d.battery >= 0)
            return d.charging ? I18n.tr("Connected, battery at %1%, charging").arg(d.battery)
                              : I18n.tr("Connected, battery at %1%").arg(d.battery);
        return I18n.tr("Connected");
    }

    function askUnpair(d) {
        var name = d.name || d.id;
        root.hub.confirm({
            title: I18n.tr("Unpair %1?").arg(name),
            text: I18n.tr("It stops connecting to this computer. To use it here again, pair it from the phone."),
            actionText: I18n.tr("Unpair")
        }, function () { root.unpair(d.id); });
    }

    function unpair(id) {
        if (root.deviceBusy !== "")
            return;
        var e = Object.assign({}, root.deviceErrors);
        delete e[id];
        root.deviceErrors = e;
        root.deviceBusy = id;
        Changes.run(["ryoku-hub", "phone", "unpair", id], function (ok, out, err) {
            root.deviceBusy = "";
            if (!ok) {
                var errs = Object.assign({}, root.deviceErrors);
                errs[id] = I18n.tr("The phone was not unpaired: %1").arg(root.errorOf(err, I18n.tr("the phone service did not answer")));
                root.deviceErrors = errs;
            }
            root.refresh();
        }, { timeout: 20000 });
    }

    // each list edit is its own undo step, named by its notice
    function listEdit(bind, label, list, notice) {
        Changes.set(bind, list, label, "phone", notice);
    }

    function stringsOf(bind) {
        var v = Changes.value(bind);
        return Array.isArray(v) ? v.map(x => String(x)) : [];
    }

    function chooseFolder() {
        var current = String(Changes.value(root.folderBind) || "");
        folderDialog.currentFolder = "file://" + (current.length > 0 ? current.replace(/^~(?=\/|$)/, Quickshell.env("HOME") || "") : (Quickshell.env("HOME") || "/"));
        folderDialog.open();
    }

    Component.onCompleted: {
        Changes.ensureStore("phone", Qt.resolvedUrl("../components/phone/PhoneStore.qml"));
        root.refresh();
    }

    // working shows past a second, progress with Cancel past ten
    Timer { id: slowTimer; interval: 1000; onTriggered: root.jobSlow = true }
    Timer { id: longTimer; interval: 10000; onTriggered: root.jobLong = true }

    Timer {
        id: bespokeLand
        property string rowId: ""
        interval: LithicTokens.motion.unfold + 20
        onTriggered: page.land(bespokeLand.rowId)
    }

    FolderChooser {
        id: folderDialog
        title: I18n.tr("Choose where phone recordings go")
        overlayParent: root.hub.overlayItem
        onAccepted: {
            var path = decodeURIComponent(String(folderDialog.selectedFolder).replace(/^file:\/\//, ""));
            if (path.length > 0)
                Changes.set(root.folderBind, path, "Phone recordings folder", "phone");
        }
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "phone"
        schema: root.schema
        stateLine: root.stateLine
        slabBodies: root.active
            ? ({ link: linkBody, webcam: root.status.configured === false && root.statusKnown ? webcamBody : null, paired: pairedBody })
            : ({ link: linkBody })
        moreBodies: root.active ? ({ notifications: alarmListsBody, mirroring: folderBody }) : ({})
        onLayoutChanged: {
            if (root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    // registers a drawn part (an Item with a settingRow) as the landing spot for a template row
    component Spot: QtObject {
        id: spot
        required property string rowId
        property Item target: null
        property bool on: true
        function sync() {
            if (spot.on && spot.target)
                page.rowItems[spot.rowId] = spot.target;
            else if (page.rowItems[spot.rowId] === spot.target)
                delete page.rowItems[spot.rowId];
        }
        onTargetChanged: spot.sync()
        onOnChanged: spot.sync()
        Component.onCompleted: spot.sync()
        Component.onDestruction: {
            if (page.rowItems[spot.rowId] === spot.target)
                delete page.rowItems[spot.rowId];
        }
    }

    component ErrorLine: Row {
        id: errorLine
        property string text: ""
        // where the focus goes when its action hides the line
        property Item home: null
        property string actionText: I18n.tr("Retry")
        property string glyph: "refresh"
        property color ink: Lithic.jasper
        signal retried()
        visible: errorLine.text.length > 0
        width: parent ? parent.width : 0
        spacing: LithicTokens.space.s3

        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, parent.width - retry.width - LithicTokens.space.s3)
            text: errorLine.text
            typeRole: "detail"
            ink: errorLine.ink
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: retry
            anchors.verticalCenter: parent.verticalCenter
            kind: "quiet"
            small: true
            glyph: errorLine.glyph
            text: errorLine.actionText
            onClicked: {
                root.handOff(retry, errorLine.home);
                errorLine.retried();
            }
        }
    }

    // the focus a hidden plaque would keep goes to item instead, marked when the keys held it
    function handOff(from, item) {
        if (!item || !from.activeFocus)
            return;
        var reason = from.visualFocus ? Qt.TabFocusReason : Qt.OtherFocusReason;
        Qt.callLater(() => item.forceActiveFocus(reason));
    }

    Component {
        id: linkBody
        Column {
            width: parent ? parent.width : 0
            spacing: LithicTokens.space.s1

            SettingRow {
                id: linkRow
                property Item settingRow: linkRow
                width: parent.width
                first: true
                title: I18n.tr("Phone link")
                detail: root.linkDetail
                highlighted: page.highlightId === "phone-link"
                activatable: root.canSwitch
                onActivated: root.requestLink(!root.linkOn)

                Working {
                    anchors.verticalCenter: parent.verticalCenter
                    visible: root.job === "on" || root.job === "off" ? root.jobSlow : (!root.statusKnown && statusSlow.fired)
                    active: visible
                }
                Toggle {
                    id: linkToggle
                    anchors.verticalCenter: parent.verticalCenter
                    enabled: root.canSwitch
                    checked: root.linkOn
                    Accessible.name: I18n.tr("Phone link")
                    onToggled: {
                        var want = linkToggle.checked;
                        linkToggle.checked = Qt.binding(() => root.linkOn);
                        root.requestLink(want);
                    }
                }

                Timer {
                    id: statusSlow
                    property bool fired: false
                    interval: 1000
                    running: !root.statusKnown
                    onTriggered: statusSlow.fired = true
                }
            }
            Spot { rowId: "phone-link"; target: linkRow }

            JobLines {
                webcam: false
                home: linkToggle
            }
            ErrorLine {
                text: root.statusError.length > 0 ? I18n.tr("Could not read the phone link: %1").arg(root.statusError) : ""
                onRetried: root.refresh()
            }

            HandoffRow {
                id: setUp
                property Item settingRow: setUp
                visible: root.linkOn
                width: parent.width
                first: false
                text: root.primary ? I18n.tr("Open Phone") : I18n.tr("Set up your phone")
                detail: root.primary ? I18n.tr("Mirror the screen, open phone apps, send files and start the webcam or microphone")
                                     : I18n.tr("Pair a phone and allow USB debugging, step by step")
                onTriggered: Spawn.runApp(["ryoku-kit", "phone"], { label: "ryoku-kit" })
            }
            Spot { rowId: "set-up"; target: setUp; on: setUp.visible }
        }
    }

    // a running job's progress with Cancel past ten seconds, its error with Retry, and the note a
    // stopped job leaves, in the section the job was started from
    component JobLines: Column {
        id: jobLines
        required property bool webcam
        readonly property bool mine: jobLines.webcam === (root.job !== "" ? root.job === "webcam" : root.lastJob === "webcam")
        readonly property string errorText: jobLines.webcam === (root.jobErrorKind === "webcam") ? root.jobError : ""
        property Item home: null
        readonly property string noteText: jobLines.mine ? root.jobNote : ""
        visible: (jobLines.mine && root.jobLong && root.job !== "") || jobLines.errorText.length > 0 || jobLines.noteText.length > 0
        width: parent ? parent.width : 0
        spacing: LithicTokens.space.s1

        Row {
            visible: jobLines.mine && root.jobLong && root.job !== ""
            width: parent.width
            spacing: LithicTokens.space.s3
            BladeProgress {
                anchors.verticalCenter: parent.verticalCenter
                width: parent.width - cancelJob.width - LithicTokens.space.s3
                indeterminate: true
                active: parent.visible
            }
            Plaque {
                id: cancelJob
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                text: I18n.tr("Cancel")
                onClicked: {
                    // the job ends on the next turn, so the hand-off after it finds the link switch enabled again
                    root.cancelJob();
                    root.handOff(cancelJob, jobLines.home);
                }
            }
        }
        ErrorLine {
            text: jobLines.errorText
            home: jobLines.home
            onRetried: root.retryJob()
        }
        ErrorLine {
            text: jobLines.noteText
            home: jobLines.home
            ink: Lithic.fg2
            actionText: I18n.tr("Check again")
            onRetried: {
                root.jobNote = "";
                root.refresh();
            }
        }
    }

    Component {
        id: webcamBody
        Column {
            width: parent ? parent.width : 0
            spacing: LithicTokens.space.s1

            SettingRow {
                id: webcamRow
                property Item settingRow: webcamRow
                width: parent.width
                first: true
                title: I18n.tr("Finish webcam setup")
                detail: root.webcamDetail
                highlighted: page.highlightId === "webcam-setup"

                // stays enabled while it works, so the keyboard focus stays on it
                Plaque {
                    id: webcamSetUp
                    anchors.verticalCenter: parent.verticalCenter
                    kind: "line"
                    small: true
                    text: I18n.tr("Set up")
                    enabled: root.job === "" || root.job === "webcam"
                    busy: root.job === "webcam"
                    onClicked: {
                        if (root.job === "")
                            root.startJob("webcam");
                    }
                }
            }
            Spot { rowId: "webcam-setup"; target: webcamRow }
            JobLines {
                webcam: true
                home: webcamSetUp
            }
        }
    }

    Component {
        id: pairedBody
        Column {
            width: parent ? parent.width : 0

            SettingRow {
                id: emptyRow
                property Item settingRow: emptyRow
                visible: root.paired.length === 0
                width: parent.width
                first: true
                title: root.statusKnown ? I18n.tr("No phone is paired") : I18n.tr("Looking for paired phones")
                detail: I18n.tr("Set up your phone to pair one, then check again.")
                highlighted: page.highlightId === "paired-list"

                Plaque {
                    anchors.verticalCenter: parent.verticalCenter
                    kind: "quiet"
                    small: true
                    glyph: "refresh"
                    text: I18n.tr("Check again")
                    enabled: !root.statusReading
                    onClicked: root.refresh()
                }
            }
            Spot { rowId: "paired-list"; target: emptyRow; on: emptyRow.visible }

            Repeater {
                model: root.paired
                delegate: Column {
                    id: dev
                    required property var modelData
                    required property int index
                    width: parent ? parent.width : 0
                    spacing: LithicTokens.space.s1

                    SettingRow {
                        id: devRow
                        property Item settingRow: devRow
                        width: parent.width
                        first: dev.index === 0
                        title: dev.modelData.name || dev.modelData.id
                        detail: root.deviceLine(dev.modelData)
                        highlighted: dev.index === 0 && page.highlightId === "paired-list"

                        Working {
                            anchors.verticalCenter: parent.verticalCenter
                            visible: root.deviceBusy === dev.modelData.id
                            active: visible
                        }
                        Plaque {
                            anchors.verticalCenter: parent.verticalCenter
                            kind: "dangerLine"
                            small: true
                            text: I18n.tr("Unpair")
                            accessibleName: I18n.tr("Unpair %1").arg(devRow.title)
                            enabled: root.deviceBusy === ""
                            onClicked: root.askUnpair(dev.modelData)
                        }
                    }
                    Spot { rowId: "paired-list"; target: devRow; on: dev.index === 0 }
                    ErrorLine {
                        text: root.deviceErrors[dev.modelData.id] || ""
                        onRetried: root.unpair(dev.modelData.id)
                    }
                }
            }
        }
    }

    Component {
        id: alarmListsBody
        Column {
            width: parent ? parent.width : 0

            ListBlock {
                rowId: "clock-apps"
                bind: root.clockBind
                title: I18n.tr("Clock apps")
                detail: I18n.tr("Apps whose notifications mean an alarm is ringing")
                changeLabel: "Clock apps"
                addText: I18n.tr("Add a clock app")
                emptyText: I18n.tr("None added. The phone's next alarm still shows.")
                placeholder: "com.google.android.deskclock"
            }
            ListBlock {
                rowId: "ignored-apps"
                bind: root.ignoreBind
                title: I18n.tr("Apps that never count as alarms")
                detail: I18n.tr("Apps that set alarms for their own automations, not to wake you")
                changeLabel: "Apps that never count as alarms"
                addText: I18n.tr("Add an app")
                emptyText: I18n.tr("None added")
                placeholder: "com.example.timer"
            }
        }
    }

    component ListBlock: Column {
        id: block
        required property string rowId
        required property var bind
        required property string title
        required property string detail
        required property string changeLabel
        required property string addText
        required property string emptyText
        required property string placeholder
        readonly property var items: root.stringsOf(block.bind)
        width: parent ? parent.width : 0
        spacing: LithicTokens.space.s2

        SettingRow {
            id: head
            property Item settingRow: head
            width: parent.width
            first: false
            title: block.title
            detail: block.detail
            highlighted: page.highlightId === block.rowId
            changed: block.items.length > 0
            onResetRequested: root.listEdit(block.bind, block.changeLabel, [])
        }
        Spot { rowId: block.rowId; target: head }

        ListEditor {
            width: parent.width
            model: block.items.map(x => ({ title: x }))
            addText: block.addText
            emptyText: block.emptyText
            accessibleName: block.title
            editor: Component {
                Item {
                    id: ed
                    property var item
                    readonly property bool valid: field.text.trim().length > 0 && /^[^\s,]+$/.test(field.text.trim())
                    function value() { return { title: field.text.trim() }; }
                    implicitHeight: field.implicitHeight
                    TextField {
                        id: field
                        width: parent.width
                        mono: true
                        text: ed.item && ed.item.title ? String(ed.item.title) : ""
                        placeholderText: block.placeholder
                        errorText: field.text.trim().length > 0 && !ed.valid ? I18n.tr("One app name, with no spaces or commas") : ""
                        Accessible.name: block.title
                    }
                }
            }
            onAdded: item => root.listEdit(block.bind, block.changeLabel, block.items.concat([item.title]),
                                           I18n.tr("%1 added").arg(item.title))
            onEdited: (index, item) => {
                var next = block.items.slice();
                next[index] = item.title;
                root.listEdit(block.bind, block.changeLabel, next, I18n.tr("%1 changed").arg(item.title));
            }
            onRemoved: index => {
                var next = block.items.slice();
                var gone = next.splice(index, 1)[0];
                root.listEdit(block.bind, block.changeLabel, next, I18n.tr("%1 removed").arg(gone));
            }
        }
        ErrorLine {
            text: Changes.error(block.bind)
            onRetried: Changes.retry(block.bind)
        }
    }

    Component {
        id: folderBody
        Column {
            id: folderBlock
            readonly property string folder: String(Changes.value(root.folderBind) || "")
            width: parent ? parent.width : 0
            spacing: LithicTokens.space.s1

            SettingRow {
                id: folderRow
                property Item settingRow: folderRow
                width: parent.width
                first: false
                title: I18n.tr("Phone recordings folder")
                detail: I18n.tr("Where recordings of the phone's screen go; empty uses your Videos folder")
                highlighted: page.highlightId === "recording-folder"
                changed: folderBlock.folder.length > 0
                onResetRequested: Changes.set(root.folderBind, "", "Phone recordings folder", "phone")

                PathField {
                    width: LithicTokens.size.settingsControlMax
                    kind: "folder"
                    path: folderBlock.folder
                    placeholder: I18n.tr("Videos")
                    accessibleName: I18n.tr("Phone recordings folder")
                    onChooseRequested: root.chooseFolder()
                    onEdited: path => Changes.set(root.folderBind, path.trim(), "Phone recordings folder", "phone")
                }
            }
            Spot { rowId: "recording-folder"; target: folderRow }
            ErrorLine {
                text: Changes.error(root.folderBind)
                onRetried: Changes.retry(root.folderBind)
            }
        }
    }
}
