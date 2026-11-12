import QtQuick

// how long a piece of work has run: working past 1 s, slow past 10 s
Item {
    id: clock

    property bool running: false
    property bool working: false
    property bool slow: false

    visible: false
    width: 0
    height: 0

    onRunningChanged: {
        clock.working = false;
        clock.slow = false;
        if (clock.running) {
            first.restart();
            second.restart();
        } else {
            first.stop();
            second.stop();
        }
    }

    Timer {
        id: first
        interval: 1000
        onTriggered: clock.working = true
    }

    Timer {
        id: second
        interval: 10000
        onTriggered: clock.slow = true
    }
}
