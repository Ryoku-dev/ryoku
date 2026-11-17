import QtQuick
import Quickshell
import Quickshell.Bluetooth
import "services"

ShellRoot {
    id: root
    property int phase: 0
    property int ticks: 0
    readonly property var radio: Bluetooth.defaultAdapter
    property QtObject owner: QtObject {}
    property QtObject second: QtObject {}

    function fail(msg) { console.error("FAIL: " + msg); Qt.quit(); }

    // start a scan and stop it again before the radio confirmed the start
    // (phase 0), then the same with a scan that is genuinely wanted (phase 3)
    Timer {
        interval: 100; running: true; repeat: true
        onTriggered: {
            root.ticks += 1;
            if (root.phase === 0) {
                BluetoothDiscovery.setDiscovering(root.owner, root.radio, true);
                BluetoothDiscovery.setDiscovering(root.owner, root.radio, false);
                root.phase = 1;
                root.ticks = 0;
            } else if (root.phase === 1) {
                // the late confirmation lands, leaving a scan nobody owns
                if (root.radio.discovering) { root.phase = 2; root.ticks = 0; }
                else if (root.ticks > 20) root.fail("the start was never confirmed (harness)");
            } else if (root.phase === 2) {
                if (!root.radio.discovering) {
                    if (BluetoothDiscovery.owesStop) root.fail("the stop landed but the claim stayed");
                    root.phase = 3; root.ticks = 0;
                } else if (root.ticks > 40) {
                    root.fail("an unowned scan was still running 4s after nobody wanted it");
                }
            } else if (root.phase === 3) {
                BluetoothDiscovery.setDiscovering(root.owner, root.radio, true);
                root.phase = 4; root.ticks = 0;
            } else if (root.phase === 4) {
                // a scan someone wants must be left alone, well past the retry interval
                if (root.ticks > 45) {
                    if (!root.radio.discovering) root.fail("the net stopped a scan that was still wanted");
                    root.phase = 5; root.ticks = 0;
                }
            } else if (root.phase === 5) {
                // two owners: the first leaving must not end the scan for the second
                BluetoothDiscovery.setDiscovering(root.second, root.radio, true);
                BluetoothDiscovery.setDiscovering(root.owner, root.radio, false);
                root.phase = 6; root.ticks = 0;
            } else if (root.phase === 6) {
                if (root.ticks > 30) {
                    if (!root.radio.discovering) root.fail("one owner leaving ended the other's scan");
                    BluetoothDiscovery.setDiscovering(root.second, root.radio, false);
                    root.phase = 7; root.ticks = 0;
                }
            } else if (root.phase === 7) {
                if (!root.radio.discovering && root.ticks > 5) {
                    console.log("PASS: a scan stopped before its start landed is still stopped, wanted scans are left alone");
                    Qt.quit();
                } else if (root.ticks > 40) {
                    root.fail("the last owner leaving did not end the scan");
                }
            }
        }
    }
}
