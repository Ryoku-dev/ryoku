pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.FrameBars
import Ryoku.Ui.Singletons


// live shell appearance config. one source of truth for the look knobs Ryoku
// Settings' Shell section edits, plus the shipped defaults the shell falls back
// to. A view of ShellSettings (the daemon's `settings` frame of
// ~/.config/ryoku/shell.json), so a save in Settings retunes the running shell
// as soon as the daemon publishes it. no reload. defaults here are canonical;
// Settings mirrors them for reset-to-default and seeds nothing of its own.
//
// geometry = unscaled base pixels at 1080p. osd values are multiplied by the
// per-monitor scale `s` where they're read.
Singleton {
    id: root
    signal settingsLoaded()


    // frame = the painted border band. Its enable, opacity, band thickness and
    // corner radius are live knobs; the rail geometry lives in frameBars.
    property alias frameEnabled:   adapter.frameEnabled
    property alias frameOpacity:   adapter.frameOpacity
    property alias frameThickness: adapter.frameThickness
    property alias frameCorner:    adapter.frameCorner

    // osd = the volume/brightness flash and notification toasts: small edge
    // windows that share the frame surface. osdRadius rounds their corners,
    // osdOpacity fades them.
    property alias osdRadius:  adapter.osdRadius
    property alias osdOpacity: adapter.osdOpacity

    // Deck frost belongs to the desktop behind client windows. Bar styles keep
    // their own opacity controls so a growing window count cannot erase them.
    // deckFrostEnabled is the on/off switch; zero strength is a hard off.
    property alias deckFrostEnabled: adapter.deckFrostEnabled
    property alias deckFrost: adapter.deckFrost

    function deckFrostIntensity(n: int): real {
        if (root.deckFrostEnabled === false)
            return 0;
        const base = Math.max(0, Math.min(1, Number(root.deckFrost) || 0));
        if (base === 0)
            return 0;
        return base + 0.25 * (Math.max(1, n) - 1);
    }

    property alias frameBars: adapter.frameBars
    readonly property var normalizedFrameBars: FrameBars.normalize(frameBars, BarCatalog, MenuCatalog)

    // barStyle selects one shipped or installed item through Shells.
    // Orpheus is the fallback and the active item owns its own presentation.
    property alias batteryAlerts: adapter.batteryAlerts
    property alias barStyle: adapter.barStyle
    property alias notifications: adapter.notifications
    readonly property var sounds: adapter.sounds
    readonly property var audio: adapter.audio
    readonly property var themeSizing: adapter.theme?.attributes?.sizing ?? ({})

    // obi: per-widget visibility for the Obi bar style, edited in Bar Studio.
    // A map of widgetId -> bool; an absent key reads as shown, so the bar is
    // full by default and only an explicit false hides a widget. Each folder
    // style gets its own key here, the extensible per-style settings store.
    property alias obi: adapter.obi
    property alias nacre: adapter.nacre
    property alias qsbar: adapter.qsbar
    property alias kairos: adapter.kairos
    property alias orpheus: adapter.orpheus

    // Style stores are loaded here, but each style persists through the daemon.
    property alias python: adapter.python
    readonly property var inir: adapter.inir

    readonly property bool styleOwnsBanners: Shells.owns("notifications")
    readonly property bool styleOwnsNotificationSound: Shells.owns("notificationSound")
    readonly property bool styleOwnsFeedback: Shells.owns("osd")

    // frame: the whole parsed shell.json, for keys the adapter does not declare
    // (an item's own switches). value() reads it by dotted path.
    property var frame: ({})
    function value(path) {
        let node = root.frame;
        const parts = String(path || "").split(".");
        for (let i = 0; i < parts.length; ++i) {
            if (node === null || typeof node !== "object")
                return undefined;
            node = node[parts[i]];
        }
        return node;
    }

    // dock: the universal app dock store. It is top-level because design,
    // placement and pins survive bar-style and dock-design changes. Stage
    // Editor writes it through the Dock singleton; every renderer reads it.
    property alias dock: adapter.dock

    // designs: the OSD, notification card and Super+Esc sidebar every bar
    // style shares. "ryoku" is the shipped design of each; any other id is a
    // design RyoStore installed. Designs resolves them for the hosts.
    property alias designs: adapter.designs

    // clipboard: geometry and corner treatment for the bottom-centred history
    // surface. The adapter keeps it a self-contained shell.json subtree so it
    // can retune live without a shell restart.
    property alias clipboard: adapter.clipboard
    // Phone integration is optional and remains dormant until its Hub install
    // transaction has completed.
    property alias phone: adapter.phone
    readonly property var normalizedNacre: NacreConfig.normalize(nacre)
    readonly property var defaultControls: ({
        "position": "top-left",
        "edgeGap": 12,
        "sections": [
            { "id": "vitals", "visible": true },
            { "id": "connections", "visible": true },
            { "id": "powerProfile", "visible": true },
            { "id": "media", "visible": false },
            { "id": "levels", "visible": true },
            { "id": "bottomControls", "visible": true }
        ],
        "hidden": []
    })
    property var controls: defaultControls

    function patchControl(key, value) {
        if (key !== "position" && key !== "edgeGap")
            return;
        const next = Object.assign({}, root.controls);
        next[key] = value;
        root.controls = next;
        ShellSettings.patch("controls." + key, value);
    }

    readonly property var askBubble: {
        const ask = adapter.ask && typeof adapter.ask === "object" ? adapter.ask : ({});
        const bubble = ask.bubble && typeof ask.bubble === "object" ? ask.bubble : ({});
        const x = Number(bubble.x);
        const y = Number(bubble.y);
        return {
            enabled: bubble.enabled === true,
            x: Math.max(0, Math.min(1, isFinite(x) ? x : 0.94)),
            y: Math.max(0, Math.min(1, isFinite(y) ? y : 0.68)),
            screen: String(bubble.screen || "")
        };
    }

    function patchAskBubble(key, value) {
        if (["enabled", "x", "y", "screen"].indexOf(key) < 0)
            return;
        let clean = value;
        if (key === "x" || key === "y") {
            const number = Number(value);
            clean = Math.max(0, Math.min(1, isFinite(number) ? number : root.askBubble[key]));
        } else if (key === "enabled") {
            clean = value === true;
        } else {
            clean = String(value || "");
        }
        const next = Object.assign({}, root.askBubble);
        next[key] = clean;
        adapter.ask = { bubble: next };
        ShellSettings.patch("ask.bubble." + key, clean);
    }

    // typography: a scale that grows or shrinks the whole shell (the bar text
    // and the surfaces around it), keeping the readout legible without overflow.
    property alias fontScale:  adapter.fontScale

    // The UI and terminal families are independent: proportional choices stay
    // in the shell and apps, while terminals keep an explicitly monospace face.
    // Both are set from Hub -> Global and applied system-wide by the daemon.
    property alias fontFamily:         adapter.fontFamily
    property alias terminalFontFamily: adapter.terminalFontFamily

    // fontSize: the base point size the daemon mirrors system-wide (apps and the
    // terminal). Set from Hub -> Global.
    property alias fontSize: adapter.fontSize

    // weather: an explicit location override (a city name; blank = auto-locate by
    // IP) and the temperature unit ("auto" follows the locale, else "celsius" /
    // "fahrenheit"). the Weather singleton reads both.
    property alias weatherLocation: adapter.weatherLocation
    property alias weatherUnit:     adapter.weatherUnit

    // formatLocale: the regional-formats locale (the LC_* equivalent) the shell
    // uses for dates, month/day names, numbers and currency, kept SEPARATE from
    // the UI language so an English desktop can still read Brazilian (or any)
    // regional formats. Empty = follow the system locale. Set from the Hub's
    // Region control; a plain passthrough key in shell.json.
    property alias formatLocale: adapter.formatLocale

    // screenShader: the compositor's print filter, by shader name. Persisted, and
    // applied live through the provider, which resolves the name to its shader.
    property alias screenShader: adapter.screenShader
    readonly property var screenShaders: ["", "halftone", "bone", "onebit", "vignette", "grain"]
    function setScreenShader(name) {
        const pick = root.screenShaders.indexOf(name) >= 0 ? name : "";
        root.screenShader = pick;
        ShellSettings.patch("screenShader", pick);
        Wm.setScreenShader(pick);
    }

    // Which surface the bar's brand logo opens: "studio" (QS Bar Settings) or
    // "quick" (the Super+Esc quick-settings sidebar). Persisted like the shader,
    // read by the launcher widget and set by the switch in either panel.
    property alias launcherTarget: adapter.launcherTarget
    function setLauncherTarget(v) {
        const pick = (v === "quick") ? "quick" : "studio";
        root.launcherTarget = pick;
        ShellSettings.patch("launcherTarget", pick);
    }

    // resolved Qt locale: the chosen region, else the system default. the
    // ".UTF-8" suffix and a BCP47 dash are normalised to what Qt.locale() wants.
    readonly property var formatLoc: {
        var f = ("" + formatLocale).trim();
        return f.length > 0 ? Qt.locale(f.split(".")[0].replace("-", "_")) : Qt.locale();
    }

    // matchWallpaper: when on, every shell surface (frame, bar, popouts, plus
    // desktop widgets, plugin tiles, the window switcher)
    // follows the live palette instead of the static Tokyo Night
    // tokens. sourced from theme.json (`FollowWallpaper`, the single colour
    // master shared with the daemon and window borders). on by default.
    property alias matchWallpaper: themeAdapter.followWallpaper

    // themePalette: the active static theme's palette (role token -> hex), which
    // the daemon resolves and writes as a top-level key in shell.json; absent for
    // the two dynamic variants (Default, Wallpaper). A missing key resolves to
    // null, and Theme.namedScheme then falls back to the wallpaper or base palette.
    property var themePalette: null
    function refreshThemePalette() {
        const o = root.settingsFrame();
        root.themePalette = typeof o.themePalette === "object" && o.themePalette !== null ? o.themePalette : null;
        root.controls = typeof o.controls === "object" && o.controls !== null ? o.controls : root.defaultControls;
    }

    function settingsFrame() {
        const d = ShellSettings.data;
        return d && typeof d === "object" ? d : ({});
    }

    // a missing or mistyped key falls back to the adapter's initial value
    readonly property var adapterKeys: [
        "frameEnabled", "frameOpacity", "frameThickness", "frameCorner", "osdRadius",
        "osdOpacity", "deckFrostEnabled", "deckFrost", "fontScale", "fontFamily",
        "terminalFontFamily", "fontSize", "weatherLocation", "weatherUnit", "formatLocale",
        "screenShader", "frameBars", "batteryAlerts", "theme", "notifications", "sounds",
        "audio", "barStyle", "launcherTarget", "obi", "nacre", "qsbar", "kairos",
        "orpheus", "python", "inir", "dock", "designs", "clipboard", "phone", "ask"
    ]
    property var adapterDefaults: ({})

    function settingValue(frame, key) {
        const def = root.adapterDefaults[key];
        const v = frame[key];
        if (v === undefined || v === null)
            return def;
        switch (typeof def) {
        case "boolean":
            return typeof v === "boolean" ? v : def;
        case "number": {
            const n = Number(v);
            return isFinite(n) ? n : def;
        }
        case "string":
            return typeof v === "string" ? v : def;
        default:
            return v;
        }
    }

    // only touched keys are re-read so object settings keep their identity and bindings stay quiet
    function applySettings(paths) {
        const frame = root.settingsFrame();
        let keys = root.adapterKeys;
        let palette = true;
        if (Array.isArray(paths) && !paths.some(p => !Array.isArray(p) || p.length === 0)) {
            const touched = {};
            for (const p of paths)
                touched[String(p[0])] = true;
            keys = root.adapterKeys.filter(k => touched[k] === true);
            palette = touched.themePalette === true || touched.controls === true;
        }
        for (const k of keys)
            adapter[k] = root.settingValue(frame, k);
        root.frame = frame;
        if (palette)
            root.refreshThemePalette();
        root.settingsLoaded();
    }

    Connections {
        target: ShellSettings
        // lastPaths since the signal argument arrives as a QML sequence
        function onChanged() { root.applySettings(ShellSettings.lastPaths); }
    }

    // brand: the desktop's mark + name, user-overridable from Ryoku Settings ->
    // Shell -> Global. a small cross-cutting identity master (like theme.json).
    // markText is the glyph/short-text seal (default 力); markImage an optional
    // image path that wins over the text; markTint recolours a single-colour
    // image to the accent; name is the wordmark ("Ryoku") shown in chrome copy.
    // Ryoku's own apps (the Hub, ryo* apps) never read this and keep the 力 brand.
    property alias markText:  brandAdapter.markText
    property alias markImage: brandAdapter.markImage
    property alias markTint:  brandAdapter.markTint
    property alias brandName: brandAdapter.name

    QtObject {
        id: adapter
        property bool frameEnabled: true
        property real frameOpacity: 1
        property real frameThickness: 2
        property real frameCorner: 8
        property real osdRadius: 0
        property real osdOpacity: 1
        property bool deckFrostEnabled: true
        property real deckFrost: 0.45
        property real fontScale: 1.3
        property string fontFamily: "Space Grotesk"
        property string terminalFontFamily: "JetBrainsMono Nerd Font"
        property int fontSize: 11
        property string weatherLocation: ""
        property string weatherUnit: "auto"
        property string formatLocale: ""
        property string screenShader: ""
        property var frameBars: FrameBars.defaultConfig()
        property var batteryAlerts: ({ enabled: true, warningEnabled: true, criticalEnabled: true, warningPercent: 25, criticalPercent: 10 })
        property var theme: ({
            "attributes": {
                "sizing": {
                    "radius_widget": 8,
                    "radius_window": 8
                }
            }
        })
        property var notifications: ({
            "notification_position": "Right",
            "popup_window_margins": 0,
            "show_images": false
        })
        property var sounds: ({})
        property var audio: ({ "boost": false })
        property string barStyle: "orpheus"
        property string launcherTarget: "studio"
        property var obi: ({})
        property var nacre: NacreConfig.defaultConfig()
        property var qsbar: ({})
        property var kairos: ({})
        property var orpheus: ({})
        property var python: ({})
        property var inir: ({})
        property var dock: ({
            "enabled": false,
            "design": "ryoku",
            "edge": "auto",
            "edgeGap": 0,
            "autohide": false,
            "revealOnEmpty": true,
            "size": 44,
            "pinned": [],
            "magnify": true,
            "style": "islands",
            "frost": true,
            "shadow": true,
            "labels": true,
            "media": false,
            "python": {
                "onTop": true,
                "floating": false,
                "opacity": 100,
                "exclusive": false,
                "smartAutohide": true,
                "autohideTimeout": 1000,
                "overrideBoundsCorrection": false,
                "enableScrolling": false,
                "visibleElements": 7,
                "hoverScale": 120,
                "cascadeScale": false
            },
            "shima": {
                "shape": "auto",
                "notch": true,
                "material": "inherit",
                "magnifySize": 150,
                "launcher": true,
                "reserveSpace": true,
                "revealOnEmpty": true,
                "badges": true
            }
        })
        property var designs: ({
            "osd": "ryoku",
            "notification": "ryoku",
            "sidebar": "ryoku"
        })
        property var clipboard: ({
            "widthPercent": 65,
            "heightPercent": 42,
            "bottomPercent": 0,
            "panelRadius": 18,
            "paneRadius": 12,
            "cardRadius": 9,
            "pruneWeekly": false
        })
        property var phone: ({
            "enabled": false,
            "desktopNotifications": true,
            "contacts": {
                "enabled": true,
                "hideUnnamed": true
            },
            "scrcpy": {
                "useWireless": false,
                "autoWirelessIp": true,
                "wirelessIp": "",
                "wirelessPort": 5555,
                "pinAdbPort": false,
                "maxSize": 0,
                "maxFps": 60,
                "videoBitRate": "8M",
                "audio": true,
                "turnScreenOff": false,
                "stayAwake": true,
                "showTouches": false,
                "clipboardSync": true,
                "keyboardMode": "uhid",
                "autoResume": true,
                "appMode": {
                    "showAppIcons": true,
                    "autoUnlock": true,
                    "flexDisplay": false,
                    "displayWidth": 1280,
                    "displayHeight": 960,
                    "displayDpi": 240,
                    "onSessionEnd": "home"
                },
                "recording": { "folder": "" }
            },
            "webcam": {
                "cameraFacing": "back",
                "width": 1280,
                "height": 720,
                "fps": 30,
                "videoDevice": ""
            },
            "microphone": {
                "micGain": 100,
                "monitor": false,
                "setDefault": false
            },
            "alarm": {
                "enabled": true,
                "pollMinutes": 10,
                "clockApps": [],
                "ignorePackages": []
            }
        })
        property var ask: ({
            "bubble": {
                "enabled": false,
                "x": 0.94,
                "y": 0.68,
                "screen": ""
            }
        })
    }

    // The colour-source master lives in theme.json (single source: the daemon,
    // window borders and shell chrome all read it). true = follow the wallpaper.
    FileView {
        id: themeFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/theme.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        JsonAdapter { id: themeAdapter; property bool followWallpaper: true }
    }

    // brand identity master (mark + name), shared with doctor and the
    // Hub's Shell -> Global editor. seeded once on first run below.
    FileView {
        id: brandFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/brand.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        atomicWrites: true
        onFileChanged: reload()
        JsonAdapter {
            id: brandAdapter
            property string markText: "力"
            property string markImage: ""
            property bool markTint: true
            property string name: "Ryoku"
        }
    }

    // first-run seed only, the daemon owns shell.json after that
    FileView {
        id: seedFile
        preload: false
        printErrors: false
        atomicWrites: true
    }

    Component.onCompleted: {
        const defaults = {};
        for (const k of root.adapterKeys)
            defaults[k] = adapter[k];
        root.adapterDefaults = defaults;
        if (ShellSettings.fileMissing) {
            seedFile.path = (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/shell.json";
            seedFile.setText(JSON.stringify(defaults, null, 2) + "\n");
        }
        if (!brandFile.text()) brandFile.writeAdapter();
        root.applySettings(null);
    }
}
