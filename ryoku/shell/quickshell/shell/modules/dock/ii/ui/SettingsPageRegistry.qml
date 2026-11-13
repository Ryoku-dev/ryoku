pragma Singleton
import QtQuick
import Quickshell

/**
 * Single source of truth for the settings window's pages.
 *
 * Pages are addressed by stable string `id` — never by array index.
 * `name` holds the untranslated string key; display code (SettingsWindow,
 * Sidebar) applies Translation.tr so language switches stay reactive.
 *
 * Fields per page:
 *  - id:         stable identifier used by deep links and navigation helpers
 *  - name:       untranslated display name key
 *  - icon:       Material Symbol name
 *  - component:  page file, relative to the ii config root
 *  - subPages:   widget config files opened from this page, relative to
 *                modules/settings/configs/ (used for search indexing)
 *  - searchSubPages: { searchSource: subPage } for a search source that indexes
 *                a sub-page's options, so its results open that sub-page
 *  - aliases:    extra untranslated search terms (old page names etc.)
 *  - hidden:     not shown in the sidebar, excluded from keyboard cycling.
 *                Hidden pages must stay at the END of the list.
 *  - families:   panel families this page applies to. Absent means every family.
 *                A restricted family (tablet) does not render some surfaces at all,
 *                and a settings page for a surface that cannot exist is worse than
 *                no page: every switch on it is inert. Filtering happens in the
 *                sidebar and in keyboard cycling; the flat `pages` array keeps every
 *                entry so page indices stay stable across families.
 *  - searchable: set false to skip the file during search indexing
 */
