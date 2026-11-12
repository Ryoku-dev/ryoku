pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import "../registry/registry.js" as Registry

// what waits on the user, shown on Home under Needs you. An item is
// { id, title, detail, actionText, action, page, row, command, args }: English source
// strings (Home translates them, then fills %1, %2 from args); the action button logs
// out (after a Confirm) when action is "log-out", runs command when set, else opens
// page/row. Pages raise and clear items as they act;
// the registry's probes read the same state once when Settings opens, so it
// survives closing the window
Singleton {
    id: root

    property var items: []

    function raise(id, item) {
        var next = Object.assign({ id: id, title: "", detail: "", actionText: "", action: "", page: "", row: "", command: [], args: [] }, item || {});
        root.items = root.items.filter(i => i.id !== id).concat([next]);
    }

    function clear(id) {
        if (root.items.some(i => i.id === id))
            root.items = root.items.filter(i => i.id !== id);
    }

    function has(id) {
        return root.items.some(i => i.id === id);
    }

    // run one registry probe again, after the page changed what it reads
    function probe(id) {
        var p = Registry.attention.find(a => a.id === id);
        if (!p)
            return;
        Changes.run(p.get, function (ok, out) {
            root.settle(p, ok, out);
        });
    }

    // the placeholders (%1, %2) a string needs filled
    function needed(text) {
        var n = 0, m, re = /%(\d)/g;
        while ((m = re.exec(String(text || ""))) !== null)
            n = Math.max(n, Number(m[1]));
        return n;
    }

    // only a well-formed answer raises an item: an object that says pending: true or
    // carries args, with every placeholder its words need. Nothing, null, false, {},
    // anything malformed or a failed probe means nothing waits
    property var reported: ({})
    function settle(p, ok, out) {
        var v = null;
        try {
            v = ok && out.trim().length > 0 ? JSON.parse(out) : null;
        } catch (e) {
            v = null;
        }
        var args = v && Array.isArray(v.args) ? v.args.filter(a => a !== null && a !== undefined && String(a).length > 0) : [];
        var claims = !!v && typeof v === "object" && !Array.isArray(v) && v.pending !== false && (v.pending === true || args.length > 0);
        var item = claims ? Object.assign({}, p, v) : null;
        var fits = !!item && args.length >= Math.max(root.needed(item.title), root.needed(item.detail));
        if (!fits) {
            if (claims && !root.reported[p.id]) {
                root.reported[p.id] = true;
                console.info("hub: " + p.id + " probe answer lacks what its notice needs: " + out.trim());
            }
            root.clear(p.id);
            return;
        }
        delete item.get;
        delete item.id;
        delete item.pending;
        item.args = args;
        root.raise(p.id, item);
    }

    Component.onCompleted: {
        for (var i = 0; i < Registry.attention.length; i++)
            root.probe(Registry.attention[i].id);
    }
}
