import QtQuick

// how long a running action has taken: Working shows past 1 s, progress with Cancel past 10 s
QtObject {
    id: pace

    property bool running: false
    property bool slow: false
    property bool lagging: false

    onRunningChanged: {
        pace.slow = false;
        pace.lagging = false;
        if (pace.running) {
            slowTimer.restart();
            longTimer.restart();
        } else {
            slowTimer.stop();
            longTimer.stop();
        }
    }

    Component.onCompleted: {
        if (pace.running) {
            slowTimer.restart();
            longTimer.restart();
        }
    }

    property Timer slowTimer: Timer {
        interval: 1000
        onTriggered: pace.slow = true
    }

    property Timer longTimer: Timer {
        interval: 10000
        onTriggered: pace.lagging = true
    }
}
