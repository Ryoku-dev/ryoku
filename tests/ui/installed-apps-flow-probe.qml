import QtQuick
import Quickshell
import Ryoku.Ui as RyokuUi
import "components/installed-apps"

// Installed apps' removal line (Ryoku.Ui's PackageJob under Settings' JobLine), driven by a stand-in pkexec and ryoku-host that print the
// real progress events: a removal that goes well shows Working, then the counted blade, and
// ends done with nothing left on the line; one that fails ends on its error with Retry, and
// Retry runs it again; a closed password prompt says so.
ShellRoot {
    id: probe

    property int failures: 0
    property int stage: 0
    property bool sawWorking: false
    property bool sawBlade: false
    property bool sawCounts: false
    property real topFraction: -1
    property bool sawRetryRun: false

    function fail(message) {
        probe.failures++;
        console.log("INSTALLED-APPS-FLOW " + message);
    }

    function expect(ok, message) {
        if (!ok)
            probe.fail(message);
    }

    function end() {
        console.log(probe.failures === 0 ? "INSTALLED-APPS-FLOW-PASS" : "INSTALLED-APPS-FLOW-FAIL " + probe.failures);
        Qt.quit();
    }

    function retryPlaque() {
        for (var i = 0; i < line.children.length; i++) {
            var row = line.children[i];
            for (var j = 0; j < row.children.length; j++)
                if (row.children[j].objectName === "jobRetry")
                    return row.children[j];
        }
        return null;
    }

    FloatingWindow {
        implicitWidth: 640
        implicitHeight: 200
        Item {
            width: 600
            height: 200
            JobLine {
                id: line
                job: job
                target: "htop"
                doing: "Removing Htop"
            }
        }
    }

    RyokuUi.PackageJob {
        id: job
        onFinished: (target, action, ok) => Qt.callLater(probe.check, ok)
    }

    // what the line shows while the job runs, sampled a few times a second
    Timer {
        interval: 60
        repeat: true
        running: job.running
        onTriggered: {
            if (line.showWorking)
                probe.sawWorking = true;
            if (line.showBlade)
                probe.sawBlade = true;
            if (line.counts !== "")
                probe.sawCounts = true;
            probe.topFraction = Math.max(probe.topFraction, job.fraction);
            if (probe.stage === 2 && job.error === "")
                probe.sawRetryRun = true;
        }
    }

    function check(ok) {
        switch (probe.stage) {
        case 0:
            probe.expect(ok, "a removal that went well reported a failure: " + job.error);
            probe.expect(probe.sawWorking, "Working never showed past one second");
            probe.expect(probe.sawBlade, "the blade never showed once the tool counted");
            probe.expect(probe.sawCounts, "the tool's counts never reached the line");
            probe.expect(probe.topFraction > 0.3, "the blade never moved: " + probe.topFraction);
            probe.expect(job.fraction === 1, "the job did not end done: " + job.fraction);
            probe.expect(job.state.finished && job.state.ok, "the done event was not read");
            probe.expect(!line.showWorking && !line.showError, "the line still shows after a removal that went well");
            probe.expect(job.cancellable, "a removal as root offers no Cancel, though ryoku-host stops when its stdin closes");
            probe.stage = 1;
            job.start("htop", "remove", "Htop", ["pkexec", "ryoku-host", "apps", "--stdin-cancel", "remove", "--manager", "pacman", "missing"]);
            break;
        case 1:
            probe.expect(!ok, "a failed removal reported success");
            probe.expect(job.error.indexOf("target not found: missing") >= 0, "the error does not carry the tool's words: " + job.error);
            probe.expect(line.showError, "the line does not show the error");
            // the line fades in; Retry is pressed once it stands
            settle.restart();
            break;
        case 2:
            probe.expect(probe.sawRetryRun, "the error stayed on the line while Retry ran");
            probe.expect(!ok && line.showError, "the retried removal did not end on its error again");
            probe.stage = 3;
            job.start("htop", "remove", "Htop", ["pkexec", "--closed", "ryoku-host", "apps", "--stdin-cancel", "remove", "--manager", "pacman", "htop"]);
            break;
        case 3:
            probe.expect(!ok, "a closed password prompt reported success");
            probe.expect(job.error === "The password prompt was closed, so nothing changed.", "a closed prompt said: " + job.error);
            probe.stage = 4;
            job.start("org.example.App", "remove", "Example", ["gpk", "remove", "-m", "flatpak", "--yes", "--events", "org.example.App"]);
            probe.expect(job.cancellable, "a removal as you offers no Cancel");
            stopSoon.restart();
            break;
        case 4:
            probe.expect(!ok && job.cancelled, "a cancelled removal did not end cancelled");
            probe.expect(job.state.finished && !job.state.ok, "gpk's last word after the stop was not read");
            probe.expect(job.error.indexOf("Stopped") === 0, "a cancelled removal said: " + job.error);
            probe.stage = 5;
            job.start("htop", "remove", "Htop", ["pkexec", "ryoku-host", "apps", "--stdin-cancel", "remove", "--manager", "pacman", "slow"]);
            stopSoon.restart();
            break;
        case 5:
            probe.expect(!ok && job.cancelled, "a removal as root, cancelled, did not end cancelled");
            probe.expect(job.state.finished && job.state.error === "cancelled", "ryoku-host never heard the stop: " + JSON.stringify(job.state));
            probe.expect(line.showError && probe.retryPlaque() !== null && !probe.retryPlaque().visible, "a cancelled removal offers Retry");
            probe.end();
            break;
        }
    }

    Timer {
        id: stopSoon
        interval: 600
        onTriggered: job.cancel()
    }

    Timer {
        id: settle
        interval: 400
        onTriggered: {
            var retry = probe.retryPlaque();
            probe.expect(line.visible && line.opacity === 1, "the error line did not fade in: " + line.opacity);
            probe.expect(!!retry && retry.visible, "the error offers no Retry");
            probe.stage = 2;
            if (retry)
                retry.clicked();
            else
                job.retry();
            probe.expect(job.running, "Retry did not run the removal again");
        }
    }

    Timer {
        interval: 20000
        running: true
        onTriggered: {
            probe.fail("timed out at stage " + probe.stage);
            probe.end();
        }
    }

    Component.onCompleted: job.start("htop", "remove", "Htop", ["pkexec", "ryoku-host", "apps", "--stdin-cancel", "remove", "--manager", "pacman", "htop"])
}
