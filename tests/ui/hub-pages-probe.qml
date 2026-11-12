import QtQuick
import Quickshell
import "registry/registry.js" as Registry

// Every page in Settings' rail must open something that loads. Walks the
// registry the frame lists its categories from and compiles each page the way
// Hub.showContent loads it (pages/<file>.qml, Home when no page is open)
// through Quickshell's qs: loader, so a page that cannot name its siblings or
// a rail entry that lost its page fails here instead of opening blank. A
// hand-off page opens another app and only has to name it.
ShellRoot {
    id: probe

    property var hub: null
    property var queue: []
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
        const entry = probe.queue.shift();
        const component = Qt.createComponent(entry.url, Component.Asynchronous);
        const settle = function () {
            if (component.status === Component.Loading)
                return;
            if (component.status !== Component.Ready)
                probe.fail("page " + entry.key + " does not load: " + component.errorString());
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
            probe.fail("Settings does not load: " + component.errorString());
            Qt.quit();
            return;
        }
        probe.hub = component.createObject(null);
        if (!probe.hub) {
            probe.fail("Settings does not build: " + component.errorString());
            Qt.quit();
            return;
        }
        const queue = [{ key: "home", url: Qt.resolvedUrl("components/Home.qml") }];
        for (const id of Registry.allPageIds()) {
            const page = Registry.page(id);
            if (!page)
                probe.fail("rail entry " + id + " has no page");
            else if (page.handoff && page.handoff.length === 0)
                probe.fail("hand-off page " + id + " names no app");
            else if (!page.handoff && !page.file)
                probe.fail("page " + id + " has no file to open");
            else if (!page.handoff)
                queue.push({ key: id, url: Qt.resolvedUrl("pages/" + page.file + ".qml") });
        }
        if (queue.length < 2)
            probe.fail("the rail lists no page to open");
        probe.queue = queue;
        probe.next();
    }
}
