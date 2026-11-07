pragma Singleton

import QtQuick
import Quickshell
import Ryoku.Ui.Singletons

// Fires the session actions the Controls bar's hold buttons complete. Lock and
// sleep are the daemon's own verbs, the same `ryoku-shell lock` the Super+L
// keybind runs on every compositor and the `ryoku-shell suspend` the lid uses:
// a fail-closed lock, then logind. Logout, reboot and shutdown are the
// `session.*` calls the daemon registers (session.go); QML never spawns
// systemctl itself, keeping the state-in-daemon / render-in-QML split. All are
// fire-and-forget: a lock or sleep answers through the compositor, and reboot
// and shutdown tear the session down, so no reply is awaited. Contract 13 sec 8.
Singleton {
    id: root

    // action is one of "lock" | "suspend" | "logout" | "reboot" | "shutdown".
    function run(action) {
        if (action === "lock" || action === "suspend") {
            Bus.run(action);
            return;
        }
        Bus.call("session." + action, {});
    }
}
