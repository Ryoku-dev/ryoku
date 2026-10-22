pragma Singleton

import QtQuick
import Caelestia.Config

QtObject {
    id: root

    property bool enabled: GlobalConfig.general.debugLogs

    function log(...args): void {
        if (root.enabled) {
            console.log(...args);
        }
    }

    // Lifecycle markers are emitted regardless of debugLogs: the installed
    // runtime smoke test asserts on them from an ordinary install, and the shell
    // already logs its startup timings the same way. They are what
    // tests/runtime-smoke-marks.txt declares, so keep the names in step.
    function mark(name: string): void {
        console.info(`[caelestia] ${name}`);
    }
}
