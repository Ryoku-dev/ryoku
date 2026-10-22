pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Caelestia
import Caelestia.Config
import caelestia.components
import caelestia.components.containers
import caelestia.components.controls
import caelestia.services
import caelestia.utils
import caelestia.modules.launcher.items
import caelestia.modules.launcher.services

StyledListView {
    id: root

    required property DrawerVisibilities visibilities
    required property real maxWidth

    property var appsList: []

    function favouriteIds(): list<string> {
        return GlobalConfig.launcher.favouriteApps ?? [];
    }

    function sameIds(a: var, b: var): bool {
        if (a.length !== b.length)
            return false;
        for (let i = 0; i < a.length; ++i)
            if (a[i].id !== b[i].id)
                return false;
        return true;
    }

    function refresh(): void {
        const alpha = Apps.alphaApps;
        const favIds = root.favouriteIds();
        const hiddenIds = GlobalConfig.launcher.hiddenApps ?? [];

        const isVisible = a => a && !Strings.testRegexList(hiddenIds, a.id);
        const isFav = a => isVisible(a) && Strings.testRegexList(favIds, a.id);
        const isOther = a => isVisible(a) && !Strings.testRegexList(favIds, a.id);

        const favs = alpha.filter(isFav);
        const rank = a => {
            const i = favIds.indexOf(a.id);
            return i < 0 ? favIds.length : i;
        };
        favs.sort((a, b) => rank(a) - rank(b));

        const others = alpha.filter(isOther);
        const combined = [...favs, ...others];

        if (!root.sameIds(root.appsList, combined))
            root.appsList = combined;

        if (root.currentIndex >= root.count)
            root.currentIndex = Math.max(0, root.count - 1);
    }

    function launch(app: DesktopEntry): void {
        Apps.launch(app);
        root.visibilities.launcher = false;
    }

    function selectTile(index: int): void {
        root.currentIndex = index;
    }

    function moveLeft(): void {}

    function moveRight(): void {}

    function toggleFocus(): void {
        root.activateCurrent();
    }

    function activateCurrent(): void {
        const item = root.currentItem;
        if (item?.modelData)
            root.launch(item.modelData);
    }

    function openContextMenu(app: DesktopEntry, targetItem: Item): void {
        contextMenu.openFor(app, targetItem);
    }

    function resetView(): void {
        root.currentIndex = 0;
        root.cancelFlick();
        root.contentY = 0;
    }

    model: ScriptModel {
        values: root.appsList
    }

    spacing: Tokens.spacing.small
    orientation: Qt.Vertical
    implicitWidth: Math.min(Tokens.sizes.launcher.itemWidth, root.maxWidth)
    implicitHeight: Math.max(0, (Tokens.sizes.launcher.itemHeight + spacing) * Math.min(Config.launcher.maxShown, count) - spacing)
    cacheBuffer: Tokens.sizes.launcher.itemHeight * 10

    preferredHighlightBegin: 0
    preferredHighlightEnd: height
    highlightRangeMode: ListView.ApplyRange

    highlightFollowsCurrentItem: false
    highlight: StyledRect {
        radius: Tokens.rounding.large
        color: Colours.palette.m3onSurface
        opacity: 0.08

        y: root.currentItem?.y ?? 0
        implicitWidth: root.width
        implicitHeight: root.currentItem?.implicitHeight ?? 0

        Behavior on y {
            Anim {}
        }
    }

    StyledScrollBar.vertical: StyledScrollBar {
        flickable: root
    }

    delegate: AppItem {
        list: root
        visibilities: root.visibilities
    }

    Component.onCompleted: {
        root.refresh();
        root.resetView();
    }

    Connections {
        function onListChanged(): void {
            root.refresh();
        }

        target: Apps
    }

    Connections {
        function onFavouriteAppsChanged(): void {
            root.refresh();
        }

        function onHiddenAppsChanged(): void {
            root.refresh();
        }

        target: GlobalConfig.launcher
    }

    Connections {
        function onLauncherChanged(): void {
            if (root.visibilities.launcher)
                root.resetView();
        }

        target: root.visibilities
    }

    AppContextMenu {
        id: contextMenu

        attachTo: root
        visibilities: root.visibilities
    }
}
