pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import stage.services
import inir.services as InirServices
import ".."
import Ryoku.Ui.Singletons as RyokuUi

Singleton {
    id: root

    // ── Desktop entry cache ───────────────────────────────────────────────
    property var _desktopEntryCache: ({})

    function getCachedDesktopEntry(appId) {
        if (!appId) return null
        if (_desktopEntryCache.hasOwnProperty(appId))
            return _desktopEntryCache[appId]
        const entry = InirServices.AppSearch.lookupDesktopEntry(appId, true)
        _desktopEntryCache[appId] = entry ?? null
        return _desktopEntryCache[appId]
    }

    function getCachedIcon(appId) {
        if (!appId) return ""
        return AppSearch.guessIcon(appId)
    }

    function invalidateDesktopEntryCache() {
        _desktopEntryCache = {}
    }

    Connections {
        target: DesktopEntries
        function onApplicationsChanged() { root.invalidateDesktopEntryCache() }
    }
    Connections {
        target: InirServices.AppSearch
        function onListChanged() { root.invalidateDesktopEntryCache() }
    }

    // ── App ID normalization ──────────────────────────────────────────────
    // Strips the .desktop suffix and lowercases for consistent comparisons
    function normalizeAppId(appId) {
        if (!appId) return ""
        const entry = getCachedDesktopEntry(appId)
        return String(entry?.id || appId).toLowerCase().trim().replace(/\.desktop$/i, "")
    }

    // ── Pinned app helpers ────────────────────────────────────────────────
    function isPinned(appId) {
        if (!appId) return false
        const norm = normalizeAppId(appId)
        return Config.options.dock.pinnedApps.some(id => normalizeAppId(id) === norm)
    }

    function togglePin(appId) {
        if (!appId) return
        const norm = normalizeAppId(appId)
        const current = Config.options.dock.pinnedApps ?? []
        Config.options.dock.pinnedApps = isPinned(appId)
            ? current.filter(id => normalizeAppId(id) !== norm)
            : current.concat([appId])
    }

    function reorderPinnedApp(fromAppId, toAppId) {
        if (fromAppId === toAppId) return
        const pinned = Array.from(Config.options.dock.pinnedApps)
        const fromIdx = pinned.indexOf(fromAppId)
        const toIdx = pinned.indexOf(toAppId)
        if (fromIdx === -1 || toIdx === -1) return
        pinned.splice(toIdx, 0, pinned.splice(fromIdx, 1)[0])
        Config.options.dock.pinnedApps = pinned
    }

    // ── Pinned file helpers ───────────────────────────────────────────────
    function _cleanPinnedFilePath(path) {
        let cleanPath = String(path ?? "").trim().replace(/^file:\/\//, "")
        try {
            cleanPath = decodeURIComponent(cleanPath)
        } catch (error) {
            // Keep the original value when an external drag supplies malformed URI data.
        }
        if (cleanPath.length > 1)
            cleanPath = cleanPath.replace(/\/+$/, "")
        return cleanPath
    }

    // Keep the visual file slots in dock.order while changing only which
    // pinned path occupies each slot. This lets Settings reorder folders
    // without disturbing the positions of apps, actions, or widgets.
    function _rebuildPinnedFileSlots(files) {
        const nextFiles = Array.from(files ?? [])
        const order = Array.from(Config.options?.dock?.order ?? [])
        const nextOrder = []
        let fileIndex = 0

        for (const entry of order) {
            if (String(entry).startsWith("file:")) {
                if (fileIndex < nextFiles.length)
                    nextOrder.push("file:" + nextFiles[fileIndex++])
                continue
            }
            nextOrder.push(entry)
        }

        while (fileIndex < nextFiles.length) {
            const fileKey = "file:" + nextFiles[fileIndex++]
            const trashIndex = nextOrder.indexOf("trash")
            if (trashIndex >= 0)
                nextOrder.splice(trashIndex, 0, fileKey)
            else
                nextOrder.push(fileKey)
        }

        Config.options.dock.order = nextOrder
    }

    function addPinnedFile(path) {
        const cleanPath = root._cleanPinnedFilePath(path)
        if (!cleanPath) return
        const current = Config.options?.dock?.pinnedFiles ?? []
        if (current.includes(cleanPath)) return
        const nextFiles = current.concat([cleanPath])
        Config.options.dock.pinnedFiles = nextFiles
        root._rebuildPinnedFileSlots(nextFiles)
    }

    function isPinnedFile(path) {
        const cleanPath = root._cleanPinnedFilePath(path)
        return cleanPath.length > 0 && (Config.options?.dock?.pinnedFiles ?? []).includes(cleanPath)
    }

    function togglePinnedFile(path) {
        if (root.isPinnedFile(path))
            root.removePinnedFile(path)
        else
            root.addPinnedFile(path)
    }

    function removePinnedFile(path) {
        const cleanPath = root._cleanPinnedFilePath(path)
        const current = Config.options?.dock?.pinnedFiles ?? []
        if (!current.includes(cleanPath)) return
        const nextFiles = current.filter(p => p !== cleanPath)
        Config.options.dock.pinnedFiles = nextFiles
        root._rebuildPinnedFileSlots(nextFiles)
    }

    function reorderPinnedFile(fromPath, toPath) {
        const cleanFromPath = root._cleanPinnedFilePath(fromPath)
        const cleanToPath = root._cleanPinnedFilePath(toPath)
        if (!cleanFromPath || !cleanToPath || cleanFromPath === cleanToPath) return
        const files = Array.from(Config.options?.dock?.pinnedFiles ?? [])
        const fromIdx = files.indexOf(cleanFromPath)
        const toIdx = files.indexOf(cleanToPath)
        if (fromIdx === -1 || toIdx === -1) return
        files.splice(toIdx, 0, files.splice(fromIdx, 1)[0])
        Config.options.dock.pinnedFiles = files
        root._rebuildPinnedFileSlots(files)
    }

    function reorderPinnedFileByIndex(fromIndex, toIndex) {
        if (fromIndex === toIndex) return
        const files = Array.from(Config.options?.dock?.pinnedFiles ?? [])
        if (fromIndex < 0 || fromIndex >= files.length || toIndex < 0 || toIndex >= files.length)
            return
        files.splice(toIndex, 0, files.splice(fromIndex, 1)[0])
        Config.options.dock.pinnedFiles = files
        root._rebuildPinnedFileSlots(files)
    }

    // The dock drag gesture edits dock.order directly. Reflect its file-slot
    // order back into pinnedFiles so the Settings list stays truthful.
    function syncPinnedFileOrder() {
        const files = Array.from(Config.options?.dock?.pinnedFiles ?? [])
        const orderedFiles = []
        const order = Config.options?.dock?.order ?? []

        for (const entry of order) {
            if (!String(entry).startsWith("file:")) continue
            const path = String(entry).substring(5)
            if (files.includes(path) && !orderedFiles.includes(path))
                orderedFiles.push(path)
        }
        for (const path of files) {
            if (!orderedFiles.includes(path))
                orderedFiles.push(path)
        }

        if (orderedFiles.length !== files.length || orderedFiles.some((path, index) => path !== files[index]))
            Config.options.dock.pinnedFiles = orderedFiles
    }

    function reorderPinned(fromIndex, toIndex) {
        if (fromIndex === toIndex) return
        var pinned = Array.from(Config.options.dock.pinnedApps)
        if (fromIndex < 0 || fromIndex >= pinned.length || toIndex < 0 || toIndex >= pinned.length) return
        pinned.splice(toIndex, 0, pinned.splice(fromIndex, 1)[0])
        Config.options.dock.pinnedApps = pinned
    }

    function reorderOrder(fromIndex, toIndex) {
        if (fromIndex === toIndex) return
        var order = Array.from(Config.options.dock.order)
        if (fromIndex < 0 || fromIndex >= order.length || toIndex < 0 || toIndex >= order.length) return
        order.splice(toIndex, 0, order.splice(fromIndex, 1)[0])
        Config.options.dock.order = order
        root.syncPinnedFileOrder()
    }

    // Bumped once after DynamicTheme generation completes. Keep color changes independent
    // from icon invalidation so a palette update cannot repeatedly recreate icon textures
    // while the theme is still being built.
    property int iconThemeRevision: 0
    /// Set by IconThemes while it applies a pack: that sequence redraws once, at its end.
    property bool iconPackApplying: false

    // The recolor script writes this hash last, after DynamicTheme has been swapped in and the
    // icon loader told about it. It is watched here rather than in IconThemes: that service
    // only loads with the settings page, so a wallpaper change redrew nothing and icons that
    // stay on screen (the dock) kept the old colours until they were recreated.
    FileView {
        path: Quickshell.env("HOME") + "/.local/share/icons/DynamicTheme.colhash"
        watchChanges: true
        onFileChanged: {
            if (!root.iconPackApplying)
                iconRedrawTimer.restart()
        }
    }

    // A single bounded toggle invalidates the icon bindings without making sourceSize grow
    // after every theme change. The delay lets the loader finish rebuilding the theme first.
    Timer {
        id: iconRedrawTimer
        interval: 250
        onTriggered: root.iconThemeRevision = 1 - root.iconThemeRevision
    }

    // ── XDG user directories ──────────────────────────────────────────────
    property var xdgUserDirs: ({})

    FileView {
        id: xdgDirsFile
        path: Quickshell.env("HOME") + "/.config/user-dirs.dirs"
        blockLoading: true
        onLoaded: {
            const home = Quickshell.env("HOME")
            const keyMap = {
                "XDG_DOWNLOAD_DIR": "downloads",
                "XDG_DOCUMENTS_DIR": "documents",
                "XDG_PICTURES_DIR": "pictures",
                "XDG_MUSIC_DIR": "music",
                "XDG_VIDEOS_DIR": "videos",
                "XDG_DESKTOP_DIR": "desktop",
                "XDG_PUBLICSHARE_DIR": "publicshare",
                "XDG_TEMPLATES_DIR": "templates",
            }
            const result = {}
            for (const line of xdgDirsFile.text().split("\n")) {
                const match = line.match(/^(\w+)="(.+)"$/)
                if (!match) continue
                const key = keyMap[match[1]]
                if (key) result[key] = match[2].replace("$HOME", home)
            }
            root.xdgUserDirs = result
        }
    }

    // ── App model ─────────────────────────────────────────────────────────
    // Merges pinned apps (from config) with running toplevels (from the compositor).
    // Pinned apps without open windows are included; running apps not in the pinned
    // list are appended at the end.
    property var apps: {
        const pinnedMap = new Map()
        const unpinnedMap = new Map()
        const pinnedApps = Config.options?.dock.pinnedApps ?? []

        const ignoredRegexes = (Config.options?.dock.ignoredAppRegexes ?? []).map(pattern => {
            try   { return new RegExp(pattern, "i") }
            catch(e) { return new RegExp("^$") }
        })

        for (const appId of pinnedApps) {
            if (appId) pinnedMap.set(appId, { pinned: true, toplevels: [] })
        }
        const directWindows = new Map()
        function addWindow(appId, toplevel) {
            if (!appId || ignoredRegexes.some(re => re.test(appId))) return
            const normToplevel = normalizeAppId(appId)
            let matchedKey = null
            for (const key of pinnedMap.keys()) {
                if (normalizeAppId(key) === normToplevel) { matchedKey = key; break }
            }
            if (matchedKey === null) {
                for (const key of pinnedMap.keys()) {
                    const startupClass = getCachedDesktopEntry(key)?.startupClass
                    if (startupClass && String(startupClass).toLowerCase() === String(appId).toLowerCase().replace(/\.desktop$/i, "")) {
                        matchedKey = key
                        break
                    }
                }
            }
            if (matchedKey !== null) {
                pinnedMap.get(matchedKey).toplevels.push(toplevel)
            } else {
                if (!unpinnedMap.has(normToplevel))
                    unpinnedMap.set(normToplevel, { appId: appId, pinned: false, toplevels: [] })
                unpinnedMap.get(normToplevel).toplevels.push(toplevel)
            }
        }
        for (const toplevel of ToplevelManager.toplevels.values) {
            if (!toplevel?.appId) continue
            const norm = normalizeAppId(toplevel.appId)
            directWindows.set(norm, (directWindows.get(norm) ?? 0) + 1)
            addWindow(toplevel.appId, toplevel)
        }
        const providerWindows = new Map()
        for (const window of (RyokuUi.Wm.windows ?? [])) {
            if (!window?.appId) continue
            const norm = normalizeAppId(window.appId)
            const count = (providerWindows.get(norm) ?? 0) + 1
            providerWindows.set(norm, count)
            if (count <= (directWindows.get(norm) ?? 0)) continue
            const windowId = window.id
            addWindow(window.appId, window.toplevel ?? {
                appId: window.appId,
                output: window.output,
                activated: RyokuUi.Wm.focusedWindow?.id === windowId,
                activate: () => RyokuUi.Wm.focusWindow(windowId),
                close: () => RyokuUi.Wm.closeWindow(windowId)
            })
        }

        const values = []
        for (const [key, value] of pinnedMap)
            values.push({ appId: key, toplevels: value.toplevels, pinned: true })
        for (const [, value] of unpinnedMap)
            values.push({ appId: value.appId, toplevels: value.toplevels, pinned: false })
        return values
    }
}
