import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    property var model: []
    property alias currentIndex: grid.currentIndex
    property Component delegate: defaultTile
    property string accessibleName: Lithic.tr("Designs")
    property string previewState: ""
    readonly property alias gridView: grid
    // s2 apart, so a selected tile's focus ring never meets its neighbour
    readonly property real gap: LithicTokens.space.s2
    readonly property int columns: Math.max(1, Math.floor((availableWidth + gap)
                                                          / (LithicTokens.size.catalogueTileWidthMin + gap)))
    readonly property real tileWidth: Math.min(LithicTokens.size.catalogueTileWidthMax,
                                               (availableWidth - (columns - 1) * gap) / columns)
    // DesignTile's height for this width: inset preview, then the two text lines
    readonly property real tileHeight: LithicTokens.space.s1
                                       + Math.round((tileWidth - LithicTokens.space.s1 * 2)
                                                    / LithicTokens.size.designPreviewRatio)
                                       + LithicTokens.space.s2 * 2 + Lithic.type.row.lineHeight
                                       + LithicTokens.size.plaqueHeightSmall
    readonly property int _count: Array.isArray(model) ? model.length : grid.count
    readonly property int _rows: Math.ceil(_count / columns)

    signal activated(int index)
    signal actionTriggered(int index)

    function select(index) {
        if (index < 0 || index >= grid.count)
            return;
        grid.currentIndex = index;
        grid.positionViewAtIndex(index, GridView.Contain);
    }

    function activate(index) {
        select(index);
        if (index >= 0 && index < grid.count)
            activated(index);
    }

    focusPolicy: Qt.StrongFocus
    implicitWidth: LithicTokens.size.catalogueTileWidthMax * 3 + gap * 2
    implicitHeight: Math.max(0, _rows * (tileHeight + gap) - gap)

    Accessible.role: Accessible.List
    Accessible.name: accessibleName

    Keys.onPressed: function(event) {
        const index = grid.currentIndex;
        let next = -1;
        if (event.key === Qt.Key_Left)
            next = Math.max(0, index - 1);
        else if (event.key === Qt.Key_Right)
            next = Math.min(grid.count - 1, index + 1);
        else if (event.key === Qt.Key_Up)
            next = index - root.columns >= 0 ? index - root.columns : index;
        else if (event.key === Qt.Key_Down)
            next = index + root.columns < grid.count ? index + root.columns : index;
        else if (event.key === Qt.Key_Home)
            next = 0;
        else if (event.key === Qt.Key_End)
            next = grid.count - 1;
        else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || event.key === Qt.Key_Space)
            return activateCurrent(event);
        else
            return;
        root.select(Math.max(0, next));
        event.accepted = true;
    }

    function activateCurrent(event) {
        if (grid.currentIndex >= 0)
            activated(grid.currentIndex);
        event.accepted = true;
    }

    Component {
        id: defaultTile

        DesignTile {
            required property var modelData
            required property int index

            width: root.tileWidth
            height: root.tileHeight
            focusPolicy: Qt.NoFocus
            title: modelData.title || ""
            status: modelData.status || ""
            resource: modelData.resource || ""
            source: modelData.source || ""
            current: !!modelData.current
            unavailable: modelData.unavailable || ""
            busy: !!modelData.busy
            actionText: modelData.actionText || ""
            actionKind: modelData.actionKind || "line"
            actionGlyph: modelData.actionGlyph || ""
            selected: index === root.currentIndex
            focusVisible: selected && (root.visualFocus || root.previewState === "focus")
            onClicked: {
                root.forceActiveFocus(Qt.MouseFocusReason);
                root.activate(index);
            }
            onActionTriggered: root.actionTriggered(index)
        }
    }

    // each cell carries its gap on the trailing sides, and the view reaches one ring outset
    // before the box so the first row and column keep their focus ring
    contentItem: Item {
        GridView {
            id: grid
            x: -LithicTokens.space.s1
            y: -LithicTokens.space.s1
            width: parent.width + LithicTokens.space.s1 + root.gap
            height: parent.height + LithicTokens.space.s1 + root.gap
            leftMargin: LithicTokens.space.s1
            topMargin: LithicTokens.space.s1
            clip: true
            model: root.model
            delegate: root.delegate
            cellWidth: root.tileWidth + root.gap
            cellHeight: root.tileHeight + root.gap
            boundsBehavior: Flickable.StopAtBounds
            interactive: contentHeight > height
            keyNavigationEnabled: false
            activeFocusOnTab: false
            currentIndex: 0
            highlightFollowsCurrentItem: false
        }
    }

    ScrollRail {
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        flickable: grid
    }
}
