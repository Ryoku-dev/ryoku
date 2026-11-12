import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import "../../Singletons"

// one install, update or removal: steps in order, slow marks, cancel, retry from the failed step
Scope {
    id: task

    property string target: ""
    property string action: ""
    // [{ argv, failText }] where failText takes the exit code as %1
    property var steps: []
    property int step: 0
    // false when stopping it would leave things half done (a terminal owns it)
    property bool cancellable: true
    property string error: ""
    property bool canceled: false
    property bool running: false
    // past 1 s and past 10 s
    property bool working: false
    property bool longRunning: false
    property var proc: null

    signal finished(string target, string action, bool ok)

    function start(target, action, steps, cancellable) {
        if (task.running)
            return false;
        task.target = target;
        task.action = action;
        task.steps = steps;
        task.cancellable = cancellable !== false;
        task.step = 0;
        task.error = "";
        task.runStep();
        return true;
    }

    function runStep() {
        task.canceled = false;
        task.working = false;
        task.longRunning = false;
        task.running = true;
        slowTimer.restart();
        longTimer.restart();
        var argv = task.steps[task.step].argv;
        // Changes.run calls back exactly once, also when the program cannot start
        task.proc = Changes.run(argv, (ok, out, err, code) => task.stepDone(argv, ok, err, code), { env: Spawn.env });
    }

    function stepDone(argv, ok, err, code) {
        task.proc = null;
        task.running = false;
        slowTimer.stop();
        longTimer.stop();
        task.working = false;
        task.longRunning = false;
        if (task.canceled) {
            task.error = I18n.tr("Stopped before it finished. This page shows what is on this computer now.");
            task.finished(task.target, task.action, false);
            return;
        }
        if (!ok) {
            if (code < 0) {
                task.error = String(err);
            } else {
                var said = task.lastLines(err);
                var plain = String(task.steps[task.step].failText || I18n.tr("It did not finish (exit %1)."));
                // the exit code is only the last clue; what the tool said replaces it
                task.error = said.length > 0
                    ? plain.replace(/ \(exit %1\)/, "").replace("%1", code) + " " + I18n.tr("It said: %1").arg(said) + (/[.!?]$/.test(said) ? "" : ".")
                    : plain.replace("%1", code);
            }
            task.finished(task.target, task.action, false);
            return;
        }
        task.step++;
        if (task.step < task.steps.length) {
            task.runStep();
            return;
        }
        task.error = "";
        task.finished(task.target, task.action, true);
    }

    function retry() {
        if (task.running || task.error === "" || task.step >= task.steps.length)
            return;
        task.error = "";
        task.runStep();
    }

    function cancel() {
        if (!task.running || !task.cancellable || !task.proc)
            return;
        task.canceled = true;
        task.proc.running = false;
    }

    function lastLines(text) {
        var lines = String(text || "").trim().split("\n").filter(l => l.trim().length > 0);
        return lines.slice(Math.max(0, lines.length - 2)).join(" ");
    }

    Timer {
        id: slowTimer
        interval: 1000
        onTriggered: task.working = task.running
    }

    Timer {
        id: longTimer
        interval: 10000
        onTriggered: task.longRunning = task.running
    }
}
