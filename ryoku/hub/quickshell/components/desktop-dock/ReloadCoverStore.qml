import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../../ReloadCoverModel.js" as ReloadCoverModel

// the picture or video shown while the desktop reloads, as a Changes store
// over brand.json; it lives as long as the window so undo can bring back an
// asset imported earlier, and copies no longer named are pruned at open
QtObject {
    id: store

    readonly property string configDir: String(Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku"
    property bool loaded: false
    property bool pruned: false

    function read(key) {
        if (key !== "cover" || !store.loaded)
            return undefined;
        return ReloadCoverModel.normalize(brandAdapter.reloadCover);
    }

    function write(key, v, done) {
        if (key !== "cover") {
            done(false, I18n.tr("The change did not apply."));
            return;
        }
        var before = ReloadCoverModel.normalize(brandAdapter.reloadCover);
        var next = ReloadCoverModel.normalize(v);
        brandAdapter.reloadCover = next;
        Changes.touch();
        // the same five keys writeAdapter() serializes
        Changes.writeText(brand, JSON.stringify({
            markText: brandAdapter.markText, markImage: brandAdapter.markImage, markTint: brandAdapter.markTint,
            name: brandAdapter.name, reloadCover: next
        }, null, 2) + "\n", function (ok, error) {
            // a save that failed changed nothing, so the row shows the cover still on disk
            if (!ok && JSON.stringify(ReloadCoverModel.normalize(brandAdapter.reloadCover)) === JSON.stringify(next)) {
                brandAdapter.reloadCover = before;
                Changes.touch();
            }
            done(ok, ok ? "" : I18n.tr("Could not save the change. Check that your settings folder can be written, then try again."));
        });
    }

    property JsonAdapter brandAdapter: JsonAdapter {
        property string markText: "力"
        property string markImage: ""
        property bool markTint: true
        property string name: "Ryoku"
        property var reloadCover: ReloadCoverModel.empty()
    }

    property FileView brand: FileView {
        path: store.configDir + "/brand.json"
        watchChanges: true
        atomicWrites: true
        printErrors: false
        adapter: store.brandAdapter
        onFileChanged: reload()
        onLoaded: {
            store.loaded = true;
            Changes.touch();
            if (!store.pruned) {
                store.pruned = true;
                var keep = ReloadCoverModel.path(store.brandAdapter.reloadCover);
                Spawn.run(keep !== "" ? ["ryoku-hub", "reload-cover", "prune", keep] : ["ryoku-hub", "reload-cover", "prune"]);
            }
        }
        onLoadFailed: error => {
            // no brand.json yet: the shipped mark shows until the first change writes one
            store.loaded = true;
            Changes.touch();
        }
    }

    Component.onCompleted: Changes.registerStore("reloadcover", store)
}
