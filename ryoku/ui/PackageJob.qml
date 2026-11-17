import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "lib/jobevents.js" as JobEvents

// One package install, update or removal at a time, read from the progress events gpk and
// `ryoku-host apps` print (lib/jobevents.js): quiet for the first second (`working` after it),
// `lengthy` past ten seconds, counted when the tool counts, and an honest end. A job run as
// the user (gpk) stops on SIGTERM; one run as root through pkexec stops only when it listens
// on its stdin (`ryoku-host apps --stdin-cancel`), since a signal from here cannot reach it.
// Settings and RyoStore each hold one; their own lines draw it.
Scope {
    id: job

    // what the job is about ("pacman:firefox", "ryoku:ryotunes", ...), its verb and its name
    property string target: ""
    property string action: ""
    property string label: ""
    // set here, not bound to the process: its running flag changes only after exited fires
    property bool running: false
    property bool working: false
    property bool lengthy: false
    property bool cancelled: false
    property var state: JobEvents.initial()
    readonly property real fraction: JobEvents.fraction(job.state)
    readonly property int done: job.state.done
    readonly property int total: job.state.total
    // real counts arrived from a progress event
    readonly property bool counting: job.state.total > 0
    readonly property int stepCount: job.state.plan.length
    readonly property int stepIndex: job.state.index
    readonly property string stepName: JobEvents.stepName(job.state)
    readonly property string lastLog: job.state.lastLog
    readonly property bool cancellable: JobEvents.cancellable(job._argv)
    // the last job's failure, kept until the next one starts or it is dismissed
    property string errorTarget: ""
    property string errorAction: ""
    property string error: ""
    property bool errorCancelled: false
    property var _argv: []
    property var _retry: null
    property string _stderr: ""

    signal finished(string target, string action, bool ok)

    function busyFor(key) {
        return job.running && job.target === key;
    }

    function errorFor(key) {
        return !job.running && job.errorTarget === key ? job.error : "";
    }

    function start(key, verb, name, argv) {
        if (job.running || !argv || argv.length === 0)
            return false;
        job.target = String(key);
        job.action = String(verb);
        job.label = String(name || "");
        job.cancelled = false;
        job.working = false;
        job.lengthy = false;
        job.state = JobEvents.initial();
        job.errorTarget = "";
        job.errorAction = "";
        job.error = "";
        job.errorCancelled = false;
        job._stderr = "";
        job._argv = argv.slice();
        job._retry = { key: job.target, verb: job.action, name: job.label, argv: job._argv };
        // the root helper reads "cancel" on stdin, and its end of file stops it too, so its
        // stdin stays open for the whole run
        proc.stdinEnabled = JobEvents.cancelsByStdin(job._argv);
        proc.command = job._argv;
        job.running = true;
        proc.running = true;
        workingTimer.restart();
        lengthyTimer.restart();
        return true;
    }

    // a step that has already started may still finish
    function cancel() {
        if (!job.running || job.cancelled || !job.cancellable)
            return;
        job.cancelled = true;
        if (proc.stdinEnabled) {
            proc.write("cancel\n");
            proc.stdinEnabled = false;
        } else {
            proc.running = false;
        }
    }

    function retry() {
        var again = job._retry;
        if (!again || job.running)
            return;
        job.start(again.key, again.verb, again.name, again.argv);
    }

    function dismiss() {
        if (job.running)
            return;
        job.errorTarget = "";
        job.error = "";
        job.errorCancelled = false;
    }

    function _wording(end) {
        if (end.kind === "cancelled")
            return I18n.tr("Stopped. A step that had already started may still finish.");
        if (end.kind === "dismissed")
            return I18n.tr("The password prompt was closed, so nothing changed.");
        if (end.kind === "denied")
            return I18n.tr("Authentication failed, so nothing changed. Try again with an administrator's password.");
        var plain = job.action === "remove" ? I18n.tr("%1 could not be removed.").arg(job.label)
            : job.action === "update" ? I18n.tr("%1 could not be updated.").arg(job.label)
            : I18n.tr("%1 could not be installed.").arg(job.label);
        if (end.kind === "message" && end.text !== "")
            return plain + " " + I18n.tr("It said: %1").arg(end.text) + (/[.!?]$/.test(end.text) ? "" : ".");
        return plain;
    }

    function _end(ok, message) {
        workingTimer.stop();
        lengthyTimer.stop();
        job.working = false;
        job.lengthy = false;
        job.running = false;
        proc.stdinEnabled = false;
        if (!ok) {
            job.errorTarget = job.target;
            job.errorAction = job.action;
            job.error = message;
            job.errorCancelled = job.cancelled;
        }
        job.finished(job.target, job.action, ok);
    }

    Timer {
        id: workingTimer
        interval: 1000
        onTriggered: job.working = job.running
    }

    Timer {
        id: lengthyTimer
        interval: 10000
        onTriggered: job.lengthy = job.running
    }

    Process {
        id: proc
        environment: Spawn.env
        stdout: SplitParser {
            splitMarker: "\n"
            onRead: data => {
                var ev = JobEvents.parse(data);
                if (ev)
                    job.state = JobEvents.reduce(job.state, ev);
            }
        }
        stderr: SplitParser {
            splitMarker: "\n"
            onRead: data => job._stderr = JobEvents.tail(job._stderr, data + "\n", 4096)
        }
        // a command that could not start never reports an exit
        onRunningChanged: {
            if (!proc.running && job.running && !job.cancelled && proc.processId === null)
                Qt.callLater(() => {
                    if (job.running && !proc.running)
                        job._end(false, I18n.tr("The tool this needs could not be started. Update Ryoku, then try again."));
                });
        }
        onExited: (code, status) => {
            if (!job.running)
                return;
            var ok = code === 0 && !job.cancelled && (!job.state.finished || job.state.ok);
            job._end(ok, ok ? "" : job._wording(JobEvents.failure(job._argv, code, job.state, job._stderr, job.cancelled)));
        }
    }
}
