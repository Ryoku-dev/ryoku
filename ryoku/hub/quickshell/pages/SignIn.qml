import QtQuick
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/sign-in"
import "../schema/sign-in.js" as Schema

// the keyring, fingerprints and security keys, and where each stands in for the password
Item {
    id: root

    required property var hub
    property string pendingRow: ""
    property string spot: ""

    readonly property var store: Changes.stores["signin"] || null
    readonly property var busy: root.store ? root.store.busy : ({})
    // groups whose write has run past a second, and past ten (they offer Stop waiting)
    property var slowGroups: ({})
    property var longGroups: ({})

    property string keyringMessage: ""
    property bool keyringFailed: false
    // what Retry does after a keyring failure: start fresh again, or (null) back to the password
    property var keyringRetry: null
    // the keyring work running from this page: "convert", "reset" or ""
    property string keyringJob: ""
    property string printsMessage: ""
    property bool printsFailed: false
    property var printsRetry: null
    property string keysMessage: ""
    property bool keysFailed: false

    // adding a security key in a terminal: idle, waiting or stopped
    property string keyFlow: "idle"
    property int keysBefore: 0
    property int keyPolls: 0
    property bool keyFlowSlow: false

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: Schema.rows.filter(r => r.template !== true)
    })

    readonly property string stateLine: {
        Changes.revision;
        var s = root.store;
        if (!s)
            return "";
        var parts = [];
        if (s.kloaded)
            parts.push(s.kmode === "never-ask" ? I18n.tr("Keyring never asks")
                : s.kmode === "ask" ? I18n.tr("Keyring asks each time")
                : I18n.tr("Keyring opens at sign-in"));
        if (s.floaded && s.fready)
            parts.push(s.fingers.length === 1 ? I18n.tr("1 fingerprint") : I18n.tr("%1 fingerprints").arg(s.fingers.length));
        if (s.skLoaded && s.sk.supported === true && s.sk.credentials > 0)
            parts.push(s.sk.credentials === 1 ? I18n.tr("1 security key") : I18n.tr("%1 security keys").arg(s.sk.credentials));
        return parts.join(". ");
    }

    readonly property var fingerOrder: ["right-index-finger", "right-thumb", "right-middle-finger", "right-ring-finger", "right-little-finger",
                                        "left-index-finger", "left-thumb", "left-middle-finger", "left-ring-finger", "left-little-finger"]

    function fingerLabel(f) {
        switch (f) {
        case "left-thumb": return I18n.tr("Left thumb");
        case "left-index-finger": return I18n.tr("Left index finger");
        case "left-middle-finger": return I18n.tr("Left middle finger");
        case "left-ring-finger": return I18n.tr("Left ring finger");
        case "left-little-finger": return I18n.tr("Left little finger");
        case "right-thumb": return I18n.tr("Right thumb");
        case "right-index-finger": return I18n.tr("Right index finger");
        case "right-middle-finger": return I18n.tr("Right middle finger");
        case "right-ring-finger": return I18n.tr("Right ring finger");
        case "right-little-finger": return I18n.tr("Right little finger");
        }
        return String(f).replace(/-/g, " ");
    }

    readonly property var prints: {
        Changes.revision;
        if (!root.store)
            return [];
        return root.store.fingers.map(f => {
            var named = root.store.fnames[f];
            return { finger: f, name: named || "", title: named || root.fingerLabel(f), detail: named ? root.fingerLabel(f) : "" };
        });
    }
    readonly property var freeFingers: root.fingerOrder.filter(f => !root.store || root.store.fingers.indexOf(f) < 0)

    function sectionIndex(id) {
        for (var i = 0; i < page.layout.length; i++)
            if (page.layout[i].id === id)
                return i;
        return -1;
    }

    // every backend answered, or failed to answer
    readonly property bool settled: {
        Changes.revision;
        var s = root.store;
        return !!s && (s.kloaded || s.kreadError !== "") && s.floaded && s.fpamLoaded && s.skLoaded;
    }
    onSettledChanged: {
        if (root.settled && root.pendingRow !== "") {
            var id = root.pendingRow;
            root.pendingRow = "";
            Qt.callLater(root.focusRow, id);
        }
    }

    function focusRow(rowId) {
        if (!rowId)
            return;
        // rows appear as each backend answers, so land once they all have
        if (page.layout.length === 0 || !root.settled) {
            root.pendingRow = rowId;
            return;
        }
        landLater.rowId = rowId;
        landLater.restart();
    }

    function land(rowId) {
        if (page.sectionOfRow(rowId)) {
            page.focusRow(rowId);
            return;
        }
        var row = Schema.rows.find(r => r.id === rowId);
        if (!row) {
            // a section rather than a row: the frame scrolls to it
            page.focusRow(rowId);
            return;
        }
        var at = root.sectionIndex(row.section);
        if (at >= 0)
            page.chooseSection(at);
        root.spot = row.id;
        spotClear.restart();
    }

    // the section a running write belongs to, so its wait shows where it was started
    function sectionOfWork(what) {
        var row = Schema.rows.find(r => r.bind && r.bind.key === what);
        if (row)
            return row.section;
        return what === "key.remove" ? "security-keys" : what === "fingerprint.delete" ? "fingerprint" : "keyring";
    }

    Timer {
        id: landLater
        property string rowId: ""
        interval: 150
        onTriggered: root.land(landLater.rowId)
    }

    onBusyChanged: {
        var slow = {}, lasting = {};
        for (var g in root.busy) {
            if (root.slowGroups[g])
                slow[g] = true;
            if (root.longGroups[g])
                lasting[g] = true;
        }
        root.slowGroups = slow;
        root.longGroups = lasting;
        if (Object.keys(root.busy).length > Object.keys(slow).length) {
            slowMark.restart();
            longMark.restart();
        }
    }

    // keyring

    function convertKeyring(password) {
        if (!root.store || password.length === 0 || root.keyringJob !== "" || root.busy.keyring || root.store.convertFor === "")
            return;
        root.keyringMessage = "";
        root.keyringRetry = null;
        root.keyringJob = "convert";
        root.store.convertKeyring(password, (ok, message) => {
            root.keyringJob = "";
            root.keyringFailed = !ok;
            root.keyringMessage = ok ? I18n.tr("Your keyring has no password now, so it never asks.") : message;
        });
    }

    // a new password prompt makes an earlier keyring success out of date
    Connections {
        target: root.store
        function onConvertForChanged() {
            if (root.store.convertFor !== "" && !root.keyringFailed)
                root.keyringMessage = "";
        }
    }

    function resetKeyring() {
        root.hub.confirm({
            title: I18n.tr("Start a fresh keyring?"),
            text: I18n.tr("Your saved passwords move to a backup and an empty keyring without a password takes their place. Apps that saved a password ask you to sign in again."),
            actionText: I18n.tr("Start fresh")
        }, root.startFresh, null);
    }

    function startFresh() {
        root.keyringMessage = "";
        if (!root.store || root.keyringJob !== "" || root.busy.keyring || root.store.convertFor === "")
            return;
        root.keyringRetry = root.startFresh;
        root.keyringJob = "reset";
        root.store.resetKeyring((ok, message) => {
            root.keyringJob = "";
            root.keyringFailed = !ok;
            root.keyringMessage = ok ? I18n.tr("A fresh keyring is in place. The old one is kept as a backup.") : message;
        });
    }

    // fingerprints

    function removePrint(index) {
        var p = root.prints[index];
        if (!p)
            return;
        root.hub.confirm({
            title: I18n.tr("Remove %1?").arg(p.title),
            text: I18n.tr("This fingerprint stops working everywhere it was used. You can add it again later."),
            actionText: I18n.tr("Remove")
        }, function () { root.deletePrints(p.finger); }, null);
    }

    function removeAllPrints() {
        if (root.busy.fingerprint)
            return;
        var n = root.prints.length;
        root.hub.confirm({
            title: I18n.tr("Remove every fingerprint?"),
            text: n === 1 ? I18n.tr("Your fingerprint stops working on the sign-in screen, the lock screen and everywhere else it was used.")
                          : I18n.tr("All %1 fingerprints stop working on the sign-in screen, the lock screen and everywhere else they were used.").arg(n),
            actionText: I18n.tr("Remove all")
        }, function () { root.deletePrints(""); }, null);
    }

    function deletePrints(finger) {
        root.printsMessage = "";
        root.printsRetry = () => root.deletePrints(finger);
        root.store.deleteFinger(finger, (ok, message) => {
            root.printsFailed = !ok;
            root.printsMessage = ok ? (finger ? I18n.tr("Fingerprint removed.") : I18n.tr("Every fingerprint is removed.")) : message;
        });
    }

    function renamePrint(index, item) {
        var p = root.prints[index];
        if (!p)
            return;
        root.printsMessage = "";
        root.printsRetry = () => root.renamePrint(index, item);
        root.store.nameFinger(p.finger, item.name, (ok, message) => {
            root.printsFailed = !ok;
            root.printsMessage = ok ? "" : message;
        });
    }

    // security keys

    function addKey() {
        if (!root.store || root.keyFlow === "waiting")
            return;
        root.keysMessage = "";
        root.keysBefore = root.store.sk.credentials || 0;
        root.keyPolls = 0;
        root.keyFlowSlow = false;
        root.keyFlow = "waiting";
        root.keyStartedAt = Date.now();
        root.keyRun = root.hub.runInTerminal("ryoku security-key enroll", "dev.ryoku.passkey", (ok, code) => root.keyTerminalClosed());
        keyPoll.restart();
        keySlow.restart();
    }

    function keyFlowSettled(added) {
        keyPoll.stop();
        keySlow.stop();
        root.keyFlowSlow = false;
        root.keyFlow = "idle";
        root.keysFailed = !added;
        root.keysMessage = added ? I18n.tr("Security key added. Turn it on below for the places you want it.")
                                 : I18n.tr("No new key was added. If the terminal showed an error, fix it, then try again.");
    }

    function stopKeyFlow() {
        if (root.keyRun)
            root.keyRun.signal(15);
        keyPoll.stop();
        keySlow.stop();
        root.keyFlow = "idle";
        root.keyFlowSlow = false;
        root.keysFailed = true;
        root.keysMessage = I18n.tr("Stopped waiting. If you finish in the terminal later, the key shows up here when you check again.");
    }

    function removeKey(cred) {
        root.hub.confirm({
            title: I18n.tr("Remove %1?").arg(cred.label || I18n.tr("Security key %1").arg(cred.id)),
            text: I18n.tr("This key stops working everywhere it was used. You can add it again later."),
            actionText: I18n.tr("Remove")
        }, function () {
            root.keysMessage = "";
            root.store.removeKey(cred.id, (ok, message) => {
                root.keysFailed = !ok;
                root.keysMessage = ok ? I18n.tr("Security key removed.") : message;
            });
        }, null);
    }

    readonly property bool keyAdded: {
        Changes.revision;
        return !!root.store && root.keyFlow === "waiting" && (root.store.sk.credentials || 0) > root.keysBefore;
    }
    onKeyAddedChanged: {
        if (root.keyAdded)
            Qt.callLater(root.keyFlowSettled, true);
    }

    Component.onCompleted: {
        if (root.store)
            root.store.reload();
        else
            Changes.ensureStore("signin", Qt.resolvedUrl("../components/sign-in/SignInStore.qml"));
    }

    Timer {
        id: spotClear
        interval: LithicTokens.motion.highlight + 400
        onTriggered: root.spot = ""
    }

    Timer {
        id: slowMark
        interval: 1000
        onTriggered: root.slowGroups = Object.assign({}, root.busy)
    }

    Timer {
        id: longMark
        interval: 10000
        onTriggered: root.longGroups = Object.assign({}, root.busy)
    }

    // the terminal may return at once, so the key list is read until the key shows up
    Timer {
        id: keyPoll
        interval: 2500
        repeat: true
        onTriggered: {
            root.keyPolls++;
            root.store.readKey();
            if (root.keyPolls >= 24 && root.keyRun === null)
                root.keyFlowSettled(false);
        }
    }

    Timer {
        id: keySlow
        interval: 10000
        onTriggered: root.keyFlowSlow = true
    }

    // the enrolment terminal, while it is open
    property var keyRun: null
    property real keyStartedAt: 0

    function keyTerminalClosed() {
        root.keyRun = null;
        root.store.readKey();
        // a terminal that stayed open until closed means the attempt is over
        if (root.keyFlow === "waiting" && Date.now() - root.keyStartedAt > 3000)
            settleAfterRead.restart();
    }

    Timer {
        id: settleAfterRead
        interval: 1500
        onTriggered: {
            if (root.keyFlow === "waiting")
                root.keyFlowSettled((root.store.sk.credentials || 0) > root.keysBefore);
        }
    }

    FingerprintDialog {
        id: fingerprintDialog
        parent: root.hub.overlayItem
        store: root.store
        onFinished: ok => root.store.readFingerprints()
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "sign-in"
        schema: root.schema
        stateLine: root.stateLine
        bodies: ({ fingerprint: printsBody, "security-keys": keysBody })
        footers: ({ keyring: keyringFooter, "where-used": whereFooter })
        onLayoutChanged: {
            if (root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    component Note: Label {
        width: parent ? parent.width : 0
        typeRole: "detail"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    component Waiting: Row {
        property string text: ""
        width: parent ? parent.width : 0
        spacing: LithicTokens.space.s2
        Working {
            anchors.verticalCenter: parent.verticalCenter
            active: parent.visible
        }
        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: parent.width - LithicTokens.space.s2 - LithicTokens.size.iconPlaqueIcon
            text: parent.text
            typeRole: "detail"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }

    component Outcome: Row {
        id: outcome
        property string text: ""
        property bool failed: false
        signal retry()
        visible: outcome.text.length > 0
        width: parent ? parent.width : 0
        spacing: LithicTokens.space.s3
        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, outcome.width - (retryPlaque.visible ? retryPlaque.width + LithicTokens.space.s3 : 0))
            text: outcome.text
            typeRole: "detail"
            ink: outcome.failed ? Lithic.jasper : Lithic.fg2
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: retryPlaque
            visible: outcome.failed
            kind: "quiet"
            small: true
            glyph: "refresh"
            text: I18n.tr("Retry")
            onClicked: outcome.retry()
        }
    }

    // a write waiting past a second, in the section it was started from; past ten it can be
    // stopped, on the same line so the rows below do not move again
    component Stall: Row {
        id: stall
        required property string section
        readonly property var groups: Object.keys(root.slowGroups).filter(g => !!root.store && root.sectionOfWork(root.store.busyWhat[g] || "") === stall.section)
        visible: stall.groups.length > 0
        width: parent ? parent.width : 0
        spacing: LithicTokens.space.s2

        Working {
            anchors.verticalCenter: parent.verticalCenter
            active: stall.visible
        }
        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: stall.width - LithicTokens.size.iconPlaqueIcon - LithicTokens.space.s2 * 2 - stopWaiting.width
            text: I18n.tr("Waiting for your password in the admin prompt")
            typeRole: "detail"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: stopWaiting
            anchors.verticalCenter: parent.verticalCenter
            opacity: stall.groups.some(g => !!root.longGroups[g]) ? 1 : 0
            enabled: opacity > 0
            kind: "quiet"
            small: true
            text: I18n.tr("Stop waiting")
            onClicked: stall.groups.forEach(g => root.store.cancel(g))
        }
    }

    Component {
        id: keyringFooter

        Column {
            spacing: LithicTokens.space.s2

            Note {
                visible: !!root.store && root.store.kloaded
                text: {
                    var s = root.store;
                    if (!s)
                        return "";
                    var f = s.kdefFormat === "encrypted" ? I18n.tr("Your keyring has a password.")
                        : s.kdefFormat === "plaintext" ? I18n.tr("Your keyring has no password.")
                        : I18n.tr("There is no keyring yet. One is made the first time an app saves a password.");
                    return f + " " + (s.kdaemon ? I18n.tr("The keyring service is running.") : I18n.tr("The keyring service is not running."));
                }
            }
            // the caveats a bare mode cannot show; the command line's notes use its own ids, so
            // the page says the same in its own words
            Note {
                visible: text.length > 0 && root.store.convertFor === ""
                text: {
                    var s = root.store;
                    if (!s || !s.kloaded || s.kmode !== "unlock-on-login")
                        return "";
                    if (s.kautologin)
                        return I18n.tr("You sign in automatically, so there is no login password to open the keyring with. Choose Never ask so apps do not keep asking.");
                    if (s.kdefFormat === "encrypted")
                        return I18n.tr("Your keyring opens at sign-in only if its password is the same as your login password.");
                    return "";
                }
                inkRole: "fg3"
            }
            Outcome {
                text: root.store ? root.store.kreadError : ""
                failed: true
                onRetry: root.store.readKeyring()
            }

            Slab {
                visible: !!root.store && root.store.convertFor !== ""
                width: parent.width

                Column {
                    width: parent.width
                    spacing: LithicTokens.space.s3

                    Label {
                        width: parent.width
                        text: I18n.tr("Your keyring is locked with a password, so it keeps asking. Type that password to remove it, or start fresh. Starting fresh keeps your old keyring as a backup and never deletes it.")
                        typeRole: "body"
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }
                    TextField {
                        id: keyringPassword
                        width: Math.min(parent.width, LithicTokens.size.settingsControlMax)
                        label: I18n.tr("Keyring password")
                        secret: true
                        // stays enabled while it works, so the focus stays here; a second Return waits
                        onAccepted: root.convertKeyring(text)
                    }
                    Row {
                        spacing: LithicTokens.space.s2
                        Plaque {
                            kind: "line"
                            small: true
                            text: I18n.tr("Remove the password")
                            enabled: keyringPassword.text.length > 0
                            busy: root.keyringJob === "convert"
                            onClicked: root.convertKeyring(keyringPassword.text)
                        }
                        Plaque {
                            kind: "dangerLine"
                            small: true
                            text: I18n.tr("Start fresh")
                            busy: root.keyringJob === "reset"
                            onClicked: {
                                if (root.keyringJob === "")
                                    root.resetKeyring();
                            }
                        }
                    }
                }
            }

            Stall {
                section: "keyring"
            }
            Outcome {
                text: root.keyringMessage
                failed: root.keyringFailed
                onRetry: {
                    root.keyringMessage = "";
                    if (root.keyringRetry)
                        root.keyringRetry();
                    else
                        keyringPassword.forceActiveFocus();
                }
            }
        }
    }

    Component {
        id: printsBody

        Slab {
            Column {
                width: parent.width
                spacing: 0

                SettingRow {
                    width: parent.width
                    first: true
                    highlighted: root.spot === "fingerprints" || root.spot === "remove-fingerprints"
                    title: {
                        var s = root.store;
                        if (!s || !s.floaded)
                            return I18n.tr("Looking for a fingerprint sensor");
                        if (!s.fdaemon && s.freadError !== "")
                            return I18n.tr("The fingerprint service is not answering");
                        if (!s.fready)
                            return I18n.tr("No fingerprint sensor found");
                        return s.fdeviceName !== "" ? s.fdeviceName : I18n.tr("Fingerprint sensor");
                    }
                    detail: {
                        var s = root.store;
                        if (!s || !s.floaded)
                            return "";
                        if (!s.fdaemon && s.freadError !== "")
                            return I18n.tr("Check again. If a sensor is connected and this keeps happening, make sure the fprintd package is installed.");
                        if (!s.fready)
                            return I18n.tr("Connect a sensor, then check again. Without one, your password still works everywhere.");
                        if (s.fingers.length === 0)
                            return I18n.tr("Ready. Add a fingerprint to unlock with a touch.");
                        return s.fingers.length === 1 ? I18n.tr("1 fingerprint added") : I18n.tr("%1 fingerprints added").arg(s.fingers.length);
                    }

                    Working {
                        visible: !root.store || !root.store.floaded
                        active: visible
                    }
                    Plaque {
                        visible: !!root.store && root.store.floaded && !root.store.fready
                        kind: "quiet"
                        small: true
                        glyph: "refresh"
                        text: I18n.tr("Check again")
                        onClicked: root.store.readFingerprints()
                    }
                }

                Item {
                    visible: printList.visible
                    width: 1
                    height: LithicTokens.size.settingsRowGap
                }

                ListEditor {
                    id: printList
                    visible: !!root.store && root.store.fready
                    width: parent.width
                    accessibleName: I18n.tr("Fingerprints")
                    model: root.prints
                    addText: I18n.tr("Add a fingerprint")
                    doneText: I18n.tr("Scan")
                    emptyText: I18n.tr("No fingerprints yet")
                    enabled: !root.busy.fingerprint
                    editor: printEditor
                    trailing: printTest
                    onAdded: item => {
                        root.printsMessage = "";
                        fingerprintDialog.start("enroll", item.finger, item.name);
                    }
                    onEdited: (index, item) => root.renamePrint(index, item)
                    onRemoved: index => root.removePrint(index)
                }

                Row {
                    visible: printList.visible && root.prints.length > 0
                    topPadding: LithicTokens.space.s3
                    spacing: LithicTokens.space.s2
                    Plaque {
                        kind: "quiet"
                        small: true
                        glyph: "fingerprint"
                        text: I18n.tr("Test a fingerprint")
                        enabled: !root.busy.fingerprint
                        onClicked: fingerprintDialog.start("verify", "", "")
                    }
                    Plaque {
                        kind: "dangerLine"
                        small: true
                        text: I18n.tr("Remove all")
                        busy: !!root.busy.fingerprint
                        // stays enabled while busy (it shows the spinner), so focus is not dropped mid-removal
                        onClicked: root.removeAllPrints()
                    }
                }

                Outcome {
                    topPadding: LithicTokens.space.s3
                    text: root.printsMessage
                    failed: root.printsFailed
                    onRetry: {
                        if (root.printsRetry)
                            root.printsRetry();
                    }
                }

                Stall {
                    topPadding: LithicTokens.space.s3
                    section: "fingerprint"
                }
            }
        }
    }

    Component {
        id: printEditor

        Column {
            id: ed
            property var item
            readonly property bool adding: !ed.item || ed.item.finger === undefined
            readonly property bool valid: !ed.adding || fingerPick.currentIndex >= 0
            function value() {
                var name = nameField.text.trim();
                return ed.adding ? { finger: root.freeFingers[fingerPick.currentIndex], name: name }
                                 : { finger: ed.item.finger, name: name };
            }
            spacing: LithicTokens.space.s3

            Picker {
                id: fingerPick
                visible: ed.adding
                width: Math.min(parent.width, LithicTokens.size.settingsControlMax)
                label: I18n.tr("Finger")
                options: root.freeFingers.map(f => root.fingerLabel(f))
                currentIndex: root.freeFingers.length > 0 ? 0 : -1
                overlayParent: root.hub.overlayItem
                onActivated: index => fingerPick.currentIndex = index
            }
            TextField {
                id: nameField
                width: Math.min(parent.width, LithicTokens.size.settingsControlMax)
                label: I18n.tr("Name")
                placeholderText: I18n.tr("Optional, for example Work hand")
                text: ed.item && ed.item.name ? ed.item.name : ""
                helpText: ed.adding ? I18n.tr("Scan opens the sensor. Touch it a few times until it is done.") : ""
            }
        }
    }

    Component {
        id: printTest

        Plaque {
            property var modelData
            property int index
            kind: "quiet"
            small: true
            text: I18n.tr("Test")
            accessibleName: I18n.tr("Test %1").arg(modelData ? modelData.title : "")
            enabled: !root.busy.fingerprint
            onClicked: fingerprintDialog.start("verify", modelData.finger, modelData.title)
        }
    }

    Component {
        id: keysBody

        Slab {
            Column {
                width: parent.width
                spacing: 0

                SettingRow {
                    width: parent.width
                    first: true
                    highlighted: root.spot === "security-key-list"
                    readonly property var sk: root.store ? root.store.sk : ({})
                    title: {
                        var s = root.store;
                        if (!s || !s.skLoaded)
                            return I18n.tr("Looking for a security key");
                        if (s.skReadError !== "")
                            return I18n.tr("Security keys could not be read");
                        if (sk.supported !== true)
                            return I18n.tr("Security keys are not set up on this computer");
                        return sk.devicePresent === true ? (sk.deviceName || I18n.tr("Security key plugged in")) : I18n.tr("No security key plugged in");
                    }
                    detail: {
                        var s = root.store;
                        if (!s || !s.skLoaded)
                            return "";
                        if (s.skReadError !== "")
                            return s.skReadError;
                        if (sk.supported !== true)
                            return I18n.tr("Security keys need the pam-u2f package. Install it, then check again.");
                        if (sk.devicePresent !== true)
                            return I18n.tr("Plug in your YubiKey or other FIDO key, then check again.");
                        return sk.enrolled === true ? I18n.tr("Add another key, or remove one you no longer use.")
                                                    : I18n.tr("Set up the key that is plugged in to use it here.");
                    }

                    Working {
                        visible: !root.store || !root.store.skLoaded
                        active: visible
                    }
                    Plaque {
                        visible: !!root.store && root.store.skLoaded && (root.store.skReadError !== "" || root.store.sk.supported !== true || root.store.sk.devicePresent !== true)
                        kind: "quiet"
                        small: true
                        glyph: "refresh"
                        text: I18n.tr("Check again")
                        onClicked: root.store.readKey()
                    }
                }

                Repeater {
                    model: root.store && root.store.sk.supported === true ? (root.store.sk.credentialIds || []) : []
                    delegate: SettingRow {
                        required property var modelData
                        width: parent.width
                        title: modelData.label || I18n.tr("Security key %1").arg(modelData.id)
                        detail: I18n.tr("Works wherever security keys are turned on below")

                        Plaque {
                            kind: "quiet"
                            small: true
                            text: I18n.tr("Remove")
                            enabled: !root.busy.key
                            accessibleName: I18n.tr("Remove %1").arg(modelData.label || modelData.id)
                            onClicked: root.removeKey(modelData)
                        }
                    }
                }

                Column {
                    visible: !!root.store && root.store.sk.supported === true
                    width: parent.width
                    topPadding: LithicTokens.space.s3
                    spacing: LithicTokens.space.s2

                    Row {
                        spacing: LithicTokens.space.s2
                        Plaque {
                            kind: "line"
                            small: true
                            glyph: "add"
                            text: root.store && root.store.sk.enrolled === true ? I18n.tr("Add a security key") : I18n.tr("Set up a security key")
                            enabled: root.keyFlow !== "waiting" && !root.busy.key && !!root.store && root.store.sk.devicePresent === true
                            busy: root.keyFlow === "waiting"
                            onClicked: root.addKey()
                        }
                        Plaque {
                            visible: root.keyFlow === "waiting" && root.keyFlowSlow
                            kind: "quiet"
                            small: true
                            text: I18n.tr("Stop waiting")
                            onClicked: root.stopKeyFlow()
                        }
                    }
                    Waiting {
                        visible: root.keyFlow === "waiting"
                        text: I18n.tr("Finish in the terminal window: touch your key, and type its PIN if it asks.")
                    }
                    BladeProgress {
                        visible: root.keyFlow === "waiting" && root.keyFlowSlow
                        width: parent.width
                        active: visible
                        indeterminate: true
                    }
                    Note {
                        text: I18n.tr("Setting up a key happens in a terminal window, because the key may ask for its PIN there.")
                        inkRole: "fg3"
                    }
                    Outcome {
                        text: root.keysMessage
                        failed: root.keysFailed
                        onRetry: {
                            root.keysMessage = "";
                            root.store.readKey();
                        }
                    }
                    Stall {
                        section: "security-keys"
                    }
                }
            }
        }
    }

    Component {
        id: whereFooter

        Column {
            spacing: LithicTokens.space.s2

            Note {
                text: I18n.tr("Your password always works, everywhere. These add another way in.")
                inkRole: "fg3"
            }
            Note {
                readonly property var s: root.store
                visible: !!s && s.fpamLoaded && !s.fpamSupported && s.fpamReadError === ""
                text: !s ? "" : I18n.tr("Fingerprints at sign-in, for sudo and for admin prompts are not available here: %1")
                    .arg((s.fpamReason !== "" ? I18n.tr(s.fpamReason) : I18n.tr("this edition does not include fingerprint sign-in.")).replace(/[.\s]+$/, "") + ".")
                inkRole: "fg3"
            }
            Outcome {
                text: root.store ? root.store.fpamReadError : ""
                failed: true
                onRetry: root.store.readFingerprintPam()
            }
            Note {
                readonly property var s: root.store
                visible: !!s && s.floaded && s.fready && s.fingers.length === 0
                text: I18n.tr("Add a fingerprint above before it can unlock anything.")
                inkRole: "fg3"
            }
            Note {
                readonly property var s: root.store
                visible: !!s && s.skLoaded && s.sk.supported === true && s.sk.enrolled !== true
                text: I18n.tr("Set up a security key above to use it here.")
                inkRole: "fg3"
            }
            Note {
                readonly property var s: root.store
                visible: !!s && s.skLoaded && s.sk.enrolled === true && s.sk.lockSupported !== true
                text: I18n.tr("A security key cannot unlock the lock screen while it must be used together with your password.")
                inkRole: "fg3"
            }
            Note {
                readonly property var s: root.store
                visible: !!s && s.skLoaded && s.sk.lockPin === true && s.sk.lock === true
                text: I18n.tr("On the lock screen your key asks for its PIN, which needs a design that can show it, such as Orbital.")
                inkRole: "fg3"
            }
            Stall {
                section: "where-used"
            }
        }
    }
}
