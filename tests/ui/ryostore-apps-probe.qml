import QtQuick
import Quickshell
import "ryostore" as Ryo
import "ryostore/Singletons" as RyoState

ShellRoot {
    id: root

    property int phase: 0
    property int attempts: 0
    property string firstKey: ""

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

            if (app.detailOpen || !app.appsBrowse)
                return;
            root.require(app.selectedKey === "flatpak:org.example.Beta", "closing the detail restores the app selection");
            root.require(app.appSort === "popular", "closing the detail keeps the chosen sort");
            console.log("RYOSTORE-APPS-PROBE-PASS");
            Qt.quit();
        }
    }
}
