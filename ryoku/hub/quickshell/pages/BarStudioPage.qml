pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import ".."
import "../barstudio"
import Ryoku.FrameBars
import "../barstudio/BarStudioModel.js" as Model
import "../ShellIndex.js" as ShellIndex

// Bar Studio chooses the active shell item and routes its declared settings.
// Built-in editors are selected by item metadata rather than item ids.
Item {
    id: page
    property var hub
    property bool subshellsOnly: false

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
    // the Obi item's qml/Scene.qml; Workspaces is the bar's identity and has no toggle.
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
    readonly property var shellIndex: shellsIndex.index
    readonly property string activeStyle: String(page.fval("barStyle", "") || "")
        || page.shellIndex.fallback
    // The index knows installed items; a catalogue card covers a missing index.
    readonly property var activeItem: ShellIndex.item(page.shellIndex, page.activeStyle)
        || page.galleryCard(page.activeStyle)
    readonly property var activeSettings: page.activeItem && page.activeItem.settings
        ? page.activeItem.settings : ({})
    readonly property string activeEditor: String(page.activeSettings.hubEditor || "")
    readonly property string activeName: page.activeItem
        ? String(page.activeItem.name || page.activeItem.id) : page.activeStyle
    readonly property string activeVersion: page.activeItem
        ? String(page.activeItem.version || "") : ""
    readonly property string activeSource: page.activeItem
        ? String(page.activeItem.source || "store") : ""
    readonly property bool activeAllowed: !page.subshellsOnly
        || (!!page.activeItem && page.activeItem.category === "subshells")
    readonly property bool activeHasIpc: String(page.activeSettings.ipc || "") !== ""
        && String(page.activeSettings.call || "") !== ""
    // Item IPC targets exist only in the running shell, which keeps its item
    // until the staged switch is committed.
    readonly property bool activeRunning: !page.hub || !page.hub.barStyleSwitchPending
        || page.hub.runningBarStyle === page.activeStyle
    readonly property var parts: {
        const kinds = [
            { kind: "dock", label: I18n.tr("DOCK DESIGN") },
            { kind: "launcher", label: I18n.tr("LAUNCHER") },
            { kind: "desktopWidgets", label: I18n.tr("DESKTOP WIDGETS") }
        ];
        let out = [];
        for (const k of kinds)
            out = out.concat(ShellIndex.contributions(page.shellIndex, k.kind)
                .map(entry => Object.assign({ kind: k.kind, kindLabel: k.label }, entry)));
        return out;
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

    function editorShown(id) {
        return page.activeAllowed && page.activeEditor === id;
    }
    function editorValue(fallback) {
        return page.fval(page.activeEditor, fallback);
    }
    function editorEdit(value) {
        page.fedit(page.activeEditor, value);
    }
    function obiShown(id) {
        const values = page.editorValue(({}));
        return !values || values[id] !== false;
    }
    function obiSet(id, on) {
        const values = Object.assign({}, page.editorValue(({})));
        values[id] = on;
        page.editorEdit(values);
    }
    function galleryCard(id) {
        const cards = styleGallery.designs || [];
        for (let i = 0; i < cards.length; i++)
            if (cards[i].id === id && cards[i].installed)
                return cards[i];
        return null;
    }
    function selectStyle(id) {
        const previous = page.activeStyle;
        page.fedit("barStyle", id);
        if (page.hub && page.hub.markBarStyleChanged)
            page.hub.markBarStyleChanged(previous);
    }
    function switchNow() {
        if (page.hub && page.hub.commitShellSelection)
            page.hub.commitShellSelection();
    }
    function openItemSettings() {
        if (!page.activeHasIpc || !page.activeRunning)
            return;
        const args = Array.isArray(page.activeSettings.args)
            ? page.activeSettings.args.map(String) : [];
        Bus.ipc(String(page.activeSettings.ipc), String(page.activeSettings.call), args);
    }
    function openDockSettings() {
        Bus.ipc("desktop", "editSection", ["dock", ""]);
    }
    function openPart(part) {
        if (part.kind === "dock")
            page.openDockSettings();
        else if (part.kind === "desktopWidgets")
            Bus.ipc("desktop", "editSection", ["widgets", ""]);
        else if (page.hub && page.hub.navigate)
            page.hub.navigate("launcher");
    }

    ShellIndexFile { id: shellsIndex }


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
                + "   |   △ = " + I18n.tr("Uses a little more than the lightest options")
                + "   ▲ = " + I18n.tr("Uses more resources than other options")
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
                property bool fullWidth: true
                width: col.colWidth
                title: page.subshellsOnly ? I18n.tr("SUBSHELL") : I18n.tr("SHELL ITEM")

                StoreDesignGallery {
                    id: styleGallery
                    width: parent.width
                    category: page.subshellsOnly ? "subshells" : "barstyles"
                    categories: page.subshellsOnly
                        ? ["subshells"] : ["subshells", "barstyles"]
                    browseCategory: page.subshellsOnly ? "subshells" : "barstyles"
                    wornId: page.activeStyle
                    shippedDesigns: []
                    shellItems: page.shellIndex.items
                    onWear: designId => page.selectStyle(designId)
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

            SettingCard {
                id: activeItemSect
                width: col.colWidth
                visible: page.activeAllowed
                title: page.activeRunning ? I18n.tr("ACTIVE ITEM") : I18n.tr("SELECTED ITEM")

                Item {
                    width: parent.width
                    height: activeItemBody.height + Tokens.s3 + Tokens.s4
                    Column {
                        id: activeItemBody
                        anchors {
                            left: parent.left
                            right: parent.right
                            top: parent.top
                            leftMargin: Tokens.s4
                            rightMargin: Tokens.s4
                            topMargin: Tokens.s3
                        }
                        spacing: Tokens.s3
                        Text {
                            width: parent.width
                            text: page.activeName
                            color: Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fRow
                            font.weight: Font.Medium
                            elide: Text.ElideRight
                        }
                        Text {
                            width: parent.width
                            text: {
                                if (!page.activeItem)
                                    return "";
                                const source = page.activeSource === "shipped"
                                    ? I18n.tr("SHIPPED") : I18n.tr("STORE");
                                return page.activeVersion.length > 0
                                    ? I18n.tr("VERSION %1 · %2").arg(page.activeVersion).arg(source)
                                    : source;
                            }
                            color: Tokens.inkMuted
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fTiny
                            font.letterSpacing: Tokens.trackLabel
                        }
                        Text {
                            width: parent.width
                            visible: text !== ""
                            text: {
                                if (page.activeHasIpc && !page.activeRunning)
                                    return I18n.tr("%1 opens its settings once it is running. Switch now to restart the desktop on it.").arg(page.activeName);
                                if (!page.activeRunning)
                                    return I18n.tr("The desktop switches to %1 when you leave this page.").arg(page.activeName);
                                if (!page.activeHasIpc && page.activeEditor === "")
                                    return I18n.tr("%1 has no settings of its own.").arg(page.activeName);
                                return "";
                            }
                            color: Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fBody
                            wrapMode: Text.WordWrap
                        }
                        Row {
                            spacing: Tokens.s2
                            Btn {
                                visible: page.activeHasIpc && page.activeRunning
                                text: I18n.tr("OPEN SETTINGS")
                                onAct: page.openItemSettings()
                            }
                            Btn {
                                visible: !page.activeRunning
                                text: I18n.tr("SWITCH NOW")
                                onAct: page.switchNow()
                            }
                        }
                    }
                }
            }

            // Parts installed items offer to the rest of the desktop. Each is
            // picked where it is used, so the Hub only points there.
            SettingCard {
                id: partsSect
                width: col.colWidth
                visible: page.parts.length > 0
                title: I18n.tr("PARTS FROM ITEMS")

                Repeater {
                    model: page.parts
                    delegate: SettingRow {
                        id: partRow
                        required property var modelData
                        required property int index
                        anchors.left: parent.left
                        anchors.right: parent.right
                        divider: partRow.index > 0
                        controlWidth: 96
                        label: partRow.modelData.name
                        desc: I18n.tr("%1 · from %2").arg(partRow.modelData.kindLabel).arg(partRow.modelData.itemName)
                            + (partRow.modelData.prepared ? "" : "   " + I18n.tr("Unpacks when chosen"))
                        Btn {
                            objectName: "part-" + partRow.modelData.kind + "-" + partRow.modelData.id
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            compact: true
                            text: I18n.tr("CHOOSE")
                            onAct: page.openPart(partRow.modelData)
                        }
                    }
                }
            }

            // OBI WIDGETS: show or hide each widget on the Obi bar.
            SettingCard {
                id: obiSect
                width: col.colWidth
                visible: page.editorShown("obi")
                title: I18n.tr("ITEM WIDGETS")

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
                visible: page.editorShown("nacre")
                title: I18n.tr("NACRE LAYOUT")

                Item {
                    width: parent.width
                    height: nacreEd.height + Tokens.s3 + Tokens.s4
                    NacreEditor {
                        id: nacreEd
                        anchors { left: parent.left; right: parent.right; top: parent.top }
                        anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4; anchors.topMargin: Tokens.s3
                        config: page.editorValue(({}))
                        onStaged: value => page.editorEdit(value)
                    }
                }
            }

            // ── FRAME: the chrome the shell draws around the desktop ─────────
            SettingCard {
                id: frameSect
                width: col.colWidth
                title: I18n.tr("FRAME")
                visible: page.editorShown("frame-rails")

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
                visible: page.editorShown("frame-rails")

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
                visible: page.editorShown("frame-rails")

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
