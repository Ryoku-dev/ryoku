pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

Singleton {
    id: root

    readonly property string stateRoot: (Quickshell.env("XDG_STATE_HOME")
        || (Quickshell.env("HOME") + "/.local/state")) + "/ryoku/store"
    readonly property var builtInIndex: ({
        schema: 1,
        generatedAt: "",
        active: "orpheus",
        fallback: "orpheus",
        moduleRoot: "",
        items: [
            {
                id: "kairos",
                category: "subshells",
                source: "shipped",
                name: I18n.tr("Kairos"),
                version: "2.0.0",
                summary: I18n.tr("A clock island that opens into Android-style quick settings."),
                dir: "subshells/kairos",
                entry: "subshells/kairos/qml/Scene.qml",
                inUse: false,
                prepared: true,
                frameHost: "each",
                owns: ["osd", "notifications"],
                ownsSetting: { osd: "kairos.islandOsd", notifications: "kairos.islandNotifications" },
                modules: [],
                settings: { ipc: "kairos", call: "settings", args: [], hubEditor: "" },
                provides: {
                    dock: [],
                    launcher: [{
                        id: "kairos",
                        name: I18n.tr("Kairos"),
                        description: I18n.tr("The Kairos clock island opens into an Android-style app drawer."),
                        entry: "subshells/kairos/qml/launcher/Main.qml",
                        preview: "subshells/kairos/qml/launcher/Preview.qml"
                    }],
                    desktopWidgets: []
                },
                lifecycle: {},
                runtime: {}
            },
            {
                id: "orpheus",
                category: "subshells",
                source: "shipped",
                name: I18n.tr("Orpheus"),
                version: "1.0.0",
                summary: I18n.tr("A fluid edge shell shaped by music and motion."),
                dir: "subshells/orpheus",
                entry: "subshells/orpheus/qml/Scene.qml",
                inUse: true,
                prepared: true,
                frameHost: "each",
                owns: [],
                modules: [],
                settings: { ipc: "orpheus", call: "settings", args: [""], hubEditor: "" },
                provides: { dock: [], launcher: [], desktopWidgets: [] },
                lifecycle: {},
                runtime: {}
            }
        ]
    })

    property var index: builtInIndex
    // indexRead: shells.json was read (or found missing). Until then, and while
    // the file was written for a different selection, the selection is pending:
    // falling back early would load the fallback item's code for nothing.
    property bool indexRead: false
    readonly property bool indexCurrent: root.indexRead && (root.index === root.builtInIndex
        || String(root.index.active || "") === root.active)
    readonly property string active: Config.barStyle
    readonly property string fallbackId: "orpheus"
    property string brokenId: ""
    readonly property var selectedItem: item(active)
    readonly property bool activePending: !root.indexCurrent
        || (selectedItem !== null && selectedItem.prepared === false)
    readonly property var activeItem: {
        if (root.activePending)
            return root.selectedItem;
        if (root.brokenId === root.active || root.selectedItem === null)
            return root.item(root.fallbackId);
        return root.selectedItem;
    }
    readonly property url activeEntry: activePending || activeItem === null
        ? "" : url(activeItem, activeItem.entry)
    readonly property bool primaryHosted: activeItem !== null
        && activeItem.frameHost === "primary"
    property string activeBarEdge: "top"

    // Hand-offs the active item may set with its own Bindings while it runs;
    // they return to these defaults when it unloads.
    // desktopCanvas: an item that owns "widgets" gives the Desktop one canvas.
    property var desktopCanvas: null
    // notificationMs: how long the active item's banners stay up (0 = Ryoku's).
    property int notificationMs: 0
    // osdScale: the active item's size for Ryoku's shipped OSD panel.
    property real osdScale: 1

    function parse(raw) {
        try {
            const value = JSON.parse(raw || "");
            if (!value || value.schema !== 1 || !Array.isArray(value.items))
                return root.builtInIndex;
            return value;
        } catch (e) {
            return root.builtInIndex;
        }
    }

    function item(id) {
        const wanted = String(id || "");
        const items = root.index && Array.isArray(root.index.items)
            ? root.index.items : [];
        for (let i = 0; i < items.length; ++i) {
            if (String(items[i]?.id || "") === wanted)
                return items[i];
        }
        return null;
    }

    function url(record, relPath) {
        if (!record)
            return "";
        const path = String(relPath || "");
        if (path.length === 0)
            return "";
        if (path.indexOf("file://") === 0)
            return path;
        if (record.source === "store")
            return path[0] === "/" ? "file://" + path : "";
        return Qt.resolvedUrl("../" + path);
    }

    function contributions(kind) {
        if (["dock", "launcher", "desktopWidgets"].indexOf(kind) < 0)
            return [];
        const result = [];
        const items = root.index && Array.isArray(root.index.items)
            ? root.index.items : [];
        for (let i = 0; i < items.length; ++i) {
            const record = items[i];
            const rows = record?.provides && Array.isArray(record.provides[kind])
                ? record.provides[kind] : [];
            for (let j = 0; j < rows.length; ++j) {
                const contribution = rows[j] || {};
                const prepared = record.prepared !== false;
                result.push({
                    itemId: String(record.id || ""),
                    id: String(contribution.id || ""),
                    name: String(contribution.name || record.name || contribution.id || ""),
                    description: String(contribution.description || ""),
                    entryUrl: prepared ? root.url(record, contribution.entry) : "",
                    previewUrl: prepared ? root.url(record, contribution.preview) : "",
                    widgetUrl: prepared ? root.url(record, contribution.widget) : "",
                    optionsUrl: prepared ? root.url(record, contribution.options) : "",
                    roster: Array.isArray(contribution.roster) ? contribution.roster : [],
                    prepared: prepared,
                    record: record
                });
            }
        }
        return result;
    }

    // An item owns a surface while it is running, lists it in "owns", and the
    // shell.json switch it names in "ownsSetting" (if any) is not false.
    function owns(surface) {
        if (root.activePending || root.activeItem === null)
            return false;
        const name = String(surface || "");
        const values = Array.isArray(root.activeItem.owns) ? root.activeItem.owns : [];
        if (values.indexOf(name) < 0)
            return false;
        const switches = root.activeItem.ownsSetting || {};
        const key = String(switches[name] || "");
        return key === "" || Config.value(key) !== false;
    }

    function markBroken(id) {
        if (String(id || "") === root.active)
            root.brokenId = root.active;
    }

    function requestSync() {
        syncDebounce.restart();
    }

    onActiveChanged: {
        brokenId = "";
        activeBarEdge = "top";
    }
    onIndexCurrentChanged: if (indexRead && !indexCurrent) requestSync()

    FileView {
        id: indexFile
        path: root.stateRoot + "/shells.json"
        blockLoading: true
        watchChanges: true
        atomicWrites: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: {
            root.index = root.parse(text());
            root.indexRead = true;
        }
        onLoadFailed: {
            root.index = root.builtInIndex;
            root.indexRead = true;
        }
    }

    Timer {
        id: syncDebounce
        interval: 150
        onTriggered: Bus.run("shells sync")
    }
}
