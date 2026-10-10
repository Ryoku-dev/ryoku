import QtQuick
import Quickshell

// Every section in the Hub's rail must open a page that loads. Resolves each
// one through Hub.pageFile, the same call the page host makes, and compiles it
// through Quickshell's qs: loader, so a page that cannot name its siblings or a
// section that lost its page fails here instead of opening blank.
ShellRoot {
    id: probe

    property var hub: null
    property var queue: []
    property var pending: null
    property int failures: 0

    function fail(message) {
        probe.failures++;
        console.log("HUB-PAGES-PROBE " + message);
    }

    function next() {
        if (probe.queue.length === 0) {
            console.log(probe.failures === 0 ? "HUB-PAGES-PROBE-PASS" : "HUB-PAGES-PROBE-FAIL " + probe.failures);
            Qt.quit();
            return;
        }
        const key = probe.queue.shift();
        const url = probe.hub.pageFile(key);
        if (url === "") {
            probe.fail("section " + key + " has no page");
            Qt.callLater(probe.next);
            return;
        }
        const component = Qt.createComponent(url, Component.Asynchronous);
        probe.pending = component;
        const settle = function () {
            if (component.status === Component.Loading)
                return;
            if (component.status !== Component.Ready)
                probe.fail("section " + key + " does not load: " + component.errorString());
            Qt.callLater(probe.next);
        };
        if (component.status === Component.Loading)
            component.statusChanged.connect(settle);
        else
            settle();
    }

    Component.onCompleted: {
        const component = Qt.createComponent(Qt.resolvedUrl("Hub.qml"));
        if (component.status !== Component.Ready) {
            probe.fail("Hub does not load: " + component.errorString());
            Qt.quit();
            return;
        }
        probe.hub = component.createObject(null);
        const keys = [];
        for (const group of probe.hub.groups)
            for (const item of group.items)
                keys.push(item.key);
        probe.queue = keys;
        probe.next();
    }
}
