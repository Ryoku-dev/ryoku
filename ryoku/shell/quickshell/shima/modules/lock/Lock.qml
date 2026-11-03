pragma ComponentBehavior: Bound
import QtQuick
import Quickshell
import Quickshell.Io
import qs

// Ryoku owns the secure lock and authentication lifecycle. This adapter keeps
// upstream lock entry points and screenLocked consumers on that same owner.
Scope {
    function activate(): void { Quickshell.execDetached(["ryoku-shell", "lock"]) }
    Process {
        id: status
        command: ["ryoku-shell", "lock", "status"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                const locked = text.trim() === "locked"
                GlobalStates.screenLocked = locked
            }
        }
    }
    Timer { interval: 1000; repeat: true; running: true; onTriggered: if (!status.running) status.running = true }
    IpcHandler {
        target: "lock"
        function activate(): void { Quickshell.execDetached(["ryoku-shell", "lock"]) }
        function prepareSleep(): string { Quickshell.execDetached(["ryoku-shell", "lock"]); return "locking" }
        function status(): string { return GlobalStates.screenLocked ? "locked" : "unlocked" }
        // Unlock requires successful authentication through Ryoku's proof owner.
        function deactivate(): void {}
        function focus(): void {}
    }
}
