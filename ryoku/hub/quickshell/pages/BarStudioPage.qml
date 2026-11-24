pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../barstudio"
import Ryoku.FrameBars
import "../barstudio/BarStudioModel.js" as Model

// Bar Studio (DESKTOP). Choose which bar the desktop draws, and tune the
// built-in styles. QS Bar keeps its layout, widgets and form in QS Bar Settings
// (the bar logo opens it, or the OPEN QS BAR SETTINGS card here); the universal
// dock has one Stage Editor handoff regardless of bar style. Sumi is
// edited in place: the frame's draw toggle and opacity, each rail's on/off and
// thickness, and the widgets in its three zones (add via a per-zone drawer,
// remove, reorder). Obi and Nacre carry their own small editors.
//
// Everything Sumi stages through the shared draft (hub.stageLive), which applies
// to the RUNNING desktop as you work and rides the Hub's Save and Revert like
// every other framed page.
Item {
    id: page
    property var hub
    property bool subshellsOnly: false
    readonly property bool showActiveDetails: !page.subshellsOnly
        || styleGallery.isSubshellStyle(page.activeStyle)

    // which rail is on the bench
    property string edge: "left"

    // Always normalize what the editor reads. A stored value that lost a subtree
    // (a legacy config, a hand edit, a partial write) would otherwise leave the
    // editor reading undefined and a rail edit cloning the gap straight back to
    // disk. Normalizing restores every subtree from the schema default, so the
    // editor is always whole and the first staged edit heals the store. The
    // probe harness's bare hub has no val(); normalize(null) still yields a
    // complete default, so the page always loads.
    readonly property var config: {
        const v = page.hub && page.hub.val ? page.hub.val("frameBars") : null;
        return FrameBars.normalize(v, BarCatalog, MenuCatalog);
    }
    // the on-disk config, normalized to the same shape so the changed marks and
    // struck defaults compare like against like.
    readonly property var committedBars: {
        const c = page.hub && page.hub.committed ? page.hub.committed.frameBars : null;
        return c ? FrameBars.normalize(c, BarCatalog, MenuCatalog) : null;
    }

    // the selected rail and its on-disk twin, for the rail cells' changed marks
    readonly property var rail: page.config.rails[page.edge]
    readonly property var railWas: page.committedBars && page.committedBars.rails ? page.committedBars.rails[page.edge] : null
    readonly property bool horizontal: page.edge === "top" || page.edge === "bottom"

    // The Obi bar's widgets, for the per-widget show/hide toggles below. Mirrors
    // barstyles/obi/Scene.qml; Workspaces is the bar's identity and has no toggle.
    readonly property var obiWidgets: [
        { id: "activeWindow", label: I18n.tr("Active window"), desc: I18n.tr("The focused window's title, far left.") },
        { id: "resources", label: I18n.tr("Resources"), desc: I18n.tr("CPU and memory rings.") },
        { id: "media", label: I18n.tr("Media"), desc: I18n.tr("Now playing with a music visualizer.") },
        { id: "audio", label: I18n.tr("Audio"), desc: I18n.tr("Output and input volume, with a mixer.") },
        { id: "clock", label: I18n.tr("Clock"), desc: I18n.tr("Time and date.") },
        { id: "connectivity", label: I18n.tr("Connections"), desc: I18n.tr("Wi-Fi and Bluetooth.") },
        { id: "battery", label: I18n.tr("Battery"), desc: I18n.tr("Charge and power profile.") },
        { id: "tray", label: I18n.tr("Tray"), desc: I18n.tr("System tray icons.") },
        { id: "weather", label: I18n.tr("Weather"), desc: I18n.tr("Current conditions.") }
    ]
    // The running bar style; an absent key renders the shipped Orpheus bar,
    // the shell's own default. The frame, rails and zone editors below are
    // Sumi's; a folder style owns its own layout.
    readonly property string activeStyle: page.fval("barStyle", "orpheus")
    readonly property bool sumiActive: page.activeStyle === "sumi"
    readonly property string activeName: {
        const designs = styleGallery.designs || [];
        for (let i = 0; i < designs.length; i++)
            if (designs[i].id === page.activeStyle) return designs[i].name;
        return page.activeStyle;
    }

    // Stage AND apply: edits ride the shared draft like every page, and the
    // hub's stageLive coalesces a settings.patch to the daemon so the running
    // desktop repaints as you work. The probe harness's bare hub has neither;
    // fall back so the page still loads and stages inertly.
    function stage(next) {
        if (!next || !page.hub) return;
        if (page.hub.stageLive) page.hub.stageLive("frameBars", next);
        else if (page.hub.edit) page.hub.edit("frameBars", next);
    }

    // The frame chrome keys live beside frameBars in shell.json: the running
    // shell reads frameEnabled for the draw toggle and frameOpacity for how solid
    // the frame paints, so they belong on the frame's own studio and stage
    // through the same live channel.
    function fval(key, fall) {
        if (!page.hub || !page.hub.val) return fall;
        const v = page.hub.val(key);
        return v === undefined || v === null ? fall : v;
    }
    function fnum(key, fall) {
        const v = Number(page.fval(key, fall));
        return isFinite(v) ? v : fall;
    }
    function fedit(key, value) {
        if (!page.hub) return;
        if (page.hub.stageLive) page.hub.stageLive(key, value);
        else if (page.hub.edit) page.hub.edit(key, value);
    }
    function fwas(key) {
        return page.hub && page.hub.committed ? page.hub.committed[key] : undefined;
    }

    // Obi's per-widget visibility lives in the `obi` map in shell.json (an absent
    // key reads as shown). Toggling stages the whole map live like every edit.
    function obiShown(id) {
        const o = page.fval("obi", ({}));
        return !o || o[id] !== false;
    }
    function obiSet(id, on) {
        const o = Object.assign({}, page.fval("obi", ({})));
        o[id] = on;
        page.fedit("obi", o);
    }

    // ── QS Bar layout summary ─────────────────────────────────────────────────
    // The QS Bar's layout, widgets and form are arranged in QS Bar Settings
    // (the bar logo opens it, or `ryoku-shell bar settings`). This page keeps
    // only a read-only summary of the order, watched off shell.json so it tracks a
    // move made from the panel or the CLI without a Hub reload.
    property var qsbarLayout: ({})
    // The iRiS family's frontend (paper-and-ink "ryoku" or the upstream "inir"
    // look) lives at inir.iris.appearance.frontend in shell.json; watched here so
    // the toggle below tracks a flip made from the iRiS settings overlay too.
    property string irisFrontend: "ryoku"
    readonly property string qsbarLayoutSummary: {
        const layout = page.qsbarLayout || ({});
        const lane = a => Array.isArray(a) ? a.join(" \u00b7 ") : "";
        const lanes = [lane(layout.left), lane(layout.center), lane(layout.right)].filter(s => s.length > 0);
        return lanes.join("  |  ");
    }
    FileView {
        id: shellJsonFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/shell.json"
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: {
            try {
                const cfg = JSON.parse(shellJsonFile.text() || "{}");
                page.qsbarLayout = (cfg.qsbar && cfg.qsbar.layout) ? cfg.qsbar.layout : ({});
                page.irisFrontend = (cfg.inir && cfg.inir.iris && cfg.inir.iris.appearance
                    && cfg.inir.iris.appearance.frontend) || "ryoku";
            } catch (e) {
                page.qsbarLayout = ({});
            }
        }
    }
    function openDockSettings() {
        Quickshell.execDetached(["qs", "-c", "shell", "ipc", "call", "desktop", "editSection", "dock", ""]);
    }

    // Writes through the shell's own config writer (iris set IPC), never the Hub
    // draft, so it does not fight the frame's live Config and applies at once.
    function setIrisFrontend(v) {
        Quickshell.execDetached(["qs", "-c", "shell", "ipc", "call", "iris", "set",
            "iris.appearance.frontend", JSON.stringify(v)]);
    }

    CatalogLabels { id: labels }

    // The active style's own cards, from the studio-card.json beside its
    // Scene.qml (or the Hub's own, for Obi and Nacre). A style without one gets
    // the LAYOUT note below.
    StyleCardSource {
        id: styleCards
        styleId: page.activeStyle
        editors: Object.keys(page.studioEditors)
    }
    function runCardButton(button) {
        if (!button)
            return;
        if (button.hub !== "") {
            if (page.hub && page.hub.navigate)
                page.hub.navigate(button.hub);
            return;
        }
        Spawn.run(button.command);
    }

    // ── head: the eyebrow band, the title, the blurb ─────────────────────────
    Column {
        id: head
        anchors.top: parent.top
        // the head sits on the body's grid: full body width from the left, so
        // the title starts over the first card column instead of floating centred
        x: 0
        width: page.width - 14
        // the register row sits off the title: a rule over a 32px
        // title needs more than the gap between two lines of body text
        spacing: Tokens.s3

        Item {
            width: parent.width
            height: 14
            Row {
                // the register row holds a fixed box, so the rule and the seal keep
                // their distance from the title on every page
                height: Tokens.s5
                id: ebrow
                spacing: Tokens.s2
                anchors.verticalCenter: parent.verticalCenter
                Rectangle { width: 16; height: 1; color: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
                Text {
                    text: UiPreferences.brandMark("力"); color: Tokens.ink; font.family: Tokens.jp
                    font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter
                }
                Text {
                    text: I18n.tr("DESKTOP"); color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                    anchors.verticalCenter: parent.verticalCenter
                }
            }
            Rectangle {
                anchors { left: ebrow.right; right: crossMark.left; verticalCenter: parent.verticalCenter }
                anchors.leftMargin: Tokens.s3; anchors.rightMargin: Tokens.s3
                height: 1; color: Tokens.lineSoft
            }
            Text {
                id: crossMark
                anchors { right: slashMark.left; rightMargin: Tokens.s2; verticalCenter: parent.verticalCenter }
                text: "+"; color: Tokens.inkFaint
                font.family: Tokens.mono; font.pixelSize: 10
            }
            Text {
                id: slashMark
                anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                text: "///"; color: Tokens.inkFaint
                font.family: Tokens.mono; font.pixelSize: 10
            }
        }
        Text {
            text: page.subshellsOnly ? I18n.tr("Subshells") : I18n.tr("Bar Studio")
            color: Tokens.ink
            font.family: Tokens.display
            font.pixelSize: Tokens.fTitle
        }
        // One compact legend serves both Subshells and Bar Studio.
        Text {
            width: parent.width
            text: (page.subshellsOnly
                ? I18n.tr("Complete shell experiences with their own panels, docks and tools.")
                : I18n.tr("Which bar the desktop draws, and how it looks."))
                + "   |   ▲ = " + I18n.tr("Uses more resources than other options")
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fBody
            wrapMode: Text.WordWrap
        }
    }

    // ── the sheet: FRAME, RAILS, WIDGETS in one scroll ───────────────────────
    Flickable {
        id: flick
        anchors { left: parent.left; right: parent.right; top: head.bottom; bottom: parent.bottom; topMargin: Tokens.s5 }
        contentWidth: width
        contentHeight: Math.max(col.height, flick.height)
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        CardColumns {

        id: col
            // a body of cards fills the measure and splits into balanced columns
            width: flick.width - 14
            spacing: Tokens.s5
            fillTo: flick.height

            // ── BAR STYLE: which bar the desktop draws ───────────────────────
            SettingCard {
                id: styleSect
                // The primary choice of the page takes the band across the
                // columns: eight style tiles need the room, and a column-width
                // card crams them two to a row while the page's other half
                // sits empty.
                property bool fullWidth: true
                width: col.colWidth
                title: page.subshellsOnly ? I18n.tr("SUBSHELL") : I18n.tr("BAR STYLE")

                StoreDesignGallery {
                    id: styleGallery
                    width: parent.width
                    category: "barstyles"
                    onlySubshells: page.subshellsOnly
                    browseCategory: page.subshellsOnly ? "subshells" : "barstyles"
                    wornId: page.activeStyle
                    shippedDesigns: []
                    onWear: designId => page.fedit("barStyle", designId)
                }
            }

            SettingCard {
                width: col.colWidth
                title: I18n.tr("DOCK")
                kana: "台"

                Item {
                    width: parent.width
                    height: dockHandoff.height + Tokens.s3 + Tokens.s4
                    Column {
                        id: dockHandoff
                        anchors { left: parent.left; right: parent.right; top: parent.top }
                        anchors.leftMargin: Tokens.s4
                        anchors.rightMargin: Tokens.s4
                        anchors.topMargin: Tokens.s3
                        spacing: Tokens.s3
                        Text {
                            width: parent.width
                            text: I18n.tr("The shared dock is configured in Stage Editor, regardless of which shell or bar is active.")
                            color: Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fBody
                            wrapMode: Text.WordWrap
                        }
                        Btn {
                            text: I18n.tr("OPEN DOCK SETTINGS")
                            onAct: page.openDockSettings()
                        }
                    }
                }
            }

            // ── the active style's declared cards (StyleCardSource) ─────────
            Repeater {
                // positions its cards only; CardColumns counts a visible item
                // as a block, and this one draws nothing itself.
                visible: false
                model: page.showActiveDetails ? styleCards.cards : []
                delegate: DeclaredCard {
                    required property var modelData
                    width: col.colWidth
                    spec: modelData
                    editors: page.studioEditors
                    onAct: button => page.runCardButton(button)
                }
            }

            // A style Bar Studio has no card for: a folder style that keeps its
            // own layout. A style that declares cards replaces this note too.
            SettingCard {
                id: folderNote
                width: col.colWidth
                visible: page.showActiveDetails && !styleCards.declared && !page.sumiActive
                title: I18n.tr("LAYOUT")

                Loader {
                    width: parent.width
                    sourceComponent: layoutNoteEditor
                }
            }
        }
    }

    // ── the editors a declared card places by name ─────────────────────────
    // A style's studio-card.json says where they go (an "editor" block); the
    // state they edit stays on this page. Inline editors sit between a card's
    // text and buttons; an editor that is a card's only block fills its body.
    readonly property var studioEditors: ({
        "qsbar-layout": qsbarLayoutEditor,
        "shima-look": shimaLookEditor,
        "layout-note": layoutNoteEditor,
        "obi-widgets": obiWidgetsEditor,
        "nacre-layout": nacreLayoutEditor,
        "sumi-frame": sumiFrameEditor,
        "sumi-rails": sumiRailsEditor,
        "sumi-widgets": sumiWidgetsEditor
    })

    // QS Bar's layout, read-only: the order its lanes hold, from shell.json.
    Component {
        id: qsbarLayoutEditor
        Text {
            readonly property bool present: page.qsbarLayoutSummary.length > 0
            width: parent.width
            text: page.qsbarLayoutSummary
            color: Tokens.inkDim
            font.family: Tokens.mono
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.WordWrap
        }
    }

    // Shima's look: the ryoku paper-and-ink frontend or the upstream iNiR one.
    Component {
        id: shimaLookEditor
        Item {
            width: parent.width
            height: Tokens.ctlH
            Text {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Look")
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fRow
            }
            Seg {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                options: ["RYOKU", "ORIGINAL"]
                current: page.irisFrontend === "inir" ? "ORIGINAL" : "RYOKU"
                onChose: key => page.setIrisFrontend(key === "ORIGINAL" ? "inir" : "ryoku")
            }
        }
    }

    // A folder style that keeps its own layout, said in one line.
    Component {
        id: layoutNoteEditor
        Text {
            width: parent.width
            leftPadding: Tokens.s4; rightPadding: Tokens.s4
            topPadding: Tokens.s3; bottomPadding: Tokens.s4
            text: I18n.tr("The %1 style manages its own layout in barstyles/%2.").arg(page.activeName).arg(page.activeStyle)
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fBody
            wrapMode: Text.WordWrap
        }
    }

    // OBI WIDGETS: show or hide each widget on the Obi bar.
    Component {
        id: obiWidgetsEditor
        Column {
            Repeater {
                model: page.obiWidgets
                delegate: SettingRow {
                    required property var modelData
                    required property int index
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: index > 0
                    controlWidth: 54
                    label: I18n.tr(modelData.label)
                    desc: I18n.tr(modelData.desc)
                    source: "shell.json"
                    Sw {
                        objectName: "obi-" + modelData.id
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        on: page.obiShown(modelData.id)
                        onToggled: value => page.obiSet(modelData.id, value)
                    }
                }
            }
        }
    }

    // Nacre's islands, lanes and look.
    Component {
        id: nacreLayoutEditor
        Item {
            width: parent.width
            height: nacreEd.height + Tokens.s3 + Tokens.s4
            NacreEditor {
                id: nacreEd
                anchors { left: parent.left; right: parent.right; top: parent.top }
                anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4; anchors.topMargin: Tokens.s3
                config: page.fval("nacre", ({}))
                onStaged: value => page.fedit("nacre", value)
            }
        }
    }

    // FRAME: the chrome the shell draws around the desktop.
    Component {
        id: sumiFrameEditor
        Column {
            id: frameEditor
            SettingRow {
                anchors.left: parent.left
                anchors.right: parent.right
                controlWidth: 54
                label: I18n.tr("Draw frame")
                def: page.fwas("frameEnabled") === undefined ? "" : (page.fwas("frameEnabled") ? I18n.tr("ON") : I18n.tr("OFF"))
                changed: page.fwas("frameEnabled") !== undefined && !!page.fval("frameEnabled", true) !== !!page.fwas("frameEnabled")
                desc: I18n.tr("The bounded band drawn around the desktop.")
                source: "shell.json"
                Sw {
                    objectName: "frame-enabled"
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    on: !!page.fval("frameEnabled", true)
                    onToggled: value => page.fedit("frameEnabled", value)
                }
            }
            SettingRow {
                anchors.left: parent.left
                anchors.right: parent.right
                divider: true
                controlWidth: Math.min(240, Math.max(160, Math.round(frameEditor.width * 0.34)))
                label: I18n.tr("Opacity")
                unit: "%"
                value: String(Math.round(page.fnum("frameOpacity", 1) * 100))
                def: page.fwas("frameOpacity") === undefined ? "" : String(Math.round(Number(page.fwas("frameOpacity")) * 100))
                changed: page.fwas("frameOpacity") !== undefined && page.fnum("frameOpacity", 1) !== Number(page.fwas("frameOpacity"))
                desc: I18n.tr("How solid the frame draws.")
                source: "shell.json"
                Slid {
                    objectName: "frame-opacity"
                    anchors.fill: parent
                    from: 0.5; to: 1.0
                    value: page.fnum("frameOpacity", 1)
                    onModified: value => page.fedit("frameOpacity", value)
                }
            }
            SettingRow {
                anchors.left: parent.left
                anchors.right: parent.right
                divider: true
                controlWidth: 58
                label: I18n.tr("Thickness")
                unit: "px"
                value: String(page.fnum("frameThickness", 2))
                def: page.fwas("frameThickness") === undefined ? "" : String(page.fwas("frameThickness"))
                changed: page.fwas("frameThickness") !== undefined && page.fnum("frameThickness", 2) !== Number(page.fwas("frameThickness"))
                desc: I18n.tr("How far the band stands into the screen.")
                source: "shell.json"
                Step {
                    objectName: "frame-thickness"
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    from: 0; to: 24
                    value: page.fnum("frameThickness", 2)
                    onModified: value => page.fedit("frameThickness", value)
                }
            }
            SettingRow {
                anchors.left: parent.left
                anchors.right: parent.right
                divider: true
                controlWidth: 58
                label: I18n.tr("Corner radius")
                unit: "px"
                value: String(page.fnum("frameCorner", 8))
                def: page.fwas("frameCorner") === undefined ? "" : String(page.fwas("frameCorner"))
                changed: page.fwas("frameCorner") !== undefined && page.fnum("frameCorner", 8) !== Number(page.fwas("frameCorner"))
                desc: I18n.tr("How round the frame cuts the screen's corners.")
                source: "shell.json"
                Step {
                    objectName: "frame-corner"
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    from: 0; to: 40
                    value: page.fnum("frameCorner", 8)
                    onModified: value => page.fedit("frameCorner", value)
                }
            }
        }
    }

    // RAILS: pick an edge, then its own switches.
    Component {
        id: sumiRailsEditor
        Column {
            id: railsEditor
            Item {
                width: parent.width
                height: railRow.height + Tokens.s3 + Tokens.s3
                Row {
                    id: railRow
                    anchors { left: parent.left; right: parent.right; top: parent.top }
                    anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4; anchors.topMargin: Tokens.s3
                    spacing: Tokens.s2
                    Repeater {
                        model: ["left"]
                        delegate: Rectangle {
                            id: plate
                            required property string modelData
                            readonly property var pRail: page.config.rails[plate.modelData]
                            readonly property int count: {
                                const zs = plate.modelData === "top" || plate.modelData === "bottom" ? ["start", "center", "end"] : ["top", "center", "bottom"];
                                let n = 0;
                                for (const zone of zs) n += (plate.pRail[zone] || []).length;
                                return n;
                            }
                            readonly property bool on: page.edge === plate.modelData

                            objectName: "rail-edge-" + plate.modelData
                            width: (railRow.width - 3 * Tokens.s2) / 4
                            height: 48
                            radius: Tokens.radius
                            color: plate.on ? Tokens.bone : (pma.containsMouse ? Tokens.tint5 : "transparent")
                            border.width: Tokens.border
                            border.color: plate.on ? Tokens.bone : Tokens.line
                            Behavior on color { ColorAnimation { duration: Tokens.snap } }

                            Column {
                                anchors { left: parent.left; leftMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                                spacing: 2
                                Text {
                                    text: labels.edge(plate.modelData).toUpperCase()
                                    color: plate.on ? Tokens.inkOnBone : Tokens.inkDim
                                    font.family: Tokens.ui
                                    font.pixelSize: 11
                                    font.weight: Font.Medium
                                    font.letterSpacing: Tokens.trackLabel
                                }
                                Text {
                                    text: plate.pRail.enabled ? I18n.tr("on · %1").arg(plate.count) : I18n.tr("off")
                                    color: plate.on ? Tokens.inkOnBoneDim : Tokens.inkFaint
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fTiny
                                }
                            }
                            MouseArea {
                                id: pma
                                anchors.fill: parent
                                hoverEnabled: true
                                preventStealing: true
                                cursorShape: Qt.PointingHandCursor
                                onClicked: page.edge = plate.modelData
                            }
                        }
                    }
                }
            }

            SettingRow {
                anchors.left: parent.left
                anchors.right: parent.right
                divider: true
                controlWidth: 54
                label: I18n.tr("Show this rail")
                def: page.railWas ? (page.railWas.enabled ? I18n.tr("ON") : I18n.tr("OFF")) : ""
                changed: !!page.railWas && page.rail.enabled !== page.railWas.enabled
                desc: I18n.tr("Draw the %1 rail on the frame.").arg(labels.edge(page.edge).toLowerCase())
                source: "shell.json"
                Sw {
                    objectName: "rail-enabled"
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    on: page.rail.enabled
                    onToggled: value => page.stage(Model.setRail(page.config, page.edge, { enabled: value }))
                }
            }
            SettingRow {
                anchors.left: parent.left
                anchors.right: parent.right
                divider: true
                controlWidth: Math.min(240, Math.max(160, Math.round(railsEditor.width * 0.34)))
                label: I18n.tr("Thickness")
                unit: "px"
                value: String(page.rail.size)
                def: page.railWas ? String(page.railWas.size) : ""
                changed: !!page.railWas && page.rail.size !== page.railWas.size
                desc: I18n.tr("How far the %1 rail stands into the screen.").arg(labels.edge(page.edge).toLowerCase())
                source: "shell.json"
                Slid {
                    objectName: "rail-thickness"
                    anchors.fill: parent
                    from: page.horizontal ? 16 : 24
                    to: page.horizontal ? 96 : 112
                    value: page.rail.size
                    onModified: value => page.stage(Model.setRail(page.config, page.edge, { size: value }))
                }
            }
        }
    }

    // WIDGETS: the selected rail's three zones and its add drawers.
    Component {
        id: sumiWidgetsEditor
        Item {
        readonly property string cardTitle: I18n.tr("WIDGETS ON THE %1 RAIL").arg(labels.edge(page.edge).toUpperCase())
            width: parent.width
            height: zoneEd.height + Tokens.s3 + Tokens.s4
            ZoneEditor {
                id: zoneEd
                anchors { left: parent.left; right: parent.right; top: parent.top }
                anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4; anchors.topMargin: Tokens.s3
                config: page.config
                edge: page.edge
                catalog: BarCatalog
                onStaged: next => page.stage(next)
            }
        }
    }
}
