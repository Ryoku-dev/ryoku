pragma ComponentBehavior: Bound

import "items"
import QtQuick
import Quickshell
import Caelestia
import Caelestia.Config
import caelestia.components.controls
import caelestia.services
import caelestia.utils

PathView {
    id: root

    required property StyledTextField search
    required property var visibilities
    required property var panels
    required property var content
    required property var contentList

    readonly property int itemWidth: Tokens.sizes.launcher.wallpaperWidth * 0.8 + Tokens.padding.medium * 2

    readonly property int numItems: {
        const screen = (QsWindow.window as QsWindow)?.screen;
        if (!screen)
            return 0;

        const isBarHorizontal = Config.bar.position === "top" || Config.bar.position === "bottom";
        const barThickness = isBarHorizontal ? panels.bar.implicitHeight : panels.bar.implicitWidth;
        const barMargins = Math.max(Config.border.thickness, barThickness);
        let outerMargins = 0;

        if (panels.popouts.hasCurrent) {
            let overlaps = false;
            if (isBarHorizontal) {
                overlaps = true;
            } else {
                overlaps = panels.popouts.currentCenter + panels.popouts.nonAnimHeight / 2 > screen.height - content.implicitHeight - Config.border.thickness * 2;
            }

            if (overlaps) {
                if (isBarHorizontal) {
                    const popoutLeft = panels.popouts.currentCenter - panels.popouts.nonAnimWidth / 2;
                    const popoutRight = panels.popouts.currentCenter + panels.popouts.nonAnimWidth / 2;
                    const spaceFromEdge = panels.popouts.currentCenter > screen.width / 2 ? screen.width - popoutLeft : popoutRight;
                    outerMargins = Math.max(outerMargins, spaceFromEdge);
                } else {
                    outerMargins = Math.max(outerMargins, panels.popouts.nonAnimWidth);
                }
            }
        }

        if (panels.notifications.implicitHeight > 0) {
            let overlaps = false;
            const notifY = isBarHorizontal ? screen.height - panels.bottomMargin - panels.notifications.implicitHeight : panels.notifications.y;
            const launcherY = isBarHorizontal ? screen.height - panels.bottomMargin - content.implicitHeight : screen.height - content.implicitHeight;
            if (notifY + panels.notifications.implicitHeight > launcherY) {
                overlaps = true;
            }

            if (overlaps) {
                outerMargins = Math.max(outerMargins, panels.notifications.implicitWidth);
            }
        }

        if (visibilities.sidebar) {
            let overlaps = false;
            const sidebarY = screen.height - panels.bottomMargin - panels.sidebar.implicitHeight;
            const launcherY = isBarHorizontal ? screen.height - panels.bottomMargin - content.implicitHeight : screen.height - content.implicitHeight;
            if (sidebarY + panels.sidebar.implicitHeight > launcherY) {
                overlaps = true;
            }

            if (overlaps) {
                outerMargins = Math.max(outerMargins, panels.sidebar.implicitWidth);
            }
        }

        const maxWidth = screen.width - Config.border.rounding * 4 - (barMargins + outerMargins) * 2;

        if (maxWidth <= 0)
            return 0;

        const maxItemsOnScreen = Math.floor(maxWidth / itemWidth);
        const visible = Math.min(maxItemsOnScreen, Config.launcher.maxWallpapers, scriptModel.values.length);

        if (visible === 2)
            return 1;
        if (visible > 1 && visible % 2 === 0)
            return visible - 1;
        return visible;
    }

    model: ScriptModel {
        id: scriptModel

        readonly property string search: root.search.text.split(" ").slice(1).join(" ")

        values: {
            let res = [];
            if (search) {
                const allWalls = Wallpapers.query(search);
                const targetCategory = contentList.currentWallpaperTab;
                const baseDir = Paths.wallsdir;
                if (targetCategory === "Main") {
                    res = allWalls.filter(w => w.parentDir === baseDir);
                } else {
                    res = allWalls.filter(w => {
                        let cat = w.parentDir.slice(baseDir.length + 1);
                        if (cat.includes("/")) cat = cat.slice(0, cat.indexOf("/"));
                        return cat === targetCategory;
                    });
                }
            } else {
                res = Wallpapers.grouped[contentList.currentWallpaperTab] || [];
            }
            return res;
        }
        onValuesChanged: {
            let idx = search ? 0 : values.findIndex(w => w.path === Wallpapers.actualCurrent);
            root.currentIndex = Math.max(0, idx);
            if (values.length > 0 && root.currentIndex >= 0 && root.currentIndex < values.length) {
                previewTimer.restart();
            }
        }
    }

    Component.onCompleted: currentIndex = Wallpapers.list.findIndex(w => w.path === Wallpapers.actualCurrent)
    Component.onDestruction: Wallpapers.stopPreview()

    Timer {
        id: previewTimer

        interval: 100
        onTriggered: {
            if (!root.visibilities.launcher) return;
            if (root.currentItem)
                Wallpapers.preview((root.currentItem as WallpaperItem).modelData.path);
        }
    }

    onCurrentItemChanged: {
        if (currentItem)
            previewTimer.restart();
    }

    implicitWidth: Math.min(numItems, count) * itemWidth

    pathItemCount: numItems

    cacheItemCount: 4

    snapMode: PathView.SnapToItem

    preferredHighlightBegin: 0.5

    preferredHighlightEnd: 0.5

    highlightRangeMode: PathView.StrictlyEnforceRange

    delegate: WallpaperItem {
        visibilities: root.visibilities
    }

    path: Path {
        startY: root.height / 2

        PathAttribute {
            name: "z"
            value: 0
        }
        PathLine {
            x: root.width / 2
            relativeY: 0
        }
        PathAttribute {
            name: "z"
            value: 1
        }
        PathLine {
            x: root.width
            relativeY: 0
        }
    }

    CustomMouseArea {
        anchors.fill: parent
        preventStealing: false
        acceptedButtons: Qt.NoButton

        function onWheel(event: WheelEvent): void {
            if (event.angleDelta.y > 0 || event.angleDelta.x > 0)
                root.decrementCurrentIndex();
            else if (event.angleDelta.y < 0 || event.angleDelta.x < 0)
                root.incrementCurrentIndex();
        }
    }
}
