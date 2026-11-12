pragma ComponentBehavior: Bound

import QtQuick
import "../kit"

// like android, a large tile that no longer fits its row starts the next one
Item {
    id: grid

    property var tiles: []
    property var pane: null
    readonly property alias pageCount: pager.count
    readonly property alias currentPage: pager.currentIndex

    readonly property int columns: 4
    readonly property int rowsPerPage: 3
    readonly property int gap: 8
    readonly property int tileHeight: 72
    readonly property real unit: (grid.width - (grid.columns - 1) * grid.gap) / grid.columns

    function pack(list) {
        var pages = [], page = [], row = 0, col = 0;
        for (var i = 0; i < list.length; i++) {
            var span = list[i].large ? 2 : 1;
            if (col + span > grid.columns) {
                row++;
                col = 0;
            }
            if (row >= grid.rowsPerPage) {
                pages.push(page);
                page = [];
                row = 0;
                col = 0;
            }
            page.push({ id: list[i].id, large: list[i].large, row: row, col: col });
            col += span;
        }
        if (page.length > 0)
            pages.push(page);
        return pages;
    }
    readonly property var pages: grid.pack(grid.tiles)
    readonly property int rows: {
        var most = 1;
        for (var p = 0; p < grid.pages.length; p++)
            for (var i = 0; i < grid.pages[p].length; i++)
                most = Math.max(most, grid.pages[p][i].row + 1);
        return most;
    }

    function show(index) { pager.positionViewAtIndex(index, ListView.Beginning); pager.currentIndex = index; }

    implicitHeight: grid.rows * grid.tileHeight + (grid.rows - 1) * grid.gap

    ListView {
        id: pager
        anchors.fill: parent
        orientation: ListView.Horizontal
        snapMode: ListView.SnapOneItem
        highlightRangeMode: ListView.StrictlyEnforceRange
        highlightMoveDuration: Md.still ? 0 : Md.morph
        boundsBehavior: Flickable.StopAtBounds
        clip: true
        model: grid.pages.length

        delegate: Item {
            id: pageItem
            required property int index
            width: pager.width
            height: pager.height

            Repeater {
                model: grid.pages[pageItem.index] || []
                delegate: Tile {
                    id: tileItem
                    required property var modelData
                    x: tileItem.modelData.col * (grid.unit + grid.gap)
                    y: tileItem.modelData.row * (grid.tileHeight + grid.gap)
                    width: tileItem.modelData.large ? 2 * grid.unit + grid.gap : grid.unit
                    height: grid.tileHeight
                    large: tileItem.modelData.large
                    icon: spec.icon
                    label: spec.label
                    secondary: spec.secondary
                    active: spec.active
                    available: spec.available
                    split: spec.split
                    hasDetail: spec.detail.length > 0
                    onToggled: spec.toggle()
                    onOpened: spec.open()

                    TileSpec {
                        id: spec
                        tileId: tileItem.modelData.id
                        pane: grid.pane
                    }
                }
            }
        }
    }
}
