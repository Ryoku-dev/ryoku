pragma Singleton

import QtQuick
import Qt.labs.folderlistmodel
import Quickshell
import Quickshell.Bluetooth
import Quickshell.Io
import shell.services as Ryoku
import inir
import inir.modules.common
import inir.modules.common.functions
import inir.services

/**
 * The frame's action registry: the `/` shelf and the palette's action mode.
 * The reference's registry drove its own wallpaper, session and theme daemons;
 * every action here instead rides a service the frame already bridges (the
 * window-manager seam, the daemon-backed toggles, the shell's own surfaces),
 * so no entry is a dead row.
 */
Singleton {
    id: root

    readonly property var allActions: _rebuildActions()

    function _rebuildActions(): var {
        const cats = []
        if (Config.options?.globalActions?.enableSystem ?? true)
            cats.push(_systemActions)
        if (Config.options?.globalActions?.enableAppearance ?? true) {
            cats.push(_appearanceActions)
            if (Config.options?.panelFamily === "iris")
                cats.push(_irisActions)
        }
        if (Config.options?.globalActions?.enableTools ?? true)
            cats.push(_toolsActions)
        if (Config.options?.globalActions?.enableMedia ?? true)
            cats.push(_mediaActions)
        if (Config.options?.globalActions?.enableSettings ?? true)
            cats.push(_settingsActions)
        if (Config.options?.globalActions?.enableCustom ?? true)
            cats.push(_userScriptActions)
        return [].concat(...cats)
    }

    // Bridges the registry into the shape LauncherSearch consumes.
    readonly property var searchActions: allActions.map(a => ({
        action: a.id,
        name: a.name,
        description: a.description,
        icon: a.icon,
        category: a.category,
        keywords: a.keywords,
        execute: a.execute
    }))

    readonly property bool irisMusicOn: Boolean(Config.options?.background?.edgeWidgets?.organic?.enable ?? false)

    // Ryoku hosts no Organic Edge widget, so music on the edges moves the frame
    // itself; switching it on makes sure there is a frame to move.
    function toggleIrisMusic(): void {
        if (root.irisMusicOn) {
            Config.setNestedValue("background.edgeWidgets.organic.enable", false)
            return
        }
        Config.setNestedValues({
            "background.edgeWidgets.organic.enable": true,
            "iris.surround.enable": true
        })
    }

    function fuzzyQuery(query: string): list<var> {
        if (!query || query.trim() === "") return allActions
        const q = query.toLowerCase().trim()
        const scored = allActions.map(action => {
            let score = 0
            const name = (action.name ?? "").toLowerCase()
            const desc = (action.description ?? "").toLowerCase()
            const id = (action.id ?? "").toLowerCase()
            const kw = (action.keywords ?? []).join(" ").toLowerCase()
            if (id === q) score += 100
            if (name.startsWith(q)) score += 60
            if (id.startsWith(q)) score += 50
            if (name.includes(q)) score += 30
            if (desc.includes(q)) score += 15
            if (id.includes(q)) score += 20
            if (kw.includes(q)) score += 10
            const words = q.split(/\s+/)
            if (words.length > 1) {
                const combined = `${name} ${desc} ${id} ${kw}`
                const matchCount = words.filter(w => combined.includes(w)).length
                score += matchCount * 8
            }
            return { action, score }
        }).filter(item => item.score > 0)
        scored.sort((a, b) => b.score - a.score)
        return scored.map(item => item.action)
    }

    function runById(actionId: string, args: string): bool {
        const action = allActions.find(a => a.id === actionId)
        if (action) {
            action.execute(args ?? "")
            return true
        }
        return false
    }

    function listByCategory(category: string): list<var> {
        if (!category || category === "all") return allActions
        return allActions.filter(a => a.category === category)
    }

    IpcHandler {
        target: "globalActions"

        function run(actionId: string): string {
            return root.runById(actionId, "") ? "ok" : "error: action not found: " + actionId
        }

        function runWithArgs(actionId: string, args: string): string {
            return root.runById(actionId, args ?? "") ? "ok" : "error: action not found: " + actionId
        }

        function list(category: string): string {
            return root.listByCategory(category ?? "all").map(a => `${a.id}\t${a.category}\t${a.name}`).join("\n")
        }

        function search(query: string): string {
            return root.fuzzyQuery(query ?? "").map(a => `${a.id}\t${a.category}\t${a.name}`).join("\n")
        }
    }

    function _stepVolume(delta: real): void {
        Audio.setSinkVolume(Math.min(1.5, Math.max(0, Audio.value + delta)))
    }

    function _setStyle(styleId: string): void {
        Config.setNestedValues({ "appearance.globalStyle": styleId })
    }

    // ── SYSTEM ──────────────────────────────────────────────────────────
    readonly property var _systemActions: [
        {
            id: "toggle-wifi",
            name: Translation.tr("Toggle WiFi"),
            description: Translation.tr("Enable or disable WiFi"),
            icon: "wifi",
            category: "system",
            keywords: ["network", "wireless", "internet", "wifi"],
            execute: () => { Network.toggleWifi() }
        },
        {
            id: "toggle-bluetooth",
            name: Translation.tr("Toggle Bluetooth"),
            description: Translation.tr("Turn the adapter on or off"),
            icon: "bluetooth",
            category: "system",
            keywords: ["bt", "wireless", "devices"],
            execute: () => BluetoothStatus.toggle()
        },
        {
            id: "toggle-nightlight",
            name: Translation.tr("Toggle Night Light"),
            description: Translation.tr("Toggle the blue light filter"),
            icon: "nightlight",
            category: "system",
            keywords: ["night", "light", "blue", "filter", "warm"],
            execute: () => { Nightlight.toggle() }
        },
        {
            id: "toggle-gamemode",
            name: Translation.tr("Toggle Game Mode"),
            description: Translation.tr("Hold back effects and notifications"),
            icon: "sports_esports",
            category: "system",
            keywords: ["game", "performance", "fps"],
            execute: () => { GameMode.toggle() }
        },
        {
            id: "toggle-dnd",
            name: Translation.tr("Toggle Do Not Disturb"),
            description: Translation.tr("Silence notifications"),
            icon: "do_not_disturb_on",
            category: "system",
            keywords: ["dnd", "silent", "notifications", "quiet", "focus"],
            execute: () => { Notifications.toggleSilent() }
        },
        {
            id: "lock-screen",
            name: Translation.tr("Lock Screen"),
            description: Translation.tr("Lock the screen"),
            icon: "lock",
            category: "system",
            keywords: ["lock", "security", "screen"],
            execute: () => { Quickshell.execDetached(["ryoku-shell", "lock"]) }
        },
        {
            id: "open-session",
            name: Translation.tr("Session Menu"),
            description: Translation.tr("Power off, reboot, logout, suspend"),
            icon: "power_settings_new",
            category: "system",
            keywords: ["power", "shutdown", "reboot", "logout", "suspend", "session"],
            execute: () => { GlobalStates.sessionOpen = true }
        },
        {
            id: "open-settings",
            name: Translation.tr("Open Settings"),
            description: Translation.tr("Open the shell settings panel"),
            icon: "settings",
            category: "system",
            keywords: ["settings", "config", "preferences", "configure"],
            execute: () => { GlobalStates.openSettings() }
        },
        {
            id: "open-network-settings",
            name: Translation.tr("Network Settings"),
            description: Translation.tr("Open the network connection manager"),
            icon: "lan",
            category: "system",
            keywords: ["network", "wifi", "ethernet", "connection", "nm"],
            execute: () => { AppLauncher.launchNetworkSettings(Network.ethernet) }
        },
        {
            id: "open-volume-mixer",
            name: Translation.tr("Volume Mixer"),
            description: Translation.tr("Open the system volume mixer"),
            icon: "tune",
            category: "system",
            keywords: ["audio", "sound", "volume", "mixer", "pavucontrol"],
            execute: () => { AppLauncher.launch("volumeMixer") }
        },
        {
            id: "open-task-manager",
            name: Translation.tr("Task Manager"),
            description: Translation.tr("Open the system monitor"),
            icon: "monitoring",
            category: "system",
            keywords: ["task", "process", "monitor", "cpu", "ram", "htop"],
            execute: () => { AppLauncher.launch("taskManager") }
        },
        {
            id: "open-ryoku-hub",
            name: Translation.tr("Ryoku Hub"),
            description: Translation.tr("Open the desktop's settings center"),
            icon: "dashboard_customize",
            category: "system",
            keywords: ["hub", "system", "settings", "desktop"],
            execute: () => { Quickshell.execDetached(["ryoku-shell", "hub", "open"]) }
        }
    ]

    // ── APPEARANCE ───────────────────────────────────────────────────────
    readonly property var _appearanceActions: [
        {
            id: "dark-mode",
            name: Translation.tr("Dark Mode"),
            description: Translation.tr("Follow the wallpaper palette in dark"),
            icon: "dark_mode",
            category: "appearance",
            keywords: ["dark", "theme", "night"],
            execute: () => { if (Appearance.m3colors.darkmode) Appearance.toggleDarkMode() }
        },
        {
            id: "light-mode",
            name: Translation.tr("Light Mode"),
            description: Translation.tr("Follow the wallpaper palette in light"),
            icon: "light_mode",
            category: "appearance",
            keywords: ["light", "theme", "day"],
            execute: () => { if (!Appearance.m3colors.darkmode) Appearance.toggleDarkMode() }
        },
        {
            id: "change-wallpaper",
            name: Translation.tr("Change Wallpaper"),
            description: Translation.tr("Open the wallpaper picker"),
            icon: "image",
            category: "appearance",
            keywords: ["wallpaper", "background", "paper"],
            execute: () => { Quickshell.execDetached(["ryogami", "wallpaper", "ui"]) }
        },
        {
            id: "style-material",
            name: Translation.tr("Style: Material"),
            description: Translation.tr("The family's Material plates"),
            icon: "palette",
            category: "appearance",
            keywords: ["style", "material", "theme"],
            execute: () => { root._setStyle("material") }
        },
        {
            id: "style-cards",
            name: Translation.tr("Style: Cards"),
            description: Translation.tr("Raised card surfaces"),
            icon: "dashboard",
            category: "appearance",
            keywords: ["style", "cards"],
            execute: () => { root._setStyle("cards") }
        },
        {
            id: "style-aurora",
            name: Translation.tr("Style: Aurora"),
            description: Translation.tr("Glass over the blurred wallpaper"),
            icon: "auto_awesome",
            category: "appearance",
            keywords: ["style", "aurora", "glass"],
            execute: () => { root._setStyle("aurora") }
        },
        {
            id: "style-inir",
            name: Translation.tr("Style: Shima"),
            description: Translation.tr("The family's own look"),
            icon: "lens_blur",
            category: "appearance",
            keywords: ["style", "iris", "theme"],
            execute: () => { root._setStyle("inir") }
        },
        {
            id: "style-angel",
            name: Translation.tr("Style: Angel"),
            description: Translation.tr("Escalonado paper plates"),
            icon: "escalator",
            category: "appearance",
            keywords: ["style", "angel"],
            execute: () => { root._setStyle("angel") }
        },
        {
            id: "style-regalia",
            name: Translation.tr("Style: Regalia"),
            description: Translation.tr("Cyanotype ink and plates"),
            icon: "ink_pen",
            category: "appearance",
            keywords: ["style", "regalia"],
            execute: () => { root._setStyle("regalia") }
        },
        {
            id: "style-zzz",
            name: Translation.tr("Style: ZZZ"),
            description: Translation.tr("Sleepy rounded surfaces"),
            icon: "bedtime",
            category: "appearance",
            keywords: ["style", "zzz", "sleep"],
            execute: () => { root._setStyle("zzz") }
        },
        {
            id: "style-editorial",
            name: Translation.tr("Style: Editorial"),
            description: Translation.tr("Printed-page surfaces"),
            icon: "menu_book",
            category: "appearance",
            keywords: ["style", "editorial", "paper"],
            execute: () => { root._setStyle("editorial") }
        },
        {
            id: "style-cookie",
            name: Translation.tr("Style: Cookie"),
            description: Translation.tr("Soft cookie surfaces"),
            icon: "cookie",
            category: "appearance",
            keywords: ["style", "cookie"],
            execute: () => { root._setStyle("cookie") }
        },
        {
            id: "open-studio",
            name: Translation.tr("Open Studio"),
            description: Translation.tr("The live appearance editor"),
            icon: "tune",
            category: "appearance",
            keywords: ["studio", "appearance", "edit", "style"],
            execute: () => { GlobalStates.irisStudioOpen = true }
        }
    ]

    readonly property var _irisActions: [
        {
            id: "frame-music",
            name: Translation.tr("Frame Music"),
            description: Translation.tr("Toggle music on the Shima frame"),
            icon: "graphic_eq",
            category: "appearance",
            keywords: ["iris", "frame", "chassis", "edge", "music", "wave", "visualizer"],
            isOn: () => root.irisMusicOn,
            execute: () => root.toggleIrisMusic()
        }
    ]

        // ── TOOLS ────────────────────────────────────────────────────────────
    readonly property var _toolsActions: [
        {
            id: "screenshot",
            name: Translation.tr("Screenshot"),
            description: Translation.tr("Open the capture tool"),
            icon: "photo_camera",
            category: "tools",
            keywords: ["screenshot", "capture", "snip", "screen"],
            execute: () => { Quickshell.execDetached(["ryoku-cmd-ryoshot"]) }
        },
        {
            id: "color-picker",
            name: Translation.tr("Color Picker"),
            description: Translation.tr("Pick a color from the screen"),
            icon: "colorize",
            category: "tools",
            keywords: ["color", "picker", "hex"],
            execute: () => { Quickshell.execDetached(["ryoku-cmd-color-picker"]) }
        },
        {
            id: "screen-record",
            name: Translation.tr("Record Screen"),
            description: Translation.tr("Start or stop a screen recording"),
            icon: "fiber_manual_record",
            category: "tools",
            keywords: ["record", "video", "screen"],
            execute: () => {
                if (RecorderStatus.isRecording)
                    Quickshell.execDetached(["ryoku-shell", "record", "stop"])
                else
                    Quickshell.execDetached(["ryoku-shell", "record", "start"])
            }
        },
        {
            id: "open-clipboard",
            name: Translation.tr("Clipboard History"),
            description: Translation.tr("Open the clipboard history"),
            icon: "content_paste",
            category: "tools",
            keywords: ["clipboard", "history", "paste"],
            execute: () => {
                const st = Ryoku.ShellState.forActive()
                if (st) st.clipboardOpen = !st.clipboardOpen
            }
        }
    ]

    // ── MEDIA ────────────────────────────────────────────────────────────
    readonly property var _mediaActions: [
        {
            id: "media-play-pause",
            name: Translation.tr("Play / Pause"),
            description: Translation.tr("Toggle the active player"),
            icon: "play_arrow",
            category: "media",
            keywords: ["play", "pause", "music", "media"],
            execute: () => { MprisController.togglePlaying() }
        },
        {
            id: "media-next",
            name: Translation.tr("Next Track"),
            description: Translation.tr("Skip to the next track"),
            icon: "skip_next",
            category: "media",
            keywords: ["next", "skip", "track"],
            execute: () => { MprisController.next() }
        },
        {
            id: "media-previous",
            name: Translation.tr("Previous Track"),
            description: Translation.tr("Back to the previous track"),
            icon: "skip_previous",
            category: "media",
            keywords: ["previous", "back", "track"],
            execute: () => { MprisController.previous() }
        },
        {
            id: "toggle-mute",
            name: Translation.tr("Mute Audio"),
            description: Translation.tr("Mute or unmute the speakers"),
            icon: "volume_up",
            category: "media",
            keywords: ["mute", "volume", "audio", "sound"],
            execute: () => { Audio.toggleMute() }
        },
        {
            id: "toggle-mic-mute",
            name: Translation.tr("Mute Microphone"),
            description: Translation.tr("Mute or unmute the microphone"),
            icon: "mic",
            category: "media",
            keywords: ["mic", "microphone", "mute"],
            execute: () => { Audio.toggleMicMute() }
        },
        {
            id: "volume-up",
            name: Translation.tr("Volume Up"),
            description: Translation.tr("Raise the speaker volume"),
            icon: "volume_up",
            category: "media",
            keywords: ["volume", "louder", "up"],
            execute: () => { root._stepVolume(Audio.volumeStep / 100) }
        },
        {
            id: "volume-down",
            name: Translation.tr("Volume Down"),
            description: Translation.tr("Lower the speaker volume"),
            icon: "volume_down",
            category: "media",
            keywords: ["volume", "quieter", "down"],
            execute: () => { root._stepVolume(-Audio.volumeStep / 100) }
        },
        {
            id: "brightness-up",
            name: Translation.tr("Brightness Up"),
            description: Translation.tr("Raise the screen brightness"),
            icon: "brightness_high",
            category: "media",
            keywords: ["brightness", "screen", "brighter"],
            execute: () => { Brightness.increaseBrightness() }
        },
        {
            id: "brightness-down",
            name: Translation.tr("Brightness Down"),
            description: Translation.tr("Lower the screen brightness"),
            icon: "brightness_low",
            category: "media",
            keywords: ["brightness", "screen", "dimmer"],
            execute: () => { Brightness.decreaseBrightness() }
        }
    ]

    // ── SETTINGS ─────────────────────────────────────────────────────────
    readonly property var _settingsActions: [
        {
            id: "toggle-bar-autohide",
            name: Translation.tr("Toggle Bar Visibility"),
            description: Translation.tr("Hide or show the shell bar"),
            icon: "expand",
            category: "settings",
            keywords: ["bar", "hide", "autohide", "show"],
            execute: () => { GlobalStates.barOpen = !GlobalStates.barOpen }
        },
        {
            id: "toggle-dock",
            name: Translation.tr("Toggle Dock"),
            description: Translation.tr("Show or hide the Dock"),
            icon: "dock_to_bottom",
            category: "settings",
            keywords: ["dock", "show", "hide"],
            execute: () => { GlobalStates.irisDockShown = !GlobalStates.irisDockShown }
        },
        {
            id: "toggle-animations",
            name: Translation.tr("Reduce Motion"),
            description: Translation.tr("Surfaces appear in place"),
            icon: "animation",
            category: "settings",
            keywords: ["animations", "motion", "reduce"],
            execute: () => {
                Config.setNestedValue("performance.reduceAnimations",
                    !(Config.options?.performance?.reduceAnimations ?? false))
            }
        },
        {
            id: "toggle-low-power",
            name: Translation.tr("Low Power Mode"),
            description: Translation.tr("Drop effects to save power"),
            icon: "battery_saver",
            category: "settings",
            keywords: ["power", "low", "effects", "save"],
            execute: () => {
                Config.setNestedValue("performance.lowPower",
                    !(Config.options?.performance?.lowPower ?? false))
            }
        },
        {
            id: "toggle-overview",
            name: Translation.tr("Toggle Overview"),
            description: Translation.tr("Open the workspace overview"),
            icon: "grid_view",
            category: "settings",
            keywords: ["overview", "workspaces", "expose"],
            execute: () => { CompositorService.toggleOverview() }
        },
        {
            id: "open-sidebar-left",
            name: Translation.tr("Open Focus Panel"),
            description: Translation.tr("The left side panel"),
            icon: "left_panel_open",
            category: "settings",
            keywords: ["sidebar", "focus", "left", "panel"],
            execute: () => { GlobalStates.openSidebarLeft("") }
        },
        {
            id: "open-ask",
            name: Translation.tr("Ask Rashin"),
            description: Translation.tr("Quick ask and chat"),
            icon: "auto_awesome",
            category: "settings",
            keywords: ["ask", "chat", "Rashin", "web", "tools"],
            execute: () => { Quickshell.execDetached(["ryoku-shell", "ask"]) }
        },
        {
            id: "zoom-in",
            name: Translation.tr("Zoom In"),
            description: Translation.tr("Magnify the screen"),
            icon: "zoom_in",
            category: "settings",
            keywords: ["zoom", "magnify", "accessibility"],
            execute: () => { GlobalStates.screenZoom = Math.min(GlobalStates.screenZoom + 0.4, 3.0) }
        },
        {
            id: "zoom-out",
            name: Translation.tr("Zoom Out"),
            description: Translation.tr("Reduce screen magnification"),
            icon: "zoom_out",
            category: "settings",
            keywords: ["zoom", "magnify", "accessibility"],
            execute: () => { GlobalStates.screenZoom = Math.max(GlobalStates.screenZoom - 0.4, 1.0) }
        }
    ]

    // ── User Script Provider ────────────────────────────────────────────
    property var _userScriptActions: {
        const actions = []
        for (let i = 0; i < userActionsFolder.count; i++) {
            const fileName = userActionsFolder.get(i, "fileName")
            const filePath = userActionsFolder.get(i, "filePath")
            if (fileName && filePath) {
                const actionName = fileName.replace(/\.[^/.]+$/, "")
                const resolvedPath = FileUtils.trimFileProtocol(filePath.toString())
                actions.push({
                    id: `custom-${actionName}`,
                    name: actionName,
                    description: Translation.tr("User script: %1").arg(fileName),
                    icon: "code",
                    category: "custom",
                    keywords: ["custom", "script", "user", actionName],
                    execute: ((path, label) => (args) => {
                        ShellExec.execDetachedArgs([path, ...(args ? args.split(" ") : [])], `Run ${label}`)
                    })(resolvedPath, actionName)
                })
            }
        }
        return actions
    }

    FolderListModel {
        id: userActionsFolder
        folder: Qt.resolvedUrl(Directories.userActions)
        showDirs: false
        showHidden: false
        sortField: FolderListModel.Name
    }

    
}
