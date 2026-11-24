import QtQuick
import Quickshell
import Quickshell.Io
import "StyleCard.js" as StyleCard

// Finds the cards a bar style declares for Bar Studio: the studio-card.json
// beside its Scene.qml. A style that ships with Ryoku is looked up in the
// shell's own tree first, the way BarProducts resolves a built-in before the
// Store, and the Store can never install a built-in's id; any other style is
// read from its verified Store view, the copy the shell loads. A missing file
// means no cards; a malformed one means no cards and one line in the log.
Item {
    id: source

    property string styleId: ""

    readonly property var cards: source.shippedCards !== null ? source.shippedCards
        : source.storeCards !== null ? source.storeCards : []
    readonly property bool declared: source.cards.length > 0

    // the shell's QML, as LauncherPage finds it: the dev checkout when the
    // shell runs from one, otherwise the copy deployed beside the Hub.
    readonly property string shellRoot: {
        const dev = String(Quickshell.env("RYOKU_SHELL_DIR") || "");
        if (dev.length > 0)
            return dev + "/quickshell/shell";
        return String(Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config"))
            + "/quickshell/shell";
    }
    readonly property string storeRoot: (Quickshell.env("XDG_STATE_HOME")
        || (Quickshell.env("HOME") + "/.local/state")) + "/ryoku/store"
    readonly property bool validId: /^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(source.styleId)
    property string storeIndex: ""
    readonly property string storeView: StyleCard.storeView(source.storeIndex, source.styleId)

    // null while a file is absent or unread, so an empty card list still means
    // "this file was read and said nothing usable".
    property var shippedCards: null
    property var storeCards: null

    // a file can load twice as it is first watched; say each problem once.
    property string lastWarning: ""
    function read(path, text, shipped) {
        const parsed = StyleCard.parse(text, { id: source.styleId, shipped: shipped });
        const warning = parsed.error === "" ? "" : "bar studio: ignoring " + path + ": " + parsed.error;
        if (warning !== "" && warning !== source.lastWarning)
            console.warn(warning);
        source.lastWarning = warning;
        return parsed.cards;
    }

    FileView {
        id: shippedFile
        path: source.validId ? source.shellRoot + "/modules/bar/barstyles/" + source.styleId + "/studio-card.json" : ""
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onPathChanged: source.shippedCards = null
        onLoaded: source.shippedCards = source.read(shippedFile.path, shippedFile.text(), true)
        onLoadFailed: source.shippedCards = null
    }

    FileView {
        id: storeIndexFile
        path: source.storeRoot + "/barstyles.json"
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: source.storeIndex = storeIndexFile.text()
        onLoadFailed: source.storeIndex = ""
    }

    FileView {
        id: storeFile
        path: source.storeView !== "" ? source.storeRoot + "/" + source.storeView + "/studio-card.json" : ""
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onPathChanged: source.storeCards = null
        onLoaded: source.storeCards = source.read(storeFile.path, storeFile.text(), false)
        onLoadFailed: source.storeCards = null
    }
}
