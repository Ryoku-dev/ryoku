pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../schema/WidgetsPage.js" as Schema

// Desktop Widgets (DESIGN.md section 8, DESKTOP). The wallpaper clock is edited
// here and mirrored in a pinned live specimen so its face, size, opacity and
// placement read without leaning over to the desktop. This is a full-bleed page:
// the shell hides its side panel and action bar, so the page draws its own head,
// preview and Save/Revert bar. It owns widgets.json directly; nothing lands on
// the desktop until Save.
Item {
    id: pg

    property var hub
    readonly property bool fullBleed: true

    readonly property string query: (pg.hub && pg.hub.query) ? pg.hub.query : ""

    // ── the file's clock key set, mirrored by the JsonAdapter below ─────────
    readonly property var keys: [
        "clockEnabled", "clockDesign", "clock24h", "clockSeconds", "clockScale",
        "clockOpacity", "clockRadius", "clockAccent", "clockBg", "clockAnchor",
        "clockX", "clockY", "clockLocked", "dateShow", "dateDesign", "widgetFont",
        "calendarEnabled", "calendarStyle", "calendarWeeks", "calendarWeekNumbers",
        "calendarHolidayRegion", "calendarScale", "calendarOpacity", "calendarAnchor",
        "calendarX", "calendarY", "calendarLocked",
        "musicEnabled", "musicStyle", "musicLyrics", "musicViz", "musicScale", "musicOpacity",
        "musicAnchor", "musicX", "musicY", "musicLocked", "musicApp",
        "aioEnabled", "aioStyle", "aioScale", "aioOpacity", "aioAnchor", "aioX", "aioY", "aioLocked",
        "statsEnabled", "statsScale", "statsOpacity", "statsAnchor", "statsX", "statsY", "statsLocked",
        "weatherEnabled", "weatherDesign", "weatherScale", "weatherOpacity", "weatherAnchor", "weatherX", "weatherY", "weatherLocked",
        "notesEnabled", "notesScale", "notesOpacity", "notesWidth", "notesHeight", "notesAnchor", "notesX", "notesY", "notesLocked"
    ]

    // Factory values mirror the wallpaper clock's canonical Config defaults.
    readonly property var factory: ({
        "clockEnabled": true, "clockDesign": "digital", "clock24h": true, "clockSeconds": false,
        "clockScale": 1.0, "clockOpacity": 1.0, "clockRadius": 26, "clockAccent": "palette",
        "clockBg": "none", "clockAnchor": "top-left", "clockX": 72, "clockY": 64, "clockLocked": false,
        "dateShow": true, "dateDesign": "inline", "widgetFont": "",
        "calendarEnabled": true, "calendarStyle": "glass", "calendarWeeks": 6,
        "calendarWeekNumbers": true, "calendarHolidayRegion": "", "calendarScale": 1.0,
        "calendarOpacity": 1.0, "calendarAnchor": "bottom-right", "calendarX": 80,
        "calendarY": 80, "calendarLocked": false,
        "musicEnabled": false, "musicStyle": "cover", "musicLyrics": true, "musicViz": "bars",
        "musicScale": 1.0, "musicOpacity": 1.0, "musicAnchor": "bottom-left",
        "musicX": 80, "musicY": 80, "musicLocked": false, "musicApp": "",
        "aioEnabled": false, "aioStyle": "wide", "aioScale": 1.0, "aioOpacity": 1.0,
        "aioAnchor": "top-right", "aioX": 80, "aioY": 80, "aioLocked": false,
        "statsEnabled": false, "statsScale": 1.0, "statsOpacity": 1.0,
        "statsAnchor": "bottom-right", "statsX": 80, "statsY": 80, "statsLocked": false,
        "weatherEnabled": false, "weatherDesign": "compact", "weatherScale": 1.0, "weatherOpacity": 1.0,
        "weatherAnchor": "top-right", "weatherX": 80, "weatherY": 80, "weatherLocked": false,
        "notesEnabled": false, "notesScale": 1.0, "notesOpacity": 1.0, "notesWidth": 260, "notesHeight": 180,
        "notesAnchor": "right", "notesX": 80, "notesY": 80, "notesLocked": false
    })

    // Scale and opacity persist as ratios; the sheet edits integer percents.
    readonly property var pctKeys: ({
        "clockOpacity": true, "calendarOpacity": true, "musicOpacity": true,
        "aioOpacity": true, "statsOpacity": true, "weatherOpacity": true, "notesOpacity": true
    })
    readonly property var scaleKeys: ({
        "clockScale": true, "calendarScale": true, "musicScale": true,
        "aioScale": true, "statsScale": true, "weatherScale": true, "notesScale": true
    })

    property var draft: ({})
    property var committed: ({})
    property bool loaded: false

    // Widget-font picker source. The bundled NibrasShell display faces (mirrors
    // the shell's Fonts singleton -- these live in the shell process, not
    // fontconfig, so they are named here as literals) come first, then every
    // installed family, read live like the Global page's system-font picker.
    // Injected into the widgetFont row's options so all fonts are reachable.
    readonly property var bundledFonts: [
        "Reckoner", "Reckoner Bold", "JF Flat", "Abberancy", "Daydream",
        "sabana", "Unrealised", "VIP Rawy Regular", "Xenophobia",
        "VEXA light R", "Overhead BRK"
    ]
    property var fontList: []
    readonly property var fontOptions: pg.bundledFonts.concat(pg.fontList)

    Process {
        id: fonts
        running: true
        command: ["bash", "-c", "fc-list : family | cut -d, -f1 | sort -u"]
        stdout: StdioCollector {
            id: fontsOut
            onStreamFinished: {
                var t = ("" + fontsOut.text).trim();
                pg.fontList = t.length > 0 ? t.split("\n").filter(function (x) { return x.length > 0; }) : [];
            }
        }
    }

    function readAdapter() {
        var m = {};
        for (var i = 0; i < pg.keys.length; i++) {
            var k = pg.keys[i];
            m[k] = cfgA[k];
        }
        return m;
    }
    function edit(k, v) {
        var d = Object.assign({}, pg.draft);
        d[k] = v;
        pg.draft = d;
    }
    function save() {
        for (var i = 0; i < pg.keys.length; i++) {
            var k = pg.keys[i];
            cfgA[k] = pg.draft[k];
        }
        cfg.writeAdapter();
        pg.committed = Object.assign({}, pg.draft);
    }
    function revert() { pg.draft = Object.assign({}, pg.committed); }
    function reset() { pg.draft = Object.assign({}, pg.factory); }

    // first load adopts disk as both baseline and draft. a later external write
    // (someone edits the file, or drags a widget on the desktop) rebases only
    // the keys the user has not locally edited, so unsaved edits survive.
    function onCfgLoaded() {
        if (!pg.loaded) {
            pg.committed = pg.readAdapter();
            pg.draft = pg.readAdapter();
            pg.loaded = true;
            return;
        }
        var disk = pg.readAdapter();
        var nc = {};
        var nd = Object.assign({}, pg.draft);
        for (var i = 0; i < pg.keys.length; i++) {
            var k = pg.keys[i];
            if (String(pg.draft[k]) === String(pg.committed[k])) {
                nd[k] = disk[k];
                nc[k] = disk[k];
            } else {
                nc[k] = pg.committed[k];
            }
        }
        pg.committed = nc;
        pg.draft = nd;
    }

    readonly property int dirtyCount: {
        if (!pg.loaded)
            return 0;
        var n = 0;
        for (var i = 0; i < pg.keys.length; i++) {
            var k = pg.keys[i];
            if (String(pg.draft[k]) !== String(pg.committed[k]))
                n++;
        }
        return n;
    }
    readonly property bool dirty: pg.dirtyCount > 0

    // Each widget keeps its own tab and its natural groups (WIDGET, FORMAT, DATE,
    // SIZE & SHAPE, PLACEMENT); the open subpage sets the sheet's tab so only that
    // widget's cards show. Ratio rows (scale, opacity) become integer-percent
    // sliders at the sheet boundary.
    readonly property var schemaRows: {
        var out = [];
        for (var i = 0; i < Schema.rows.length; i++) {
            var r = Schema.rows[i];
            var c = {};
            for (var p in r)
                c[p] = r[p];
            if (pg.pctKeys[r.key]) {
                c.ctl = "slid"; c.lo = 20; c.hi = 100; c.unit = "%"; c.pct = false;
            } else if (pg.scaleKeys[r.key]) {
                c.ctl = "slid"; c.lo = 50; c.hi = 250; c.unit = "%"; c.pct = false;
            }
            if (r.key === "widgetFont")
                c.opts = pg.fontOptions;
            out.push(c);
        }
        return out;
    }

    // the flat maps handed to the sheet: ratios shown as whole percents.
    readonly property var sheetDraft: {
        var m = {};
        for (var i = 0; i < pg.keys.length; i++) {
            var k = pg.keys[i];
            var v = pg.draft[k];
            if (pg.pctKeys[k] || pg.scaleKeys[k])
                m[k] = Math.round((Number(v) || 0) * 100);
            else
                m[k] = v;
        }
        return m;
    }
    readonly property var sheetDefaults: {
        var m = {};
        for (var i = 0; i < pg.keys.length; i++) {
            var k = pg.keys[i];
            var v = pg.committed[k];
            if (v === undefined) { m[k] = undefined; continue; }
            if (pg.pctKeys[k] || pg.scaleKeys[k])
                m[k] = Math.round((Number(v) || 0) * 100);
            else
                m[k] = v;
        }
        return m;
    }

    function onSheetEdited(k, v) {
        if (pg.pctKeys[k] || pg.scaleKeys[k])
            pg.edit(k, v / 100);
        else
            pg.edit(k, v);
    }
    function onSheetPick(r) { pg.pickRow = r; }

    // anchor -> 0/0.5/1 on each axis, for the card's corner mini-map.
    function afx(a) { return a.indexOf("left") >= 0 ? 0 : (a.indexOf("right") >= 0 ? 1 : 0.5); }
    function afy(a) { return a.indexOf("top") >= 0 ? 0 : (a.indexOf("bottom") >= 0 ? 1 : 0.5); }

    // one live specimen: a framed card that renders the real desktop widget,
    // scaled to fit and centred, dimmed with a struck header when the widget is
    // off, and a 3x3 corner map marking where it sits on the wallpaper.
    component SpecimenCard: Rectangle {
        id: card
        property string title: ""
        property bool on: true
        property string anchor: "center"
        property real natW: 200
        property real natH: 140
        property real userScale: 1
        property real userOpacity: 1
        property Component preview: null

        color: Tokens.paperLift
        radius: Tokens.radius
        border.width: Tokens.border
        border.color: card.on ? Tokens.line : Tokens.lineSoft
        clip: true
        opacity: card.on ? 1 : 0.5
        Behavior on opacity { NumberAnimation { duration: Tokens.snap } }

        Item {
            id: hdr
            anchors { left: parent.left; right: parent.right; top: parent.top }
            anchors.leftMargin: Tokens.s3; anchors.rightMargin: Tokens.s3; anchors.topMargin: Tokens.s2
            height: 14
            Text {
                anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                text: card.title
                color: card.on ? Tokens.inkMuted : Tokens.inkFaint
                font.family: Tokens.ui; font.pixelSize: 9; font.weight: Font.Medium
                font.letterSpacing: Tokens.trackLabel; font.strikeout: !card.on
            }
            Grid {
                anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                columns: 3; rowSpacing: 2; columnSpacing: 2
                Repeater {
                    model: 9
                    Rectangle {
                        required property int index
                        width: 3; height: 3
                        readonly property bool lit: card.on
                            && (index % 3) === Math.round(pg.afx(card.anchor) * 2)
                            && Math.floor(index / 3) === Math.round(pg.afy(card.anchor) * 2)
                        color: lit ? Tokens.ink : Tokens.lineStrong
                    }
                }
            }
        }

        Item {
            id: bodyHolder
            anchors { left: parent.left; right: parent.right; top: hdr.bottom; bottom: parent.bottom }
            anchors.margins: Tokens.s2
            clip: true
            Item {
                width: card.natW; height: card.natH
                anchors.centerIn: parent
                opacity: card.userOpacity
                scale: Math.min(bodyHolder.width / card.natW, bodyHolder.height / card.natH, 1.15) * card.userScale
                Loader { anchors.fill: parent; sourceComponent: card.preview }
            }
        }
    }

    // ── persistence: the page owns widgets.json ──────────────────────────────
    FileView {
        id: cfg
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/widgets.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        atomicWrites: true
        onFileChanged: reload()
        onLoaded: pg.onCfgLoaded()
        onLoadFailed: {
            if (!pg.loaded) {
                pg.committed = pg.readAdapter();
                pg.draft = pg.readAdapter();
                pg.loaded = true;
            }
        }

        JsonAdapter {
            id: cfgA
            property bool clockEnabled: true
            property string clockDesign: "digital"
            property bool clock24h: true
            property bool clockSeconds: false
            property real clockScale: 1.0
            property real clockOpacity: 1.0
            property int clockRadius: 26
            property string clockAccent: "palette"
            property string clockBg: "none"
            property string clockAnchor: "top-left"
            property int clockX: 72
            property int clockY: 64
            property bool clockLocked: false
            property bool dateShow: true
            property string dateDesign: "inline"
            property string widgetFont: ""
            property bool calendarEnabled: true
            property string calendarStyle: "glass"
            property int calendarWeeks: 6
            property bool calendarWeekNumbers: true
            property string calendarHolidayRegion: ""
            property real calendarScale: 1.0
            property real calendarOpacity: 1.0
            property string calendarAnchor: "bottom-right"
            property int calendarX: 80
            property int calendarY: 80
            property bool calendarLocked: false
            property bool musicEnabled: false
            property string musicStyle: "cover"
            property bool musicLyrics: true
            property string musicViz: "bars"
            property real musicScale: 1.0
            property real musicOpacity: 1.0
            property string musicAnchor: "bottom-left"
            property int musicX: 80
            property int musicY: 80
            property bool musicLocked: false
            property string musicApp: ""
            property bool aioEnabled: false
            property string aioStyle: "wide"
            property real aioScale: 1.0
            property real aioOpacity: 1.0
            property string aioAnchor: "top-right"
            property int aioX: 80
            property int aioY: 80
            property bool aioLocked: false
            property bool statsEnabled: false
            property real statsScale: 1.0
            property real statsOpacity: 1.0
            property string statsAnchor: "bottom-right"
            property int statsX: 80
            property int statsY: 80
            property bool statsLocked: false
            property bool weatherEnabled: false
            property string weatherDesign: "compact"
            property real weatherScale: 1.0
            property real weatherOpacity: 1.0
            property string weatherAnchor: "top-right"
            property int weatherX: 80
            property int weatherY: 80
            property bool weatherLocked: false
            property bool notesEnabled: false
            property real notesScale: 1.0
            property real notesOpacity: 1.0
            property int notesWidth: 260
            property int notesHeight: 180
            property string notesAnchor: "right"
            property int notesX: 80
            property int notesY: 80
            property bool notesLocked: false
        }
    }

    // ── head: eyebrow, Fraunces title, blurb (matches every settings page) ──
    Column {
        id: head
        anchors.top: parent.top
        anchors.topMargin: Tokens.s6
        // the head sits on the body's grid, so the title starts over the first card
        x: Tokens.s6
        width: Math.max(320, pg.width - Tokens.s6 * 2 - Tokens.s3)
        // the register row sits off the title: a rule over a 32px
        // title needs more than the gap between two lines of body text
        spacing: Tokens.s3

        Row {
            // the register row holds a fixed box, so the rule and the seal keep
            // their distance from the title on every page
            height: Tokens.s5
            spacing: Tokens.s2
            Rectangle {
                width: 16; height: 1; color: Tokens.ink
                anchors.verticalCenter: parent.verticalCenter
            }
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
        Text {
            text: I18n.tr("Desktop Widgets"); color: Tokens.ink
            font.family: Tokens.display; font.pixelSize: Tokens.fTitle
        }
        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("The widgets on your wallpaper, previewed live.")
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
        }
    }

    // ── the widget catalogue: a card per widget; a card opens its settings ────
    // Grid view lists every desktop widget as a live-preview card with an on/off
    // toggle; clicking a card slides in that widget's settings subpage. One draft,
    // one Save bar for all of them (unchanged below).
    property string selected: ""   // "" = grid; else the open widget's tab

    readonly property var widgets: [
        { "tab": "clock",    "title": I18n.tr("Clock"),        "jp": "時計", "enable": "clockEnabled",    "anchor": "clockAnchor",    "natW": 300, "natH": 150 },
        { "tab": "aio",      "title": I18n.tr("All-in-one"),   "jp": "一体", "enable": "aioEnabled",      "anchor": "aioAnchor",      "natW": 354, "natH": 227 },
        { "tab": "stats",    "title": I18n.tr("System Stats"), "jp": "統計", "enable": "statsEnabled",    "anchor": "statsAnchor",    "natW": 261, "natH": 458 },
        { "tab": "calendar", "title": I18n.tr("Calendar"),     "jp": "暦",   "enable": "calendarEnabled", "anchor": "calendarAnchor", "natW": 330, "natH": 210 },
        { "tab": "music",    "title": I18n.tr("Music"),        "jp": "音楽", "enable": "musicEnabled",    "anchor": "musicAnchor",    "natW": 400, "natH": 216 },
        { "tab": "weather",  "title": I18n.tr("Weather"),      "jp": "天気", "enable": "weatherEnabled",  "anchor": "weatherAnchor",  "natW": 220, "natH": 390 },
        { "tab": "notes",    "title": I18n.tr("Notes"),        "jp": "メモ", "enable": "notesEnabled",    "anchor": "notesAnchor",    "natW": 260, "natH": 180 }
    ]
    function widgetOf(tab) { for (var i = 0; i < pg.widgets.length; i++) if (pg.widgets[i].tab === tab) return pg.widgets[i]; return null; }
    readonly property var curWidget: pg.selected === "" ? null : pg.widgetOf(pg.selected)

    // one preview per widget, reused by the grid card and the subpage specimen.
    Component { id: clockPrevC; Loader { anchors.fill: parent; source: Qt.resolvedUrl("../ClockPreview.qml")
        onLoaded: { item.design = Qt.binding(() => pg.draft.clockDesign || "digital"); item.is24 = Qt.binding(() => pg.draft.clock24h === true); item.seconds = Qt.binding(() => pg.draft.clockSeconds === true); item.accentChoice = Qt.binding(() => pg.draft.clockAccent || "palette"); item.dateShow = Qt.binding(() => pg.draft.dateShow === true); item.dateDesign = Qt.binding(() => pg.draft.dateDesign || "inline"); } } }
    Component { id: aioPrevC; Loader { anchors.fill: parent; source: Qt.resolvedUrl("../AioPreview.qml")
        onLoaded: { item.style = Qt.binding(() => pg.draft.aioStyle || "wide"); } } }
    Component { id: statsPrevC; Loader { anchors.fill: parent; source: Qt.resolvedUrl("../StatsPreview.qml") } }
    Component { id: calPrevC; Loader { anchors.fill: parent; source: Qt.resolvedUrl("../CalendarPreview.qml")
        onLoaded: { item.style = Qt.binding(() => pg.draft.calendarStyle || "glass"); item.weeks = Qt.binding(() => pg.draft.calendarWeeks || 6); item.showWeekNumbers = Qt.binding(() => pg.draft.calendarWeekNumbers === true); } } }
    Component { id: musicPrevC; Loader { anchors.fill: parent; source: Qt.resolvedUrl("../MusicPreview.qml")
        onLoaded: { item.style = Qt.binding(() => pg.draft.musicStyle || "cover"); item.lyrics = Qt.binding(() => pg.draft.musicLyrics === true); item.viz = Qt.binding(() => pg.draft.musicViz || "bars"); } } }
    Component { id: weatherPrevC; Loader { anchors.fill: parent; source: Qt.resolvedUrl("../WeatherPreview.qml")
        onLoaded: { item.design = Qt.binding(() => pg.draft.weatherDesign || "compact"); } } }
    Component { id: notesPrevC; Loader { anchors.fill: parent; source: Qt.resolvedUrl("../NotesPreview.qml") } }
    function previewFor(tab) {
        switch (tab) { case "aio": return aioPrevC; case "stats": return statsPrevC; case "calendar": return calPrevC; case "music": return musicPrevC; case "weather": return weatherPrevC; case "notes": return notesPrevC; default: return clockPrevC; }
    }

    // ── store-installed desktop widgets ──────────────────────────────────────
    // Plugins whose home is the wallpaper, discovered exactly as the Add-ons
    // page does (discover.sh --all, then keep only the desktopWidget host).
    // They live below the built-in grid, grouped by the set their manifest
    // names, and toggle live through ryoku-plugins-place -- outside this page's
    // draft/Save flow entirely.
    property var storeRows: []

    readonly property string shellDir: Quickshell.env("RYOKU_SHELL_DIR")
    readonly property string discoverScript: (pg.shellDir && pg.shellDir.length > 0)
        ? pg.shellDir + "/quickshell/plugins/discover.sh"
        : (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/quickshell/plugins/discover.sh"

    // group into sections in first-seen set order; plugins that name no set fall
    // to a final "Plugins" section, so any future set slots in with no change.
    readonly property var storeSections: {
        var order = [];
        var byKey = ({});
        var loose = [];
        for (var i = 0; i < pg.storeRows.length; i++) {
            var r = pg.storeRows[i];
            var s = r.set || "";
            if (s === "") { loose.push(r); continue; }
            if (byKey[s] === undefined) { byKey[s] = []; order.push(s); }
            byKey[s].push(r);
        }
        var out = [];
        for (var j = 0; j < order.length; j++)
            out.push({ "name": order[j], "rows": byKey[order[j]] });
        if (loose.length > 0)
            out.push({ "name": I18n.tr("Plugins"), "rows": loose });
        return out;
    }

    function refreshStore() { storeProc.running = false; storeProc.running = true; }
    function placePlugin(id, enabled) {
        if (!id)
            return;
        storePlaceProc.command = ["ryoku-plugins-place", id, "enabled", enabled ? "true" : "false"];
        storePlaceProc.running = true;
    }

    Process {
        id: storeProc
        command: ["bash", pg.discoverScript, "--all"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                var list = [];
                try { list = JSON.parse(text || "[]"); } catch (e) { list = []; }
                var rows = [];
                for (var i = 0; i < list.length; i++) {
                    var e = list[i];
                    var man = e.manifest || ({});
                    var place = e.placement || ({});
                    var host = place.host
                        ? place.host
                        : ((man.defaults && man.defaults.host) ? man.defaults.host : "framePopout");
                    if (host !== "desktopWidget")
                        continue;
                    rows.push({
                        "id": e.id,
                        "title": man.name || e.id,
                        "set": man.set || "",
                        "enabled": place.enabled === true,
                        "icon": (man.defaults && man.defaults.icon) ? man.defaults.icon : "",
                        "dir": e.dir || "",
                        "settings": (place.settings && typeof place.settings === "object") ? place.settings : ({})
                    });
                }
                pg.storeRows = rows;
            }
        }
    }
    // a placement toggle re-reads the truth, so the ON/OFF line and the switch
    // settle on what actually landed on disk.
    Process { id: storePlaceProc; onExited: pg.refreshStore() }

    // installing a set from the store writes plugins.json; that write lights up
    // this shelf without reopening the Hub.
    FileView {
        id: placementWatch
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/plugins.json"
        watchChanges: true
        printErrors: false
        onFileChanged: pg.refreshStore()
    }

    Item {
        id: content
        anchors { left: parent.left; right: parent.right; top: head.bottom; bottom: bar.top }
        anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6
        anchors.topMargin: Tokens.s5; anchors.bottomMargin: Tokens.s4

        // ── grid of widget cards ──────────────────────────────────────────────
        Flickable {
            id: gridFlick
            anchors.fill: parent
            contentHeight: (pg.storeSections.length > 0 ? storeStack.y + storeStack.height : grid.height) + Tokens.s5
            clip: true
            interactive: pg.selected === ""
            opacity: pg.selected === "" ? 1 : 0
            visible: opacity > 0.01
            enabled: pg.selected === ""
            Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
            WheelScroll { }

            Flow {
                id: grid
                width: gridFlick.width
                spacing: Tokens.s4

                Repeater {
                    model: pg.widgets
                    delegate: Rectangle {
                        id: wcard
                        required property var modelData
                        readonly property bool on: pg.draft[wcard.modelData.enable] === true
                        width: Math.max(280, Math.min(360, (grid.width - Tokens.s4 * 2) / 3))
                        height: 236
                        radius: Tokens.radius
                        color: Tokens.paperLift
                        border.width: Tokens.border
                        border.color: wcHover.hovered ? Tokens.sun : (wcard.on ? Tokens.line : Tokens.lineSoft)
                        Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

                        HoverHandler { id: wcHover }
                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: pg.selected = wcard.modelData.tab
                        }

                        Item {
                            id: pbody
                            anchors { left: parent.left; right: parent.right; top: parent.top }
                            anchors.margins: Tokens.s3
                            height: wcard.height - footer.height - Tokens.s3 * 3
                            clip: true
                            opacity: wcard.on ? 1 : 0.45
                            Behavior on opacity { NumberAnimation { duration: Tokens.snap } }
                            Item {
                                id: pwrap
                                width: wcard.modelData.natW
                                // the preview's own natural height when it reports one,
                                // so a face with a date line is scaled to fit whole
                                // rather than clipped at the card's edge.
                                readonly property real natH: (pv.item && pv.item.implicitHeight > 0)
                                    ? pv.item.implicitHeight : wcard.modelData.natH
                                height: natH
                                anchors.centerIn: parent
                                // never upscale past natural size: a preview blown
                                // up to fill clips flush against the footer and its
                                // glyphs read as overlapping the label row.
                                scale: Math.min(pbody.width / pwrap.width, pbody.height / pwrap.natH, 1.0)
                                Loader { id: pv; anchors.fill: parent; sourceComponent: pg.previewFor(wcard.modelData.tab) }
                            }
                        }

                        Item {
                            id: footer
                            anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
                            anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s3; anchors.bottomMargin: Tokens.s3
                            height: 34
                            Column {
                                anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                                spacing: 2
                                Text { text: I18n.tr(wcard.modelData.title); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fRow; font.weight: Font.Medium }
                                Text {
                                    text: (wcard.on ? I18n.tr("On") : I18n.tr("Off")).toUpperCase() + "  ·  " + String(pg.draft[wcard.modelData.anchor] || "").toUpperCase()
                                    color: wcard.on ? Tokens.inkMuted : Tokens.inkFaint
                                    font.family: Tokens.mono; font.pixelSize: Tokens.fTiny; font.letterSpacing: 0.6
                                }
                            }
                            Sw {
                                anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                                on: wcard.on
                                onToggled: (v) => pg.edit(wcard.modelData.enable, v)
                            }
                        }
                    }
                }
            }

            // ── store-installed desktop widgets, grouped by set ───────────────
            // A second shelf under the built-in grid: a divider, then a section
            // per set. Nothing draws when none are installed, so a stock desktop
            // reads exactly as before. These toggle live through the placement
            // backend and stay clear of the Save bar's dirty state.
            Column {
                id: storeStack
                anchors.top: grid.bottom
                anchors.topMargin: Tokens.s5
                width: gridFlick.width
                spacing: Tokens.s5
                visible: pg.storeSections.length > 0

                Rectangle { width: parent.width; height: 1; color: Tokens.line }

                Repeater {
                    model: pg.storeSections
                    delegate: Column {
                        id: setSection
                        required property var modelData
                        width: storeStack.width
                        spacing: Tokens.s4

                        Text {
                            text: setSection.modelData.name
                            color: Tokens.inkMuted; font.family: Tokens.ui
                            font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                            font.letterSpacing: Tokens.trackMark
                            font.capitalization: Font.AllUppercase
                        }

                        Flow {
                            width: setSection.width
                            spacing: Tokens.s4
                            Repeater {
                                model: setSection.modelData.rows
                                delegate: StoreWidgetCard {
                                    required property var modelData
                                    width: Math.max(280, Math.min(360, (setSection.width - Tokens.s4 * 2) / 3))
                                    title: modelData.title
                                    on: modelData.enabled === true
                                    icon: modelData.icon
                                    dir: modelData.dir
                                    settings: modelData.settings
                                    onToggled: (v) => pg.placePlugin(modelData.id, v)
                                }
                            }
                        }
                    }
                }
            }
        }

        // ── a widget's settings subpage ───────────────────────────────────────
        Item {
            id: sub
            anchors.fill: parent
            opacity: pg.selected !== "" ? 1 : 0
            visible: opacity > 0.01
            enabled: pg.selected !== ""
            Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }

            Item {
                id: backRow
                anchors { left: parent.left; right: parent.right; top: parent.top }
                height: 30
                Row {
                    id: crumb
                    anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                    spacing: Tokens.s2
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: "‹  " + I18n.tr("All widgets")
                        color: backHover.hovered ? Tokens.ink : Tokens.inkMuted
                        font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; font.weight: Font.Medium
                    }
                    HoverHandler { id: backHover }
                }
                MouseArea { anchors.fill: crumb; cursorShape: Qt.PointingHandCursor; onClicked: pg.selected = "" }
                Text {
                    anchors { left: crumb.right; leftMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                    text: pg.curWidget ? I18n.tr(pg.curWidget.title) : ""
                    color: Tokens.ink; font.family: Tokens.display; font.pixelSize: Tokens.fHero
                }
            }

            Loader {
                id: sheetLoader
                anchors {
                    left: parent.left; right: subPreview.left
                    top: backRow.bottom; bottom: parent.bottom
                    rightMargin: Tokens.s5; topMargin: Tokens.s4
                }
                source: Qt.resolvedUrl("../SettingsSheet.qml")
                onLoaded: {
                    item.schema = Qt.binding(() => pg.schemaRows);
                    item.draft = Qt.binding(() => pg.sheetDraft);
                    item.defaults = Qt.binding(() => pg.sheetDefaults);
                    item.tab = Qt.binding(() => pg.selected);
                    item.query = Qt.binding(() => pg.query);
                    // the rail's Advanced toggle reveals the per-widget desktop lock
                    item.advanced = Qt.binding(() => !!(pg.hub && pg.hub.advanced));
                    item.edited.connect(pg.onSheetEdited);
                    item.pickRequested.connect(pg.onSheetPick);
                    item.appPickRequested.connect(pg.onSheetAppPick);
                }
            }

            SpecimenCard {
                id: subPreview
                anchors { right: parent.right; top: backRow.bottom; bottom: parent.bottom }
                anchors.topMargin: Tokens.s4
                width: Math.round(Math.min(420, Math.max(300, pg.width * 0.34)))
                title: pg.curWidget ? I18n.tr("LIVE PREVIEW") : ""
                on: pg.curWidget ? (pg.draft[pg.curWidget.enable] === true) : false
                anchor: pg.curWidget ? (pg.draft[pg.curWidget.anchor] || "center") : "center"
                natW: pg.curWidget ? pg.curWidget.natW : 300
                natH: pg.curWidget ? pg.curWidget.natH : 200
                preview: pg.selected !== "" ? pg.previewFor(pg.selected) : null
            }
        }
    }

    // ── action bar: status + Reset / Revert / Save ──────────────────────────
    // full-bleed, so the shell's global bar is hidden and this is the only way
    // to persist. nothing writes until Save (DESIGN.md section 11).
    Rectangle {
        id: bar
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        height: 60
        color: "transparent"

        Rectangle {
            anchors { left: parent.left; right: parent.right; top: parent.top }
            height: 1; color: Tokens.line
        }

        Row {
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s6
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3

            Rectangle {
                id: dot
                anchors.verticalCenter: parent.verticalCenter
                width: 6; height: 6; radius: 3
                antialiasing: false
                color: pg.dirty ? Tokens.ink : "transparent"
                border.width: pg.dirty ? 0 : Tokens.border
                border.color: Tokens.inkFaint

                SequentialAnimation on opacity {
                    running: pg.dirty
                    loops: Animation.Infinite
                    NumberAnimation { to: 0.3; duration: 600; easing.type: Easing.InOutSine }
                    NumberAnimation { to: 1.0; duration: 600; easing.type: Easing.InOutSine }
                    onStopped: dot.opacity = 1
                }
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: pg.dirty
                    ? (pg.dirtyCount === 1
                       ? I18n.tr("%1 CHANGE · PREVIEWING · NOT SAVED").arg(pg.dirtyCount)
                       : I18n.tr("%1 CHANGES · PREVIEWING · NOT SAVED").arg(pg.dirtyCount))
                    : I18n.tr("SAVED · LIVE ON YOUR DESKTOP")
                color: pg.dirty ? Tokens.ink : Tokens.inkMuted
                font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
                font.capitalization: Font.AllUppercase
            }
        }

        Row {
            anchors.right: parent.right
            anchors.rightMargin: Tokens.s6
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3

            Btn {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("RESET TO DEFAULTS")
                onAct: pg.reset()
            }
            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                width: 1; height: 20; color: Tokens.line
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("REVERT")
                armed: pg.dirty
                onAct: pg.revert()
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("SAVE")
                primary: true
                armed: pg.dirty
                onAct: pg.save()
            }
        }
    }

    // ── the anchor catalogue overlay (Picker), shared by the pick cells ──────
    property var pickRow: null
    property var appPickRow: null
    function onSheetAppPick(r) { pg.appPickRow = r; }

    MouseArea {
        id: scrim
        anchors.fill: parent
        visible: pg.pickRow !== null
        z: 100
        onClicked: pg.pickRow = null
        onVisibleChanged: if (visible) picker.open()

        Picker {
            id: picker
            anchors.centerIn: parent
            title: pg.pickRow ? I18n.tr(pg.pickRow.label) : ""
            options: pg.pickRow ? (pg.pickRow.opts || []) : []
            current: pg.pickRow ? String(pg.draft[pg.pickRow.key]) : ""
            // the only pick rows on this page are the anchor pickers; give the
            // new "auto" value a human label, the rest keep their raw key.
            labels: ({ "auto": I18n.tr("Auto (calm spot)") })
            onChose: (key) => {
                if (pg.pickRow)
                    pg.edit(pg.pickRow.key, key);
                pg.pickRow = null;
            }
            onDismissed: pg.pickRow = null

            MouseArea { anchors.fill: parent; z: -1 }
        }
    }

    // ── the music-app picker overlay (keybinds-style), for the "app" cell ────
    Loader {
        id: appPickerLoader
        anchors.fill: parent
        z: 101
        active: pg.appPickRow !== null
        source: active ? Qt.resolvedUrl("../AppPicker.qml") : ""
        onLoaded: {
            item.title = Qt.binding(() => pg.appPickRow ? I18n.tr(pg.appPickRow.label) : "");
            item.chosen.connect(function (cmd) {
                if (pg.appPickRow) pg.edit(pg.appPickRow.key, cmd);
                pg.appPickRow = null;
            });
            item.dismissed.connect(function () { pg.appPickRow = null; });
        }
    }
}
