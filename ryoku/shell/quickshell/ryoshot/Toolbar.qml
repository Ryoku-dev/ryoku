pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Lithic
import Ryoku.Ui.Singletons
import "look"
import "dock"
import "Singletons"
import "lib/dockplace.js" as Place

// The annotation dock: the tools in groups on the first row; on the second the
// ink and stroke, the options the tool has, then undo, Beautify, Copy, Save
// and the overflow. It places itself under the selection, else over it, else
// inside its bottom edge; Lithic hangs it from the selection's left edge by a
// leader, like a caption under a figure, the others centre it.
Item {
    id: tb

    property real u: 1
    // the selection on this output, and how far the grip has moved the dock
    property var sel: null
    property real dragX: 0
    property real dragY: 0

    property var tools: []
    property string activeTool: "rect"
    property color activeColor: Look.sel
    property int activeWidth: 4
    property bool activeFill: false
    property bool activeRough: false
    property bool hasFill: false
    property bool hasRough: false
    property string openPopover: ""
    property bool canUndo: false
    property bool canRedo: false
    property bool settingsOpen: false
    // Copy and Save style every shot (Beautify's "Use every time")
    property bool styled: false
    // Beautify is opening over the dock: it steps out, then hides
    property bool away: false

    signal toolPicked(string tool)
    signal colorButtonClicked()
    signal widthButtonClicked()
    signal moreButtonClicked()
    signal fillToggled()
    signal roughToggled()
    signal undoRequested()
    signal redoRequested()
    signal copyRequested()
    signal saveRequested()
    signal beautifyRequested()
    signal dragged(real dx, real dy)

    // the full height of the dock over a button, in the dock's parent, so a
    // popover opened from the second row never covers the first
    readonly property rect colorRect: Qt.rect(tb.x + optionRow.x + colorBtn.x, tb.y, colorBtn.width, tb.height)
    readonly property rect widthRect: Qt.rect(tb.x + optionRow.x + widthBtn.x, tb.y, widthBtn.width, tb.height)
    readonly property rect moreRect: Qt.rect(tb.x + moreAnchor.x, tb.y, moreAnchor.width, tb.height)
    readonly property alias moreAnchor: moreAnchor
    // which side of the selection the dock took, so popovers open away from it
    readonly property string side: tb.place.side
    // the Beautify button, in the dock's parent, for the studio's container transform
    readonly property rect beautifyRect: Qt.rect(tb.x + actionRow.x + beautifyBtn.x, tb.y + actionRow.y + beautifyBtn.y,
        beautifyBtn.width, beautifyBtn.height)

    readonly property var groups: [
        ["select"],
        ["rect", "ellipse", "line", "arrow"],
        ["pen", "marker"],
        ["counter", "text"],
        ["blur", "redact"],
        ["spotlight", "magnify"],
        ["ocr"]
    ]
    readonly property var toolById: {
        var m = {};
        for (var i = 0; i < tb.tools.length; i++)
            m[tb.tools[i].id] = tb.tools[i];
        return m;
    }

    // logical metrics per design
    readonly property real cellW: Look.lithic ? 28 : 32
    readonly property real cellH: Look.lithic ? 28 : Look.adwaita ? 34 : 32
    readonly property real groupGap: Look.lithic ? 8 : 6
    readonly property real pad: Look.lithic ? 4 : 6
    readonly property real gripW: Look.lithic ? 12 : 14
    readonly property real gripGap: Look.lithic ? 4 : 2
    // Lithic and Material stack two separate planes; Adwaita and Fluent one card
    readonly property real stripGap: Look.lithic ? 4 : Look.material ? 8 : 0
    readonly property real rowGap: Look.lithic || Look.material ? tb.pad * 2 + tb.stripGap : 2

    readonly property real rowH: Math.round(tb.cellH * tb.u)
    readonly property real stripH: Math.round((tb.cellH + tb.pad * 2) * tb.u)
    readonly property real row2Y: Math.round((tb.pad + tb.cellH + tb.rowGap) * tb.u)
    readonly property real toolsX: Math.round((tb.pad + tb.gripW + tb.gripGap) * tb.u)

    // the second row at its widest (every option the tools can show), so a
    // tool change never resizes the dock
    readonly property real row2Width: Math.round((tb.cellW * 3 + 40 + 6 + tb.groupGap * 2) * tb.u) + actionRow.width
    implicitWidth: Math.ceil(tb.toolsX + Math.max(toolRow.width, tb.row2Width) + tb.pad * tb.u)
    implicitHeight: Math.ceil(tb.row2Y + tb.rowH + tb.pad * tb.u)
    width: implicitWidth
    height: implicitHeight

    readonly property var area: tb.parent ? { w: tb.parent.width, h: tb.parent.height } : { w: 0, h: 0 }
    readonly property var place: tb.sel
        ? Place.dock(tb.sel, { w: tb.width, h: tb.height }, tb.area, {
            align: Look.lithic ? "left" : "center",
            // Lithic's width dimension stands 8 over the frame; the dock clears it
            gap: 12 * tb.u, gapAbove: (Look.lithic ? 30 : 12) * tb.u, inset: 12 * tb.u, margin: 8 * tb.u,
            dx: tb.dragX, dy: tb.dragY
        })
        : { x: 0, y: 0, side: "below" }
    x: tb.place.x
    y: tb.place.y

    readonly property bool roomAbove: tb.y > 40 * tb.u
    readonly property bool roomBelow: tb.area.h - tb.y - tb.height > 40 * tb.u

    // the dock arrives with its design's motion each time editing begins, and
    // steps out the way it came when Beautify opens over it
    property real reveal: 0
    readonly property bool present: tb.visible && !tb.away
    function arrival() {
        if (!tb.visible) {
            arrive.stop();
            leave.stop();
            tb.reveal = 0;
        } else if (tb.away) {
            arrive.stop();
            if (Look.reduceMotion) tb.reveal = 0;
            else leave.restart();
        } else if (Look.reduceMotion) {
            leave.stop();
            tb.reveal = 1;
        } else {
            leave.stop();
            arrive.restart();
        }
    }
    onPresentChanged: tb.arrival()
    Component.onCompleted: tb.arrival()
    NumberAnimation {
        id: arrive
        target: tb
        property: "reveal"
        to: 1
        duration: Look.dur(Look.enterMs)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Look.material ? Look.springDefault : Look.enterCurve
    }
    NumberAnimation {
        id: leave
        target: tb
        property: "reveal"
        to: 0
        duration: Look.dur(Look.exitMs)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Look.exitCurve
    }
    readonly property real travel: (Look.material ? 16 : Look.fluent ? 12 : Look.adwaita ? 12 : 6) * tb.u
    readonly property real rise: tb.side === "above" ? -1 : 1
    Item {
        id: body
        width: tb.width
        height: tb.height
        opacity: Math.min(1, tb.reveal * (Look.material ? 2 : 1))
        y: tb.travel * tb.rise * (1 - tb.reveal)
        scale: Look.material ? 0.9 + 0.1 * tb.reveal : 1
        transformOrigin: tb.side === "above" ? Item.Bottom : Item.Top

        // Lithic's leader: a hairline from the selection's crop mark to the
        // dock, drawn while the dock hangs where it was placed
        Rectangle {
            readonly property bool hangs: Look.lithic && tb.sel !== null && tb.place.side !== "inside"
                && tb.dragX === 0 && tb.dragY === 0 && Math.abs(tb.x - tb.sel.x) < 0.5
            readonly property real reach: !hangs ? 0
                : tb.place.side === "below" ? tb.y - (tb.sel.y + tb.sel.h + 4 * tb.u)
                : (tb.sel.y - 4 * tb.u) - (tb.y + tb.height)
            visible: hangs && reach > 0
            width: Math.max(1, Math.round(tb.u))
            height: reach
            // past the strip's cut corner, still under the crop mark's arm
            x: Math.round(8 * tb.u)
            y: tb.place.side === "below" ? -reach : tb.height
            color: Lithic.alpha(Look.sel, 0.75)
        }

        Plane {
            anchors.fill: parent
            u: tb.u
            stripHeight: tb.stripH
            stripGap: Math.round(tb.stripGap * tb.u)
        }

        Item {
            id: grip
            x: Math.round(tb.pad * tb.u)
            y: Math.round(tb.pad * tb.u)
            width: Math.round(tb.gripW * tb.u)
            height: tb.rowH

            Icon {
                anchors.centerIn: parent
                name: "grip"
                size: 16 * tb.u
                tint: gripArea.containsMouse || gripArea.pressed ? Look.fg : Look.fg3
            }

            // Deltas are reported in window space: the dock moves under the
            // pointer as it is dragged, so a delta measured against the dock's
            // own coordinates would feed back on itself.
            MouseArea {
                id: gripArea
                anchors.fill: parent
                hoverEnabled: true
                cursorShape: pressed ? Qt.ClosedHandCursor : Qt.OpenHandCursor
                property real lastX: 0
                property real lastY: 0
                onPressed: (m) => {
                    var p = mapToItem(null, m.x, m.y);
                    lastX = p.x;
                    lastY = p.y;
                }
                onPositionChanged: (m) => {
                    if (!pressed) return;
                    var p = mapToItem(null, m.x, m.y);
                    tb.dragged(p.x - lastX, p.y - lastY);
                    lastX = p.x;
                    lastY = p.y;
                }
            }
        }

        // Lithic's sel plate glides from tool to tool; the round designs let
        // each button morph its own
        CutRect {
            id: glide
            visible: Look.lithic && tb.toolIndex(tb.activeTool) >= 0
            design: "lithic"
            x: toolRow.x + tb.toolX(tb.activeTool)
            y: toolRow.y
            width: Math.round(tb.cellW * tb.u)
            height: tb.rowH
            cut: Look.cutSm * tb.u
            fill: Look.sel
            Behavior on x {
                NumberAnimation { duration: Look.dur(180); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.decelerate }
            }
        }

        Row {
            id: toolRow
            x: tb.toolsX
            y: Math.round(tb.pad * tb.u)
            spacing: Math.round(tb.groupGap * tb.u)

            Repeater {
                model: tb.groups
                Row {
                    id: group
                    required property var modelData
                    Repeater {
                        model: group.modelData
                        GlyphButton {
                            required property string modelData
                            readonly property var tool: tb.toolById[modelData] || { icon: modelData, label: "", key: "" }
                            u: tb.u
                            cellWidth: tb.cellW
                            cellHeight: tb.cellH
                            icon: tool.icon
                            checked: tb.activeTool === modelData
                            checkedFill: Look.lithic ? "transparent" : Look.checkedFill
                            tip: tool.label
                            keyHint: tool.key
                            tipBelow: !tb.roomAbove
                            onClicked: tb.toolPicked(modelData)
                        }
                    }
                }
            }
        }

        Row {
            id: optionRow
            x: tb.toolsX
            y: tb.row2Y
            spacing: Math.round(2 * tb.u)

            Swatch {
                id: colorBtn
                u: tb.u
                cellWidth: tb.cellW
                cellHeight: tb.cellH
                ink: tb.activeColor
                active: tb.openPopover === "color"
                tip: I18n.tr("Colour")
                keyHint: "1–8"
                tipBelow: tb.roomBelow
                onClicked: tb.colorButtonClicked()
            }
            Stroke {
                id: widthBtn
                u: tb.u
                cellWidth: 40
                cellHeight: tb.cellH
                strokeWidth: tb.activeWidth
                active: tb.openPopover === "width"
                tip: I18n.tr("Width")
                keyHint: "[ ]"
                tipBelow: tb.roomBelow
                onClicked: tb.widthButtonClicked()
            }
            Contextual {
                shown: tb.hasFill
                GlyphButton {
                    u: tb.u
                    cellWidth: tb.cellW
                    cellHeight: tb.cellH
                    icon: "fill"
                    checked: tb.activeFill
                    tip: I18n.tr("Fill")
                    keyHint: "f"
                    tipBelow: tb.roomBelow
                    onClicked: tb.fillToggled()
                }
            }
            Contextual {
                shown: tb.hasRough
                GlyphButton {
                    u: tb.u
                    cellWidth: tb.cellW
                    cellHeight: tb.cellH
                    icon: "sketch"
                    checked: tb.activeRough
                    tip: I18n.tr("Sketch")
                    keyHint: "k"
                    tipBelow: tb.roomBelow
                    onClicked: tb.roughToggled()
                }
            }
        }

        Row {
            id: actionRow
            x: tb.width - width - Math.round(tb.pad * tb.u)
            y: tb.row2Y
            spacing: Math.round(tb.groupGap * tb.u)

            Row {
                GlyphButton {
                    u: tb.u
                    cellWidth: tb.cellW
                    cellHeight: tb.cellH
                    icon: "undo"
                    enabled: tb.canUndo
                    tip: I18n.tr("Undo")
                    keyHint: Look.keyHint("Ctrl+Z")
                    tipBelow: tb.roomBelow
                    onClicked: tb.undoRequested()
                }
                GlyphButton {
                    u: tb.u
                    cellWidth: tb.cellW
                    cellHeight: tb.cellH
                    icon: "redo"
                    enabled: tb.canRedo
                    tip: I18n.tr("Redo")
                    keyHint: Look.keyHint("Ctrl+Shift+Z")
                    tipBelow: tb.roomBelow
                    onClicked: tb.redoRequested()
                }
            }

            BeautifyButton {
                id: beautifyBtn
                u: tb.u
                cellHeight: tb.cellH
                tipBelow: tb.roomBelow
                firstUse: !Config.beautifySeen
                present: tb.present
                onClicked: tb.beautifyRequested()
            }

            Row {
                spacing: Math.round(4 * tb.u)
                TextButton {
                    id: copyBtn
                    u: tb.u
                    primary: true
                    cellHeight: tb.cellH
                    icon: Look.lithic ? "" : "copy"
                    text: I18n.tr("Copy")
                    keyHint: Look.keyHint("Ctrl+C")
                    tip: tb.styled ? I18n.tr("Copies with your Beautify style") : ""
                    tipBelow: tb.roomBelow
                    onClicked: tb.copyRequested()

                    // "Use every time" is on: Copy and Save style the shot
                    Item {
                        readonly property real d: Math.round(8 * tb.u)
                        readonly property real ring: Math.max(1, Math.round(2 * tb.u))
                        width: d + 2 * ring
                        height: width
                        x: parent.width - width / 2 - Math.round(3 * tb.u)
                        y: -height / 2 + Math.round(3 * tb.u)
                        opacity: tb.styled ? 1 : 0
                        scale: tb.styled ? 1 : 0.4
                        visible: opacity > 0
                        Behavior on opacity { NumberAnimation { duration: Look.dur(160); easing.type: Easing.OutCubic } }
                        Behavior on scale { NumberAnimation { duration: Look.dur(260); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.springFast } }
                        CutRect {
                            visible: Look.lithic
                            anchors.fill: parent
                            design: "lithic"
                            cut: 2 * tb.u
                            fill: Look.sel
                            strokeColor: Look.plane
                            strokeWidth: parent.ring
                        }
                        Rectangle {
                            visible: !Look.lithic
                            anchors.fill: parent
                            radius: width / 2
                            color: Look.sel
                            border.width: parent.ring
                            border.color: Look.plane
                            antialiasing: true
                        }
                    }
                }
                GlyphButton {
                    u: tb.u
                    cellWidth: tb.cellW
                    cellHeight: tb.cellH
                    icon: "save"
                    tip: tb.styled ? I18n.tr("Saves with your Beautify style") : I18n.tr("Save")
                    keyHint: Look.keyHint("Ctrl+S")
                    tipBelow: tb.roomBelow
                    onClicked: tb.saveRequested()
                }
                GlyphButton {
                    id: moreBtn
                    u: tb.u
                    cellWidth: tb.cellW
                    cellHeight: tb.cellH
                    icon: Look.material ? "more" : Look.adwaita ? "menu" : "moreH"
                    checked: tb.openPopover === "more" || tb.openPopover === "phone" || tb.settingsOpen
                    checkedFill: Look.press
                    checkedInk: Look.fg
                    tip: I18n.tr("More")
                    tipBelow: tb.roomBelow
                    onClicked: tb.moreButtonClicked()
                }
            }
        }

        Item {
            id: moreAnchor
            x: actionRow.x + actionRow.width - moreBtn.width
            width: moreBtn.width
            height: tb.height
        }
    }

    function toolIndex(id) {
        var n = 0;
        for (var g = 0; g < tb.groups.length; g++)
            for (var i = 0; i < tb.groups[g].length; i++) {
                if (tb.groups[g][i] === id) return n;
                n++;
            }
        return -1;
    }
    function toolX(id) {
        var n = 0;
        for (var g = 0; g < tb.groups.length; g++)
            for (var i = 0; i < tb.groups[g].length; i++) {
                if (tb.groups[g][i] === id)
                    return n * Math.round(tb.cellW * tb.u) + g * Math.round(tb.groupGap * tb.u);
                n++;
            }
        return 0;
    }

    // a row-two option that only some tools have: it opens and folds its
    // width, so the row never jumps
    component Contextual: Item {
        id: ctx
        property bool shown: false
        default property alias content: holder.data
        width: ctx.shown ? holder.childrenRect.width : 0
        height: tb.rowH
        clip: true
        opacity: ctx.shown ? 1 : 0
        Behavior on width {
            NumberAnimation { duration: Look.dur(160); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.decelerate }
        }
        Behavior on opacity {
            NumberAnimation { duration: Look.dur(120) }
        }
        Item {
            id: holder
            width: childrenRect.width
            height: parent.height
        }
    }
}
