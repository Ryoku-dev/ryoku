import QtQuick
import Quickshell
import "ryostore" as Ryo
import "ryostore/Singletons" as RyoState

ShellRoot {
    id: root

    property int phase: 0
    property real savedOffset: 0
    property int attempts: 0
    property string proofDir: Quickshell.env("RYOSTORE_PROOF_DIR")
    property bool proofPending: false

    FloatingWindow {
        implicitWidth: 1180
        implicitHeight: 760

        Ryo.App {
            id: app
            anchors.fill: parent
        }
    }

    function require(condition, label) {
        if (!condition)
            throw new Error("RYOSTORE-FLOW-PROBE-FAIL " + label);
    }

    function findObject(item, name) {
        if (!item)
            return null;
        if (item.objectName === name)
            return item;
        const children = item.children || [];
        for (const child of children) {
            const found = root.findObject(child, name);
            if (found)
                return found;
        }
        return null;
    }

    function item(id) {
        for (const candidate of RyoState.Store.items)
            if (candidate.id === id)
                return candidate;
        return null;
    }

    function saveProof(name, done) {
        if (proofDir === "") {
            done();
            return;
        }
        proofPending = true;
        app.grabToImage(function(result) {
            root.require(result.saveToFile(proofDir + "/" + name), "saved " + name);
            proofPending = false;
            done();
        });
    }

    Timer {
        interval: 50
        repeat: true
        running: true
        onTriggered: {
            root.attempts++;
            if (root.attempts > 300)
                throw new Error("RYOSTORE-FLOW-PROBE-FAIL timed out in phase " + root.phase);

            if (root.phase === 0) {
                if (RyoState.Store.items.length < 10)
                    return;
                app.openRoute("lockscreens");
                app.selectKey("lockscreens:clock");
                root.require(app.categoryID === "lockscreens", "category deep link");
                root.require(app.selectedKey === "lockscreens:clock", "stable keyed selection");
                app.openSelectedDetail();
                root.phase = 1;
                return;
            }

            if (root.phase === 1) {
                if (!root.findObject(app, "ryostore-detail") || !app.detailItem)
                    return;
                const openGrid = root.findObject(app, "ryostore-grid");
                root.require(openGrid && openGrid.enabled === false,
                             "browse grid is inert while the detail is open");
                RyoState.Store.install(app.detailItem);
                root.phase = 2;
                return;
            }

            if (root.phase === 2) {
                if (RyoState.Store.busyKey !== "lockscreens:clock")
                    return;
                app.detailItem = root.item("broken");
                const blocked = root.findObject(app, "ryostore-detail-install");
                if (!blocked || blocked.text !== "DOWNLOADING ANOTHER ITEM")
                    return;
                root.require(blocked.armed === false, "another product is visibly disabled while a download runs");
                if (!root.proofPending)
                    root.saveProof("ryostore-download-blocked.png", function() {
                        app.detailItem = root.item("clock");
                        root.phase = 3;
                    });
                return;
            }

            if (root.phase === 3) {
                const installed = root.item("clock");
                if (!installed || !installed.installed || RyoState.Store.busyKey !== "")
                    return;
                root.require(installed.active === false, "install did not activate");
                root.require(app.detailItem.installed === true, "detail refreshed from backend");
                app.closeDetail();
                root.require(app.selectedKey === "lockscreens:clock", "detail restored selection");
                root.phase = 4;
                return;
            }

            if (root.phase === 4) {
                const grid = root.findObject(app, "ryostore-grid");
                if (!grid)
                    return;
                if (!grid.activeFocus) {
                    grid.forceActiveFocus();
                    return;
                }
                grid.restoreOffset(140);
                root.savedOffset = grid.contentY;
                root.require(root.savedOffset > 0, "fixture starts from a nonzero grid offset");
                app.searchFor("installed clock");
                const flowHeader = root.findObject(app, "ryostore-header");
                if (flowHeader)
                    flowHeader.focusSearch();
                root.phase = 5;
                return;
            }

            if (root.phase === 5) {
                const searchField = root.findObject(app, "ryostore-header-search-field");
                if (searchField && !searchField.activeFocus)
                    searchField.forceActiveFocus();
                if (!app.searchOpen || !searchField || !searchField.activeFocus)
                    return;
                root.require(app.collection.length === 1, "showroom search projection");
                root.require(app.selectedKey === "lockscreens:clock", "search retained matching selection");
                app.openSelectedDetail();
                root.phase = 6;
                return;
            }

            if (root.phase === 6) {
                const detail = root.findObject(app, "ryostore-detail");
                const detailClose = root.findObject(detail, "ryostore-detail-close");
                if (!app.detailOpen || !detail || !detailClose || !detailClose.activeFocus)
                    return;
                root.require(detail.activeFocus, "detail focus scope owns keyboard focus");
                root.require(detailClose.visible, "detail exposes a visible close action");
                app.escapeLayer();
                root.phase = 7;
                return;
            }

            if (root.phase === 7) {
                const searchField = root.findObject(app, "ryostore-header-search-field");
                if (!app.searchOpen || !searchField || !searchField.activeFocus)
                    return;
                root.require(app.query === "installed clock",
                             "detail restores query and search keyboard focus");
                app.escapeLayer();
                root.phase = 8;
                return;
            }

            if (root.phase === 8) {
                const restoredGrid = root.findObject(app, "ryostore-grid");
                if (app.searchOpen || !restoredGrid || !restoredGrid.activeFocus
                        || Math.abs(restoredGrid.contentY - root.savedOffset) >= 0.5)
                    return;
                root.require(app.categoryID === "lockscreens" && app.selectedKey === "lockscreens:clock",
                             "search restored exact context");
                root.require(Math.abs(app.gridOffset - root.savedOffset) < 0.5,
                             "coordinator mirrors restored grid offset");
                const names = app.collection.map(item => String(item.name || item.id));
                for (let i = 1; i < names.length; i++)
                    root.require(names[i - 1].localeCompare(names[i]) <= 0, "category is ordered A to Z");
                root.require(root.findObject(app, "ryostore-category-search-field") !== null,
                             "category search field is mounted");
                app.setCategoryQuery("broken");
                root.phase = 9;
                return;
            }

            if (app.collection.length !== 1 || app.collection[0].id !== "broken")
                return;
            root.require(app.categoryID === "lockscreens" && !app.searchOpen,
                         "category search remains scoped to the open category");
            if (!root.proofPending)
                root.saveProof("ryostore-category-search.png", function() {
                    console.log("RYOSTORE-FLOW-PROBE-PASS");
                    Qt.quit();
                });
        }
    }
}
