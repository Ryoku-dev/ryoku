import QtQuick
import Ryoku.Ui.Singletons
import "../../Singletons"

// the system card's design (profile.json) as a Changes store; a key is a dotted
// path, and "layout" is the preset with the parts it shows, so its Undo also
// brings back parts someone switched by hand
QtObject {
    id: root

    // "*" is the whole design, for reset and its undo
    function read(key) {
        ProfileStore.data;
        if (key === "*")
            return JSON.parse(JSON.stringify(ProfileStore.data || {}));
        if (key === "layout") {
            var d = ProfileStore.data || {};
            var out = {};
            if (d.preset !== undefined)
                out.preset = d.preset;
            if (d.blocks && typeof d.blocks === "object")
                out.blocks = JSON.parse(JSON.stringify(d.blocks));
            return out;
        }
        return ProfileStore.get(key, undefined);
    }

    // a value's keys in the order the object already has, so an Undo writes the
    // same bytes back even when the value came through a signal sorted
    function inOrder(old, value) {
        var out = {};
        if (old && typeof old === "object")
            for (var k in old)
                if (value[k] !== undefined)
                    out[k] = value[k];
        for (var n in value)
            if (out[n] === undefined && value[n] !== undefined)
                out[n] = value[n];
        return out;
    }

    function write(key, value, done) {
        // a refused write says where to look before Retry
        var answer = function (ok, error) {
            done(ok, ok ? "" : I18n.tr("%1. Check that your settings folder can be written to, then press Retry.")
                .replace("%1", String(error || I18n.tr("The system card could not be saved")).replace(/\.+$/, "")));
        };
        if (key === "*") {
            ProfileStore.replace(value || {}, answer);
            Changes.touch();
            return;
        }
        // the key's value is replaced, not merged into: an Undo that writes the
        // earlier hero back must not keep fields the change added
        var next = JSON.parse(JSON.stringify(ProfileStore.data || {}));
        var v = value === undefined ? undefined : JSON.parse(JSON.stringify(value));
        if (key === "layout") {
            v = v || {};
            if (v.preset === undefined)
                delete next.preset;
            else
                next.preset = v.preset;
            if (v.blocks && typeof v.blocks === "object")
                next.blocks = root.inOrder(next.blocks, v.blocks);
            else
                delete next.blocks;
        } else {
            var parts = String(key).split(".");
            var cur = next;
            for (var i = 0; i < parts.length - 1; i++) {
                if (!cur[parts[i]] || typeof cur[parts[i]] !== "object" || Array.isArray(cur[parts[i]]))
                    cur[parts[i]] = {};
                cur = cur[parts[i]];
            }
            var last = parts[parts.length - 1];
            if (v === undefined)
                delete cur[last];
            else if (v && typeof v === "object" && !Array.isArray(v))
                cur[last] = root.inOrder(cur[last], v);
            else
                cur[last] = v;
        }
        ProfileStore.replace(next, answer);
        Changes.touch();
    }

    property Connections watch: Connections {
        target: ProfileStore
        function onDataChanged() { Changes.touch(); }
    }

    Component.onCompleted: Changes.registerStore("card", root)
}
