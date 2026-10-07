import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../ryoku/hub/quickshell" as Hub
import "../ryoku/hub/quickshell/schema/GlobalPage.js" as GlobalSchema

Item {
    id: root
    width: 800
    height: 500
    property var draft: ({ "general.clock_format_24_h": false })
    property string editedKey: ""
    property var editedValue: null
    Hub.SchemaPage {
        id: page
        anchors.fill: parent
        schema: GlobalSchema.rows.filter(r => r.key === "general.clock_format_24_h")
        draft: root.draft
        defaults: ({ "general.clock_format_24_h": false })
        onEdited: (key, value) => {
            root.editedKey = key;
            root.editedValue = value;
            var next = Object.assign({}, root.draft);
            next[key] = value;
            root.draft = next;
        }
    }
    function findSwitch(item) {
        if (typeof item.activate === "function" && typeof item.on === "boolean") return item;
        for (var child of item.children || []) {
            var found = findSwitch(child);
            if (found) return found;
        }
        return null;
    }
    property int phase: 0
    property int attempts: 0
    readonly property date midnight: new Date(2026, 0, 1, 0, 5, 9)
    readonly property date noon: new Date(2026, 0, 1, 12, 5, 9)
    readonly property date evening: new Date(2026, 0, 1, 23, 5, 9)

    function equal(actual, expected) {
        if (actual !== expected) throw new Error(actual + " != " + expected);
    }
    function write24(value) {
        fixture.setText(JSON.stringify({ general: { clock_format_24_h: value }, formatLocale: "en_US" }));
    }
    FileView {
        id: fixture
        path: Quickshell.env("RYOKU_TIME_TEST_FILE")
        atomicWrites: true
    }
    Timer {
        id: testTimer
        interval: 50
        repeat: true
        running: true
        onTriggered: {
            try {
                if (++root.attempts > 100) throw new Error("time-format watch timed out at phase " + root.phase);
                if (root.phase === 0) {
                    var control = root.findSwitch(page);
                    if (!control) return;
                    control.activate();
                    root.equal(root.editedKey, "general.clock_format_24_h");
                    root.equal(root.editedValue, true);
                    root.equal(control.on, true);
                    control.activate();
                    root.equal(root.editedValue, false);
                    root.write24(false);
                    root.phase = 1;
                } else if (root.phase === 1 && TimeFormat.formatLocale === "en_US" && !TimeFormat.is24h) {
                    root.equal(TimeFormat.format(root.midnight), "12:05 AM");
                    root.equal(TimeFormat.format(root.noon), "12:05 PM");
                    root.equal(TimeFormat.withHourCycle("HH.mm 'hours'"), "hh.mm 'hours' AP");
                    root.equal(TimeFormat.withHourCycle("h:mm ap"), "h:mm ap");
                    root.equal(TimeFormat.withHourCycle("'HH AP'"), "'HH AP'");
                    root.equal(TimeFormat.format(root.evening, true), "11:05:09 PM");
                    root.write24(true);
                    root.phase = 2;
                } else if (root.phase === 2 && TimeFormat.is24h) {
                    root.equal(TimeFormat.format(root.midnight), "00:05");
                    root.equal(TimeFormat.format(root.noon), "12:05");
                    root.equal(TimeFormat.withHourCycle("hh.mm ap"), "HH.mm");
                    root.equal(TimeFormat.withHourCycle("'at' h:mm AP"), "'at' H:mm");
                    root.equal(TimeFormat.format(root.evening, true), "23:05:09");
                    root.write24(false);
                    root.phase = 3;
                } else if (root.phase === 3 && !TimeFormat.is24h) {
                    root.equal(TimeFormat.format(root.evening), "11:05 PM");
                    console.log("PASS: Hub switch, midnight, noon, custom patterns and live 12/24/12 switching");
                    Qt.quit();
                }
            } catch (e) {
                console.error("FAIL:", e.message);
                testTimer.stop();
                Qt.quit();
            }
        }
    }
}
