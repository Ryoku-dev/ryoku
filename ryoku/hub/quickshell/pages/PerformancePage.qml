pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"

// Performance (DESIGN.md section 11, ADVANCED). The tweaks that trade a little
// eye-candy, idle animation or resident memory for lower CPU, GPU and RAM use.
// Self-contained full-bleed page: it owns its whole content region, so it draws
// its own head, the schema grid, and -- because the shell hides its global
// action bar -- its own dirty status + Reset/Revert/Save bar. Nothing writes to
// disk until Save; every value is a Token.
//
// This page is performance.json's ONLY writer, and cfg.writeAdapter() serialises
// the whole adapter, so every key the file carries is declared below or it would
// be dropped on the next save. That is also how a retired key leaves: the two
// `freeze*WhenIdle` switches are gone because the behaviour they asked for is now
// unconditional -- an audio analyser with no audio has nothing to show, so it
// always stops -- and a stale copy in someone's file is ignored by the adapter
// and pruned the next time this page saves. The cheap-on-RAM keys ship ON
// (unloadWidgetsWhenCovered, unloadVisualizerWhenSilent and the launcher/overview
// unloads each free a hidden surface's whole process); their defaults mirror the
// Go watchers and must not drift.
// The lowPowerMode implication is applied downstream by each
// consumer (`lowPower || flag`), NOT here: the page writes only raw keys, so a
// sub-toggle stays visibly OFF while lowPowerMode overrides its behaviour, and
// un-toggling lowPowerMode restores the user's own choices intact.
//
// Blur, shadows and low-power are the only keys the compositor itself reads, at
// config parse time, so they are offered only where the compositor evaluates its
// config live (the liveConfigEval capability) and dropped elsewhere rather than
// shown as switches nothing reads. A Save that changes one of those three, and
// only those, reloads the active compositor once the write has landed, to re-read
// it live. Shell singletons watch the file themselves and need no reload.
Item {
    id: pg

    property var hub
    readonly property bool fullBleed: true

    // factory defaults: every tweak off except the two that ship on. The single
    // source RESET walks back to; it mirrors the JsonAdapter defaults below.
    readonly property var factory: ({
        "lowPowerMode": false,
        "powerProfileEffects": true,
        "autoPowerSaverOnBattery": false,
        "reduceMotion": false,
        "disableBlur": false,
        "disableShadows": false,
        "liveWallpaper60": false,
        "ambientBarMotion": false,
        "pauseLiveWallpaperWhenFullscreen": true,
        "unloadVisualizerWhenSilent": true,
        "unloadWidgetsWhenCovered": true,
        "unloadLauncherWhenIdle": true,
        "unloadOverviewWhenIdle": true
    })

    // the keys the compositor itself reads; a Save touching one reloads it.
    readonly property var compositorKeys: ["lowPowerMode", "disableBlur", "disableShadows"]

    // set by save(), consumed by the FileView's onSaved once the file is written.
    property bool reloadPending: false

    // committed = what is on disk; draft = the live, previewed edit; dirty = they
    // differ. Both are plain maps, reassigned wholesale so the schema re-renders.
    property var committed: null
    property var draft: null

    readonly property int dirtyCount: {
        if (!pg.draft || !pg.committed)
            return 0;
        var n = 0;
        for (var k in pg.factory)
            if (pg.draft[k] !== pg.committed[k])
                n++;
        return n;
    }
    // RESET is a no-op when the draft already equals stock, so gate it on that.
    readonly property bool offDefaults: {
        if (!pg.draft)
            return false;
        for (var k in pg.factory)
            if (pg.draft[k] !== pg.factory[k])
                return true;
        return false;
    }

    function clone(o) {
        var r = {};
        for (var k in o)
            r[k] = o[k];
        return r;
    }
    function fromAdapter() {
        return {
            "lowPowerMode": cfgA.lowPowerMode,
            "powerProfileEffects": cfgA.powerProfileEffects,
            "autoPowerSaverOnBattery": cfgA.autoPowerSaverOnBattery,
            "reduceMotion": cfgA.reduceMotion,
            "disableBlur": cfgA.disableBlur,
            "disableShadows": cfgA.disableShadows,
            "liveWallpaper60": cfgA.liveWallpaper60,
            "ambientBarMotion": cfgA.ambientBarMotion,
            "pauseLiveWallpaperWhenFullscreen": cfgA.pauseLiveWallpaperWhenFullscreen,
            "unloadVisualizerWhenSilent": cfgA.unloadVisualizerWhenSilent,
            "unloadWidgetsWhenCovered": cfgA.unloadWidgetsWhenCovered,
            "unloadLauncherWhenIdle": cfgA.unloadLauncherWhenIdle,
            "unloadOverviewWhenIdle": cfgA.unloadOverviewWhenIdle
        };
    }

    // adopt the on-disk state. First load seeds the draft too; a later external
    // edit rebases committed but keeps an in-flight draft (DESIGN.md section 8),
    // while a clean view simply follows the file.
    function adopt() {
        var wasClean = pg.draft === null || pg.dirtyCount === 0;
        pg.committed = pg.fromAdapter();
        if (wasClean)
            pg.draft = pg.clone(pg.committed);
    }

    function edit(k, v) {
        if (!pg.draft)
            return;
        var d = pg.clone(pg.draft);
        d[k] = v;
        pg.draft = d;
    }
    function revert() {
        if (pg.committed)
            pg.draft = pg.clone(pg.committed);
    }
    function reset() {
        pg.draft = pg.clone(pg.factory);
    }
    function save() {
        if (!pg.draft || !pg.committed)
            return;
        var needsReload = false;
        for (var i = 0; i < pg.compositorKeys.length; i++) {
            var k = pg.compositorKeys[i];
            if (pg.draft[k] !== pg.committed[k])
                needsReload = true;
        }
        cfgA.lowPowerMode = pg.draft.lowPowerMode;
        cfgA.powerProfileEffects = pg.draft.powerProfileEffects;
        cfgA.autoPowerSaverOnBattery = pg.draft.autoPowerSaverOnBattery;
        cfgA.reduceMotion = pg.draft.reduceMotion;
        cfgA.disableBlur = pg.draft.disableBlur;
        cfgA.disableShadows = pg.draft.disableShadows;
        cfgA.liveWallpaper60 = pg.draft.liveWallpaper60;
        cfgA.ambientBarMotion = pg.draft.ambientBarMotion;
        cfgA.pauseLiveWallpaperWhenFullscreen = pg.draft.pauseLiveWallpaperWhenFullscreen;
        cfgA.unloadVisualizerWhenSilent = pg.draft.unloadVisualizerWhenSilent;
        cfgA.unloadWidgetsWhenCovered = pg.draft.unloadWidgetsWhenCovered;
        cfgA.unloadLauncherWhenIdle = pg.draft.unloadLauncherWhenIdle;
        cfgA.unloadOverviewWhenIdle = pg.draft.unloadOverviewWhenIdle;
        cfg.writeAdapter();
        pg.committed = pg.clone(pg.draft);
        // reload once the write is on disk, not here: writeAdapter() completes
        // asynchronously, so reloading straight away made the compositor re-parse the
        // PREVIOUS performance.json. A compositor toggle then landed one save
        // late, which reads exactly like the switch being inverted.
        pg.reloadPending = needsReload;
    }

    Component.onCompleted: pg.adopt()

    // performance.json, this page's only writer. blockLoading makes the first
    // read synchronous; watchChanges + onFileChanged re-render on an external
    // edit; the seeding write creates the file (populated with every default)
    // when it is absent, so downstream consumers always have a file to read.
    FileView {
        id: cfg
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/performance.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: pg.adopt()
        onSaved: {
            if (!pg.reloadPending)
                return;
            pg.reloadPending = false;
            if (pg.hub) pg.hub.wmAct("config.reload");
        }

        JsonAdapter {
            id: cfgA
            property bool lowPowerMode: false
            property bool powerProfileEffects: true
            property bool autoPowerSaverOnBattery: false
            property bool reduceMotion: false
            property bool disableBlur: false
            property bool disableShadows: false
            property bool liveWallpaper60: false
            property bool ambientBarMotion: false
            property bool pauseLiveWallpaperWhenFullscreen: true
            property bool unloadVisualizerWhenSilent: true
            property bool unloadWidgetsWhenCovered: true
            property bool unloadLauncherWhenIdle: true
            property bool unloadOverviewWhenIdle: true
        }

        Component.onCompleted: if (!cfg.text()) cfg.writeAdapter()
    }

    // ── the schema, grouped by what you trade away ──
    // POWER (the profile the desktop follows), EFFECTS (what it draws), MOTION
    // (what keeps moving), MEMORY (what it unloads while nothing needs it). The
    // memory knobs open parked: they are the ones a user reaches for once, and
    // the card's caret plus search keep them a click away rather than a wall of
    // switches in the way.
    readonly property var schema: [
        { "tab": "", "group": I18n.tr("POWER"), "key": "powerProfileEffects", "ctl": "sw", "src": "performance",
          "label": I18n.tr("Follow the power profile"),
          "desc": I18n.tr("Power Saver strips motion, blur and shadows.") },
        { "tab": "", "group": I18n.tr("POWER"), "key": "autoPowerSaverOnBattery", "ctl": "sw", "src": "performance",
          "label": I18n.tr("Auto power saver on battery"),
          "desc": I18n.tr("Switches to Power Saver when you unplug.") },

        { "tab": "", "group": I18n.tr("EFFECTS"), "key": "lowPowerMode", "ctl": "sw", "src": "performance", "caps": "liveConfigEval",
          "label": I18n.tr("Low power mode"),
          "desc": I18n.tr("Turns every effect switch here on at once.") },
        { "tab": "", "group": I18n.tr("EFFECTS"), "key": "reduceMotion", "ctl": "sw", "src": "performance",
          "label": I18n.tr("Reduce motion"),
          "desc": I18n.tr("Shell transitions land instantly.") },
        { "tab": "", "group": I18n.tr("EFFECTS"), "key": "disableBlur", "ctl": "sw", "src": "performance", "caps": "liveConfigEval",
          "label": I18n.tr("Disable blur"),
          "desc": I18n.tr("Drops the frosted-glass look everywhere.") },
        { "tab": "", "group": I18n.tr("EFFECTS"), "key": "disableShadows", "ctl": "sw", "src": "performance", "caps": "liveConfigEval",
          "label": I18n.tr("Disable shadows"),
          "desc": I18n.tr("Surfaces draw without a shadow pass.") },

        { "tab": "", "group": I18n.tr("MOTION"), "key": "liveWallpaper60", "ctl": "sw", "src": "performance",
          "label": I18n.tr("60fps live wallpaper"),
          "desc": I18n.tr("Smoother video wallpaper, at a decode cost.") },
        { "tab": "", "group": I18n.tr("MOTION"), "key": "pauseLiveWallpaperWhenFullscreen", "ctl": "sw", "src": "performance",
          "label": I18n.tr("Pause video wallpaper"),
          "desc": I18n.tr("Stops the video behind a fullscreen window.") },
        { "tab": "", "group": I18n.tr("MOTION"), "key": "ambientBarMotion", "ctl": "sw", "src": "performance",
          "label": I18n.tr("Bar drifts when silent"),
          "desc": I18n.tr("The bar keeps drifting while nothing plays.") },

        { "tab": "", "group": I18n.tr("MEMORY"), "key": "unloadWidgetsWhenCovered", "ctl": "sw", "src": "performance",
          "label": I18n.tr("Hide covered widgets"),
          "desc": I18n.tr("Parks widgets while every monitor is covered.") },
        { "tab": "", "group": I18n.tr("MEMORY"), "key": "unloadVisualizerWhenSilent", "ctl": "sw", "src": "performance",
          "label": I18n.tr("Unload the visualiser"),
          "desc": I18n.tr("Frees ~250 MB after 30s of silence.") },
        { "tab": "", "group": I18n.tr("MEMORY"), "key": "unloadLauncherWhenIdle", "ctl": "sw", "src": "performance",
          "label": I18n.tr("Unload the launcher"),
          "desc": I18n.tr("Frees ~250 MB a minute after closing.") },
        { "tab": "", "group": I18n.tr("MEMORY"), "key": "unloadOverviewWhenIdle", "ctl": "sw", "src": "performance",
          "label": I18n.tr("Unload the overview"),
          "desc": I18n.tr("Frees ~250 MB a minute after Super+Tab closes.") }
    ]

    // Some effect switches are read by the compositor at config parse time, so
    // they only exist where the compositor evaluates its config live. Their caps
    // gate drops them on a compositor without it, the way SchemaPage filters a
    // row, instead of drawing a switch nothing on this desktop reads.
    readonly property var visibleSchema: pg.schema.filter(function (r) {
        return Settings.supports(r.caps, r.hostCaps);
    })

    // group order and membership come straight from the schema, so a regroup is
    // a data edit. groups keeps first-seen order (EYE CANDY, IDLE, MEMORY).
    readonly property var groups: {
        var g = [];
        for (var i = 0; i < pg.visibleSchema.length; i++)
            if (g.indexOf(pg.visibleSchema[i].group) < 0)
                g.push(pg.visibleSchema[i].group);
        return g;
    }
    function rowsIn(group) {
        return pg.visibleSchema.filter(function (r) { return r.group === group; });
    }

    // ── head: eyebrow, Fraunces title, blurb (matches every settings page) ──
    Column {
        id: head
        anchors.top: parent.top
        anchors.topMargin: Tokens.s6
        // the head sits on the body's grid, so the title starts over the first
        // card column instead of floating in the middle of a page-wide window
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
                text: UiPreferences.brandMark("力"); color: Tokens.ink; font.family: Tokens.jp
                font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: I18n.tr("SYSTEM"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Text {
            text: I18n.tr("Performance"); color: Tokens.ink
            font.family: Tokens.display; font.pixelSize: Tokens.fTitle
        }
        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("Less eye-candy for lower CPU, GPU and RAM use. Previewed live.")
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
        }
    }

    // ── the switch grid: three meaning-groups, each a SettingCard drawer that
    // stacks its compact SettingRows; membership comes straight from the schema. ──
    Flickable {
        id: flick
        anchors {
            left: parent.left; right: parent.right
            top: head.bottom; bottom: bar.top
            leftMargin: Tokens.s6; rightMargin: Tokens.s6
            topMargin: Tokens.s5
        }
        contentWidth: width
        contentHeight: Math.max(col.height, height)
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        CardColumns {
            id: col
            // the page fills its measure: groups land in as many columns as fit,
            // balanced, instead of one column beside an empty half
            width: flick.width - Tokens.s3
            spacing: Tokens.s5
            fillTo: flick.height

            Repeater {
                model: pg.groups

                delegate: SettingCard {
                    id: sect
                    required property string modelData
                    width: col.colWidth
                    title: sect.modelData
                    // the memory knobs park by default: reached for once, then
                    // left alone, so they open folded and stay out of the way
                    expanded: sect.modelData !== I18n.tr("MEMORY")
                    summary: pg.rowsIn(sect.modelData).length + " " + I18n.tr("SWITCHES")

                    Repeater {
                        model: pg.rowsIn(sect.modelData)

                        delegate: SettingRow {
                            id: cell
                            required property var modelData
                            required property int index
                            readonly property var r: cell.modelData

                            anchors.left: parent.left
                            anchors.right: parent.right
                            divider: cell.index > 0
                            controlWidth: 54
                            label: I18n.tr(cell.r.label)
                            desc: I18n.tr(cell.r.desc)
                            def: (pg.committed && pg.committed[cell.r.key]) ? I18n.tr("ON") : I18n.tr("OFF")
                            changed: !!(pg.draft && pg.committed) && pg.draft[cell.r.key] !== pg.committed[cell.r.key]
                            source: cell.r.src + ".json"

                            // inline switch, right-aligned in the reserved slot.
                            Sw {
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                on: !!(pg.draft && pg.draft[cell.r.key])
                                onToggled: (v) => pg.edit(cell.r.key, v)
                            }
                        }
                    }
                }
            }
        }
    }

    // ── action bar: dirty status left, Reset / Revert / Save right ──
    // full-bleed hides the shell's global bar, so this is the only way to
    // persist. RESET walks every key to stock (creating dirt), REVERT drops the
    // unsaved draft, SAVE writes it -- and reloads the compositor when needed.
    Rectangle {
        id: bar
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        height: 60
        color: "transparent"

        // hairline lid, like the shell's action bar (DESIGN.md section 8).
        Rectangle {
            anchors { left: parent.left; right: parent.right; top: parent.top }
            height: 1; color: Tokens.line
        }

        Row {
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s6
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3

            // filled ink while dirty, with a 600/600 heartbeat -- the one
            // perpetual animation allowed on an app surface; a hairline dot clean.
            Rectangle {
                id: dot
                anchors.verticalCenter: parent.verticalCenter
                width: 6; height: 6; radius: 3
                antialiasing: false
                readonly property bool lit: pg.dirtyCount > 0
                color: lit ? Tokens.ink : "transparent"
                border.width: lit ? 0 : Tokens.border
                border.color: Tokens.inkFaint

                SequentialAnimation on opacity {
                    running: pg.dirtyCount > 0
                    loops: Animation.Infinite
                    NumberAnimation { to: 0.3; duration: 600; easing.type: Easing.InOutSine }
                    NumberAnimation { to: 1.0; duration: 600; easing.type: Easing.InOutSine }
                    onStopped: dot.opacity = 1
                }
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: pg.dirtyCount > 0
                    ? (pg.dirtyCount === 1
                        ? I18n.tr("%1 CHANGE · PREVIEWING · NOT SAVED").arg(pg.dirtyCount)
                        : I18n.tr("%1 CHANGES · PREVIEWING · NOT SAVED").arg(pg.dirtyCount))
                    : I18n.tr("SAVED · LIVE ON YOUR DESKTOP")
                color: pg.dirtyCount > 0 ? Tokens.ink : Tokens.inkMuted
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
                armed: pg.offDefaults
                onAct: pg.reset()
            }
            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                width: 1; height: 20; color: Tokens.line
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("REVERT")
                armed: pg.dirtyCount > 0
                onAct: pg.revert()
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("SAVE")
                primary: true
                armed: pg.dirtyCount > 0
                onAct: pg.save()
            }
        }
    }
}
