pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/updates"
import "../schema/updates.js" as Schema

// where Ryoku stands, the update itself as it runs, what it brings, and the ways back
Item {
    id: root

    required property var hub
    property string pendingRow: ""

    readonly property var systemBind: Schema.rows.find(r => r.id === "whole-system").bind
    readonly property bool systemFull: Changes.value(root.systemBind) === true
    readonly property bool known: Updates.lastChecked !== null
    readonly property bool building: Updates.buildState === "building"
    readonly property bool systemWaiting: Updates.packages.length > 0
    readonly property bool pendingAction: Updates.available || (root.systemFull && root.systemWaiting)
    readonly property var frame: Updates.frame || ({})

    // a check from here: busy past 1 s, a blade past 10 s, an answer or an error in the end
    property bool checking: false
    property bool checkSlow: false
    property bool checkLong: false
    property string checkError: ""

    function check() {
        if (root.checking)
            return;
        root.checkError = "";
        root.checking = true;
        root.checkSlow = false;
        root.checkLong = false;
        slowTimer.restart();
        longTimer.restart();
        capTimer.restart();
        // the desktop service takes the request at once; the answer comes later as a new status
        Changes.call("updates.check", {}, function (ok) {
            if (ok || !root.checking)
                return;
            root.checkSettled();
            root.checkError = I18n.tr("The check could not start. Make sure your desktop is running, then press Retry.");
        }, 10000);
    }

    function checkSettled() {
        root.checking = false;
        root.checkSlow = false;
        root.checkLong = false;
        slowTimer.stop();
        longTimer.stop();
        capTimer.stop();
    }

    Timer { id: slowTimer; interval: 1000; onTriggered: root.checkSlow = root.checking }
    Timer { id: longTimer; interval: 10000; onTriggered: root.checkLong = root.checking }
    Timer {
        id: capTimer
        interval: 180000
        onTriggered: {
            if (!root.checking)
                return;
            root.checkSettled();
            root.checkError = I18n.tr("The check did not finish. Make sure your desktop is running, or run ryoku status in a terminal to see why.");
        }
    }

    Connections {
        target: Updates
        function onLastCheckedChanged() {
            root.checkSettled();
            root.reland();
        }
    }

    // an answer or a window resize reflows the page under a landing made just before it, so it is redone
    function reland() {
        if (root.landedRow !== "" && Date.now() - root.landedAt < 30000
                && (landSettle.running || (root.Window.activeFocusItem === root.landedFocus && page.currentSection === root.landedSection)))
            frameLand.restart();
    }
    onWidthChanged: root.reland()
    onHeightChanged: root.reland()

    // the status, notices and lists take a frame or two to place what an answer brought
    Timer {
        id: frameLand
        interval: 150
        onTriggered: {
            root.focusRow(root.landedRow, true);
            landSettle.restart();
        }
    }

    // the last deep landing, and where the keys and the reading were once it settled
    property string landedRow: ""
    property double landedAt: 0
    property Item landedFocus: null
    property int landedSection: -1
    Timer {
        id: landSettle
        interval: 1200
        onTriggered: {
            root.landedFocus = root.Window.activeFocusItem;
            root.landedSection = page.currentSection;
        }
    }

    function startUpdate() {
        if (!root.pendingAction || root.building)
            return;
        updateRun.start(root.systemFull);
    }

    function openTerminal(script, args) {
        Spawn.runApp(["kitty", "--class=dev.ryoku.update", "-e", "sh", "-c",
            script + "; printf '\\n%s ' \"$1\"; read -r _", "sh", I18n.tr("Press Enter to close.")].concat(args || []), { label: "kitty" });
    }

    // a translated sentence whose %1, %2 are runs of spans; prose breaks at words so it wraps
    function spans(template, values) {
        var out = [];
        var parts = String(template).split(/(%\d)/);
        for (var i = 0; i < parts.length; i++) {
            var m = /^%(\d)$/.exec(parts[i]);
            if (m) {
                out = out.concat(values[Number(m[1]) - 1] || []);
                continue;
            }
            var words = parts[i].split(" ");
            for (var w = 0; w < words.length; w++) {
                var word = w < words.length - 1 ? words[w] + " " : words[w];
                if (word.length > 0)
                    out.push({ text: word });
            }
        }
        return out;
    }

    function release(name, version) {
        var out = [];
        if (name !== "")
            out.push({ text: "Ryoku " + name + (version !== "" ? " " : "") });
        if (version !== "")
            out.push({ text: version, mono: true });
        return out;
    }

    // Updates ticks every 30 s while this page is open
    readonly property string checkedAgo: {
        Updates.tick;
        if (!Updates.lastChecked)
            return I18n.tr("not yet");
        var s = Math.floor((Date.now() - Updates.lastChecked.getTime()) / 1000);
        if (s < 60)
            return I18n.tr("just now");
        var m = Math.floor(s / 60);
        if (m < 60)
            return m === 1 ? I18n.tr("a minute ago") : I18n.tr("%1 minutes ago").arg(m);
        var h = Math.floor(m / 60);
        if (h < 24)
            return h === 1 ? I18n.tr("an hour ago") : I18n.tr("%1 hours ago").arg(h);
        var d = Math.floor(h / 24);
        return d === 1 ? I18n.tr("a day ago") : I18n.tr("%1 days ago").arg(d);
    }

    readonly property string headline: !root.known ? I18n.tr("Checking for updates.")
        : root.frame.channelPinStale === true ? I18n.tr("Ryoku is not getting updates.")
        : Updates.available ? I18n.tr("An update is ready.")
        : root.systemWaiting && root.systemFull ? I18n.tr("Ryoku is up to date. Your system has updates ready.")
        : I18n.tr("Ryoku is up to date.")

    readonly property var proof: {
        if (!root.known)
            return [];
        var mine = root.release(Updates.currentName, Updates.currentVersion);
        var next = root.release(Updates.latestName !== "" ? Updates.latestName : "", Updates.latestVersion);
        var out = [];
        if (Updates.available && Updates.latestVersion !== "" && Updates.latestVersion !== Updates.currentVersion)
            out = root.spans(I18n.tr("%1 replaces %2."), [next, mine]);
        else if (mine.length > 0)
            out = root.spans(I18n.tr("You have %1."), [mine]);
        if (Updates.available && Updates.behind > 0)
            out = out.concat({ text: " " }, root.spans(Updates.behind === 1 ? I18n.tr("One change.") : I18n.tr("%1 changes.").arg(Updates.behind), []));
        if (Updates.branch !== "")
            out = out.concat({ text: " " }, root.spans(I18n.tr("Follows %1 updates."), [[{ text: Updates.branch, mono: true }]]));
        return out.concat({ text: " " }, root.spans(I18n.tr("Checked %1.").arg(root.checkedAgo), []));
    }

    // what holds the update back or needs a step: { tone, text, action, act }
    readonly property var notices: {
        var out = [];
        var f = root.frame;
        if (Updates.buildState === "building")
            out.push({ tone: "attention",
                text: I18n.tr("The next version is still being built, %1 so far. Update now waits until it is ready.").arg(updateRun.human(Updates.buildMinutes * 60000)),
                action: Updates.buildRun !== "" ? I18n.tr("See progress") : "", act: "build" });
        else if (Updates.buildState === "failed")
            out.push({ tone: "note", text: I18n.tr("The newest version did not build. Updating takes the last version that did."),
                action: Updates.buildRun !== "" ? I18n.tr("See why") : "", act: "build" });
        if (Updates.checkState === "checking")
            out.push({ tone: "note", text: I18n.tr("The newest version is still being tested. Updating takes the last tested version."),
                action: Updates.checkRun !== "" ? I18n.tr("See progress") : "", act: "tests" });
        else if (Updates.checkState === "failed")
            out.push({ tone: "note", text: I18n.tr("The newest version failed its tests. Updating stays on the last version that passed."),
                action: Updates.checkRun !== "" ? I18n.tr("See why") : "", act: "tests" });
        if (f.channelPinStale === true) {
            var back = String(f.recoverChannel || "stable");
            out.push({ tone: "failed",
                text: I18n.tr("This computer is held on a release older than the one it runs, so it gets no updates. Follow %1 again to get them.").arg(back),
                action: I18n.tr("Follow %1 again").arg(back), act: "track", channel: back });
        }
        if (String(f.repositoryNotice || "") !== "")
            out.push({ tone: "note", text: String(f.repositoryNotice), action: "", act: "" });
        if (root.systemWaiting && !root.systemFull)
            out.push({ tone: "note",
                text: Updates.packages.length === 1
                    ? I18n.tr("Your distribution also has one update waiting. Update the whole system too installs it along with Ryoku's.")
                    : I18n.tr("Your distribution also has %1 updates waiting. Update the whole system too installs them along with Ryoku's.").arg(Updates.packages.length),
                action: I18n.tr("Turn on"), act: "system" });
        return out;
    }

    function act(notice) {
        switch (notice.act) {
        case "build":
            Spawn.runApp(["xdg-open", Updates.buildRun], { label: "browser" });
            break;
        case "tests":
            Spawn.runApp(["xdg-open", Updates.checkRun], { label: "browser" });
            break;
        case "track":
            root.openTerminal("ryoku track \"$2\"", [notice.channel]);
            break;
        case "system":
            Changes.set(root.systemBind, true, "Update the whole system too", "updates");
            break;
        }
    }

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: Schema.rows.filter(r => r.template !== true).map(r => r.id !== "ways-back" ? r : Object.assign({}, r, {
            command: ["kitty", "--class=dev.ryoku.update", "-e", "sh", "-c",
                "ryoku rollback; printf '\\n%s ' \"$1\"; read -r _", "sh", I18n.tr("Press Enter to close.")]
        }))
    })

    // drawn rows land on themselves; a row the page draws lands on its section
    function focusRow(rowId, again) {
        if (!rowId)
            return;
        if (page.layout.length === 0) {
            root.pendingRow = rowId;
            return;
        }
        if (again !== true) {
            root.landedRow = rowId;
            root.landedAt = Date.now();
            landSettle.restart();
        }
        if (page.sectionOfRow(rowId)) {
            page.focusRow(rowId);
            return;
        }
        var row = Schema.rows.find(r => r.id === rowId);
        // a drawn row or a section id lands on its section once the fresh page has placed it
        page.focusRow(row ? row.section : rowId);
    }

    RunState { id: updateRun }

    // the frame's Back: the run view steps the keys out of its password field first
    property RunView runView: null
    function back() {
        return !!root.runView && root.runView.back();
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "updates"
        schema: root.schema
        bodies: ({ status: updateRun.shown ? runBody : statusBody, "whats-new": newsBody, rollback: rollbackBody })
        footers: ({ system: packagesBody })
        onLayoutChanged: {
            if (root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    Component.onCompleted: {
        var had = !!Changes.stores.updates;
        Changes.ensureStore("updates", Qt.resolvedUrl("../components/updates/UpdatesStore.qml"));
        if (had) {
            Changes.stores.updates.load("interval");
            Changes.stores.updates.load("system");
        }
        Updates.clockShown = true;
    }
    Component.onDestruction: Updates.clockShown = false

    Component {
        id: statusBody

        Slab {
            id: statusSlab
            // Done or Dismiss dropped the keys with the run view, so they land on the first button
            // (with the keyboard mark only when the keys pressed)
            Component.onCompleted: Qt.callLater(() => {
                var f = statusSlab.Window.activeFocusItem;
                if (f && (f === statusSlab || f === statusSlab.parent))
                    (updateButton.visible ? updateButton : checkButton).forceActiveFocus(updateRun.handReason);
            })
            Column {
                id: statusColumn
                width: parent.width
                spacing: LithicTokens.space.s3

                Label {
                    width: parent.width
                    text: root.headline
                    typeRole: "sentence"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Sentence {
                    width: parent.width
                    visible: root.proof.length > 0
                    spans: root.proof
                    typeRole: "body"
                }

                Repeater {
                    model: root.notices
                    delegate: Row {
                        id: notice
                        required property var modelData
                        width: statusColumn.width
                        spacing: LithicTokens.space.s3

                        Label {
                            anchors.verticalCenter: parent.verticalCenter
                            width: parent.width - (noticeAction.visible ? noticeAction.width + parent.spacing : 0)
                            text: notice.modelData.text
                            typeRole: "detail"
                            ink: notice.modelData.tone === "failed" ? Lithic.jasper
                                : notice.modelData.tone === "attention" ? Lithic.amber : Lithic.fg2
                            wrapMode: Text.Wrap
                            elide: Text.ElideNone
                        }
                        Plaque {
                            id: noticeAction
                            anchors.verticalCenter: parent.verticalCenter
                            visible: notice.modelData.action !== ""
                            kind: "quiet"
                            small: true
                            glyph: notice.modelData.act === "system" ? "" : "open_in_new"
                            text: notice.modelData.action
                            // "Turn on" hides its own notice, so keyboard focus moves on to the next button
                            onClicked: {
                                var moveOn = activeFocus && notice.modelData.act === "system";
                                var keys = noticeAction.visualFocus;
                                var next = updateButton.visible ? updateButton : checkButton;
                                root.act(notice.modelData);
                                if (moveOn)
                                    next.forceActiveFocus(keys ? Qt.TabFocusReason : Qt.MouseFocusReason);
                            }
                        }
                    }
                }

                Item { width: 1; height: LithicTokens.space.s1 }

                Flow {
                    width: parent.width
                    spacing: LithicTokens.space.s2

                    Plaque {
                        id: updateButton
                        visible: root.pendingAction
                        kind: "gold"
                        glyph: "system_update"
                        text: I18n.tr("Update now")
                        enabled: !root.building && !updateRun.starting
                        onClicked: {
                            updateRun.pressedBy(updateButton);
                            root.startUpdate();
                        }
                    }
                    // a running check keeps its plaque (and the keyboard focus) and ignores presses
                    Plaque {
                        id: checkButton
                        kind: "tonal"
                        glyph: "refresh"
                        text: I18n.tr("Check for updates")
                        busy: root.checkSlow
                        onClicked: root.check()
                    }
                    Plaque {
                        id: stopWaiting
                        visible: root.checkLong
                        kind: "quiet"
                        text: I18n.tr("Stop waiting")
                        // the plaque hides as the check settles, so keyboard focus moves to Check for updates
                        onClicked: {
                            var had = stopWaiting.activeFocus, keys = stopWaiting.visualFocus;
                            root.checkSettled();
                            root.checkError = I18n.tr("Stopped waiting. The check goes on in the background and this page updates when it ends.");
                            if (had)
                                checkButton.forceActiveFocus(keys ? Qt.TabFocusReason : Qt.MouseFocusReason);
                        }
                    }
                }

                BladeProgress {
                    width: parent.width
                    visible: root.checkLong
                    active: root.checkLong
                    indeterminate: true
                }
                Label {
                    width: parent.width
                    visible: root.checkSlow
                    text: I18n.tr("Checking for updates. This can take a minute while your distribution's package lists refresh.")
                    typeRole: "detail"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }

                Row {
                    width: parent.width
                    visible: root.checkError !== ""
                    spacing: LithicTokens.space.s3
                    Label {
                        anchors.verticalCenter: parent.verticalCenter
                        width: parent.width - checkRetry.width - parent.spacing
                        text: root.checkError
                        typeRole: "detail"
                        ink: Lithic.jasper
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }
                    Plaque {
                        id: checkRetry
                        anchors.verticalCenter: parent.verticalCenter
                        kind: "quiet"
                        small: true
                        glyph: "refresh"
                        text: I18n.tr("Retry")
                        onClicked: {
                            var had = checkRetry.activeFocus, keys = checkRetry.visualFocus;
                            root.check();
                            if (had)
                                checkButton.forceActiveFocus(keys ? Qt.TabFocusReason : Qt.MouseFocusReason);
                        }
                    }
                }

                Row {
                    width: parent.width
                    visible: updateRun.actionError !== ""
                    spacing: LithicTokens.space.s3
                    Label {
                        anchors.verticalCenter: parent.verticalCenter
                        width: parent.width - startRetry.width - parent.spacing
                        text: updateRun.actionError
                        typeRole: "detail"
                        ink: Lithic.jasper
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }
                    Plaque {
                        id: startRetry
                        anchors.verticalCenter: parent.verticalCenter
                        visible: root.pendingAction
                        kind: "quiet"
                        small: true
                        glyph: "refresh"
                        text: I18n.tr("Retry")
                        onClicked: {
                            updateRun.pressedBy(startRetry);
                            root.startUpdate();
                        }
                    }
                }
            }
        }
    }

    Component {
        id: runBody

        Slab {
            RunView {
                width: parent.width
                run: updateRun
                hub: root.hub
                Component.onCompleted: root.runView = this
            }
        }
    }

    Component {
        id: newsBody

        Column {
            id: news
            spacing: LithicTokens.space.s2

            readonly property var items: Updates.available ? Updates.updates : Updates.recent

            Label {
                width: parent.width
                text: Updates.available ? I18n.tr("Coming in this update") : I18n.tr("Already on this computer")
                typeRole: "detail"
                inkRole: "fg2"
            }
            Slab {
                width: parent.width

                Column {
                    width: parent.width

                    ChangeList {
                        width: parent.width
                        items: news.items
                    }
                    Label {
                        width: parent.width
                        visible: news.items.length === 0
                        text: !root.known ? I18n.tr("The list appears after the first check.")
                            : Updates.available ? I18n.tr("This update came with no list of changes.")
                            : I18n.tr("The latest changes show here after the next check.")
                        typeRole: "detail"
                        inkRole: "fg3"
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }
                }
            }
        }
    }

    Component {
        id: packagesBody

        Column {
            spacing: LithicTokens.space.s2

            Label {
                width: parent.width
                visible: root.known
                text: !root.systemWaiting ? I18n.tr("Nothing is waiting from your distribution.")
                    : Updates.packages.length === 1 ? I18n.tr("One update is waiting from your distribution.")
                    : I18n.tr("%1 updates are waiting from your distribution.").arg(Updates.packages.length)
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Disclosure {
                id: packagesFold
                width: parent.width
                visible: root.systemWaiting
                text: I18n.tr("Show the packages")

                Loader {
                    width: packagesFold.width
                    active: packagesFold.open
                    sourceComponent: ChangeList {
                        width: packagesFold.width
                        items: Updates.packages
                    }
                }
            }
        }
    }

    Component {
        id: rollbackBody

        Column {
            spacing: LithicTokens.space.s2

            Label {
                width: parent.width
                text: Updates.snapshotsSupported
                    ? I18n.tr("Before every update Ryoku saves a snapshot of the whole system. If an update leaves the computer unable to start or work properly, choose that snapshot in the boot menu to return to how it was.")
                    : I18n.tr("This computer cannot keep snapshots of the whole system: %1").arg(Updates.snapshotReason !== "" ? Updates.snapshotReason : I18n.tr("its disk setup does not support them."))
                typeRole: "body"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Label {
                width: parent.width
                text: Updates.snapshotsSupported
                    ? I18n.tr("Ryoku itself can also move back to an earlier release without a restart.")
                    : I18n.tr("Ryoku itself can still move back to an earlier release without a restart.")
                typeRole: "body"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Label {
                width: parent.width
                visible: Updates.snapshotsSupported && root.known
                text: root.frame.snapshotsKnown === true
                    ? (root.frame.snapshots === 1 ? I18n.tr("One snapshot is on this computer.") : I18n.tr("%1 snapshots are on this computer.").arg(root.frame.snapshots || 0))
                    : I18n.tr("The number of snapshots could not be read. Ryoku doctor can grant the access it needs.")
                typeRole: "detail"
                inkRole: "fg3"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }
    }
}
