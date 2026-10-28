import QtQuick
import Quickshell
import "../kit"
import "../kit/Widgets.js" as Widgets
import "../../modules"
import Ryoku.Ui
import Ryoku.Ui.Singletons

// Layout route (配置): the centrepiece. The bar is data, so here it is drawn as
// its own geometry -- three lanes side by side (LEFT, CENTER, RIGHT), each a
// list of its widgets in bar order, top to bottom, every one with its own on/off
// switch so nothing hides behind a scroll. Pick a row and the strip beneath
// moves it: up or down its lane, across to the next lane, or into its settings.
// + ADD WIDGET brings a hidden built-in or an installed plugin onto the bar;
// UNLOCK THE BAR hands you the in-place drag; RESET LAYOUT restores the shipped
// order. Every write goes through the root's barLayout* API, and
// barLayoutChanged rebinds the lanes.
Item {
    id: page
    property var root: null
    property var cc: null
    readonly property var tk: page.cc ? page.cc.tokens : null
    readonly property real colW: Math.min(page.width, page.tk ? page.tk.contentW : 640)
    implicitHeight: col.implicitHeight

    property string selId: ""
    property bool pickerOpen: false
    property bool armedReset: false
    Timer { id: resetDisarm; interval: 2600; onTriggered: page.armedReset = false }

    // barLayout is a live bound var, but a Repeater's model does not re-read a
    // function; `rev` is bumped on barLayoutChanged so every lane and chip binding
    // re-derives from the current layout and catalogue in one place.
    property int rev: 0
    Connections {
        target: page.root
        ignoreUnknownSignals: true
        function onBarLayoutChanged() { page.rev = page.rev + 1 }
    }

    readonly property var leftIds: { page.rev; return page.laneIds("left") }
    readonly property var centerIds: { page.rev; return page.laneIds("center") }
    readonly property var rightIds: { page.rev; return page.laneIds("right") }
    // flattened bar order, for stepping the selection with the arrow keys.
    readonly property var flatIds: page.leftIds.concat(page.centerIds, page.rightIds)

    function laneIds(section) {
        if (!page.root || !page.root.barLayout) return []
        var l = page.root.barLayout[section]
        return l ? l : []
    }
    function entryOf(id) {
        if (!page.root || !page.root.barCatalog) return null
        var c = page.root.barCatalog
        for (var i = 0; i < c.length; i++) if (c[i].id === id) return c[i]
        return null
    }
    readonly property var selEntry: { page.rev; return page.entryOf(page.selId) }

    function labelOf(id) { var e = page.entryOf(id); return e ? e.label : id }
    function glossOf(id)  { var e = page.entryOf(id); return e ? (e.gloss || "") : "" }
    function shownOf(id)  { var e = page.entryOf(id); return e ? e.shown === true : true }

    // ── mutations, all through the root API (guarded: the API may land after the
    // page does while the Theme is being built) ──
    function move(id, section, index) {
        if (page.root && page.root.barLayoutMove) page.root.barLayoutMove(id, section, index)
    }
    function show(id, on) {
        if (page.root && page.root.barLayoutShow) page.root.barLayoutShow(id, on)
    }
    function reset() {
        if (page.root && page.root.barLayoutReset) page.root.barLayoutReset()
    }
    function addWidget(id) {
        page.show(id, true)
        page.selId = id
        page.pickerOpen = false
    }
    function openSettings(id) {
        if (page.cc) page.cc.go("widgets", id)
    }

    // move the selected chip one step within its own lane.
    function nudge(dir) {
        var e = page.selEntry
        if (!e || e.section === "") return
        var lane = page.laneIds(e.section)
        var to = e.index + dir
        if (to < 0 || to >= lane.length) return
        page.move(page.selId, e.section, to)
    }
    // step the selection along the flattened bar order.
    function step(dir) {
        var flat = page.flatIds
        if (flat.length === 0) return
        var i = flat.indexOf(page.selId)
        if (i < 0) { page.selId = flat[dir > 0 ? 0 : flat.length - 1]; return }
        var to = Math.max(0, Math.min(flat.length - 1, i + dir))
        page.selId = flat[to]
    }

    // ── the hidden / plugin pools the ADD picker offers ──
    function hiddenBuiltins() {
        page.rev
        var out = []
        if (!page.root || !page.root.barCatalog) return out
        var c = page.root.barCatalog
        for (var i = 0; i < c.length; i++)
            if (c[i].kind === "builtin" && c[i].shown !== true) out.push(c[i])
        return out
    }
    function barPlugins() {
        page.rev
        var out = []
        if (!page.root || !page.root.barCatalog) return out
        var c = page.root.barCatalog
        for (var i = 0; i < c.length; i++)
            if (c[i].kind === "plugin" && c[i].shown !== true) out.push(c[i])
        return out
    }

    // step the selected row across lanes, keeping its place near the end.
    readonly property var laneOrder: ["left", "center", "right"]
    function shiftLane(dir) {
        var e = page.selEntry
        if (!e) return
        var i = page.laneOrder.indexOf(e.section)
        var to = (i < 0 ? (dir > 0 ? 0 : 2) : i + dir)
        if (to < 0 || to > 2) return
        page.move(page.selId, page.laneOrder[to], page.laneIds(page.laneOrder[to]).length)
    }

    // ── keyboard: up/down step the selection, Shift+up/down move the row within
    // its lane, left/right move it across lanes, Delete hides it, Escape bubbles
    // to the plate (which closes the panel). ──
    focus: true
    Component.onCompleted: page.forceActiveFocus()
    Keys.onPressed: function (e) {
        if (page.pickerOpen) {
            if (e.key === Qt.Key_Escape) { page.pickerOpen = false; e.accepted = true }
            return
        }
        if (e.key === Qt.Key_Up) {
            if (e.modifiers & Qt.ShiftModifier) page.nudge(-1); else page.step(-1)
            e.accepted = true
        } else if (e.key === Qt.Key_Down) {
            if (e.modifiers & Qt.ShiftModifier) page.nudge(1); else page.step(1)
            e.accepted = true
        } else if (e.key === Qt.Key_Left) {
            page.shiftLane(-1); e.accepted = true
        } else if (e.key === Qt.Key_Right) {
            page.shiftLane(1); e.accepted = true
        } else if (e.key === Qt.Key_Delete || e.key === Qt.Key_Backspace) {
            if (page.selId !== "") { page.show(page.selId, false); e.accepted = true }
        }
    }

    // one widget as a lane row: its glyph (or initial), its label, and its own
    // switch. Dimmed while hidden, a bone plate while selected.
    component WRow: Rectangle {
        id: wrow
        property string wid: ""
        property string section: ""
        readonly property bool shown: page.shownOf(wrow.wid)
        readonly property bool sel: page.selId === wrow.wid
        readonly property string glyph: Widgets.glyphFor(wrow.wid)
        readonly property bool canHide: Widgets.hideable(wrow.wid)

        width: parent ? parent.width : 0
        height: page.tk ? page.tk.rowH - 6 : 34
        radius: Tokens.radius
        color: wrow.sel ? Tokens.bone : (rma.containsMouse ? Tokens.tint5 : "transparent")
        Behavior on color { ColorAnimation { duration: Tokens.snap } }

        IconText {
            id: rglyph
            visible: wrow.glyph !== ""
            anchors.left: parent.left
            anchors.leftMargin: page.tk ? page.tk.gap / 2 : 6
            anchors.verticalCenter: parent.verticalCenter
            text: wrow.glyph
            color: wrow.sel ? Tokens.inkOnBone : (wrow.shown ? Tokens.inkDim : Tokens.inkFaint)
            font.pixelSize: Tokens.fBody
        }
        UiText {
            id: rinit
            visible: wrow.glyph === ""
            anchors.left: parent.left
            anchors.leftMargin: page.tk ? page.tk.gap / 2 : 6
            anchors.verticalCenter: parent.verticalCenter
            width: Tokens.fBody
            horizontalAlignment: Text.AlignHCenter
            text: Widgets.initialFor(page.labelOf(wrow.wid))
            color: wrow.sel ? Tokens.inkOnBone : (wrow.shown ? Tokens.inkDim : Tokens.inkFaint)
            font.family: Tokens.mono
            font.pixelSize: Tokens.fSmall
            font.weight: Font.DemiBold
        }
        UiText {
            anchors.left: wrow.glyph !== "" ? rglyph.right : rinit.right
            anchors.leftMargin: page.tk ? page.tk.gap / 2 : 6
            anchors.right: rsw.left
            anchors.rightMargin: page.tk ? page.tk.gap / 2 : 6
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr(page.labelOf(wrow.wid))
            color: wrow.sel ? Tokens.inkOnBone : (wrow.shown ? Tokens.ink : Tokens.inkFaint)
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            elide: Text.ElideRight
        }
        Sw {
            id: rsw
            anchors.right: parent.right
            anchors.rightMargin: page.tk ? page.tk.gap / 2 : 6
            anchors.verticalCenter: parent.verticalCenter
            scale: 0.8
            transformOrigin: Item.Right
            enabled: wrow.canHide
            opacity: wrow.canHide ? 1 : 0.35
            on: wrow.shown
            onToggled: (v) => page.show(wrow.wid, v)
        }
        MouseArea {
            id: rma
            anchors.left: parent.left
            anchors.right: rsw.left
            anchors.top: parent.top
            anchors.bottom: parent.bottom
            hoverEnabled: true
            cursorShape: Qt.PointingHandCursor
            onClicked: { page.selId = wrow.sel ? "" : wrow.wid; page.forceActiveFocus() }
        }
    }

    // one lane: its LEFT/CENTER/RIGHT eyebrow and a plate listing its widgets in
    // bar order, top to bottom.
    component Lane: Column {
        id: lane
        property string section: "left"
        property var ids: []
        property string caption: ""
        spacing: page.tk ? page.tk.gap / 2 : 6

        Row {
            spacing: page.tk ? page.tk.gap / 2 : 6
            UiText {
                text: lane.caption
                color: Tokens.inkFaint
                font.family: Tokens.mono
                font.pixelSize: Tokens.fTiny
                font.letterSpacing: Tokens.trackMark
                font.weight: Font.DemiBold
            }
            UiText {
                text: "" + lane.ids.length
                color: Tokens.inkFaint
                font.family: Tokens.mono
                font.pixelSize: Tokens.fTiny
                opacity: 0.7
            }
        }

        Rectangle {
            width: lane.width
            height: Math.max(page.tk ? page.tk.rowH : 40, laneCol.implicitHeight + (page.tk ? page.tk.gap : 12))
            radius: Tokens.radius
            color: Tokens.paperLift
            border.width: Tokens.border
            border.color: Tokens.line

            Column {
                id: laneCol
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.margins: page.tk ? page.tk.gap / 2 : 6
                spacing: 2
                Repeater {
                    model: lane.ids
                    delegate: WRow {
                        required property string modelData
                        wid: modelData
                        section: lane.section
                    }
                }
            }
            UiText {
                anchors.centerIn: parent
                visible: lane.ids.length === 0
                text: I18n.tr("empty")
                color: Tokens.inkFaint
                font.family: Tokens.mono
                font.pixelSize: Tokens.fTiny
            }
        }
    }

    // what is still below the plate's edge; the stage reads it for the fade.
    readonly property real scrollRemaining: Math.max(0, flick.contentHeight - flick.height - flick.contentY)

    Flickable {
        id: flick
        anchors.fill: parent
        contentWidth: width
        contentHeight: col.implicitHeight + (col.implicitHeight > height && page.tk ? page.tk.tailPad : 0)
        clip: true
        boundsBehavior: Flickable.StopAtBounds

        Column {
            id: col
            width: page.colW
            spacing: page.tk ? page.tk.sectionGap : 24

            // ── toolbar ──
            Entrance {
                width: page.colW
                index: 0
                Row {
                    width: page.colW
                    spacing: page.tk ? page.tk.gap : 12

                    Btn {
                        text: I18n.tr("+ ADD WIDGET")
                        primary: true
                        onAct: page.pickerOpen = true
                    }
                    Btn {
                        text: (page.root && page.root.barUnlocked) ? I18n.tr("LOCK THE BAR") : I18n.tr("UNLOCK THE BAR")
                        onAct: {
                            if (page.root) page.root.barUnlocked = !page.root.barUnlocked
                            // hand the desktop over to the drag: the panel covers the
                            // bar, so it steps aside while you rearrange in place.
                            if (page.root && page.root.barUnlocked && page.cc) page.cc.close()
                        }
                    }
                    Btn {
                        text: page.armedReset ? I18n.tr("CONFIRM RESET") : I18n.tr("RESET LAYOUT")
                        onAct: {
                            if (!page.armedReset) { page.armedReset = true; resetDisarm.restart(); return }
                            page.armedReset = false
                            resetDisarm.stop()
                            page.selId = ""
                            page.reset()
                        }
                    }
                }
            }

            // ── the three lanes, side by side as they are on the bar ──
            Entrance {
                width: page.colW
                index: 1
                Row {
                    id: lanes
                    width: page.colW
                    spacing: page.tk ? page.tk.gap : 12
                    readonly property real laneW: Math.floor((width - spacing * 2) / 3)
                    Lane { width: lanes.laneW; section: "left";   ids: page.leftIds;   caption: I18n.tr("LEFT") }
                    Lane { width: lanes.laneW; section: "center"; ids: page.centerIds; caption: I18n.tr("CENTER") }
                    Lane { width: lanes.laneW; section: "right";  ids: page.rightIds;  caption: I18n.tr("RIGHT") }
                }
            }

            // ── the selected row's control strip; always present, so the page
            // never jumps when a row is picked ──
            Entrance {
                width: page.colW
                index: 2
                Rectangle {
                    width: page.colW
                    height: stripRow.implicitHeight + (page.tk ? page.tk.gap * 2 : 24)
                    radius: Tokens.radius
                    color: "transparent"
                    border.width: Tokens.border
                    border.color: Tokens.line

                    Row {
                        id: stripRow
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.leftMargin: page.tk ? page.tk.gap : 12
                        anchors.rightMargin: page.tk ? page.tk.gap : 12
                        spacing: page.tk ? page.tk.gap : 12

                        Column {
                            anchors.verticalCenter: parent.verticalCenter
                            width: parent.width - actions.width - parent.spacing
                            spacing: 1
                            UiText {
                                width: parent.width
                                text: page.selEntry ? I18n.tr(page.selEntry.label) : I18n.tr("Pick a widget")
                                color: page.selEntry ? Tokens.ink : Tokens.inkFaint
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fRow
                                font.weight: Font.DemiBold
                                elide: Text.ElideRight
                            }
                            UiText {
                                width: parent.width
                                text: page.selEntry
                                    ? ((page.selEntry.kind === "plugin" ? I18n.tr("Plugin") : I18n.tr("Built-in"))
                                       + (page.selEntry.gloss ? "  " + page.selEntry.gloss : ""))
                                    : I18n.tr("Then move it up, down, or across a lane")
                                color: Tokens.inkFaint
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall
                                elide: Text.ElideRight
                            }
                        }
                        Row {
                            id: actions
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: page.tk ? page.tk.gap / 2 : 6
                            Btn {
                                text: "\u25b2"
                                armed: page.selEntry && page.selEntry.section !== "" && page.selEntry.index > 0
                                onAct: page.nudge(-1)
                            }
                            Btn {
                                text: "\u25bc"
                                armed: {
                                    var e = page.selEntry
                                    return e && e.section !== "" && e.index < page.laneIds(e.section).length - 1
                                }
                                onAct: page.nudge(1)
                            }
                            Btn {
                                text: "\u25c0 " + I18n.tr("LANE")
                                armed: page.selEntry && page.selEntry.section !== "left"
                                onAct: page.shiftLane(-1)
                            }
                            Btn {
                                text: I18n.tr("LANE") + " \u25b6"
                                armed: page.selEntry && page.selEntry.section !== "right"
                                onAct: page.shiftLane(1)
                            }
                            Btn {
                                text: I18n.tr("SETTINGS")
                                armed: !!page.selEntry
                                onAct: page.openSettings(page.selId)
                            }
                        }
                    }
                }
            }
        }
    }

    CcScrollRail { root: page.root; tk: page.tk; flick: flick; z: 5 }

    // ── + ADD WIDGET picker: an in-panel overlay of the two pools ──
    Item {
        id: picker
        anchors.fill: parent
        visible: opacity > 0.001
        opacity: page.pickerOpen ? 1 : 0
        z: 60
        Behavior on opacity { NumberAnimation { duration: page.tk ? page.tk.fade : 150 } }

        Rectangle {
            anchors.fill: parent
            color: page.tk ? Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, 0.32) : Qt.rgba(0, 0, 0, 0.38)
            MouseArea { anchors.fill: parent; onClicked: page.pickerOpen = false }
        }

        Rectangle {
            anchors.centerIn: parent
            width: Math.min(page.width - (page.tk ? page.tk.pad * 2 : 40), page.tk ? page.tk.contentW : 640)
            height: Math.min(page.height - (page.tk ? page.tk.pad * 2 : 40), pickerCol.implicitHeight + (page.tk ? page.tk.pad * 2 : 40))
            radius: Tokens.radius
            color: Tokens.paperLift
            border.width: Tokens.border
            border.color: Tokens.lineStrong

            MouseArea { anchors.fill: parent; onClicked: {} }   // eat clicks inside

            Flickable {
                id: pickerFlick
                anchors.fill: parent
                anchors.margins: page.tk ? page.tk.pad : 20
                contentWidth: width
                contentHeight: pickerCol.implicitHeight
                clip: true
                boundsBehavior: Flickable.StopAtBounds

                Column {
                    id: pickerCol
                    width: pickerFlick.width
                    spacing: page.tk ? page.tk.gap : 12

                    Row {
                        width: parent.width
                        UiText {
                            text: I18n.tr("ADD WIDGET")
                            color: Tokens.inkMuted
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fMicro
                            font.letterSpacing: Tokens.trackMark
                            font.weight: Font.DemiBold
                        }
                        Item { width: parent.width - 200; height: 1 }
                        UiText {
                            text: "\u5f15\u5165"
                            color: Tokens.inkFaint
                            font.family: Tokens.jp
                            font.pixelSize: Tokens.fSmall
                        }
                    }

                    // HIDDEN built-ins
                    PickGroup { caption: I18n.tr("HIDDEN"); pool: page.hiddenBuiltins() }
                    // PLUGINS declaring the bar host
                    PickGroup { caption: I18n.tr("PLUGINS"); pool: page.barPlugins() }

                    // the open door: the store, for widgets that are not installed.
                    Rectangle {
                        width: parent.width
                        height: page.tk ? page.tk.rowH : 40
                        radius: Tokens.radius
                        color: storeMa.containsMouse ? Tokens.tint5 : "transparent"
                        border.width: Tokens.border
                        border.color: Tokens.line
                        Behavior on color { ColorAnimation { duration: Tokens.snap } }
                        Row {
                            anchors.left: parent.left
                            anchors.leftMargin: page.tk ? page.tk.gap : 12
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: page.tk ? page.tk.gap / 2 : 6
                            IconText {
                                anchors.verticalCenter: parent.verticalCenter
                                text: "storefront"
                                color: Tokens.inkDim
                                font.pixelSize: Tokens.fBody
                            }
                            UiText {
                                anchors.verticalCenter: parent.verticalCenter
                                text: I18n.tr("GET MORE IN RYOSTORE")
                                color: storeMa.containsMouse ? Tokens.ink : Tokens.inkDim
                                font.family: Tokens.mono
                                font.pixelSize: Tokens.fTiny
                                font.letterSpacing: Tokens.trackLabel
                            }
                        }
                        UiText {
                            anchors.right: parent.right
                            anchors.rightMargin: page.tk ? page.tk.gap : 12
                            anchors.verticalCenter: parent.verticalCenter
                            text: "\u276f"
                            color: Tokens.inkFaint
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fTiny
                        }
                        MouseArea {
                            id: storeMa
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                Spawn.runApp(["ryostore", "open", "plugins"], { label: "ryostore" })
                                page.pickerOpen = false
                            }
                        }
                    }
                }
            }
        }
    }

    // one titled pool inside the picker: HIDDEN or PLUGINS. Each row adds its
    // widget to the bar and lands the selection on it.
    component PickGroup: Column {
        id: pg
        property string caption: ""
        property var pool: []
        width: parent ? parent.width : page.colW
        spacing: page.tk ? page.tk.gap / 2 : 6

        Row {
            spacing: page.tk ? page.tk.gap / 2 : 6
            UiText {
                text: pg.caption
                color: Tokens.inkFaint
                font.family: Tokens.mono
                font.pixelSize: Tokens.fTiny
                font.letterSpacing: Tokens.trackMark
                font.weight: Font.DemiBold
            }
            UiText {
                text: "" + pg.pool.length
                color: Tokens.inkFaint
                font.family: Tokens.mono
                font.pixelSize: Tokens.fTiny
                opacity: 0.7
            }
        }

        UiText {
            visible: pg.pool.length === 0
            width: parent.width
            leftPadding: page.tk ? page.tk.gap : 12
            text: pg.caption === I18n.tr("PLUGINS") ? I18n.tr("No bar plugins installed yet.")
                                                    : I18n.tr("Every built-in is already on the bar.")
            color: Tokens.inkFaint
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
        }

        Repeater {
            model: pg.pool
            delegate: Rectangle {
                id: prow
                required property var modelData
                width: pg.width
                height: page.tk ? page.tk.rowH + (prow.modelData.desc ? page.tk.gap : 0) : 48
                radius: Tokens.radius
                color: prowMa.containsMouse ? Tokens.tint5 : "transparent"
                border.width: Tokens.border
                border.color: Tokens.line
                Behavior on color { ColorAnimation { duration: Tokens.snap } }

                IconText {
                    id: prowGlyph
                    visible: Widgets.glyphFor(prow.modelData.id) !== ""
                    anchors.left: parent.left
                    anchors.leftMargin: page.tk ? page.tk.gap : 12
                    anchors.verticalCenter: parent.verticalCenter
                    text: Widgets.glyphFor(prow.modelData.id)
                    color: Tokens.inkDim
                    font.pixelSize: Tokens.fRow
                }
                Column {
                    anchors.left: prowGlyph.visible ? prowGlyph.right : parent.left
                    anchors.leftMargin: page.tk ? page.tk.gap / (prowGlyph.visible ? 2 : 1) : 12
                    anchors.right: addMark.left
                    anchors.rightMargin: page.tk ? page.tk.gap : 12
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 1
                    Row {
                        spacing: page.tk ? page.tk.gap / 2 : 6
                        UiText {
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr(prow.modelData.label)
                            color: Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fBody
                        }
                        UiText {
                            anchors.verticalCenter: parent.verticalCenter
                            text: UiPreferences.text(prow.modelData.gloss || "", "")
                                + (prow.modelData.category ? "  " + String(prow.modelData.category).toUpperCase() : "")
                            color: Tokens.inkFaint
                            font.family: Tokens.jp
                            font.pixelSize: Tokens.fTiny
                        }
                    }
                    UiText {
                        visible: !!prow.modelData.desc
                        width: parent.width
                        text: I18n.tr(prow.modelData.desc || "")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        elide: Text.ElideRight
                    }
                }
                UiText {
                    id: addMark
                    anchors.right: parent.right
                    anchors.rightMargin: page.tk ? page.tk.gap : 12
                    anchors.verticalCenter: parent.verticalCenter
                    text: "+"
                    color: prowMa.containsMouse ? Tokens.ink : Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fRow
                }
                MouseArea {
                    id: prowMa
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: page.addWidget(prow.modelData.id)
                }
            }
        }
    }
}
