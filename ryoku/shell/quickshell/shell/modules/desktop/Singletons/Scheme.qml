pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons as Ui

// Thin reader of the daemon palette for the desktop widgets. The theme daemon is
// the sole author of colour in Ryoku: a fixed named theme publishes its palette
// into shell.json's themePalette key, and follow-the-wallpaper mode writes the
// live palette to ~/.cache/ryoku/colors.json. This resolves each role the
// pill's way -- a named scheme wins, then the live wallpaper palette, then the
// compiled default -- so the clock, its card and the right-click menu retint on
// ANY scheme change (static named theme or wallpaper-follow) with no colour math
// of its own. Defaults are a cool near-black so widgets read before the daemon's
// first write. base/elevated/deep/line map onto the palette's own surface depth
// ramp; accent/accent2 are Material accent roles consumed verbatim.
Singleton {
    id: root

    // Follow-the-wallpaper master (theme.json), the static named scheme's palette
    // (shell.json themePalette; null for the dynamic variants) and the live
    // wallpaper roles (colors.json). All three are parsed straight from the file
    // text: half the role names start with "on" (which JsonAdapter's
    // signal-handler grammar rejects), a removed themePalette key only reads as
    // absent from raw text, and a bare JsonAdapter does not repopulate reliably
    // for a lazily-created singleton.
    property bool matchWallpaper: true
    property var namedScheme: null
    property var wall: ({})
    property var preview: ({})

    // A palette value is only usable when it is a non-empty hex string; a null,
    // missing or half-written role falls through to the next layer, so a partial
    // colors.json (mid-write) or a scheme missing a role never paints black.
    function usable(v) { return typeof v === "string" && v.length > 0; }

    // Resolve one role through the layer chain: a selected named scheme wins,
    // then the live wallpaper palette while Match wallpaper is on, then the base.
    function role(key, base) {
        if (usable(preview[key]))
            return preview[key];
        if (namedScheme && usable(namedScheme[key]))
            return namedScheme[key];
        if (matchWallpaper && usable(wall[key]))
            return wall[key];
        return base;
    }

    // --- surface depth ramp (the menu plate and the clock card sit on these) --
    readonly property color surface:  role("surface", "#17161d")
    readonly property color base:     surface
    readonly property color elevated: role("surfaceContainer", "#26242d")
    readonly property color deep:     role("surfaceContainerLowest", "#0f0c07")
    readonly property color line:     role("outlineVariant", "#3d484d")

    // --- ink, resolved from the palette so text stays legible on its surface.
    // Declared plain and set via Binding: QML's signal-handler grammar parses an
    // inline `onSurface:` as a change-handler for the sibling `surface` property
    // and silently drops the binding (the transparent-black trap), so a Binding
    // element assigns the role by name instead. ---
    property color onSurface
    property color onSurfaceVariant
    Binding { target: root; property: "onSurface";        value: root.role("onSurface", "#f5f3ff") }
    Binding { target: root; property: "onSurfaceVariant"; value: root.role("onSurfaceVariant", "#9aa3c8") }

    // --- accents: Material accent roles, verbatim (the daemon curates them) ----
    readonly property color accent:    role("primary", "#e2342a")
    readonly property color accent2:   role("secondary", "#7fbbb3")
    // The sweep the ring clock samples, held as ramp names, and only the
    // wallpaper's own hue: matugen's primary and secondary both derive from the
    // source colour, so each stop reads as the picture under the widget.
    // tertiary (a +60 hue rotation) and error (a fixed red) are Material accents
    // the wallpaper never contains, so a sweep built from them paints colours
    // nowhere in the picture. `seeds` are the matching roles, re-lit when no
    // ramps are published.
    readonly property var ramps: ["secondary", "primary", "primary", "primary", "primary", "secondary"]
    readonly property var seeds: [accent2, accent, accent, accent, accent, accent2]

    // The sweep at t in [0,1] over a background of L* `bgL`.
    function colorAt(t, bgL) {
        var n = root.ramps.length;
        var x = Math.max(0, Math.min(0.999999, t)) * (n - 1);
        var i = Math.floor(x);
        var f = x - i;
        var a = root.stopAt(i, bgL);
        var b = root.stopAt(Math.min(n - 1, i + 1), bgL);
        return Qt.rgba(a.r + (b.r - a.r) * f,
                       a.g + (b.g - a.g) * f,
                       a.b + (b.b - a.b) * f, 1);
    }

    function stopAt(i, bgL) {
        return Ui.Ink.accentOver(root.ramps[i], root.seeds[i], bgL, 55, Ui.Ink.side(bgL));
    }

    // Ink for a widget with no card under it: it sits on the wallpaper, where
    // on-surface means nothing -- it tracks a panel that is not there and goes
    // near-black under a light scheme. `l` is the widget's own patch of picture,
    // measured by WidgetSlot. 62 L* clears 5.5:1 for a run of text.
    function inkOn(l)     { return Ui.Ink.inkOver("neutral", root.onSurface, l, 62); }
    function inkDimOn(l)  { return Ui.Ink.inkOver("neutral", root.onSurfaceVariant, l, 45); }
    function inkSoftOn(l) { return Ui.Ink.inkOver("neutral", root.onSurface, l, 53); }
    function accentOn(l)  { return Ui.Ink.accentOver("primary", root.accent, l, 55, Ui.Ink.side(l)); }

    // Ink reaches the rest of the config through here. A file that imports the
    // module loses this singleton to QtQuick's own Scheme type, so this is the
    // one place in the config that imports it.
    readonly property var inkTones: Ui.Ink.tones
    readonly property real wallLstar: Ui.Ink.wallLstar
    function lstarAt(nx, ny, nw, nh) { return Ui.Ink.lstarAt(nx, ny, nw, nh); }
    function overLstar(bgL, plate) { return Ui.Ink.overLstar(bgL, plate); }
    // Detail (local contrast) and the calm-region placement helper, so the
    // desktop's `auto` anchor can land a widget on a quiet, tonally-even patch.
    // calmSpot reads Ink's tone map internally, so a binding that calls it
    // re-resolves when a new wallpaper publishes a fresh map.
    function detailAt(nx, ny, nw, nh) { return Ui.Ink.detailAt(nx, ny, nw, nh); }
    function calmSpot(nw, nh, marginN) { return Ui.Ink.calmSpot(nw, nh, marginN); }

    function refreshWall() {
        try {
            const t = wallFile.text();
            root.wall = t && t.length ? (JSON.parse(t) || {}) : {};
        } catch (e) {
            root.wall = {};
        }
    }
    function refreshNamed() {
        var pal = null;
        try {
            const t = shellFile.text();
            if (t) {
                const o = JSON.parse(t);
                if (o && typeof o.themePalette === "object" && o.themePalette !== null)
                    pal = o.themePalette;
            }
        } catch (e) {
            pal = null;
        }
        root.namedScheme = pal;
    }
    function refreshMatch() {
        try {
            const t = themeFile.text();
            const o = t ? JSON.parse(t) : null;
            // Default ON when theme.json is absent or omits the key, matching
            // services/Config.qml and the daemon's matchWallpaperOn: a fresh box
            // follows the wallpaper. Only an explicit false locks it off.
            root.matchWallpaper = (o && typeof o.followWallpaper === "boolean") ? o.followWallpaper : true;
        } catch (e) {
            root.matchWallpaper = true;
        }
    }

    FileView {
        id: wallFile
        path: (Quickshell.env("XDG_CACHE_HOME") || (Quickshell.env("HOME") + "/.cache")) + "/ryoku/colors.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.refreshWall()
    }
    FileView {
        id: previewFile
        path: (Quickshell.env("XDG_RUNTIME_DIR") || "/dev/null") + "/ryoku-wall-preview.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: {
            try { root.preview = JSON.parse(text() || "{}") || {} }
            catch (e) { root.preview = {} }
        }
    }
    FileView {
        id: shellFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/shell.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.refreshNamed()
    }
    FileView {
        id: themeFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/theme.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.refreshMatch()
    }

    Component.onCompleted: {
        refreshWall();
        refreshNamed();
        refreshMatch();
    }
}
