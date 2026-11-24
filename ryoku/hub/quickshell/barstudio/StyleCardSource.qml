import QtQuick
import Quickshell
import Quickshell.Io
import "StyleCard.js" as StyleCard

// Finds the cards a bar style declares for Bar Studio: the studio-card.json
// beside its Scene.qml. A style that ships with Ryoku is looked up in the
// shell's own tree first, the way BarProducts resolves a built-in before the
// Store, and the Store can never install a built-in's id; any other style is
// read from its verified Store view, the copy the shell loads. Last come the
// Hub's own declarations (barstudio/cards/<id>.json) for Store styles whose
// editors live in the Hub, Obi and Nacre. A missing file means no cards; a
// malformed one means no cards and one line in the log.
//
// The files are read as the page is built, not a frame later: Bar Studio's
// columns are balanced once, over the cards present at its first layout, so
// a card that arrived late would land in a different column than the
// hand-built card it replaces.
Item {
    id: source

    property string styleId: ""
    // the editor names the page can place, for an editor block to name one.
    property var editors: []

    // what the files say, worked out without side effects: the cards of the
    // first usable file, and why any file before it was refused.
    readonly property var lookup: {
        const warnings = [];
        const shipped = source.declaredIn(shippedFile, true, warnings);
        if (shipped !== null)
            return { cards: shipped, warnings: warnings };
        const store = source.declaredIn(storeFile, false, warnings);
        if (store !== null)
            return { cards: store, warnings: warnings };
        const hub = source.declaredIn(hubFile, true, warnings);
        return { cards: hub !== null ? hub : [], warnings: warnings };
    }
    readonly property var cards: source.lookup.cards
    readonly property bool declared: source.cards.length > 0

    // a file is read again on every change; say each problem once.
    property var warned: ({})
    function sayWarnings() {
        const next = {};
        for (const warning of source.lookup.warnings) {
            if (!source.warned[warning])
                console.warn(warning);
            next[warning] = true;
        }
        source.warned = next;
    }
    onLookupChanged: source.sayWarnings()
    Component.onCompleted: source.sayWarnings()

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

    // null for no file, an empty one or one that was refused, so the lookup
    // goes on to the next place as if the file were not there.
    function declaredIn(file, shipped, warnings) {
        if (file.path === "")
            return null;
        const text = file.text();
        if (!file.loaded)
            return null;
        const parsed = StyleCard.parse(text, { id: source.styleId, shipped: shipped, editors: source.editors });
        if (parsed.error !== "")
            warnings.push("bar studio: ignoring " + file.path + ": " + parsed.error);
        return parsed.cards.length > 0 ? parsed.cards : null;
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

    FileView {
        id: hubFile
        path: source.validId ? Quickshell.shellDir + "/barstudio/cards/" + source.styleId + ".json" : ""
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
    }
}
