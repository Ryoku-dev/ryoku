pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services
import caelestia.modules.launcher.items
import caelestia.modules.launcher.services

Item {
    id: root

    required property DrawerVisibilities visibilities
    required property real maxWidth

    property string currentCategory: "favorites"
    property bool sidebarFocused: false
    property var visibleCategories: []

    readonly property var currentItem: grid.currentItem
    readonly property int count: grid.count

    readonly property int padding: Tokens.padding.large
    readonly property int sidebarWidth: Tokens.sizes.launcher.browseSidebarWidth
    readonly property int tileCellWidth: Tokens.sizes.launcher.browseTileWidth + Tokens.spacing.medium
    readonly property int tileCellHeight: Tokens.sizes.launcher.browseTileHeight + Tokens.spacing.medium

    readonly property int columns: Math.max(1, Math.floor((root.implicitWidth - root.sidebarWidth - root.padding * 2 - Tokens.spacing.medium) / root.tileCellWidth))
    readonly property int rows: Math.ceil(root.count / root.columns)

    function sameItems(a: var, b: var): bool {
        if (a === b)
            return true;
        if (a.length !== b.length)
            return false;
        for (let i = 0; i < a.length; i++)
            if (a[i] !== b[i])
                return false;
        return true;
    }

    function refresh(): void {
        const all = Apps.allApps();
        const nextCategories = Categories.visibleCategories(all);
        const nextApps = Categories.appsFor(root.currentCategory, all);
        if (!sameItems(root.visibleCategories, nextCategories))
            root.visibleCategories = nextCategories;
        if (!sameItems(gridModel.values, nextApps))
            gridModel.values = nextApps;
    }

    function launch(app): void {
        Apps.launch(app);
        root.visibilities.launcher = false;
    }

    function selectTile(index: int): void {
        grid.currentIndex = index;
    }

    function selectCategory(id: string): void {
        root.currentCategory = id;
        root.sidebarFocused = false;
        const idx = root.visibleCategories.findIndex(d => d.id === id);
        sidebar.currentIndex = Math.max(0, idx);
        root.refresh();
        grid.currentIndex = grid.count > 0 ? 0 : -1;
    }

    function incrementCurrentIndex(): void {
        if (root.sidebarFocused)
            sidebar.incrementCurrentIndex();
        else
            grid.moveCurrentIndexDown();
    }

    function decrementCurrentIndex(): void {
        if (root.sidebarFocused)
            sidebar.decrementCurrentIndex();
        else
            grid.moveCurrentIndexUp();
    }

    function moveLeft(): void {
        if (root.sidebarFocused)
            return;
        const before = grid.currentIndex;
        grid.moveCurrentIndexLeft();
        if (grid.currentIndex === before)
            root.sidebarFocused = true;
    }

    function moveRight(): void {
        if (root.sidebarFocused)
            root.selectCategory(sidebar.currentItem?.modelData?.id ?? "favorites");
        else
            grid.moveCurrentIndexRight();
    }

    function toggleFocus(): void {
        if (root.sidebarFocused)
            root.selectCategory(sidebar.currentItem?.modelData?.id ?? "favorites");
        else
            root.sidebarFocused = true;
    }

    function activateCurrent(): void {
        if (root.sidebarFocused)
            root.selectCategory(sidebar.currentItem?.modelData?.id ?? "favorites");
        else if (grid.currentItem?.modelData)
            root.launch(grid.currentItem.modelData);
    }

    function openContextMenu(app: DesktopEntry, targetItem: Item): void {
        contextMenu.openFor(app, targetItem);
    }

    implicitWidth: Math.min(Tokens.sizes.launcher.browseWidth, root.maxWidth)
    implicitHeight: root.padding * 2 + (root.count === 0 ? root.tileCellHeight * 2 : root.rows * root.tileCellHeight)

    Component.onCompleted: {
        root.refresh();
        grid.currentIndex = grid.count > 0 ? 0 : -1;
        sidebar.currentIndex = 0;
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

    AppContextMenu {
        id: contextMenu

        attachTo: root
        visibilities: root.visibilities
    }

    RowLayout {
        anchors.fill: parent
        anchors.margins: root.padding
        spacing: Tokens.spacing.medium

        Item {
            Layout.preferredWidth: root.sidebarWidth
            Layout.fillHeight: true

            ListView {
                id: sidebar

                anchors.fill: parent

                model: root.visibleCategories
                spacing: Tokens.spacing.extraSmall
                clip: true

                currentIndex: 0
                highlightFollowsCurrentItem: false
                highlight: Item {}

                delegate: CategoryButton {
                    width: parent?.width ?? 0
                    selected: sidebar.currentIndex === index
                    onClicked: root.selectCategory(modelData.id)
                }
            }
        }

        Item {
            Layout.fillWidth: true
            Layout.fillHeight: true

            GridView {
                id: grid

                anchors.fill: parent

                model: ScriptModel {
                    id: gridModel

                    values: []
                    onValuesChanged: grid.currentIndex = grid.count > 0 ? 0 : -1
                }

                cellWidth: root.tileCellWidth
                cellHeight: root.tileCellHeight
                clip: true

                currentIndex: -1
                highlightFollowsCurrentItem: false
                cacheBuffer: root.tileCellHeight * 4

                StyledScrollBar.vertical: StyledScrollBar {
                    flickable: grid
                }

                delegate: AppTile {
                    browser: root
                    selected: grid.currentIndex === index
                }
            }

            Column {
                anchors.centerIn: parent
                spacing: Tokens.spacing.medium
                visible: grid.count === 0

                MaterialIcon {
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: "apps"
                    color: Colours.palette.m3outline
                    fontStyle: Tokens.font.icon.extraLarge
                }

                StyledText {
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: qsTr("No apps in this category")
                    color: Colours.palette.m3outline
                    font: Tokens.font.body.builders.large.weight(Font.Medium).build()
                }
            }
        }
    }
}
