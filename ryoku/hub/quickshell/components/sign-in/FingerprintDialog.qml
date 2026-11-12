import QtQuick
import QtQuick.Templates as T
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// adding or testing a fingerprint: fprintd as this user, every way out ends in a result
Dialog {
    id: root

    required property var store
    property string mode: "enroll"
    property string finger: ""
    property string name: ""
    // running, done or failed
    property string phase: ""
    property string status: ""
    property string outcome: ""
    property string log: ""
    property int stagesPassed: 0
    property int stagesTotal: 0
    property var offsets: ({})
    property bool slow: false
    property bool stopped: false

    signal finished(bool ok)

    readonly property bool running: root.phase === "running"

    title: root.mode === "enroll" ? I18n.tr("Add a fingerprint") : I18n.tr("Test a fingerprint")
    closePolicy: root.running ? T.Popup.CloseOnEscape : (T.Popup.CloseOnEscape | T.Popup.CloseOnPressOutside)
    actions: root.running ? [{ text: I18n.tr("Stop"), kind: "quiet", closeOnTrigger: false }]
        : root.phase === "done" ? [{ text: I18n.tr("Done"), kind: "gold" }]
        : [{ text: I18n.tr("Close"), kind: "quiet" }, { text: I18n.tr("Try again"), kind: "line", closeOnTrigger: false }]

    onActionTriggered: index => {
        if (root.running)
            root.stop();
        else if (root.phase === "failed" && index === 1)
            root.begin();
    }
    onClosed: {
        if (root.running)
            root.stop();
        root.finished(root.phase === "done");
    }

    function start(mode, finger, name) {
        root.mode = mode;
        root.finger = finger || "";
        root.name = name || "";
        root.open();
        root.begin();
    }

    function begin() {
        root.phase = "running";
        root.stopped = false;
        root.status = "";
        root.outcome = "";
        root.log = "";
        root.stagesPassed = 0;
        root.stagesTotal = 0;
        root.offsets = {};
        root.slow = false;
        slowMark.restart();
        var f = root.finger !== "" ? ["-f", root.finger] : [];
        scan.launched = false;
        scan.command = root.mode === "enroll" ? ["fprintd-enroll"].concat(f) : ["fprintd-verify"].concat(f);
        scan.running = true;
        if (root.mode === "enroll") {
            stagesRetry.tries = 0;
            stages.running = true;
        }
    }

    function stop() {
        if (scan.running)
            scan.signal(15);
        root.phase = "failed";
        root.stopped = true;
        root.outcome = root.mode === "enroll" ? I18n.tr("No fingerprint was added.") : I18n.tr("The test ended before a match.");
    }

    // fresh bytes of one stream: the log strip and each status token, counted once
    function pump(stream, full) {
        full = full || "";
        var start = root.offsets[stream] || 0;
        if (full.length <= start)
            return;
        var chunk = full.substring(start);
        root.offsets[stream] = full.length;
        var lines = chunk.split("\n").map(s => s.trim()).filter(s => s !== "" && !/^Using device/i.test(s));
        if (lines.length > 0)
            root.log = (root.log + "\n" + lines.join("\n")).trim().split("\n").slice(-2).join("\n");
        var m, re = /(?:Enroll result|Verify result):\s*([a-z0-9-]+)/gi;
        while ((m = re.exec(chunk)) !== null) {
            root.status = m[1].toLowerCase();
            if (root.status === "enroll-stage-passed")
                root.stagesPassed++;
        }
    }

    function headline() {
        if (root.stopped)
            return I18n.tr("Stopped");
        var s = root.status;
        var pre = root.mode === "enroll" ? "enroll-" : "verify-";
        if (s.indexOf(pre) === 0)
            s = s.slice(pre.length);
        // it ended before the sensor said anything: the outcome below says why
        if (root.phase === "failed" && s === "")
            return root.mode === "enroll" ? I18n.tr("Could not add a fingerprint") : I18n.tr("Could not test a fingerprint");
        var retry = {
            "retry-scan": I18n.tr("That scan was not clear. Touch the sensor again"),
            "swipe-too-short": I18n.tr("Too short. Hold your finger on the sensor"),
            "too-fast": I18n.tr("Too fast. Slow down and touch again"),
            "finger-not-centered": I18n.tr("Centre your finger on the sensor"),
            "remove-and-retry": I18n.tr("Lift your finger, then touch again")
        };
        if (s === "disconnected")
            return I18n.tr("The sensor was disconnected");
        if (retry[s])
            return retry[s];
        if (root.mode === "enroll") {
            if (s === "stage-passed")
                return I18n.tr("Got it. Lift your finger and touch again");
            if (s === "completed")
                return I18n.tr("Fingerprint added");
        } else if (s === "match") {
            return I18n.tr("It matches");
        } else if (s === "no-match") {
            return I18n.tr("No match");
        }
        return I18n.tr("Touch the sensor");
    }

    function ended(code, text) {
        stagesRetry.stop();
        slowMark.stop();
        if (root.phase !== "running")
            return;
        if (root.mode === "enroll") {
            if (/enroll-completed/i.test(text)) {
                root.status = "enroll-completed";
                root.phase = "done";
                var label = root.name.trim();
                root.outcome = label !== "" ? I18n.tr("Added as %1. Turn fingerprints on under Where each is used to choose what it unlocks.").arg(label)
                    : I18n.tr("Added. Turn fingerprints on under Where each is used to choose what it unlocks.");
                if (root.finger !== "" && label !== "")
                    root.store.nameFinger(root.finger, label, null);
            } else {
                root.phase = "failed";
                root.outcome = /enroll-data-full/i.test(text) ? I18n.tr("The sensor's storage is full. Remove a fingerprint, then try again.")
                    : /enroll-duplicate/i.test(text) ? I18n.tr("That fingerprint is already added. Try another finger.")
                    : /enroll-disconnected/i.test(text) ? I18n.tr("The sensor was disconnected. Plug it back in, then try again.")
                    : /enroll-failed|enroll-unknown-error/i.test(text) ? I18n.tr("The sensor could not read the finger. Try again.")
                    : I18n.tr("No fingerprint was added (%1). Try again, and check the sensor if it keeps failing.").arg(String(text).trim().split("\n").slice(-1)[0] || ("exit " + code));
            }
        } else if (!/verify-no-match/i.test(text) && (code === 0 || /verify-match/i.test(text))) {
            // some fprintd versions exit 0 on no-match, so the token decides
            root.status = "verify-match";
            root.phase = "done";
            root.outcome = I18n.tr("This fingerprint unlocks your computer.");
        } else {
            root.status = /verify-no-match/i.test(text) ? "verify-no-match" : root.status;
            root.phase = "failed";
            root.outcome = /verify-no-match/i.test(text) ? I18n.tr("That finger is not one you added. Try again, or add it.")
                : I18n.tr("The test did not finish (%1). Try again.").arg(String(text).trim().split("\n").slice(-1)[0] || ("exit " + code));
        }
    }

    Timer {
        id: slowMark
        interval: 10000
        onTriggered: root.slow = true
    }

    Process {
        id: scan
        property bool launched: false
        stdout: StdioCollector { id: scanOut; waitForEnd: false; onDataChanged: root.pump("out", this.text) }
        stderr: StdioCollector { id: scanErr; waitForEnd: false; onDataChanged: root.pump("err", this.text) }
        onStarted: scan.launched = true
        onExited: code => root.ended(code, (scanOut.text || "") + "\n" + (scanErr.text || ""))
        // a program that cannot start reports no exit, only that it stopped running
        onRunningChanged: {
            if (scan.running)
                return;
            if (scan.launched || root.phase !== "running")
                return;
            stagesRetry.stop();
            slowMark.stop();
            root.phase = "failed";
            root.outcome = Changes.missingText(String(scan.command[0]));
        }
    }

    Process {
        id: stages
        command: ["busctl", "call", "net.reactivated.Fprint", "/net/reactivated/Fprint/Device/0",
                  "org.freedesktop.DBus.Properties", "Get", "ss", "net.reactivated.Fprint.Device", "num-enroll-stages"]
        stdout: StdioCollector { id: stagesOut }
        onExited: code => {
            var m = (stagesOut.text || "").match(/i\s+(\d+)/);
            if (code === 0 && m)
                root.stagesTotal = parseInt(m[1], 10);
            else
                stagesRetry.restart();
        }
    }

    Timer {
        id: stagesRetry
        interval: 1500
        property int tries: 0
        onTriggered: {
            if (!root.running || root.mode !== "enroll" || root.stagesTotal > 0 || stagesRetry.tries >= 3)
                return;
            stagesRetry.tries++;
            stages.running = true;
        }
    }

    Column {
        width: parent.width
        spacing: LithicTokens.space.s3

        Label {
            width: parent.width
            text: root.headline()
            typeRole: "section"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        Label {
            width: parent.width
            text: root.running
                ? (root.mode === "enroll"
                    ? (root.stagesTotal > 0 ? I18n.tr("%1 of %2 touches").arg(root.stagesPassed).arg(root.stagesTotal)
                                            : I18n.tr("Keep touching the sensor until it is done"))
                    : root.finger !== "" ? I18n.tr("Touch the sensor with %1").arg(root.name !== "" ? root.name : root.finger.replace(/-/g, " "))
                    : (root.store.fingers.length === 1 ? I18n.tr("Comparing against your fingerprint")
                                                       : I18n.tr("Comparing against your %1 fingerprints").arg(root.store.fingers.length)))
                : root.outcome
            typeRole: "body"
            ink: root.phase === "failed" ? Lithic.jasper : Lithic.fg2
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        BladeProgress {
            width: parent.width
            visible: root.running
            active: root.running
            indeterminate: root.mode !== "enroll" || root.stagesTotal === 0
            value: root.stagesTotal > 0 ? Math.min(1, root.stagesPassed / root.stagesTotal) : 0
        }

        Label {
            width: parent.width
            visible: root.running && root.slow
            text: I18n.tr("Still waiting for the sensor. Stop if it does not respond.")
            typeRole: "detail"
            inkRole: "fg3"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        Label {
            width: parent.width
            visible: root.log.length > 0
            text: root.log
            typeRole: "monoSm"
            inkRole: "fg3"
            wrapMode: Text.NoWrap
            elide: Text.ElideRight
            maximumLineCount: 2
        }
    }
}
