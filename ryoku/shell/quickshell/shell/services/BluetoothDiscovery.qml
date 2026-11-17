pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Bluetooth
import "../utils/menupoll.js" as MenuPoll

Singleton {
    id: root

    property var owners: []
    property bool owesStop: false
    readonly property var radio: Bluetooth.defaultAdapter

    function setDiscovering(owner, adapter, enabled) {
        owners = MenuPoll.setOwnership(owners, owner, enabled);
        const target = adapter || root.radio;
        if (!target)
            return;
        const want = owners.length > 0 && target.enabled;
        target.discovering = want;
        if (want) {
            root.owesStop = true;
            claimExpiry.restart();
        }
    }

    function stopAll(adapter) {
        owners = [];
        const target = adapter || root.radio;
        if (target)
            target.discovering = false;
    }

    // Quickshell drops a discovering write that equals the last state BlueZ
    // reported, so a stop sent while a start is still unconfirmed vanishes and
    // the scan then runs until the shell restarts. This asks again once the
    // start lands, for as long as nobody wants the scan. The claim only covers
    // scans started here, and only for a short while after the request.
    Timer {
        id: claimExpiry
        interval: 15000
        onTriggered: root.owesStop = false
    }

    Timer {
        id: orphanStop
        property int attempts: 0
        interval: 1000
        repeat: true
        running: root.owners.length === 0 && root.owesStop
            && root.radio !== null && root.radio.discovering === true
        onRunningChanged: if (running) attempts = 0
        onTriggered: {
            attempts += 1;
            if (attempts > 3) {
                root.owesStop = false;
                return;
            }
            root.radio.discovering = false;
        }
    }

    Connections {
        target: root.radio
        function onDiscoveringChanged() {
            if (!root.radio.discovering && root.owners.length === 0)
                root.owesStop = false;
        }
    }
}
