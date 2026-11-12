import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

// one account command at a time: an optional secret on stdin, json lines out,
// slow past 1 s and long past 10 s, and an honest end even when cancelled
Scope {
    id: job

    // set here, not bound to the process: its running flag changes only after exited fires
    property bool running: false
    property bool slow: false
    property bool lengthy: false
    property bool cancelled: false
    property string out: ""
    property string err: ""
    property var _command: []
    property string _secret: ""
    property bool _hasSecret: false

    signal line(var value)
    signal finished(bool ok, string message)

    function start(command, secret) {
        if (proc.running || job.running)
            return false;
        job._command = command;
        job._hasSecret = secret !== undefined && secret !== null;
        job._secret = job._hasSecret ? String(secret) : "";
        job.out = "";
        job.err = "";
        job.slow = false;
        job.lengthy = false;
        job.cancelled = false;
        proc.stdinEnabled = job._hasSecret;
        proc.command = command;
        job.running = true;
        proc.running = true;
        slowTimer.restart();
        longTimer.restart();
        return true;
    }

    // stops waiting; a step that already runs as root may still finish
    function cancel() {
        if (!job.running)
            return;
        job.cancelled = true;
        proc.running = false;
    }

    // the backend's own words, then stderr, then the caller's
    function message(code, fallback) {
        var lines = job.out.trim().split("\n");
        for (var i = lines.length - 1; i >= 0; i--) {
            try {
                var v = JSON.parse(lines[i]);
                if (v && v.error)
                    return String(v.error);
            } catch (e) {
            }
        }
        if (job._command[0] === "pkexec" && code === 126)
            return I18n.tr("The password prompt was closed, so nothing changed.");
        if (job._command[0] === "pkexec" && code === 127)
            return I18n.tr("Authentication failed, so nothing changed. Try again with an administrator's password.");
        var text = job.err.trim();
        return text !== "" ? text.split("\n").pop() : fallback;
    }

    Timer {
        id: slowTimer
        interval: 1000
        onTriggered: job.slow = proc.running
    }

    Timer {
        id: longTimer
        interval: 10000
        onTriggered: job.lengthy = proc.running
    }

    Process {
        id: proc
        environment: Spawn.env
        stdinEnabled: false
        stdout: SplitParser {
            splitMarker: "\n"
            onRead: data => {
                job.out += data + "\n";
                var text = String(data).trim();
                if (text === "")
                    return;
                try {
                    job.line(JSON.parse(text));
                } catch (e) {
                }
            }
        }
        stderr: SplitParser {
            splitMarker: "\n"
            onRead: data => job.err += data + "\n"
        }
        onStarted: {
            if (job._hasSecret) {
                write(job._secret + "\n");
                job._secret = "";
            }
        }
        // a command that could not start never reports an exit
        onRunningChanged: {
            if (!proc.running && job.running && !job.cancelled && proc.processId === null)
                Qt.callLater(() => {
                    if (job.running && !proc.running) {
                        job.running = false;
                        slowTimer.stop();
                        longTimer.stop();
                        job.slow = false;
                        job.lengthy = false;
                        job.finished(false, I18n.tr("The tool this needs could not be started. Update Ryoku, then try again."));
                    }
                });
        }
        onExited: (code, status) => {
            job.running = false;
            slowTimer.stop();
            longTimer.stop();
            job.slow = false;
            job.lengthy = false;
            job._secret = "";
            proc.stdinEnabled = false;
            if (job.cancelled) {
                job.finished(false, I18n.tr("Stopped. A step that had already started may still finish."));
                return;
            }
            job.finished(code === 0, code === 0 ? "" : job.message(code, I18n.tr("The change could not be completed.")));
        }
    }
}
