pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services
import caelestia.utils
import caelestia.modules.launcher.items
import caelestia.modules.launcher.services

Item {
    id: root

    required property DrawerVisibilities visibilities
    required property real maxWidth

    readonly property var currentItem: root.itemAt(root.currentIndex)
    readonly property int count: root.favourites.length + root.others.length
    readonly property bool showSeparator: root.favourites.length > 0 && root.others.length > 0

    readonly property int padding: Tokens.padding.large
    readonly property int tileWidth: Tokens.sizes.launcher.browseTileWidth
    readonly property int tileHeight: Tokens.sizes.launcher.browseTileHeight
    readonly property int gridSpacing: Tokens.spacing.medium
    readonly property int columns: Math.max(1, Math.floor((root.implicitWidth - root.padding * 2 + root.gridSpacing) / (root.tileWidth + root.gridSpacing)))
    readonly property int cellWidth: Math.floor((root.implicitWidth - root.padding * 2) / root.columns)
    readonly property int cellHeight: root.tileHeight + root.gridSpacing

    readonly property int favRows: Math.ceil(root.favourites.length / root.columns)
    readonly property int otherRows: Math.ceil(root.others.length / root.columns)

    property var favourites: []
    property var others: []
    property int currentIndex: 0

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

        if (!root.sameIds(root.favourites, favs))
            root.favourites = favs;
        if (!root.sameIds(root.others, others))
            root.others = others;

        if (root.currentIndex >= root.count)
            root.currentIndex = Math.max(0, root.count - 1);
    }

    function launch(app): void {
        Apps.launch(app);
        root.visibilities.launcher = false;
    }

    function selectTile(index: int): void {
        root.currentIndex = index;
    }

    function itemAt(index: int): Item {
        if (index < 0)
            return null;
        if (index < root.favourites.length)
            return grid.headerItem?.favouriteRepeater?.itemAt(index)?.tile ?? null;
        return grid.itemAtIndex(index - root.favourites.length)?.tile ?? null;
    }

    function moveBy(delta: int): void {
        if (root.count === 0)
            return;
        const from = root.currentIndex < 0 ? 0 : root.currentIndex;
        root.currentIndex = Math.max(0, Math.min(root.count - 1, from + delta));
        root.ensureVisible();
    }

    function incrementCurrentIndex(): void {
        root.moveBy(root.columns);
    }

    function decrementCurrentIndex(): void {
        root.moveBy(-root.columns);
    }

    function moveLeft(): void {
        root.moveBy(-1);
    }

    function moveRight(): void {
        root.moveBy(1);
    }

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

    function ensureVisible(): void {
        if (root.currentIndex < 0)
            return;
        if (root.currentIndex < root.favourites.length) {
            grid.contentY = grid.originY;
            return;
        }
        grid.positionViewAtIndex(root.currentIndex - root.favourites.length, GridView.Contain);
    }

    function resetView(): void {
        root.currentIndex = 0;
        grid.cancelFlick();
        grid.contentY = grid.originY;
    }

    implicitWidth: Math.min(Tokens.sizes.launcher.browseWidth, root.maxWidth)
    implicitHeight: Tokens.sizes.launcher.browseMinHeight

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

    GridView {
        id: grid

        anchors.fill: parent
        anchors.margins: root.padding
        clip: true
        boundsBehavior: Flickable.StopAtBounds

        contentY: originY

        onOriginYChanged: {
            if (root.currentIndex === 0 && !grid.moving && !grid.dragging)
                grid.contentY = grid.originY;
        }

        cellWidth: root.cellWidth
        cellHeight: root.cellHeight
        currentIndex: -1
        highlightFollowsCurrentItem: false
        cacheBuffer: root.cellHeight * 4

        model: ScriptModel {
            values: root.others
        }

        header: Item {
            id: headerItem

            readonly property alias favouriteRepeater: favRepeater

            width: grid.width
            height: headerColumn.implicitHeight

            Column {
                id: headerColumn

                width: parent.width
                spacing: root.gridSpacing

                Flow {
                    width: grid.width
                    spacing: root.columns > 1 ? root.cellWidth - root.tileWidth : root.gridSpacing

                    Repeater {
                        id: favRepeater

                        model: root.favourites

                        delegate: Tile {
                            flatIndex: index
                        }
                    }
                }

                Rectangle {
                    width: parent.width - Tokens.padding.medium * 2
                    height: 1
                    color: Colours.palette.m3outline
                    opacity: 0.5
                    visible: root.showSeparator

                    anchors.horizontalCenter: parent.horizontalCenter
                }
            }
        }

        delegate: Tile {
            flatIndex: root.favourites.length + index
        }

        StyledScrollBar.vertical: StyledScrollBar {
            flickable: grid
        }
    }

    component Tile: Item {
        id: tileWrapper

        required property var modelData
        required property int index

        property int flatIndex: index
        property alias tile: innerTile

        implicitWidth: innerTile.implicitWidth
        implicitHeight: innerTile.implicitHeight

        AppTile {
            id: innerTile

            modelData: tileWrapper.modelData
            index: tileWrapper.flatIndex
            selected: root.currentIndex === tileWrapper.flatIndex
            browser: root
        }
    }
}
