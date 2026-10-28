import QtQuick
import Quickshell
ShellRoot {
    id: root
    property var keyboard: ({numLock: false})
    property var config: ({})
    property var sddm: ({hostName: "sandbox", fingerprintHint: true})
    property int phase: 0
    readonly property date yesterday: new Date(Date.now() - 86400000)
    property real secStart: 0
    property real secTarget: 0
    property real minStart: 0
    property real minTarget: 0
    Loader {
        id: theme
        source: "qylock/themes/clockwork/orbital/Main.qml"
    }
    function hasText(item, value) {
        if (item.text !== undefined && item.text === value && item.visible) return true;
        for (var child of item.children || []) if (hasText(child, value)) return true;
        return false;
    }
    function findNamed(item, name) {
        if (item.objectName === name) return item;
        for (var child of item.children || []) {
            var match = findNamed(child, name);
            if (match) return match;
        }
        return null;
    }
    function day(d) { return Qt.formatDate(d, "dddd").toUpperCase(); }
    Timer {
        interval: 250; running: true; repeat: true
        onTriggered: {
            if (!theme.item) { console.error("FAIL: theme did not load"); Qt.quit(); return; }
            if (root.phase === 0) {
                if (theme.item.curDate === undefined) {
                    console.error("FAIL: theme has no reactive curDate property"); Qt.quit(); return;
                }
                if (theme.item.softwareCursorEnabled !== undefined) {
                    console.error("FAIL: theme still owns the greeter host cursor"); Qt.quit(); return;
                }
                var pill = root.findNamed(theme.item, "indicatorPill");
                var minuteRing = root.findNamed(theme.item, "minuteRing");
                var secondRing = root.findNamed(theme.item, "secondRing");
                if (!pill || !minuteRing || !secondRing
                        || minuteRing.parent !== pill.parent || secondRing.parent !== pill.parent
                        || minuteRing.z <= pill.z || secondRing.z <= pill.z) {
                    console.error("FAIL: clock rings must pass in front of the password pill"); Qt.quit(); return;
                }
                root.secStart = theme.item.smoothSecAngle;
                root.secTarget = root.secStart - 30;
                root.minStart = theme.item.smoothMinAngle;
                root.minTarget = root.minStart - 15;
                theme.item.secRingTarget = root.secTarget;
                theme.item.minRingTarget = root.minTarget;
                theme.item.curDate = root.yesterday;
                if (!root.hasText(theme.item, root.day(root.yesterday))) {
                    console.error("FAIL: date label does not follow curDate"); Qt.quit(); return;
                }
                root.phase++;
            } else if (root.phase === 1) {
                if (!root.hasText(theme.item, root.day(root.yesterday))) {
                    console.error("FAIL: date changed before the next clock sample"); Qt.quit(); return;
                }
                if (!(theme.item.smoothSecAngle < root.secStart - 0.5
                        && theme.item.smoothSecAngle > root.secTarget + 0.5)
                        || !(theme.item.smoothMinAngle < root.minStart - 0.5
                        && theme.item.smoothMinAngle > root.minTarget + 0.5)) {
                    console.error("FAIL: pointer highlight does not follow the rendered rings"); Qt.quit(); return;
                }
                root.phase++;
            } else if (root.phase < 4) {
                root.phase++;
            } else {
                if (!root.hasText(theme.item, root.day(new Date())) || root.hasText(theme.item, root.day(root.yesterday))) {
                    console.error("FAIL: date label stayed on " + root.day(root.yesterday)); Qt.quit(); return;
                }
                console.log("PASS: lockscreen pointer follows its rings and the date rolls over"); Qt.quit();
            }
        }
    }
}
