pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import "../registry/progression.js" as Model

Singleton {
    id: root

    readonly property string directory: (Quickshell.env("XDG_DATA_HOME") ||
        ((Quickshell.env("HOME") || "") + "/.local/share")) + "/ryoku"
    property var data: Model.clean({})
    property var lastRead: Model.clean({})
    property var writingState: null
    property var pendingState: null
    property bool writeInFlight: false
    property bool saveBlocked: false
    property bool ready: false
    property bool directoryReady: false
    property string error: ""
    property var pending: []
    property var pendingVisits: []
    property string pendingSessionAt: ""
    property bool pendingKeepsake: false
    property bool editMode: false
    property string selectedSticker: ""
    readonly property int balance: data.balance || 0
    readonly property var achievements: Model.achievements
    readonly property var cosmetics: Model.cosmetics
    readonly property var completionScene: Model.completionScene
    readonly property bool completionSceneUnlocked: Model.completionSceneUnlocked(root.data)
    function progressInfoFor(id) { return Model.progressInfo(root.data, id); }
    readonly property var groups: Model.groups
    readonly property var rarityColours: Model.rarityColours
    readonly property int discoveredSecrets: Model.secretCount(root.data)
    function visibleBadges(category) { return Model.visibleAchievements(root.data, category); }
    function progressFor(id) { return Model.progress(root.data, id); }
    readonly property var stickers: Model.availableStickers(root.data)
    readonly property var canvas: data.canvas || ({ background: "", fonts: {}, stickers: [] })
    readonly property var activePalette: Model.palette(root.data)
    readonly property var visited: data.visited || ({})
    readonly property string featuredBadge: data.featured || ""

    Process {
        running: true
        command: ["mkdir", "-p", root.directory]
        onExited: (code) => {
            if (code !== 0)
                root.error = "Could not create the progression folder";
            else {
                root.directoryReady = true;
                file.reload();
            }
        }
    }

    FileView {
        id: file
        path: root.directory + "/progression.json"
        watchChanges: true
        atomicWrites: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: {
            try {
                var loaded = Model.clean(JSON.parse(text()));
                root.lastRead = loaded;
                root.data = loaded;
                if (!root.saveBlocked) root.error = "";
                root.ready = root.directoryReady;
                Qt.callLater(root.flushPending);
            } catch (e) {
                root.ready = false;
                root.error = "Progression data is invalid. Restore it before continuing.";
            }
        }
        onLoadFailed: (loadError) => {
            if (!root.directoryReady)
                return;
            if (!root.ready && loadError === FileViewError.FileNotFound) {
                root.data = Model.clean({});
                root.lastRead = root.data;
                root.error = "";
                root.ready = true;
                Qt.callLater(root.flushPending);
            } else {
                root.error = "Could not load progression data. The existing save has not been changed.";
            }
        }
        onSaved: {
            root.lastRead = root.writingState;
            root.writingState = null;
            root.writeInFlight = false;
            Qt.callLater(root.writePending);
        }
        onSaveFailed: (saveError) => {
            root.saveBlocked = true;
            root.pendingState = null;
            root.writingState = null;
            root.writeInFlight = false;
            root.data = root.lastRead;
            root.error = "Could not save progression data. Check disk space and permissions, then restart Ryoku Settings.";
        }
    }

    function flushPending() {
        if (!root.ready || root.error !== "")
            return;
        var queued = root.pending;
        root.pending = [];
        for (var i = 0; i < queued.length; i++)
            root.award(queued[i]);
        var visits = root.pendingVisits;
        root.pendingVisits = [];
        for (var j = 0; j < visits.length; j++)
            root.visitedPage(visits[j].page, visits[j].category, visits[j].at);
        if (root.pendingSessionAt !== "") {
            var at = root.pendingSessionAt;
            root.pendingSessionAt = "";
            root.openedHub(at);
        }
        if (root.pendingKeepsake) {
            root.pendingKeepsake = false;
            root.recordKeepsake();
        }
    }

    function openedHub(at) {
        var today = at || new Date().toISOString();
        if (!root.ready) {
            root.pendingSessionAt = today;
            return;
        }
        if (root.error !== "") return;
        var result = Model.startDay(root.data, today);
        if (result.changed) root.save(result.state);
    }

    function recordKeepsake() {
        if (!root.ready) {
            root.pendingKeepsake = true;
            return;
        }
        if (root.error !== "") return;
        var result = Model.keepsake(root.data);
        if (result.changed) root.save(result.state);
    }

    function writePending() {
        if (!root.ready || root.error !== "" || root.writeInFlight || !root.pendingState)
            return;
        root.writingState = root.pendingState;
        root.pendingState = null;
        root.writeInFlight = true;
        file.setText(JSON.stringify(root.writingState, null, 2) + "\n");
    }

    function save(state) {
        if (!root.ready || root.error !== "")
            return;
        var next = Model.clean(state);
        root.data = next;
        root.pendingState = next;
        Qt.callLater(root.writePending);
    }

    function award(id) {
        if (!root.ready) {
            root.pending = root.pending.concat([id]);
            return;
        }
        if (root.error !== "")
            return;
        var result = Model.unlock(root.data, id);
        if (result.changed)
            root.save(result.state);
    }

    function claim(id) {
        if (!root.ready || root.error !== "")
            return;
        var result = Model.claim(root.data, id);
        if (result.changed)
            root.save(result.state);
    }

    function purchase(id) {
        if (!root.ready || root.error !== "")
            return;
        var result = Model.purchase(root.data, id);
        if (result.changed)
            root.save(result.state);
    }

    function equip(id) {
        if (!root.ready || root.error !== "")
            return;
        var result = Model.equip(root.data, id);
        if (result.changed)
            root.save(result.state);
    }

    function visitedPage(page, category, at) {
        var when = at || new Date().toISOString();
        if (!root.ready) {
            root.pendingVisits = root.pendingVisits.concat([
                { page: page, category: category, at: when }
            ]);
            return;
        }
        if (root.error !== "") return;
        var result = Model.visit(root.data, page, category, when);
        if (result.changed)
            root.save(result.state);
    }

    function setBackground(id) {
        if (!root.ready || root.error !== "")
            return;
        var result = Model.background(root.data, id);
        if (result.changed)
            root.save(result.state);
    }

    function setCustomImage(url) {
        if (!root.ready || root.error !== "")
            return;
        var result = Model.customImage(root.data, String(url));
        if (result.changed)
            root.save(result.state);
    }

    function setFont(role, id) {
        if (!root.ready || root.error !== "")
            return;
        var result = Model.font(root.data, role, id);
        if (result.changed)
            root.save(result.state);
    }

    function addSticker(id) {
        if (!root.ready || root.error !== "")
            return;
        var result = Model.addSticker(root.data, id);
        if (result.changed)
            root.save(result.state);
    }

    function moveSticker(slot, x, y, size, rotation) {
        if (!root.ready || root.error !== "")
            return;
        var result = Model.moveSticker(root.data, slot, x, y, size, rotation);
        if (result.changed)
            root.save(result.state);
    }

    function reorderSticker(slot, direction) {
        if (!root.ready || root.error !== "") return;
        var result = Model.reorderSticker(root.data, slot, direction);
        if (result.changed) root.save(result.state);
    }

    function removeSticker(slot) {
        if (!root.ready || root.error !== "")
            return;
        var result = Model.removeSticker(root.data, slot);
        if (result.changed)
            root.save(result.state);
    }

    function fontFamily(role, fallback) {
        var id = root.canvas.fonts ? root.canvas.fonts[role] : "";
        var item = Model.cosmetic(id);
        return item && item.type === "font" ? item.family : fallback;
    }

    function stickerArt(id) {
        return Model.stamp(id);
    }

    function recordSignal() {
        if (!root.ready || root.error !== "")
            return;
        var result = Model.signal(root.data);
        if (result.changed)
            root.save(result.state);
    }

    function featureBadge(id) {
        if (!root.ready || root.error !== "")
            return;
        var result = Model.feature(root.data, id);
        if (result.changed)
            root.save(result.state);
    }

    function inspectProfile(profile) {
        var p = profile || {};
        if (p.text && (String(p.text.name || "").trim() !== "" || String(p.text.tagline || "").trim() !== ""))
            root.award("identity");
        if (p.hero && p.hero.kind && p.hero.kind !== "default")
            root.award("portrait");
        if (p.preset === "minimal" || p.homeView === "card" || p.heroSide === "left")
            root.award("layout");
        if (p.blocks && Object.keys(p.blocks).some(function(key) { return p.blocks[key] === false; }))
            root.award("layout");
    }
}
