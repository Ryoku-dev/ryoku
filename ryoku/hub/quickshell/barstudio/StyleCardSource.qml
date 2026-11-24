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
//
// The files are read as the page is built, not a frame later: Bar Studio's
// columns are balanced once, over the cards present at its first layout, so
// a card that arrived late would land in a different column than the
// hand-built card it replaces.
Item {
    id: source

    property string styleId: ""

    readonly property var cards: {
        const shipped = source.declaredIn(shippedFile, true);
        if (shipped !== null)
            return shipped;
        const store = source.declaredIn(storeFile, false);
        return store !== null ? store : [];
    }
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
    readonly property string storeView: StyleCard.storeView(
        storeIndexFile.loaded ? storeIndexFile.text() : "", source.styleId)

    // null for no file, so an empty list still means "read, and nothing usable".
    function declaredIn(file, shipped) {
        if (file.path === "")
            return null;
        const text = file.text();
        if (!file.loaded)
            return null;
        return source.read(file.path, text, shipped);
    }

    // a file is read again on every change; say each problem once.
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
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
    }

    FileView {
        id: storeIndexFile
        path: source.storeRoot + "/barstyles.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
    }

    FileView {
        id: storeFile
        path: source.storeView !== "" ? source.storeRoot + "/" + source.storeView + "/studio-card.json" : ""
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
    }
}
