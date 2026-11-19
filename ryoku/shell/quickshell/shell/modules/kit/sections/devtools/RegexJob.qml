pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io
import "devtools.js" as DevTools

QtObject {
    id: root

    required property string runnerPath
    property var process: null
    readonly property bool running: process !== null
    signal finished(int ticket, var result)

    function cancel() {
        deadline.stop();
        const child = process;
        process = null;
        if (child)
            child.signal(9);
    }
    function run(ticket, request) {
        cancel();
        const child = childComponent.createObject(root, { ticket: ticket, request: request });
        process = child;
        child.running = true;
        deadline.start();
    }
    function receive(child, line) {
        const marker = "RYOKU_KIT_REGEX_RESULT:";
        const index = line.indexOf(marker);
        if (child !== process || child.result !== null || index < 0)
            return;
        try {
            const result = JSON.parse(line.slice(index + marker.length));
            if (typeof result.output === "string" && (!result.error || typeof result.error === "string"))
                child.result = result;
        } catch (error) {}
    }
    function exited(child, exitCode) {
        if (child === process) {
            deadline.stop();
            process = null;
            finished(child.ticket, child.failure.length ? { output: "", error: child.failure }
                : exitCode === 0 && child.result !== null ? child.result
                : { output: "", error: "Regular expression process failed" });
        }
        child.destroy();
    }
    Component.onDestruction: cancel()

    property Timer deadline: Timer {
        interval: DevTools.workLimits().regexMilliseconds
        onTriggered: {
            if (!root.process)
                return;
            root.process.failure = "Regular expression exceeded the 1500 ms execution limit";
            if (root.process.running)
                root.process.signal(9);
            else
                root.exited(root.process, -1);
        }
    }
    property Component childComponent: Component {
        Process {
            id: child
            required property int ticket
            required property string request
            property var result: null
            property string failure: ""
            command: ["qs", "--no-color", "-p", root.runnerPath]
            environment: ({
                "QT_QPA_PLATFORM": "offscreen",
                "QML_DISABLE_DISK_CACHE": "1",
                "QS_NO_RELOAD_POPUP": "1",
                "RYOKU_KIT_REGEX_REQUEST": request,
                "__QUICKSHELL_CRASH_INFO_FD": null,
                "__QUICKSHELL_CRASH_DUMP_PID": null,
                "__QUICKSHELL_CRASH_DUMP_FD": null,
                "__QUICKSHELL_CRASH_LOG_FD": null,
                "__QUICKSHELL_CRASH_SIGNAL": null
            })
            stdout: SplitParser { onRead: line => root.receive(child, line) }
            stderr: SplitParser { onRead: line => root.receive(child, line) }
            onExited: (exitCode, exitStatus) => root.exited(child, exitCode)
        }
    }
}
