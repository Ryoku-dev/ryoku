import QtQuick
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../../schema/phone.js" as Schema

// picture quality and webcam size, each several shell keys set as one choice;
// Changes owns the store so undo works after leaving the page
QtObject {
    id: store

    // the choice being written, so the half-written keys never read as another choice
    property var pending: ({})

    function read(key) {
        if (store.pending[key] !== undefined)
            return store.pending[key];
        return Schema.composite(key, k => Settings.get(k));
    }

    function hold(key, v) {
        var next = Object.assign({}, store.pending);
        if (v === undefined)
            delete next[key];
        else
            next[key] = v;
        store.pending = next;
    }

    function write(key, v, done) {
        var parts = Schema.compositeParts(key, v);
        if (parts.length === 0) {
            done(true, "");
            return;
        }
        store.hold(key, v);
        var left = parts.length, failed = "";
        parts.forEach(function (part) {
            Settings.patch(part[0], part[1], function (ok, why) {
                if (!ok && failed === "")
                    failed = String(why || I18n.tr("The settings service did not take the change."));
                left--;
                if (left === 0) {
                    store.hold(key, undefined);
                    done(failed === "", failed);
                }
            });
        });
    }

    Component.onCompleted: Changes.registerStore("phone", store)
}
