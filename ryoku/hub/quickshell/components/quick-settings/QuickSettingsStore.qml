import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../../schema/quick-settings.js" as Schema

// the panel's blocks, parts and extension cards as a Changes store, so each
// toggle, move and restore is one change with its own undo; it lives as long
// as the window so undo works after leaving the page
QtObject {
    id: store

    // extension cards: add-ons shown as cards on the panel, in their order
    property var cards: []
    property bool cardsLoaded: false
    property string cardsError: ""
    property bool placing: false
    property var placeQueue: []
    property var placeDone: []
    property string placeError: ""

    readonly property string discoverScript: {
        var dir = String(Quickshell.env("RYOKU_SHELL_DIR") || "");
        if (dir !== "")
            return dir + "/quickshell/plugins/discover.sh";
        return String(Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config"))
            + "/quickshell/plugins/discover.sh";
    }

    function current() {
        return Schema.normalize(Settings.get("controls.sections"), Settings.get("controls.hidden"));
    }

    function read(key) {
        var c = store.current();
        if (key === "order")
            return c.sections.map(s => s.id).join(",");
        if (key === "layout")
            return JSON.stringify(c);
        if (key === "cards")
            return store.cards.map(x => x.id).join(",");
        if (key.indexOf("block:") === 0) {
            var id = key.slice(6);
            var hit = c.sections.find(s => s.id === id);
            return hit ? hit.visible : undefined;
        }
        if (key.indexOf("part:") === 0)
            return c.hidden.indexOf(key.slice(5)) < 0;
        return undefined;
    }

    function patch(path, value, done) {
        Settings.patch(path, value, function (ok, err) {
            done(ok === true, ok === true ? "" : String(err || ""));
        });
    }

    function write(key, v, done) {
        var c = store.current();
        if (key.indexOf("block:") === 0) {
            var id = key.slice(6);
            store.patch("controls.sections", c.sections.map(s => s.id === id ? { id: id, visible: v === true } : s), done);
        } else if (key.indexOf("part:") === 0) {
            var part = key.slice(5);
            var hidden = c.hidden.filter(h => h !== part);
            if (v !== true)
                hidden.push(part);
            store.patch("controls.hidden", hidden, done);
        } else if (key === "order") {
            var ids = String(v || "").split(",");
            var next = Schema.normalize(ids.map(i => c.sections.find(s => s.id === i) || i), c.hidden).sections;
            store.patch("controls.sections", next, done);
        } else if (key === "layout") {
            var want = JSON.parse(String(v));
            var n = Schema.normalize(want.sections, want.hidden);
            var pending = 2, failed = "";
            var both = function (ok, err) {
                if (!ok && failed === "")
                    failed = err || I18n.tr("The change did not apply.");
                if (--pending === 0)
                    done(failed === "", failed);
            };
            store.patch("controls.sections", n.sections, both);
            store.patch("controls.hidden", n.hidden, both);
        } else if (key === "cards") {
            store.placeCards(String(v || "").split(",").filter(x => x.length > 0), done);
        } else {
            done(false, I18n.tr("The change did not apply."));
        }
        Changes.touch();
    }

    // cards: discover.sh reads plugins.json the way the panel does

    function refreshCards() {
        if (discover.running)
            return;
        discover.running = true;
    }

    function cardOf(plugin) {
        var place = plugin.placement || {};
        var card = place.sidebarCard || {};
        var manifest = plugin.manifest || {};
        var defaults = manifest.defaults || {};
        return {
            id: String(plugin.id),
            label: String(card.label || defaults.label || manifest.name || plugin.id),
            glyph: String(card.glyph || defaults.glyph || defaults.icon || "extension"),
            tab: typeof card.tab === "string" && card.tab.trim() !== "" ? card.tab.trim() : "Plugins",
            order: isFinite(Number(card.order)) ? Number(card.order) : 0,
            summary: String(manifest.description || manifest.summary || "")
        };
    }

    function readFailure() {
        return I18n.tr("Could not read your add-ons. Run ryoku doctor to repair the desktop, then try again.");
    }

    function takeCards(text) {
        var all = [];
        try {
            all = JSON.parse(text || "[]");
        } catch (e) {
            store.cardsError = store.readFailure();
            store.cardsLoaded = true;
            return;
        }
        if (!Array.isArray(all))
            all = [];
        var out = all.filter(p => p && p.placement && p.placement.enabled === true && p.placement.host === "sidebarCard")
            .map(p => store.cardOf(p));
        out.sort((a, b) => a.order === b.order ? a.id.localeCompare(b.id) : a.order - b.order);
        store.cards = out;
        store.cardsError = "";
        store.cardsLoaded = true;
        Changes.touch();
    }

    // the new order lands at once; each card whose place changed is written in
    // turn, and a move made meanwhile queues behind the writes still running
    function placeCards(ids, done) {
        var byId = {};
        store.cards.forEach(c => byId[c.id] = c);
        var next = ids.filter(id => byId[id]).map(id => byId[id]);
        store.cards.forEach(c => { if (ids.indexOf(c.id) < 0) next.push(c); });
        var queue = [];
        next = next.map((c, i) => {
            var order = (i + 1) * 10;
            if (c.order !== order)
                queue.push(["ryoku-plugins-place", c.id, "sidebarCard", c.tab, String(order), c.label, c.glyph]);
            return Object.assign({}, c, { order: order });
        });
        store.cards = next;
        if (queue.length === 0 && !store.placing) {
            done(true, "");
            return;
        }
        store.placeQueue = store.placeQueue.concat(queue);
        store.placeDone = store.placeDone.concat([done]);
        if (store.placing)
            return;
        store.placeError = "";
        store.placing = true;
        store.placeNext();
    }

    function placeNext() {
        if (store.placeQueue.length === 0) {
            store.placing = false;
            var cbs = store.placeDone, err = store.placeError;
            store.placeDone = [];
            cbs.forEach(cb => cb(err === "", err));
            store.refreshCards();
            return;
        }
        var argv = store.placeQueue[0];
        store.placeQueue = store.placeQueue.slice(1);
        // a hung write ends after 15 s, so the arrows come back with Retry instead of staying greyed;
        // the first failure drops the writes still queued (each would fail the same way) and the
        // reread afterwards shows the order the add-ons really have
        Changes.run(argv, function (ok, out, err, code, late) {
            if (!ok) {
                store.placeQueue = [];
                if (store.placeError === "")
                    store.placeError = late ? I18n.tr("The new order was not saved: the change took too long. Try again.")
                        : code === -1 && String(err || "").trim() !== "" ? String(err).trim()
                        : I18n.tr("The new order was not saved. Check that the add-ons are still installed, then try again.");
            }
            store.placeNext();
        }, { timeout: 15000 });
    }

    property Process discover: Process {
        command: ["bash", store.discoverScript, "--all"]
        stdout: StdioCollector { id: discoverOut }
        stderr: StdioCollector { id: discoverErr }
        onExited: (code, status) => {
            if (code !== 0) {
                store.cardsError = store.readFailure();
                store.cardsLoaded = true;
                Changes.touch();
                return;
            }
            store.takeCards(discoverOut.text);
        }
    }

    Component.onCompleted: {
        Changes.registerStore("quicksettings", store);
        store.refreshCards();
    }
}
