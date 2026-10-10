import QtQuick
import Quickshell
import Ryoku.Ui.Singletons

QtObject {
    id: state

    property LibraryView view
    property CardField field

    property string collection: "wallpapers"
    property SettingValue _displayMode: SettingValue { key: "components.wallpaperSelector.displayMode" }
    readonly property string mode: {
        var v = state._displayMode.value
        return (v === undefined || v === null || v === "") ? "slices" : String(v)
    }

    // The setting decides how each open starts; the keymap toggle flips it until the picker hides.
    property bool filterBarToggled: false
    readonly property bool filterBarShown: state._bool(state._barAlwaysVisible) !== state.filterBarToggled
    property bool searchOpen: false
    property string searchMode: "tags"   // "tags" | "describe"
    property bool helpOpen: false

    property string sheet: ""             // "" when none
    property var sheetArgs: ({})
    // True while an open sheet wants the card scene visible through it (a folio design section).
    property bool sceneThrough: false

    property bool themeBarOpen: false
    property bool riceWorkshopOpen: false
    property var riceEntry: null
    // The rice share dialog: "save" names a capture of this desktop, "export" writes
    // riceShareEntry to a folder, "import" installs a folder someone shared.
    property string riceShare: ""
    property var riceShareEntry: null
    property bool multipickerOpen: false
    property int multipickerRow: -1

    property string monitor: ""           // output name the picker opened on
    function currentWorkspace() {
        const list = Wm.workspaces ?? []
        for (let i = 0; i < list.length; ++i) {
            const workspace = list[i]
            if (workspace.active && (!state.monitor || workspace.output === state.monitor)) {
                return {
                    "id": String(workspace.id ?? ""),
                    "name": String(workspace.name ?? ""),
                    "output": String(workspace.output || state.monitor || "")
                }
            }
        }
        return null
    }

    readonly property string currentWorkspaceName: {
        const workspace = state.currentWorkspace()
        if (!workspace)
            return ""
        return workspace.name || workspace.id
    }
    property bool shown: false

    readonly property string hoverKey: (state.field && state.view && state.field.hoveredIndex >= 0)
        ? (state.view.get(state.field.hoveredIndex).key || "")
        : ""

    signal toastRequested(string message, string kind)
    signal hideRequested()

    property SettingValue _applyOnMonitor: SettingValue { key: "general.applyOnPickerMonitor" }
    property SettingValue _closeOnSelection: SettingValue { key: "general.closeOnSelection" }
    property SettingValue _startPosition: SettingValue { key: "components.wallpaperSelector.startPosition" }
    property SettingValue _lastApplied: SettingValue { key: "components.wallpaperSelector.lastAppliedKey" }
    property SettingValue _livePreview: SettingValue { key: "selector.livePreview" }
    property SettingValue _shellHover: SettingValue { key: "shell.hoverPreview" }
    property SettingValue _barAlwaysVisible: SettingValue { key: "general.filterBarAlwaysVisible" }

    function _bool(sv) { return sv && sv.value === true }

    function setCollection(name) {
        if (state.collection === name)
            return
        state.collection = name
        if (state.view)
            state.view.collection = name
    }

    function openSheet(name, args) {
        state.sheetArgs = args || ({})
        state.sheet = name
    }
    function closeSheet() {
        state.sheet = ""
        state.sheetArgs = ({})
    }

    function openBrowser(provider) {
        state.openSheet("browser", { provider: provider || "" })
    }

    function toast(message, kind) {
        state.toastRequested(message, kind || "info")
    }

    function openRiceShare(mode, entry) {
        state.riceShareEntry = entry || null
        state.riceShare = mode
    }
    function closeRiceShare() {
        state.riceShare = ""
        state.riceShareEntry = null
    }

    function runAction(id, args) {
        Daemon.call("settings.action", { id: id, args: args || ({}) }, function(result, error) {
            if (error)
                state.toast(error.message || I18n.tr("That action could not be completed."), "error")
        })
    }

    function applyRow(row, chooseDisplays) {
        if (!state.view)
            return
        var e = state.view.get(row)
        if (!e || !e.key)
            return
        if (state.collection === "themes") {
            // A theme tile is the fixed-palette choice; the daemon applies it and the masthead's
            // wallpaper-colours toggle reads the same policy, so the two never disagree.
            if (e.key === "Wallpaper") {
                Settings.set("theme.policy", "wallpaper")
            } else {
                Settings.set("theme.staticTheme", e.key)
                Settings.set("theme.policy", "fixed")
            }
            if (state._bool(state._closeOnSelection))
                state.hideRequested()
            return
        }
        if (state.collection === "rices") {
            state.riceEntry = Library.entry("rices", e.key)
            state.riceWorkshopOpen = true
            return
        }
        if (chooseDisplays) {
            state.multipickerRow = row
            state.multipickerOpen = true
            return
        }
        var full = Library.entry(state.collection, e.key)
        var outs = (state._bool(state._applyOnMonitor) && state.monitor) ? [state.monitor] : []
        state._applyWall(full && full.key ? full : e, outs, ({}), ({}))
        if (state._bool(state._closeOnSelection))
            state.hideRequested()
    }

    function applyEntry(entry, outputs, audioMap, volumeMap, target) {
        if (!entry)
            return
        state._applyWall(entry, outputs, audioMap, volumeMap, target)
        state.multipickerOpen = false
        if (state._bool(state._closeOnSelection))
            state.hideRequested()
    }

    function _applyWall(entry, outputs, audioMap, volumeMap, target) {
        var type = entry.type || "static"
        var params = { type: type, outputs: outputs || [] }
        if (type === "we")
            params.we_id = entry.weId || entry.we_id || entry.key
        else
            params.path = entry.path || entry.videoFile || entry.key
        if (target && target.kind === "workspace") {
            params.target = "workspace"
            params.workspace = target.workspace || state.currentWorkspace()
        }
        if (audioMap && Object.keys(audioMap).length > 0)
            params.outputs_audio = audioMap
        if (volumeMap && Object.keys(volumeMap).length > 0)
            params.outputs_volume = volumeMap
        Daemon.call("wall.apply", params, function(result, error) {
            if (error)
                state.toast(error.message || I18n.tr("This wallpaper could not be applied."), "error")
        })
    }

    function toggleFavourite(row) {
        if (!state.view || (state.collection !== "wallpapers" && state.collection !== "workshop"))
            return
        var e = state.view.get(row)
        if (!e || !e.key)
            return
        Daemon.call("wall.set_favourite", { key: e.key, favourite: !e.favourite })
    }

    function closeTopLayer() {
        if (state.sheet !== "") { state.closeSheet(); return true }
        if (state.multipickerOpen) { state.multipickerOpen = false; return true }
        if (state.riceShare !== "") { state.closeRiceShare(); return true }
        if (state.riceWorkshopOpen) { state.riceWorkshopOpen = false; return true }
        if (state.themeBarOpen) { state.themeBarOpen = false; return true }
        if (state.helpOpen) { state.helpOpen = false; return true }
        if (state.searchOpen) { state.searchOpen = false; return true }
        if (state.field && state.field.flippedIndex >= 0) { state.field.unflip(); return true }
        return false
    }
    function goBack() {
        if (!state.closeTopLayer())
            state.hideRequested()
    }

    function applyStartPosition() {
        if (!state.field || !state.view)
            return
        var mode = state._startPosition.value ? String(state._startPosition.value) : "beginning"
        if (mode === "browsing")
            return
        if (mode === "applied") {
            var k = state._lastApplied.value ? String(state._lastApplied.value) : ""
            var i = k ? state.view.indexOfKey(k) : -1
            if (i >= 0) { state.field.currentIndex = i; return }
        }
        state.field.currentIndex = 0
    }

    readonly property bool _hoverActive: state.shown && state.sheet === "" && !state.multipickerOpen
    property Timer _hoverDwell: Timer {
        interval: 50
        onTriggered: state._pushHover()
    }
    onHoverKeyChanged: if (state._hoverActive) state._hoverDwell.restart()

    // Accepts either chrome role names or the desktop's material names.
    function _chromePalette(p) {
        if (!p)
            return null
        function has(k) { return typeof p[k] === "string" && p[k].length > 0 }
        return {
            primary: has("primary") ? p.primary : undefined,
            primaryText: has("primaryText") ? p.primaryText : (has("onPrimary") ? p.onPrimary : undefined),
            surface: has("surface") ? p.surface : undefined,
            surfaceText: has("surfaceText") ? p.surfaceText : (has("onSurface") ? p.onSurface : undefined),
            surfaceVariant: has("surfaceVariant") ? p.surfaceVariant : undefined,
            surfaceContainer: has("surfaceContainer") ? p.surfaceContainer : undefined,
            background: has("background") ? p.background : undefined,
            outline: has("outline") ? p.outline : undefined,
            tertiary: has("tertiary") ? p.tertiary : undefined
        }
    }

    function _pushHover() {
        var key = state.hoverKey
        if (state._bool(state._livePreview)) {
            if (key) {
                Daemon.call("palette.preview", { key: key }, function(result, error) {
                    if (state.hoverKey !== key)
                        return
                    Theme.previewPalette = (!error && result) ? state._chromePalette(result) : null
                })
            } else {
                Theme.previewPalette = null
            }
        }
        if (state._bool(state._shellHover))
            Daemon.call("palette.hover", key ? { key: key } : ({}))
    }

    onShownChanged: {
        if (state.shown) {
            state.filterBarToggled = false
            state.applyStartPosition()
        } else {
            state._hoverDwell.stop()
            if (Theme.previewPalette)
                Theme.previewPalette = null
            if (state._bool(state._shellHover))
                Daemon.call("palette.hover", ({}))
            Settings.flush()
            state.closeSheet()
            state.multipickerOpen = false
            state.helpOpen = false
            state.searchOpen = false
            state.themeBarOpen = false
            state.riceWorkshopOpen = false
            state.closeRiceShare()
        }
    }

    property Connections _daemonConn: Connections {
        target: Daemon
        function onEvent(name, data) {
            if (name === "ryogami.wall.applied") {
                var k = (data && data.key) ? data.key : ""
                if (k)
                    Settings.set("components.wallpaperSelector.lastAppliedKey", k)
            }
        }
    }
}
