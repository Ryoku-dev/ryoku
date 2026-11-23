pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import "../look"
import "../lib/list.js" as Geometry

// A sectioned list with one selection plate that glides between rows. Geometry
// comes from lib/list.js, so the plate, a click and the scroll all read the same
// numbers. sections: [{id, title, note, rows:[row]}] (row: see ResultRow).
Item {
    id: root

    required property real s
    property var sections: []
    property real maximumHeight: 420 * s
    property string selectedId: ""
    // a view's own row in place of ResultRow: an Item with plain properties s,
    // row and selected; the list sizes and places it
    property Component rowDelegate: null

    readonly property var layout: Geometry.build(root.sections, { scale: root.s, boxed: Look.boxed })
    readonly property int count: root.layout.rows.length
    readonly property int selectedIndex: Geometry.indexOf(root.layout.rows, root.selectedId)
    readonly property var current: root.selectedIndex >= 0 ? root.layout.rows[root.selectedIndex].row : null
    readonly property real inset: Math.round((Look.boxed ? 12 : 8) * root.s)
    readonly property real viewport: Math.min(root.maximumHeight, root.layout.height)

    signal activated(var row)
    signal menuRequested(var row)

    implicitHeight: root.viewport

    // the hand chose a row; until then the selection rides the first one
    property bool held: false
    property var scrollLayout: null
    // the slot of a row on its way out: whatever holds the slot once it has gone takes the
    // selection. the row may leave only when its owner answers, so the slot keeps a moment
    property int keepAt: -1
    property real keepUntil: 0

    function select(index, reveal) {
        if (index < 0 || index >= root.count)
            return;
        root.keepAt = -1;
        root.held = true;
        root.selectedId = String(root.layout.rows[index].row.id);
        if (reveal !== false)
            root.reveal(index);
    }

    function reveal(index) {
        const target = Geometry.reveal(root.layout.rows, index, flick.contentY, flick.height, root.layout.height, 30 * root.s);
        if (Math.abs(target - flick.contentY) < 0.5)
            return;
        glide.stop();
        glide.from = flick.contentY;
        glide.to = target;
        glide.start();
    }

    function move(delta) {
        root.select(Geometry.move(root.layout.rows, root.selectedIndex, delta));
    }

    function jumpSection(delta) {
        root.select(Geometry.jump(root.layout.rows, root.selectedIndex, delta));
    }

    function page(delta) {
        root.select(Geometry.page(root.layout.rows, root.selectedIndex, delta, flick.height));
    }

    function selectFirst() {
        const index = Geometry.first(root.layout.rows);
        root.held = false;
        root.selectedId = index >= 0 ? String(root.layout.rows[index].row.id) : "";
        glide.stop();
        flick.contentY = 0;
    }

    function keepNear(rowId) {
        root.keepAt = Geometry.indexOf(root.layout.rows, String(rowId));
        root.keepUntil = Date.now() + 2000;
    }

    function selectId(rowId) {
        root.select(Geometry.indexOf(root.layout.rows, String(rowId)), true);
    }

    function settleScroll() {
        // a list still being laid out has no viewport to reveal a row in
        if (!flick || !glide || flick.height <= 0)
            return;
        glide.stop();
        const index = Geometry.indexOf(root.layout.rows, root.selectedId);
        flick.contentY = index >= 0
            ? Geometry.reveal(root.layout.rows, index, flick.contentY, flick.height, root.layout.height, 30 * root.s)
            : 0;
    }

    function samePositions(before, after) {
        if (!before || before.height !== after.height || before.rows.length !== after.rows.length)
            return false;
        for (let i = 0; i < after.rows.length; i++) {
            const a = before.rows[i], b = after.rows[i];
            if (a.y !== b.y || a.h !== b.h || String(a.row.id) !== String(b.row.id))
                return false;
        }
        return true;
    }

    function reconcile(force) {
        const unchanged = root.samePositions(root.scrollLayout, root.layout);
        root.scrollLayout = root.layout;
        const rows = root.layout.rows;
        const selected = Geometry.indexOf(rows, root.selectedId);
        if (!force && unchanged && selected >= 0)
            return;
        if (glide)
            glide.stop();
        if (selected < 0 && root.keepAt >= 0 && Date.now() < root.keepUntil) {
            const index = Geometry.near(rows, root.keepAt);
            root.keepAt = -1;
            root.held = root.held || index !== Geometry.first(rows);
            root.selectedId = index >= 0 ? String(rows[index].row.id) : "";
        } else if (!root.held || selected < 0) {
            const index = Geometry.first(rows);
            root.held = false;
            root.selectedId = index >= 0 ? String(rows[index].row.id) : "";
        }
        Qt.callLater(root.settleScroll);
    }

    onLayoutChanged: root.reconcile(false)
    onHeightChanged: root.reconcile(true)

    NumberAnimation {
        id: glide
        target: flick
        property: "contentY"
        duration: Look.dur(Look.plateMs)
        easing.type: Easing.OutCubic
    }

    Flickable {
        id: flick
        anchors.fill: parent
        contentWidth: width
        contentHeight: root.layout.height
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        interactive: contentHeight > height + 1
        onMovementStarted: glide.stop()

        // Adwaita's boxed lists
        Repeater {
            model: root.layout.boxes.length

            delegate: Shape {
                required property int index
                readonly property var box: root.layout.boxes[index] || ({ y: 0, h: 0 })

                x: Math.round(8 * root.s)
                y: box.y
                width: flick.width - x * 2
                height: box.h
                s: root.s
                cutSize: Look.cutCard
                fill: Look.card
            }
        }

        Shape {
            id: plate

            readonly property var entry: root.selectedIndex >= 0 ? root.layout.rows[root.selectedIndex] : null
            property real restY: 0
            property real restH: Math.round(40 * root.s)
            // landing on a list that just appeared is not a glide
            property bool placed: false

            x: root.inset
            y: plate.restY
            width: flick.width - root.inset * 2
            height: plate.restH
            s: root.s
            cutSize: Look.cutRow
            fill: Look.plate
            strokeColor: Look.plateStroke
            strokeWidth: Qt.colorEqual(Look.plateStroke, "transparent") ? 0 : 1
            visible: root.count > 0
            opacity: plate.entry ? 1 : 0

            // the plate keeps its place while a list is replaced, then glides
            onEntryChanged: {
                if (!plate.entry) {
                    plate.placed = false;
                    return;
                }
                plate.restY = plate.entry.y;
                plate.restH = plate.entry.h;
                if (!plate.placed)
                    Qt.callLater(() => plate.placed = true);
            }
            Behavior on restY {
                enabled: plate.placed
                NumberAnimation { duration: Look.dur(Look.plateMs); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.arrive }
            }
            Behavior on restH {
                enabled: plate.placed
                NumberAnimation { duration: Look.dur(Look.plateMs); easing.type: Easing.OutCubic }
            }
            Behavior on opacity {
                NumberAnimation { duration: Look.dur(Look.stateMs) }
            }
        }

        Repeater {
            model: root.layout.heads.length

            delegate: Item {
                id: head

                required property int index
                readonly property var entry: root.layout.heads[index] || null
                readonly property var section: head.entry ? head.entry.section : ({})

                x: root.inset + Math.round(12 * root.s)
                y: head.entry ? head.entry.y : 0
                width: flick.width - x * 2
                height: head.entry ? head.entry.h : 0

                Micro {
                    id: label
                    anchors.left: parent.left
                    anchors.bottom: parent.bottom
                    anchors.bottomMargin: Math.round(7 * root.s)
                    width: Math.min(implicitWidth, parent.width * 0.7)
                    s: root.s
                    text: String(head.section.title || "")
                }
                // Lithic rules a head from its name to its count
                Rectangle {
                    visible: Look.lithic
                    anchors.left: label.right
                    anchors.leftMargin: Math.round(10 * root.s)
                    anchors.right: note.visible ? note.left : parent.right
                    anchors.rightMargin: note.visible ? Math.round(10 * root.s) : 0
                    anchors.verticalCenter: label.verticalCenter
                    height: 1
                    color: Lithic.alpha(Lithic.vein, 0.7)
                }
                Micro {
                    id: note
                    anchors.right: parent.right
                    anchors.verticalCenter: label.verticalCenter
                    visible: text.length > 0
                    s: root.s
                    text: String(head.section.note || "")
                    font.features: ({ "tnum": 1 })
                }
            }
        }

        Repeater {
            model: root.rowDelegate ? 0 : root.count

            delegate: ResultRow {
                required property int index
                readonly property var entry: root.layout.rows[index] || null

                x: root.inset
                y: entry ? entry.y : 0
                width: flick.width - root.inset * 2
                height: entry ? entry.h : 0
                s: root.s
                row: entry ? entry.row : ({})
                selected: index === root.selectedIndex

                Rectangle {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    anchors.leftMargin: Math.round(44 * root.s)
                    height: 1
                    color: Look.divider
                    visible: Look.boxed && parent.entry !== null && parent.entry.last !== true
                }
            }
        }

        // a view's own row, in the list's geometry
        Repeater {
            model: root.rowDelegate ? root.count : 0

            delegate: Loader {
                id: custom

                required property int index
                readonly property var entry: root.layout.rows[index] || null

                x: root.inset
                y: custom.entry ? custom.entry.y : 0
                width: flick.width - root.inset * 2
                height: custom.entry ? custom.entry.h : 0
                sourceComponent: root.rowDelegate

                Binding { target: custom.item; property: "s"; value: root.s; when: custom.item !== null }
                Binding { target: custom.item; property: "row"; value: custom.entry ? custom.entry.row : ({}); when: custom.item !== null }
                Binding { target: custom.item; property: "selected"; value: custom.index === root.selectedIndex; when: custom.item !== null }
            }
        }

        MouseArea {
            id: pointer

            property real lastX: -1
            property real lastY: -1

            width: flick.width
            height: flick.contentHeight
            hoverEnabled: true
            acceptedButtons: Qt.LeftButton | Qt.RightButton
            cursorShape: Geometry.hit(root.layout.rows, mouseY) >= 0 ? Qt.PointingHandCursor : Qt.ArrowCursor

            // a list sliding, growing or arriving under a resting pointer is not the
            // hand choosing: only the pointer's own place on the screen counts
            onPositionChanged: mouse => {
                const at = pointer.mapToItem(null, mouse.x, mouse.y);
                if (Math.abs(at.x - pointer.lastX) < 2 && Math.abs(at.y - pointer.lastY) < 2)
                    return;
                const first = pointer.lastX < 0;
                pointer.lastX = at.x;
                pointer.lastY = at.y;
                if (first)
                    return;
                const index = Geometry.hit(root.layout.rows, mouse.y);
                if (index >= 0 && root.layout.rows[index].row.disabled !== true)
                    root.select(index, false);
            }
            onExited: {
                pointer.lastX = -1;
                pointer.lastY = -1;
            }
            onClicked: mouse => {
                const index = Geometry.hit(root.layout.rows, mouse.y);
                if (index < 0 || root.layout.rows[index].row.disabled === true)
                    return;
                root.select(index, false);
                if (mouse.button === Qt.RightButton)
                    root.menuRequested(root.layout.rows[index].row);
                else
                    root.activated(root.layout.rows[index].row);
            }
        }
    }

    ScrollRail {
        anchors.right: parent.right
        anchors.rightMargin: Math.round(2 * root.s)
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.topMargin: Math.round(4 * root.s)
        anchors.bottomMargin: Math.round(4 * root.s)
        flickable: flick
    }
}
