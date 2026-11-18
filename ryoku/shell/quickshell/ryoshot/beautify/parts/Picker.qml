pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../look"
import "../.."

// One choice among a few, as the design makes it: Lithic's mono slugs on the
// sel plate, Material's connected button group, Adwaita's combo row and
// Fluent's SettingsCard with a ComboBox, each list opening in the panel's
// popover. extra adds a second list behind a "For" menu: Lithic's slug,
// Material's filter chip, a section of the drop-down in Adwaita and Fluent.
Field {
    id: pk

    required property Item menus
    // [{ k, l, f }]: key, short label, the friendly name a combo shows
    property var options: []
    property var extra: []
    property string extraTitle: ""
    property string current: ""
    signal picked(string k)
    // the row's name, shown where the design gives the choice a row of its own
    property string label: ""
    title: pk.combo ? pk.label : ""

    readonly property bool combo: Look.adwaita || Look.fluent
    readonly property var all: pk.options.concat(pk.extra)
    function nameOf(k) {
        for (const o of pk.all) {
            if (o.k === k)
                return o.f || o.l;
        }
        return "";
    }
    readonly property bool extraChosen: pk.extra.some(o => o.k === pk.current)
    function entries() {
        const list = pk.options.map(o => ({ kind: "item", k: o.k, l: o.f || o.l }));
        if (pk.extra.length > 0) {
            list.push({ kind: "heading", k: "", l: pk.extraTitle });
            for (const o of pk.extra)
                list.push({ kind: "item", k: o.k, l: o.f || o.l });
        }
        return list;
    }
    function openCombo(anchor) {
        pk.menus.show(anchor, pk.entries(), pk.current, anchor.width, k => pk.picked(k));
    }
    function openExtra(anchor) {
        pk.menus.show(anchor, pk.extra.map(o => ({ kind: "item", k: o.k, l: o.f || o.l })), pk.current, 0, k => pk.picked(k));
    }

    activatable: Look.adwaita
    onClicked: pk.openCombo(adwValue)

    // Adwaita: the current choice and the drop-down arrow at the row's end
    Row {
        id: adwValue
        visible: Look.adwaita
        spacing: Math.round(8 * pk.u)
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: pk.nameOf(pk.current)
            color: Look.fg2
            font.family: Look.sans
            font.pixelSize: Math.round(14 * pk.u)
            renderType: Text.CurveRendering
        }
        Icon {
            anchors.verticalCenter: parent.verticalCenter
            name: "chevron"
            size: Math.round(16 * pk.u)
            tint: Look.fg
        }
    }

    // Fluent: the ComboBox
    Item {
        id: fluentCombo
        visible: Look.fluent
        width: Math.round(132 * pk.u)
        height: Math.round(32 * pk.u)
        Rectangle {
            anchors.fill: parent
            radius: 4 * pk.u
            color: comboTap.pressed ? Lithic.alpha(Look.fg, Look.dark ? 0.03 : 0.02)
                : comboHover.hovered ? Lithic.alpha(Look.fg, Look.dark ? 0.08 : 0.05)
                : (Look.dark ? Lithic.alpha(Look.fg, 0.06) : Lithic.alpha(Look.ground, 0.7))
            border.width: Math.max(1, Math.round(pk.u))
            border.color: Look.planes.cardStroke
            antialiasing: true
            Behavior on color {
                ColorAnimation { duration: Look.dur(Look.stateMs) }
            }
        }
        Text {
            x: Math.round(11 * pk.u)
            anchors.verticalCenter: parent.verticalCenter
            width: parent.width - x - Math.round(32 * pk.u)
            elide: Text.ElideRight
            text: pk.nameOf(pk.current)
            color: Look.fg
            font.family: Look.sans
            font.pixelSize: Math.round(14 * pk.u)
            renderType: Text.CurveRendering
        }
        Icon {
            anchors.right: parent.right
            anchors.rightMargin: Math.round(11 * pk.u)
            anchors.verticalCenter: parent.verticalCenter
            name: "chevron"
            size: Math.round(12 * pk.u)
            tint: Look.fg2
        }
        HoverHandler {
            id: comboHover
            cursorShape: Qt.PointingHandCursor
        }
        TapHandler {
            id: comboTap
            onTapped: pk.openCombo(fluentCombo)
        }
    }

    block: Column {
        visible: !pk.combo
        width: parent ? parent.width : 0
        spacing: Math.round((Look.lithic ? 8 : 12) * pk.u)

        // Lithic's slugs and Material's connected group share one row of cells
        Item {
            id: cells
            width: parent.width
            height: Math.round((Look.lithic ? 24 : 40) * pk.u)
            readonly property real gap: Math.round(2 * pk.u)
            readonly property real cellW: (width - (pk.options.length - 1) * gap) / Math.max(1, pk.options.length)

            Repeater {
                model: pk.options
                delegate: Item {
                    id: cell
                    required property var modelData
                    required property int index
                    readonly property bool chosen: pk.current === cell.modelData.k
                    readonly property bool first: cell.index === 0
                    readonly property bool last: cell.index === pk.options.length - 1
                    x: cell.index * (cells.cellW + cells.gap)
                    width: cells.cellW
                    height: cells.height

                    readonly property color fillNow: cell.chosen ? Look.sel
                        : cellTap.pressed ? (Look.lithic ? Look.press : Lithic.mix(Look.container, Look.fg, 0.12))
                        : cellHover.hovered ? (Look.lithic ? Look.hover : Lithic.mix(Look.container, Look.fg, 0.08))
                        : Look.lithic ? Lithic.alpha(Look.hover, 0) : Look.container

                    CutRect {
                        visible: Look.lithic
                        anchors.fill: parent
                        design: "lithic"
                        cut: (Look.cutSm + (cellTap.pressed ? Lithic.pressCutGrow : 0)) * pk.u
                        fill: cell.fillNow
                        strokeColor: cell.chosen ? "transparent" : Look.vein
                        strokeWidth: Math.max(1, Math.round(pk.u))
                        Behavior on fill {
                            ColorAnimation { duration: Look.dur(Look.stateMs) }
                        }
                    }
                    // M3 Expressive: the outer ends round fully, the inner corners
                    // stay small, and the chosen button rounds all the way
                    Rectangle {
                        id: m3
                        visible: Look.material
                        anchors.fill: parent
                        readonly property real full: height / 2
                        readonly property real inner: 8 * pk.u
                        property real lead: cell.chosen || cell.first ? m3.full : m3.inner
                        property real trail: cell.chosen || cell.last ? m3.full : m3.inner
                        topLeftRadius: m3.lead
                        bottomLeftRadius: m3.lead
                        topRightRadius: m3.trail
                        bottomRightRadius: m3.trail
                        color: cell.fillNow
                        antialiasing: true
                        Behavior on lead {
                            NumberAnimation { duration: Look.dur(300); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.springDefault }
                        }
                        Behavior on trail {
                            NumberAnimation { duration: Look.dur(300); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.springDefault }
                        }
                        Behavior on color {
                            ColorAnimation { duration: Look.dur(Look.stateMs) }
                        }
                    }
                    Text {
                        anchors.centerIn: parent
                        anchors.verticalCenterOffset: Look.lithic && cellTap.pressed ? Lithic.pressSink * pk.u : 0
                        text: Look.lithic ? cell.modelData.l.toUpperCase() : cell.modelData.l
                        color: cell.chosen ? Look.onSel : Look.lithic ? (cellHover.hovered ? Look.fg : Look.fg2) : Look.containerInk
                        font.family: Look.lithic ? Look.mono : Look.sans
                        font.pixelSize: Math.round((Look.lithic ? 9.5 : 14) * pk.u)
                        font.weight: Look.lithic ? Font.Bold : Font.Medium
                        font.letterSpacing: Look.lithic ? Look.tracking * 0.6 * pk.u : 0
                        renderType: Text.CurveRendering
                    }
                    HoverHandler {
                        id: cellHover
                        cursorShape: Qt.PointingHandCursor
                    }
                    TapHandler {
                        id: cellTap
                        onTapped: pk.picked(cell.modelData.k)
                    }
                }
            }
        }

        // the "For" menu: Lithic's caption and slug, Material's filter chip
        Item {
            visible: pk.extra.length > 0
            width: parent.width
            height: Math.round((Look.lithic ? 24 : 32) * pk.u)

            Text {
                id: forCaption
                visible: Look.lithic
                anchors.verticalCenter: parent.verticalCenter
                text: pk.extraTitle.toUpperCase()
                color: Look.fg3
                font.family: Look.mono
                font.pixelSize: Math.round(9 * pk.u)
                font.weight: Font.Bold
                font.letterSpacing: Look.tracking * pk.u
                renderType: Text.CurveRendering
            }

            Item {
                id: forChip
                x: Look.lithic ? forCaption.width + Math.round(10 * pk.u) : 0
                width: Look.lithic ? parent.width - x : Math.ceil(chipArrow.x + chipArrow.width + Math.round(8 * pk.u))
                height: parent.height
                readonly property color fillNow: pk.extraChosen ? (Look.lithic ? Look.sel : Look.container)
                    : forTap.pressed ? Look.press : forHover.hovered ? Look.hover : Lithic.alpha(Look.hover, 0)
                readonly property color inkNow: pk.extraChosen ? (Look.lithic ? Look.onSel : Look.containerInk) : (Look.lithic ? Look.fg2 : Look.fg2)

                CutRect {
                    visible: Look.lithic
                    anchors.fill: parent
                    design: "lithic"
                    cut: Look.cutSm * pk.u
                    fill: forChip.fillNow
                    strokeColor: pk.extraChosen ? "transparent" : Look.vein
                    strokeWidth: Math.max(1, Math.round(pk.u))
                    Behavior on fill {
                        ColorAnimation { duration: Look.dur(Look.stateMs) }
                    }
                }
                Rectangle {
                    visible: Look.material
                    anchors.fill: parent
                    radius: 8 * pk.u
                    color: forChip.fillNow
                    border.width: pk.extraChosen ? 0 : Math.max(1, Math.round(pk.u))
                    border.color: Look.edge
                    antialiasing: true
                    Behavior on color {
                        ColorAnimation { duration: Look.dur(Look.stateMs) }
                    }
                }
                Row {
                    id: chipRow
                    x: Look.lithic ? Math.round(8 * pk.u) : Math.round((pk.extraChosen ? 8 : 12) * pk.u)
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Math.round(6 * pk.u)
                    Icon {
                        visible: Look.material && pk.extraChosen
                        anchors.verticalCenter: parent.verticalCenter
                        name: "check"
                        size: Math.round(18 * pk.u)
                        tint: forChip.inkNow
                    }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: {
                            const name = pk.extraChosen ? pk.nameOf(pk.current) : (Look.lithic ? "\u2014" : pk.extraTitle);
                            return Look.lithic ? name.toUpperCase() : name;
                        }
                        color: forChip.inkNow
                        font.family: Look.lithic ? Look.mono : Look.sans
                        font.pixelSize: Math.round((Look.lithic ? 9.5 : 14) * pk.u)
                        font.weight: Look.lithic ? Font.Bold : Font.Medium
                        font.letterSpacing: Look.lithic ? Look.tracking * pk.u : 0
                        renderType: Text.CurveRendering
                    }
                }
                Icon {
                    anchors.right: parent.right
                    anchors.rightMargin: Math.round((Look.lithic ? 6 : 8) * pk.u)
                    anchors.verticalCenter: parent.verticalCenter
                    visible: Look.lithic
                    name: "chevron"
                    size: Math.round(14 * pk.u)
                    tint: forChip.inkNow
                }
                Icon {
                    id: chipArrow
                    visible: Look.material
                    x: chipRow.x + chipRow.implicitWidth + Math.round(2 * pk.u)
                    anchors.verticalCenter: parent.verticalCenter
                    name: "chevron"
                    size: Math.round(18 * pk.u)
                    tint: forChip.inkNow
                }
                HoverHandler {
                    id: forHover
                    cursorShape: Qt.PointingHandCursor
                }
                TapHandler {
                    id: forTap
                    onTapped: pk.openExtra(forChip)
                }
            }
        }
    }
}
