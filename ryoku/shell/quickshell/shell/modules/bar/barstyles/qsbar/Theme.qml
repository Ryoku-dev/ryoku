import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Services.UPower
import shell.services
import Ryoku.Ui.Singletons
import "core"
import "Palette.js" as Palette

Item {
    id: theme
    // The single qsbar bar system; the bar form lives in `barShellStyle`.

    property string omarchyCurrentRoot: Quickshell.env("HOME") + "/.config/ryoku/current"
    property string omarchyInstallRoot: Quickshell.env("HOME") + "/.local/share/ryoku"
    property bool omarchyCurrentRootResolved: false
    readonly property string themeNamePath: omarchyCurrentRoot + "/theme.name"
    readonly property string colorsPath: (Quickshell.env("XDG_CACHE_HOME") || (Quickshell.env("HOME") + "/.cache")) + "/ryoku/colors.json"
    readonly property string shellConfigPath: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/shell.json"
    readonly property string themeJsonPath: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/theme.json"
    // Palette resolution state (the desktop Scheme's chain): the live wallpaper
    // roles, the fixed named scheme, the follow-the-wallpaper master, and the
    // compiled base captured before the daemon's first write so turning the
    // wallpaper off reverts to the shipped look.
    property var _wallColors: ({})
    property var _namedPalette: null
    property bool _followWallpaper: true
    property var _basePalette: null
    readonly property string currentBackgroundPath: omarchyCurrentRoot + "/background"
    property string currentThemeName: ""
    readonly property var wallpaperSourcePaths: [
        Quickshell.env("HOME") + "/Pictures/Wallpapers",
        Quickshell.env("HOME") + "/Pictures/livewalls"
    ]

    function setCurrentThemeName(rawName) {
        var name = String(rawName || "").trim()
        if (name === "." || name === ".." || name.indexOf("/") >= 0 || name.indexOf("\\") >= 0)
            name = ""
        currentThemeName = name
    }

    function reloadCurrentThemeFiles() {
        if (!omarchyCurrentRootResolved) return
        themeReloadDebounce.restart()
    }

    property color paper:   "#181616"
    property color ink:     "#c5c9c5"
    property color sumi:    "#a6a69c"
    property color color01: "#c4746e"
    property color color02: "#8a9a73"
    property color color03: "#c8b36a"
    property color color04: "#658594"
    property color color05: "#957fb8"
    property color color06: "#7aa89f"
    property color color07: "#c8c093"
    readonly property color inkDeep: color07
    readonly property color indigo:  color04
    readonly property color sealRaw: color01
    readonly property color sumiHi:  Qt.rgba(sumi.r*0.45 + ink.r*0.55, sumi.g*0.45 + ink.g*0.55, sumi.b*0.45 + ink.b*0.55, 1.0)  // lifted section-header text
    property color green:   "#8a9a73"   // gate "OK" verdict
    property color accentHint: sealRaw    // filled by palette; default = same as red
    readonly property color foregroundSoft: Qt.rgba(
        ink.r * 0.88 + paper.r * 0.12,
        ink.g * 0.88 + paper.g * 0.12,
        ink.b * 0.88 + paper.b * 0.12,
        1.0)
    property string barColor: "color01"
    readonly property bool barColorIsAccent: barColor === "accent"
    // Compatibility alias for older local code/reviews that still use the
    // previous boolean name.
    readonly property bool useThemeAccent: barColorIsAccent

    function paletteColor(id) {
        if (id === "color02") return color02
        if (id === "color03") return color03
        if (id === "color04") return color04
        if (id === "color05") return color05
        if (id === "color06") return color06
        if (id === "color07") return color07
        if (id === "foreground") return foregroundSoft
        if (id === "accent") return accentHint
        return color01
    }
    // "accent" means follow the wallpaper's own accent, so it is a real choice and
    // must survive the cold-start cache; only the legacy "red" collapses.
    function normalizedPaletteId(id) {
        if (id === "accent") return "accent"
        if (id === "red") return "color01"
        return paletteColorValid(id) ? id : "color01"
    }
    readonly property color seal: paletteColor(barColor)
    // Legacy global foreground switch is retained only for cache compatibility.
    // Widget colors now inherit Bar Color or use a per-GID palette style.
    readonly property color widgetIconColor: seal
    readonly property var barColorOptions: [
        "color01", "color02", "color03", "color04",
        "color05", "color06", "color07", "foreground"
    ]
    function paletteColorValid(id) {
        return id === "color01" || id === "color02" || id === "color03"
            || id === "color04" || id === "color05" || id === "color06"
            || id === "color07" || id === "foreground"
    }
    function barColorValid(id) {
        return paletteColorValid(id) || id === "red" || id === "accent"
    }
    function barColorLabel(id) {
        if (id === "color01" || id === "red" || id === "accent") return I18n.tr("Color 01")
        if (id === "color02") return I18n.tr("Color 02")
        if (id === "color03") return I18n.tr("Color 03")
        if (id === "color04") return I18n.tr("Color 04")
        if (id === "color05") return I18n.tr("Color 05")
        if (id === "color06") return I18n.tr("Color 06")
        if (id === "color07") return I18n.tr("Color 07")
        if (id === "foreground") return I18n.tr("Foreground")
        return I18n.tr("Color 01")
    }

    readonly property string mono:  "JetBrainsMono Nerd Font"

    // ── transparency knobs (0.0 = fully transparent, 1.0 = opaque) ──
    property real barOpacity:  0.94   // durchgehende V2-Leiste
    property real pillOpacity: 0.18   // einzelne Widget-Pillen (workspace, mem, cpu, …)

    // Frost overrides the user's barOpacity with the reference's island alpha.
    readonly property real surfaceOpacity: barFrostEnabled ? 0.68 : barOpacity
    readonly property color bg:     Qt.rgba(paper.r, paper.g, paper.b, surfaceOpacity)
    readonly property color barBg:  Qt.rgba(paper.r, paper.g, paper.b, surfaceOpacity)
    readonly property color pill:   Qt.rgba(paper.r, paper.g, paper.b, pillOpacity)
    readonly property color fg:     ink
    readonly property color muted:  sumi
    readonly property color accent: seal
    readonly property color warn:   seal
    // A genuine danger red. The palette is generated from the wallpaper, so no slot
    // in it can be relied on to read as danger: under a blue scheme every slot is
    // blue. The hue is fixed and only the lightness follows the theme, so boosted
    // volume looks risky whatever the wallpaper is.
    readonly property color danger: Qt.hsla(0.995, 0.68, paper.hslLightness > 0.5 ? 0.46 : 0.60, 1)
    readonly property color sep:    Qt.rgba(ink.r, ink.g, ink.b, 0.18)

    // ── interactive fill tokens (button/tile backgrounds) ──
    // One source of truth so every panel uses the same hover/active/idle alpha
    // instead of ad-hoc rgba literals scattered across the panels.
    readonly property real  fillActiveAlpha: 0.18
    readonly property real  fillHoverAlpha:  0.10
    readonly property color fillActive:      Qt.rgba(seal.r, seal.g, seal.b, fillActiveAlpha) // selected/active OR ghost-action hover
    readonly property color fillHover:        Qt.rgba(seal.r, seal.g, seal.b, fillHoverAlpha)  // light-seal hover (idle chip → this → fillActive)
    readonly property color fillIdle:         Qt.rgba(0, 0, 0, 0.12)              // resting chip (slight darken)
    // faint, NEUTRAL backdrop behind picker thumbnails - NOT an interactive fill.
    // ink-tinted and much weaker than fillIdle so a thumbnail sits on a quiet frame,
    // not on a dark interactive-looking box.
    readonly property color frameWeak:        Qt.rgba(ink.r, ink.g, ink.b, 0.05)
    readonly property color fillPrimaryHover: Qt.lighter(seal, 1.15)                // solid-seal button hover
    function evenW(w) { return 2 * Math.round(w / 2) }  // even px width -> integer-centered native text (crisp)

    // ── Multi-monitor popup routing ─────────────────────────────
    // Bars exist per screen, but panels remain singletons. The bar under the
    // pointer publishes its screen + local anchor map before any widget opens a
    // popup, so the singleton panel can move to the correct output.
    property var activePopupScreen: null
    property string activePopupScreenName: ""
    property var barAnchorsByScreen: ({})
    property bool _closingPopups: false
    property var barLayoutControllers: ({})
    property bool _barLayoutSyncing: false

    readonly property bool anyPopupVisible: dashboardVisible || cpuVisible || gpuVisible
        || thermalVisible || aiUsageVisible
        || memVisible || volVisible || controlVisible || networkVisible || bluetoothVisible
        || batteryVisible || brightnessVisible || mprisVisible
        || workspaceVisible || imagePickerVisible || mediaBrowserVisible || notifVisible
        || powerProfileVisible || storageVisible || trayVisible || trayMenuVisible
    readonly property bool keyboardPopupVisible: imagePickerVisible || mediaBrowserVisible

    function registerBarLayoutController(screenName, controller) {
        if (!screenName || !controller) return

        var next = {}
        for (var screen in barLayoutControllers) next[screen] = barLayoutControllers[screen]
        next[screenName] = controller
        barLayoutControllers = next
    }

    function unregisterBarLayoutController(screenName, controller) {
        if (!screenName) return
        if (controller && barLayoutControllers[screenName] !== controller) return

        var next = {}
        for (var screen in barLayoutControllers) {
            if (screen !== screenName) next[screen] = barLayoutControllers[screen]
        }
        barLayoutControllers = next
    }

    function barLayoutControllerScreenValid(screenName) {
        if (!screenName) return false

        for (var i = 0; i < Quickshell.screens.length; i++) {
            var screen = Quickshell.screens[i]
            if (screen.name === screenName && screen.width > 0 && screen.height > 0) return true
        }
        return false
    }

    function barLayoutControllerKeys() {
        var keys = []
        for (var screen in barLayoutControllers) {
            if (barLayoutControllerScreenValid(screen)) keys.push(screen)
        }
        keys.sort()
        return keys
    }

    function _applyLayoutToBars(exceptScreen) {
        var keys = barLayoutControllerKeys()

        _barLayoutSyncing = true
        try {
            for (var i = 0; i < keys.length; i++) {
                if (exceptScreen && keys[i] === exceptScreen) continue
                var controller = barLayoutControllers[keys[i]]
                if (controller && controller.applyLayout) controller.applyLayout()
            }
        } finally {
            _barLayoutSyncing = false
        }
    }

    // Legacy combined reset: shipped order and visibility PLUS presentation.
    // The panel's own RESET LAYOUT calls barLayoutReset(); this stays for any
    // caller that wants the whole bar returned to shipped in one move.
    function resetAllBarLayouts() {
        barLayoutReset()
        resetBarLayoutPresentation()
    }

    function resetBarLayoutPresentation() {
        var separatorsChanged = barSeps.length > 0
        var densityChanged = iconOnlyGids.length > 0
        var widgetFillsChanged = resetAllWidgetFillColors()
        var mprisChanged = mprisBarStyle !== "default"

        if (separatorsChanged) barSeps = []
        if (densityChanged) iconOnlyGids = []

        // mprisBarStyle has its own persistence handler. If it was already at
        // the default, persist the other layout-only resets explicitly.
        if (mprisChanged) mprisBarStyle = "default"
        else if ((separatorsChanged || densityChanged || widgetFillsChanged)
                && _widgetsLoaded) saveWidgets()
    }

    // ─────────────────────── bar layout as data (qsbar.layout) ───────────────
    // The bar's widget order lives in shell.json under qsbar.layout, one document
    // of { version, left, center, right } id arrays. Built-in ids come from the
    // catalogue (BarCatalog); a plugin id is an installed plugin enabled on the
    // bar. `_storedLayout` is the raw persisted document; `barLayout` is the
    // live, normalised view every consumer reads (duplicates dropped, known
    // widgets the layout omits appended). Visibility is separate: a built-in
    // stays in the layout when hidden, a plugin drops out when it is not enabled
    // on the bar.
    BarCatalog { id: barCat }
    BarPlugins { id: barPluginHost }

    property var _storedLayout: null
    property bool barLayoutReady: false
    property bool _barLayoutInitialized: false
    readonly property var barLayout: _normalizeLayout(_storedLayout)

    // The set of ids that may hold a place in the bar: every built-in (always,
    // even hidden) plus every plugin currently enabled and hosted on the bar.
    readonly property var barPluginIds: barPluginHost.pluginIds
    // Installed plugins whose manifest declares topbarGlyph (installed, not
    // necessarily on the bar) - the add-widget picker's plugin group.
    readonly property var barPluginCatalog: barPluginHost.barCapable
    function barPluginEntryFor(id) { return barPluginHost.entryFor(id) }
    function barPluginIsBar(id) { return barPluginHost.isEnabledBar(id) }

    // A plugin enabled after the layout was written appears live and lands at the
    // end of its section; it is not persisted until the next explicit edit, so
    // toggling a plugin never races the daemon's shell.json writes. The bars
    // follow barLayout itself, not barPluginIds: the normalised layout re-derives
    // after the id set changes, and a handler on the ids ran before that
    // re-derivation and handed the bars the old arrays (the plugin never drew).
    onBarLayoutChanged: if (barLayoutReady) _applyLayoutToBars("")

    function _eligibleIds() {
        var m = ({})
        var ents = barCat.entries
        for (var i = 0; i < ents.length; i++) m[ents[i].id] = true
        var pids = barPluginIds
        for (var j = 0; j < pids.length; j++) m[pids[j]] = true
        return m
    }
    function _eligibleOrder() {
        var out = []
        var ents = barCat.entries
        for (var i = 0; i < ents.length; i++) out.push(ents[i].id)
        var pids = barPluginIds
        for (var j = 0; j < pids.length; j++) out.push(pids[j])
        return out
    }
    // Built-ins have no manifest, so a missing built-in falls to the right lane;
    // a plugin honours its manifest defaults.bar.section when it names one.
    function _defaultSectionFor(id) {
        if (barCat.byId(id)) return "right"
        var e = barPluginEntryFor(id)
        var sec = (e && e.manifest && e.manifest.defaults && e.manifest.defaults.bar)
            ? e.manifest.defaults.bar.section : ""
        return (sec === "left" || sec === "center" || sec === "right") ? sec : "right"
    }
    function _normalizeLayout(raw) {
        var secNames = ["left", "center", "right"]
        var out = { version: 1, left: [], center: [], right: [] }
        var used = ({})
        var eligible = _eligibleIds()
        for (var s = 0; s < 3; s++) {
            var arr = (raw && raw[secNames[s]]) || []
            for (var i = 0; i < arr.length; i++) {
                var id = arr[i]
                if (eligible[id] && !used[id]) { out[secNames[s]].push(id); used[id] = true }
            }
        }
        var order = _eligibleOrder()
        for (var k = 0; k < order.length; k++) {
            var mid = order[k]
            if (used[mid]) continue
            out[_defaultSectionFor(mid)].push(mid)
            used[mid] = true
        }
        return out
    }
    function _cloneLayout(L) {
        return {
            version: 1,
            left: ((L && L.left) || []).slice(),
            center: ((L && L.center) || []).slice(),
            right: ((L && L.right) || []).slice()
        }
    }
    function _sectionIndexOf(L, id) {
        var secNames = ["left", "center", "right"]
        for (var s = 0; s < 3; s++) {
            var arr = (L && L[secNames[s]]) || []
            var idx = arr.indexOf(id)
            if (idx >= 0) return { section: secNames[s], index: idx }
        }
        return { section: "", index: -1 }
    }

    // The shipped default order (design section 1). Presentation keys (separators,
    // density, per-widget colour) are untouched by a layout reset.
    function _shippedLayout() {
        return {
            version: 1,
            left: ["launcher", "workspaces", "status", "cpu", "volume", "memory", "ai"],
            center: ["clock"],
            right: ["media", "quick", "network", "power", "battery", "brightness",
                    "cputemp", "storage", "gpu", "bluetooth", "layout"]
        }
    }

    // Convert the retired ~/.cache/quickshell_barorder_v2 string
    // ("B:G1,B:G16,...|B:G8|B:G9,...") to id arrays via the catalogue's gid map.
    // Empty cells ("_"/"") and the B:/E: slot-kind prefixes are dropped: the new
    // layout carries placement only. This is the function the migration cites.
    function _cacheToLayout(str) {
        var parts = String(str).split("|")
        if (parts.length !== 3) return null
        var secNames = ["left", "center", "right"]
        var out = { version: 1, left: [], center: [], right: [] }
        for (var s = 0; s < 3; s++) {
            var raw = parts[s]
            var tokens = raw === "" ? [] : raw.split(",")
            for (var i = 0; i < tokens.length; i++) {
                var tok = tokens[i]
                var gid = (tok.length >= 2 && tok.charAt(1) === ":") ? tok.substring(2) : tok
                if (gid === "" || gid === "_") continue
                var id = barCat.idOf(gid)
                if (id) out[secNames[s]].push(id)
            }
        }
        return out
    }

    function _commitLayout(raw, persist) {
        barLayoutReady = true
        _barLayoutInitialized = true
        _storedLayout = raw ? raw : { version: 1, left: [], center: [], right: [] }
        if (persist) _persistLayout(barLayout)
    }
    function _persistLayout(L) {
        var v = { version: 1, left: L.left, center: L.center, right: L.right }
        _cfgCtl.queued += "call settings.patch "
            + JSON.stringify({ path: "qsbar.layout", value: v }) + "\n"
        if (_cfgCtl.connected) _cfgCtl.flushQueued()
        else _cfgCtl.connected = true
    }

    // First load with no qsbar.layout: migrate the cache once (or write the
    // shipped default), persist it, then delete the cache. Runs exactly once.
    readonly property string _barOrderCachePath: Quickshell.env("HOME") + "/.cache/quickshell_barorder_v2"
    function _migrateBarLayout() {
        if (_barLayoutInitialized) return
        _barLayoutInitialized = true
        _barCacheLoadProc.running = true
    }
    Process {
        id: _barCacheLoadProc
        command: ["cat", theme._barOrderCachePath]
        stdout: StdioCollector {
            onStreamFinished: {
                var t = (this.text || "").trim()
                var migrated = t.length > 0 ? theme._cacheToLayout(t) : null
                theme._commitLayout(migrated ? migrated : theme._shippedLayout(), true)
                theme._applyLayoutToBars("")
                Quickshell.execDetached(["rm", "-f", theme._barOrderCachePath])
            }
        }
    }

    // ── the merged catalogue every panel/CLI reads (design section 4) ──
    readonly property var barCatalog: _buildBarCatalog()
    function _buildBarCatalog() {
        var out = []
        var L = barLayout
        var ents = barCat.entries
        for (var i = 0; i < ents.length; i++) {
            var e = ents[i]
            var si = _sectionIndexOf(L, e.id)
            out.push({
                id: e.id, gid: e.gid || "",
                label: e.label || e.id, gloss: e.gloss || "",
                category: e.category || "", desc: e.desc || "",
                kind: "builtin", official: true, author: "", version: "",
                shown: e.visKey ? _modForVisKey(e.visKey) : true,
                section: si.section, index: si.index,
                settings: e.settings || [], pluginDir: ""
            })
        }
        var cap = barPluginCatalog
        for (var j = 0; j < cap.length; j++) {
            var p = cap[j]
            var man = p.manifest || ({})
            var pl = p.placement || ({})
            var psi = _sectionIndexOf(L, p.id)
            // `official` is the manifest's own claim; Ryoku's plugins ship it
            // true, so anything else is a community widget and the panel parts it.
            out.push({
                id: p.id, gid: "",
                label: man.name || p.id, gloss: "",
                category: "", desc: man.description || "",
                kind: "plugin", official: man.official === true,
                author: man.author || "", version: man.version || "",
                shown: pl.enabled === true && pl.host === "topbarGlyph",
                section: psi.section, index: psi.index,
                settings: (man.metadata && man.metadata.settings) || [],
                pluginDir: p.dir || ""
            })
        }
        return out
    }

    // ── the root API BarSettings and the qsbar IPC drive (design section 4) ──
    signal barSettingsOpenRequested(string route)
    function openBarSettings(route) {
        barSettingsOpenRequested((route === undefined || route === null) ? "" : String(route))
    }
    function gidForId(id) { return barCat.gidOf(id) }
    function idForGid(gid) { return barCat.idOf(gid) }

    function barLayoutMove(id, section, index) {
        if (!id || ["left", "center", "right"].indexOf(section) < 0) return
        var L = _cloneLayout(barLayout)
        var secs = ["left", "center", "right"]
        for (var s = 0; s < 3; s++)
            L[secs[s]] = L[secs[s]].filter(function (x) { return x !== id })
        var arr = L[section]
        var idx = (index === undefined || index === null) ? arr.length
            : Math.max(0, Math.min(index, arr.length))
        arr.splice(idx, 0, id)
        _commitLayout(L, true)
        _applyLayoutToBars("")
    }

    function barLayoutReset() {
        _applyShippedVisibility()
        _commitLayout(_shippedLayout(), true)
        _applyLayoutToBars("")
    }
    function _applyShippedVisibility() {
        var wl = _widgetsLoaded
        _widgetsLoaded = false
        modStatus = true; modMemory = true; modCpu = true; modVolume = true
        modWeather = true; modNetwork = true; modBrightness = true; modMedia = true
        modMpris = true; modQuick = true; modBattery = true; modLayout = true
        modCpuTemperature = true; modGpu = true; modStorage = true
        modClaude = false; modPower = false; modBluetooth = false
        _widgetsLoaded = wl
        if (wl) saveWidgets()
    }

    // Called by a BarSlot when its in-place drag reorders a lane: persist the
    // whole layout, then re-apply to every OTHER monitor (the source already
    // reflects the drag, so re-applying to it would repack it mid-edit).
    function saveBarLayoutFromSlot(raw, sourceScreen) {
        _commitLayout(raw, true)
        _applyLayoutToBars(sourceScreen)
    }

    function barLayoutShow(id, on) {
        var e = barCat.byId(id)
        if (e) {
            if (!e.visKey) return   // launcher/workspaces/clock are always shown
            _setVisByKey(e.visKey, on === true)
            return
        }
        if (on === true) {
            _placePlugin([id, "host", "topbarGlyph"])
            _placePlugin([id, "enabled", "true"])
        } else {
            _placePlugin([id, "enabled", "false"])
        }
    }

    function barWidgetGet(id, key) {
        var e = barCat.byId(id)
        if (e) {
            if (_isVisKeySetting(id, key)) return _modForVisKey(key)
            return theme[key]
        }
        var pe = barPluginEntryFor(id)
        if (pe && pe.placement && pe.placement.settings
                && pe.placement.settings[key] !== undefined)
            return pe.placement.settings[key]
        return undefined
    }
    function barWidgetSet(id, key, value) {
        var e = barCat.byId(id)
        if (e) {
            if (_isVisKeySetting(id, key)) { _setVisByKey(key, value === true); return }
            theme[key] = value   // the change handler persists through saveWidgets
            return
        }
        var obj = ({}); obj[key] = value
        _placePlugin([id, "settings", JSON.stringify(obj)])
    }

    // A catalogue settings row with visKey:true (clock's Weather) toggles a
    // widgets visibility key instead of a plain Theme property.
    function _isVisKeySetting(id, key) {
        var e = barCat.byId(id)
        if (!e || !e.settings) return false
        for (var i = 0; i < e.settings.length; i++)
            if (e.settings[i].key === key && e.settings[i].visKey === true) return true
        return false
    }
    // Direct (not bracket) property access so barCatalog's `shown` binding
    // captures the mod* dependency and re-derives when visibility changes.
    function _modForVisKey(visKey) {
        switch (visKey) {
        case "status": return modStatus
        case "memory": return modMemory
        case "cpu": return modCpu
        case "volume": return modVolume
        case "weather": return modWeather
        case "network": return modNetwork
        case "brightness": return modBrightness
        case "media": return modMedia
        case "mpris": return modMpris
        case "quick": return modQuick
        case "claude": return modClaude
        case "power": return modPower
        case "bluetooth": return modBluetooth
        case "gpu": return modGpu
        case "cpuTemperature": return modCpuTemperature
        case "storage": return modStorage
        case "battery": return modBattery
        case "layout": return modLayout
        }
        return true
    }
    function _setVisByKey(visKey, on) {
        switch (visKey) {
        case "status": modStatus = on; break
        case "memory": modMemory = on; break
        case "cpu": modCpu = on; break
        case "volume": modVolume = on; break
        case "weather": modWeather = on; break
        case "network": modNetwork = on; break
        case "brightness": modBrightness = on; break
        case "media": modMedia = on; break
        case "mpris": modMpris = on; break
        case "quick": modQuick = on; break
        case "claude": modClaude = on; break
        case "power": modPower = on; break
        case "bluetooth": modBluetooth = on; break
        case "gpu": modGpu = on; break
        case "cpuTemperature": modCpuTemperature = on; break
        case "storage": modStorage = on; break
        case "battery": modBattery = on; break
        case "layout": modLayout = on; break
        }
    }

    readonly property string _pluginShellDir: Quickshell.env("RYOKU_SHELL_DIR")
    readonly property string _pluginPlaceTool: (_pluginShellDir && _pluginShellDir.length > 0)
        ? _pluginShellDir + "/quickshell/plugins/ryoku-plugins-place"
        : "ryoku-plugins-place"
    function _placePlugin(args) { Quickshell.execDetached([_pluginPlaceTool].concat(args)) }

    function activatePopupScreen(screen) {
        if (!screen || screen.name === "") return

        activePopupScreen = screen
        activePopupScreenName = screen.name
        applyActiveBarAnchors()
    }

    function activateFocusedPopupScreen() {
        var targetName = Wm.focusedOutput

        for (var i = 0; i < Quickshell.screens.length; i++) {
            var candidate = Quickshell.screens[i]
            if (candidate.name === targetName
                    && candidate.width > 0
                    && candidate.height > 0) {
                activatePopupScreen(candidate)
                return true
            }
        }

        if (activePopupScreenName !== "") return true

        for (var j = 0; j < Quickshell.screens.length; j++) {
            var fallback = Quickshell.screens[j]
            if (fallback.name !== ""
                    && fallback.width > 0
                    && fallback.height > 0) {
                activatePopupScreen(fallback)
                return true
            }
        }

        return false
    }

    function activatePopupScreenByName(screenName) {
        if (screenName) {
            for (var i = 0; i < Quickshell.screens.length; i++) {
                var candidate = Quickshell.screens[i]
                if (candidate.name === screenName
                        && candidate.width > 0
                        && candidate.height > 0) {
                    activatePopupScreen(candidate)
                    return true
                }
            }
        }

        return activateFocusedPopupScreen()
    }

    Connections {
        target: Wm

        function onFocusedOutputChanged() {
            if (!theme.keyboardPopupVisible || theme.activePopupScreenName === "") return

            var focusedName = Wm.focusedOutput
            if (focusedName !== "" && focusedName !== theme.activePopupScreenName) {
                theme.closePopups()
            }
        }
    }

    function isActivePopupScreenName(screenName) {
        return activePopupScreenName !== "" && screenName === activePopupScreenName
    }

    function applyAnchor(name, x) {
        if (name === "tray") trayBarX = x
        else if (name === "trayCaret") trayCaretBarX = x
        else if (name === "notif") notifBarX = x
        else if (name === "notifCaret") notifCaretBarX = x
        else if (name === "quickActions") quickActionsBarX = x
        else if (name === "volume") volumeBarX = x
        else if (name === "network") networkBarX = x
        else if (name === "battery") batteryBarX = x
        else if (name === "memory") memoryBarX = x
        else if (name === "cpu") cpuBarX = x
        else if (name === "gpu") gpuBarX = x
        else if (name === "thermal") thermalBarX = x
        else if (name === "storage") storageBarX = x
        else if (name === "ai") aiBarX = x
        else if (name === "workspace") workspaceBarX = x
        else if (name === "bluetooth") bluetoothBarX = x
        else if (name === "brightness") brightnessBarX = x
        else if (name === "power") powerBarX = x
        else if (name === "mpris") mprisBarX = x
        else if (name === "dashboard") dashboardBarX = x
        else if (name === "launcher") launcherBarX = x
        else if (name === "trayMenu") trayMenuX = x
        else if (name.indexOf("plugin:") === 0) {
            var next = {}
            for (var k in pluginBarX) next[k] = pluginBarX[k]
            next[name.substring(7)] = x
            pluginBarX = next
        }
    }

    function applyActiveBarAnchors() {
        var anchors = activePopupScreenName ? barAnchorsByScreen[activePopupScreenName] : null
        if (!anchors) return

        for (var name in anchors) applyAnchor(name, anchors[name])
    }

    function publishBarAnchors(screenName, anchors) {
        if (!screenName || !anchors) return

        var next = {}
        for (var screen in barAnchorsByScreen) next[screen] = barAnchorsByScreen[screen]
        next[screenName] = anchors
        barAnchorsByScreen = next

        if (screenName === activePopupScreenName) applyActiveBarAnchors()
    }

    function setPanelAnchor(name, x, screenName) {
        var targetScreen = screenName || activePopupScreenName
        if (targetScreen) {
            var next = {}
            for (var screen in barAnchorsByScreen) next[screen] = barAnchorsByScreen[screen]

            var current = next[targetScreen] || {}
            var anchors = {}
            for (var key in current) anchors[key] = current[key]
            anchors[name] = x
            next[targetScreen] = anchors
            barAnchorsByScreen = next
        }

        if (!targetScreen || targetScreen === activePopupScreenName) applyAnchor(name, x)
    }

    function closePopups(except) {
        _closingPopups = true
        if (except !== "dashboardVisible") dashboardVisible = false
        if (except !== "cpuVisible") cpuVisible = false
        if (except !== "gpuVisible") gpuVisible = false
        if (except !== "thermalVisible") thermalVisible = false
        if (except !== "aiUsageVisible") aiUsageVisible = false
        if (except !== "memVisible") memVisible = false
        if (except !== "volVisible") volVisible = false
        if (except !== "controlVisible") controlVisible = false
        if (except !== "networkVisible") networkVisible = false
        if (except !== "bluetoothVisible") bluetoothVisible = false
        if (except !== "batteryVisible") batteryVisible = false
        if (except !== "brightnessVisible") brightnessVisible = false
        if (except !== "mprisVisible") mprisVisible = false
        if (except !== "workspaceVisible") workspaceVisible = false
        if (except !== "imagePickerVisible") imagePickerVisible = false
        if (except !== "mediaBrowserVisible") mediaBrowserVisible = false
        if (except !== "notifVisible") notifVisible = false
        if (except !== "powerProfileVisible") powerProfileVisible = false
        if (except !== "storageVisible") storageVisible = false
        if (except !== "trayVisible") trayVisible = false
        if (except !== "trayMenuVisible") trayMenuVisible = false
        if (except !== "pluginPanelVisible") pluginPanelId = ""
        hideTooltip()
        _closingPopups = false
    }

    function popupUsesConnectedInset(prop) {
        return prop !== "imagePickerVisible"
            && prop !== "mediaBrowserVisible"
            && prop !== "trayMenuVisible"
    }

    function popupOpened(prop) {
        if (!_closingPopups && theme[prop]) {
            // The new panel surface publishes its clamped caret position on the
            // first reveal tick. Invalidate the previous panel's cached position
            // now so the bar cannot render one frame at that stale anchor.
            if (popupUsesConnectedInset(prop)) panelInsetReady = false
            closePopups(prop)
        }
    }

    function openImagePicker(mode, screen) {
        if (imagePickerVisible && imagePickerMode !== mode)
            imagePickerVisible = false
        if (screen && screen.name !== "") activatePopupScreen(screen)
        else activateFocusedPopupScreen()
        mediaBrowserVisible = false
        imagePickerMode = mode
        imagePickerVisible = true
    }

    function toggleImagePicker(mode, screen) {
        if (imagePickerVisible && imagePickerMode === mode) {
            imagePickerVisible = false
            return
        }

        openImagePicker(mode, screen)
    }

    function openMediaBrowser(mode) {
        if (mediaBrowserVisible && mediaBrowserMode !== mode)
            mediaBrowserVisible = false
        activateFocusedPopupScreen()
        imagePickerVisible = false
        mediaBrowserMode = mode
        mediaBrowserVisible = true
    }

    // ── pill/card border (default, non-borderless mode) ──
    // A premium "inactive window border" look: the surface tone (paper) nudged a
    // tick toward the foreground (ink) → a quiet edge a touch brighter than the
    // background, theme-aware in BOTH dark and light palettes. Tune via pillBorderMix.
    property real pillBorderMix: 0.13
    readonly property color pillBorder: Qt.rgba(
        paper.r * (1 - pillBorderMix) + ink.r * pillBorderMix,
        paper.g * (1 - pillBorderMix) + ink.g * pillBorderMix,
        paper.b * (1 - pillBorderMix) + ink.b * pillBorderMix, 1.0)
    // outer frame (the island edge against the wallpaper): a tick brighter than
    // the inner pill border so the bar lifts off the background → two readable
    // borders (subtle inner pills + a defined outer frame).
    property real islandBorderMix: 0.16
    readonly property color islandBorder: Qt.rgba(
        paper.r * (1 - islandBorderMix) + ink.r * islandBorderMix,
        paper.g * (1 - islandBorderMix) + ink.g * islandBorderMix,
        paper.b * (1 - islandBorderMix) + ink.b * islandBorderMix, 1.0)

    // ── V2 continuous edge-bar tokens ──
    // The compact visible strip uses one full-width surface, one quiet
    // screen-facing edge and a shadow cast away from that edge. Keeping these
    // separate from the pill recipe lets widgets and panels retain their
    // established hierarchy.
    readonly property int v2BarHeight: Math.round(33 * barScale)
    readonly property int v2NotchFrameThickness: 6
    readonly property int v2NotchFrameRadius: 14
    // Horizontal rhythm for the bar. Closely related icon buttons use the
    // 2px cluster step; icon+text pairs use 4px; independent widgets use 6px;
    // distinct information sections use 8px. A compact action cell stays 22px
    // wide, yielding a calm 24px centre-to-centre pitch inside icon clusters.
    readonly property int v2IconClusterSpacing: 2
    readonly property int v2InlineSpacing: 4
    readonly property int v2WidgetSpacing: 6
    readonly property int v2SectionSpacing: 8
    readonly property int v2ActionIconCellWidth: 22
    readonly property int v2IconGroupPadding: 5
    property real v2BarBorderMix: 0.22
    readonly property color v2BarBorder: Qt.rgba(
        paper.r * (1 - v2BarBorderMix) + ink.r * v2BarBorderMix,
        paper.g * (1 - v2BarBorderMix) + ink.g * v2BarBorderMix,
        paper.b * (1 - v2BarBorderMix) + ink.b * v2BarBorderMix, 1.0)
    readonly property color v2BarShadow: Qt.rgba(0, 0, 0, 0.46)
    // Depth (barShadowEnabled) lifts the bar shell itself, not only its popouts:
    // the resting shadow is a soft seat, the lifted one a real drop. BarSlot's two
    // shell shadows read these, so the toggle is visible on the bar it names.
    readonly property color barShellShadow: barShadowEnabled ? Qt.rgba(0, 0, 0, 0.66) : v2BarShadow
    readonly property int   barShellShadowBlur: barShadowEnabled ? 20 : 9
    readonly property int   barShellShadowOffset: barShadowEnabled ? 5 : 2
    // Popovers, tooltips and their interactive tiles share the calmer V2 shape;
    // bar-widget pills remain independently configurable below.
    readonly property int panelRadius: 6
    readonly property int panelButtonRadius: 6
    readonly property color panelBorder: v2BarBorder
    readonly property int panelBorderW: 1
    readonly property color panelOuterBorderColor: panelTooltipBorderEnabled
        ? panelBorder : Qt.rgba(0, 0, 0, 0)
    readonly property int panelOuterBorderW: panelTooltipBorderEnabled ? panelBorderW : 0

    // Fixed V2 geometry. The former Style section was removed; keeping these
    // shared constants avoids duplicating the established dimensions.
    readonly property int   pillRadius:   12
    readonly property int   pillH:        24
    readonly property int   pillBorderW:  1
    readonly property int   tileRadius:   10
    readonly property int   wsPillPad:    0
    readonly property color pillShadow:   Qt.rgba(0, 0, 0, 0.55)   // dark, theme-independent

    property string lastAppliedName: ""

    // ── Tooltip state ──
    property string tooltipText: ""
    property real tooltipX: 0
    property real tooltipY: 0
    property real tooltipTopY: 0
    property real tooltipBottomY: 0
    property bool tooltipShown: false
    property var tooltipOwner: null   // the widget currently owning the tooltip

    function showTooltip(text, x, topY, bottomY, owner) {
        if (!text) return;
        tooltipText = text;
        tooltipX = x;
        tooltipTopY = topY;
        tooltipBottomY = bottomY;
        tooltipY = (topY + bottomY) / 2;
        tooltipOwner = owner !== undefined ? owner : null;
        tooltipShown = true;
    }

    // hide only if the caller owns the current tooltip (owner match is stable
    // even when the tooltip text changes, e.g. a live timer). A null/undefined
    // owner force-hides. Legacy string args fall back to a text match.
    function hideTooltip(owner) {
        if (owner === undefined || owner === null) {
            tooltipShown = false; tooltipOwner = null;
        } else if (typeof owner === "object") {
            if (tooltipOwner === owner) { tooltipShown = false; tooltipOwner = null; }
        } else if (tooltipText === owner) {
            tooltipShown = false; tooltipOwner = null;
        }
    }

    // safety net: if the owning widget disappears while its tooltip is shown
    // (e.g. ScreenRecord stops mid-hover, or a slot widget gets disabled), force-hide.
    // Via Connections - NOT a `_visible` property whose change-handler writes
    // tooltipOwner (that property read tooltipOwner → binding loop).
    Connections {
        target: theme.tooltipOwner
        ignoreUnknownSignals: true
        function onVisibleChanged() {
            if (theme.tooltipOwner && !theme.tooltipOwner.visible) {
                theme.tooltipShown = false; theme.tooltipOwner = null;
            }
        }
    }

    // ── Calendar state ──
    property bool dashboardVisible: false
    onDashboardVisibleChanged: popupOpened("dashboardVisible")
    property int calendarMonthOffset: 0
    property int calendarTick: 0
    property int selectedDay: 0

    readonly property var calendarCells: {
        calendarTick;
        const now = new Date();
        const first = new Date(now.getFullYear(), now.getMonth() + calendarMonthOffset, 1);
        const year = first.getFullYear();
        const month = first.getMonth();
        const lastDay = new Date(year, month + 1, 0).getDate();
        const startDay = (first.getDay() + 6) % 7;
        const today = new Date();
        const isCurrentMonth = year === today.getFullYear() && month === today.getMonth();
        const cells = [];
        for (let i = 0; i < startDay; i++) cells.push({day: 0, today: false});
        for (let d = 1; d <= lastDay; d++) {
            cells.push({day: d, today: isCurrentMonth && d === today.getDate()});
        }
        while (cells.length < 42) cells.push({day: 0, today: false});
        return cells;
    }

    readonly property string calendarMonthName: {
        const months = ["JANUARY","FEBRUARY","MARCH","APRIL","MAY","JUNE",
                        "JULY","AUGUST","SEPTEMBER","OCTOBER","NOVEMBER","DECEMBER"];
        const now = new Date();
        return months[(now.getMonth() + calendarMonthOffset + 12000) % 12];
    }

    readonly property string calendarYear: {
        const now = new Date();
        const d = new Date(now.getFullYear(), now.getMonth() + calendarMonthOffset, 1);
        return String(d.getFullYear());
    }

    function openDashboard() {
        calendarMonthOffset = 0;
        calendarTick++;
        selectedDay = (new Date()).getDate();
        dashboardVisible = true;
    }

    // ── CPU panel state ──
    property bool cpuVisible: false
    onCpuVisibleChanged: popupOpened("cpuVisible")

    // ── GPU panel state ──
    property bool gpuVisible: false
    onGpuVisibleChanged: popupOpened("gpuVisible")

    // ── Thermal panel state ──
    property bool thermalVisible: false
    onThermalVisibleChanged: popupOpened("thermalVisible")

    // ── AI usage panel state + which tool the bar pill shows ──
    property bool   aiUsageVisible: false
    property real   aiUsageReveal: aiUsageVisible ? 1 : 0
    Behavior on aiUsageReveal {
        NumberAnimation {
            duration: theme.aiUsageVisible ? 160 : 120
            easing.type: theme.aiUsageVisible ? Easing.OutCubic : Easing.InCubic
        }
    }
    onAiUsageVisibleChanged: {
        popupOpened("aiUsageVisible")
        if (aiUsageVisible) refreshAiUsage()
    }
    // Which agents the AI pill shows on the bar. A set, not a pick: the pill draws
    // one chip per provider, so several can ride the bar at once. A chosen provider
    // still needs data (or a live session) to earn its chip, so this narrows what
    // may appear rather than forcing an empty chip into the row. Persisted through
    // the shell.json bridge only - the legacy widgets cache is a positional record
    // and this set has no field there.
    readonly property var aiToolOptions: ["claude", "codex", "opencode"]
    property var aiTools: ["claude", "codex", "opencode"]
    // Lifts the safe 100% cap to 150% for quiet hardware. It has to live in
    // shell.json rather than the in-memory adapter, because the media-key binding
    // reads `.qsbar.audioBoost` from that file to decide wpctl's limit.
    property bool audioBoost: false
    function aiToolShown(id) { return (aiTools || []).indexOf(id) >= 0 }
    // A provider with no collected usage can never draw a chip, so Settings shows
    // it dimmed rather than as a switch that appears to do nothing. OpenCode, for
    // instance, only reports once its local session database exists.
    function aiToolHasData(id) {
        if (id === "claude")   return aiClHas
        if (id === "codex")    return aiCxHas
        if (id === "opencode") return aiOcHas
        return false
    }
    function aiToolsUnavailable() {
        return aiToolOptions.filter(function (o) { return !aiToolHasData(o) })
    }
    function toggleAiTool(id) {
        if (aiToolOptions.indexOf(id) < 0) return
        var next = (aiTools || []).slice()
        var at = next.indexOf(id)
        // The last chosen provider stays chosen: an empty set would hide the pill
        // while its widget still reads as On, and the widget's own toggle already
        // means off. Like any segmented control, one option is always chosen.
        if (at >= 0 && next.length <= 1) return
        if (at >= 0) next.splice(at, 1)
        else next.push(id)
        // keep the row in the order the pill draws, never click order
        aiTools = aiToolOptions.filter(function (o) { return next.indexOf(o) >= 0 })
    }
    function serializeAiTools() { return (aiTools || []).join(",") }
    function parseAiTools(csv) {
        var raw = String(csv || "").split(",")
        var next = aiToolOptions.filter(function (o) { return raw.indexOf(o) >= 0 })
        if (next.length === 0) next = aiToolOptions.slice()
        // A `var` handed a fresh array always reports a change, and that change
        // saves, and the save re-applies the config back through here. Assign only
        // on a real difference, or the two chase each other until the shell dies.
        if (next.join(",") !== (aiTools || []).join(",")) aiTools = next
    }

    // ── AI usage data (single source of truth) ───────────────────
    // The bar pill (ClaudeWidget) and the AiUsagePanel both render from these -
    // the cache parsing lives ONLY here so the two views can never drift apart.
    // Token strings are bare "X.XXM / Y.YM"; the pill tooltip appends " tokens".
    property bool   aiClHas: false
    property bool   aiClFresh: false
    property int    aiClPct5h: 0
    property int    aiClPct7d: 0
    property bool   aiClBlocked: false
    property string aiClTokens: ""
    property string aiClRate: ""
    property int    aiClReset5hTs: 0
    property int    aiClReset7dTs: 0
    property int    aiClToday: 0
    property var    aiClRecent: []

    property bool   aiCxHas: false
    property bool   aiCxFresh: false
    property int    aiCxPct5h: 0
    property int    aiCxPct7d: 0
    property string aiCxPlan: ""
    property string aiCxTokens: ""
    property string aiCxRate: ""
    property int    aiCxReset5hTs: 0
    property int    aiCxReset7dTs: 0
    property int    aiCxToday: 0
    property var    aiCxRecent: []
    property var    aiCxBuckets: []
    property var    aiCxWindows: []
    property int    aiCxPrimaryPct: 0
    property string aiCxPrimaryLabel: ""
    property int    aiCxPrimaryResetTs: 0
    property int    aiCxQuotaPct: 0
    property string aiCxQuotaLabel: ""
    property string aiCxLimitStatus: ""
    property string aiCxLimitReachedType: ""

    property bool   aiOcHas: false
    property bool   aiOcFresh: false
    property int    aiOcPct5h: 0
    property int    aiOcPct7d: 0
    property string aiOcPlan: ""
    property string aiOcTokens: ""
    property string aiOcRate: ""
    property string aiOcModel: ""
    property int    aiOcToday: 0
    property var    aiOcRecent: []
    property var    aiOcModels: []
    property int    aiClockTick: 0
    // Drives the AI panel's refresh spinner while the collectors regenerate the
    // on-disk caches (see regenerateAiUsage).
    property bool   aiRefreshing: false

    // F15: clamp an external 0..1 utilization to a 0-100 int (a negative/over-range value would
    // otherwise produce wrong text and negative/overwide usage bars)
    function aiPct(v) { return Math.max(0, Math.min(100, Math.round((parseFloat(v) || 0) * 100))) }

    function aiWindowLabel(minutes) {
        if (minutes === 300) return "5h"
        if (minutes === 10080) return I18n.tr("Weekly")
        if (minutes > 0 && minutes % 1440 === 0) return (minutes / 1440) + "d"
        if (minutes > 0 && minutes % 60 === 0) return (minutes / 60) + "h"
        return minutes > 0 ? minutes + "m" : I18n.tr("window")
    }

    function aiResetCodexUsage() {
        aiCxHas = false; aiCxFresh = false
        aiCxPct5h = 0; aiCxPct7d = 0
        aiCxPlan = ""; aiCxTokens = ""; aiCxRate = ""; aiCxToday = 0; aiCxRecent = []
        aiCxReset5hTs = 0; aiCxReset7dTs = 0
        aiCxBuckets = []; aiCxWindows = []
        aiCxPrimaryPct = 0; aiCxPrimaryLabel = ""; aiCxPrimaryResetTs = 0
        aiCxQuotaPct = 0; aiCxQuotaLabel = ""
        aiCxLimitStatus = ""; aiCxLimitReachedType = ""
    }

    function aiCodexWindowFromCache(w) {
        w = w || {}
        var minutes = parseInt(w.minutes) || 0
        return {
            kind: String(w.kind || ""),
            minutes: minutes,
            label: String(w.label || theme.aiWindowLabel(minutes)),
            pct: theme.aiPct(w.utilization),
            resetTs: parseInt(w.reset) || 0
        }
    }

    function aiCodexWindowsFromArray(arr) {
        var out = []
        if (!arr || arr.length === undefined) return out
        for (var i = 0; i < arr.length; i++) {
            out.push(theme.aiCodexWindowFromCache(arr[i]))
        }
        return out
    }

    function aiCodexLegacyWindowsFromCache(d) {
        var out = []
        if (!d) return out
        var has5 = d["5h-utilization"] !== undefined && String(d["5h-utilization"]) !== ""
        var has7 = d["7d-utilization"] !== undefined && String(d["7d-utilization"]) !== ""
        if (has5 || parseInt(d["5h-reset"]) > 0)
            out.push({ kind: "primary", minutes: 300, label: "5h", pct: theme.aiPct(d["5h-utilization"]), resetTs: parseInt(d["5h-reset"]) || 0 })
        if (has7 || parseInt(d["7d-reset"]) > 0)
            out.push({ kind: "secondary", minutes: 10080, label: I18n.tr("Weekly"), pct: theme.aiPct(d["7d-utilization"]), resetTs: parseInt(d["7d-reset"]) || 0 })
        return out
    }

    function aiCodexBucketsFromCache(d) {
        var buckets = []
        if (d && parseInt(d.schemaVersion) === 3 && d.buckets && d.buckets.length !== undefined) {
            for (var i = 0; i < d.buckets.length; i++) {
                var b = d.buckets[i] || {}
                var windows = theme.aiCodexWindowsFromArray(b.windows)
                if (windows.length === 0) continue
                buckets.push({
                    id: String(b.id || ""),
                    label: String(b.label || b.id || "Codex"),
                    isGeneral: b.isGeneral === true,
                    windows: windows,
                    plan: String(b.plan || ""),
                    rateLimitReachedType: String(b.rateLimitReachedType || "")
                })
            }
        } else if (d && parseInt(d.schemaVersion) === 2 && d.windows && d.windows.length !== undefined) {
            var win2 = theme.aiCodexWindowsFromArray(d.windows)
            if (win2.length > 0)
                buckets.push({ id: "codex", label: "Codex", isGeneral: true, windows: win2, plan: String(d._plan || "") })
        } else {
            var legacy = theme.aiCodexLegacyWindowsFromCache(d)
            if (legacy.length > 0)
                buckets.push({ id: "codex", label: "Codex", isGeneral: true, windows: legacy, plan: String((d && d._plan) || "") })
        }
        return buckets
    }

    function aiCodexGeneralBucket(buckets) {
        for (var i = 0; i < buckets.length; i++) {
            if (buckets[i].isGeneral === true || buckets[i].id === "codex") return buckets[i]
        }
        return { windows: [] }
    }

    function aiPlanLabel(plan) {
        var p = String(plan || "").toLowerCase()
        if (p === "prolite") return I18n.tr("Pro Lite")
        if (p === "pro") return I18n.tr("Pro")
        if (p === "plus") return I18n.tr("Plus")
        if (p === "team" || p === "business") return I18n.tr("Business")
        if (p === "enterprise") return I18n.tr("Enterprise")
        if (p === "edu") return I18n.tr("Edu")
        if (p === "free") return I18n.tr("Free")
        return String(plan || "")
    }

    function aiApplyCodexCache(d, ageOk) {
        var buckets = theme.aiCodexBucketsFromCache(d)
        var general = theme.aiCodexGeneralBucket(buckets)
        var windows = general.windows || []
        theme.aiCxHas = windows.length > 0
        theme.aiCxFresh = ageOk && d._source !== "stale"
        theme.aiCxBuckets = buckets
        theme.aiCxWindows = windows
        theme.aiCxPlan = theme.aiPlanLabel(d._plan || general.plan || "")
        theme.aiCxPct5h = 0; theme.aiCxPct7d = 0
        theme.aiCxReset5hTs = 0; theme.aiCxReset7dTs = 0
        theme.aiCxQuotaPct = 0
        theme.aiCxQuotaLabel = ""
        theme.aiCxLimitStatus = String(d.status || "")
        theme.aiCxLimitReachedType = String(d._limit_reached_type || general.rateLimitReachedType || "")
        for (var i = 0; i < windows.length; i++) {
            var w = windows[i]
            if (i === 0) {
                theme.aiCxPrimaryPct = w.pct
                theme.aiCxPrimaryLabel = w.label
                theme.aiCxPrimaryResetTs = w.resetTs
            }
            if (w.minutes === 300) { theme.aiCxPct5h = w.pct; theme.aiCxReset5hTs = w.resetTs }
            else if (w.minutes === 10080) { theme.aiCxPct7d = w.pct; theme.aiCxReset7dTs = w.resetTs }
            if (w.pct > theme.aiCxQuotaPct) {
                theme.aiCxQuotaPct = w.pct
                theme.aiCxQuotaLabel = String(general.label || "Codex") + " " + String(w.label || I18n.tr("window"))
            }
        }
        if (windows.length === 0) {
            theme.aiCxPrimaryPct = 0
            theme.aiCxPrimaryLabel = ""
            theme.aiCxPrimaryResetTs = 0
        }
        var cxRateH = Math.round((d["_rate_per_hour"] || 0) / 1000)
        theme.aiCxRate = cxRateH > 0 ? cxRateH + "k tok/h" : ""
        theme.aiCxToday = parseInt(d._today_tokens) || 0
        theme.aiCxRecent = d._recent_days instanceof Array ? d._recent_days : []
        theme.aiCxTokens = ""
    }

    function aiCodexStatusLabel(status, reachedType) {
        if (status === "rejected")
            return reachedType ? I18n.tr("reached (%1)").arg(reachedType) : I18n.tr("reached")
        if (status === "allowed_warning") return I18n.tr("warning")
        if (status === "allowed") return I18n.tr("ok (not reached)")
        return I18n.tr("unknown")
    }

    function aiFmtReset(ts) {
        aiClockTick
        var now = Date.now() / 1000
        if (!(ts > now)) return ""
        var mins = Math.round((ts - now) / 60)
        if (mins < 60) return mins + "m"
        var h = Math.floor(mins / 60), m = mins % 60
        if (h < 24) return h + "h " + m + "m"
        var d = Math.floor(h / 24); return d + "d " + (h % 24) + "h"
    }

    function aiPad2(n) { return n < 10 ? "0" + n : "" + n }

    function aiFmtResetClock(ts) {
        aiClockTick
        if (!(ts > Date.now() / 1000)) return ""
        var d = new Date(ts * 1000)
        var now = new Date()
        var day0 = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
        var resetDay0 = new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
        var dayDelta = Math.round((resetDay0 - day0) / 86400000)
        var time = aiPad2(d.getHours()) + ":" + aiPad2(d.getMinutes())
        if (dayDelta <= 0) return time
        var days = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"]
        return days[d.getDay()] + " " + time
    }

    function aiFmtResetDetail(ts) {
        var rel = aiFmtReset(ts)
        var clock = aiFmtResetClock(ts)
        return rel && clock ? rel + " • " + clock : (rel || clock)
    }

    // ── prorated pace on the 7-day window (ported from omarchy-agent-usage) ──
    //   weighs the allowance still left against the fraction of the seven-day
    //   window still ahead in time. burning faster than the clock ⇒ behind pace
    //   (the provider turns red). pct7d is 0..100; resetTs is the weekly reset
    //   in UNIX seconds (0 ⇒ no weekly window, so pace is undefined).
    readonly property real aiWeekMs: 7 * 24 * 60 * 60 * 1000
    function aiExpectedRemaining(resetTs) {
        aiClockTick
        if (!(resetTs > 0)) return -1
        return Math.max(0, Math.min(1, (resetTs * 1000 - Date.now()) / aiWeekMs))
    }
    function aiExpectedPct(resetTs) {
        var e = aiExpectedRemaining(resetTs)
        return e < 0 ? 0 : Math.round(e * 100)
    }
    function aiPaceDiff(pct7d, resetTs) {
        var e = aiExpectedRemaining(resetTs)
        if (e < 0) return 0
        return (1 - Math.max(0, Math.min(100, pct7d)) / 100) - e
    }
    function aiBehindPace(pct7d, resetTs) {
        var e = aiExpectedRemaining(resetTs)
        if (e < 0) return false
        return (1 - Math.max(0, Math.min(100, pct7d)) / 100) + 0.0005 < e
    }
    function aiPaceText(pct7d, resetTs) {
        if (!(resetTs > 0)) return ""
        var pts = Math.round(Math.abs(aiPaceDiff(pct7d, resetTs)) * 100)
        if (pts === 0) return I18n.tr("on pace")
        return aiBehindPace(pct7d, resetTs) ? I18n.tr("%1% behind pace").arg(pts) : I18n.tr("%1% ahead of pace").arg(pts)
    }

    // ── LAST 7 DAYS token chart helpers (ported from omarchy-agent-usage) ──
    //   _recent_days is [{date:"YYYY-MM-DD", tokens:int}] oldest→newest.
    function aiDayTokens(day) { return Math.max(0, (day && day.tokens) || 0) }
    function aiRecentTotal(days) {
        var list = days instanceof Array ? days : [], t = 0
        for (var i = 0; i < list.length; i++) t += aiDayTokens(list[i])
        return t
    }
    function aiRecentPeak(days) {
        var list = days instanceof Array ? days : [], p = 0
        for (var i = 0; i < list.length; i++) p = Math.max(p, aiDayTokens(list[i]))
        return p
    }
    function aiTokenCount(v) {
        var a = Math.max(0, v || 0)
        if (a >= 1e9) return (a / 1e9).toFixed(a >= 1e10 ? 0 : 1).replace(/\.0$/, "") + "B"
        if (a >= 1e6) return (a / 1e6).toFixed(a >= 1e8 ? 0 : 1).replace(/\.0$/, "") + "M"
        if (a >= 1e3) return (a / 1e3).toFixed(a >= 1e5 ? 0 : 1).replace(/\.0$/, "") + "K"
        return String(Math.round(a))
    }
    function aiDayLabel(dateStr) {
        var m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(String(dateStr || ""))
        if (!m) return "—"
        var d = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]))
        return ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"][d.getDay()]
    }

    Process {
        id: aiReadClaude
        command: ["bash", "-c",
            "f=\"$HOME/.cache/claude-usage.json\"; stat -c %Y \"$f\" 2>/dev/null; cat \"$f\" 2>/dev/null"]
        stdout: StdioCollector {
            onStreamFinished: {
                var raw = this.text, nl = raw.indexOf("\n")
                var mtime = nl > 0 ? (parseInt(raw.substring(0, nl)) || 0) : 0
                var ageOk = mtime > 0 && (Date.now() / 1000 - mtime) < 900
                try {
                    var d = JSON.parse((nl > 0 ? raw.substring(nl + 1) : "").trim())
                    theme.aiClHas = true
                    theme.aiClFresh = ageOk && d._source !== "stale"
                    theme.aiClPct5h = theme.aiPct(d["5h-utilization"])
                    theme.aiClPct7d = theme.aiPct(d["7d-utilization"])
                    theme.aiClBlocked = d.status === "rejected" || d.status === "blocked"
                    theme.aiClReset5hTs = parseInt(d["5h-reset"]) || 0
                    theme.aiClReset7dTs = parseInt(d["7d-reset"]) || 0
                    var used = (d["_tokens_used"] || 0), lim = (d["_window_limit"] || 0)
                    theme.aiClTokens = used ? (used / 1e6).toFixed(2) + "M / " + (lim / 1e6).toFixed(1) + "M" : ""
                    var rateH = Math.round((d["_rate_per_hour"] || 0) / 1000)
                    theme.aiClRate = rateH > 0 ? rateH + "k tok/h" : ""
                    theme.aiClToday = parseInt(d._today_tokens) || 0
                    theme.aiClRecent = d._recent_days instanceof Array ? d._recent_days : []
                } catch (e) {
                    theme.aiClHas = false; theme.aiClFresh = false
                    theme.aiClPct5h = 0; theme.aiClPct7d = 0
                    theme.aiClBlocked = false; theme.aiClTokens = ""; theme.aiClRate = ""
                    theme.aiClReset5hTs = 0; theme.aiClReset7dTs = 0; theme.aiClToday = 0; theme.aiClRecent = []
                }
            }
        }
    }

    Process {
        id: aiReadCodex
        command: ["bash", "-c",
            "f=\"$HOME/.cache/codex-usage.json\"; stat -c %Y \"$f\" 2>/dev/null; cat \"$f\" 2>/dev/null"]
        stdout: StdioCollector {
            onStreamFinished: {
                var raw = this.text, nl = raw.indexOf("\n")
                var mtime = nl > 0 ? (parseInt(raw.substring(0, nl)) || 0) : 0
                var ageOk = mtime > 0 && (Date.now() / 1000 - mtime) < 900
                try {
                    var d = JSON.parse((nl > 0 ? raw.substring(nl + 1) : "").trim())
                    theme.aiApplyCodexCache(d, ageOk)
                } catch (e) {
                    theme.aiResetCodexUsage()
                }
            }
        }
    }

    Process {
        id: aiReadOpenCode
        command: ["bash", "-c",
            "f=\"$HOME/.cache/opencode-usage.json\"; stat -c %Y \"$f\" 2>/dev/null; cat \"$f\" 2>/dev/null"]
        stdout: StdioCollector {
            onStreamFinished: {
                var raw = this.text, nl = raw.indexOf("\n")
                var mtime = nl > 0 ? (parseInt(raw.substring(0, nl)) || 0) : 0
                var ageOk = mtime > 0 && (Date.now() / 1000 - mtime) < 900
                try {
                    var d = JSON.parse((nl > 0 ? raw.substring(nl + 1) : "").trim())
                    theme.aiOcHas = true
                    theme.aiOcFresh = ageOk && d._source !== "stale"
                    theme.aiOcPct5h = theme.aiPct(d["5h-utilization"])
                    theme.aiOcPct7d = theme.aiPct(d["7d-utilization"])
                    theme.aiOcPlan = d._plan || ""
                    var ocUsed = (d["_tokens_used"] || 0), ocLim = (d["_window_limit"] || 0)
                    theme.aiOcTokens = ocUsed ? (ocUsed / 1e6).toFixed(2) + "M / " + (ocLim / 1e6).toFixed(1) + "M" : ""
                    var ocRateH = Math.round((d["_rate_per_hour"] || 0) / 1000)
                    theme.aiOcRate = ocRateH > 0 ? ocRateH + "k tok/h" : ""
                    theme.aiOcToday = parseInt(d._today_tokens) || 0
                    theme.aiOcRecent = d._recent_days instanceof Array ? d._recent_days : []
                    theme.aiOcModel = d._model || ""
                    theme.aiOcModels = d._models instanceof Array ? d._models : []
                } catch (e) {
                    theme.aiOcHas = false; theme.aiOcFresh = false
                    theme.aiOcPct5h = 0; theme.aiOcPct7d = 0
                    theme.aiOcPlan = ""; theme.aiOcTokens = ""; theme.aiOcRate = ""; theme.aiOcModel = ""
                    theme.aiOcToday = 0; theme.aiOcModels = []; theme.aiOcRecent = []
                }
            }
        }
    }

    function refreshAiUsage(selectedOnly) {
        aiClockTick++
        var only = selectedOnly === true
        if (!only || aiToolShown("claude")) {
            aiReadClaude.running = false; aiReadClaude.running = true
        }
        if (!only || aiToolShown("codex")) {
            aiReadCodex.running = false;  aiReadCodex.running = true
        }
        if (!only || aiToolShown("opencode")) {
            aiReadOpenCode.running = false; aiReadOpenCode.running = true
        }
    }

    // Manual refresh: actually regenerate the on-disk caches by running the
    // collector service (claude/codex/opencode usage), then re-read them. The
    // periodic Timer below only re-reads; this is the only path that refetches.
    Process {
        id: aiCollectProc
        command: ["systemctl", "--user", "start", "ryoku-ai-usage.service"]
        onExited: {
            aiRefreshTimeout.stop()
            theme.refreshAiUsage(false)
            theme.aiRefreshing = false
        }
    }
    Timer {
        // Safety net so the spinner can never spin forever if the collector
        // stalls (claude-usage does a network fetch with its own timeout).
        id: aiRefreshTimeout
        interval: 30000
        onTriggered: theme.aiRefreshing = false
    }
    function regenerateAiUsage() {
        if (theme.aiRefreshing) return
        theme.aiRefreshing = true
        theme.aiClockTick++
        aiRefreshTimeout.restart()
        aiCollectProc.running = false
        aiCollectProc.running = true
    }

    Timer {
        // 30s normally; 5s while the AI panel is open (responsive when looked at)
        interval: (theme.aiUsageVisible ? 5000 : 30000) * Perf.pollFactor
        running: true; repeat: true; triggeredOnStart: true
        onTriggered: theme.refreshAiUsage(false)
    }

    // ── Central system telemetry ──
    // One ShellRoot-level sampler feeds all monitor BarSlots. This replaces
    // per-widget bash/awk polling for CPU and memory, so adding monitors does not
    // multiply these status process chains.
    property int systemCpuPercent: 0
    property int systemCpuUserPercent: 0
    property int systemCpuSystemPercent: 0
    property int systemCpuIoWaitPercent: 0
    property real _systemCpuPrevIdle: -1
    property real _systemCpuPrevTotal: -1
    property real _systemCpuPrevUser: -1
    property real _systemCpuPrevSystem: -1
    property real _systemCpuPrevIoWait: -1
    property string cpuModelName: ""
    property int cpuCoreCount: 0
    property int cpuThreadCount: 0
    property int cpuClockMHz: 0
    property int cpuMaxClockMHz: 0
    property string cpuEnergyPreference: ""
    property string cpuScalingGovernor: ""
    property int cpuThrottleCount: 0
    property real systemLoad1: 0
    property real systemLoad5: 0
    property real systemLoad15: 0
    property string kernelRelease: ""
    property var cpuTopProcesses: []

    property int systemMemTotalMiB: 0
    property int systemMemAvailMiB: 0
    property int systemMemFreeMiB: 0
    property int systemMemBuffersMiB: 0
    property int systemMemCachedMiB: 0
    property string memoryType: ""
    property int memorySpeedMTs: 0
    readonly property int systemMemUsedMiB: Math.max(0, systemMemTotalMiB - systemMemAvailMiB)
    readonly property int systemMemPercent: systemMemTotalMiB > 0
        ? Math.max(0, Math.min(100, Math.round(systemMemUsedMiB / systemMemTotalMiB * 100)))
        : 0
    readonly property real systemMemUsedGiB: systemMemUsedMiB / 1024
    readonly property real systemMemTotalGiB: systemMemTotalMiB / 1024

    // Compact V2 telemetry. These samplers live on the singleton Theme so a
    // second monitor adds another view, not another nvidia-smi/df/hwmon poller.
    property string gpuBackend: ""
    property string gpuName: ""
    property string gpuDriverVersion: ""
    property int gpuPercent: 0
    property int gpuTemperatureC: 0
    property int gpuMemoryUsedMiB: 0
    property int gpuMemoryTotalMiB: 0
    property int gpuClockMHz: 0
    property real gpuPowerW: 0
    property real gpuPowerLimitW: 0
    property string gpuPerformanceState: ""
    property int gpuFanPercent: 0
    readonly property bool gpuAvailable: gpuBackend !== ""

    property int cpuTemperatureC: 0
    property int cpuCoreMaxTemperatureC: 0
    property int cpuTemperatureMaxC: 0
    property int cpuTemperatureCriticalC: 0
    property int nvmeTemperatureC: 0
    property int nvmeTemperatureMaxC: 0
    property int nvmeTemperatureCriticalC: 0
    property int memoryTemperatureC: 0
    readonly property bool cpuTemperatureAvailable: cpuTemperatureC > 0

    property string barTemperatureSource: "cpu"
    readonly property int barTemperatureC: barTemperatureSource === "core" ? cpuCoreMaxTemperatureC
        : barTemperatureSource === "gpu" ? gpuTemperatureC
        : barTemperatureSource === "nvme" ? nvmeTemperatureC
        : barTemperatureSource === "memory" ? memoryTemperatureC
        : cpuTemperatureC
    readonly property bool barTemperatureAvailable: barTemperatureC > 0

    function barTemperatureSourceValid(source) {
        return source === "cpu" || source === "core" || source === "gpu"
            || source === "nvme" || source === "memory"
    }
    function barTemperatureSourceLabel(source) {
        if (source === "core") return I18n.tr("Hottest CPU core")
        if (source === "gpu") return "GPU"
        if (source === "nvme") return "NVMe"
        if (source === "memory") return I18n.tr("Memory")
        return I18n.tr("CPU package")
    }
    function barTemperatureSourceAvailable(source) {
        if (source === "core") return cpuCoreMaxTemperatureC > 0
        if (source === "gpu") return gpuTemperatureC > 0
        if (source === "nvme") return nvmeTemperatureC > 0
        if (source === "memory") return memoryTemperatureC > 0
        return cpuTemperatureC > 0
    }

    property int storagePercent: 0
    property real storageUsedBytes: 0
    property real storageTotalBytes: 0
    property bool storageAvailable: false
    readonly property real storageUsedGiB: storageUsedBytes / 1073741824
    readonly property real storageTotalGiB: storageTotalBytes / 1073741824
    property var storageDrives: []
    property bool storageInventoryAvailable: false

    function parseSystemCpu(text) {
        var lines = String(text || "").split("\n")
        if (lines.length === 0 || lines[0].indexOf("cpu ") !== 0) return
        var parts = lines[0].trim().split(/\s+/)
        if (parts.length < 8) return

        function field(index) {
            var value = parseFloat(parts[index])
            return isNaN(value) ? 0 : value
        }
        var user = field(1) + field(2)
        var system = field(3) + field(6) + field(7) + field(8)
        var ioWait = field(5)
        var idle = field(4) + ioWait
        var total = 0
        for (var i = 1; i < parts.length; i++) {
            var v = parseFloat(parts[i])
            if (!isNaN(v)) total += v
        }
        if (isNaN(idle) || isNaN(total) || total <= 0) return

        if (_systemCpuPrevTotal >= 0 && total > _systemCpuPrevTotal) {
            var totalDelta = total - _systemCpuPrevTotal
            var idleDelta = idle - _systemCpuPrevIdle
            var busy = totalDelta > 0 ? Math.round((totalDelta - idleDelta) / totalDelta * 100) : 0
            systemCpuPercent = Math.max(0, Math.min(100, busy))
            systemCpuUserPercent = Math.max(0, Math.min(100,
                Math.round((user - _systemCpuPrevUser) / totalDelta * 100)))
            systemCpuSystemPercent = Math.max(0, Math.min(100,
                Math.round((system - _systemCpuPrevSystem) / totalDelta * 100)))
            systemCpuIoWaitPercent = Math.max(0, Math.min(100,
                Math.round((ioWait - _systemCpuPrevIoWait) / totalDelta * 100)))
        }

        _systemCpuPrevIdle = idle
        _systemCpuPrevTotal = total
        _systemCpuPrevUser = user
        _systemCpuPrevSystem = system
        _systemCpuPrevIoWait = ioWait
    }

    function parseCpuInfo(text) {
        var blocks = String(text || "").trim().split(/\n\s*\n/)
        var model = ""
        var threads = 0
        var cores = {}
        var fallbackCores = 0

        for (var i = 0; i < blocks.length; i++) {
            var lines = blocks[i].split("\n")
            var physical = "0"
            var core = ""
            var processorFound = false
            for (var j = 0; j < lines.length; j++) {
                var splitAt = lines[j].indexOf(":")
                if (splitAt < 0) continue
                var key = lines[j].slice(0, splitAt).trim()
                var value = lines[j].slice(splitAt + 1).trim()
                if (key === "processor") processorFound = true
                else if ((key === "model name" || key === "Hardware") && model === "") model = value
                else if (key === "physical id") physical = value
                else if (key === "core id") core = value
                else if (key === "cpu cores" && fallbackCores === 0) fallbackCores = parseInt(value) || 0
            }
            if (processorFound) threads++
            if (core !== "") cores[physical + ":" + core] = true
        }

        cpuModelName = model.replace(/\(R\)|\(TM\)/g, "")
            .replace(/\s+CPU\s+@\s+.*$/, "").replace(/\s+/g, " ").trim()
        cpuThreadCount = threads
        var coreKeys = Object.keys(cores)
        cpuCoreCount = coreKeys.length > 0 ? coreKeys.length : fallbackCores
    }

    function parseSystemLoad(text) {
        var fields = String(text || "").trim().split(/\s+/)
        if (fields.length < 3) return
        systemLoad1 = parseFloat(fields[0]) || 0
        systemLoad5 = parseFloat(fields[1]) || 0
        systemLoad15 = parseFloat(fields[2]) || 0
    }

    function parseCpuDetail(text) {
        var fields = String(text || "").trim().split("|")
        if (fields.length < 5) return
        cpuClockMHz = Math.max(0, parseInt(fields[0]) || 0)
        cpuMaxClockMHz = Math.max(0, parseInt(fields[1]) || 0)
        cpuEnergyPreference = String(fields[2] || "").trim()
        cpuScalingGovernor = String(fields[3] || "").trim()
        cpuThrottleCount = Math.max(0, parseInt(fields[4]) || 0)
    }

    function parseCpuTopProcesses(text) {
        var lines = String(text || "").split("\n")
        var processes = []
        for (var i = 0; i < lines.length && processes.length < 3; i++) {
            var match = lines[i].trim().match(/^(.*\S)\s+([0-9]+(?:[.,][0-9]+)?)$/)
            if (!match || match[1] === "ps") continue
            var percent = parseFloat(match[2].replace(",", "."))
            if (isNaN(percent)) continue
            processes.push({ name: match[1], percent: percent })
        }
        cpuTopProcesses = processes
    }

    function parseSystemMem(text) {
        var total = 0, avail = 0, free = 0, buffers = 0, cached = 0
        var lines = String(text || "").split("\n")
        for (var i = 0; i < lines.length; i++) {
            var parts = lines[i].trim().split(/\s+/)
            if (parts.length < 2) continue
            var value = parseInt(parts[1])
            if (isNaN(value)) continue
            if (parts[0] === "MemTotal:") total = value
            else if (parts[0] === "MemAvailable:") avail = value
            else if (parts[0] === "MemFree:") free = value
            else if (parts[0] === "Buffers:") buffers = value
            else if (parts[0] === "Cached:") cached = value
        }
        if (total <= 0) return
        systemMemTotalMiB = Math.round(total / 1024)
        systemMemAvailMiB = Math.round(avail / 1024)
        systemMemFreeMiB = Math.round(free / 1024)
        systemMemBuffersMiB = Math.round(buffers / 1024)
        systemMemCachedMiB = Math.round(cached / 1024)
    }

    function parseMemoryHardware(text) {
        var raw = String(text || "")
        var matcher = /type:\s*(DDR[0-9]+)\b[^\n]*\bspeed:\s*([0-9]+)\s*MT\/s/gi
        var match
        var type = ""
        var speed = 0
        while ((match = matcher.exec(raw)) !== null) {
            var candidate = parseInt(match[2]) || 0
            if (type === "") type = match[1].toUpperCase()
            if (candidate > 0 && (speed === 0 || candidate < speed)) speed = candidate
        }
        memoryType = type
        memorySpeedMTs = speed
    }

    function parseGpuTelemetry(text) {
        var fields = String(text || "").trim().split("|")
        // A runtime-suspended NVIDIA dGPU is doing no work; the poll below skips
        // nvidia-smi so the card stays asleep and emits "suspended". Keep the
        // last-known identity/VRAM and zero only the live load, so the widget reads
        // 0% rather than vanishing or waking the card (a ~9-10W idle drain).
        if (fields[0] === "suspended") {
            gpuPercent = 0
            gpuTemperatureC = 0
            gpuClockMHz = 0
            gpuPowerW = 0
            gpuFanPercent = 0
            return
        }
        if (fields.length < 12 || fields[0] === "none") {
            gpuBackend = ""
            gpuName = ""
            gpuDriverVersion = ""
            gpuPercent = 0
            gpuTemperatureC = 0
            gpuMemoryUsedMiB = 0
            gpuMemoryTotalMiB = 0
            gpuClockMHz = 0
            gpuPowerW = 0
            gpuPowerLimitW = 0
            gpuPerformanceState = ""
            gpuFanPercent = 0
            return
        }

        function clean(value) { return String(value || "").trim() }
        function number(value) {
            var parsed = parseFloat(clean(value))
            return isNaN(parsed) ? 0 : parsed
        }

        gpuBackend = clean(fields[0])
        gpuName = clean(fields[1])
        gpuDriverVersion = clean(fields[2])
        gpuPercent = Math.max(0, Math.min(100, Math.round(number(fields[3]))))
        gpuTemperatureC = Math.max(0, Math.round(number(fields[4])))
        gpuMemoryUsedMiB = Math.max(0, Math.round(number(fields[5])))
        gpuMemoryTotalMiB = Math.max(0, Math.round(number(fields[6])))
        gpuClockMHz = Math.max(0, Math.round(number(fields[7])))
        gpuPowerW = Math.max(0, number(fields[8]))
        gpuPowerLimitW = Math.max(0, number(fields[9]))
        gpuPerformanceState = clean(fields[10])
        gpuFanPercent = Math.max(0, Math.min(100, Math.round(number(fields[11]))))
    }

    function parseThermalTelemetry(text) {
        var fields = String(text || "").trim().split("|")
        function thermal(index) {
            var value = parseInt(fields[index])
            return isNaN(value) ? 0 : Math.max(0, Math.min(150, value))
        }
        cpuTemperatureC = thermal(0)
        cpuCoreMaxTemperatureC = thermal(1)
        cpuTemperatureMaxC = thermal(2)
        cpuTemperatureCriticalC = thermal(3)
        nvmeTemperatureC = thermal(4)
        nvmeTemperatureMaxC = thermal(5)
        nvmeTemperatureCriticalC = thermal(6)
        memoryTemperatureC = thermal(7)
    }

    function parseStorageInventory(text) {
        var parsed
        try {
            parsed = JSON.parse(String(text || ""))
        } catch (error) {
            storageInventoryAvailable = false
            storageDrives = []
            return
        }

        var devices = parsed && parsed.blockdevices ? parsed.blockdevices : []
        var drives = []

        function textValue(value) {
            return value === null || value === undefined ? "" : String(value).trim()
        }
        function collectVolumes(node, target) {
            var fs = textValue(node.fstype)
            var mounts = node.mountpoints || []
            var mountedAt = ""
            for (var m = 0; m < mounts.length; m++) {
                var candidate = textValue(mounts[m])
                if (candidate !== "" && candidate !== "[SWAP]") {
                    mountedAt = candidate
                    break
                }
            }
            if (fs !== "") {
                var pct = parseInt(textValue(node["fsuse%"]).replace("%", ""))
                var freeText = textValue(node.fsavail)
                var freeBytes = freeText === "" ? -1 : Number(freeText)
                var usedText = textValue(node.fsused)
                var usedBytes = usedText === "" ? -1 : Number(usedText)
                target.push({
                    fs: fs,
                    mount: mountedAt,
                    percent: isNaN(pct) ? -1 : Math.max(0, Math.min(100, pct)),
                    freeBytes: isNaN(freeBytes) ? -1 : Math.max(0, freeBytes),
                    usedBytes: isNaN(usedBytes) ? -1 : Math.max(0, usedBytes)
                })
            }
            var children = node.children || []
            for (var c = 0; c < children.length; c++) collectVolumes(children[c], target)
        }

        for (var i = 0; i < devices.length; i++) {
            var device = devices[i]
            var name = textValue(device.name)
            // Kernel pseudo-devices report TYPE=disk like real hardware, so name
            // alone is not enough: nbd0-15 exist merely because the nbd module is
            // loaded. A drive with no capacity is not a drive, which also drops an
            // empty card slot or an unpopulated array, so size carries the rule.
            if (device.type !== "disk"
                    || (Number(device.size) || 0) <= 0
                    || name.indexOf("loop") === 0
                    || name.indexOf("ram") === 0
                    || name.indexOf("zram") === 0
                    || name.indexOf("nbd") === 0)
                continue

            var volumes = []
            collectVolumes(device, volumes)
            var fileSystems = []
            var mountedAt = ""
            var usage = -1
            var freeBytes = -1
            var usedBytes = -1
            for (var v = 0; v < volumes.length; v++) {
                if (fileSystems.indexOf(volumes[v].fs) < 0) fileSystems.push(volumes[v].fs)
                if (mountedAt === "" && volumes[v].mount !== "") {
                    mountedAt = volumes[v].mount
                    usage = volumes[v].percent
                    freeBytes = volumes[v].freeBytes
                    usedBytes = volumes[v].usedBytes
                }
            }

            var transport = textValue(device.tran).toUpperCase()
            var removable = device.rm === true || device.hotplug === true || transport === "USB"
            var driveType = transport === "NVME" ? "nvme"
                : device.rota === true ? "hdd"
                : "ssd"
            var media = removable ? "USB DRIVE"
                : transport === "NVME" ? "NVME SSD"
                : device.rota === true ? (transport !== "" ? transport + " HDD" : "HDD")
                : (transport !== "" ? transport + " SSD" : "SSD")
            var state = mountedAt !== "" ? mountedAt
                : (fileSystems.length > 0 ? I18n.tr("Not mounted") : I18n.tr("No filesystem"))

            drives.push({
                name: name,
                model: textValue(device.model) || name,
                size: Number(device.size) || 0,
                driveType: driveType,
                media: media,
                fileSystems: fileSystems.join(" + ").toUpperCase(),
                state: state,
                percent: usage,
                freeBytes: freeBytes,
                usedBytes: usedBytes,
                totalBytes: usedBytes >= 0 && freeBytes >= 0 ? usedBytes + freeBytes : -1
            })
        }

        storageDrives = drives
        storageInventoryAvailable = true
    }

    function parseStorageTelemetry(text) {
        var fields = String(text || "").trim().split("|")
        if (fields.length < 3) {
            storageAvailable = false
            storagePercent = 0
            storageUsedBytes = 0
            storageTotalBytes = 0
            return
        }

        var percent = parseInt(fields[0])
        var used = parseFloat(fields[1])
        var total = parseFloat(fields[2])
        if (isNaN(percent) || isNaN(used) || isNaN(total) || total <= 0) {
            storageAvailable = false
            return
        }

        storageAvailable = true
        storagePercent = Math.max(0, Math.min(100, percent))
        storageUsedBytes = Math.max(0, used)
        storageTotalBytes = Math.max(0, total)
    }

    FileView {
        id: systemCpuFile
        path: "/proc/stat"
        onLoaded: theme.parseSystemCpu(systemCpuFile.text())
    }

    FileView {
        id: systemCpuInfoFile
        path: "/proc/cpuinfo"
        onLoaded: theme.parseCpuInfo(systemCpuInfoFile.text())
    }

    FileView {
        id: systemLoadFile
        path: "/proc/loadavg"
        onLoaded: theme.parseSystemLoad(systemLoadFile.text())
    }

    FileView {
        id: kernelReleaseFile
        path: "/proc/sys/kernel/osrelease"
        onLoaded: theme.kernelRelease = String(kernelReleaseFile.text() || "").trim()
    }

    FileView {
        id: systemMemFile
        path: "/proc/meminfo"
        onLoaded: theme.parseSystemMem(systemMemFile.text())
    }

    Process {
        id: cpuDetailProc
        command: ["bash", "-c",
            "sum=0; count=0; max=0; epp=''; governor=''; throttle=0; "
            + "for f in /sys/devices/system/cpu/cpu[0-9]*/cpufreq/scaling_cur_freq; do [[ -r $f ]] || continue; IFS= read -r v < \"$f\"; "
            + "[[ $v =~ ^[0-9]+$ ]] || continue; sum=$((sum + v)); count=$((count + 1)); done; "
            + "(( count > 0 )) && avg=$((sum / count / 1000)) || avg=0; "
            + "f=/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq; [[ -r $f ]] && { IFS= read -r v < \"$f\"; [[ $v =~ ^[0-9]+$ ]] && max=$((v / 1000)); }; "
            + "f=/sys/devices/system/cpu/cpu0/cpufreq/energy_performance_preference; [[ -r $f ]] && IFS= read -r epp < \"$f\"; "
            + "f=/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor; [[ -r $f ]] && IFS= read -r governor < \"$f\"; "
            + "f=/sys/devices/system/cpu/cpu0/thermal_throttle/package_throttle_count; [[ -r $f ]] && { IFS= read -r v < \"$f\"; [[ $v =~ ^[0-9]+$ ]] && throttle=$v; }; "
            + "printf '%s|%s|%s|%s|%s\\n' \"$avg\" \"$max\" \"$epp\" \"$governor\" \"$throttle\""]
        stdout: StdioCollector { onStreamFinished: theme.parseCpuDetail(this.text) }
    }

    Process {
        id: memoryHardwareProc
        command: ["bash", "-c",
            "if command -v inxi >/dev/null 2>&1; then LC_ALL=C inxi -m -c 0 --no-host 2>/dev/null; fi"]
        running: true
        stdout: StdioCollector { onStreamFinished: theme.parseMemoryHardware(this.text) }
    }

    Process {
        id: cpuTopProcessesProc
        command: ["ps", "-eo", "comm=,%cpu=", "--sort=-%cpu"]
        stdout: StdioCollector { onStreamFinished: theme.parseCpuTopProcesses(this.text) }
    }

    Process {
        id: gpuTelemetryProc
        command: ["bash", "-c",
            "for st in /sys/bus/pci/drivers/nvidia/*/power/runtime_status; do [[ -r $st ]] || continue; IFS= read -r s < \"$st\"; [[ $s == suspended ]] && { printf 'suspended|||||||||||\\n'; exit 0; }; break; done; "
            + "if command -v nvidia-smi >/dev/null 2>&1; then "
            + "IFS=, read -r name driver util temp used total clock power limit pstate fan < <(nvidia-smi --query-gpu=name,driver_version,utilization.gpu,temperature.gpu,memory.used,memory.total,clocks.current.graphics,power.draw,power.limit,pstate,fan.speed --format=csv,noheader,nounits 2>/dev/null | head -n1); "
            + "if [[ $util =~ ^[[:space:]]*[0-9]+[[:space:]]*$ && $temp =~ ^[[:space:]]*[0-9]+[[:space:]]*$ && $used =~ ^[[:space:]]*[0-9]+[[:space:]]*$ && $total =~ ^[[:space:]]*[0-9]+[[:space:]]*$ ]]; then "
            + "printf 'nvidia|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s\\n' \"$name\" \"$driver\" \"$util\" \"$temp\" \"$used\" \"$total\" \"$clock\" \"$power\" \"$limit\" \"$pstate\" \"$fan\"; exit 0; fi; "
            + "fi; "
            + "for busy in /sys/class/drm/card*/device/gpu_busy_percent; do "
            + "[[ -r $busy ]] || continue; read -r util < \"$busy\"; temp=0; "
            + "for sensor in \"${busy%/gpu_busy_percent}\"/hwmon/hwmon*/temp1_input; do "
            + "[[ -r $sensor ]] || continue; read -r raw < \"$sensor\"; temp=$((raw / 1000)); break; done; "
            + "printf 'sysfs|GPU||%s|%s|0|0|0|0|0||0\\n' \"$util\" \"$temp\"; exit 0; done; "
            + "printf 'none|||||||||||\\n'"]
        stdout: StdioCollector { onStreamFinished: theme.parseGpuTelemetry(this.text) }
    }

    Process {
        id: cpuTemperatureProc
        command: ["bash", "-c",
            "cpu=0; core=0; cpu_max=0; cpu_crit=0; nvme=0; nvme_max=0; nvme_crit=0; dimm=0; "
            + "for d in /sys/class/hwmon/hwmon*; do [[ -r $d/name ]] || continue; IFS= read -r name < \"$d/name\"; "
            + "case $name in coretemp|k10temp|zenpower|cpu_thermal) "
            + "for input in \"$d\"/temp*_input; do [[ -r $input ]] || continue; raw=0; IFS= read -r raw < \"$input\"; [[ $raw =~ ^[0-9]+$ ]] || continue; "
            + "label_file=${input%_input}_label; label=''; [[ -r $label_file ]] && IFS= read -r label < \"$label_file\"; value=$((raw / 1000)); "
            + "case $label in 'Package id 0'|Tctl|Tdie|'CPU Package'|CPU) cpu=$value; max_file=${input%_input}_max; crit_file=${input%_input}_crit; "
            + "[[ -r $max_file ]] && { IFS= read -r v < \"$max_file\"; [[ $v =~ ^[0-9]+$ ]] && cpu_max=$((v / 1000)); }; "
            + "[[ -r $crit_file ]] && { IFS= read -r v < \"$crit_file\"; [[ $v =~ ^[0-9]+$ ]] && cpu_crit=$((v / 1000)); };; "
            + "Core*) (( value > core )) && core=$value;; esac; (( cpu == 0 )) && cpu=$value; done;; "
            + "nvme) for label_file in \"$d\"/temp*_label; do [[ -r $label_file ]] || continue; IFS= read -r label < \"$label_file\"; [[ $label == Composite ]] || continue; "
            + "input=${label_file%_label}_input; [[ -r $input ]] || continue; IFS= read -r raw < \"$input\"; [[ $raw =~ ^[0-9]+$ ]] || continue; nvme=$((raw / 1000)); "
            + "max_file=${input%_input}_max; crit_file=${input%_input}_crit; [[ -r $max_file ]] && { IFS= read -r v < \"$max_file\"; [[ $v =~ ^[0-9]+$ ]] && nvme_max=$((v / 1000)); }; "
            + "[[ -r $crit_file ]] && { IFS= read -r v < \"$crit_file\"; [[ $v =~ ^[0-9]+$ ]] && nvme_crit=$((v / 1000)); }; break; done;; "
            + "jc42) for input in \"$d\"/temp*_input; do [[ -r $input ]] || continue; IFS= read -r raw < \"$input\"; [[ $raw =~ ^[0-9]+$ ]] || continue; "
            + "value=$((raw / 1000)); (( value > dimm )) && dimm=$value; done;; esac; done; "
            + "if (( cpu == 0 )); then for zone in /sys/class/thermal/thermal_zone*; do [[ -r $zone/type && -r $zone/temp ]] || continue; "
            + "IFS= read -r type < \"$zone/type\"; case $type in x86_pkg_temp|cpu-thermal|cpu_thermal) IFS= read -r raw < \"$zone/temp\"; "
            + "[[ $raw =~ ^[0-9]+$ ]] && { cpu=$((raw / 1000)); break; };; esac; done; fi; "
            + "printf '%s|%s|%s|%s|%s|%s|%s|%s\\n' \"$cpu\" \"$core\" \"$cpu_max\" \"$cpu_crit\" \"$nvme\" \"$nvme_max\" \"$nvme_crit\" \"$dimm\""]
        stdout: StdioCollector { onStreamFinished: theme.parseThermalTelemetry(this.text) }
    }

    Process {
        id: storageTelemetryProc
        command: ["bash", "-c",
            "LC_ALL=C df -P -B1 / 2>/dev/null | awk 'NR == 2 { gsub(/%/, \"\", $5); printf \"%s|%s|%s\\n\", $5, $3, $2 }'"]
        stdout: StdioCollector { onStreamFinished: theme.parseStorageTelemetry(this.text) }
    }

    Process {
        id: storageInventoryProc
        command: ["lsblk", "-J", "-b", "-o",
            "NAME,PATH,TYPE,SIZE,FSTYPE,FSUSED,FSAVAIL,FSUSE%,MOUNTPOINTS,MODEL,TRAN,ROTA,RM,HOTPLUG"]
        stdout: StdioCollector { onStreamFinished: theme.parseStorageInventory(this.text) }
    }

    Timer {
        interval: ((theme.modCpu || theme.cpuVisible || theme.modMemory || theme.memVisible) ? 2000 : 10000) * Perf.pollFactor
        running: true; repeat: true; triggeredOnStart: true
        onTriggered: {
            systemCpuFile.reload()
            systemLoadFile.reload()
            systemMemFile.reload()
        }
    }

    Timer {
        interval: 2500 * Perf.pollFactor
        running: theme.cpuVisible
        repeat: true
        triggeredOnStart: true
        onTriggered: if (!cpuDetailProc.running) cpuDetailProc.running = true
    }

    Timer {
        interval: 3000 * Perf.pollFactor
        running: theme.cpuVisible
        repeat: true
        triggeredOnStart: true
        onTriggered: if (!cpuTopProcessesProc.running) cpuTopProcessesProc.running = true
    }

    Timer {
        interval: 2500 * Perf.pollFactor
        running: theme.modGpu || theme.gpuVisible || theme.thermalVisible
        repeat: true
        triggeredOnStart: true
        onTriggered: if (!gpuTelemetryProc.running) gpuTelemetryProc.running = true
    }

    Timer {
        interval: 5000 * Perf.pollFactor
        running: theme.modCpuTemperature || theme.cpuVisible || theme.thermalVisible
        repeat: true
        triggeredOnStart: true
        onTriggered: if (!cpuTemperatureProc.running) cpuTemperatureProc.running = true
    }

    Timer {
        interval: 30000 * Perf.pollFactor
        running: theme.modStorage || theme.storageVisible
        repeat: true
        triggeredOnStart: true
        onTriggered: if (!storageTelemetryProc.running) storageTelemetryProc.running = true
    }

    Timer {
        interval: (theme.storageVisible ? 5000 : 60000) * Perf.pollFactor
        running: theme.modStorage || theme.storageVisible
        repeat: true
        triggeredOnStart: true
        onTriggered: if (!storageInventoryProc.running) storageInventoryProc.running = true
    }

    // ── Memory panel state ──
    property bool memVisible: false
    onMemVisibleChanged: popupOpened("memVisible")

    // ── Volume panel state ──
    property bool volVisible: false
    property real volReveal: volVisible ? 1 : 0
    Behavior on volReveal {
        NumberAnimation {
            duration: theme.volVisible ? 160 : 120
            easing.type: theme.volVisible ? Easing.OutCubic : Easing.InCubic
        }
    }
    onVolVisibleChanged: popupOpened("volVisible")

    // ── Control center state ──
    property bool controlVisible: false
    onControlVisibleChanged: {
        popupOpened("controlVisible")
        if (!controlVisible) wwSubVisible = false
    }

    // ── Bar layout / unlock (drag&drop reorder). barUnlocked is transient. ──
    property bool barUnlocked: false
    property var  fnDefaultLayout: function () { theme.resetAllBarLayouts() }
    property bool wwSubVisible: false   // "Widgets & Workspaces" fly-out

    // ── module enable flags (controlled by ControlPanel) ──
    property bool modStatus:     true
    property bool modMemory:     true
    property bool modCpu:        true
    property bool modCpuTemperature: true
    property bool modGpu:        true
    property bool modStorage:    true
    property bool modVolume:     true
    property bool modWeather:    true
    property bool modNetwork:    true
    property bool modLayout:     true
    property string networkMode: "none"   // mirrored from NetworkWidget: wifi/ethernet/none
    // Centralized status indicators. These live on Theme so BarSlot-per-monitor
    // widgets don't each spawn their own status poller.
    property bool stayAwake: false            // idle lock disabled / stay-awake indicator
    readonly property bool hypridleAwake: stayAwake // compatibility alias for older modules
    property bool _idleBackendChecked: false
    property bool _idleRyokuShellBackend: false
    property bool _idleRyokuShellSystem: false
    property bool _omarchyBackendReprobePending: false
    property int _omarchyBackendRetryIndex: 0
    readonly property var _omarchyBackendRetryDelays: [2000, 5000, 15000]
    readonly property string omarchyShellConfigPath: Quickshell.env("HOME") + "/.config/ryoku/shell.json"
    readonly property string idleStatePath: Quickshell.env("HOME") + "/.local/state/ryoku/indicators/stay-awake"
    property bool notifSilenced: false        // notification do-not-disturb mode
    property bool _notifBackendChecked: false
    property bool _notifRyokuShellBackend: false
    property bool _notifRyokuShellSystem: false
    // Whether `makoctl` (the external mako DND source used in Omarchy-compat mode)
    // is on PATH. Ryoku runs its own notification server and never ships mako, so
    // this stays false there and the makoctl DND probe is skipped instead of
    // failing to spawn on every status refresh (endless "binary not found" log
    // spam). Detected once by makoDetectProc below.
    property bool _makoAvailable: false
    readonly property string notificationsStatePath: Quickshell.env("HOME") + "/.local/state/ryoku/notifications.json"
    property bool screenRecording: false
    property int screenRecordingElapsed: 0
    property string _screenRecordingPid: ""
    property string _screenRecordingElapsedProbePid: ""
    property int _screenRecordingBaseElapsed: 0
    property real _screenRecordingBaseMs: 0
    readonly property string screenRecordingStatePath: "/tmp/ryoku-screenrecord-filename"
    property bool _recordingRefreshPending: false
    property string voxState: "idle"          // idle/recording/transcribing
    property string voxHint: ""
    property bool voxAvailable: true
    readonly property bool _statusPollingWanted: modStatus
    readonly property bool _voxActive: voxState === "recording" || voxState === "transcribing"

    function refreshIdleStatus() {
        if (!_idleBackendChecked) return
        if (_idleRyokuShellSystem) {
            idleStateFile.reload()
            return
        }
        if (!idleProc.running) idleProc.running = true
    }
    function refreshStatusIndicators() {
        refreshIdleStatus()
        refreshNotificationStatus()
    }
    function reprobeRyokuShellBackends() {
        if (idleBackendProc.running || notifBackendProc.running) {
            _omarchyBackendReprobePending = true
            return
        }
        _omarchyBackendReprobePending = false
        idleBackendProc.running = true
        notifBackendProc.running = true
    }
    function finishRyokuBackendReprobe() {
        if (idleBackendProc.running || notifBackendProc.running) return
        if (_omarchyBackendReprobePending) {
            omarchyBackendProbeDebounce.restart()
            return
        }
        scheduleRyokuBackendRetry()
    }
    function omarchyBackendRetryNeeded() {
        return (_idleRyokuShellSystem && !_idleRyokuShellBackend)
            || (_notifRyokuShellSystem && !_notifRyokuShellBackend)
    }
    function scheduleRyokuBackendRetry() {
        if (!omarchyBackendRetryNeeded()) {
            omarchyBackendRetryTimer.stop()
            _omarchyBackendRetryIndex = 0
            return
        }
        if (omarchyBackendConfirmTimer.running || omarchyBackendRetryTimer.running
                || _omarchyBackendRetryIndex >= _omarchyBackendRetryDelays.length) return
        omarchyBackendRetryTimer.interval = _omarchyBackendRetryDelays[_omarchyBackendRetryIndex]
        _omarchyBackendRetryIndex++
        omarchyBackendRetryTimer.restart()
    }
    function resetRyokuBackendProbes() {
        omarchyBackendRetryTimer.stop()
        _omarchyBackendRetryIndex = 0
        omarchyBackendProbeDebounce.restart()
        omarchyBackendConfirmTimer.restart()
        if (!makoDetectProc.running) makoDetectProc.running = true
    }
    function parseNotificationsState(text) {
        try {
            var parsed = JSON.parse(String(text || "{}"))
            notifSilenced = parsed && parsed.dnd === true
        } catch (e) {
            notifSilenced = false
        }
    }
    function refreshNotificationStatus() {
        if (!_notifBackendChecked) return
        if (_notifRyokuShellSystem) {
            notificationsStateFile.reload()
            return
        }
        // No Ryoku notification backend: read DND from mako, but only if it is
        // actually installed. Without this guard the Process fails to spawn on
        // every refresh (mako is not a Ryoku dependency), spamming the log.
        if (_makoAvailable && !dndProc.running) dndProc.running = true
    }
    function refreshRecordingStatus() {
        if (recordingPidProc.running) {
            _recordingRefreshPending = true
            return
        }
        recordingPidProc.running = true
    }
    function reconcileSlowStatusIndicators() {
        refreshRecordingStatus()
    }
    function setScreenRecordingPid(pid) {
        pid = String(pid || "").trim()
        if (pid === _screenRecordingPid) return

        _screenRecordingPid = pid
        if (pid === "") {
            screenRecording = false
            screenRecordingElapsed = 0
            _screenRecordingBaseElapsed = 0
            _screenRecordingBaseMs = 0
            _screenRecordingElapsedProbePid = ""
            return
        }

        screenRecording = true
        screenRecordingElapsed = 0
        _screenRecordingBaseElapsed = 0
        _screenRecordingBaseMs = Date.now()
        _screenRecordingElapsedProbePid = pid
        recordingElapsedProc.command = ["ps", "-o", "etimes=", "-p", pid]
        recordingElapsedProc.running = false
        recordingElapsedProc.running = true
    }
    function updateScreenRecordingElapsed() {
        if (!screenRecording || _screenRecordingBaseMs <= 0) return
        screenRecordingElapsed = _screenRecordingBaseElapsed + Math.floor((Date.now() - _screenRecordingBaseMs) / 1000)
    }
    function refreshVoxtypeStatus() {
        if (!voxAvailable && !modStatus) return
        if (voxProc.running) return
        voxProc.running = true
    }

    Process {
        id: idleBackendProc
        command: ["bash", "-c", "root=${RYOKU_PATH:-/usr/share/ryoku}; [[ -f $root/shell/plugins/services/idle/manifest.json ]] || exit 2; command -v true >/dev/null 2>&1 && RYOKU_PATH=$root true idle status >/dev/null 2>&1"]
        running: true
        onExited: (exitCode) => {
            theme._idleRyokuShellSystem = exitCode !== 2
            theme._idleRyokuShellBackend = exitCode === 0
            theme._idleBackendChecked = true
            theme.refreshIdleStatus()
            theme.finishRyokuBackendReprobe()
        }
    }

    FileView {
        id: idleStateFile
        path: theme.idleStatePath
        watchChanges: theme._idleRyokuShellSystem
        printErrors: false
        onFileChanged: idleStateFile.reload()
        onLoaded: {
            if (theme._idleRyokuShellSystem) theme.stayAwake = true
        }
        onLoadFailed: {
            if (theme._idleRyokuShellSystem) theme.stayAwake = false
        }
    }

    Process {
        id: notifBackendProc
        command: ["bash", "-c", "root=${RYOKU_PATH:-/usr/share/ryoku}; [[ -f $root/shell/plugins/notifications/manifest.json ]] || exit 2; command -v true >/dev/null 2>&1 && RYOKU_PATH=$root true notifications ping 2>/dev/null | grep -Fxq ok"]
        running: true
        onExited: (exitCode) => {
            theme._notifRyokuShellSystem = exitCode !== 2
            theme._notifRyokuShellBackend = exitCode === 0
            theme._notifBackendChecked = true
            theme.refreshNotificationStatus()
            theme.finishRyokuBackendReprobe()
        }
    }

    FileView {
        id: notificationsStateFile
        path: theme.notificationsStatePath
        watchChanges: theme._notifRyokuShellSystem
        printErrors: false
        onFileChanged: notificationsStateFile.reload()
        onLoaded: {
            if (theme._notifRyokuShellSystem) theme.parseNotificationsState(notificationsStateFile.text())
        }
        onLoadFailed: {
            if (theme._notifRyokuShellSystem) theme.notifSilenced = false
        }
    }

    Timer {
        id: omarchyBackendProbeDebounce
        interval: 250
        repeat: false
        onTriggered: theme.reprobeRyokuShellBackends()
    }

    Timer {
        id: omarchyBackendRetryTimer
        repeat: false
        onTriggered: theme.reprobeRyokuShellBackends()
    }

    Timer {
        id: omarchyBackendConfirmTimer
        interval: 2000
        repeat: false
        onTriggered: {
            theme._omarchyBackendRetryIndex = Math.max(1, theme._omarchyBackendRetryIndex)
            theme.reprobeRyokuShellBackends()
        }
    }

    FileView {
        id: omarchyShellConfigFile
        path: theme.omarchyShellConfigPath
        watchChanges: true
        printErrors: false
        onFileChanged: {
            omarchyShellConfigFile.reload()
            theme.resetRyokuBackendProbes()
        }
    }

    Process {
        id: idleProc
        command: ["pgrep", "-x", "hypridle"]
        running: false
        onExited: (exitCode) => {
            if (!theme._idleRyokuShellSystem) theme.stayAwake = exitCode !== 0
        }
    }

    // One-shot: is makoctl on PATH? Gates the DND probe above so a box without
    // mako (every Ryoku box) never tries to spawn it. Re-runs when the backend is
    // reprobed via resetRyokuBackendProbes(), so installing mako is picked up.
    Process {
        id: makoDetectProc
        command: ["sh", "-c", "command -v makoctl"]
        running: true
        onExited: (exitCode) => {
            theme._makoAvailable = exitCode === 0
            if (theme._makoAvailable) theme.refreshNotificationStatus()
        }
    }

    Process {
        id: dndProc
        command: ["makoctl", "mode"]
        running: false
        onExited: (exitCode) => {
            if (exitCode !== 0 && !theme._notifRyokuShellSystem) theme.notifSilenced = false
        }
        stdout: StdioCollector {
            onStreamFinished: {
                if (!theme._notifRyokuShellSystem) theme.notifSilenced = this.text.indexOf("do-not-disturb") >= 0
            }
        }
    }

    Process {
        id: recordingPidProc
        command: ["pgrep", "-o", "-f", "^gpu-screen-recorder"]
        running: false
        onExited: (exitCode) => {
            if (exitCode !== 0) theme.setScreenRecordingPid("")
            if (theme._recordingRefreshPending) {
                theme._recordingRefreshPending = false
                theme.refreshRecordingStatus()
            }
        }
        stdout: StdioCollector {
            onStreamFinished: {
                var parts = this.text.trim().split(/\s+/)
                theme.setScreenRecordingPid(parts[0] || "")
            }
        }
    }

    FileView {
        id: screenRecordingStateFile
        path: theme.screenRecordingStatePath
        watchChanges: true
        printErrors: false
        onFileChanged: {
            screenRecordingStateFile.reload()
            theme.refreshRecordingStatus()
        }
        onLoaded: theme.refreshRecordingStatus()
        onLoadFailed: theme.refreshRecordingStatus()
    }

    Process {
        id: recordingElapsedProc
        command: ["true"]
        running: false
        stdout: StdioCollector {
            onStreamFinished: {
                if (theme._screenRecordingElapsedProbePid !== theme._screenRecordingPid) return
                var elapsed = parseInt(this.text.trim())
                if (isNaN(elapsed)) elapsed = 0
                theme._screenRecordingBaseElapsed = Math.max(0, elapsed)
                theme._screenRecordingBaseMs = Date.now()
                theme.updateScreenRecordingElapsed()
            }
        }
    }

    Timer {
        interval: 1500
        running: theme._statusPollingWanted
        repeat: true
        triggeredOnStart: true
        onTriggered: theme.refreshStatusIndicators()
    }

    Timer {
        interval: 45000
        running: theme._statusPollingWanted
        repeat: true
        triggeredOnStart: true
        onTriggered: theme.reconcileSlowStatusIndicators()
    }

    Timer {
        interval: 1000
        running: theme.screenRecording
        repeat: true
        triggeredOnStart: true
        onTriggered: theme.updateScreenRecordingElapsed()
    }

    Process {
        id: voxProc
        command: ["bash", "-c",
            "if command -v voxtype >/dev/null 2>&1; then " +
            "timeout 1 voxtype status --extended --format json 2>/dev/null | jq -r '[(.class // .alt // \"idle\"), ((.tooltip // \"\") | split(\"\\n\")[0])] | @tsv' 2>/dev/null; " +
            "else echo 'MISSING'; fi"
        ]
        running: false
        stdout: StdioCollector {
            onStreamFinished: {
                var parts = this.text.trim().split("\t")
                if (parts[0] === "MISSING") {
                    theme.voxAvailable = false
                    theme.voxState = "idle"
                    theme.voxHint = ""
                    return
                }
                theme.voxAvailable = true
                theme.voxState = parts[0] || "idle"
                theme.voxHint = parts[1] || ""
            }
        }
    }
    Timer {
        interval: theme._voxActive ? 1000 : (theme.voxAvailable ? 10000 : 60000)
        running: theme._statusPollingWanted
        repeat: true
        triggeredOnStart: true
        onTriggered: theme.refreshVoxtypeStatus()
    }
    // battery presence (laptop) - drives the Battery indicator tile's visibility (shown only
    // where a battery exists, like Brightness uses hasBacklight). Direct UPower check, event-driven.
    readonly property bool hasBattery: UPower.displayDevice !== null && UPower.displayDevice.isLaptopBattery
    // NetworkManager active (Ryoku 4.0) → the panel's iwctl scan/connect won't work,
    // so it shows an "open nmtui" button instead of an empty list
    property bool useNM: false
    Process {
        command: ["bash", "-c", "systemctl is-active --quiet NetworkManager && echo 1 || echo 0"]
        running: true
        stdout: StdioCollector { onStreamFinished: theme.useNM = this.text.trim() === "1" }
    }

    // ── wifi/bluetooth settings launchers ──
    // Both open Ryoku Hub's Connections page (Wi-Fi, Bluetooth and Hotspot all
    // live there); the daemon spawns or navigates the single Hub instance. This
    // replaces the old terminal launchers (impala / bluetui / bluetoothctl) that
    // predate the Hub owning connection settings.
    readonly property string launchWifiCmd: "ryoku-shell hub open connections"
    readonly property string launchBtCmd:   "ryoku-shell hub open connections"
    property bool modPower:      false   // default off (toggle in ControlPanel)
    property bool modBluetooth:  false   // default off (toggle in ControlPanel)
    property bool modBrightness: true
    property bool modMedia:      true
    property bool modQuick:      true    // G10 group pill (idle-inhibitor · media · theme)
    property bool modMpris:      true    // G9 now-playing / mpris pill
    property string mprisBarStyle: "default" // "default" or "full"
    property bool modClaude:     false   // default off (toggle in ControlPanel)
    property bool modBattery:    true    // G12 battery pill (laptop batteries only)

    // backlight presence - set by BrightnessWidget once it probes /sys/class/backlight.
    // ControlPanel uses this to hide the Brightness toggle on desktops without one.
    property bool hasBacklight:  false

    // ── workspace display mode ──
    property string workspaceMode: "active"   // "10", "5", "active"
    // ── workspace display style (orthogonal to mode; persisted) ──
    // "rings" is the stable cache token for the user-facing Frame style.
    property string workspaceStyle: "default"   // default, numbers, magic, kanji, rings, aurora
    // The marker styles this variant offers (continuous V2 adds Kanji, Frame,
    // Aurora; "rings" is the persisted cache token for the Frame style).
    readonly property var workspaceStyleOptions: [
        { key: "default", label: I18n.tr("Dots") },
        { key: "numbers", label: I18n.tr("Numbers") },
        { key: "magic",   label: I18n.tr("Glyph") },
        { key: "kanji",   label: I18n.tr("Kanji") },
        { key: "rings",   label: I18n.tr("Frame") },
        { key: "aurora",  label: I18n.tr("Aurora") },
        { key: "pacman",  label: I18n.tr("Pacman") }
    ]

    // ── bar screen position (persisted) ──
    property string barPosition: "top"   // "top" or "bottom"
    // ── gap animation mode (Bar Studio; drives the V2 gap/reactor layer) ──
    // 0=off, 1=stream, 2=surge, 3=bolt, 7=reactor, 8=quotes.
    property int barAnim: 1
    onBarAnimChanged: if (_widgetsLoaded) saveWidgets()
    // Reactor IPC (barAnim modes 7/8): ParticleStream listens via onReactorTest.
    signal reactorTest(string kind, string arg)
    // ── outer bar shell (persisted) ──
    // islands = split pills: each populated region (left · center · right) is its
    //           own rounded pill, with the reactor stream flowing in the gaps
    // full    = edge-to-edge strip
    // fit     = centered content-width capsule
    // dock    = centered content-width surface attached to the screen edge
    // notch   = attached content-width surface with desktop-facing side wings
    // Scale the bar itself without changing the display's UI scale.
    property real barScale: 1
    function clampBarScale(value) {
        var n = Number(value)
        return isFinite(n) ? Math.round(Math.max(1, Math.min(2, n)) * 10) / 10 : 1
    }

    property string barShellStyle: "full"
    property bool barBorderEnabled: true
    // Outer bar-shell corner radius in px, applied to the fitted shell forms.
    // Persisted in shell.json .qsbar; set from the control center and Bar Studio.
    property int barCornerRadius: 6
    property bool panelTooltipBorderEnabled: true

    // Depth is the pill/panel/tooltip shadow; the bar shell keeps its own either way.
    property bool barShadowEnabled: false
    property bool barFrostEnabled: false
    // Auto-hide: reserve no space, slide off the edge, and reveal on a slow
    // hover anywhere along the edge (even over the gaps between islands).
    property bool barAutoHide: false

    // Gaps hold the shell off each output edge. Default top 3 matches the
    // reference's island offset.
    property int barGapTop: 3
    property int barGapBottom: 0
    property int barGapLeft: 0
    property int barGapRight: 0
    function clampGap(value) {
        var n = Math.round(Number(value) || 0)
        return Math.max(-12, Math.min(64, n))
    }
    // Lead faces the anchored edge, trail the desktop, so consumers skip barPosition.
    readonly property int barGapLead: barPosition === "bottom" ? barGapBottom : barGapTop
    readonly property int barGapTrail: barPosition === "bottom" ? barGapTop : barGapBottom
    function barShellStyleValid(value) {
        return value === "islands" || value === "full" || value === "fit"
            || value === "dock" || value === "notch"
    }

    // ── picker visual style (theme/wallpaper/screenshot/video pickers) ──
    property string pickerStyle: "tanzaku"   // "tanzaku", "hearthstone", "carousel"
    property string launcherLogoMode: "text"     // "text" or "icon"
    property string launcherLogoText: "ryoku"    // "ryoku", "omarchy", "hyprland", "arch", or "omacom"
    property string launcherLogoIcon: "ryoku"    // see launcherLogoIconGlyph()
    // Legacy launcher-logo text values migrated to the current text mode.
    readonly property var legacyLogoTextValues: ["omarchy", "hyprland"]
    property bool   weatherImperial: false   // false = °C / km·h, true = °F / mph
    property bool   clock12h:        false   // false = 24h, true = 12h (AM/PM)

    // ── widget/workspace state persistence ──
    property var barSeps: []
    property var iconOnlyGids: []
    function sepAfter(gid) { return barSeps.indexOf(gid) >= 0 }
    function iconOnly(gid) { return iconOnlyGids.indexOf(gid) >= 0 }
    function toggleSep(gid) {
        var a = barSeps.slice()
        var i = a.indexOf(gid)
        if (i >= 0) a.splice(i, 1); else a.push(gid)
        barSeps = a
        if (_widgetsLoaded) saveWidgets()
    }
    function toggleIconOnly(gid) {
        if (gid === "G3") return // Status is an icon-only group by design.
        var a = iconOnlyGids.slice()
        var i = a.indexOf(gid)
        if (i >= 0) a.splice(i, 1); else a.push(gid)
        iconOnlyGids = a
        if (_widgetsLoaded) saveWidgets()
    }
    // Per-widget geometry overrides (continuous bar), matching upstream Shibumi's
    // bar.shibumi.widgets.Gx.appearance.v2: a per-GID {pad,radius,opacity}. Unset
    // keys inherit the shared defaults, so the curated look is unchanged until a
    // widget is deliberately customised in the Control Center.
    property var widgetGeom: ({})
    function widgetGeomOf(gid) { var g = widgetGeom[gid]; return g ? g : ({}) }
    function widgetPad(gid) { var g = widgetGeomOf(gid); return g.pad !== undefined ? g.pad : 0 }
    function widgetRadius(gid) { var g = widgetGeomOf(gid); return (g.radius !== undefined && g.radius >= 0) ? g.radius : panelButtonRadius }
    function widgetOpacity(gid) { var g = widgetGeomOf(gid); return (g.opacity !== undefined && g.opacity >= 0) ? g.opacity : 1 }
    function widgetGeomCustomized(gid) { return widgetGeom[gid] !== undefined }
    function setWidgetGeom(gid, key, value) {
        var m = Object.assign({}, widgetGeom)
        var g = Object.assign({}, m[gid] || ({}))
        g[key] = value
        m[gid] = g
        widgetGeom = m
        if (_widgetsLoaded) persistWidgetsToConfig()
    }
    function resetWidgetGeom(gid) {
        if (widgetGeom[gid] === undefined) return
        var m = Object.assign({}, widgetGeom)
        delete m[gid]
        widgetGeom = m
        if (_widgetsLoaded) persistWidgetsToConfig()
    }
    function parseGidCsv(s) {
        if (!s || s === "-") return []
        var out = []
        var toks = s.split(",")
        for (var i = 0; i < toks.length; i++)
            if (/^G\d{1,2}$/.test(toks[i]) && toks[i] !== "G3") out.push(toks[i])
        return out
    }

    // ── per-widget palette styles ──
    // Stored by stable GID, so slot reordering and temporarily hidden widgets
    // never lose their visual assignment.
    property var widgetColorStyles: ({})
    function widgetGidValid(gid) {
        var m = String(gid || "").match(/^G(\d{1,2})$/)
        if (!m) return false
        var n = Number(m[1])
        return n >= 1 && n <= 19
    }
    function widgetColorModeValid(mode) {
        return mode === "fill" || mode === "border" || mode === "both"
    }
    function normalizedWidgetColorMode(mode, colorId) {
        var borderOn = mode === "both" || mode === "border"
        if (!borderOn) return "fill"
        return colorId === "inherit" ? "border" : "both"
    }
    function widgetToneValid(tone) {
        return tone === "auto" || tone === "background" || tone === "foreground"
    }
    function widgetBorderWidthValid(w) {
        return w === 0.5 || w === 1 || w === 1.5 || w === 2
    }
    function widgetBorderColorKeyValid(key) {
        return key === "inherit" || key === "surface" || paletteColorValid(key)
    }
    function widgetColorStyle(gid) {
        var raw = widgetColorStyles[gid]
        var colorId = raw && (raw.color === "inherit" || paletteColorValid(raw.color))
            ? raw.color
            : "inherit"
        return {
            color: colorId,
            mode: raw && widgetColorModeValid(raw.mode)
                ? normalizedWidgetColorMode(raw.mode, colorId)
                : "fill",
            tone: raw && widgetToneValid(raw.tone) ? raw.tone : "auto",
            borderWidth: raw && widgetBorderWidthValid(raw.borderWidth) ? raw.borderWidth : 1,
            borderColorKey: raw && widgetBorderColorKeyValid(raw.borderColorKey) ? raw.borderColorKey : "inherit"
        }
    }
    function _storeWidgetColorStyle(gid, colorId, mode, tone, borderWidth, borderColorKey) {
        if (!widgetGidValid(gid)) return
        var next = {}
        for (var key in widgetColorStyles) next[key] = widgetColorStyles[key]

        var storedColor = colorId === "inherit"
            ? "inherit"
            : (paletteColorValid(colorId) ? colorId : "color01")
        var storedMode = normalizedWidgetColorMode(mode, storedColor)
        if (storedColor === "inherit" && storedMode !== "border") {
            delete next[gid]
        } else {
            next[gid] = {
                color: storedColor,
                mode: storedMode,
                tone: widgetToneValid(tone) ? tone : "auto",
                borderWidth: widgetBorderWidthValid(borderWidth) ? borderWidth : 1,
                borderColorKey: widgetBorderColorKeyValid(borderColorKey) ? borderColorKey : "inherit"
            }
        }
        widgetColorStyles = next
        if (_widgetsLoaded) saveWidgets()
    }
    function setWidgetColorStyle(gid, colorId, mode, tone) {
        var cur = widgetColorStyle(gid)
        _storeWidgetColorStyle(gid, colorId, mode, tone, cur.borderWidth, cur.borderColorKey)
    }
    function setWidgetBorderWidth(gid, w) {
        var cur = widgetColorStyle(gid)
        _storeWidgetColorStyle(gid, cur.color, cur.mode, cur.tone, w, cur.borderColorKey)
    }
    function setWidgetBorderColorKey(gid, key) {
        var cur = widgetColorStyle(gid)
        _storeWidgetColorStyle(gid, cur.color, cur.mode, cur.tone, cur.borderWidth, key)
    }
    function setWidgetPaletteColor(gid, colorId) {
        var style = widgetColorStyle(gid)
        setWidgetColorStyle(gid, colorId, style.mode, style.tone)
    }
    function setWidgetColorMode(gid, mode) {
        var style = widgetColorStyle(gid)
        setWidgetColorStyle(gid, style.color, mode, style.tone)
    }
    function setWidgetBorderEnabled(gid, enabled) {
        var style = widgetColorStyle(gid)
        setWidgetColorStyle(gid, style.color,
            enabled ? (style.color === "inherit" ? "border" : "both") : "fill",
            style.tone)
    }
    function setWidgetTone(gid, tone) {
        var style = widgetColorStyle(gid)
        if (style.color !== "inherit") setWidgetColorStyle(gid, style.color, style.mode, tone)
    }
    function resetWidgetColor(gid) {
        var style = widgetColorStyle(gid)
        setWidgetColorStyle(gid, "inherit",
            style.mode === "both" || style.mode === "border" ? "border" : "fill",
            "auto")
    }
    function resetAllWidgetFillColors() {
        var next = {}
        var changed = false

        for (var n = 1; n <= 18; n++) {
            var gid = "G" + n
            var style = widgetColorStyle(gid)

            if (style.color !== "inherit") changed = true
            if (style.mode === "border" || style.mode === "both") {
                next[gid] = {
                    color: "inherit",
                    mode: "border",
                    tone: "auto",
                    borderWidth: style.borderWidth,
                    borderColorKey: style.borderColorKey
                }
            }
        }

        if (changed) widgetColorStyles = next
        return changed
    }
    function widgetPaletteId(gid) { return widgetColorStyle(gid).color }
    function widgetColorMode(gid) { return widgetColorStyle(gid).mode }
    function widgetTone(gid) { return widgetColorStyle(gid).tone }
    function widgetBorderWidth(gid) { return widgetColorStyle(gid).borderWidth }
    function widgetBorderColorKey(gid) { return widgetColorStyle(gid).borderColorKey }
    function widgetHasFill(gid) {
        var style = widgetColorStyle(gid)
        return style.color !== "inherit"
    }
    function widgetHasBorder(gid) {
        var style = widgetColorStyle(gid)
        return style.mode === "border" || style.mode === "both"
    }
    function widgetAssignedColor(gid) {
        var id = widgetPaletteId(gid)
        return id === "inherit" ? seal : paletteColor(id)
    }
    function _linearColorChannel(v) {
        return v <= 0.04045 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4)
    }
    function _relativeLuminance(c) {
        return 0.2126 * _linearColorChannel(c.r)
             + 0.7152 * _linearColorChannel(c.g)
             + 0.0722 * _linearColorChannel(c.b)
    }
    function _contrastRatio(a, b) {
        var la = _relativeLuminance(a)
        var lb = _relativeLuminance(b)
        return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05)
    }
    function paletteContrastColor(id) {
        var fill = paletteColor(id)
        return _contrastRatio(fill, paper) >= _contrastRatio(fill, ink) ? paper : ink
    }
    function widgetContrastColor(gid) {
        var fill = widgetAssignedColor(gid)
        var tone = widgetTone(gid)
        if (tone === "background") return paper
        if (tone === "foreground") return ink
        return _contrastRatio(fill, paper) >= _contrastRatio(fill, ink) ? paper : ink
    }
    function widgetContentColor(gid, fallback) {
        return widgetHasFill(gid) ? widgetContrastColor(gid) : fallback
    }
    function widgetFillColor(gid) {
        return widgetHasFill(gid) ? widgetAssignedColor(gid) : Qt.rgba(0, 0, 0, 0)
    }
    function widgetBorderColor(gid) {
        if (!widgetHasBorder(gid)) return Qt.rgba(0, 0, 0, 0)
        var key = widgetBorderColorKey(gid)
        if (key === "inherit") return panelBorder
        if (key === "surface") return widgetAssignedColor(gid)
        return paletteColor(key)
    }
    function serializeWidgetColorStyles() {
        var out = []
        for (var n = 1; n <= 19; n++) {
            var gid = "G" + n
            var style = widgetColorStyle(gid)
            if (style.color !== "inherit" || style.mode === "border")
                out.push(gid + "~" + style.color + "~" + style.mode + "~" + style.tone
                    + "~" + style.borderWidth + "~" + style.borderColorKey)
        }
        return out.length ? out.join(",") : "-"
    }
    function parseWidgetColorStyles(raw) {
        var out = {}
        if (!raw || raw === "-") return out
        var entries = String(raw).split(",")
        for (var i = 0; i < entries.length; i++) {
            var fields = entries[i].split("~")
            if ((fields.length !== 4 && fields.length !== 6) || !widgetGidValid(fields[0])
                    || (fields[1] !== "inherit" && !paletteColorValid(fields[1]))
                    || !widgetColorModeValid(fields[2])
                    || !widgetToneValid(fields[3])) continue
            var bw = fields.length === 6 && widgetBorderWidthValid(Number(fields[4])) ? Number(fields[4]) : 1
            var bck = fields.length === 6 && widgetBorderColorKeyValid(fields[5]) ? fields[5] : "inherit"
            out[fields[0]] = {
                color: fields[1],
                mode: normalizedWidgetColorMode(fields[2], fields[1]),
                tone: fields[3],
                borderWidth: bw,
                borderColorKey: bck
            }
        }
        return out
    }

    readonly property string widgetsCachePath: Quickshell.env("HOME") + "/.cache/quickshell_widgets_v2"
    property bool _widgetsLoaded: false

    onModMemoryChanged:     if (_widgetsLoaded) saveWidgets()
    onModCpuTemperatureChanged: if (_widgetsLoaded) saveWidgets()
    onModGpuChanged:        if (_widgetsLoaded) saveWidgets()
    onModStorageChanged:    if (_widgetsLoaded) saveWidgets()
    onModBrightnessChanged: if (_widgetsLoaded) saveWidgets()
    onModClaudeChanged:     if (_widgetsLoaded) saveWidgets()
    onModPowerChanged:      if (_widgetsLoaded) saveWidgets()
    onModBluetoothChanged:  if (_widgetsLoaded) saveWidgets()
    onModNetworkChanged:    if (_widgetsLoaded) saveWidgets()
    onModStatusChanged:     if (_widgetsLoaded) saveWidgets()
    onModQuickChanged:      if (_widgetsLoaded) saveWidgets()
    onModCpuChanged:        if (_widgetsLoaded) saveWidgets()
    onModVolumeChanged:     if (_widgetsLoaded) saveWidgets()
    onModMprisChanged:      if (_widgetsLoaded) saveWidgets()
    onMprisBarStyleChanged: if (_widgetsLoaded) saveWidgets()
    onAiToolsChanged:       if (_widgetsLoaded) saveWidgets()
    // straight to the daemon: this key is not in the positional widgets record
    onAudioBoostChanged:    if (_widgetsLoaded) persistWidgetsToConfig()
    onWorkspaceModeChanged: if (_widgetsLoaded) saveWidgets()
    onPickerStyleChanged:   if (_widgetsLoaded) saveWidgets()
    onLauncherLogoModeChanged: if (_widgetsLoaded) saveWidgets()
    onLauncherLogoTextChanged: if (_widgetsLoaded) saveWidgets()
    onLauncherLogoIconChanged: if (_widgetsLoaded) saveWidgets()
    onWeatherImperialChanged: if (_widgetsLoaded) saveWidgets()
    onClock12hChanged:        if (_widgetsLoaded) saveWidgets()
    onWorkspaceStyleChanged:   if (_widgetsLoaded) saveWidgets()
    onBarPositionChanged:      if (_widgetsLoaded) saveWidgets()
    onBarScaleChanged:         if (_widgetsLoaded) saveWidgets()
    onBarShellStyleChanged:    if (_widgetsLoaded) saveWidgets()
    onBarBorderEnabledChanged: if (_widgetsLoaded) saveWidgets()
    onPanelTooltipBorderEnabledChanged: if (_widgetsLoaded) saveWidgets()
    onBarCornerRadiusChanged:  if (_widgetsLoaded) saveWidgets()
    onBarColorChanged:         if (_widgetsLoaded) saveWidgets()
    onBarTemperatureSourceChanged: if (_widgetsLoaded) saveWidgets()
    onModBatteryChanged:       if (_widgetsLoaded) saveWidgets()
    onBarShadowEnabledChanged: if (_widgetsLoaded) saveWidgets()
    onModLayoutChanged:        if (_widgetsLoaded) saveWidgets()
    onBarFrostEnabledChanged:  if (_widgetsLoaded) saveWidgets()
    onBarAutoHideChanged:      if (_widgetsLoaded) saveWidgets()
    onBarGapTopChanged:        if (_widgetsLoaded) saveWidgets()
    onBarGapBottomChanged:     if (_widgetsLoaded) saveWidgets()
    onBarGapLeftChanged:       if (_widgetsLoaded) saveWidgets()
    onBarGapRightChanged:      if (_widgetsLoaded) saveWidgets()

    function saveWidgets() {
        var line = (modMemory    ? "1" : "0") + " "
                 + (modBrightness ? "1" : "0") + " "
                 + (modClaude    ? "1" : "0") + " "
                 + (modPower     ? "1" : "0") + " "
                 + (modBluetooth ? "1" : "0") + " "
                 + workspaceMode + " "
                 + pickerStyle + " "
                 + (weatherImperial ? "1" : "0") + " "
                 + (clock12h        ? "1" : "0") + " "
                 + (modNetwork      ? "1" : "0") + " "
                 + "0 "                                     // +5 legacy shadow field (V2 disabled)
                 + "0 0 "                                     // +6..+7 retired V2 Style fields
                 + workspaceStyle + " "
                 + barPosition + " "
                 + "1 "                                     // +10 legacy border field (V2 always on)
                 + (modStatus ? "1" : "0") + " "          // +11 group pill: status (arch/tray/notif)
                 + (modQuick  ? "1" : "0") + " "          // +12 group pill: quick (idle/media/theme)
                 + (modCpu    ? "1" : "0") + " "          // +13
                 + (modVolume ? "1" : "0") + " "          // +14
                 + (modMpris  ? "1" : "0") + " "          // +15 now-playing / mpris
                 + "- "                                   // +16 retired single-AI-tool field
                 + "0 "                                     // +17 retired transparent/frost field
                 + launcherLogoMode + " "                 // +18 launcher logo mode (text/icon)
                 + launcherLogoText + " "                 // +19 text logo id
                 + launcherLogoIcon + " "                 // +20 icon logo id
                 + "0 "                                     // +21 retired updater package badge
                 + "0 "                                     // +22 retired updater clean-theme badge
                 + "1 1 1 1 1 1 1 1 "                       // +23..+30 legacy compact fields; V2 is always compact
                 + "0 "                                     // +31 retired updater shell badge
                 + barColor + " "                            // +32 V2 bar accent source
                 + (modGpu            ? "1" : "0") + " "  // +33 GPU load
                 + (modCpuTemperature ? "1" : "0") + " "  // +34 CPU temperature
                 + (modStorage        ? "1" : "0") + " "  // +35 root-filesystem usage
                 + (barSeps.length ? barSeps.join(",") : "-") + " "         // +36 separator gids (CSV, "-" = none)
                 + (iconOnlyGids.length ? iconOnlyGids.join(",") : "-") + " " // +37 icon-only gids (CSV, "-" = none)
                 + barTemperatureSource + " "                            // +38 temperature sensor shown in bar
                 + "0 "                                                  // +39 retired global widget-foreground switch
                 + serializeWidgetColorStyles() + " "                   // +40 per-GID palette styles
                 + mprisBarStyle + " "                                  // +41 now-playing bar presentation
                 + barShellStyle + " "                                  // +42 outer bar shell
                 + (barBorderEnabled ? "1" : "0") + " "                 // +43 outer bar border
                 + (panelTooltipBorderEnabled ? "1" : "0") + " "        // +44 panel + tooltip outer border
                 + barAnim                                              // +45 gap-animation mode
                 + (modLayout ? "1" : "0")                              // +46 keyboard layout
        widgetSaveProc.command = ["bash", "-c",
            "echo '" + line + "' > '" + widgetsCachePath + "'"]
        widgetSaveProc.running = false
        widgetSaveProc.running = true
        persistWidgetsToConfig()
    }

    // Ryoku's own marks, plus the neighbours whose wordmarks we actually ship.
    // "omarchy" is deliberately absent from both lists: its mark is a Private Use
    // Area glyph in a font we do not ship, and it was also the dead default these
    // ids carried while the launcher drew a hardcoded mark. Leaving it out makes
    // every one of those stale records fail validation and land on "ryoku", so an
    // existing desktop keeps its own brand instead of being silently rebranded.
    readonly property var launcherLogoTextOptions: ["ryoku", "hyprland", "arch", "omacom", "linux", "nixos", "debian", "fedora", "gentoo", "void", "wayland", "sway", "gnome", "dwm"]
    readonly property var launcherLogoIconOptions: ["ryoku", "hyprland", "arch", "grid", "spark", "power", "dragon", "mark", "nix", "branch", "rebel", "ubuntu", "debian", "fedora", "gentoo", "void", "artix", "manjaro", "suse", "alpine", "endeavour", "garuda", "cachyos", "freebsd", "apple", "raspi", "elementary", "gnome", "alma", "centos", "devuan", "arco", "ferris", "codeberg", "gitea", "tux", "android", "mint", "kali", "popos", "zorin", "plasma", "wayland", "docker", "github", "git", "gitlab", "python", "rust", "go", "node", "react", "vue", "kube", "vim", "neovim", "firefox", "chrome", "java", "js", "ts", "cpp", "ruby", "php", "swift", "kotlin", "lua", "haskell", "blender", "figma", "redis", "postgres", "steam", "spotify", "discord", "telegram", "slack", "empire", "jedi", "sith", "mandalorian", "firstorder", "deathstar", "spaceinvaders", "ghost", "pokeball", "pokemon", "gamepad", "retropad", "d20", "playstation", "xbox", "switch", "minecraft", "wizard", "dungeon", "sword", "shield", "crown", "skull", "crossbones", "ninja", "robot", "alien", "knight", "paw", "masks", "theater", "film", "spade", "superpowers", "reddit", "twitch"]
    // id -> glyph codepoint for every option. The brand/OS/dev marks are real
    // SpaceMono Nerd Font glyphs (font-logos, devicons, font-awesome), so nothing
    // is hand-drawn; ryoku stays the CJK 力. The picker and the bar both resolve
    // through here, so a new entry shows up in both.
    readonly property var launcherLogoIconCodes: ({
        "ryoku": 0x529B, "hyprland": 0xF359, "arch": 0xE732, "grid": 0xEEED, "spark": 0xE6A4, "power": 0xF011, "dragon": 0x2EEF, "mark": 0xEE99, "nix": 0xF313, "branch": 0xE666, "rebel": 0xF1D0,
        "ubuntu": 0xF31B, "debian": 0xF306, "fedora": 0xF30A, "gentoo": 0xF30D, "void": 0xF32E, "artix": 0xF31F, "manjaro": 0xF312, "suse": 0xF314, "alpine": 0xF300, "endeavour": 0xF322, "garuda": 0xF337, "cachyos": 0xF385, "freebsd": 0xF30C, "apple": 0xF302, "raspi": 0xF315, "elementary": 0xF309, "gnome": 0xF361, "alma": 0xF31D, "centos": 0xF304, "devuan": 0xF307, "arco": 0xF346, "ferris": 0xF323, "codeberg": 0xF330, "gitea": 0xF339, "tux": 0xF31A, "android": 0xE70E, "mint": 0xF30E, "kali": 0xF327, "popos": 0xF32A, "zorin": 0xF32F, "plasma": 0xF332, "wayland": 0xF367, "docker": 0xF308,
        "github": 0xE709, "git": 0xE702, "gitlab": 0xE7EB, "python": 0xE73C, "rust": 0xE7A8, "go": 0xE724, "node": 0xE719, "react": 0xE7BA, "vue": 0xE8DC, "kube": 0xE81D, "vim": 0xE7C5, "neovim": 0xE83A, "firefox": 0xE745, "chrome": 0xE743, "java": 0xE738, "js": 0xE781, "ts": 0xE8CA, "cpp": 0xE7A3, "ruby": 0xE739, "php": 0xE73D, "swift": 0xE755, "kotlin": 0xE81B, "lua": 0xE826, "haskell": 0xE777, "blender": 0xE766, "figma": 0xE7DA, "redis": 0xE76D, "postgres": 0xE76E, "steam": 0xF1B6, "spotify": 0xF1BC, "discord": 0xF1FF, "telegram": 0xF2C6, "slack": 0xF198,
        "empire": 0xF1D1, "jedi": 0xEECC, "sith": 0xEDDC, "mandalorian": 0xEDD9, "firstorder": 0xF2B0, "deathstar": 0xF08D8, "spaceinvaders": 0xF0BC9, "ghost": 0xEEFE, "pokeball": 0xF041D, "pokemon": 0xF0A09, "gamepad": 0xF11B, "retropad": 0xF0B82, "d20": 0xEEF5, "playstation": 0xED18, "xbox": 0xED3E, "switch": 0xF07E1, "minecraft": 0xF0373, "wizard": 0xEF01, "dungeon": 0xEEFA, "sword": 0xF04E5, "shield": 0xED25, "crown": 0xEDEB, "skull": 0xEE15, "crossbones": 0xEF0E, "ninja": 0xF0774, "robot": 0xEE0D, "alien": 0xF089A, "knight": 0xED63, "paw": 0xF1B0, "masks": 0xF0D02, "theater": 0xEEB6, "film": 0xF008, "spade": 0xF08D1, "superpowers": 0xF2DD, "reddit": 0xF1A1, "twitch": 0xF1E8
    })

    function launcherLogoTextIndex(id) {
        for (var i = 0; i < launcherLogoTextOptions.length; i++)
            if (launcherLogoTextOptions[i] === id) return i
        return 0
    }
    function launcherLogoIconIndex(id) {
        for (var i = 0; i < launcherLogoIconOptions.length; i++)
            if (launcherLogoIconOptions[i] === id) return i
        return 0
    }
    function launcherLogoTextLabel(id) {
        return String(id).toUpperCase()
    }
    // omarchy's U+E900 needs a font Ryoku doesn't ship, so it's absent here and
    // any stale "omarchy" icon id falls back to ryoku via the validators below.
    function launcherLogoIconGlyph(id) {
        var c = launcherLogoIconCodes[id]
        return String.fromCodePoint(c !== undefined ? c : launcherLogoIconCodes["ryoku"])
    }
    function launcherLogoIconFont(id) {
        return id === "ryoku" ? "Noto Sans CJK JP" : mono
    }
    function launcherLogoIconSize(id) {
        if (id === "arch") return 17
        if (id === "dragon") return 16
        if (id === "ryoku") return 15
        return 16
    }
    function launcherLogoIconXOffset(id) {
        if (id === "mark") return 0.5
        if (id === "arch") return 1
        if (id === "grid") return -1
        return 0
    }
    function launcherLogoTextValid(id) {
        return launcherLogoTextIndex(id) >= 0 && launcherLogoTextOptions[launcherLogoTextIndex(id)] === id
    }
    function launcherLogoIconValid(id) {
        return launcherLogoIconIndex(id) >= 0 && launcherLogoIconOptions[launcherLogoIconIndex(id)] === id
    }
    function launcherConfigValue(config, a, b, c) {
        if (!config) return undefined
        if (config[a] !== undefined) return config[a]
        if (b && config[b] !== undefined) return config[b]
        if (c && config[c] !== undefined) return config[c]
        return undefined
    }
    function applyLauncherConfig(config) {
        if (!config) return

        var launcher = config.launcher || config.logo || config
        var mode = launcherConfigValue(launcher, "launcherLogoMode", "logoMode", "mode")
        var text = launcherConfigValue(launcher, "launcherLogoText", "textLogo", "text")
        var icon = launcherConfigValue(launcher, "launcherLogoIcon", "iconLogo", "icon")

        if (mode === "text" || mode === "icon") launcherLogoMode = mode
        if (text !== undefined && launcherLogoTextValid(text)) launcherLogoText = text
        if (icon !== undefined && launcherLogoIconValid(icon)) launcherLogoIcon = icon
    }

    Process {
        id: widgetLoadProc
        command: ["cat", theme.widgetsCachePath]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                var parts = this.text.trim().split(" ")
                if (parts.length >= 4) {
                    theme.modMemory    = parts[0] !== "0"
                    theme.modBrightness = parts[1] !== "0"
                    theme.modClaude    = parts[2] !== "0"
                    theme.modPower     = parts[3] !== "0"
                }
                // parts[4] is the bluetooth flag in the new format, but in the OLD
                // format it was the workspace mode ("10"/"5"/"active") - detect which.
                var wsField = -1
                if (parts.length >= 5) {
                    if (parts[4] === "5" || parts[4] === "active" || parts[4] === "10") {
                        wsField = 4                         // old format: no bluetooth field
                    } else {
                        theme.modBluetooth = parts[4] !== "0"
                        wsField = 5
                    }
                }
                if (wsField >= 0 && parts.length > wsField) {
                    var m = parts[wsField]
                    theme.workspaceMode = (m === "5" || m === "active") ? m : "10"
                    // pickerStyle is the field right after the workspace mode
                    if (parts.length > wsField + 1) {
                        var ps = parts[wsField + 1]
                        if (ps === "hearthstone" || ps === "carousel" || ps === "tanzaku")
                            theme.pickerStyle = ps
                    }
                    // weatherImperial / clock12h follow pickerStyle
                    if (parts.length > wsField + 2) theme.weatherImperial = parts[wsField + 2] === "1"
                    if (parts.length > wsField + 3) theme.clock12h        = parts[wsField + 3] === "1"
                    if (parts.length > wsField + 4) theme.modNetwork      = parts[wsField + 4] === "1"
                    // style tokens - appended after modNetwork, each guarded
                    // +5 is the retired V2 shadow field and is intentionally ignored.
                    // +6..+7 are retired V2 Style fields, retained only so older
                    // cache layouts keep stable offsets.
                    if (parts.length > wsField + 8) {
                        var wss = parts[wsField + 8]
                        if (wss === "numbers" || wss === "magic" || wss === "kanji"
                                || wss === "rings" || wss === "aurora" || wss === "default")
                            theme.workspaceStyle = wss
                    }
                    if (parts.length > wsField + 9) {
                        var bp = parts[wsField + 9]
                        if (bp === "top" || bp === "bottom") theme.barPosition = bp
                    }
                    // +10 is the retired V2 border toggle; structural borders stay on.
                    // +11..+15 widget-group toggles (default ON → only an explicit "0"
                    // hides; old caches lack these fields → groups stay visible)
                    if (parts.length > wsField + 11) theme.modStatus = parts[wsField + 11] !== "0"
                    if (parts.length > wsField + 12) theme.modQuick  = parts[wsField + 12] !== "0"
                    if (parts.length > wsField + 13) theme.modCpu    = parts[wsField + 13] !== "0"
                    if (parts.length > wsField + 14) theme.modVolume = parts[wsField + 14] !== "0"
                    if (parts.length > wsField + 15) theme.modMpris  = parts[wsField + 15] !== "0"
                    // +16 is the retired single-AI-tool field: the pill's provider
                    // set lives in shell.json, not in this positional record.
                    // +17 is the retired transparent/frost field.
                    if (parts.length > wsField + 18) {
                        var lm = parts[wsField + 18]
                        if (lm === "text" || lm === "icon") {
                            theme.launcherLogoMode = lm
                            if (parts.length > wsField + 19 && theme.launcherLogoTextValid(parts[wsField + 19]))
                                theme.launcherLogoText = parts[wsField + 19]
                            if (parts.length > wsField + 20 && theme.launcherLogoIconValid(parts[wsField + 20]))
                                theme.launcherLogoIcon = parts[wsField + 20]
                        } else if (theme.legacyLogoTextValues.indexOf(lm) >= 0) {
                            // Legacy cache field from the first text-logo picker.
                            theme.launcherLogoMode = "text"
                            theme.launcherLogoText = lm
                        }
                    }
                    // +21/+22 retired updater package/clean-theme badges.
                    // +23..+30 are retained only as cache-schema placeholders.
                    // V2 has one compact presentation and deliberately ignores them.
                    // +31 retired updater shell badge.
                    if (parts.length > wsField + 32 && theme.barColorValid(parts[wsField + 32]))
                        theme.barColor = theme.normalizedPaletteId(parts[wsField + 32])
                    if (parts.length > wsField + 33) theme.modGpu = parts[wsField + 33] !== "0"
                    if (parts.length > wsField + 34) theme.modCpuTemperature = parts[wsField + 34] !== "0"
                    if (parts.length > wsField + 35) theme.modStorage = parts[wsField + 35] !== "0"
                    if (parts.length > wsField + 36) theme.barSeps      = theme.parseGidCsv(parts[wsField + 36])
                    if (parts.length > wsField + 37) theme.iconOnlyGids = theme.parseGidCsv(parts[wsField + 37])
                    if (parts.length > wsField + 38 && theme.barTemperatureSourceValid(parts[wsField + 38]))
                        theme.barTemperatureSource = parts[wsField + 38]
                    // +39 was the retired global foreground override. Bar Color
                    // now includes Foreground and per-widget styles provide local overrides.
                    if (parts.length > wsField + 40)
                        theme.widgetColorStyles = theme.parseWidgetColorStyles(parts[wsField + 40])
                    if (parts.length > wsField + 41) {
                        var mbs = parts[wsField + 41]
                        if (mbs === "default" || mbs === "full") theme.mprisBarStyle = mbs
                    }
                    if (parts.length > wsField + 42) {
                        var bss = parts[wsField + 42]
                        if (theme.barShellStyleValid(bss)) theme.barShellStyle = bss
                    }
                    if (parts.length > wsField + 43)
                        theme.barBorderEnabled = parts[wsField + 43] !== "0"
                    if (parts.length > wsField + 44)
                        theme.panelTooltipBorderEnabled = parts[wsField + 44] !== "0"
                    if (parts.length > wsField + 45) {
                        var ba = parseInt(parts[wsField + 45], 10)
                        if (isFinite(ba) && ba >= 0 && ba <= 8) theme.barAnim = ba
                    }
                    if (parts.length > wsField + 46)
                        theme.modLayout = parts[wsField + 46] !== "0"
                }
                theme._widgetsLoaded = true
                theme.applyStudioSettings()
            }
        }
    }

    Process { id: widgetSaveProc }

    // Persist widget visibility to shell.json .qsbar.widgets, the store Bar Studio
    // writes and applyStudioSettings applies on every load. Without this, a
    // control-center toggle only reached the local cache and was overridden by a
    // stale Bar Studio value on the next reload (the widgets reset). shell.json has
    // one writer, the daemon; reach it over the settings.patch seam Weather uses.
    readonly property string _cfgSockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"
    function persistWidgetsToConfig() {
        var cur = Config.qsbar || ({})
        var q = ({})
        for (var k in cur) q[k] = cur[k]
        // Mirror EVERY key applyStudioSettings() applies, from the LIVE
        // properties. Persisting only `widgets` (and copying the rest from the
        // stale Config frame) let applyStudioSettings re-apply an old value over
        // an unpersisted control-center change on the next qsbar change or
        // reload, resetting the user's choice. Keep this set in lockstep with
        // applyStudioSettings below.
        q.barColor = barColor
        q.barAnim = barAnim
        q.barPosition = barPosition
        q.workspaceMode = workspaceMode
        q.workspaceStyle = workspaceStyle
        q.pickerStyle = pickerStyle
        q.launcherLogoMode = launcherLogoMode
        q.aiTools = serializeAiTools()
        q.audioBoost = audioBoost
        q.barTemperatureSource = barTemperatureSource
        q.barShellStyle = barShellStyle
        q.barScale = barScale
        q.barBorderEnabled = barBorderEnabled
        q.panelTooltipBorderEnabled = panelTooltipBorderEnabled
        q.barCornerRadius = barCornerRadius
        q.barShadowEnabled = barShadowEnabled
        q.barFrostEnabled = barFrostEnabled
        q.barAutoHide = barAutoHide
        q.barGapTop = barGapTop
        q.barGapBottom = barGapBottom
        q.barGapLeft = barGapLeft
        q.barGapRight = barGapRight
        q.widgetGeom = widgetGeom
        q.widgetColorStyles = serializeWidgetColorStyles()
        q.barSeps = barSeps
        q.iconOnlyGids = iconOnlyGids
        // Carry the live in-memory layout, never the (possibly stale) Config
        // frame: a widget toggle firing this must not revert a just-made move
        // that Config has not re-read yet.
        if (barLayoutReady)
            q.layout = { version: 1, left: barLayout.left, center: barLayout.center, right: barLayout.right }
        q.widgets = {
            "status": modStatus, "memory": modMemory, "cpu": modCpu, "volume": modVolume,
            "weather": modWeather, "network": modNetwork, "brightness": modBrightness,
            "media": modMedia, "mpris": modMpris, "quick": modQuick, "claude": modClaude,
            "power": modPower, "bluetooth": modBluetooth, "gpu": modGpu,
            "cpuTemperature": modCpuTemperature, "storage": modStorage,
            "battery": modBattery, "layout": modLayout
        }
        _cfgCtl.queued += "call settings.patch " + JSON.stringify({ path: "qsbar", value: q }) + "\n"
        if (_cfgCtl.connected) _cfgCtl.flushQueued()
        else _cfgCtl.connected = true
    }
    Socket {
        id: _cfgCtl
        path: theme._cfgSockPath
        property string queued: ""
        function flushQueued() {
            if (queued.length === 0) return
            write(queued); flush(); queued = ""
        }
        onConnectionStateChanged: if (connected) flushQueued()
    }

    // ── Bar Studio bridge ──
    // Ryoku Hub (Bar Studio) writes a `qsbar` map into shell.json; the Config
    // singleton watches it, so these settings apply live and win over the local
    // widget cache. Only keys the user set are present; the rest keep defaults.
    function applyStudioSettings() {
        var q = Config.qsbar
        if (!q) return
        // Config is the source of truth for these keys, so suppress the widget
        // cache writes the property-change handlers would make (all gated on
        // _widgetsLoaded). Clearing a key in Bar Studio then reverts to the
        // cache/default on the next load instead of sticking.
        var wl = _widgetsLoaded
        _widgetsLoaded = false
        if (q.barColor !== undefined && barColorValid(q.barColor)) barColor = q.barColor
        if (q.barAnim !== undefined) barAnim = q.barAnim
        if (q.barPosition === "top" || q.barPosition === "bottom") barPosition = q.barPosition
        if (q.workspaceMode !== undefined) workspaceMode = q.workspaceMode
        if (q.workspaceStyle  !== undefined) workspaceStyle  = q.workspaceStyle
        if (q.pickerStyle     !== undefined) pickerStyle     = q.pickerStyle
        if (q.launcherLogoMode !== undefined) launcherLogoMode = q.launcherLogoMode
        if (q.aiTools !== undefined) parseAiTools(q.aiTools)
        if (q.audioBoost !== undefined) audioBoost = q.audioBoost === true
        if (q.barTemperatureSource !== undefined && barTemperatureSourceValid(q.barTemperatureSource)) barTemperatureSource = q.barTemperatureSource
        if (q.barShellStyle !== undefined && barShellStyleValid(q.barShellStyle)) barShellStyle = q.barShellStyle
        if (q.barScale !== undefined) barScale = clampBarScale(q.barScale)
        if (q.barBorderEnabled !== undefined) barBorderEnabled = q.barBorderEnabled
        if (q.panelTooltipBorderEnabled !== undefined) panelTooltipBorderEnabled = q.panelTooltipBorderEnabled
        if (q.barCornerRadius !== undefined) barCornerRadius = Math.max(0, Math.min(40, q.barCornerRadius))
        if (q.barShadowEnabled !== undefined) barShadowEnabled = q.barShadowEnabled
        if (q.barFrostEnabled !== undefined) barFrostEnabled = q.barFrostEnabled
        if (q.barAutoHide !== undefined) barAutoHide = q.barAutoHide === true
        if (q.barGapTop !== undefined) barGapTop = clampGap(q.barGapTop)
        if (q.barGapBottom !== undefined) barGapBottom = clampGap(q.barGapBottom)
        if (q.barGapLeft !== undefined) barGapLeft = clampGap(q.barGapLeft)
        if (q.barGapRight !== undefined) barGapRight = clampGap(q.barGapRight)
        if (q.widgetGeom !== undefined && q.widgetGeom !== null) widgetGeom = q.widgetGeom
        if (q.widgetColorStyles !== undefined && q.widgetColorStyles !== null) widgetColorStyles = parseWidgetColorStyles(q.widgetColorStyles)
        if (q.barSeps !== undefined && q.barSeps !== null) barSeps = q.barSeps
        if (q.iconOnlyGids !== undefined && q.iconOnlyGids !== null) iconOnlyGids = q.iconOnlyGids
        var w = q.widgets
        if (w) {
            if (w.status     !== undefined) modStatus     = w.status
            if (w.memory     !== undefined) modMemory     = w.memory
            if (w.cpu        !== undefined) modCpu        = w.cpu
            if (w.volume     !== undefined) modVolume     = w.volume
            if (w.weather    !== undefined) modWeather    = w.weather
            if (w.network    !== undefined) modNetwork    = w.network
            if (w.brightness !== undefined) modBrightness = w.brightness
            if (w.media      !== undefined) modMedia      = w.media
            if (w.mpris      !== undefined) modMpris      = w.mpris
            if (w.quick      !== undefined) modQuick      = w.quick
            if (w.claude     !== undefined) modClaude     = w.claude
            if (w.power      !== undefined) modPower      = w.power
            if (w.bluetooth  !== undefined) modBluetooth  = w.bluetooth
            if (w.gpu            !== undefined) modGpu            = w.gpu
            if (w.cpuTemperature !== undefined) modCpuTemperature = w.cpuTemperature
            if (w.storage        !== undefined) modStorage        = w.storage
            if (w.battery        !== undefined) modBattery        = w.battery
            if (w.layout         !== undefined) modLayout         = w.layout
        }
        // Bar layout (design section 1): apply the stored document, and on the
        // first load without one migrate the retired cache (or write the shipped
        // default) exactly once. A normalised incoming equal to the live layout
        // is our own persist echo, so skip re-applying it.
        if (q.layout && q.layout.left !== undefined) {
            var incoming = {
                version: 1,
                left: q.layout.left || [],
                center: q.layout.center || [],
                right: q.layout.right || []
            }
            if (JSON.stringify(_normalizeLayout(incoming)) !== JSON.stringify(barLayout)) {
                _commitLayout(incoming, false)
                _applyLayoutToBars("")
            } else {
                barLayoutReady = true
                _barLayoutInitialized = true
            }
        } else {
            _migrateBarLayout()
        }
        _widgetsLoaded = wl
    }
    Connections {
        target: Config
        function onQsbarChanged() { theme.applyStudioSettings() }
    }
    Component.onCompleted: theme.applyStudioSettings()

    // ── New widget panel states ──
    property bool networkVisible:   false
    onNetworkVisibleChanged: popupOpened("networkVisible")
    property bool storageVisible:   false
    onStorageVisibleChanged: popupOpened("storageVisible")
    property bool bluetoothVisible: false
    onBluetoothVisibleChanged: popupOpened("bluetoothVisible")
    property bool batteryVisible:   false
    onBatteryVisibleChanged: popupOpened("batteryVisible")
    property bool brightnessVisible: false
    onBrightnessVisibleChanged: popupOpened("brightnessVisible")
    property bool mprisVisible:     false
    onMprisVisibleChanged: popupOpened("mprisVisible")
    property bool workspaceVisible: false
    onWorkspaceVisibleChanged: popupOpened("workspaceVisible")

    // ── Image picker state (theme/wallpaper carousel) ──
    property bool   imagePickerVisible:  false
    onImagePickerVisibleChanged: popupOpened("imagePickerVisible")
    property string imagePickerMode:     "wallpaper"   // "theme" or "wallpaper"
    property real   quickActionsBarX:    0
    // ── Media browser state (screenshots/videos carousel) ──
    property bool   mediaBrowserVisible: false
    onMediaBrowserVisibleChanged: popupOpened("mediaBrowserVisible")
    property string mediaBrowserMode:    "screenshots"  // "screenshots" or "videos"
    // ── Idle inhibitor (Wayland idle-inhibit protocol) ──
    property bool   idleInhibited:       false
    // ── Notification state ──
    property bool notifVisible: false
    onNotifVisibleChanged: popupOpened("notifVisible")
    property int  notifCount:   0
    property real notifBarX:    0

    // ── Power Profile state ──
    property bool powerProfileVisible: false
    onPowerProfileVisibleChanged: popupOpened("powerProfileVisible")
    // The shell daemon owns power-profiles-daemon (ryoku-shell powerprofiles.go)
    // and streams the live pick on the powerprofiles topic; these read that
    // stream. A second poller of powerprofilesctl here would drift from the
    // daemon's banked profile and make a switch look like it did nothing.
    readonly property string powerProfileCurrent: PowerProfiles.profile
    readonly property var powerProfileAvailable: PowerProfiles.available
        ? PowerProfiles.profiles
        : ["power-saver", "balanced", "performance"]

    function gotoWorkspace(id) { Wm.focusWorkspace(id) }

    // Bumped by the ryoku.system-update IPC after `ryoku update` finishes so the
    // clock's UpdateWidget re-polls its status instead of waiting for its cycle.
    property int updateRefreshTick: 0

    // ── Tray state ──
    property bool trayVisible: false
    onTrayVisibleChanged: popupOpened("trayVisible")
    property var trayPinned: []
    property real trayBarX: 10
    property real trayCaretBarX: trayBarX

    // ── slot-aware panel X anchors (center-X of each group; set by BarSlot) ──
    property real volumeBarX:     0
    property real networkBarX:    0
    property real batteryBarX:    0
    property real memoryBarX:     0
    property real cpuBarX:        0
    property real gpuBarX:        0
    property real thermalBarX:    0
    property real storageBarX:    0
    property real aiBarX:         0
    property real workspaceBarX:  0
    property real notifCaretBarX: notifBarX
    property real bluetoothBarX:  0
    property real brightnessBarX: 0
    property real powerBarX:      0
    property real mprisBarX:      0
    property real dashboardBarX:  0
    property real launcherBarX:   6   // ControlPanel follows the Launcher/Control group

    // One connected popover at a time: the active panel publishes the exact
    // bar-widget center used by both the caret and the small border opening.
    readonly property bool anchoredPanelVisible: dashboardVisible || cpuVisible || gpuVisible
        || thermalVisible || aiUsageVisible
        || memVisible || volVisible || controlVisible || networkVisible || bluetoothVisible
        || batteryVisible || brightnessVisible || mprisVisible
        || workspaceVisible || notifVisible || powerProfileVisible || storageVisible
        || trayVisible

    // Preserve the panel surface's actually rendered tip while it closes so
    // the matching bar notch retracts at the same point. Edge panels clamp the
    // tip away from their rounded corner, so the raw widget center is not
    // always the final rendered position.
    property real panelInsetX: 0
    property bool panelInsetReady: false
    function setPanelInsetX(x) {
        if (!anchoredPanelVisible) return

        if (isFinite(x) && x > 0) {
            panelInsetX = x
            panelInsetReady = true
        }
    }
    property real panelInsetReveal: anchoredPanelVisible ? 1 : 0
    onPanelInsetRevealChanged: {
        if (!anchoredPanelVisible && panelInsetReveal <= 0.001)
            panelInsetReady = false
    }
    Behavior on panelInsetReveal {
        NumberAnimation {
            duration: theme.anchoredPanelVisible ? 160 : 120
            easing.type: theme.anchoredPanelVisible ? Easing.OutCubic : Easing.InCubic
        }
    }

    // ── Tray context-menu state (themed menu, rendered by TrayMenu.qml) ──
    property bool trayMenuVisible: false
    onTrayMenuVisibleChanged: popupOpened("trayMenuVisible")

    // ── plugin bar panels ──
    // A bar plugin that ships entryPoints.panel opens it under its glyph in the
    // shared PluginPanel window, one at a time, closing the built-in panels the
    // way they close each other. The opening slot hands over its api (the
    // service instance, settings, dir) so the panel talks to the same service
    // the glyph does. pluginBarX carries each plugin's bar x from the slot
    // anchors, keyed by id, replaced whole so bindings re-derive.
    property string pluginPanelId: ""
    property var pluginPanelApi: null
    property var pluginBarX: ({})
    readonly property bool pluginPanelVisible: pluginPanelId !== ""
    onPluginPanelVisibleChanged: popupOpened("pluginPanelVisible")
    function openPluginPanel(id, api) {
        if (!id) return
        pluginPanelApi = api
        pluginPanelId = id
    }
    function closePluginPanel() { pluginPanelId = "" }
    function togglePluginPanel(id, api) {
        if (pluginPanelId === id) closePluginPanel()
        else openPluginPanel(id, api)
    }
    property string trayMenuService: ""  // service key of the clicked item; menu resolved live from Tray.items
    property real trayMenuX: 0           // global x to anchor the menu under the icon
    property string trayMenuTitle: ""
    property string trayMenuIcon: ""

    function trayDisplayName(item) {
        if (!item) return I18n.tr("Tray App")

        var title = String(item.title || "").trim()
        if (title !== "") return title

        var tooltipTitle = String((item.tooltip && item.tooltip.title) || "").trim()
        if (tooltipTitle !== "") return tooltipTitle

        var fallback = String(item.id || "").trim()
        var slash = fallback.lastIndexOf("/")
        if (slash >= 0 && slash < fallback.length - 1)
            fallback = fallback.substring(slash + 1)
        fallback = fallback.replace(/^org\.(kde|ayatana|freedesktop)\./i, "")
                           .replace(/[_-]+/g, " ")
        return fallback !== "" ? fallback : I18n.tr("Tray App")
    }

    function trayDescription(item, displayName) {
        if (!item) return ""

        var description = String((item.tooltip && item.tooltip.description) || "").trim()
        var tooltipTitle = String((item.tooltip && item.tooltip.title) || "").trim()
        var name = String(displayName || "").trim().toLowerCase()
        if (description !== "" && description.toLowerCase() !== name)
            return description
        if (tooltipTitle !== "" && tooltipTitle.toLowerCase() !== name)
            return tooltipTitle
        return ""
    }

    function openTrayMenu(service, x, title, icon) {
        if (!service) return
        trayMenuService = service
        trayMenuTitle = String(title || "App Menu")
        trayMenuIcon = String(icon || "")
        setPanelAnchor("trayMenu", x)
        trayMenuVisible = true
    }

    function trayIsHidden(item) {
        return trayPinned.indexOf(item.service) < 0
    }

    // toggle: hidden items get pinned (shown in bar); pinned items get unpinned (back to panel)
    function trayToggleHide(item) {
        var key = item.service
        if (!key) return
        var i = trayPinned.indexOf(key)
        if (i >= 0) {
            var a = trayPinned.slice(0, i)
            var b = trayPinned.slice(i + 1)
            trayPinned = a.concat(b)
            trayVisible = false
        } else {
            trayPinned = trayPinned.concat([key])
        }
    }

    Process {
        id: omarchyCurrentRootProbe
        command: ["bash", "-c",
            "state=\"$HOME/.local/state/ryoku/current\"; legacy=\"$HOME/.config/ryoku/current\"; " +
            "if command -v true >/dev/null 2>&1 && [ -d \"$state\" ] && [ -d /usr/share/ryoku ]; then printf '%s\\t%s\\n' \"$state\" /usr/share/ryoku; " +
            "elif [ -d \"$legacy\" ]; then printf '%s\\t%s\\n' \"$legacy\" \"$HOME/.local/share/ryoku\"; " +
            "else printf '%s\\t%s\\n' \"$legacy\" \"$HOME/.local/share/ryoku\"; fi"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                var parts = this.text.trim().split("\t")
                var resolved = parts.length > 0 ? parts[0] : ""
                var installRoot = parts.length > 1 ? parts[1] : ""
                if (resolved) theme.omarchyCurrentRoot = resolved
                if (installRoot) theme.omarchyInstallRoot = installRoot
                theme.omarchyCurrentRootResolved = true
                currentThemeNameWatcher.reload()
                theme.reloadCurrentThemeFiles()
            }
        }
    }

    Timer {
        id: themeReloadDebounce
        interval: 40
        repeat: false
        onTriggered: {
            paletteReader.running = false
            paletteReader.running = true
        }
    }

    // Ryoku writes theme.name immediately after the complete theme directory
    // swap, before its slower application retint commands and final hooks.
    FileView {
        id: currentThemeNameWatcher
        path: theme.themeNamePath
        watchChanges: theme.omarchyCurrentRootResolved
        printErrors: false
        onLoaded: {
            theme.setCurrentThemeName(currentThemeNameWatcher.text())
            theme.reloadCurrentThemeFiles()
        }
        onLoadFailed: theme.setCurrentThemeName("")
        onFileChanged: {
            reload()
            theme.reloadCurrentThemeFiles()
        }
    }

    // Snapshot the compiled palette once, before any daemon write, so the base
    // layer can revert slots the wallpaper/named scheme does not supply.
    function _captureBase() {
        if (theme._basePalette)
            return;
        theme._basePalette = {
            paper: theme.paper.toString(), ink: theme.ink.toString(), sumi: theme.sumi.toString(),
            color01: theme.color01.toString(), color02: theme.color02.toString(),
            color03: theme.color03.toString(), color04: theme.color04.toString(),
            color05: theme.color05.toString(), color06: theme.color06.toString(),
            color07: theme.color07.toString(), accentHint: theme.accentHint.toString()
        };
    }

    function _applyPalette() {
        theme._captureBase();
        Palette.applyResolved(theme, theme._basePalette, theme._namedPalette, theme._wallColors, theme._followWallpaper);
    }

    Process {
        id: paletteReader
        command: ["cat", theme.colorsPath]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                theme._wallColors = Palette.parseAll(this.text);
                theme._applyPalette();
            }
        }
    }

    // retint live on a palette-file change (a wallpaper swap never touches theme.name)
    FileView {
        id: paletteWatcher
        path: theme.colorsPath
        watchChanges: true
        printErrors: false
        onFileChanged: { paletteWatcher.reload(); theme.reloadCurrentThemeFiles() }
    }

    // A fixed named theme publishes its palette here; retint when it changes.
    FileView {
        id: namedPaletteWatcher
        path: theme.shellConfigPath
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: { theme._namedPalette = Palette.parseNamed(namedPaletteWatcher.text()); theme._applyPalette(); }
    }

    // The follow-the-wallpaper master gates whether the live palette applies.
    FileView {
        id: followWallpaperWatcher
        path: theme.themeJsonPath
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: { theme._followWallpaper = Palette.parseFollow(followWallpaperWatcher.text()); theme._applyPalette(); }
    }

    function ipcApplyTheme(payload) {
        let p;
        try { p = JSON.parse(payload); }
        catch (e) { console.warn("theme.apply: bad payload -", e); return; }
        if (!p || !p.colors) return;
        Palette.apply(theme, Palette.mapKeys(p.colors));
        theme.lastAppliedName = p.name || "";
    }

    function ipcApplyLauncher(payload) {
        let p;
        try { p = JSON.parse(payload); }
        catch (e) { console.warn("theme.applyLauncher: bad payload -", e); return; }
        theme.applyLauncherConfig(p);
    }

    function ipcReloadTheme() {
        paletteReader.running = false;
        paletteReader.running = true;
    }

    function ipcOpenPicker(mode) {
        if (mode === "theme" || mode === "wallpaper") openImagePicker(mode)
        else if (mode === "screenshots" || mode === "videos") openMediaBrowser(mode)
    }

}
