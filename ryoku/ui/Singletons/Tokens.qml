pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

// The one place Ryoku's look is defined. Everything that draws imports this;
// nothing hardcodes a hex, a size or a duration.
//
// Colour resolves live from the daemon palette the same way the pill's Theme
// does: a fixed named colour scheme (shell.json themePalette) wins, then the
    // live wallpaper palette (~/.cache/ryoku/colors.json Material roles), then the
// compiled signature default. So every Ryoku app (the Hub, Ryowalls, ...)
// retints on ANY scheme change, a named theme or a wallpaper switch, with no
// colour math of its own. The kit's role names are kept; only their source moved
// onto the Material roles.
Singleton {
    id: t

    // Follow-the-wallpaper master (theme.json), the static named scheme's palette
    // (shell.json themePalette; null for the dynamic variants) and the live
    // wallpaper roles (colors.json), each parsed straight from the file text: the
    // "on" role names defeat JsonAdapter's signal-handler grammar, a removed
    // themePalette key only reads as absent from raw text, and a bare adapter does
    // not repopulate reliably for a lazily-created singleton.
    property bool matchWallpaper: true
    property var namedScheme: null
    property var wall: ({})

    // A palette value is only usable when it is a non-empty hex string; a null,
    // missing or half-written role falls through to the next layer, so a partial
    // colors.json (mid-write) or a scheme missing a role never paints black.
    function usable(v) { return typeof v === "string" && v.length > 0; }

    // Resolve one Material role through the layer chain: a named scheme wins,
    // then the live wallpaper palette while Match wallpaper is on, then the base.
    // Mid-cross-fade the wallpaper layer is a mix of the two palettes.
    function role(key, base) {
        if (namedScheme && usable(namedScheme[key]))
            return namedScheme[key];
        if (matchWallpaper && usable(wall[key])) {
            if (blend >= 0.999 || !usable(wallPrev[key]))
                return wall[key];
            return t.mixRole(Qt.color(wallPrev[key]), Qt.color(wall[key]), blend);
        }
        return base;
    }

    // ── default signature constants (the base each role falls back to) ───────
    readonly property color defaultPaper: "#000000"
    readonly property color defaultPaperLift: "#0a0a0a"
    readonly property color defaultInk: "#cdc4ba"
    readonly property color defaultInkDim: "#b0a9a0"

    // ── paper ────────────────────────────────────────────────────────────
    readonly property color paper: role("surface", defaultPaper)
    readonly property color paperLift: role("surfaceContainerLow", defaultPaperLift)

    // ── mode flag ────────────────────────────────────────────────────────
    // True when the resolved surface is light. Decoration authored for a dark
    // surface (emboss shadows, edge vignettes, light-on-dark placeholders) must
    // flip its scrim to stay legible in a light palette; this is that switch.
    readonly property bool light: (0.299 * paper.r + 0.587 * paper.g + 0.114 * paper.b) > 0.5

    // ── ink, on paper ────────────────────────────────────────────────────
    // Text uses the Material on-surface roles so it inverts with the palette; muted
    // and faint are the secondary role at reduced opacity, so they stay legible on a
    // light surface too (never the outline roles, which wash out on white).
    readonly property color ink: role("onSurface", defaultInk)
    readonly property color inkDim: role("onSurfaceVariant", defaultInkDim)
    // 0.88/0.68 over pure-black paper keep both tiers above the 4.5:1 legibility
    // floor the doc guarantees; the old 0.78/0.55 dipped under it once the
    // wallpaper dimmed onSurfaceVariant, which read as "options hard to see".
    readonly property color inkMuted: Qt.rgba(inkDim.r, inkDim.g, inkDim.b, 0.88)
    readonly property color inkFaint: Qt.rgba(inkDim.r, inkDim.g, inkDim.b, 0.68)

    // ── bone stock (inverted): the Material inverse-surface pair, so the light
    // plate and its dark ink keep contrast on a light OR dark theme ───────────
    readonly property color bone: role("inverseSurface", defaultInk)
    readonly property color inkOnBone: role("inverseOnSurface", "#000000")
    readonly property color inkOnBoneDim: Qt.rgba(inkOnBone.r, inkOnBone.g, inkOnBone.b, 0.62)
    readonly property color lineOnBone: Qt.rgba(inkOnBone.r, inkOnBone.g, inkOnBone.b, 0.26)

    // ── recording keycaps: fixed media stock, independent of wallpaper tint ─
    readonly property color keycapDark: "#17171a"
    readonly property color keycapLight: "#f4f2ed"
    readonly property color keycapOnDark: "#ffffff"
    readonly property color keycapOnLight: "#0d0d0f"

    // ── hairlines and tints (ink-derived, so they follow the resolved ink) ────
    readonly property color line: Qt.rgba(t.ink.r, t.ink.g, t.ink.b, 0.26)
    readonly property color lineSoft: Qt.rgba(t.ink.r, t.ink.g, t.ink.b, 0.13)
    readonly property color lineStrong: Qt.rgba(t.ink.r, t.ink.g, t.ink.b, 0.42)
    readonly property color tint5: Qt.rgba(t.ink.r, t.ink.g, t.ink.b, 0.05)   // surface hover
    readonly property color tint10: Qt.rgba(t.ink.r, t.ink.g, t.ink.b, 0.10)  // control hover
    readonly property color tint16: Qt.rgba(t.ink.r, t.ink.g, t.ink.b, 0.16)  // pressed

    // ── colour: sun is the live accent (the palette primary); alert stays a
    // fixed attention red so a badge always reads as an alert ─────────────────
    readonly property color sun: role("primary", "#e2342a")
    readonly property color sunDeep: Qt.darker(sun, 1.3)
    readonly property color alert: "#e2342a"

    // ── type ─────────────────────────────────────────────────────────────
    readonly property string display: "Fraunces"
    property string ui: "Space Grotesk"
    property string mono: "SpaceMono Nerd Font"
    readonly property string jp: "Noto Sans CJK JP"

    // ── chrome ───────────────────────────────────────────────────────────
    // One setting voice: the quiet, function-first one. The poster layer that
    // once dressed the sheet (register crosshairs, film grain, chapter plates,
    // the oversized display title) is gone, and with it the `hubDecor` switch
    // that used to trade it on, so a page reads as a printed instrument sheet
    // and the ornament question does not exist.
    readonly property int fTitle: px(32)    // page title, Fraunces
    readonly property int fHero: px(34)     // a headline readout
    readonly property int fValue: px(26)    // a cell's value
    readonly property int fRow: px(15)      // a row name
    readonly property int fBody: px(14)
    readonly property int fSmall: px(13)    // descriptions
    readonly property int fMicro: px(11)    // tracked labels
    readonly property int fTiny: px(9)      // corner tags, struck defaults

    readonly property real trackLabel: 1.4   // letter-spacing for micro labels
    readonly property real trackMark: 2.2    // for eyebrows and section marks

    // ── space ────────────────────────────────────────────────────────────
    readonly property int s1: px(4)
    readonly property int s2: px(8)
    readonly property int s3: px(12)
    readonly property int s4: px(16)
    readonly property int s5: px(24)
    readonly property int s6: px(32)
    readonly property int s7: px(48)

    // ── geometry ─────────────────────────────────────────────────────────
    readonly property int radius: px(6)
    readonly property real border: 1
    readonly property int rowH: px(48)
    readonly property int cellH: px(104)
    readonly property int railW: px(268)
    // The widest a stack of settings rows reads at before a label and its control
    // stop being one thing. A page may cap its own column here and centre it.
    readonly property int contentMax: px(1000)
    // The widest a card grows when a page has few of them: the grid narrows its
    // column count and lets the cards take the space, rather than leaving a
    // window-wide gap beside a lone card.
    readonly property int cardWide: px(760)
    readonly property int ctlH: px(26)

    // ── motion ───────────────────────────────────────────────────────────
    // dur() scales every duration by motionScale (shell.json theme.motion) and
    // zeroes it under reduceMotion, so the Hub retimes the desktop live.
    property real motionScale: 1.0
    property bool reduceMotion: false
    function dur(ms) { return reduceMotion ? 0 : Math.round(ms * motionScale); }

    // ── per-monitor UI scale ─────────────────────────────────────────────
    // shell.json displays.ui_scale, keyed by output name. Surfaces multiply
    // their own sizes by uiScaleFor(screen) so one monitor's chrome shrinks
    // without touching the compositor scale that apps depend on.
    property var uiScales: ({})
    property var barVisibility: ({})
    property var widgetVisibility: ({})

    function uiScaleFor(name) {
        var v = (name && uiScales) ? uiScales[name] : undefined;
        if (typeof v !== "number" || !(v > 0))
            return 1;
        return Math.max(0.5, Math.min(2, v));
    }

    function barEnabledFor(name) {
        var v = (name && barVisibility) ? barVisibility[name] : undefined;
        return v !== false;
    }

    function widgetsEnabledFor(name) {
        var v = (name && widgetVisibility) ? widgetVisibility[name] : undefined;
        return v !== false;
    }

    // A single-window process (the Hub) sets uiScale to scale its whole UI at
    // once; the multi-monitor shell leaves it 1 and scales each surface via
    // uiScaleFor. px() folds it into the size tokens below.
    property real uiScale: 1
    function px(n) { return Math.round(n * uiScale); }

    readonly property int snap: dur(90)     // hover, press, state flip
    readonly property int move: dur(170)    // a selector travelling
    readonly property int swap: dur(210)    // content exchanging
    readonly property int flap: dur(110)    // a value changing
    readonly property int ease: Easing.OutCubic
    readonly property int easeSnap: Easing.OutQuad

    // Material 3 expressive durations and curves (easing.bezierCurve points) that
    // Anim selects by role. Ported from caelestia-dots/shell (Config/tokens.hpp).
    readonly property int durSmall: dur(200)
    readonly property int durNormal: dur(400)
    readonly property int durLarge: dur(600)
    readonly property int durXl: dur(1000)
    readonly property int durFastSpatial: dur(350)
    readonly property int durDefaultSpatial: dur(500)
    readonly property int durSlowSpatial: dur(650)
    readonly property int durFastEffects: dur(150)
    readonly property int durDefaultEffects: dur(200)
    readonly property int durSlowEffects: dur(300)

    readonly property var curveStandard: [0.2, 0, 0, 1, 1, 1]
    readonly property var curveEmphasized: [0.05, 0, 0.133333, 0.06, 0.166667, 0.4, 0.208333, 0.82, 0.25, 1, 1, 1]
    readonly property var curveFastSpatial: [0.42, 1.67, 0.21, 0.9, 1, 1]
    readonly property var curveDefaultSpatial: [0.38, 1.21, 0.22, 1, 1, 1]
    readonly property var curveSlowSpatial: [0.39, 1.29, 0.35, 0.98, 1, 1]
    readonly property var curveFastEffects: [0.31, 0.94, 0.34, 1, 1, 1]
    readonly property var curveDefaultEffects: [0.34, 0.8, 0.34, 1, 1, 1]
    readonly property var curveSlowEffects: [0.34, 0.88, 0.34, 1, 1, 1]

    // ── grain ────────────────────────────────────────────────────────────
    // Art only: the film tooth rides a recording thumbnail or a launcher
    // preview, never the paper a setting is read on (see Doc/ui-ux.md).
    readonly property real grainOpacity: 0.10

    // ── palette cross-fade ───────────────────────────────────────────────────
    // A new palette used to land in one frame while the wallpaper it came from was
    // still wiping in. `blend` walks the roles across instead, so the ink arrives
    // with the picture. Every role is a mix while it runs, so every binding that
    // reads one re-evaluates per frame: bounded to the change, skipped when motion
    // is reduced.
    property var wallPrev: ({})
    property real blend: 1
    // an explicit animation, not a Behavior: assigning 0 then 1 in one block never
    // leaves the property at 0, so a Behavior would animate 1 to 1 and show nothing
    NumberAnimation {
        id: blendWalk
        target: t
        property: "blend"
        from: 0
        to: 1
        duration: t.durSlowEffects
        easing.type: Easing.Bezier
        easing.bezierCurve: t.curveDefaultEffects
    }

    function mixRole(from, to, at) {
        return Qt.rgba(from.r + (to.r - from.r) * at,
                       from.g + (to.g - from.g) * at,
                       from.b + (to.b - from.b) * at,
                       from.a + (to.a - from.a) * at);
    }

    // ── daemon palette readers ───────────────────────────────────────────────
    function refreshWall() {
        var next = ({});
        try {
            const txt = paletteFile.text();
            next = txt && txt.length ? (JSON.parse(txt) || {}) : {};
        } catch (e) {
            next = {};
        }
        // the session's first palette has nothing to come from: land it solid
        const first = Object.keys(t.wall).length === 0;
        t.wallPrev = t.wall;
        t.wall = next;
        if (first || t.reduceMotion) {
            blendWalk.stop();
            t.blend = 1;
            return;
        }
        blendWalk.restart();
    }
    function refreshNamed() {
        var pal = null;
        var scale = 1.0;
        var reduce = false;
        var scales = ({});
        var bars = ({});
        var widgets = ({});
        var monoFont = "SpaceMono Nerd Font";
        var uiFont = "Space Grotesk";
        try {
            const txt = shellFile.text();
            if (txt) {
                const o = JSON.parse(txt);
                if (o && typeof o.themePalette === "object" && o.themePalette !== null)
                    pal = o.themePalette;
                const m = o && o.theme && o.theme.motion;
                if (m) {
                    if (typeof m.scale === "number" && m.scale > 0)
                        scale = m.scale;
                    reduce = m.reduce === true;
                }
                const displays = o && o.displays;
                const u = displays && displays.ui_scale;
                if (u && typeof u === "object" && u !== null)
                    scales = u;
                const b = displays && displays.bar;
                if (b && typeof b === "object" && b !== null)
                    bars = b;
                const w = displays && displays.widgets;
                if (w && typeof w === "object" && w !== null)
                    widgets = w;
                if (typeof o.fontFamily === "string" && o.fontFamily.length) { uiFont = o.fontFamily; monoFont = o.fontFamily; }
            }
        } catch (e) {
            pal = null;
        }
        t.namedScheme = pal;
        t.motionScale = scale;
        t.reduceMotion = reduce;
        t.uiScales = scales;
        t.barVisibility = bars;
        t.widgetVisibility = widgets;
        t.ui = uiFont;
        t.mono = monoFont;
    }
    function refreshMatch() {
        try {
            const txt = themeFile.text();
            const o = txt ? JSON.parse(txt) : null;
            // Default ON when theme.json is absent or omits the key, matching
            // services/Config.qml and the daemon's matchWallpaperOn: a fresh box
            // follows the wallpaper. Only an explicit false locks it off.
            t.matchWallpaper = (o && typeof o.followWallpaper === "boolean") ? o.followWallpaper : true;
        } catch (e) {
            t.matchWallpaper = true;
        }
    }

    FileView {
        id: themeFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/theme.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: t.refreshMatch()
    }
    FileView {
        id: shellFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/shell.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: t.refreshNamed()
    }
    FileView {
        id: paletteFile
        path: (Quickshell.env("XDG_CACHE_HOME") || (Quickshell.env("HOME") + "/.cache")) + "/ryoku/colors.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: t.refreshWall()
    }

    Component.onCompleted: {
        refreshWall();
        refreshNamed();
        refreshMatch();
    }
}
