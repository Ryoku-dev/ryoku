pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Effects
import "Singletons"
import Ryoku.Ui.Singletons
import shell.services as Services
import "ContributedWidgets.js" as ContributedWidgets
import "ii/IiRoster.js" as IiRoster
import "options/OptionsCatalog.js" as OptionsCatalog
import stage.services
import stage.modules.common as StageCommon
import stage.modules.ii.background.widgets

// The widget inspector is one settings body with two hosts. Ordinary desktop
// customization docks it beside the widget; the Stage Editor embeds it in the
// drawer. Look and each MenuSection from the widget's own options panel become
// tabs here, so neither host carries a second copy of widget-specific logic.
// Every control writes the same monitor-selected widgets Config and retunes the
// widget live.
Item {
    id: insp

    anchors.fill: parent

    property string scope: "clock"
    property bool open: false
    // In the Stage Editor the same inspector body belongs to the drawer. The
    // floating shell stays available for ordinary desktop customization.
    property bool embedded: false
    implicitHeight: insp.embedded
        ? tabStrip.implicitHeight + insp.gap + sheet.bodyDesired : 0
    // The live WidgetSlot, so the dock geometry follows the widget as it resizes.
    property var slot: null

    // Live while the sheet is on screen (open, or still fading shut) so the host
    // surface stays mapped through the fade and unmaps only once it settles.
    readonly property bool showing: insp.embedded || insp.open || sheet.opacity > 0.01
    visible: insp.showing
    // Take focus while shown so Esc reaches the floating sheet; the drawer owns
    // keyboard routing for its embedded copy.
    onShowingChanged: if (insp.showing && !insp.embedded) insp.forceActiveFocus()
    Keys.onEscapePressed: event => {
        if (insp.embedded) {
            event.accepted = false;
            return;
        }
        insp.close();
        event.accepted = true;
    }

    // The host surface masks input to this item so a press off it passes through
    // to the widgets; the video picker draws full-surface, so widen then.
    readonly property alias sheetItem: sheet
    readonly property bool pickerOpen: videoPicker.visible

    // Remember the tab picked per widget for the session. Until one is picked,
    // a hosted ii face opens on its own settings and everything else on Look.
    property var lastTab: ({})
    function _tabFor(widget) {
        const remembered = insp.lastTab[widget];
        if (remembered !== undefined)
            return remembered;
        const face = IiRoster.byPrefix(widget);
        if (!face)
            return 0;
        return WidgetsRegistry.getConfigPagePath(face.id).toString().length > 0 ? 1 : 0;
    }
    function _resetScopeState(widget) {
        insp.optScroll = 0;
        insp.curTab = insp._clampTab(insp._tabFor(widget));
    }
    onScopeChanged: insp._resetScopeState(insp.scope)

    // The widget rect, live off the slot, for docking.
    readonly property real wx: insp.slot ? insp.slot.x : 0
    readonly property real wy: insp.slot ? insp.slot.y : 0
    readonly property real ww: insp.slot ? insp.slot.width : 0
    readonly property real wh: insp.slot ? insp.slot.height : 0

    function retarget(widget) {
        insp.scope = widget;
        // Also reset when reopening the already-targeted widget, where
        // onScopeChanged does not fire.
        insp._resetScopeState(widget);
    }
    function openFor(widget, slot) {
        insp.retarget(widget);
        insp.slot = slot || null;
        insp.open = true;
    }
    function close() { if (!insp.embedded) insp.open = false; }
    function cap(s) { return (s && s.length > 0) ? s.charAt(0).toUpperCase() + s.slice(1) : s; }
    function configValue(key) { return Config.get(key, Config.writeMonitor); }

    // ── widget identity: glyph, name and kanji seal for the title row ──────
    readonly property var contributedMatch: ContributedWidgets.find(
        Services.Shells.contributions("desktopWidgets"), insp.scope)
    readonly property bool isContributed: insp.contributedMatch !== null
    readonly property var contributedSet: insp.isContributed
        ? insp.contributedMatch.set : null
    readonly property var contributedRecord: insp.isContributed
        ? insp.contributedMatch.record : null
    readonly property var iiFace: IiRoster.byPrefix(insp.scope)
    readonly property bool isIi: insp.iiFace !== null
    // An ii face draws its own card, so its slot plate is off by default and
    // the plate's shape rows only matter once one is chosen.
    readonly property bool iiPlate: insp.isIi && (insp.configValue(insp.scope + "Bg") || "none") !== "none"
    readonly property var hostedFace: insp.isContributed
        ? insp.contributedRecord : insp.iiFace
    readonly property bool isCanvas: insp.isContributed
        && insp.contributedRecord.kind === "canvas"
    readonly property var builtinInfo: ({
        clock: { label: "Clock", icon: "schedule", gloss: "時計" },
        calendar: { label: "Calendar", icon: "calendar_month", gloss: "暦" },
        music: { label: "Music", icon: "music_note", gloss: "音楽" },
        aio: { label: "All-in-one", icon: "dashboard", gloss: "一体" },
        stats: { label: "System stats", icon: "monitor_heart", gloss: "計測" },
        weather: { label: "Weather", icon: "partly_cloudy_day", gloss: "天気" },
        notes: { label: "Notes", icon: "sticky_note_2", gloss: "筆記" },
        dayprogress: { label: "Day Progress", icon: "donut_large", gloss: "経過" },
        shape: { label: "Shape", icon: "category", gloss: "図形" },
        visualizer: { label: "Visualizer", icon: "graphic_eq", gloss: "音波" }
    })
    readonly property var _info: insp.builtinInfo[insp.scope] || null
    readonly property string wLabel: insp.hostedFace ? insp.hostedFace.label : (insp._info ? insp._info.label : insp.cap(insp.scope))
    readonly property string wIcon: insp.hostedFace ? insp.hostedFace.icon : (insp._info ? insp._info.icon : "widgets")
    readonly property string wGloss: insp.hostedFace ? insp.hostedFace.gloss : (insp._info ? insp._info.gloss : "")

    // ── look / colour / shape state (the generic controls the menu shed) ──
    readonly property bool isClock: insp.scope === "clock"
    readonly property bool isMusic: insp.scope === "music"
    readonly property bool isCalendar: insp.scope === "calendar"
    readonly property bool isAio: insp.scope === "aio"
    readonly property bool isDayprogress: insp.scope === "dayprogress"
    readonly property bool isShape: insp.scope === "shape"
    // The visualiser's own store owns its look, colour and box, so the generic
    // Look tab (which writes widgets.json) would edit keys nothing reads; its
    // tabs come entirely from its options panel.
    readonly property bool isVisualizer: insp.scope === "visualizer"
    readonly property bool isRyokuStyle: insp.isContributed
        && insp.configValue(insp.scope + "Style") === "ryoku"
    readonly property var facePresets: insp.isContributed
        && Array.isArray(insp.contributedRecord.sizes)
        ? insp.contributedRecord.sizes : []
    readonly property string curFacePreset: insp.facePresets.length > 0
        ? (insp.configValue(insp.scope + "Size") || insp.facePresets[0]) : ""

    readonly property var designLists: ({
        clock: ["digital", "minimal", "grand", "column", "outline", "banner", "analog", "flip", "rings", "bighour", "metal", "goodnight"],
        calendar: ["glass", "paper"],
        music: ["cover", "glass"],
        aio: ["wide", "tall"],
        weather: ["compact", "full"],
        dayprogress: ["ring", "arc"],
        shape: ["dot", "ring", "diamond", "square"]
    })
    readonly property string designKey: insp.isCalendar || insp.isMusic || insp.isAio || insp.isDayprogress
        ? insp.scope + "Style" : insp.isShape ? insp.scope + "Kind" : insp.scope + "Design"
    readonly property string curDesign: insp.configValue(insp.designKey) ?? ""
    readonly property bool hasDesign: !insp.isContributed
        && (insp.designLists[insp.scope] !== undefined)

    readonly property string curColor: insp.configValue(insp.scope + "Color") || ""
    readonly property bool curGradient: insp.configValue(insp.scope + "Gradient") === true
    readonly property string colorMode: insp.curColor === "" ? "auto" : (insp.curGradient ? "gradient" : "solid")

    function cycleDesign() {
        const d = insp.designLists[insp.scope];
        if (!d)
            return;
        Config.set(insp.designKey, d[(d.indexOf(insp.curDesign) + 1) % d.length]);
    }
    function cycleFacePreset() {
        const presets = insp.facePresets;
        if (presets.length < 2)
            return;
        const current = presets.indexOf(insp.curFacePreset);
        Config.set(insp.scope + "Size", presets[(current + 1) % presets.length]);
    }
    function hexOf(c) {
        return "#" + [c.r, c.g, c.b].map(function (x) {
            const s = Math.round(x * 255).toString(16);
            return s.length === 1 ? "0" + s : s;
        }).join("").toUpperCase();
    }
    // Auto clears the pin; Solid/Gradient need a base, so seed the wallpaper
    // accent (and a lighter twin for the second stop) when coming from Auto.
    function setColorMode(m) {
        if (m === "auto") {
            Config.set(insp.scope + "Color", "");
            Config.set(insp.scope + "Gradient", false);
            return;
        }
        if (insp.curColor === "")
            Config.set(insp.scope + "Color", insp.hexOf(Scheme.accent));
        if (m === "gradient") {
            if ((insp.configValue(insp.scope + "Color2") || "") === "")
                Config.set(insp.scope + "Color2", insp.hexOf(Qt.lighter(Scheme.accent, 1.5)));
            Config.set(insp.scope + "Gradient", true);
        } else {
            Config.set(insp.scope + "Gradient", false);
        }
    }
    function videoLabel(v) { return v === "canvas" ? "Spotify Canvas" : v === "custom" ? "Custom" : "Off"; }
    function videoName(p) {
        if (!p || p.length === 0)
            return "None";
        const s = ("" + p).replace(/\/+$/, "");
        return decodeURIComponent(s.slice(s.lastIndexOf("/") + 1));
    }
    function cycleVideo() {
        const d = ["off", "canvas", "custom"];
        Config.set("musicVideo", d[(d.indexOf(insp.configValue("musicVideo")) + 1) % d.length]);
    }

    readonly property var iiWidgetOptions: StageCommon.Config.options.background.widgets
    readonly property string iiScheme: String(insp.iiWidgetOptions.colorScheme ?? "default")
    readonly property bool iiTintEnabled: insp.iiWidgetOptions.tintOpacityEnabled ?? false
    readonly property real iiTintOpacity: {
        const value = insp.iiWidgetOptions.tintOpacity ?? 0.55;
        return Number.isFinite(value) ? Math.max(0, Math.min(1, value)) : 0.55;
    }
    function setIiGlobalOption(key, value) {
        if (StageCommon.Config.ready && insp.iiWidgetOptions)
            insp.iiWidgetOptions[key] = value;
    }

    component SchemeChip: Item {
        id: schemeChip

        property string scheme: ""
        property string title: ""
        property color cardColor: "transparent"
        property color dotColor: "transparent"
        property bool selected: false
        signal activated()

        implicitHeight: 42
        scale: schemeMouse.pressed ? 0.96 : 1
        Behavior on scale { NumberAnimation { duration: Theme.quick; easing.type: Theme.ease } }

        Rectangle {
            anchors.fill: parent
            radius: 7
            color: schemeChip.selected ? Theme.bone
                : schemeMouse.containsMouse ? Theme.tileHover : "transparent"
            border.width: 1
            border.color: schemeChip.selected ? Theme.bone
                : schemeMouse.containsMouse ? Theme.lineStrong : Theme.line
            Behavior on color { ColorAnimation { duration: Theme.quick } }
            Behavior on border.color { ColorAnimation { duration: Theme.quick } }
        }

        Rectangle {
            id: schemeSwatch
            anchors {
                left: parent.left
                leftMargin: 7
                verticalCenter: parent.verticalCenter
            }
            width: 24
            height: 24
            radius: 6
            color: schemeChip.cardColor
            border.width: 1
            border.color: Theme.lineStrong

            Rectangle {
                anchors {
                    right: parent.right
                    bottom: parent.bottom
                    margins: 3
                }
                width: 7
                height: 7
                radius: 4
                color: schemeChip.dotColor
                border.width: 1
                border.color: schemeChip.cardColor
            }
        }

        Text {
            anchors {
                left: schemeSwatch.right
                leftMargin: 7
                right: schemeMark.visible ? schemeMark.left : parent.right
                rightMargin: 7
                verticalCenter: parent.verticalCenter
            }
            text: schemeChip.title
            color: schemeChip.selected ? Theme.inkOnBone : Theme.inkSoft
            wrapMode: Text.WordWrap
            elide: Text.ElideRight
            maximumLineCount: 2
            font.family: Theme.font
            font.pixelSize: 10
            font.weight: Font.DemiBold
        }

        Text {
            id: schemeMark
            visible: schemeChip.selected
            anchors {
                right: parent.right
                rightMargin: 7
                verticalCenter: parent.verticalCenter
            }
            text: "check"
            color: Theme.inkOnBone
            font.family: Theme.iconFont
            font.pixelSize: 15
        }

        MouseArea {
            id: schemeMouse
            anchors.fill: parent
            hoverEnabled: true
            cursorShape: Qt.PointingHandCursor
            onClicked: schemeChip.activated()
        }
    }

    readonly property bool iiHasOptions: insp.isIi
        && WidgetsRegistry.getConfigPagePath(insp.iiFace.id).toString().length > 0
    readonly property string contributedOptionsSource:
        ContributedWidgets.optionSource(insp.contributedMatch)
    readonly property bool hasOptions: OptionsCatalog.has(insp.scope)
        || insp.iiHasOptions || insp.contributedOptionsSource.length > 0

    // ── tabs: Look, then one per MenuSection of the hosted panel ──
    // The section header items discovered in the hosted panel, in document order.
    property var sections: []
    function rescan() {
        var arr = [];
        var col = optionsLoader.item;
        if (col && col.children) {
            for (var i = 0; i < col.children.length; i++) {
                var c = col.children[i];
                if (c && c.ryoSection === true && c.label !== undefined && String(c.label).length > 0)
                    arr.push(c);
            }
        }
        insp.sections = arr;
    }

    readonly property var tabs: {
        var t = insp.isVisualizer ? [] : [{ kind: "look", label: I18n.tr("Look"), gloss: "見た目" }];
        if (insp.hasOptions) {
            var s = insp.sections;
            var shown = s.filter(sec => sec && sec.visible !== false);
            if (shown.length <= 1) {
                // one section or none: a single tab named after the widget itself.
                t.push({ kind: "opt", label: insp.wLabel, gloss: insp.wGloss, si: (shown.length === 1 ? insp.sections.indexOf(shown[0]) : -1) });
            } else {
                for (var i = 0; i < shown.length; i++)
                    t.push({ kind: "opt", label: shown[i].label, gloss: shown[i].gloss, si: insp.sections.indexOf(shown[i]) });
            }
        }
        return t;
    }
    // The tab list follows the scope a beat behind its change, so the opening
    // tab is chosen again once it settles.
    onTabsChanged: insp.curTab = insp._clampTab(insp.lastTab[insp.scope] === undefined
        ? insp._tabFor(insp.scope) : insp.curTab)

    property int curTab: 0
    onCurTabChanged: insp.optScroll = 0
    function _clampTab(i) { const n = insp.tabs.length; return Math.max(0, Math.min(i, Math.max(0, n - 1))); }
    readonly property var curTabObj: insp.tabs[Math.max(0, Math.min(insp.curTab, insp.tabs.length - 1))] || insp.tabs[0]

    // ── the band of the hosted panel shown by the selected option tab ─────
    property real optScroll: 0
    readonly property real optTop: {
        if (!insp.curTabObj || insp.curTabObj.kind !== "opt")
            return 0;
        var col = optionsLoader.item;
        if (!col)
            return 0;
        var si = insp.curTabObj.si;
        if (si < 0)
            return 0;
        var s = insp.sections[si];
        return s ? s.y + s.height : 0;
    }
    readonly property real optBottom: {
        if (!insp.curTabObj || insp.curTabObj.kind !== "opt")
            return 0;
        var col = optionsLoader.item;
        if (!col)
            return 0;
        var si = insp.curTabObj.si;
        if (si < 0)
            return col.implicitHeight;
        var arr = insp.sections;
        for (var j = si + 1; j < arr.length; j++)
            if (arr[j] && arr[j].visible)
                return arr[j].y;
        return col.implicitHeight;
    }
    readonly property real optBand: Math.max(0, insp.optBottom - insp.optTop)

    // ── dock geometry: the side of the widget with more room ──────────────
    readonly property int pad: Theme.s4
    readonly property int gap: Theme.s3
    readonly property int dockGap: Theme.s3
    readonly property int edgeMargin: Theme.s2
    readonly property bool dockRight: insp.slot ? (insp.width - (insp.wx + insp.ww) >= insp.wx) : true
    readonly property real dockRoom: !insp.slot ? insp.width - insp.edgeMargin * 2
        : insp.dockRight
            ? insp.width - (insp.wx + insp.ww) - insp.dockGap - insp.edgeMargin
            : insp.wx - insp.dockGap - insp.edgeMargin
    // The source pages normally have an 870px content viewport. Give ii pages
    // enough width for their row controls while yielding to the widget and the
    // monitor edge when either side is tighter.
    readonly property int sheetW: Math.max(1, Math.floor(Math.min(
        insp.isIi ? 520 : 384, Math.max(1, insp.dockRoom))))

    MultiEffect {
        source: sheet
        anchors.fill: sheet
        visible: !insp.embedded && !Performance.shadowsDisabled && sheet.opacity > 0.01
        shadowEnabled: true
        shadowColor: Theme.shadow
        shadowBlur: 1.0
        shadowVerticalOffset: 12
        blurMax: 48
        autoPaddingEnabled: true
    }

    Rectangle {
        id: sheet

        readonly property real chrome: insp.pad + titleRow.height + insp.gap + tabStrip.height + insp.gap + insp.pad
        readonly property real bodyDesired: (insp.curTabObj && insp.curTabObj.kind === "look") ? lookCol.implicitHeight
            : insp.optBand

        width: insp.embedded ? insp.width : insp.sheetW
        height: insp.embedded ? insp.height
            : Math.min(insp.height - Theme.s2 * 2, sheet.chrome + sheet.bodyDesired)
        radius: insp.embedded ? 0 : Theme.menuRadius
        color: insp.embedded ? "transparent" : Theme.surface
        border.width: insp.embedded ? 0 : 1
        border.color: Theme.line

        x: {
            if (insp.embedded)
                return 0;
            const mx = insp.edgeMargin;
            if (!insp.slot)
                return Math.max(mx, (insp.width - width) / 2);
            const rx = insp.wx + insp.ww + insp.dockGap;
            const lx = insp.wx - insp.dockGap - width;
            return insp.dockRight ? rx : lx;
        }
        y: {
            if (insp.embedded)
                return 0;
            const my = Theme.s2;
            if (!insp.slot)
                return Math.max(my, (insp.height - height) / 2);
            return Math.max(my, Math.min(insp.wy, insp.height - height - my));
        }

        opacity: insp.embedded || insp.open ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Theme.quick } }
        Behavior on height { NumberAnimation { duration: Theme.quick; easing.type: Theme.ease } }
        transform: Scale {
            origin.x: insp.dockRight ? 0 : sheet.width
            origin.y: 0
            xScale: insp.embedded || insp.open ? 1 : 0.96
            yScale: insp.embedded || insp.open ? 1 : 0.96
            Behavior on xScale { NumberAnimation { duration: Theme.quick; easing.type: Theme.ease } }
            Behavior on yScale { NumberAnimation { duration: Theme.quick; easing.type: Theme.ease } }
        }

        // ── title: glyph, name, kanji, and a close button ──
        Item {
            id: titleRow
            visible: !insp.embedded
            anchors { top: parent.top; left: parent.left; right: parent.right; margins: insp.pad }
            height: Theme.s6

            Text {
                id: tGlyph
                anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                text: insp.wIcon
                color: Theme.ink
                font.family: Theme.iconFont
                font.pixelSize: Theme.fBody + 6
            }
            Text {
                id: tName
                anchors { left: tGlyph.right; leftMargin: Theme.s2; verticalCenter: parent.verticalCenter }
                text: I18n.tr(insp.wLabel)
                color: Theme.ink
                font.family: Theme.font
                font.pixelSize: Theme.fBody + 1
                font.weight: Font.DemiBold
            }
            Text {
                anchors { left: tName.right; leftMargin: Theme.s2; verticalCenter: parent.verticalCenter }
                visible: insp.wGloss.length > 0
                text: insp.wGloss
                color: Theme.faint
                font.family: Theme.fontJp
                font.pixelSize: Theme.fSmall
            }
            Rectangle {
                id: closeBtn
                anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                width: Theme.s6
                height: Theme.s6
                radius: Theme.menuTileRadius
                color: closeMa.containsMouse ? Theme.tileHover : "transparent"
                Behavior on color { ColorAnimation { duration: Theme.quick } }
                Text {
                    anchors.centerIn: parent
                    text: "close"
                    color: closeMa.containsMouse ? Theme.ink : Theme.inkDim
                    font.family: Theme.iconFont
                    font.pixelSize: Theme.fBody + 2
                }
                MouseArea {
                    id: closeMa
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: insp.close()
                }
            }
        }

        // ── tab strip: chips wrap onto a second line rather than scroll, so every
        // tab stays visible and nothing is cut at the sheet's edge ──
        Flow {
            id: tabStrip
            anchors.top: insp.embedded ? parent.top : titleRow.bottom
            anchors.topMargin: insp.embedded ? 0 : insp.gap
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.leftMargin: insp.embedded ? 0 : insp.pad
            anchors.rightMargin: insp.embedded ? 0 : insp.pad
            spacing: Theme.s1
            Repeater {
                id: tabRep
                model: insp.tabs
                delegate: MenuChip {
                    id: tabChip
                    required property var modelData
                    required property int index
                    height: Theme.ctlH
                    label: tabChip.modelData.label
                    selected: tabChip.index === insp.curTab
                    onClicked: {
                        insp.curTab = tabChip.index;
                        insp.lastTab[insp.scope] = tabChip.index;
                    }
                }
            }
        }

        // ── content: one tab at a time, each scrolling vertically on overflow ──
        Item {
            id: contentArea
            anchors {
                top: tabStrip.bottom; topMargin: insp.gap
                left: parent.left; right: parent.right; bottom: parent.bottom
                leftMargin: insp.embedded ? 0 : insp.pad
                rightMargin: insp.embedded ? 0 : insp.pad
                bottomMargin: insp.embedded ? 0 : insp.pad
            }

            // ── Look ──
            Flickable {
                id: lookFlick
                anchors.fill: parent
                visible: insp.curTabObj && insp.curTabObj.kind === "look"
                contentWidth: width
                contentHeight: lookCol.implicitHeight
                clip: true
                interactive: false
                boundsBehavior: Flickable.StopAtBounds
                WheelHandler {
                    enabled: !insp.embedded
                    onWheel: (e) => {
                        const m = Math.max(0, lookFlick.contentHeight - lookFlick.height);
                        lookFlick.contentY = Math.max(0, Math.min(m, lookFlick.contentY - e.angleDelta.y));
                    }
                }

                Column {
                    id: lookCol
                    width: lookFlick.width
                    spacing: Theme.s1

                    MenuRow {
                        visible: insp.hasDesign
                        label: I18n.tr("Design")
                        value: insp.cap(insp.curDesign)
                        closeOnTrigger: false
                        onTriggered: insp.cycleDesign()
                    }
                    MenuRow {
                        visible: insp.isContributed
                        label: I18n.tr("Style")
                        value: insp.isRyokuStyle ? "Ryoku" : "Original"
                        on: insp.isRyokuStyle
                        closeOnTrigger: false
                        onTriggered: Config.set(insp.scope + "Style",
                            insp.isRyokuStyle ? insp.contributedSet.id : "ryoku")
                    }
                    MenuRow {
                        visible: insp.isContributed && insp.facePresets.length > 1
                        label: I18n.tr("Preset")
                        value: insp.cap(insp.curFacePreset)
                        closeOnTrigger: false
                        onTriggered: insp.cycleFacePreset()
                    }
                    Text {
                        visible: insp.isCanvas && insp.isRyokuStyle
                        width: parent.width
                        leftPadding: Theme.s3
                        rightPadding: Theme.s3
                        topPadding: Theme.s1
                        bottomPadding: Theme.s1
                        wrapMode: Text.WordWrap
                        text: I18n.tr("Ryoku style wraps this widget; it keeps its own colours")
                        color: Theme.inkDim
                        font.family: Theme.font
                        font.pixelSize: Theme.fSmall
                    }
                    MenuRow {
                        visible: insp.isMusic
                        label: I18n.tr("Backdrop")
                        value: insp.videoLabel(insp.configValue("musicVideo"))
                        on: insp.configValue("musicVideo") !== "off"
                        closeOnTrigger: false
                        onTriggered: insp.cycleVideo()
                    }
                    MenuRow {
                        visible: insp.isMusic
                        label: I18n.tr("Video / GIF…")
                        value: insp.videoName(insp.configValue("musicVideoFile"))
                        closeOnTrigger: false
                        onTriggered: videoPicker.open = true
                    }

                    // Short two-state settings as a compact pair of chips per row.
                    Grid {
                        id: toggleGrid
                        visible: insp.isClock || insp.isDayprogress || insp.isShape || insp.isMusic
                        width: parent.width
                        columns: 2
                        columnSpacing: Theme.s1
                        rowSpacing: Theme.s1
                        readonly property real cw: (width - columnSpacing) / 2
                        MenuChip {
                            visible: insp.isClock
                            width: toggleGrid.cw; height: Theme.ctlH
                            label: I18n.tr("Date"); selected: insp.configValue("dateShow") === true
                            onClicked: Config.toggle("dateShow")
                        }
                        MenuChip {
                            visible: insp.isDayprogress
                            width: toggleGrid.cw; height: Theme.ctlH
                            label: I18n.tr("Date"); selected: insp.configValue("dayprogressShowDate") === true
                            onClicked: Config.toggle("dayprogressShowDate")
                        }
                        MenuChip {
                            visible: insp.isShape
                            width: toggleGrid.cw; height: Theme.ctlH
                            label: I18n.tr("Outline"); selected: insp.configValue("shapeOutline") === true
                            onClicked: Config.toggle("shapeOutline")
                        }
                        MenuChip {
                            visible: insp.isMusic
                            width: toggleGrid.cw; height: Theme.ctlH
                            label: I18n.tr("Lyrics"); selected: insp.configValue("musicLyrics") === true
                            onClicked: Config.toggle("musicLyrics")
                        }
                        MenuChip {
                            visible: insp.isMusic
                            width: toggleGrid.cw; height: Theme.ctlH
                            label: I18n.tr("Waveform"); selected: insp.configValue("musicViz") === "wave"
                            onClicked: Config.set("musicViz", insp.configValue("musicViz") === "wave" ? "bars" : "wave")
                        }
                        MenuChip {
                            visible: insp.isMusic
                            width: toggleGrid.cw; height: Theme.ctlH
                            label: I18n.tr("Tall canvas"); selected: insp.configValue("musicShape") === "tall"
                            onClicked: Config.set("musicShape", insp.configValue("musicShape") === "tall" ? "wide" : "tall")
                        }
                    }

                    MenuSection {
                        visible: insp.isIi
                        label: I18n.tr("Colours")
                        gloss: "配色"
                    }
                    Text {
                        visible: insp.isIi
                        width: parent.width
                        leftPadding: Theme.s3
                        rightPadding: Theme.s3
                        topPadding: Theme.s1
                        bottomPadding: Theme.s1
                        wrapMode: Text.WordWrap
                        text: I18n.tr("Shared by every widget in this set")
                        color: Theme.inkDim
                        font.family: Theme.font
                        font.pixelSize: Theme.fSmall
                    }
                    Grid {
                        id: schemeGrid
                        visible: insp.isIi
                        width: parent.width
                        columns: 2
                        columnSpacing: Theme.s1
                        rowSpacing: Theme.s1
                        readonly property real chipWidth: (width - columnSpacing) / 2

                        Repeater {
                            model: WidgetColorScheme.availableSchemes
                            delegate: SchemeChip {
                                id: schemeChoice
                                required property string modelData
                                width: schemeGrid.chipWidth
                                scheme: schemeChoice.modelData
                                title: WidgetColorScheme.schemeNames[schemeChoice.scheme] ?? schemeChoice.scheme
                                cardColor: WidgetColorScheme.getCardBgColor(schemeChoice.scheme)
                                dotColor: WidgetColorScheme.getAccentColor(schemeChoice.scheme)
                                selected: insp.iiScheme === schemeChoice.scheme
                                onActivated: insp.setIiGlobalOption("colorScheme", schemeChoice.scheme)
                            }
                        }
                    }
                    MenuRow {
                        visible: insp.isIi
                        label: I18n.tr("See-through background")
                        value: insp.iiTintEnabled ? I18n.tr("On") : I18n.tr("Off")
                        on: insp.iiTintEnabled
                        closeOnTrigger: false
                        onTriggered: insp.setIiGlobalOption("tintOpacityEnabled", !insp.iiTintEnabled)
                    }
                    MenuSlider {
                        id: iiTintSlider
                        visible: insp.isIi && insp.iiTintEnabled
                        label: I18n.tr("Background opacity")
                        from: 0
                        to: 1
                        step: 0.01
                        value: insp.iiTintOpacity
                        valueText: Math.round(iiTintSlider.value * 100) + "%"
                        onMoved: (v) => insp.setIiGlobalOption("tintOpacity", v)
                        onReleased: (v) => insp.setIiGlobalOption("tintOpacity", v)
                    }

                    MenuSection { label: I18n.tr("Adjust"); gloss: "調整" }
                    MenuSlider {
                        id: sizeSlider
                        visible: !insp.embedded
                        label: I18n.tr("Size")
                        from: 0.5; to: 2.5; step: 0.02
                        // Size is kept per desktop, like the writes below.
                        value: insp.configValue(insp.scope + "Scale") || 1
                        valueText: Math.round(sizeSlider.value * 100) + "%"
                        onMoved: (v) => Config.setLive(insp.scope + "Scale", v)
                        onReleased: (v) => Config.set(insp.scope + "Scale", v)
                    }
                    MenuSlider {
                        id: opacitySlider
                        label: I18n.tr("Opacity")
                        from: 0.2; to: 1.0; step: 0.01
                        value: insp.configValue(insp.scope + "Opacity")
                        valueText: Math.round(opacitySlider.value * 100) + "%"
                        onMoved: (v) => Config.setLive(insp.scope + "Opacity", v)
                        onReleased: (v) => Config.set(insp.scope + "Opacity", v)
                    }

                    MenuSection {
                        visible: insp.isContributed || insp.isIi
                        label: insp.isIi ? I18n.tr("Plate") : I18n.tr("Shape")
                        gloss: insp.isIi ? "台座" : "形状"
                    }
                    MenuRow {
                        visible: insp.isIi || insp.isRyokuStyle
                        label: I18n.tr("Background")
                        value: insp.cap(insp.configValue(insp.scope + "Bg") || (insp.isIi ? "none" : "card"))
                        closeOnTrigger: false
                        onTriggered: {
                            const fallback = insp.isIi ? "none" : "card";
                            const choices = insp.isIi ? ["none", "card", "glass"] : ["card", "glass", "none"];
                            const current = insp.configValue(insp.scope + "Bg") || fallback;
                            Config.set(insp.scope + "Bg", choices[(choices.indexOf(current) + 1) % choices.length]);
                        }
                    }
                    MenuSlider {
                        id: radiusSlider
                        visible: insp.iiPlate || (insp.isContributed && insp.isRyokuStyle)
                        label: I18n.tr("Corner radius")
                        from: 0; to: 120; step: 1
                        value: insp.configValue(insp.scope + "Radius") >= 0 ? insp.configValue(insp.scope + "Radius") : Theme.radius
                        valueText: Math.round(radiusSlider.value)
                        onMoved: (v) => Config.setLive(insp.scope + "Radius", Math.round(v))
                        onReleased: (v) => Config.set(insp.scope + "Radius", Math.round(v))
                    }
                    MenuSlider {
                        id: padSlider
                        visible: insp.iiPlate || (insp.isContributed && insp.isRyokuStyle)
                        label: I18n.tr("Padding")
                        from: 0; to: 48; step: 1
                        value: insp.configValue(insp.scope + "Pad") >= 0 ? insp.configValue(insp.scope + "Pad") : 0
                        valueText: Math.round(padSlider.value)
                        onMoved: (v) => Config.setLive(insp.scope + "Pad", Math.round(v))
                        onReleased: (v) => Config.set(insp.scope + "Pad", Math.round(v))
                    }
                    MenuSlider {
                        id: borderSlider
                        visible: insp.iiPlate || insp.isRyokuStyle
                        label: I18n.tr("Border width")
                        from: 0; to: 4; step: 0.5
                        value: insp.configValue(insp.scope + "Border") >= 0 ? insp.configValue(insp.scope + "Border") : 1
                        valueText: borderSlider.value.toFixed(1)
                        onMoved: (v) => Config.setLive(insp.scope + "Border", v)
                        onReleased: (v) => Config.set(insp.scope + "Border", v)
                    }
                    MenuSlider {
                        id: borderOpSlider
                        visible: insp.iiPlate || insp.isRyokuStyle
                        label: I18n.tr("Border opacity")
                        from: 0; to: 1; step: 0.01
                        value: insp.configValue(insp.scope + "BorderOpacity") >= 0 ? insp.configValue(insp.scope + "BorderOpacity") : 0.16
                        valueText: Math.round(borderOpSlider.value * 100) + "%"
                        onMoved: (v) => Config.setLive(insp.scope + "BorderOpacity", v)
                        onReleased: (v) => Config.set(insp.scope + "BorderOpacity", v)
                    }
                    MenuSlider {
                        id: backingOpSlider
                        visible: insp.iiPlate || insp.isRyokuStyle
                        label: I18n.tr("Backing opacity")
                        from: 0; to: 1; step: 0.01
                        value: insp.configValue(insp.scope + "BackingOpacity") >= 0 ? insp.configValue(insp.scope + "BackingOpacity") : 0.5
                        valueText: Math.round(backingOpSlider.value * 100) + "%"
                        onMoved: (v) => Config.setLive(insp.scope + "BackingOpacity", v)
                        onReleased: (v) => Config.set(insp.scope + "BackingOpacity", v)
                    }

                    // The ii faces colour themselves from the vendored library's
                    // own scheme store, so the slot's ink push is inert: the
                    // Colour row would be a dead control.
                    MenuSection { visible: !insp.isCanvas && !insp.isIi; label: I18n.tr("Colour"); gloss: "彩色" }
                    Item {
                        visible: !insp.isCanvas && !insp.isIi
                        width: parent.width
                        implicitHeight: !insp.isCanvas && !insp.isIi ? colorCol.implicitHeight : 0
                        Column {
                            id: colorCol
                            width: parent.width
                            spacing: Theme.s1
                            Row {
                                id: modeRow
                                width: parent.width
                                spacing: Theme.s1
                                readonly property real cw: (width - 2 * Theme.s1) / 3
                                MenuChip {
                                    width: modeRow.cw; height: Theme.ctlH
                                    label: I18n.tr("Auto"); selected: insp.colorMode === "auto"
                                    onClicked: insp.setColorMode("auto")
                                }
                                MenuChip {
                                    width: modeRow.cw; height: Theme.ctlH
                                    label: I18n.tr("Solid"); selected: insp.colorMode === "solid"
                                    onClicked: insp.setColorMode("solid")
                                }
                                MenuChip {
                                    width: modeRow.cw; height: Theme.ctlH
                                    label: I18n.tr("Gradient"); selected: insp.colorMode === "gradient"
                                    onClicked: insp.setColorMode("gradient")
                                }
                            }
                            MenuColorPicker {
                                width: parent.width
                                visible: insp.colorMode !== "auto"
                                scope: insp.scope
                                gradient: insp.colorMode === "gradient"
                            }
                        }
                    }
                }
            }

            // ── Options: the widget's own panel, hosted once and sliced to the
            // selected group (its MenuSection band), scrolled with the wheel. ──
            Item {
                id: optView
                anchors.fill: parent
                visible: insp.hasOptions && insp.curTabObj && insp.curTabObj.kind === "opt"
                clip: true

                Item {
                    id: optHolder
                    width: parent.width
                    y: -(insp.optTop - Math.max(0, Math.min(insp.optScroll, Math.max(0, insp.optBand - optView.height))))
                    Loader {
                        id: optionsLoader
                        width: parent.width
                        active: insp.hasOptions
                        source: !active ? "" : insp.isIi ? "options/IiFaceOptions.qml"
                            : insp.isContributed ? insp.contributedOptionsSource
                            : "options/" + (OptionsCatalog.panelFile(insp.scope)
                                || insp.cap(insp.scope) + "Options.qml")
                        onLoaded: insp.rescan()
                        onItemChanged: insp.rescan()
                    }
                    Binding {
                        target: optionsLoader.item
                        property: "widget"
                        value: insp.scope
                        when: optionsLoader.item !== null
                            && optionsLoader.item.hasOwnProperty("widget")
                    }
                    Binding {
                        target: optionsLoader.item
                        property: "config"
                        value: Config
                        when: insp.isContributed && optionsLoader.item !== null
                            && optionsLoader.item.hasOwnProperty("config")
                    }
                }

                WheelHandler {
                    enabled: !insp.embedded
                    onWheel: (e) => {
                        const m = Math.max(0, insp.optBand - optView.height);
                        insp.optScroll = Math.max(0, Math.min(m, insp.optScroll - e.angleDelta.y));
                    }
                }
            }
        }
    }

    MusicVideoPicker {
        id: videoPicker
        onChose: (url) => {
            Config.set("musicVideoFile", url);
            Config.set("musicVideo", "custom");
        }
    }
}
