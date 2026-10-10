import QtQuick
import Quickshell
import "ryostore" as Ryo
import "ryostore/Singletons" as RyoState

ShellRoot {
    id: root

    property int phase: 0
    property int attempts: 0
    property var dimensions: String(Quickshell.env("RYOSTORE_PROBE_SIZE") || "1180x760").split("x")
    readonly property int probeWidth: Number(dimensions[0]) || 1180
    readonly property int probeHeight: Number(dimensions[1]) || 760

    Ryo.App {
        id: app
        width: root.probeWidth
        height: root.probeHeight
    }

    function require(condition, label) {
        if (!condition)
            throw new Error("RYOSTORE-SHELL-PROBE-FAIL " + label);
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

    function inside(item, container) {
        if (!item || !container)
            return false;
        const topLeft = item.mapToItem(container, 0, 0);
        return topLeft.x >= -1 && topLeft.y >= -1
                && topLeft.x + item.width <= container.width + 1
                && topLeft.y + item.height <= container.height + 1;
    }

    Timer {
        interval: 50
        repeat: true
        running: true
        onTriggered: {
            root.attempts++;
            if (root.attempts > 200)
                throw new Error("RYOSTORE-SHELL-PROBE-FAIL timed out in phase " + root.phase);

            const header = root.findObject(app, "ryostore-header");
            const stage = root.findObject(app, "ryostore-stage");
            const grid = root.findObject(app, "ryostore-grid");
            if (root.phase === 0) {
                if (RyoState.Store.categories.length !== 6 || !header || !stage || !grid)
                    return;
                if (app.selectedKey !== "rices:paper") {
                    app.selectKey("rices:paper");
                    return;
                }
                root.require(root.findObject(app, "ryostore-rail"), "Hub-family navigation rail");
                root.require(app.view === "discover" && app.categoryID === "", "Discover route");
                root.require(root.findObject(app, "ryostore-header-discover"), "Discover control");
                root.require(root.findObject(app, "ryostore-header-search"), "Search control");
                root.require(root.findObject(app, "ryostore-header-library"), "Library control");
                root.require(root.findObject(app, "ryostore-page-head"), "page head");
                root.require(root.findObject(app, "ryostore-action-bar"), "action bar");
                root.require(root.findObject(app, "ryostore-stage-primary"), "primary action");
                root.require(root.findObject(app, "ryostore-stage-details"), "details action");
                root.require(root.findObject(app, "ryostore-status-ACTIVE"), "active status");
                root.require(root.inside(header, app), "header fits responsive window");
                root.require(root.inside(stage, app), "stage fits responsive window");
                root.require(root.inside(grid, app), "grid fits responsive window");
                root.require(stage.y + stage.height <= grid.y + 1, "hero sits above the grid");
                app.selectKey("plugins:market");
                root.phase = 1;
                return;
            }

            if (root.phase === 1) {
                if (app.selectedKey !== "plugins:market")
                    return;
                root.require(root.findObject(app, "ryostore-status-UPDATE"), "update status");
                root.require(root.findObject(app, "ryostore-stage-primary").text === "UPDATE",
                             "update is the primary product action");
                app.selectKey("bundles:creator");
                root.phase = 2;
                return;
            }

            if (root.phase === 2) {
                if (app.selectedKey !== "bundles:creator")
                    return;
                root.require(root.findObject(app, "ryostore-status-2 / 4 INSTALLED"), "partial status");
                app.openRoute("library");
                root.phase = 3;
                return;
            }

            if (root.phase === 3) {
                if (app.view !== "library" || app.collection.length !== 7)
                    return;
                root.require(app.collection.some(item => item.active), "Library retains active products");
                root.require(app.collection.some(item => item.updateAvailable), "Library retains updates");
                root.require(app.collection.some(item => Number(item.installedCount || 0) > 0),
                             "Library retains partial products");
                app.selectKey("lockscreens:clock");
                root.phase = 4;
                return;
            }

            if (app.selectedKey !== "lockscreens:clock")
                return;
            root.require(root.findObject(app, "ryostore-status-INSTALLED"), "installed status");
            console.log("RYOSTORE-SHELL-PROBE-PASS");
            Qt.quit();
        }
    }
}
