import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// a category in the navigation tree: its folder line, then its pages hung on a guide. The
// pages unfold with the folder, each a beat after the one above it
Column {
    id: root

    required property var hub
    required property var category
    required property var tree
    property bool expanded: false
    // where this folder starts in the tree, so the selection mark can follow a page into it
    readonly property real treeY: root.y
    readonly property var pages: {
        Settings.revision;
        return root.tree.pagesOf(root.category.id);
    }
    readonly property bool holdsPage: root.pages.some(p => p.id === root.hub.pageId)
    readonly property Item header: head

    function rowFor(pageId) {
        for (var i = 0; i < leaves.count; i++) {
            var leaf = leaves.itemAt(i);
            if (leaf && leaf.pageId === pageId)
                return leaf;
        }
        return null;
    }

    width: parent ? parent.width : 0

    onExpandedChanged: {
        if (!root.expanded)
            return;
        for (var i = 0; i < leaves.count; i++) {
            var leaf = leaves.itemAt(i);
            if (leaf)
                leaf.enter(i);
        }
    }

    NavRow {
        id: head
        width: root.width
        folder: true
        glyph: root.category.glyph
        text: I18n.tr(root.category.label)
        expanded: root.expanded
        holdsSelection: root.holdsPage && !root.expanded
        trailing: String(root.pages.length)
        onClicked: root.tree.toggle(root.category.id)
        onHoveredChanged: root.tree.hover(head, { id: root.category.id, glyph: root.category.glyph, title: root.category.label, folder: true }, head.hovered)
        onFolded: open => root.tree.setExpanded(root.category.id, open)
        onStepped: direction => root.tree.step(head, direction)
    }

    Item {
        id: box
        width: root.width
        height: root.expanded ? leafColumn.implicitHeight + 4 : 0
        clip: true
        visible: height > 0
        Behavior on height {
            enabled: root.tree.animate
            NumberAnimation { duration: Tone.unfold; easing.type: Easing.OutCubic }
        }

        // the guide the pages hang on, from the folder's chevron down to the last page's branch
        Rectangle {
            anchors.left: parent.left
            anchors.leftMargin: 16
            y: 0
            width: 1
            height: Math.max(0, leafColumn.lastBranchY)
            // the guide of the open page's folder is drawn brighter, as an editor lights the
            // indent the cursor stands in
            color: root.holdsPage ? Tone.guideLit : Tone.guide
            Behavior on color { ColorAnimation { duration: Tone.move } }
        }

        Column {
            id: leafColumn
            width: parent.width
            readonly property real lastBranchY: {
                var last = leaves.count > 0 ? leaves.itemAt(leaves.count - 1) : null;
                return last ? last.y + last.lineHeight / 2 : 0;
            }

            Repeater {
                id: leaves
                model: root.pages
                delegate: Column {
                    id: entry
                    required property var modelData
                    required property int index
                    readonly property string pageId: modelData.id
                    readonly property bool open: modelData.id === root.hub.pageId
                    // where the page's own line stands in the tree, for the selection mark
                    readonly property real treeY: root.y + box.y + entry.y
                    readonly property real lineHeight: leaf.height
                    // set by enter(): the line slides in from the guide as the folder opens
                    property real shift: 0

                    function enter(order) {
                        if (!root.tree.animate || Tone.stagger === 0)
                            return;
                        arrive.stop();
                        entry.opacity = 0;
                        entry.shift = -8;
                        wait.duration = order * Tone.stagger;
                        arrive.start();
                    }

                    width: root.width
                    transform: Translate { x: entry.shift }

                    NavRow {
                        id: leaf
                        width: entry.width
                        depth: 1
                        glyph: entry.modelData.glyph
                        text: I18n.tr(entry.modelData.title)
                        selected: entry.open
                        trailingGlyph: entry.modelData.handoff ? "open_in_new" : ""
                        onClicked: root.hub.navigate(entry.modelData.id, "")
                        onStepped: direction => root.tree.step(leaf, direction)
                        onHoveredChanged: root.tree.hover(leaf, entry.modelData, leaf.hovered)
                        onFolded: open => {
                            if (!open) {
                                root.tree.setExpanded(root.category.id, false);
                                head.forceActiveFocus(Qt.TabFocusReason);
                            }
                        }
                    }

                    NavOutline {
                        hub: root.hub
                        tree: root.tree
                        shown: entry.open
                        animate: root.tree.animate
                    }

                    SequentialAnimation {
                        id: arrive
                        PauseAnimation { id: wait; duration: 0 }
                        ParallelAnimation {
                            NumberAnimation { target: entry; property: "opacity"; to: 1; duration: Tone.move; easing.type: Easing.OutCubic }
                            NumberAnimation { target: entry; property: "shift"; to: 0; duration: Tone.move; easing.type: Easing.OutCubic }
                        }
                    }
                }
            }
        }
    }
}
