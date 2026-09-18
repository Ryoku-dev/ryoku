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
// built-in styles. QS Bar is a folder style that keeps its own layout, widgets,
// form and dock in QS Bar Settings (the bar logo opens it, or the OPEN QS BAR
// SETTINGS card here); this page shows only a live summary of its order. Sumi is
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

    property var barStyles: []

    function browseBarStyles() {
        Quickshell.execDetached(["ryostore", "open", "barstyles"]);
    }

    Process {
        id: styleProc
        command: ["ryostore", "catalog", "--category", "barstyles"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    const catalog = JSON.parse(this.text || "{}");
                    page.barStyles = (catalog.items || [])
                        .filter(item => item.category === "barstyles" && item.installed === true)
                        .map(item => ({
                            id: item.id,
                            name: item.name || item.id,
                            desc: item.summary || item.description || "",
                            active: item.active === true
                        }));
                } catch (e) {
                    page.barStyles = [];
                }
            }
        }
    }

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

    readonly property var chromaWidgets: [
        { id: "identity", label: I18n.tr("Launcher"), desc: I18n.tr("The Chroma identity button and app launcher.") },
        { id: "workspaces", label: I18n.tr("Workspaces"), desc: I18n.tr("The numbered workspace rail.") },
        { id: "media", label: I18n.tr("Media"), desc: I18n.tr("Now playing, progress and playback spectrum.") },
        { id: "notifications", label: I18n.tr("Notifications"), desc: I18n.tr("Notification state and unread count.") },
        { id: "wallpaper", label: I18n.tr("Wallpaper"), desc: I18n.tr("Shortcut to wallpaper and theme controls.") },
        { id: "network", label: I18n.tr("Network"), desc: I18n.tr("Current wired or wireless network state.") },
        { id: "audio", label: I18n.tr("Audio"), desc: I18n.tr("Output volume and scroll volume control.") },
        { id: "battery", label: I18n.tr("Battery"), desc: I18n.tr("Battery state when a battery is present.") },
        { id: "settings", label: I18n.tr("Quick settings"), desc: I18n.tr("Shortcut to the Ryoku quick-settings surface.") },
        { id: "clock", label: I18n.tr("Clock"), desc: I18n.tr("Time and date block.") }
    ]
    // The running bar style, default the built-in frame style. The frame, rails
    // and zone editors below are Sumi's; a folder style owns its own layout.
    readonly property string activeStyle: page.fval("barStyle", "sumi")
    readonly property bool sumiActive: page.activeStyle === "sumi"
    readonly property string activeName: {
        for (let i = 0; i < page.barStyles.length; i++)
            if (page.barStyles[i].id === page.activeStyle) return page.barStyles[i].name;
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

    function chromaConfig() {
        return Object.assign({}, page.fval("chroma", ({})));
    }
    function chromaScale() {
        const n = Number(page.chromaConfig().scale);
        return isFinite(n) ? Math.max(0.6, Math.min(1.4, n)) : 1;
    }
    function chromaSetScale(value) {
        const cfg = page.chromaConfig();
        cfg.scale = Math.round(Math.max(0.6, Math.min(1.4, Number(value))) * 20) / 20;
        page.fedit("chroma", cfg);
    }
    function chromaShown(id) {
        const widgets = page.chromaConfig().widgets || ({});
        return widgets[id] !== false;
    }
    function chromaSetWidget(id, on) {
        const cfg = page.chromaConfig();
        const widgets = Object.assign({}, cfg.widgets || ({}));
        widgets[id] = on;
        cfg.widgets = widgets;
        page.fedit("chroma", cfg);
    }

    // ── QS Bar layout summary ─────────────────────────────────────────────────
    // The QS Bar's layout, widgets, form and dock are arranged in QS Bar Settings
    // now (the bar logo opens it, or `ryoku-shell bar settings`). This page keeps
    // only a read-only summary of the order, watched off shell.json so it tracks a
    // move made from the panel or the CLI without a Hub reload.
    property var qsbarLayout: ({})
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
            } catch (e) {
                page.qsbarLayout = ({});
            }
        }
    }
    function openQsBarSettings() {
        Quickshell.execDetached(["ryoku-shell", "bar", "settings"]);
    }

    CatalogLabels { id: labels }

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
                    text: "力"; color: Tokens.ink; font.family: Tokens.jp
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
            text: I18n.tr("Bar Studio")
            color: Tokens.ink
            font.family: Tokens.display
            font.pixelSize: Tokens.fTitle
        }
        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("Which bar the desktop draws, and how it looks.")
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
                title: I18n.tr("BAR STYLE")

                Item {
                    width: parent.width
                    height: styleBody.height + Tokens.s3 + Tokens.s4
                    Column {
                        id: styleBody
                        anchors { left: parent.left; right: parent.right; top: parent.top }
                        anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4; anchors.topMargin: Tokens.s3
                        spacing: Tokens.s3
                        // A gallery, not a filmstrip: with a bar style per tile the
                        // captions are what tell them apart, so the tiles wrap into
                        // as many rows as they need instead of squeezing 8 into one.
                        // The height is computed, not measured: a Flow reports its
                        // post-wrap height a polish pass late, and CardColumns would
                        // place the next card over this one's second row.
                        Flow {
                            id: styleRow
                            width: parent.width
                            spacing: Tokens.s2
                            readonly property int perRow: Math.max(2, Math.floor((width + Tokens.s2) / 210))
                            readonly property int tileH: 64
                            readonly property int rowCount: Math.max(1, Math.ceil(styleRep.count / perRow))
                            height: rowCount * tileH + (rowCount - 1) * spacing
                            Repeater {
                                id: styleRep
                                model: page.barStyles
                                delegate: Rectangle {
                                    id: styleCard
                                    required property var modelData
                                    readonly property bool on: page.activeStyle === styleCard.modelData.id

                                    objectName: "bar-style-" + styleCard.modelData.id
                                    // fill the row evenly: as many ~210px tiles as the
                                    // measure holds, stretched to close the last gap.
                                    width: Math.floor((styleRow.width - (styleRow.perRow - 1) * Tokens.s2) / styleRow.perRow)
                                    height: styleRow.tileH
                                    radius: Tokens.radius
                                    color: styleCard.on ? Tokens.bone : (sma.containsMouse ? Tokens.tint5 : "transparent")
                                    border.width: Tokens.border
                                    border.color: styleCard.on ? Tokens.bone : Tokens.line
                                    Behavior on color { ColorAnimation { duration: Tokens.snap } }

                                    Column {
                                        anchors { left: parent.left; right: parent.right; margins: Tokens.s3; verticalCenter: parent.verticalCenter }
                                        spacing: 3
                                        Text {
                                            text: styleCard.modelData.name.toUpperCase()
                                            color: styleCard.on ? Tokens.inkOnBone : Tokens.inkDim
                                            font.family: Tokens.ui
                                            font.pixelSize: 12
                                            font.weight: Font.Medium
                                            font.letterSpacing: Tokens.trackLabel
                                        }
                                        Text {
                                            width: parent.width
                                            text: I18n.tr(styleCard.modelData.desc)
                                            color: styleCard.on ? Tokens.inkOnBoneDim : Tokens.inkFaint
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fTiny
                                            elide: Text.ElideRight
                                        }
                                    }
                                    MouseArea {
                                        id: sma
                                        anchors.fill: parent
                                        hoverEnabled: true
                                        preventStealing: true
                                        cursorShape: Qt.PointingHandCursor
                                        onClicked: page.fedit("barStyle", styleCard.modelData.id)
                                    }
                                }
                            }
                        }
                        Btn {
                            text: I18n.tr("BROWSE RYOSTORE")
                            onAct: page.browseBarStyles()
                        }
                    }
                }
            }

            // ── QS BAR: its layout, widgets, form and dock live in QS Bar Settings
            SettingCard {
                id: qsbarSect
                width: col.colWidth
                visible: page.activeStyle === "qsbar"
                title: I18n.tr("QS BAR")
                kana: "帯"

                Item {
                    width: parent.width
                    height: qsbarBody.height + Tokens.s3 + Tokens.s4
                    Column {
                        id: qsbarBody
                        anchors { left: parent.left; right: parent.right; top: parent.top }
                        anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4; anchors.topMargin: Tokens.s3
                        spacing: Tokens.s3
                        Text {
                            width: parent.width
                            text: I18n.tr("The QS Bar arranges its own layout, widgets, form and dock in QS Bar Settings. The bar logo opens it, or the button below.")
                            color: Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fBody
                            wrapMode: Text.WordWrap
                        }
                        Text {
                            width: parent.width
                            visible: page.qsbarLayoutSummary.length > 0
                            text: page.qsbarLayoutSummary
                            color: Tokens.inkDim
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }
                        Btn {
                            text: I18n.tr("OPEN QS BAR SETTINGS")
                            onAct: page.openQsBarSettings()
                        }
                    }
                }
            }

            SettingCard {
                id: chromaSect
                width: col.colWidth
                visible: page.activeStyle === "chroma"
                title: I18n.tr("CHROMA")

                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    label: I18n.tr("Size")
                    desc: I18n.tr("Scale Chroma without changing the display scale.")
                    source: "shell.json"
                    unit: "%"
                    value: String(Math.round(page.chromaScale() * 100))
                    controlWidth: Math.min(240, Math.max(160, Math.round(chromaSect.width * 0.34)))
                    Slid {
                        anchors.fill: parent
                        from: 0.6
                        to: 1.4
                        value: page.chromaScale()
                        onModified: value => page.chromaSetScale(value)
                    }
                }

                Repeater {
                    model: page.chromaWidgets
                    delegate: SettingRow {
                        required property var modelData
                        required property int index
                        anchors.left: parent.left
                        anchors.right: parent.right
                        divider: true
                        controlWidth: 54
                        label: I18n.tr(modelData.label)
                        desc: I18n.tr(modelData.desc)
                        source: "shell.json"
                        Sw {
                            objectName: "chroma-" + modelData.id
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            on: page.chromaShown(modelData.id)
                            onToggled: value => page.chromaSetWidget(modelData.id, value)
                        }
                    }
                }
            }

            // A folder style owns its own frame, rails and widgets inside its
            // barstyles/<id>/ folder, so the Sumi editors below stand down.
            SettingCard {
                id: folderNote
                width: col.colWidth
                visible: !page.sumiActive && page.activeStyle !== "qsbar" && page.activeStyle !== "chroma"
                title: I18n.tr("LAYOUT")

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
            SettingCard {
                id: obiSect
                width: col.colWidth
                visible: page.activeStyle === "obi"
                title: I18n.tr("OBI WIDGETS")

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

            SettingCard {
                id: nacreSect
                width: col.colWidth
                visible: page.activeStyle === "nacre"
                title: I18n.tr("NACRE LAYOUT")

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

            // ── FRAME: the chrome the shell draws around the desktop ─────────
            SettingCard {
                id: frameSect
                width: col.colWidth
                title: I18n.tr("FRAME")
                visible: page.sumiActive

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
                    controlWidth: Math.min(240, Math.max(160, Math.round(frameSect.width * 0.34)))
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

            // ── RAILS: pick an edge, then its own switches ───────────────────
            SettingCard {
                id: railSect
                width: col.colWidth
                title: I18n.tr("RAILS")
                visible: page.sumiActive

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
                    controlWidth: Math.min(240, Math.max(160, Math.round(railSect.width * 0.34)))
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

            // ── WIDGETS: the selected rail's three zones and its add drawers ──
            SettingCard {
                id: zoneSect
                width: col.colWidth
                title: I18n.tr("WIDGETS ON THE %1 RAIL").arg(labels.edge(page.edge).toUpperCase())
                visible: page.sumiActive

                Item {
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
    }

}
