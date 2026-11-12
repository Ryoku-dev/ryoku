import QtQuick
import "../../Singletons"
import "anim.js" as Anim

// one window-manager animation as a Changes value, so each has its own undo,
// changed mark and reset; the writes land in the desktop store's list of them
QtObject {
    id: store

    // the desktop store key of the list, set by the page
    property string itemsKey: ""
    readonly property var itemsBind: ({ store: "desktop", key: store.itemsKey })

    function listed(list, leaf) {
        for (var i = 0; i < (list || []).length; i++)
            if (list[i] && list[i].leaf === leaf)
                return list[i];
        return null;
    }

    function defaultsOf(leaf) {
        if (store.itemsKey === "" || !Changes.desktopDefaultsReady)
            return null;
        return store.listed(Changes.pathGet(Changes.desktopDefaults, Changes.storePath(store.itemsKey)), leaf);
    }

    function shape(leaf, it) {
        return Anim.leafShape(leaf, it);
    }

    function read(leaf) {
        if (store.itemsKey === "")
            return undefined;
        var it = store.listed(Changes.value(store.itemsBind), leaf) || store.defaultsOf(leaf);
        return it ? store.shape(leaf, it) : undefined;
    }

    function write(leaf, v, done) {
        var arr = (Changes.value(store.itemsBind) || []).slice();
        var next = store.shape(leaf, v || {}), found = false;
        for (var i = 0; i < arr.length; i++)
            if (arr[i] && arr[i].leaf === leaf) {
                arr[i] = next;
                found = true;
            }
        if (!found)
            arr.push(next);
        Changes.desktopEdit(store.itemsBind, arr, "", "animations");
        // the failure belongs to this row, not to the list's desktop row
        Changes.afterDesktopSave(function (ok, error) {
            if (!ok && Changes.error(store.itemsBind) !== "")
                Changes.setError(store.itemsBind, "");
            done(ok, error);
        });
    }

    Component.onCompleted: Changes.registerStore("anim-leaves", store)
}
