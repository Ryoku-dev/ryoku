import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons

// widgets in named lanes plus the ones left off; drag a widget or open its menu to move it
Column {
    id: root

    // [{ id, title, moveText, addText, items: [widget ids] }]
    property var lanes: []
    property var spare: []
    property string spareTitle: ""
    property string spareEmpty: ""
    property bool sideBySide: false
    property var labelFor: function (id) { return id; }
    property Item overlayParent: null
    // the lane whose widget is being dragged, so it draws above its neighbours
    property string dragLane: ""
    property real tallest: 0
    // the widget whose chip holds keyboard focus: a move from its menu, Undo or any redraw makes new
    // chips, and the one for this widget takes focus back, marked when the keys had it
    property string focusId: ""
    property bool focusMarked: false
    function noteFocus(chip) {
        root.focusId = chip.widgetId;
        root.focusMarked = chip.visualFocus;
    }
    // focus that left the chips for another control lets go; focus that fell out with a redrawn chip does not
    function checkFocus() {
        var now = root.Window.activeFocusItem;
        for (var p = now; p; p = p.parent)
            if (p === root)
                return;
        if (now && now !== root.Window.contentItem && !root.holds(now))
            root.focusId = "";
    }
    function holds(item) {
        for (var p = root.parent; p; p = p.parent)
            if (p === item)
                return true;
        return false;
    }

    // toIndex counts positions before the widget leaves its lane; toLane "" takes it off
    signal moved(string widgetId, string fromLane, int fromIndex, string toLane, int toIndex)

    readonly property bool across: root.sideBySide && root.width >= 600
    readonly property string dragKey: "ryoku-bar-widget"

    spacing: LithicTokens.space.s2

    function measure() {
        var h = 0;
        for (var i = 0; i < laneRepeater.count; i++) {
            var lane = laneRepeater.itemAt(i);
            if (lane)
                h = Math.max(h, lane.naturalHeight);
        }
        root.tallest = h;
    }

    function laneTitle(id) {
        for (var i = 0; i < root.lanes.length; i++)
            if (root.lanes[i].id === id)
                return root.lanes[i].title;
        return root.spareTitle;
    }

    function openMenu(chip) {
        var actions = [];
        var from = chip.laneId;
        var count = from === "" ? root.spare.length : (root.lanes.find(l => l.id === from) || { items: [] }).items.length;
        for (var i = 0; i < root.lanes.length; i++) {
            var lane = root.lanes[i];
            if (lane.id === from)
                continue;
            actions.push({ label: from === "" ? lane.addText : lane.moveText, glyph: from === "" ? "add" : "arrow_forward", lane: lane.id, index: lane.items.length });
        }
        if (from !== "" && chip.position > 0)
            actions.push({ label: I18n.tr("Move earlier"), glyph: "arrow_upward", lane: from, index: chip.position - 1 });
        if (from !== "" && chip.position < count - 1)
            actions.push({ label: I18n.tr("Move later"), glyph: "arrow_downward", lane: from, index: chip.position + 2 });
        if (from !== "") {
            actions.push({ divider: true });
            actions.push({ label: I18n.tr("Take off the bar"), glyph: "remove", lane: "", index: root.spare.length });
        }
        menuPop.widgetId = chip.widgetId;
        menuPop.keys = chip.visualFocus;
        menuPop.fromLane = from;
        menuPop.fromIndex = chip.position;
        menuPop.actions = actions;
        menu.focusedIndex = -1;
        menuPop.anchorItem = chip;
        menuPop.opener = chip;
        menuPop.open();
    }

    Grid {
        id: grid
        width: parent.width
        columns: root.across ? Math.max(1, root.lanes.length) : 1
        columnSpacing: LithicTokens.space.s2
        rowSpacing: LithicTokens.space.s2

        Repeater {
            id: laneRepeater
            model: root.lanes
            delegate: Lane {
                required property var modelData
                laneId: modelData.id
                title: modelData.title
                items: modelData.items
                width: root.across ? (grid.width - grid.columnSpacing * (grid.columns - 1)) / grid.columns : grid.width
                height: root.across ? Math.max(naturalHeight, root.tallest) : naturalHeight
                onNaturalHeightChanged: Qt.callLater(root.measure)
            }
        }
    }

    Lane {
        width: parent.width
        height: naturalHeight
        laneId: ""
        title: root.spareTitle
        items: root.spare
        emptyText: root.spareEmpty
    }

    component Lane: Item {
        id: lane

        property string laneId: ""
        property string title: ""
        property var items: []
        property string emptyText: I18n.tr("Drop a widget here")
        property int dropIndex: -1
        readonly property real naturalHeight: flow.y + Math.max(flow.implicitHeight, LithicTokens.size.plaqueHeightSmall) + LithicTokens.space.s3

        z: root.dragLane === lane.laneId ? 2 : 0

        function insertionIndex(x, y) {
            var lx = x - flow.x, ly = y - flow.y;
            for (var i = 0; i < chips.count; i++) {
                var chip = chips.itemAt(i);
                if (chip && (ly < chip.y || (ly <= chip.y + chip.height && lx < chip.x + chip.width / 2)))
                    return i;
            }
            return lane.items.length;
        }

        CutRect {
            anchors.fill: parent
            cut: LithicTokens.cut.small
            fill: drop.containsDrag ? Lithic.mix(Lithic.slab, Lithic.fg, LithicTokens.alpha.hoverPlane) : Lithic.slab
            strokeColor: Lithic.vein
            strokeWidth: LithicTokens.size.hairline
        }

        Label {
            id: heading
            x: LithicTokens.space.s3
            y: LithicTokens.space.s2
            width: parent.width - LithicTokens.space.s3 * 2
            text: lane.title
            typeRole: "caption"
            inkRole: "fg2"
        }

        Flow {
            id: flow
            x: LithicTokens.space.s3
            y: heading.y + heading.height + LithicTokens.space.s2
            width: parent.width - LithicTokens.space.s3 * 2
            spacing: LithicTokens.space.s1

            Repeater {
                id: chips
                model: lane.items
                delegate: Chip {
                    required property string modelData
                    required property int index
                    widgetId: modelData
                    laneId: lane.laneId
                    position: index
                }
            }
        }

        Label {
            visible: lane.items.length === 0
            x: flow.x
            y: flow.y + (LithicTokens.size.plaqueHeightSmall - height) / 2
            text: lane.emptyText
            typeRole: "detail"
            inkRole: "fg3"
        }

        Rectangle {
            readonly property Item at: lane.dropIndex >= 0 && chips.count > 0 ? chips.itemAt(Math.min(lane.dropIndex, chips.count - 1)) : null
            visible: drop.containsDrag && lane.dropIndex >= 0 && at !== null
            x: at ? flow.x + (lane.dropIndex < chips.count ? at.x - LithicTokens.space.s1 / 2 - 1 : at.x + at.width + LithicTokens.space.s1 / 2 - 1) : 0
            y: at ? flow.y + at.y : 0
            width: 2
            height: LithicTokens.size.plaqueHeightSmall
            color: Lithic.fg
        }

        DropArea {
            id: drop
            anchors.fill: parent
            keys: [root.dragKey]
            onEntered: drag => lane.dropIndex = lane.insertionIndex(drag.x, drag.y)
            onPositionChanged: drag => lane.dropIndex = lane.insertionIndex(drag.x, drag.y)
            onExited: lane.dropIndex = -1
            onDropped: drop => {
                var source = drop.source;
                var at = lane.insertionIndex(drop.x, drop.y);
                lane.dropIndex = -1;
                if (source && source.widgetId !== undefined)
                    Qt.callLater(root.moved, source.widgetId, source.laneId, source.position, lane.laneId, at);
            }
        }
    }

    component Chip: T.AbstractButton {
        id: chip

        property string widgetId: ""
        property string laneId: ""
        property int position: 0
        readonly property bool held: dragger.active

        implicitWidth: Math.min(200, chipLabel.implicitWidth + LithicTokens.space.s3 * 2)
        implicitHeight: LithicTokens.size.plaqueHeightSmall
        hoverEnabled: true
        focusPolicy: Qt.StrongFocus
        z: chip.held ? 10 : 0
        Accessible.role: Accessible.Button
        Accessible.name: root.labelFor(chip.widgetId)
        Accessible.description: root.laneTitle(chip.laneId)

        onActiveFocusChanged: {
            if (chip.activeFocus)
                root.noteFocus(chip);
            else
                Qt.callLater(root.checkFocus);
        }
        // the chip joins the scene after it completes, so focus waits a turn
        Component.onCompleted: {
            if (root.focusId !== chip.widgetId)
                return;
            var reason = root.focusMarked ? Qt.TabFocusReason : Qt.OtherFocusReason;
            Qt.callLater(() => chip.forceActiveFocus(reason));
        }

        onClicked: root.openMenu(chip)
        Keys.onReturnPressed: root.openMenu(chip)
        Keys.onEnterPressed: root.openMenu(chip)
        onHeldChanged: root.dragLane = chip.held ? chip.laneId : ""

        background: null
        contentItem: null

        CutRect {
            id: face
            width: chip.width
            height: chip.height
            cut: LithicTokens.cut.token
            fill: chip.held ? Lithic.sel
                : chip.down ? Lithic.pressRow(Lithic.field)
                : chip.hovered ? Lithic.hoverFill(Lithic.field)
                : Lithic.field
            Drag.active: chip.held
            Drag.source: chip
            Drag.keys: [root.dragKey]
            Drag.hotSpot.x: width / 2
            Drag.hotSpot.y: height / 2

            Label {
                id: chipLabel
                anchors.centerIn: parent
                width: Math.min(implicitWidth, chip.width - LithicTokens.space.s3 * 2)
                text: root.labelFor(chip.widgetId)
                typeRole: "label"
                ink: chip.held ? Lithic.onSel : Lithic.fg
                elide: Text.ElideRight
            }

            FocusRing {
                x: -outset
                y: -outset
                width: face.width + outset * 2
                height: face.height + outset * 2
                keyboardFocusVisible: chip.visualFocus
                cut: face.cut
            }
        }

        DragHandler {
            id: dragger
            target: face
            onActiveChanged: {
                if (!active) {
                    face.Drag.drop();
                    face.x = 0;
                    face.y = 0;
                }
            }
        }
    }

    Popover {
        id: menuPop
        parent: root.overlayParent ? root.overlayParent : root
        property string widgetId: ""
        property string fromLane: ""
        property int fromIndex: -1
        property var actions: []
        // opened from the keys, so the moved widget's chip takes focus with the mark
        property bool keys: false

        Menu {
            id: menu
            focus: true
            items: menuPop.actions.map(a => a.divider ? { divider: true } : { label: a.label, glyph: a.glyph })
            onTriggered: index => {
                var a = menuPop.actions[index];
                menuPop.close();
                if (!a || a.divider)
                    return;
                // the chip that opened the menu goes with the move; its replacement in the new place takes focus
                root.focusId = menuPop.widgetId;
                root.focusMarked = menuPop.keys;
                Qt.callLater(root.moved, menuPop.widgetId, menuPop.fromLane, menuPop.fromIndex, a.lane, a.index);
            }
        }
    }
}
