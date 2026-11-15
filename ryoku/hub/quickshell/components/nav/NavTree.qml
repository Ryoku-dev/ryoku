import QtQuick
import Lithic
import "../../Singletons"
import "../../registry/registry.js" as Registry

// every category as a folder and every page as a leaf, the way a project tree shows a
// codebase. Folders open and shut as the reader likes; the open page's folder always opens
// when the page does, and one mark follows the open page wherever it moves in the tree
Flickable {
    id: root

    required property var hub
    // category id -> open; the open page's category is opened for it
    property var open: ({})
    // the first build stands still; from then on folders unfold and the mark travels
    property bool animate: false

    // the page line resting under the pointer, which the sidebar peeks into after a moment
    property Item peekRow: null
    property var peekPage: null
    property bool peekShown: false
    function hover(row, page, on) {
        if (on) {
            root.peekRow = row;
            root.peekPage = page;
            if (!root.peekShown)
                peekWait.restart();
        } else if (root.peekRow === row) {
            peekLeave.restart();
        }
    }
    Timer {
        id: peekWait
        interval: 480
        onTriggered: root.peekShown = !!root.peekRow && root.peekRow.hovered
    }
    // a short grace, so moving from one line to the next keeps the peek and slides it along
    Timer {
        id: peekLeave
        interval: 90
        onTriggered: {
            if (root.peekRow && root.peekRow.hovered)
                return;
            peekWait.stop();
            root.peekShown = false;
            root.peekRow = null;
        }
    }
    onMovementStarted: root.peekShown = false

    readonly property var folderCategories: Registry.categories.filter(c => c.id !== "updates")

    function pagesOf(categoryId) {
        return Registry.pagesOf(categoryId, root.hub.gate);
    }
    function isOpen(id) {
        return root.open[id] === true;
    }
    function setExpanded(id, value) {
        if (root.isOpen(id) === value)
            return;
        var next = Object.assign({}, root.open);
        next[id] = value;
        root.open = next;
    }
    function toggle(id) {
        root.setExpanded(id, !root.isOpen(id));
    }
    // Up and Down walk the lines on show, in reading order
    function step(from, direction) {
        var next = from.nextItemInFocusChain(direction > 0);
        for (var p = next; p; p = p.parent)
            if (p === root.contentItem) {
                next.forceActiveFocus(Qt.TabFocusReason);
                root.reveal(next);
                return;
            }
    }
    // the first line, for the keys coming down from the search
    function focusFirst() {
        var first = folders.count > 0 ? folders.itemAt(0) : null;
        if (first)
            first.header.forceActiveFocus(Qt.TabFocusReason);
    }
    function reveal(item) {
        var y = item.mapToItem(root.contentItem, 0, 0).y;
        if (y < root.contentY)
            root.contentY = Math.max(0, y - 8);
        else if (y + item.height > root.contentY + root.height)
            root.contentY = Math.min(root.contentHeight - root.height, y + item.height - root.height + 8);
    }

    // the folder of the open page, and the page's own line while that folder is open
    readonly property Item activeFolder: {
        root.hub.pageId;
        for (var i = 0; i < folders.count; i++) {
            var f = folders.itemAt(i);
            if (f && f.holdsPage)
                return f;
        }
        return null;
    }
    readonly property Item activeLeaf: {
        root.hub.pageId;
        return root.activeFolder ? root.activeFolder.rowFor(root.hub.pageId) : null;
    }
    readonly property bool onLeaf: !!root.activeLeaf && !!root.activeFolder && root.activeFolder.expanded

    Connections {
        target: root.hub
        function onPageIdChanged() {
            var p = Registry.page(root.hub.pageId);
            if (p)
                root.setExpanded(p.category, true);
            mark.travel();
            keepShown.restart();
        }
    }
    // the open page's line comes into view once its folder has unfolded
    Timer {
        id: keepShown
        interval: Tone.unfold + 20
        onTriggered: {
            if (root.onLeaf)
                root.reveal(root.activeLeaf);
        }
    }

    Component.onCompleted: {
        var p = Registry.page(root.hub.pageId);
        if (p)
            root.setExpanded(p.category, true);
        Qt.callLater(() => root.animate = true);
    }

    contentWidth: width
    contentHeight: column.implicitHeight + 8
    boundsBehavior: Flickable.StopAtBounds
    clip: true

    // the open page's mark: a wash under its line, or under its shut folder
    Rectangle {
        id: mark
        property bool moving: false
        function travel() {
            mark.moving = root.animate;
            settle.restart();
        }
        visible: !!root.activeFolder
        x: root.onLeaf ? 26 : 4
        y: root.onLeaf ? root.activeLeaf.treeY : root.activeFolder ? root.activeFolder.treeY : 0
        width: root.width - x - 4
        height: root.onLeaf ? root.activeLeaf.lineHeight : root.activeFolder ? root.activeFolder.header.height : 0
        radius: Tone.radiusSmall
        color: Tone.accentSoft
        Behavior on y {
            enabled: mark.moving
            NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic }
        }
        Behavior on x {
            enabled: root.animate
            NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic }
        }
        Behavior on height {
            enabled: mark.moving
            NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic }
        }
        Timer {
            id: settle
            interval: Tone.move + 40
            onTriggered: mark.moving = false
        }
    }

    Column {
        id: column
        width: root.width

        Repeater {
            id: folders
            model: root.folderCategories
            delegate: NavFolder {
                required property var modelData
                hub: root.hub
                category: modelData
                tree: root
                expanded: root.isOpen(modelData.id)
                onExpandedChanged: {
                    if (holdsPage)
                        mark.travel();
                }
            }
        }
    }
}
