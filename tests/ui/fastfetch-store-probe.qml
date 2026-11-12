import QtQuick
import Quickshell
import Lithic
import "hub/quickshell/pages" as Pages
import "hub/quickshell/components/terminal" as Terminal

// Settings > Terminal lists the greeting styles installed from RyoStore, and
// choosing one confirms, then runs the apply command (the stub logs it). The
// page's rows wait on the settings daemon, so the probe draws the page's own
// greeting-style row control beside it, hosted by the page
ShellRoot {
    id: root

    property var confirmed: null
    property int ticks: 0
    readonly property var styleRow: page.schema.rows.find(r => r.id === "greeting-style") || null

    function require(condition, label) {
        if (!condition) {
            console.log("FASTFETCH-STORE-PROBE-FAIL " + label);
            Qt.exit(1);
        }
        return condition;
    }

    FloatingWindow {
        id: win
        implicitWidth: 1280
        implicitHeight: 820

        Item {
            id: overlay
            anchors.fill: parent
            z: 10
        }

        // what a page asks of the Settings frame
        QtObject {
            id: fakeHub
            readonly property Item overlayItem: overlay
            readonly property string sizeClass: "wide"
            function confirm(options, yes, no) {
                root.confirmed = options;
                yes();
            }
            function navigate(page, row) {}
            function providerRowsFor(id) { return []; }
            function rowAvailable(row) { return !row.unavailable; }
            function pageAvailable(id) { return true; }
        }

        Pages.Terminal {
            id: page
            anchors.fill: parent
            hub: fakeHub
        }

        Terminal.StyleControl {
            id: control
            row: root.styleRow || ({})
        }
    }

    Timer {
        interval: 250
        repeat: true
        running: true
        onTriggered: {
            root.ticks++;
            root.require(root.ticks < 60, "the installed style never reached the greeting style picker");
            if (!root.styleRow || control.styles.length === 0)
                return;
            stop();
            var picker = control.children.find(it => it.options !== undefined && typeof it.activated === "function");
            if (!root.require(!!picker, "the greeting style picker"))
                return;
            var labels = picker.options.map(o => String(o.label !== undefined ? o.label : o));
            var at = labels.indexOf("Probe Readout");
            if (!root.require(at > 0 && labels.indexOf("Not Installed") < 0, "installed styles only in " + JSON.stringify(labels)))
                return;
            picker.activated(at);
            if (!root.require(root.confirmed && String(root.confirmed.title).indexOf("Probe Readout") >= 0, "the apply Confirm"))
                return;
            settle.start();
        }
    }

    // the stub logs the apply command; the shell script checks its log
    Timer {
        id: settle
        interval: 1500
        onTriggered: {
            console.log("FASTFETCH-STORE-PROBE-PASS installed style picker applies");
            Qt.quit();
        }
    }
}
