pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Lithic

// Every surface reads these tokens; no component hardcodes a colour, size or duration.
QtObject {
    id: theme

    // Lithic reads the desktop's look itself; the picker only says when a file changed.
    readonly property string _configDir: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/"
    readonly property string _cacheDir: (Quickshell.env("XDG_CACHE_HOME") || (Quickshell.env("HOME") + "/.cache")) + "/ryoku/"
    property list<FileView> lookFiles: [
        FileView { path: theme._configDir + "shell.json"; watchChanges: true; onFileChanged: Lithic.reload() },
        FileView { path: theme._configDir + "theme.json"; watchChanges: true; onFileChanged: Lithic.reload() },
        FileView { path: theme._configDir + "matugen.json"; watchChanges: true; onFileChanged: Lithic.reload() },
        // The daemon creates this directory before starting the picker: a missing parent cannot be watched.
        FileView {
            path: theme._cacheDir + "colors.json"
            watchChanges: true
            onFileChanged: { Lithic.reload(); reload() }
            onLoaded: theme._parseWall(text())
            onLoadFailed: theme.wall = ({})
        }
    ]
    property var wall: ({})
    function _parseWall(t) {
        t = String(t || "").trim()
        if (!t) { theme.wall = ({}); return }
        try { theme.wall = JSON.parse(t) || ({}) }
        catch (e) { theme.wall = ({}) }
    }

    readonly property string design: Lithic.shape.id
    readonly property bool lithic: theme.design === "lithic"
    readonly property bool material: theme.design === "material"
    readonly property bool adwaita: theme.design === "adwaita"
    readonly property bool fluent: theme.design === "fluent"
    readonly property bool cut: Lithic.shape.corner === "cut"
    function cutPx(base) { return base * theme.scale }
    function corner(cut, w, h) { return Lithic.cornerRadius(cut, w, h) }
    readonly property real smoothing: (theme.material ? 16 : theme.adwaita ? 13 : theme.fluent ? 9 : 12) * theme.scale
    readonly property var planes: Lithic.planes()

    function _usable(v) { return typeof v === "string" && v.length > 0 }

    // Role order per entry: primary, primaryText, surface, surfaceText, surfaceVariant, surfaceContainer, background, outline, tertiary.
    readonly property var fallbackPalette: ({
        primary: "#ffb4ab", primaryText: "#1d100e", surface: "#1d100e",
        surfaceText: "#f1dedb", surfaceVariant: "#534341", surfaceContainer: "#271815",
        background: "#1d100e", outline: "#a08c89", tertiary: "#dfc38c"
    })

    readonly property var presets: ({
        "nord": { primary:"#88c0d0", primaryText:"#2e3440", surface:"#3b4252", surfaceText:"#eceff4", surfaceVariant:"#434c5e", surfaceContainer:"#4c566a", background:"#2e3440", outline:"#4c566a", tertiary:"#b48ead" },
        "dracula": { primary:"#bd93f9", primaryText:"#282a36", surface:"#343746", surfaceText:"#f8f8f2", surfaceVariant:"#44475a", surfaceContainer:"#414458", background:"#282a36", outline:"#6272a4", tertiary:"#ff79c6" },
        "tokyo-night": { primary:"#7aa2f7", primaryText:"#1a1b26", surface:"#24283b", surfaceText:"#c0caf5", surfaceVariant:"#414868", surfaceContainer:"#2f3549", background:"#1a1b26", outline:"#565f89", tertiary:"#bb9af7" },
        "tokyo-night-storm": { primary:"#7aa2f7", primaryText:"#1f2335", surface:"#24283b", surfaceText:"#c0caf5", surfaceVariant:"#292e42", surfaceContainer:"#414868", background:"#1f2335", outline:"#565f89", tertiary:"#bb9af7" },
        "tokyo-night-moon": { primary:"#82aaff", primaryText:"#1e2030", surface:"#222436", surfaceText:"#c8d3f5", surfaceVariant:"#2f334d", surfaceContainer:"#444a73", background:"#1e2030", outline:"#636da6", tertiary:"#c099ff" },
        "catppuccin": { primary:"#89b4fa", primaryText:"#1e1e2e", surface:"#313244", surfaceText:"#cdd6f4", surfaceVariant:"#45475a", surfaceContainer:"#292c3c", background:"#1e1e2e", outline:"#6c7086", tertiary:"#f5c2e7" },
        "catppuccin-macchiato": { primary:"#c6a0f6", primaryText:"#181926", surface:"#24273a", surfaceText:"#cad3f5", surfaceVariant:"#363a4f", surfaceContainer:"#494d64", background:"#181926", outline:"#6e738d", tertiary:"#f5bde6" },
        "catppuccin-frappe": { primary:"#ca9ee6", primaryText:"#232634", surface:"#303446", surfaceText:"#c6d0f5", surfaceVariant:"#414559", surfaceContainer:"#51576d", background:"#232634", outline:"#737994", tertiary:"#f4b8e4" },
        "catppuccin-latte": { primary:"#8839ef", primaryText:"#ffffff", surface:"#e6e9ef", surfaceText:"#4c4f69", surfaceVariant:"#ccd0da", surfaceContainer:"#dce0e8", background:"#eff1f5", outline:"#9ca0b0", tertiary:"#ea76cb" },
        "gruvbox": { primary:"#fabd2f", primaryText:"#282828", surface:"#3c3836", surfaceText:"#ebdbb2", surfaceVariant:"#504945", surfaceContainer:"#32302f", background:"#282828", outline:"#665c54", tertiary:"#fe8019" },
        "rose-pine": { primary:"#ebbcba", primaryText:"#191724", surface:"#1f1d2e", surfaceText:"#e0def4", surfaceVariant:"#26233a", surfaceContainer:"#21202e", background:"#191724", outline:"#524f67", tertiary:"#c4a7e7" },
        "rose-pine-moon": { primary:"#c4a7e7", primaryText:"#232136", surface:"#2a273f", surfaceText:"#e0def4", surfaceVariant:"#393552", surfaceContainer:"#44415a", background:"#232136", outline:"#6e6a86", tertiary:"#ea9a97" },
        "rose-pine-dawn": { primary:"#907aa9", primaryText:"#faf4ed", surface:"#fffaf3", surfaceText:"#575279", surfaceVariant:"#f2e9e1", surfaceContainer:"#dfdad9", background:"#faf4ed", outline:"#9893a5", tertiary:"#d7827e" },
        "everforest": { primary:"#a7c080", primaryText:"#2d353b", surface:"#343f44", surfaceText:"#d3c6aa", surfaceVariant:"#3d484d", surfaceContainer:"#475258", background:"#2d353b", outline:"#859289", tertiary:"#83c092" },
        "kanagawa": { primary:"#7e9cd8", primaryText:"#16161d", surface:"#1f1f28", surfaceText:"#dcd7ba", surfaceVariant:"#2a2a37", surfaceContainer:"#363646", background:"#16161d", outline:"#727169", tertiary:"#d27e99" },
        "solarized-dark": { primary:"#268bd2", primaryText:"#fdf6e3", surface:"#073642", surfaceText:"#93a1a1", surfaceVariant:"#0a4652", surfaceContainer:"#11525f", background:"#002b36", outline:"#586e75", tertiary:"#2aa198" },
        "solarized-light": { primary:"#268bd2", primaryText:"#fdf6e3", surface:"#eee8d5", surfaceText:"#657b83", surfaceVariant:"#e4ddc4", surfaceContainer:"#d9d2b8", background:"#fdf6e3", outline:"#93a1a1", tertiary:"#b58900" },
        "one-dark": { primary:"#61afef", primaryText:"#21252b", surface:"#282c34", surfaceText:"#abb2bf", surfaceVariant:"#2c313a", surfaceContainer:"#333842", background:"#21252b", outline:"#5c6370", tertiary:"#c678dd" },
        "monokai": { primary:"#f92672", primaryText:"#1e1f1c", surface:"#272822", surfaceText:"#f8f8f2", surfaceVariant:"#34352f", surfaceContainer:"#3e3d32", background:"#1e1f1c", outline:"#75715e", tertiary:"#66d9ef" },
        "github-dark": { primary:"#58a6ff", primaryText:"#0d1117", surface:"#161b22", surfaceText:"#c9d1d9", surfaceVariant:"#21262d", surfaceContainer:"#30363d", background:"#0d1117", outline:"#484f58", tertiary:"#bc8cff" },
        "github-light": { primary:"#0969da", primaryText:"#ffffff", surface:"#f6f8fa", surfaceText:"#24292f", surfaceVariant:"#eaeef2", surfaceContainer:"#d0d7de", background:"#ffffff", outline:"#8c959f", tertiary:"#8250df" },
        "night-owl": { primary:"#82aaff", primaryText:"#011627", surface:"#0b2942", surfaceText:"#d6deeb", surfaceVariant:"#1d3b53", surfaceContainer:"#234d70", background:"#011627", outline:"#5f7e97", tertiary:"#c792ea" },
        "palenight": { primary:"#c792ea", primaryText:"#292d3e", surface:"#32364a", surfaceText:"#a6accd", surfaceVariant:"#3a3f58", surfaceContainer:"#444867", background:"#292d3e", outline:"#676e95", tertiary:"#89ddff" },
        "synthwave84": { primary:"#ff7edb", primaryText:"#241b2f", surface:"#262335", surfaceText:"#f0eff1", surfaceVariant:"#2f2745", surfaceContainer:"#3a2f56", background:"#241b2f", outline:"#848bbd", tertiary:"#36f9f6" },
        "ayu-dark": { primary:"#e6b450", primaryText:"#0b0e14", surface:"#0f131a", surfaceText:"#bfbdb6", surfaceVariant:"#161b24", surfaceContainer:"#1e232d", background:"#0b0e14", outline:"#565b66", tertiary:"#59c2ff" },
        "ayu-mirage": { primary:"#ffcc66", primaryText:"#171b24", surface:"#1f2430", surfaceText:"#cccac2", surfaceVariant:"#242936", surfaceContainer:"#2d3444", background:"#171b24", outline:"#707a8c", tertiary:"#5ccfe6" }
    })

    readonly property var wallPalette: ({
        primary: String(Lithic.sel),
        primaryText: String(Lithic.onSel),
        surface: String(Lithic.stone2),
        surfaceText: String(Lithic.fg),
        surfaceVariant: String(Lithic.stone3),
        surfaceContainer: String(Lithic.field),
        background: String(Lithic.ground),
        outline: String(Lithic.edge),
        tertiary: String(Lithic.jasper)
    })

    // null wears the desktop's palette; any role the candidate omits falls through.
    property var previewPalette: null

    readonly property var palette: {
        var base = theme.wallPalette
        var p = theme.previewPalette
        if (!p)
            return base
        function pick(k) { return theme._usable(p[k]) ? p[k] : base[k] }
        return {
            primary: pick("primary"),
            primaryText: pick("primaryText"),
            surface: pick("surface"),
            surfaceText: pick("surfaceText"),
            surfaceVariant: pick("surfaceVariant"),
            surfaceContainer: pick("surfaceContainer"),
            background: pick("background"),
            outline: pick("outline"),
            tertiary: pick("tertiary")
        }
    }

    readonly property color primary: theme.palette.primary            // accent, selection, fills, focus, markers
    readonly property color primaryText: theme.palette.primaryText    // text on accent fills
    readonly property color surface: theme.palette.surface            // panel/sheet background
    readonly property color surfaceText: theme.palette.surfaceText    // body text
    readonly property color surfaceVariant: theme.palette.surfaceVariant   // hover fills
    readonly property color surfaceContainer: theme.palette.surfaceContainer // cards, rows, idle button fill
    readonly property color background: theme.palette.background      // backdrop behind panels; marker interiors
    readonly property color outline: theme.palette.outline            // borders, dividers, rules
    readonly property color tertiary: theme.palette.tertiary          // destructive accent (delete/stop)
    readonly property color destructive: theme.palette.tertiary

    // Tempers shadow and scrim strength on a light theme.
    readonly property bool isDark: {
        var c = Qt.color(theme.surface)
        return (0.2126 * c.r + 0.7152 * c.g + 0.0722 * c.b) < 0.5
    }

    function withAlpha(c, a) { var q = Qt.color(c); return Qt.rgba(q.r, q.g, q.b, a) }

    property SettingValue uiScaleSetting: SettingValue { key: "general.uiScale" }
    readonly property real scale: {
        var v = Number(theme.uiScaleSetting.value)
        if (isNaN(v) || v <= 0) v = 1.2
        return Math.max(0.5, Math.min(2.0, v))
    }
    // Text floor: shrink geometry freely but keep type readable.
    readonly property real legibleScale: Math.max(theme.scale, 0.95)
    function px(base) { return base * theme.scale }   // geometry
    function fs(base) { return base * theme.legibleScale }   // text

    // Base px; use fs() and px() for any size not named here.
    readonly property var typeScale: ({
        micro: 8, tiny: 8.5, fine: 9, mini: 9.5, small: 10, xsmall: 10.5,
        base: 11, wide: 11.5, body: 12, body2: 12.5, label: 13, label2: 13.5,
        field: 14, lead: 15, head: 16, title: 20, segment: 22, studio: 31, folio: 46
    })
    readonly property real fontMicro: fs(8)
    readonly property real fontTiny: fs(8.5)
    readonly property real fontFine: fs(9)
    readonly property real fontMini: fs(9.5)
    readonly property real fontSmall: fs(10)
    readonly property real fontXSmall: fs(10.5)
    readonly property real fontBase: fs(11)
    readonly property real fontWide: fs(11.5)
    readonly property real fontBody: fs(12)
    readonly property real fontBody2: fs(12.5)
    readonly property real fontLabel: fs(13)
    readonly property real fontLabel2: fs(13.5)
    readonly property real fontField: fs(14)
    readonly property real fontLead: fs(15)
    readonly property real fontHead: fs(16)
    readonly property real fontTitle: fs(20)
    readonly property real fontSegment: fs(22)
    readonly property real fontStudio: fs(31)
    readonly property real fontSection: fs(46)

    function space(n) { return n * theme.scale }
    readonly property real spaceXs: space(4)
    readonly property real spaceSm: space(7)
    readonly property real spaceMd: space(9)
    readonly property real spaceLg: space(13)
    readonly property real spaceXl: space(20)
    readonly property real spaceXxl: space(28)

    // A plain Rectangle cannot cut, so Lithic keeps a hair of rounding there.
    readonly property real radius: theme.cut ? 2 * theme.scale : Lithic.cornerRadius(7, 200, 40) * theme.scale
    readonly property real panelSkew: 14 * theme.scale

    // Folio sheet geometry (base px; components clamp to the viewport).
    readonly property real folioSheetWidth: 1390
    readonly property real folioSheetHeight: 870
    readonly property real folioSheetMargin: 22
    readonly property real folioSheetMinWidth: 220
    readonly property real folioSheetMinHeight: 160
    readonly property real folioIndexWidth: 318
    readonly property real folioMastheadHeight: 54

    property SettingValue fastSetting: SettingValue { key: "motion.fastMs" }
    property SettingValue standardSetting: SettingValue { key: "motion.standardMs" }
    property SettingValue slowSetting: SettingValue { key: "motion.slowMs" }
    function _ms(sv, def) { var v = Number(sv.value); return (isNaN(v) || v <= 0) ? def : Math.round(v) }
    readonly property int fast: _ms(theme.fastSetting, 180)
    readonly property int standard: _ms(theme.standardSetting, 250)
    readonly property int slow: _ms(theme.slowSetting, 450)

    property SettingValue openFadeSetting: SettingValue { key: "general.openFadeFrom" }
    readonly property real openFadeFrom: {
        var v = Number(theme.openFadeSetting.value)
        return (isNaN(v) ? 0 : Math.max(0, Math.min(100, v))) / 100
    }

    // InOutQuad is the built-in curve nearest smoothstep, so reveals run without per-frame JS.
    readonly property int revealEasing: Easing.InOutQuad
    function smoothstep(t) { t = Math.max(0, Math.min(1, t)); return t * t * (3 - 2 * t) }

    readonly property string ui: Fonts.ui
    readonly property int uiWeight: Fonts.uiWeight
    readonly property string icon: Fonts.icon
    readonly property string display: Fonts.display
    readonly property string sans: Fonts.sans
    readonly property string jp: Fonts.jp
    readonly property string mono: Fonts.mono
}
