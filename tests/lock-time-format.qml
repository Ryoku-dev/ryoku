import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons as Clock
import inir.modules.common as Inir
ShellRoot {
    id: root
    property var keyboard: ({numLock: false})
    property var sddm: ({fingerprintHint: false})
    property var config: ({})
    property int phase: -1
    FileView { id: store; path: Quickshell.env("XDG_CONFIG_HOME") + "/ryoku/shell.json"; blockLoading: true; atomicWrites: true }
    Loader {
        id: theme
        source: "qylock/themes/clockwork/orbital/Main.qml"
        onLoaded: {
            item.clock24h = Qt.binding(() => Clock.TimeFormat.is24h);
            item.clockLocale = Qt.binding(() => Clock.TimeFormat.locale);
            item.curH = 23;
            item.curM = 5;
            item.curDate = new Date(2026, 11, 6);
            if (!Clock.TimeFormat.is24HourPattern("HH:mm 'at'")
                    || Clock.TimeFormat.is24HourPattern("h:mm AP")) {
                console.error("FAIL: clock pattern classification"); Qt.quit(); return;
            }
            Inir.Config.setNestedValue("time.format", "h:mm AP");
            if (Inir.Config.pending["general.clock_format_24_h"] !== false
                    || Inir.Config.pending["inir.time.format"] !== undefined) {
                console.error("FAIL: Shima 12-hour setting did not route exclusively to the global clock"); Qt.quit(); return;
            }
            root.writeFormat("en_US", false);
            root.phase = 0;
        }
    }
    function hasText(item, value) {
        if (item.text !== undefined && item.text === value && item.visible) return true;
        for (var child of item.children || []) if (hasText(child, value)) return true;
        return false;
    }
    function writeFormat(locale, is24h) {
        store.setText(JSON.stringify({
            general: {clock_format_24_h: is24h},
            formatLocale: locale
        }));
    }
    Timer {
        interval: 250; running: true; repeat: true
        onTriggered: {
            if (!theme.item) { console.error("FAIL: theme did not load"); Qt.quit(); return; }
            if (root.phase === 0 && Clock.TimeFormat.formatLocale === "en_US" && !theme.item.clock24h) {
                if (!root.hasText(theme.item, "11") || !root.hasText(theme.item, "PM")
                        || !root.hasText(theme.item, "06 DEC 2026") || !root.hasText(theme.item, "SUNDAY")) {
                    console.error("FAIL: English 12-hour theme"); Qt.quit(); return;
                }
                root.writeFormat("es_ES", false); root.phase++;
            } else if (root.phase === 1 && Clock.TimeFormat.formatLocale === "es_ES" && !theme.item.clock24h) {
                if (!root.hasText(theme.item, "11") || !root.hasText(theme.item, Clock.TimeFormat.locale.pmText)
                        || !root.hasText(theme.item, "06 DIC 2026") || !root.hasText(theme.item, "DOMINGO")) {
                    console.error("FAIL: Spanish 12-hour theme"); Qt.quit(); return;
                }
                Inir.Config.setNestedValue("time.format", "HH:mm");
                if (Inir.Config.pending["general.clock_format_24_h"] !== true
                        || Inir.Config.pending["inir.time.format"] !== undefined) {
                    console.error("FAIL: Shima 24-hour setting did not route exclusively to the global clock"); Qt.quit(); return;
                }
                root.writeFormat("en_US", true); root.phase++;
            } else if (root.phase === 2 && Clock.TimeFormat.formatLocale === "en_US" && theme.item.clock24h) {
                if (!root.hasText(theme.item, "23") || root.hasText(theme.item, "PM")
                        || !root.hasText(theme.item, "06 DEC 2026") || !root.hasText(theme.item, "SUNDAY")) {
                    console.error("FAIL: English 24-hour theme"); Qt.quit(); return;
                }
                root.writeFormat("es_ES", true); root.phase++;
            } else if (root.phase === 3 && Clock.TimeFormat.formatLocale === "es_ES" && theme.item.clock24h) {
                if (!root.hasText(theme.item, "23") || root.hasText(theme.item, Clock.TimeFormat.locale.pmText)
                        || !root.hasText(theme.item, "06 DIC 2026") || !root.hasText(theme.item, "DOMINGO")) {
                    console.error("FAIL: Spanish 24-hour theme"); Qt.quit(); return;
                }
                console.log("PASS: Shima and the stock lockscreen share English and Spanish 12/24-hour time"); Qt.quit();
            }
        }
    }
}
