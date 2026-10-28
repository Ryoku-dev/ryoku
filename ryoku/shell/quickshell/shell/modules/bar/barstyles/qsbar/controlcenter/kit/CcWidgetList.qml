import QtQuick
import "Widgets.js" as Widgets
import "../../modules"
import Ryoku.Ui
import Ryoku.Ui.Singletons

// The widget sheet both the Widgets and the Community routes are built from: one
// titled card of rows, one per catalogue entry handed in through `entries`. A
// row carries the widget's glyph, its name and gloss, a PLUGIN tag when it is
// one, and an on/off switch; selecting it expands to its density (icon only), its
// per-widget colour, its own settings from the catalogue rendered by type
// through the root barWidget seam, and, for a community plugin, its author line
// and its EXPORT / SHARE / REMOVE actions. The colour popover and the plugin
// CLI runs live at route level, so the sheet only asks for them (signals).
SettingCard {
    id: list
    property var root: null
    property var tk: null
    property var entries: []
    property string selId: ""      // the expanded widget row
    property string colorGid: ""   // the widget whose colour popover the route has open
    signal colorRequested(string gid, string label)
    // the route runs these through the plugin CLI and shows the result.
    signal exportRequested(string id)
    signal shareRequested(string id)
    signal removeRequested(string id)

    // Per-widget colour is gated on widgetHasFill so the sheet stays error-free if
    // the live Theme (or the offscreen probe) does not expose the helpers.
    readonly property bool colorSupported: !!(list.root && list.root.widgetHasFill)

    // Every setting value goes through the root barWidget seam. Reads are reactive
    // because barWidgetGet reads a live Theme property (or pluginSettings) during
    // evaluation, so a control reflects a change without a manual refresh.
    function getW(id, key) {
        return (list.root && list.root.barWidgetGet) ? list.root.barWidgetGet(id, key) : undefined
    }
    function setW(id, key, val) {
        if (list.root && list.root.barWidgetSet) list.root.barWidgetSet(id, key, val)
    }
    function showW(id, on) {
        if (list.root && list.root.barLayoutShow) list.root.barLayoutShow(id, on)
    }
    // density (icon-only) is keyed by gid, not id.
    function isIcon(gid) { return !!(list.root && list.root.iconOnly && list.root.iconOnly(gid)) }
    function toggleDensity(gid) { if (list.root && list.root.toggleIconOnly) list.root.toggleIconOnly(gid) }
    // one catalogue setting, rendered by type; options may be plain strings
    // (built-in) or {value,label} objects (a plugin manifest).
    component WSetting: Column {
        id: ws
        property string wid: ""
        property var setting: null
        readonly property string skey: ws.setting ? String(ws.setting.key) : ""
        readonly property string stype: ws.setting ? String(ws.setting.type) : ""
        readonly property var opts: ws.optionValues(ws.setting)
        readonly property var optLabels: ws.optionLabelMap(ws.setting)
        readonly property bool allStrings: ws.optionsAllStrings(ws.setting)

        width: parent ? parent.width : 0
        spacing: list.tk ? list.tk.gap / 2 : 6

        function optionValues(s) {
            var o = (s && s.options) ? s.options : []
            var out = []
            for (var i = 0; i < o.length; i++)
                out.push((o[i] && typeof o[i] === "object") ? String(o[i].value) : String(o[i]))
            return out
        }
        function optionLabelMap(s) {
            var o = (s && s.options) ? s.options : []
            var m = ({})
            for (var i = 0; i < o.length; i++)
                if (o[i] && typeof o[i] === "object") m[String(o[i].value)] = String(o[i].label)
            return m
        }
        function optionsAllStrings(s) {
            var o = (s && s.options) ? s.options : []
            for (var i = 0; i < o.length; i++)
                if (o[i] && typeof o[i] === "object") return false
            return true
        }

        UiText {
            text: I18n.tr(ws.setting ? ws.setting.label : "")
            color: Tokens.inkMuted
            font.family: Tokens.mono
            font.pixelSize: Tokens.fTiny
            font.letterSpacing: Tokens.trackLabel
        }

        Component {
            id: segK
            Seg {
                options: ws.opts
                current: { var v = list.getW(ws.wid, ws.skey); return v === undefined ? "" : String(v) }
                onChose: (k) => list.setW(ws.wid, ws.skey, k)
            }
        }
        Component {
            id: chipsK
            Chips {
                width: ws.width
                options: ws.opts
                labels: ws.optLabels
                current: { var v = list.getW(ws.wid, ws.skey); return v === undefined ? "" : String(v) }
                onChose: (k) => list.setW(ws.wid, ws.skey, k)
            }
        }
        Component {
            id: swK
            Sw {
                on: list.getW(ws.wid, ws.skey) === true
                onToggled: (v) => list.setW(ws.wid, ws.skey, v)
            }
        }
        Component {
            id: multiK
            Multi {
                width: ws.width
                options: ws.opts
                chosen: { var v = list.getW(ws.wid, ws.skey); return Array.isArray(v) ? v : [] }
                onToggled: (k) => {
                    var cur = list.getW(ws.wid, ws.skey)
                    var arr = Array.isArray(cur) ? cur.slice() : []
                    var i = arr.indexOf(k)
                    if (i >= 0) arr.splice(i, 1); else arr.push(k)
                    list.setW(ws.wid, ws.skey, arr)
                }
            }
        }
        Component {
            id: stepK
            // the number sits beside its stepper: a Step alone shows no readout,
            // and a plugin's int setting has no SettingRow value slot to use.
            Row {
                spacing: list.tk ? list.tk.gap : 12
                UiText {
                    anchors.verticalCenter: parent.verticalCenter
                    text: String(stepCtl.value)
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fBody
                    font.weight: Font.Light
                }
                Step {
                    id: stepCtl
                    anchors.verticalCenter: parent.verticalCenter
                    from: (ws.setting && ws.setting.min !== undefined) ? ws.setting.min : 0
                    to: (ws.setting && ws.setting.max !== undefined) ? ws.setting.max : 100
                    value: { var v = Number(list.getW(ws.wid, ws.skey)); return isNaN(v) ? from : v }
                    onModified: (v) => list.setW(ws.wid, ws.skey, v)
                }
            }
        }
        Component {
            id: fieldK
            Field {
                width: Math.min(ws.width, 260)
                tabular: true
                text: { var v = list.getW(ws.wid, ws.skey); return v === undefined ? "" : String(v) }
                onCommitted: (v) => list.setW(ws.wid, ws.skey, v)
            }
        }

        // Only the controls that lay out across the column take its width; a
        // switch, a segment bar or a stepper keep their natural size (a Loader
        // with a width forces the loaded item to it, which stretched a switch
        // across the whole row).
        Loader {
            readonly property bool wide: ws.stype === "multi" || ws.stype === "text"
                || (ws.stype === "choice" && !(ws.allStrings && ws.opts.length <= 4))
            width: wide ? ws.width : (item ? item.implicitWidth : 0)
            sourceComponent: {
                switch (ws.stype) {
                case "toggle": return swK
                case "multi":  return multiK
                case "int":    return stepK
                case "text":   return fieldK
                case "choice": return (ws.allStrings && ws.opts.length <= 4) ? segK : chipsK
                default:       return null
                }
            }
        }
    }

    Repeater {
        id: rep
        model: list.entries

        delegate: Column {
            id: wr
            required property var modelData
            required property int index
            readonly property string wid: wr.modelData.id
            readonly property string gid: wr.modelData.gid || ""
            readonly property bool isPlugin: wr.modelData.kind === "plugin"
            readonly property bool expanded: list.selId === wr.wid
            readonly property var settings: wr.modelData.settings || []
            readonly property string glyph: Widgets.glyphFor(wr.wid)
            width: parent ? parent.width : 0

            // ── the row header ──
            Rectangle {
                width: parent.width
                height: list.tk ? list.tk.rowH : 40
                color: (headMa.containsMouse || wr.expanded) ? Tokens.tint5 : "transparent"
                Behavior on color { ColorAnimation { duration: Tokens.snap } }

                Rectangle {
                    visible: wr.index > 0
                    anchors { top: parent.top; left: parent.left; right: parent.right }
                    anchors.leftMargin: list.tk ? list.tk.pad : 24
                    anchors.rightMargin: list.tk ? list.tk.pad : 24
                    height: 1
                    color: Tokens.lineSoft
                }

                IconText {
                    id: rowGlyph
                    visible: wr.glyph !== ""
                    anchors.left: parent.left
                    anchors.leftMargin: list.tk ? list.tk.pad : 24
                    anchors.verticalCenter: parent.verticalCenter
                    text: wr.glyph
                    color: wr.modelData.shown ? Tokens.inkDim : Tokens.inkFaint
                    font.pixelSize: Tokens.fRow
                }
                UiText {
                    id: rowInitial
                    visible: wr.glyph === ""
                    anchors.left: parent.left
                    anchors.leftMargin: list.tk ? list.tk.pad : 24
                    anchors.verticalCenter: parent.verticalCenter
                    text: Widgets.initialFor(wr.modelData.label)
                    color: wr.modelData.shown ? Tokens.inkDim : Tokens.inkFaint
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fBody
                    font.weight: Font.DemiBold
                }
                Row {
                    anchors.left: (wr.glyph !== "" ? rowGlyph.right : rowInitial.right)
                    anchors.leftMargin: list.tk ? list.tk.gap : 12
                    anchors.right: tag.left
                    anchors.rightMargin: list.tk ? list.tk.gap : 12
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: list.tk ? list.tk.gap / 2 : 6
                    UiText {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr(wr.modelData.label)
                        color: Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fRow
                        font.weight: Font.Medium
                    }
                    UiText {
                        anchors.verticalCenter: parent.verticalCenter
                        text: UiPreferences.text(wr.modelData.gloss || "", "")
                        color: Tokens.inkFaint
                        font.family: Tokens.jp
                        font.pixelSize: Tokens.fSmall
                    }
                }
                UiText {
                    id: tag
                    anchors.right: caret.left
                    anchors.rightMargin: list.tk ? list.tk.gap : 12
                    anchors.verticalCenter: parent.verticalCenter
                    visible: wr.isPlugin
                    text: I18n.tr("PLUGIN")
                    color: Tokens.inkFaint
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fTiny
                    font.letterSpacing: Tokens.trackLabel
                }
                UiText {
                    id: caret
                    anchors.right: sw.left
                    anchors.rightMargin: list.tk ? list.tk.gap : 12
                    anchors.verticalCenter: parent.verticalCenter
                    text: "\u276f"
                    rotation: wr.expanded ? 90 : 0
                    color: Tokens.inkFaint
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fTiny
                    Behavior on rotation { NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap } }
                }
                Sw {
                    id: sw
                    anchors.right: parent.right
                    anchors.rightMargin: list.tk ? list.tk.pad : 24
                    anchors.verticalCenter: parent.verticalCenter
                    enabled: Widgets.hideable(wr.wid)
                    on: wr.modelData.shown === true
                    onToggled: (v) => list.showW(wr.wid, v)
                }

                // expand on a click anywhere left of the switch.
                MouseArea {
                    id: headMa
                    anchors.left: parent.left
                    anchors.right: sw.left
                    anchors.top: parent.top
                    anchors.bottom: parent.bottom
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: list.selId = wr.expanded ? "" : wr.wid
                }
            }

            // ── the expansion ──
            Item {
                width: parent.width
                clip: true
                height: wr.expanded ? detail.implicitHeight + (list.tk ? list.tk.gap * 2 : 24) : 0
                Behavior on height { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }

                Column {
                    id: detail
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    anchors.topMargin: list.tk ? list.tk.gap : 12
                    anchors.leftMargin: list.tk ? list.tk.pad : 24
                    anchors.rightMargin: list.tk ? list.tk.pad : 24
                    spacing: list.tk ? list.tk.gap : 12

                    // density (icon-only), for the widgets that have it.
                    Column {
                        width: parent.width
                        visible: Widgets.densitySupported(wr.wid) && wr.gid !== ""
                        spacing: list.tk ? list.tk.gap / 2 : 6
                        UiText {
                            text: I18n.tr("DENSITY")
                            color: Tokens.inkMuted
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fTiny
                            font.letterSpacing: Tokens.trackLabel
                        }
                        Seg {
                            options: ["Full", "Icon"]
                            current: list.isIcon(wr.gid) ? "Icon" : "Full"
                            onChose: (k) => { if ((k === "Icon") !== list.isIcon(wr.gid)) list.toggleDensity(wr.gid) }
                        }
                    }

                    // per-widget colour: opens the colour popover.
                    Column {
                        width: parent.width
                        visible: list.colorSupported && wr.gid !== ""
                        spacing: list.tk ? list.tk.gap / 2 : 6
                        UiText {
                            text: I18n.tr("COLOUR")
                            color: Tokens.inkMuted
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fTiny
                            font.letterSpacing: Tokens.trackLabel
                        }
                        Row {
                            spacing: list.tk ? list.tk.gap : 12
                            Rectangle {
                                id: swatch
                                readonly property bool assigned: list.colorSupported && list.root.widgetHasFill(wr.gid)
                                readonly property bool menuOpen: list.colorGid === wr.gid   // set by the route while its popover is open
                                width: Tokens.ctlH
                                height: Tokens.ctlH
                                radius: Tokens.radius
                                color: swatch.assigned ? list.root.widgetAssignedColor(wr.gid)
                                    : (swatchMa.containsMouse ? Tokens.tint5 : "transparent")
                                border.width: (swatch.menuOpen || swatch.assigned) ? 2 : 1
                                border.color: (swatch.menuOpen || swatch.assigned) ? Tokens.bone
                                    : (swatchMa.containsMouse ? Tokens.ink : Tokens.line)
                                Behavior on color { ColorAnimation { duration: Tokens.snap } }
                                Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
                                IconText {
                                    anchors.centerIn: parent
                                    visible: !swatch.assigned
                                    text: "palette"
                                    color: swatchMa.containsMouse ? Tokens.inkMuted : Tokens.inkFaint
                                    font.pixelSize: Tokens.fSmall
                                }
                                MouseArea {
                                    id: swatchMa
                                    anchors.fill: parent
                                    hoverEnabled: true
                                    cursorShape: Qt.PointingHandCursor
                                    onClicked: list.colorRequested(wr.gid, wr.modelData.label)
                                }
                            }
                            UiText {
                                anchors.verticalCenter: parent.verticalCenter
                                text: swatch.assigned ? I18n.tr("Tinted. Tap to change.") : I18n.tr("Give this widget its own accent.")
                                color: Tokens.inkFaint
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall
                            }
                        }
                    }

                    // the widget's own catalogue settings.
                    Repeater {
                        model: wr.settings
                        delegate: WSetting {
                            required property var modelData
                            wid: wr.wid
                            setting: modelData
                        }
                    }

                    // a community plugin: who wrote it, and its three doors: out
                    // (REMOVE), to a folder (EXPORT), to the catalogue (SHARE).
                    Column {
                        visible: wr.isPlugin && !wr.modelData.official
                        width: parent.width
                        spacing: list.tk ? list.tk.gap / 2 : 6
                        UiText {
                            width: parent.width
                            text: (wr.modelData.author ? wr.modelData.author : I18n.tr("Unknown author"))
                                + (wr.modelData.version ? "  v" + wr.modelData.version : "")
                            color: Tokens.inkFaint
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fTiny
                            elide: Text.ElideRight
                        }
                        Flow {
                            width: parent.width
                            spacing: list.tk ? list.tk.gap / 2 : 6
                            Btn {
                                text: I18n.tr("EXPORT")
                                onAct: list.exportRequested(wr.wid)
                            }
                            Btn {
                                text: I18n.tr("SHARE TO RYOSTORE")
                                onAct: list.shareRequested(wr.wid)
                            }
                            Btn {
                                text: I18n.tr("REMOVE")
                                onAct: list.removeRequested(wr.wid)
                            }
                        }
                    }

                }
            }
        }
    }
}
