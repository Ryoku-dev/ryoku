import QtQuick
import Quickshell
import "ryostore" as Ryo
import "ryostore/Singletons" as RyoState

ShellRoot {
    id: root

    property int phase: 0
    property int attempts: 0
    property string firstKey: ""
    property string proofDir: Quickshell.env("RYOSTORE_PROOF_DIR")
    property bool proofPending: false
    readonly property int captureWidth: Number(Quickshell.env("RYOSTORE_PROBE_WIDTH") || "1180")

    FloatingWindow {
        id: win
        implicitWidth: root.captureWidth
        implicitHeight: 760

        Ryo.App {
            id: app
            anchors.fill: parent
        }
    }

    function require(condition, label) {
        if (!condition)
            throw new Error("RYOSTORE-APPS-PROBE-FAIL " + label);
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

    function keys() {
        return app.collection.map(item => item.id);
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
                throw new Error("RYOSTORE-APPS-PROBE-FAIL timed out in phase " + root.phase);

            if (root.phase === 0) {
                if (RyoState.Store.items.length < 5)
                    return;
                app.openRoute("flatpak");
                root.phase = 1;
                return;
            }

            if (root.phase === 1) {
                if (!app.appsBrowse)
                    return;
                if (root.captureWidth < 980) {
                    root.require(app.narrowAppsTools, "narrow Apps tools breakpoint");
                    const tools = root.findObject(app, "ryostore-apps-tools");
                    const search = root.findObject(app, "ryostore-apps-search");
                    root.require(tools && search && tools.height >= 88,
                                 "narrow Apps tools reserve a second row");
                    root.require(search.y >= 44,
                                 "narrow Apps search sits below the sort chips");
                }
                root.require(root.findObject(app, "ryostore-apps-hero") !== null, "featured hero is mounted");
                root.require(root.findObject(app, "ryostore-apps-sort") !== null, "sort strip is mounted");
                const grid = root.findObject(app, "ryostore-grid");
                root.require(grid !== null, "compact grid is mounted");
                root.require(grid.compact, "grid uses the compact app layout");
                root.require(grid.cellH < 140, "compact cells are shorter than the artwork cards");
                root.require(JSON.stringify(root.keys()) === JSON.stringify(
                        ["org.example.Gamma", "org.example.Alpha", "org.example.Beta",
                         "org.example.Epsilon", "org.example.Delta"]),
                        "Trending is the default order");
                root.firstKey = app.selectedKey;
                const popular = root.findObject(app, "ryostore-provider-popular");
                root.require(popular !== null, "Popular plate exists");
                popular.chose();
                root.phase = 2;
                return;
            }

            if (root.phase === 2) {
                if (app.appSort !== "popular")
                    return;
                root.require(root.keys()[0] === "org.example.Beta", "Popular reorders by monthly installs");
                root.require(app.selectedKey !== root.firstKey, "selection follows the new order");
                app.selectKey("flatpak:org.example.Beta");
                app.openSelectedDetail();
                root.phase = 3;
                return;
            }

            if (root.phase === 3) {
                if (!app.detailOpen)
                    return;
                root.require(app.detailItem && app.detailItem.id === "org.example.Beta", "detail opens the selected app");
                app.escapeLayer();
                root.phase = 4;
                return;
            }

            if (root.phase === 4) {
                if (app.detailOpen || app.detailItem !== null || !app.appsBrowse)
                    return;
                root.require(app.selectedKey === "flatpak:org.example.Beta", "closing the detail restores the app selection");
                root.require(app.appSort === "popular", "closing the detail keeps the chosen sort");
                root.require(app.appTabs.length === 4, "market tabs list the categories present");
                root.require(root.findObject(app, "ryostore-apps-tabs") !== null, "category strip is mounted");
                root.require(root.findObject(app, "ryostore-apps-search-field") !== null, "market search field is mounted");
                app.setAppTag("game");
                root.phase = 5;
                return;
            }

            if (root.phase === 5) {
                if (app.appsTag !== "game")
                    return;
                root.require(JSON.stringify(root.keys()) === JSON.stringify(
                        ["org.example.Epsilon", "org.example.Gamma"]),
                        "a category tab narrows the market");
                root.require(app.appsFiltered, "the market reports a filter");
                root.require(!root.findObject(app, "ryostore-apps-hero").visible,
                             "the featured shelf hides under a filter");
                app.setAppsQuery("eps");
                root.phase = 6;
                return;
            }

            if (root.phase === 6) {
                if (app.appsQuery !== "eps")
                    return;
                root.require(JSON.stringify(root.keys()) === JSON.stringify(["org.example.Epsilon"]),
                             "the market search narrows the chosen tab");
                app.clearAppFilter();
                root.phase = 7;
                return;
            }

            if (app.appsFiltered)
                return;
            root.require(app.appsTag === "" && app.appsQuery === "", "clearing the filter resets both");
            root.require(root.keys().length === 5, "the whole market returns");
            if (root.proofPending)
                return;
            root.saveProof(root.captureWidth < 900
                    ? "ryostore-apps-narrow.png" : "ryostore-apps-wide.png", function() {
                console.log("RYOSTORE-APPS-PROBE-PASS");
                Qt.quit();
            });
        }
    }
}
