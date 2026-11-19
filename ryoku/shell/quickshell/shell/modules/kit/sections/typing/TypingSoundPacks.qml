pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io

QtObject {
    id: packs

    property var clickPacks: []
    property var errorPacks: []
    property bool loaded: false
    readonly property string assetsPath: String(Qt.resolvedUrl("assets")).replace(/^file:\/\//, "")
    readonly property string soundsPath: assetsPath + "/sounds"

    function clickPack(id) {
        return clickPacks.find(pack => pack.id === id)
            ?? clickPacks.find(pack => pack.id === "click1") ?? null;
    }

    function errorPack(id) {
        return errorPacks.find(pack => pack.id === id)
            ?? errorPacks.find(pack => pack.id === "error1") ?? null;
    }

    function variantUrl(pack, index) {
        if (!pack || !pack.files || pack.files.length === 0)
            return "";
        return "file://" + soundsPath + "/" + pack.id + "/" + pack.files[index % pack.files.length];
    }

    property FileView manifest: FileView {
        path: packs.assetsPath + "/sounds-manifest.json"
        blockLoading: true
        onLoaded: {
            try {
                const parsed = JSON.parse(text());
                packs.clickPacks = Array.from(parsed.clickPacks || []);
                packs.errorPacks = Array.from(parsed.errorPacks || []);
                packs.loaded = packs.clickPacks.length > 0;
            } catch (error) {
                packs.loaded = false;
            }
        }
        onLoadFailed: packs.loaded = false
    }
}
