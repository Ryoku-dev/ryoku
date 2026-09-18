pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.FrameBars
import Ryoku.Ui.Singletons

// live shell appearance config. one source of truth for the look knobs Ryoku
// Settings' Shell section edits, plus the shipped defaults the shell falls back
// to. JSON at ~/.config/ryoku/shell.json, watched, so a save in Settings
// retunes the running shell on the next file event. no reload. defaults here
// are canonical; Settings mirrors them for reset-to-default and seeds nothing
// of its own.
//
// geometry = unscaled base pixels at 1080p. osd values are multiplied by the
// per-monitor scale `s` where they're read.
Singleton {
    id: root

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

    property alias frameBars: adapter.frameBars
    readonly property var normalizedFrameBars: FrameBars.normalize(frameBars, BarCatalog, MenuCatalog)

    // barStyle: which bar design renders. "qsbar" is the default QS Bar top bar
    // (a shipped folder style under modules/bar/barstyles/qsbar); "sumi" is the
    // built-in painted left rail; any other id is an installed store folder
    // style. Each owns its own bar, popouts and settings. Default qsbar.
    property alias barStyle: adapter.barStyle

    // obi: per-widget visibility for the Obi bar style, edited in Bar Studio.
    // A map of widgetId -> bool; an absent key reads as shown, so the bar is
    // full by default and only an explicit false hides a widget. Each folder
    // style gets its own key here, the extensible per-style settings store.
    property alias obi: adapter.obi
    property alias nacre: adapter.nacre
    property alias qsbar: adapter.qsbar
    property alias chroma: adapter.chroma
    property alias kairos: adapter.kairos

    // dock: the first-class app dock surface (modules/dock). A top-level store,
    // not a bar-style key, because the dock is now style-agnostic -- neither qsbar
    // nor Sumi owns it. Off until the user turns it on (Hub -> Bar Studio -> Dock).
    // Read and written through the services Dock singleton so every consumer goes
    // through one place.
    property alias dock: adapter.dock
    readonly property var normalizedNacre: NacreConfig.normalize(nacre)

    function chromaScale() {
        const cfg = root.chroma || ({});
        const value = Number(cfg.scale);
        if (!isFinite(value))
            return 1;
        return Math.round(Math.max(0.6, Math.min(1.4, value)) * 20) / 20;
    }

    function chromaWidgetEnabled(id) {
        const cfg = root.chroma || ({});
        const widgets = cfg.widgets || ({});
        return widgets[id] !== false;
    }

    // typography: a scale that grows or shrinks the whole shell (the bar text
    // and the surfaces around it), keeping the readout legible without overflow.
    property alias fontScale:  adapter.fontScale

    // fontFamily: the single system font. Empty resolves to Space Grotesk (UI)
    // and SpaceMono (monospace/terminal); when set, the daemon mirrors it to GTK,
    // Qt and the terminal so everything matches. Set from Hub -> Global.
    property alias fontFamily: adapter.fontFamily

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
        shaderCtl.queued += "call settings.patch "
            + JSON.stringify({ path: "screenShader", value: pick }) + "\n";
        if (shaderCtl.connected)
            shaderCtl.flushQueued();
        else
            shaderCtl.connected = true;
        Wm.setScreenShader(pick);
    }

    // Which surface the bar's brand logo opens: "studio" (QS Bar Settings) or
    // "quick" (the Super+Esc quick-settings sidebar). Persisted like the shader,
    // read by the launcher widget and set by the switch in either panel.
    property alias launcherTarget: adapter.launcherTarget
    function setLauncherTarget(v) {
        const pick = (v === "quick") ? "quick" : "studio";
        root.launcherTarget = pick;
        shaderCtl.queued += "call settings.patch "
            + JSON.stringify({ path: "launcherTarget", value: pick }) + "\n";
        if (shaderCtl.connected)
            shaderCtl.flushQueued();
        else
            shaderCtl.connected = true;
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
    // the two dynamic variants (Default, Wallpaper). The shared JsonAdapter can't
    // represent a removed key -- it only overwrites keys still present -- so read
    // presence straight from the frame text: a missing key resolves to null, and
    // Theme.namedScheme then falls back to the wallpaper or base palette.
    property var themePalette: null
    function refreshThemePalette() {
        var pal = null;
        var t = file.text();
        if (t) {
            try {
                var o = JSON.parse(t);
                if (o && typeof o.themePalette === "object" && o.themePalette !== null)
                    pal = o.themePalette;
            } catch (e) {
            }
        }
        themePalette = pal;
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

    FileView {
        id: file
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/shell.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        atomicWrites: true
        onFileChanged: reload()
        onLoaded: root.refreshThemePalette()

        JsonAdapter {
            id: adapter
            property bool frameEnabled: true
            property real frameOpacity: 1
            property real frameThickness: 2
            property real frameCorner: 8
            property real osdRadius: 0
            property real osdOpacity: 1
            property real fontScale: 1.3
            property string fontFamily: "Space Grotesk"
            property int fontSize: 11
            property string weatherLocation: ""
            property string weatherUnit: "auto"
            property string formatLocale: ""
            property string screenShader: ""
            property var frameBars: FrameBars.defaultConfig()
            property string barStyle: "qsbar"
            property string launcherTarget: "studio"
            property var obi: ({})
            property var nacre: NacreConfig.defaultConfig()
            property var qsbar: ({})
            property var chroma: ({})
            property var kairos: ({})
            property var dock: ({
                "enabled": false,
                "edge": "auto",
                "autohide": true,
                "pinned": [],
                "magnify": true,
                "frost": true,
                "shadow": true,
                "labels": true,
                "media": false
            })
        }
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

    // The daemon's control socket: shell.json is read-only here (the daemon owns
    // it and serialises every writer), so a persisted write goes through it.
    Socket {
        id: shaderCtl
        path: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"
        property string queued: ""
        function flushQueued() {
            if (queued.length === 0)
                return;
            write(queued);
            flush();
            queued = "";
        }
        onConnectionStateChanged: if (connected) flushQueued()
    }

    // seed only on a genuine first run (nothing to load), so a slow or failed
    // load can't overwrite a present file with defaults.
    Component.onCompleted: {
        if (!file.text()) file.writeAdapter();
        if (!brandFile.text()) brandFile.writeAdapter();
        root.refreshThemePalette();
    }
}
