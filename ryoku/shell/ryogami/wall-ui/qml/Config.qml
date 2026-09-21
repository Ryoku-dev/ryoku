pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import "services"

QtObject {
    id: config

    readonly property string version: "0.1.0"

    function _resolve(path) { return path ? path.replace("~", homeDir) : "" }

    readonly property string homeDir: Quickshell.env("HOME")
    readonly property string configDir: Quickshell.env("RYOGAMI_WALL_CONFIG")
        || (Quickshell.env("XDG_CONFIG_HOME") || (homeDir + "/.config")) + "/ryogami-wall"
    readonly property string installDir: Quickshell.env("RYOGAMI_WALL_INSTALL")
        || configDir

    property var _data: ({})
    property bool configLoaded: false

    property var _configFile: FileView {
        path: BootstrapService.ready ? (configDir + "/config.json") : ""
        watchChanges: true
        onLoaded: config._reparse()
        onFileChanged: { reload(); config._reparse() }
    }

    function _reparse() {
        var raw = _configFile.text() || ""
        if (!raw) { config._data = {}; config.configLoaded = false; return }
        var parsed
        try { parsed = JSON.parse(raw) }
        catch (e) { config._data = {}; config.configLoaded = false; return }
        config._data = config._migrateBackdropKeys(parsed)
        config.configLoaded = true
    }

    // _migrateBackdropKeys renames the pre-seam niri.* overview-backdrop keys to
    // overviewBackdrop.* once, then writes the file so the rename sticks.
    function _migrateBackdropKeys(data) {
        if (!data || typeof data.niri !== "object" || data.niri === null) return data
        var map = {
            "overviewBackdrop": "enabled",
            "overviewBackdropBlur": "blur",
            "overviewBackdropBlurEnabled": "blurEnabled",
            "backdrop": "path",
            "backdropFollowWallpaper": "followWallpaper",
            "backdropAutoTheme": "autoTheme",
            "backdropTheme": "theme",
            "backdropDim": "dim"
        }
        var ob = (data.overviewBackdrop && typeof data.overviewBackdrop === "object") ? data.overviewBackdrop : {}
        for (var oldKey in map) {
            if (data.niri[oldKey] !== undefined && ob[map[oldKey]] === undefined)
                ob[map[oldKey]] = data.niri[oldKey]
        }
        data.overviewBackdrop = ob
        delete data.niri
        _configWriter.setText(JSON.stringify(data, null, 2) + "\n")
        return data
    }

    function saveKey(path, value) {
        var src = _data && typeof _data === "object" ? _data : {}
        var data
        try { data = JSON.parse(JSON.stringify(src)) } catch(e) { data = {} }
        var parts = path.split(".")
        var obj = data
        for (var i = 0; i < parts.length - 1; i++) {
            if (typeof obj[parts[i]] !== "object" || obj[parts[i]] === null)
                obj[parts[i]] = {}
            obj = obj[parts[i]]
        }
        obj[parts[parts.length - 1]] = value
        config._data = data
        config.configLoaded = true
        _configWriter.setText(JSON.stringify(data, null, 2) + "\n")
    }

    property var _configWriter: FileView {
        path: Config.configDir + "/config.json"
        preload: true
    }

    readonly property string runtimeDir: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryogami-wall"

    readonly property string scriptsDir: _resolve(_data.paths?.scripts) || (installDir + "/scripts")
    readonly property string templateDir: _resolve(_data.paths?.templates) || (installDir + "/data/matugen/templates")
    readonly property string cacheDir: _resolve(_data.paths?.cache)
        || Quickshell.env("RYOGAMI_WALL_CACHE")
        || (Quickshell.env("XDG_CACHE_HOME") || (homeDir + "/.cache")) + "/ryogami-wall"
    readonly property string wallpaperDir: _resolve(_data.paths?.wallpaper)
        || (homeDir + "/Pictures/Wallpapers")
    readonly property string videoDir: _resolve(_data.paths?.videoWallpaper)
        || wallpaperDir
    readonly property string weDir: _resolve(_data.paths?.steamWorkshop)
        || _detectWeDir()
    function _detectWeDir() {
        var steamRoot = _resolve(_data.paths?.steam) || (homeDir + "/.local/share/Steam")
        var candidate = steamRoot + "/steamapps/workshop/content/431960"
        return candidate
    }
    readonly property string weAssetsDir: _resolve(_data.paths?.steamWeAssets)
    readonly property string steamDir: _resolve(_data.paths?.steam)

    readonly property string mainMonitor: _data.monitor ?? ""

    readonly property bool closeOnSelection: _data.general?.closeOnSelection !== false
    readonly property bool filterBarAlwaysVisible: _data.general?.filterBarAlwaysVisible !== false
    readonly property int randomInterval: _data.general?.randomInterval ?? 300
    readonly property bool randomIncludeStatic: _data.general?.randomIncludeStatic !== false
    readonly property bool randomIncludeVideo: _data.general?.randomIncludeVideo !== false
    readonly property bool randomIncludeWE: _data.general?.randomIncludeWE !== false
    readonly property bool randomIncludeFavourites: _data.general?.randomIncludeFavourites !== false
    readonly property bool wallpaperPerMonitor: _data.general?.wallpaperPerMonitor === true
    readonly property int selectorBackdropOpacity: Math.max(0, Math.min(100, _data.general?.selectorBackdropOpacity ?? 0))
    readonly property bool notifyOnWallpaperChange: _data.general?.notifyOnWallpaperChange !== false
    readonly property string notificationsBuiltIn: _data.notifications?.builtIn ?? "never"
    readonly property real uiScale: Math.max(0.5, Math.min(2.0, _data.general?.uiScale ?? 1.0))

    readonly property bool matugenEnabled: _data.features?.matugen !== false
    readonly property bool steamEnabled: _data.features?.steam !== false
    readonly property bool wallhavenEnabled: _data.features?.wallhaven !== false
    readonly property bool videoPreviewEnabled: _data.features?.videoPreview !== false
    readonly property bool videoPreviewInstant: _data.features?.videoPreviewInstant === true
    readonly property bool videoAutoScale: _data.features?.videoAutoScale === true

    readonly property bool transitionEnabled: _data.transition?.enabled !== false
    readonly property string transitionShader: _data.transition?.shader ?? "random"
    readonly property int transitionDurationMs: {
        var v = _data.transition?.durationMs
        if (typeof v !== "number") return 600
        return Math.max(50, Math.min(5000, Math.round(v)))
    }
    // The live fill mode the daemon actually paints: capitalized content_fit in
    // shell.json (Cover/Contain/Fill/ScaleDown/Center/Tile).
    readonly property string contentFit: _shellGet("wallpaper.content_fit", "Cover")
    readonly property bool wallpaperMute: _data.wallpaperMute !== false
    readonly property int wallpaperVolume: {
        var v = _data.wallpaperVolume
        if (typeof v !== "number") return 100
        return Math.max(0, Math.min(100, Math.round(v)))
    }

    // The ryogami daemon reads the wallpaper engine options from shell.json,
    // so the picker reads and patches the same file it does.
    readonly property string ryokuShellPath: (Quickshell.env("XDG_CONFIG_HOME") || (homeDir + "/.config")) + "/ryoku/shell.json"

    property var _shell: ({})

    property var _shellFile: FileView {
        path: Config.ryokuShellPath
        watchChanges: true
        onLoaded: Config._reparseShell()
        onFileChanged: { reload(); Config._reparseShell() }
    }

    function _reparseShell() {
        var raw = _shellFile.text() || ""
        if (!raw) { config._shell = {}; return }
        try { config._shell = JSON.parse(raw) } catch (e) { config._shell = {} }
    }

    // _shellGet walks a dotted path in the parsed shell.json, defaulting to def.
    function _shellGet(dotted, def) {
        var o = config._shell
        var parts = dotted.split(".")
        for (var i = 0; o != null && i < parts.length; i++)
            o = o[parts[i]]
        return (o === undefined || o === null) ? def : o
    }

    // _shellSet patches shell.json (atomic write), preserving every other key.
    function _shellSet(dotted, value) {
        var src = (config._shell && typeof config._shell === "object") ? config._shell : {}
        var data
        try { data = JSON.parse(JSON.stringify(src)) } catch (e) { data = {} }
        var parts = dotted.split(".")
        var obj = data
        for (var i = 0; i < parts.length - 1; i++) {
            if (typeof obj[parts[i]] !== "object" || obj[parts[i]] === null)
                obj[parts[i]] = {}
            obj = obj[parts[i]]
        }
        obj[parts[parts.length - 1]] = value
        config._shell = data
        _shellWriter.setText(JSON.stringify(data, null, 2) + "\n")
    }

    property var _shellWriter: FileView {
        path: Config.ryokuShellPath
        preload: true
    }

    readonly property string engineVideoEngine: config._shellGet("wallpaper.video_engine", "ryogami")
    readonly property bool   engineVideoEnabled: config._shellGet("wallpaper.video_enabled", true) !== false
    readonly property bool   engineVideoTranscode: config._shellGet("wallpaper.video_transcode", false) === true
    readonly property int    engineVideoTransFps: {
        var v = config._shellGet("wallpaper.video_transcode_fps", 24)
        return (typeof v === "number" && v > 0) ? Math.round(v) : 24
    }
    readonly property int    engineVideoTransWidth: {
        var v = config._shellGet("wallpaper.video_transcode_width", 1920)
        return (typeof v === "number" && v > 0) ? Math.round(v) : 1920
    }

    readonly property string videoConvertPreset: _data.performance?.videoConvertPreset ?? "balanced"
    readonly property string videoConvertResolution: _data.performance?.videoConvertResolution ?? "2k"
    readonly property string imageOptimizePreset: _data.performance?.imageOptimizePreset ?? "balanced"
    readonly property string imageOptimizeResolution: _data.performance?.imageOptimizeResolution ?? "2k"

    readonly property bool autoOptimizeImages: _data.performance?.autoOptimizeImages === true
    readonly property bool autoConvertVideos: _data.performance?.autoConvertVideos === true
    readonly property int imageTrashDays: _data.performance?.imageTrashDays ?? 7
    readonly property int videoTrashDays: _data.performance?.videoTrashDays ?? 7
    readonly property bool autoDeleteImageTrash: _data.performance?.autoDeleteImageTrash === true
    readonly property bool autoDeleteVideoTrash: _data.performance?.autoDeleteVideoTrash === true
    readonly property int videoCacheDays: _data.performance?.videoCacheDays ?? 30
    readonly property bool autoCleanVideoCache: _data.performance?.autoCleanVideoCache !== false

    readonly property int maxThumbJobs: _data.performance?.maxThumbJobs ?? 16

    readonly property string colorSource: _data.colorSource ?? "magick"

    readonly property string matugenConfig: cacheDir + "/matugen-config.toml"
    readonly property string defaultMatugenConfig: _resolve(_data.defaultMatugenConfig ?? "~/.config/matugen/config.toml")
    readonly property string externalMatugenCommand: _data.externalMatugenCommand ?? "matugen -c %config% image %path% -t %scheme% -m %mode% --source-color-index %index%"
    // The matugen knobs live in the shell's own store, not in this vendored
    // config: ryoku-hub is their single writer and ryoku-shell retints from that
    // file, so the picker reads them there and falls back to the vendored block
    // only for a box that has never saved one.
    property var _matugenKnobs: ({})
    function _reparseMatugenKnobs() {
        try { config._matugenKnobs = JSON.parse(_matugenKnobsFile.text() || "") || {} }
        catch (e) { config._matugenKnobs = {} }
    }
    property var _matugenKnobsFile: FileView {
        path: (Quickshell.env("XDG_CONFIG_HOME") || (homeDir + "/.config")) + "/ryoku/matugen.json"
        watchChanges: true
        onLoaded: config._reparseMatugenKnobs()
        onFileChanged: { reload(); config._reparseMatugenKnobs() }
    }

    readonly property string matugenScheme: _matugenKnobs.schemeType || (_data.matugen && _data.matugen.schemeType) || "scheme-fidelity"
    readonly property string matugenMode: _matugenKnobs.mode || (_data.matugen && _data.matugen.mode) || "dark"
    // The hub's source index runs 0..4 and the picker offers all five, so the
    // clamp has to allow 4: at 3 a stored 4 displayed as 3.
    readonly property int matugenColorIndex: {
        var v = _matugenKnobs.sourceColorIndex
        if (typeof v !== "number" && _data.matugen && typeof _data.matugen.colorIndex === "number")
            v = _data.matugen.colorIndex
        return (typeof v === "number") ? Math.max(0, Math.min(4, v | 0)) : 0
    }
    readonly property real matugenContrast: {
        var v = _matugenKnobs.contrast
        if (typeof v !== "number" && _data.matugen && typeof _data.matugen.contrast === "number")
            v = _data.matugen.contrast
        return (typeof v === "number") ? Math.max(-1, Math.min(1, v)) : 0
    }

    readonly property var integrations: _data.integrations ?? []
    onIntegrationsChanged: _generateMatugenConfig()

    property var _matugenConfigWriter: FileView { id: matugenConfigWriter }
    function _generateMatugenConfig() {
        if (!matugenEnabled) return
        var ints = integrations
        if (!ints || ints.length === 0) return
        var tDir = templateDir
        var lines = ["[config]", "reload_apps = false", ""]
        for (var i = 0; i < ints.length; i++) {
            var integ = ints[i]
            if (!integ.template) continue
            var inputPath = integ.template.indexOf("/") >= 0
                ? _resolve(integ.template)
                : tDir + "/" + integ.template
            var outputPath = integ.output
                ? (integ.output.indexOf("/") >= 0
                    ? _resolve(integ.output)
                    : cacheDir + "/" + integ.output)
                : ""
            if (!outputPath) continue
            var safe = (integ.name || "integration_" + i).replace(/[^a-zA-Z0-9_-]/g, "_")
            lines.push("[templates." + safe + "]")
            lines.push('input_path = "' + inputPath + '"')
            lines.push('output_path = "' + outputPath + '"')
            lines.push("")
        }
        matugenConfigWriter.path = matugenConfig
        matugenConfigWriter.setText(lines.join("\n"))
        console.log("Config: generated matugen config with", ints.length, "integrations")
    }

    Component.onCompleted: { console.log("Configuration Loaded"); config._loadCaps() }

    property var _components: _data.components ?? {}
    property var _wallpaperSelector: (typeof _components.wallpaperSelector === "object" && _components.wallpaperSelector !== null) ? _components.wallpaperSelector : {}

    readonly property var _screen: Quickshell.screens[0] ?? null
    readonly property int _screenW: _screen ? _screen.width : 1920
    readonly property int _screenH: _screen ? _screen.height : 1080
    readonly property bool _isSmallScreen: _screenW <= 1600

    readonly property int wallpaperSliceHeight: _wallpaperSelector.sliceHeight ?? (_isSmallScreen ? 360 : 520)
    readonly property int wallpaperVisibleCount: _wallpaperSelector.visibleCount ?? (_isSmallScreen ? 8 : 12)
    readonly property int wallpaperExpandedWidth: _wallpaperSelector.expandedWidth ?? (_isSmallScreen ? 600 : 924)
    readonly property int wallpaperSliceWidth: _wallpaperSelector.sliceWidth ?? (_isSmallScreen ? 90 : 135)
    readonly property int wallpaperSliceSpacing: _wallpaperSelector.sliceSpacing ?? -30
    readonly property int wallpaperSkewOffset: _wallpaperSelector.skewOffset ?? (_isSmallScreen ? 25 : 35)
    readonly property bool wallpaperSliceRoundCorners: _wallpaperSelector.roundCorners === true
    readonly property int wallpaperSliceCornerRadius: wallpaperSliceRoundCorners ? (_wallpaperSelector.cornerRadius ?? 16) : 0
    readonly property int wallpaperSliceCornerTL: !wallpaperSliceRoundCorners ? 0 : Math.max(0, (typeof _wallpaperSelector.cornerTL === "number") ? _wallpaperSelector.cornerTL : (_wallpaperSelector.cornerRadius ?? 16))
    readonly property int wallpaperSliceCornerTR: !wallpaperSliceRoundCorners ? 0 : Math.max(0, (typeof _wallpaperSelector.cornerTR === "number") ? _wallpaperSelector.cornerTR : (_wallpaperSelector.cornerRadius ?? 16))
    readonly property int wallpaperSliceCornerBR: !wallpaperSliceRoundCorners ? 0 : Math.max(0, (typeof _wallpaperSelector.cornerBR === "number") ? _wallpaperSelector.cornerBR : (_wallpaperSelector.cornerRadius ?? 16))
    readonly property int wallpaperSliceCornerBL: !wallpaperSliceRoundCorners ? 0 : Math.max(0, (typeof _wallpaperSelector.cornerBL === "number") ? _wallpaperSelector.cornerBL : (_wallpaperSelector.cornerRadius ?? 16))
    readonly property var wallpaperCustomPresets: _wallpaperSelector.customPresets ?? {}

    readonly property string displayMode: _wallpaperSelector.displayMode ?? "slices"
    readonly property int hexRadius: _wallpaperSelector.hexRadius ?? (_isSmallScreen ? 100 : 140)
    readonly property int hexRows: _wallpaperSelector.hexRows ?? 3
    readonly property int hexCols: _wallpaperSelector.hexCols ?? (_isSmallScreen ? 5 : 7)
    readonly property int hexScrollStep: _wallpaperSelector.hexScrollStep ?? 1
    readonly property bool hexArc: _wallpaperSelector.hexArc !== false
    readonly property real hexArcIntensity: _wallpaperSelector.hexArcIntensity ?? 1.2

    readonly property int gridColumns: _wallpaperSelector.gridColumns ?? (_isSmallScreen ? 4 : 6)
    readonly property int gridRows: _wallpaperSelector.gridRows ?? 3
    readonly property int gridThumbWidth: _wallpaperSelector.gridThumbWidth ?? (_isSmallScreen ? 220 : 300)
    readonly property int gridThumbHeight: _wallpaperSelector.gridThumbHeight ?? (_isSmallScreen ? 124 : 169)

    readonly property int mosaicCells: _wallpaperSelector.mosaicCells ?? (_isSmallScreen ? 30 : 48)
    readonly property int mosaicSeed: _wallpaperSelector.mosaicSeed ?? 7
    readonly property int mosaicRelaxation: _wallpaperSelector.mosaicRelaxation ?? 2
    readonly property int mosaicWidth: _wallpaperSelector.mosaicWidth ?? (_isSmallScreen ? 1100 : 1500)
    readonly property int mosaicHeight: _wallpaperSelector.mosaicHeight ?? (_isSmallScreen ? 600 : 800)

    readonly property int wallhavenColumns: _wallpaperSelector.wallhavenColumns ?? (_isSmallScreen ? 4 : 6)
    readonly property int wallhavenRows: _wallpaperSelector.wallhavenRows ?? 3
    readonly property int wallhavenThumbWidth: _wallpaperSelector.wallhavenThumbWidth ?? (_isSmallScreen ? 220 : 300)
    readonly property int wallhavenThumbHeight: _wallpaperSelector.wallhavenThumbHeight ?? (_isSmallScreen ? 124 : 169)
    readonly property string wallhavenApiKey: Quickshell.env("WALLHAVEN_API_KEY") || (_data.wallhaven?.apiKey ?? "")

    readonly property int steamColumns: _wallpaperSelector.steamColumns ?? (_isSmallScreen ? 4 : 6)
    readonly property int steamRows: _wallpaperSelector.steamRows ?? 3
    readonly property int steamThumbWidth: _wallpaperSelector.steamThumbWidth ?? (_isSmallScreen ? 220 : 300)
    readonly property int steamThumbHeight: _wallpaperSelector.steamThumbHeight ?? (_isSmallScreen ? 124 : 169)
    readonly property string steamApiKey: Quickshell.env("STEAM_API_KEY") || (_data.steam?.apiKey ?? "")
    readonly property string steamUsername: _data.steam?.username ?? ""
    readonly property string steamBackend: _data.steam?.backend ?? "steam"

    readonly property bool autoRecolorEnabled: _data.effects?.autoRecolor === true
    readonly property string autoRecolorTheme: _data.effects?.autoTheme ?? "Catppuccin"

    readonly property int weRenderFps: _data.weRender?.fps ?? 30
    readonly property bool weRenderNoFullscreenPause: _data.weRender?.noFullscreenPause !== false
    readonly property bool weRenderFullscreenPauseOnlyActive: _data.weRender?.fullscreenPauseOnlyActive === true
    readonly property bool weRenderNoautomute: _data.weRender?.noautomute !== false
    readonly property bool weRenderNoAudioProcessing: _data.weRender?.noAudioProcessing === true
    readonly property bool weRenderDisableParticles: _data.weRender?.disableParticles === true
    readonly property bool weRenderDisableMouse: _data.weRender?.disableMouse === true
    readonly property bool weRenderDisableParallax: _data.weRender?.disableParallax === true
    readonly property string weRenderScaling: _data.weRender?.scaling ?? "fill"
    readonly property string weRenderClamp: _data.weRender?.clamp ?? "border"

    readonly property var postProcessing: _data.postProcessing ?? []
    readonly property bool postProcessOnRestore: _data.postProcessOnRestore === true
    readonly property string externalWallpaperCommand: _data.externalWallpaperCommand ?? ""
    readonly property bool pickOnlyMode: _data.pickOnlyMode === true
    readonly property bool restoreOnStartup: _data.restoreOnStartup !== false

    readonly property bool isKDE: {
        var desktop = (Quickshell.env("XDG_CURRENT_DESKTOP") || "").toLowerCase()
        return desktop.indexOf("kde") >= 0 || desktop.indexOf("plasma") >= 0
    }

    // Compositor capabilities from the ryogami daemon (which holds the wm seam
    // client), so the picker gates features on what the compositor can do, never
    // on its name.
    property var _caps: ({})
    readonly property bool canOverviewBackdrop: {
        var s = _caps.supports
        return Array.isArray(s) && s.indexOf("nativeOverview") >= 0
    }
    function _loadCaps() {
        DaemonClient.call("wm.caps", {}, function(result, err) {
            if (!err && result) config._caps = result
        })
    }
    property var _capsConn: Connections {
        target: DaemonClient
        function onReadyChanged() { if (DaemonClient.ready) config._loadCaps() }
    }

    readonly property bool overviewBackdropEnabled: _data.overviewBackdrop?.enabled !== false
    readonly property string overviewBackdropPath: _data.overviewBackdrop?.path ?? ""
    readonly property bool overviewBackdropFollowWallpaper: _data.overviewBackdrop?.followWallpaper === true
    readonly property int overviewBackdropDim: Math.max(0, Math.min(100, _data.overviewBackdrop?.dim ?? 0))
    readonly property int overviewBackdropBlur: Math.max(1, Math.min(200, _data.overviewBackdrop?.blur ?? 30))
    readonly property bool overviewBackdropBlurEnabled: _data.overviewBackdrop?.blurEnabled !== false

    readonly property string kdeVideoPlugin: "luisbocanegra.smart.video.wallpaper.reborn"
}
