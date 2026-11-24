import QtQuick
import Quickshell
import "pages"

// Bar Studio draws the card a Store style declares in its studio-card.json, in
// place of the LAYOUT note, and its button runs the declared IPC call. The
// shell script lays out a Store index and view for "probe-style"; a second run
// makes that file malformed and expects the LAYOUT note back. A third gives
// Obi a refused Store file and expects the Hub's own Obi card behind it.
ShellRoot {
    id: probe

    readonly property bool expectCard: Quickshell.env("PROBE_EXPECT") === "card"

    QtObject {
        id: fakeHub
        property var store: ({ barStyle: Quickshell.env("PROBE_STYLE") || "probe-style" })
        property var committed: fakeHub.store
        function val(key) { return fakeHub.store[key]; }
    }

    FloatingWindow {
        implicitWidth: 1400
        implicitHeight: 900
        BarStudioPage { id: page; anchors.fill: parent; hub: fakeHub }
    }

    function find(item, test) {
        if (!item)
            return null;
        if (test(item))
            return item;
        const kids = item.children || [];
        for (let i = 0; i < kids.length; i++) {
            const hit = probe.find(kids[i], test);
            if (hit)
                return hit;
        }
        return null;
    }

    Timer {
        interval: 3000
        running: true
        onTriggered: {
            const button = probe.find(page, item => item.objectName === "studio-card-button-1-0");
            const note = probe.find(page, item => item.title === "LAYOUT" && item.visible);
            let ok = true;
            if (Quickshell.env("PROBE_EXPECT") === "obi") {
                const obi = probe.find(page, item => item.title === "OBI WIDGETS" && item.visible);
                ok = !!obi && !!probe.find(page, item => item.objectName === "obi-media");
            } else if (probe.expectCard) {
                ok = !!button && button.visible && button.text === "OPEN PROBE SETTINGS" && !note;
                if (button)
                    button.act();
            } else {
                ok = !button && !!note;
            }
            console.log(ok ? "CARD-PROBE-DONE" : "CARD-PROBE-FAIL button=" + !!button + " note=" + !!note);
            Qt.callLater(Qt.quit);
        }
    }
}
