import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons as Clock
ShellRoot {
    id: root
    property var keyboard: ({numLock: false})
    property var config: ({})
    property int phase: 0
    FileView { id: store; path: Quickshell.env("XDG_CONFIG_HOME") + "/ryoku/shell.json"; blockLoading: true; atomicWrites: true }
    Loader {
        id: theme
        source: "qylock/themes/clockwork/orbital/Main.qml"
        onLoaded: {
            item.clock24h = Qt.binding(() => Clock.TimeFormat.is24h);
            item.curH = 23;
            item.curM = 5;
        }
    }
    function hasText(item, value) {
        if (item.text !== undefined && item.text === value && item.visible) return true;
        for (var child of item.children || []) if (hasText(child, value)) return true;
        return false;
    }
    Timer {
        interval: 250; running: true; repeat: true
        onTriggered: {
            if (!theme.item) { console.error("FAIL: theme did not load"); Qt.quit(); return; }
            if (root.phase === 0) {
                if (theme.item.clock24h || !root.hasText(theme.item, "11") || !root.hasText(theme.item, "PM")) {
                    console.error("FAIL: 12-hour theme"); Qt.quit(); return;
                }
                store.setText(JSON.stringify({general: {clock_format_24_h: true}})); root.phase++;
            } else {
                if (!theme.item.clock24h || !root.hasText(theme.item, "23") || root.hasText(theme.item, "PM")) {
                    console.error("FAIL: live 24-hour theme"); Qt.quit(); return;
                }
                console.log("PASS: real lockscreen theme renders 12-hour PM and switches live to 24-hour"); Qt.quit();
            }
        }
    }
}