Singleton {
    // Group 1 – Look & Feel
    // Group 2 – Modules
    // Group 3 – Desktop & Windows
    // Group 4 – Tools
    // Group 5 – System & Services
    // Hidden pages — keep at the end of the list

    id: root

    readonly property var pages: [
        {
            "id": "colors",
            "name": "Colors & Themes",
            "icon": "palette",
            "component": "modules/settings/configs/ColorsThemesConfig.qml",
            "subPages": ["widgets/OpenRGBConfig.qml", "widgets/WallpaperEngineConfig.qml"],
            "searchSources": ["sections/ColorsPreviewSection.qml", "sections/ColorsSchedulingSection.qml", "sections/ColorsWallpaperThemingSection.qml", "sections/ColorsWallpaperVariantsSection.qml", "sections/ColorsPickerSection.qml"],
            "aliases": []
        },
        {
            "id": "bar",
            "name": "Bar",
            "icon": "space_bar",
            "component": "modules/settings/configs/BarConfig.qml",
            "searchComponent": false,
            "searchSources": ["sections/BarPlacementSection.qml", "sections/BarAppearanceSection.qml"],
            "subPages": ["widgets/BarAppearanceConfig.qml", "widgets/BarLayoutConfig.qml", "widgets/BarWidgetsWaffleConfig.qml", "widgets/ActiveWindowConfig.qml", "widgets/SearchBarWidgetConfig.qml", "widgets/DateBarWidgetConfig.qml", "widgets/ClockBarWidgetConfig.qml", "widgets/WeatherBarWidgetConfig.qml", "widgets/MediaPlayerConfig.qml", "widgets/UtilButtonsConfig.qml", "widgets/KeyboardLayoutConfig.qml", "widgets/SystemMonitorConfig.qml", "widgets/AiPlanUsageConfig.qml", "widgets/PortWatcherConfig.qml", "widgets/NetworkSpeedConfig.qml", "widgets/PrivacyPillConfig.qml", "widgets/IndicatorsConfig.qml", "widgets/RecordIndicatorConfig.qml", "widgets/SportsConfig.qml", "widgets/BluetoothConfig.qml", "widgets/SystemTrayConfig.qml", "widgets/BatteryConfig.qml", "widgets/DashboardButtonConfig.qml", "widgets/ClockDateWidgetConfig.qml", "widgets/WaffleTweaksConfig.qml", "widgets/DockToPanelConfig.qml", "widgets/BarScrollActionsConfig.qml", "widgets/BarTooltipsConfig.qml", "widgets/BarPopupsConfig.qml"],
            "aliases": ["Bar & Status Bar", "Status Bar", "Shell mode", "Waffle", "Bar appearance", "Bar layout", "Bar style", "Brand icon", "Bar popups", "Floating popups", "Click bar to open sidebars"]
        },
        {
            "id": "wallpaper",
            "name": "Background",
            "icon": "wallpaper",
            "component": "modules/settings/configs/BackgroundConfig.qml",
            "subPages": ["widgets/ParallaxConfig.qml", "widgets/MediaModeBackgroundConfig.qml", "widgets/DepthEffectConfig.qml", "widgets/VideoWallpaperConfig.qml"],
            "searchComponent": false,
            "searchSources": ["sections/BackgroundOptionsSection.qml", "sections/ContextMenusOptionsSection.qml"],
            // The context menus page is a sub-page here; its options open it.
            "searchSubPages": { "sections/ContextMenusOptionsSection.qml": "widgets/ContextMenusConfig.qml" },
            "aliases": ["Wallpaper", "Backgrounds", "Wallpaper Engine", "Context menus", "Right-click menu", "Compact menu", "Custom commands"]
        },
        {
            "id": "interfaceFonts",
            "name": "Interface & Fonts",
            "icon": "font_download",
            "component": "modules/settings/configs/InterfaceFontsConfig.qml",
            "subPages": ["widgets/CustomFontsConfig.qml"],
            "searchComponent": false,
            "searchSources": ["sections/InterfaceFontsOptionsSection.qml"],
            "aliases": ["Base Icon Themes", "Decorative Options", "Shell family", "Panel family", "Tablet mode", "Waffle", "illogical-impulse", "Switch shell"]
        },
        {
            "id": "presets",
            "name": "Presets",
            "icon": "auto_awesome",
            "component": "modules/settings/configs/PresetsConfig.qml",
            "subPages": [
                "presets/PublishSubPage.qml",
                "presets/PresetDetailSubPage.qml",
                "presets/PushUpdateSubPage.qml",
                "presets/PresetScreenshotsSubPage.qml",
                "presets/PresetDiffSubPage.qml"
            ],
            "aliases": []
        },
        {
            "id": "tablet",
            "name": "Tablet",
            "icon": "tablet_android",
            "component": "modules/settings/configs/TabletConfig.qml",
            // Only for the family it configures. Everything on it is inert elsewhere.
            "families": ["tablet"],
            "subPages": [],
            "aliases": ["Shade", "Home screen", "Dock", "App drawer", "Gestures", "Android"]
        },
        {
            "id": "sidebars",
            "name": "Sidebars",
            "icon": "side_navigation",
            "component": "modules/settings/configs/SidebarsConfig.qml",
            "subPages": ["widgets/SidebarQuickTogglesConfig.qml", "widgets/ScreenCornersConfig.qml"],
            "searchComponent": false,
            "searchSources": ["sections/SidebarsOptionsSection.qml"],
            "aliases": ["Sidebars & Panels", "Panels"]
        },
        {
            "id": "dock",
            // The tablet family draws no ii dock; an Android-style dock replaces it.
            "families": ["ii", "waffle"],
            "name": "Dock",
            "icon": "dock_to_bottom",
            "component": "modules/settings/configs/DockConfig.qml",
            "subPages": [
                "widgets/DockUtilitiesConfig.qml",
                "widgets/DockOverviewButtonConfig.qml",
                "widgets/DockAppearanceConfig.qml",
                "widgets/DockLivePreviewConfig.qml",
                "widgets/DockMagnificationConfig.qml"
            ],
            "searchSources": ["sections/DockLookSection.qml", "sections/DockWidgetsSection.qml"],
            "aliases": ["Dock Content", "Dock Widgets", "Widgets & buttons", "Utility widgets", "Dock presets", "Dock folders", "Stopwatch", "Pomodoro", "Shelf", "Color picker", "Dock Appearance", "Taskbar", "Pinned apps", "Magnification", "Smart grouping"]
        },
        {
            "id": "workspaces",
            "name": "Workspaces",
            "icon": "workspaces",
            "component": "modules/settings/configs/WorkspacesConfig.qml",
            "subPages": ["widgets/DockWorkspaceConfig.qml"],
            "searchComponent": false,
            "searchSources": ["sections/WorkspacesOptionsSection.qml"],
            "aliases": ["Tint workspaces icons", "Active indicator", "Numerals", "Workspace compactor"]
        },
        {
            "id": "overview",
            "name": "Overview",
            "icon": "grid_view",
            "component": "modules/settings/configs/OverviewConfig.qml",
            "subPages": [],
            "aliases": ["Overview Screen"]
        },
        {
            "id": "widgets",
            "name": "Desktop Widgets",
            "icon": "widgets",
            "component": "modules/settings/configs/WidgetsConfig.qml",
            "subPages": ["widgets/DesktopClockWidgetConfig.qml", "widgets/DesktopWeatherWidgetConfig.qml", "widgets/DesktopMediaWidgetConfig.qml", "widgets/DesktopAtAGlanceConfig.qml"],
            "aliases": []
        },
        {
            "id": "dynamicIsland",
            // No notch on a fixed tablet status bar.
            "families": ["ii", "waffle"],
            "name": "Dynamic Island",
            "icon": "water_drop",
            "component": "modules/settings/configs/DynamicIslandConfig.qml",
            "subPages": [
                "widgets/DynamicIslandActivitiesConfig.qml",
                "features/TeleprompterConfig.qml"
            ],
            "searchSources": ["sections/DynamicIslandModeSection.qml", "sections/DynamicIslandActivitiesSection.qml"],
            "aliases": ["Notch", "Floating notch", "Activity notches", "Island activities", "Earbuds battery", "Weather", "Dynamic Island in bar center"]
        },
        {
            "id": "overlays",
            "name": "Overlays & OSD",
            "icon": "picture_in_picture",
            "component": "modules/settings/configs/OverlaysConfig.qml",
            "subPages": [
                "widgets/GameOverlayConfig.qml",
                "widgets/PerfMonitorConfig.qml",
                "widgets/OnScreenKeyboardConfig.qml",
                "widgets/OsdIndicatorsConfig.qml"
            ],
            "aliases": ["System Overlays", "Media overlay", "Game overlay", "Performance HUD", "FPS counter", "MangoHud", "RivaTuner"]
        },
        {
            "id": "modes",
            // Desktop automation; the tablet family loads no modes overlay.
            "families": ["ii", "waffle"],
            "name": "Modes & Routines",
            "icon": "tune",
            "component": "modules/settings/configs/ModesConfig.qml",
            "subPages": [],
            "aliases": ["Modes", "Routines", "Automation", "Game detection", "Focus mode", "Do not disturb", "Gaming mode", "Activity log"]
        },
        {
            "id": "screenCapture",
            "name": "Screenshots & Recording",
            "icon": "screenshot_region",
            "component": "modules/settings/configs/ScreenCaptureConfig.qml",
            "subPages": ["widgets/ScreenRecordingConfig.qml", "widgets/ScreenCaptureLensConfig.qml"],
            "aliases": ["Region Selector", "Screenshot", "Screen recording", "Google Lens", "wf-recorder", "OBS", "Circle to Search"]
        },
        {
            "id": "notifications",
            "name": "Notifications",
            "icon": "notifications",
            "component": "modules/settings/configs/NotificationsConfig.qml",
            "subPages": [],
            "aliases": []
        },
        {
            "id": "launcher",
            "name": "Launcher",
            "icon": "search",
            "component": "modules/settings/configs/LauncherConfig.qml",
            "subPages": [
                "widgets/LauncherSearchMatchingConfig.qml",
                "widgets/LauncherResultsConfig.qml",
                "ClipboardConfig.qml",
                "widgets/LauncherSuggestionsConfig.qml",
                "widgets/LauncherPrefixesConfig.qml",
                "widgets/LauncherAliasesConfig.qml",
                "widgets/LauncherModulesConfig.qml",
                "widgets/LauncherQuicklinksConfig.qml",
                "widgets/LauncherSnippetsConfig.qml",
                "widgets/LauncherShortcutsConfig.qml",
                "widgets/LauncherTypeToSearchConfig.qml",
                "widgets/LauncherAppearanceConfig.qml",
                "widgets/LauncherDataConfig.qml"
            ],
            "aliases": ["App Search", "Search Prefixes", "App Aliases", "Search Modules", "Quicklinks", "Search Shortcuts", "Type to search", "Type to run", "KRunner", "Launcher Privacy", "Search matching", "Typo tolerance", "Fuzzy matching", "Best match", "Result priority", "Clipboard", "Clipboard History", "Content detectors"]
        },
        {
            "id": "cheatSheet",
            // A keyboard-shortcut reference, on a device with no keyboard.
            "families": ["ii", "waffle"],
            "name": "Cheat Sheet",
            "icon": "help",
            "component": "modules/settings/configs/CheatSheetConfig.qml",
            "subPages": [],
            "searchComponent": false,
            "searchSources": ["sections/CheatSheetOptionsSection.qml"],
            "aliases": ["Shortcuts", "Keybinds", "Timetable", "Gmail", "Amino acids", "Commands reference", "Periodic table"]
        },
        {
            "id": "windows",
            "name": "Windows",
            "icon": "rule",
            "component": "modules/settings/configs/WindowsConfig.qml",
            "subPages": ["widgets/WindowsBlurConfig.qml"],
            "aliases": ["Hyprland Rules", "Transparency", "Blur", "Advanced blur", "Gaps", "Borders", "Animations", "Alt+Tab", "Window switcher", "Task switcher"]
        },
        {
            "id": "tiling",
            // Dragging windows into a tiling grid needs a pointer.
            "families": ["ii", "waffle"],
            "name": "Window Tiling",
            "icon": "view_quilt",
            "component": "modules/settings/configs/TilingConfig.qml",
            "subPages": [],
            "aliases": ["Tiling assistant", "Snap", "Zones", "Quick tile", "Window snapping", "Tiling popup"]
        },
        {
            "id": "displays",
            "name": "Displays",
            "icon": "monitor",
            "component": "modules/settings/configs/DisplaysConfig.qml",
            "subPages": [],
            "aliases": ["Monitors", "hyprmon", "Resolution", "Refresh rate", "Scale", "OLED Saver", "Blackout", "Always On Display", "AOD"]
        },
        {
            "id": "hyprland",
            "name": "Hyprland",
            "icon": "instant_mix",
            "component": "modules/settings/configs/HyprlandConfig.qml",
            "subPages": [],
            "searchSources": ["hyprland/InputTab.qml", "hyprland/LayoutTab.qml", "hyprland/ShortcutsTab.qml", "hyprland/DefaultAppsTab.qml", "hyprland/RulesTab.qml", "hyprland/EnvironmentTab.qml", "hyprland/AllOptionsTab.qml"],
            "aliases": ["Compositor", "Keyboard layout", "Key repeat", "Mouse", "Touchpad", "Cursor", "Dwindle", "Master layout", "Keybinds", "Default apps", "Default applications", "XDG", "MIME types", "File associations", "Window rules", "Layer rules", "Environment variables", "hyprland.conf", "custom lua"]
        },
        {
            "id": "touchGestures",
            "name": "Touch & Gestures",
            "icon": "touch_app",
            "component": "modules/settings/configs/TouchGesturesConfig.qml",
            "subPages": [
                "widgets/TouchpadGesturesConfig.qml",
                "widgets/TouchEdgeGesturesConfig.qml",
                "widgets/TouchSensitivityConfig.qml"
            ],
            "aliases": ["Touchscreen", "Touch", "Swipe", "Gestures", "Edge gestures", "Tablet", "Calibration", "Touchpad", "Touchpad gestures", "Pinch", "Three finger swipe", "Four finger swipe"]
        },
        {
            "id": "mediaMusic",
            "name": "Media & Music",
            "icon": "album",
            "component": "modules/settings/configs/MediaMusicConfig.qml",
            "subPages": ["widgets/MusicRecognitionConfig.qml", "widgets/LyricsConfig.qml", "widgets/MediaDownloaderConfig.qml"],
            "aliases": ["Core Services", "Media Integrations", "Media Downloader", "Music", "Lyrics", "yt-dlp"]
        },
        {
            "id": "languageTime",
            "name": "Language & Time",
            "icon": "translate",
            "component": "modules/settings/configs/LanguageTimeConfig.qml",
            "subPages": ["widgets/TimeDateFormatsConfig.qml"],
            "aliases": ["Core Services", "Language & Translation", "Time & Date", "World Clocks", "Alarms", "Translator", "Date format", "Clock format", "Custom format strings"]
        },
        {
            "id": "weather",
            "name": "Weather",
            "icon": "cloud",
            "component": "modules/settings/configs/WeatherConfig.qml",
            "subPages": [],
            "aliases": ["Core Services", "Weather Service"]
        },
        {
            "id": "aiAssistant",
            "name": "AI Assistant",
            "icon": "neurology",
            "component": "modules/settings/configs/AiAssistantConfig.qml",
            "subPages": [
                "ai/AiContextMemoryConfig.qml",
                "ai/AiConversationAppearanceConfig.qml",
                "ai/AiNotificationsConfig.qml",
                "ai/AiUsageCostConfig.qml",
                "ai/AiPromptPrivacyConfig.qml",
                "ai/AiModelsKeysConfig.qml",
                "ai/AiToolsPermissionsConfig.qml",
                "ai/AiFilesVisionVoiceConfig.qml",
                "ai/RagConfig.qml",
                "ai/AiRequestLimitsConfig.qml",
                "ai/AiRemoteAccessConfig.qml",
                "ai/AdvancedAiConfig.qml",
                "ai/CustomModelsConfig.qml"
            ],
            "aliases": ["Core Services", "Gemini", "AI", "System prompt", "Tokens", "Usage", "Context", "Memory", "Formatting", "Thinking", "Cost", "Privacy"]
        },
        {
            "id": "tasksAccounts",
            "name": "Accounts & Backup",
            "icon": "checklist",
            "component": "modules/settings/configs/TasksAccountsConfig.qml",
            "subPages": ["widgets/GoogleDriveBackupConfig.qml", "widgets/AdvancedDriveConfig.qml", "widgets/CoreGoogleTasksConfig.qml", "widgets/CoreTickTickConfig.qml", "widgets/ShellBackupConfig.qml"],
            "aliases": ["Core Services", "Tasks & Accounts", "TickTick", "Google Tasks", "Google Tasks API", "Tasks", "Accounts", "Google Drive", "Backup", "Cloud backup", "rclone", "Settings backup", "Restore", "Restore settings", "Export settings", "Import settings", "Zip"]
        },
        {
            "id": "soundAlerts",
            "name": "Sound & Alerts",
            "icon": "volume_up",
            "component": "modules/settings/configs/SoundAlertsConfig.qml",
            "subPages": ["widgets/SoundEventsConfig.qml", "widgets/CustomSoundsConfig.qml", "widgets/AlarmAudioConfig.qml", "widgets/AudioProtectionConfig.qml"],
            "aliases": ["Core Services", "Audio Controls", "Earbang protection", "Interactive Alerts", "Battery sound", "Pomodoro sound"]
        },
        {
            "id": "power",
            "name": "Power & Battery",
            "icon": "battery_android_full",
            "component": "modules/settings/configs/PowerConfig.qml",
            "subPages": [],
            "aliases": ["Core Services", "Suspend", "Battery warning", "Automatic suspend",
                "Charge limit", "Charge threshold", "Battery full notification", "Battery health"]
        },
        {
            "id": "usageStats",
            "name": "App Usage",
            "icon": "bar_chart",
            "component": "modules/settings/configs/UsageStatsConfig.qml",
            "subPages": [],
            "aliases": ["Usage stats", "Screen time", "App usage", "Digital wellbeing", "Energy per app", "RAPL", "History retention", "Sampler"]
        },
        {
            "id": "network",
            "name": "Network",
            "icon": "wifi",
            "component": "modules/settings/configs/NetworkConfig.qml",
            "subPages": [],
            "searchSources": ["network/WifiTab.qml", "network/BluetoothTab.qml", "network/HotspotTab.qml",
                "network/WiredTab.qml"],
            "aliases": ["Wi-Fi", "WiFi", "Wireless", "Bluetooth", "Pairing", "Hotspot", "Tethering", "Access point", "Ethernet", "Wired", "802.1X", "Enterprise Wi-Fi", "Hidden network", "NetworkManager", "IP address", "DNS"]
        },
        {
            "id": "devicesPhone",
            "name": "Devices & Phone",
            "icon": "smartphone",
            "component": "modules/settings/configs/DevicesPhoneConfig.qml",
            "subPages": [],
            "searchSources": ["sections/PhoneBluetoothImagesSection.qml"],
            "aliases": ["Core Services", "Bluetooth Device Images", "LocalSend", "Phone", "KDE Connect", "File Sharing", "Share files"]
        },
        {
            "id": "privacy",
            "name": "Privacy & Content",
            "icon": "policy",
            "component": "modules/settings/configs/PrivacyConfig.qml",
            "subPages": ["widgets/VPNConfig.qml", "widgets/TailscaleConfig.qml"],
            "aliases": ["Core Services", "Work Safety", "Hide clipboard images", "Hide suspect wallpapers", "Hiding Suspects"]
        },
        {
            "id": "lockScreen",
            "name": "Lock Screen",
            "icon": "lock",
            "component": "modules/settings/configs/LockScreenConfig.qml",
            "subPages": ["widgets/LockscreenNotificationsConfig.qml", "widgets/FingerprintConfig.qml"],
            "searchSources": ["sections/LockLookSection.qml", "sections/LockGeneralSection.qml"],
            "aliases": ["Fingerprint", "Biometrics", "fprintd", "Fingerprint reader", "Lock blur", "Vignette", "Color wash", "Desaturate"]
        },
        {
            "id": "about",
            "name": "About & Updates",
            "icon": "info",
            "component": "modules/settings/configs/AboutConfig.qml",
            "aliases": []
        },
        {
            "id": "profile",
            "name": "User Profile",
            "icon": "account_circle",
            "component": "modules/settings/configs/UserProfileConfig.qml",
            "subPages": ["widgets/BannerImageConfig.qml"],
            "aliases": ["Profile", "Avatar Appearance", "Sidebar header", "Right Sidebar Banner"],
            "hidden": true
        },
        {
            "id": "clipboard",
            "name": "Clipboard",
            "icon": "content_paste",
            "component": "modules/settings/configs/ClipboardConfig.qml",
            "subPages": [],
            "aliases": ["Clipboard History Search", "Clipboard"],
            "hidden": true
        },
        {
            "id": "search",
            "name": "Search Results",
            "icon": "search",
            "component": "modules/settings/configs/SearchPage.qml",
            "subPages": [],
            "aliases": [],
            "hidden": true,
            "searchable": false
        }
    ]
    /// Whether a page applies to the running panel family. Pages without a `families`
    /// field apply everywhere, which is the overwhelming majority.
    function availableForFamily(page, family) {
        if (!page || !page.families)
            return true;
        return page.families.indexOf(family ?? "ii") !== -1;
    }

    readonly property var groups: [
        {
            "id": "lookAndFeel",
            "name": "Look & Feel",
            "pageIds": ["colors", "bar", "interfaceFonts", "presets"]
        },
        {
            "id": "modules",
            "name": "Modules",
            "pageIds": ["tablet", "sidebars", "dock", "dynamicIsland"]
        },
        {
            "id": "desktopWindows",
            "name": "Desktop & Windows",
            "pageIds": ["wallpaper", "workspaces", "overview", "windows", "tiling", "lockScreen", "widgets"]
        },
        {
            "id": "tools",
            "name": "Tools",
            "pageIds": ["launcher", "screenCapture", "notifications", "overlays", "modes", "cheatSheet"]
        },
        {
            "id": "servicesIntegrations",
            "name": "Services & Integrations",
            "pageIds": ["mediaMusic", "languageTime", "weather", "aiAssistant", "tasksAccounts"]
        },
        {
            "id": "system",
            "name": "System",
            "pageIds": ["displays", "hyprland", "network", "soundAlerts", "touchGestures", "power", "devicesPhone", "usageStats", "privacy", "about"]
        }
    ]

    function pageIndexById(id) {
        for (let i = 0; i < pages.length; i++) {
            if (pages[i].id === id)
                return i;
        }
        return -1;
    }

    function pageById(id) {
        const i = pageIndexById(id);
        return i >= 0 ? pages[i] : null;
    }
}
