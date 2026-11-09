pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

// Game Bar state: whether the strip is up, which monitor it belongs to, the
// cards on screen, and each card's saved spot and size. All of it persists.
Singleton {
    id: root

    readonly property var order: ["capture", "performance", "resources", "tasks", "media", "voice", "audio", "gallery"]
    readonly property string defaultCard: "performance"

    // The strip shows the remembered set; closing it leaves only pinned cards.
    readonly property var visibleCards: {
        const out = [];
        for (let i = 0; i < root.order.length; i++) {
            const id = root.order[i];
            if (root.pinned.indexOf(id) !== -1 || (root.open && root.opened.indexOf(id) !== -1))
                out.push(id);
        }
        return out;
    }

    function isPinned(id) {
        return root.pinned.indexOf(id) !== -1;
    }

    function toggleCard(id) {
        const open = root.opened.slice();
        const pinned = root.pinned.slice();
        const listed = open.indexOf(id) !== -1 || pinned.indexOf(id) !== -1;
        if (listed) {
            root.opened = open.filter(x => x !== id);
            root.pinned = pinned.filter(x => x !== id);
            return;
        }
        root.opened = open.concat([id]);
    }

function setPinned(id, on) {
    if (on === root.isPinned(id))
        return;
    if (!on && root.open && root.opened.indexOf(id) === -1)
        root.opened = root.opened.concat([id]);
    root.pinned = on ? root.pinned.concat([id])
        : root.pinned.filter(x => x !== id);
}

    function togglePinned(id) {
        root.setPinned(id, !root.isPinned(id));
    }

    function closeCard(id) {
        root.opened = root.opened.filter(x => x !== id);
        root.pinned = root.pinned.filter(x => x !== id);
    }

    function positionFor(id) {
        const list = root.layout || [];
        for (let i = 0; i < list.length; i++) {
            if (list[i] && list[i].id === id)
                return list[i];
        }
        return null;
    }

    function place(id, x, y) {
        const next = (root.layout || []).filter(e => e && e.id !== id);
        next.push({ id: id, x: Math.round(x), y: Math.round(y) });
        root.layout = next;
    }

    function sizeFor(id) {
        const list = root.sizes || [];
        for (let i = 0; i < list.length; i++) {
            if (list[i] && list[i].id === id)
                return list[i];
        }
        return null;
    }

    function resize(id, w, h) {
        const next = (root.sizes || []).filter(e => e && e.id !== id);
        next.push({ id: id, w: Math.round(w), h: Math.round(h) });
        root.sizes = next;
    }

    function openBar(mon) {
        if (root.open)
            return;
        root.monitor = mon && mon.length > 0 ? mon : (Wm.focusedOutput || root.monitor);
        root.open = true;
    }

    function closeBar() {
        root.open = false;
    }

    function toggleBar(mon) {
        if (root.open)
            root.closeBar();
        else
            root.openBar(mon);
    }

    FileView {
        id: file
        path: (Quickshell.env("XDG_STATE_HOME") || (Quickshell.env("HOME") + "/.local/state")) + "/ryoku/gamebar.json"
        blockLoading: true
        printErrors: false
        atomicWrites: true
        onAdapterUpdated: writeAdapter()

        JsonAdapter {
            id: adapter
            property bool open: false
            property var pinned: []
            property var cards: []
            property var layout: []
            property var sizes: []
            property string monitor: ""
            property int resourceTab: 0
        }
    }

    property alias open: adapter.open
    property alias pinned: adapter.pinned
    property alias opened: adapter.cards
    property alias layout: adapter.layout
    property alias sizes: adapter.sizes
    property alias monitor: adapter.monitor
    property alias resourceTab: adapter.resourceTab

    // The shell is the only writer, so the file is read once and never watched:
    // a watch would reload the state just after saving it and could put back a
    // card the pointer had already turned off.
    //
    // A file from before the open set was saved has no cards key, so seed it
    // once from the pins, or with the card the bar used to open on.
    Component.onCompleted: {
        const text = file.text();
        if (text && text.indexOf("\"cards\"") !== -1)
            return;
        if (root.opened.length > 0)
            return;
        root.opened = root.pinned.length > 0 ? root.pinned.slice() : [root.defaultCard];
        file.writeAdapter();
    }
}
