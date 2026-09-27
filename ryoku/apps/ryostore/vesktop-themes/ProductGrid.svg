import QtQuick
import Ryoku.Ui.Singletons
import "lib/store.js" as StoreLogic

// The browse surface: a scannable, scrolling grid of product tiles. Selection
// is driven from the app (arrow keys move it in two dimensions, hover previews,
// a tap opens the detail), so many products are visible at once instead of one
// hero and a strip.
FocusScope {
    id: grid

    property var items: []
    property string selectedKey: ""
    property bool reducedMotion: false

    signal previewRequested(var item)
    signal selectionRequested(var item)
    signal activated(var item)

    activeFocusOnTab: true

    // a target tile width near 340px keeps cards large enough to read art yet
    // yields more columns on a wide panel; at least two so a tile is never full
    // width.
    readonly property int columns: Math.max(2, Math.round(view.width / 340))
    readonly property real cellW: view.width / grid.columns
    readonly property real cellH: Math.round(grid.cellW * 0.66)
    property alias contentY: view.contentY
    property bool restoring: false

    function positionFor(key) {
        for (var i = 0; i < items.length; i++)
            if (StoreLogic.itemKey(items[i]) === key)
                return i;
        return -1;
    }

    function select(index) {
        if (items.length === 0)
            return;
        var bounded = Math.max(0, Math.min(items.length - 1, index));
        selectionRequested(items[bounded]);
    }

    function moveBy(delta) {
        select(Math.max(0, positionFor(selectedKey)) + delta);
    }

    function scrollToSelected() {
        var index = positionFor(selectedKey);
        if (index >= 0)
            view.positionViewAtIndex(index, GridView.Contain);
    }

    // set the scroll position without the smooth Behavior, so a context restore
    // lands exactly where it was saved instead of animating toward it.
    function restoreOffset(offset) {
        var maximum = Math.max(0, view.contentHeight - view.height);
        grid.restoring = true;
        view.contentY = Math.max(0, Math.min(maximum, Number(offset) || 0));
        grid.restoring = false;
    }

    // the selected tile's rect in grid-local coordinates, so the detail overlay
    // can grow from the exact card the user opened.
    function cellRectFor(key) {
        var index = positionFor(key);
        if (index < 0)
            return Qt.rect(0, 0, 0, 0);
        var row = Math.floor(index / grid.columns);
        var col = index % grid.columns;
        return Qt.rect(col * grid.cellW, row * grid.cellH - view.contentY,
                       grid.cellW, grid.cellH);
    }

    onSelectedKeyChanged: Qt.callLater(grid.scrollToSelected)

    Keys.onPressed: event => {
        var cols = grid.columns;
        if (event.key === Qt.Key_Left)
            grid.moveBy(-1);
        else if (event.key === Qt.Key_Right)
            grid.moveBy(1);
        else if (event.key === Qt.Key_Up)
            grid.moveBy(-cols);
        else if (event.key === Qt.Key_Down)
            grid.moveBy(cols);
        else if (event.key === Qt.Key_Home)
            grid.select(0);
        else if (event.key === Qt.Key_End)
            grid.select(grid.items.length - 1);
        else if (event.key === Qt.Key_PageUp)
            grid.moveBy(-cols * 3);
        else if (event.key === Qt.Key_PageDown)
            grid.moveBy(cols * 3);
        else
            return;
        event.accepted = true;
    }

    // Hover-intent: commit a hero preview only after the pointer rests on a card
    // briefly, so sweeping across the grid to reach a target never thrashes the
    // hero and its remote art (no mainstream store drives a distant hero from raw
    // hover; a short dwell is the standard guard). Deliberately not motion-scaled:
    // this is intent detection, not an animation, so it holds under reduce-motion.
    property var pendingPreview: null
    Timer {
        id: dwell
        interval: 150
        onTriggered: grid.previewRequested(grid.pendingPreview)
    }
    onItemsChanged: dwell.stop()

    GridView {
        id: view
        objectName: "ryostore-grid-view"
        anchors.fill: parent
        clip: true
        model: grid.items
        cellWidth: grid.cellW
        cellHeight: grid.cellH
        cacheBuffer: Math.ceil(grid.cellH * 3)
        keyNavigationEnabled: false
        boundsBehavior: Flickable.StopAtBounds

        Behavior on contentY {
            enabled: !grid.reducedMotion && !grid.restoring && !view.dragging && !view.flicking
            NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease }
        }
        // Clear the preview only when the pointer leaves the grid, not on each
        // card exit: holding the last preview across the inter-card gap is what
        // keeps the hero from flashing back to the selection mid-sweep.
        HoverHandler {
            onHoveredChanged: if (!hovered) { dwell.stop(); grid.previewRequested(null); }
        }

        delegate: ProductCard {
            required property var modelData
            width: grid.cellW
            height: grid.cellH
            item: modelData
            selected: StoreLogic.itemKey(modelData) === grid.selectedKey
            focusVisible: grid.activeFocus
                    && StoreLogic.itemKey(modelData) === grid.selectedKey
            reducedMotion: grid.reducedMotion
            onHoverChanged: hovered => { if (hovered) { grid.pendingPreview = modelData; dwell.restart(); } }
            onActivated: grid.activated(modelData)
        }
    }
}
