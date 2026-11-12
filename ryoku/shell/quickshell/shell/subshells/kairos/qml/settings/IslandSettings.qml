pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.Ui as RyokuUi
import Ryoku.Ui.Singletons
import "../IslandMetrics.js" as M

Scope {
    id: s

    readonly property var raw: (kairosSetting.value && typeof kairosSetting.value === "object")
        ? kairosSetting.value : ({})

    RyokuUi.ShellSetting {
        id: kairosSetting
        path: "kairos"
        fallback: ({})
    }

    function num(key, fallback) {
        var v = s.raw[key];
        return (typeof v === "number" && isFinite(v)) ? v : fallback;
    }
    function bool(key, fallback) {
        var v = s.raw[key];
        return (typeof v === "boolean") ? v : fallback;
    }
    function clamp(v, lo, hi) { return Math.max(lo, Math.min(hi, v)) }

    readonly property real scale: s.clamp(s.num("scale", 1), 0.8, 1.3)
    readonly property bool clock12h: !TimeFormat.is24h
    readonly property bool clockSeconds: s.bool("clockSeconds", false)
    readonly property bool music: s.bool("music", true)
    readonly property bool tray: s.bool("tray", true)
    readonly property bool showLocation: s.bool("showLocation", true)
    readonly property bool islandOsd: s.bool("islandOsd", true)
    readonly property bool islandNotifications: s.bool("islandNotifications", true)

    // a missing or broken list falls back to the defaults
    readonly property var defaultTiles: [
        { id: "wifi", large: true }, { id: "bluetooth", large: true },
        { id: "dnd", large: false }, { id: "night", large: false }, { id: "power", large: true },
        { id: "mic", large: false }, { id: "awake", large: false },
        { id: "shot", large: false }, { id: "record", large: false },
        { id: "game", large: true }, { id: "clipboard", large: false }, { id: "wallpaper", large: false }
    ]
    readonly property var tiles: {
        var list = s.raw.tiles;
        if (!Array.isArray(list) || list.length === 0)
            return s.defaultTiles;
        var out = [], seen = {};
        for (var i = 0; i < list.length; i++) {
            var t = list[i];
            var id = t && typeof t.id === "string" ? t.id : "";
            if (id.length === 0 || seen[id])
                continue;
            seen[id] = true;
            out.push({ id: id, large: t.large === true });
        }
        return out.length > 0 ? out : s.defaultTiles;
    }
    function setTiles(list) {
        s._queue("kairos.tiles", list);
    }

    // the hover glance: up to five chips of its own, and whether it shows the weather and battery
    readonly property int glanceMax: 5
    readonly property var defaultGlanceTiles: ["wifi", "bluetooth", "dnd", "night", "power"]
    readonly property var glanceTiles: {
        var list = s.raw.glanceTiles;
        if (!Array.isArray(list))
            return s.defaultGlanceTiles;
        var out = [];
        for (var i = 0; i < list.length && out.length < s.glanceMax; i++)
            if (typeof list[i] === "string" && list[i].length > 0 && out.indexOf(list[i]) < 0)
                out.push(list[i]);
        return out;
    }
    function setGlanceTiles(list) {
        s._queue("kairos.glanceTiles", list.slice(0, s.glanceMax));
    }
    readonly property bool glanceWeather: s.bool("glanceWeather", true)
    readonly property bool glanceBattery: s.bool("glanceBattery", true)

    readonly property real topGap: s.clamp(s.num("topGap", M.topGap), 0, 24)
    readonly property real restHeight: M.restHeight * s.scale
    readonly property real restFontSize: M.restFontSize * s.scale
    readonly property real restPadX: M.restPadX * s.scale
    readonly property real hoverWidth: M.hoverWidth * s.scale
    readonly property real hoverHeight: M.hoverHeight * s.scale
    readonly property real hoverRadius: M.hoverRadius * s.scale
    readonly property real hoverFontSize: M.hoverFontSize * s.scale

    function defaults() {
        return {
            scale: 1,
            topGap: M.topGap,
            clockSeconds: false,
            music: true,
            tray: true,
            showLocation: true,
            islandOsd: true,
            islandNotifications: true,
            glanceWeather: true,
            glanceBattery: true
        };
    }

    function set(key, value) {
        if (key === "clock12h") TimeFormat.set24h(!value);
        else s._queue("kairos." + key, value);
    }
    // A shell-wide key (not under "kairos"): the weather location lives in the
    // shared shell store, so it is patched straight.
    function setGlobal(path, value) {
        s._queue(path, value);
    }
    // the tile layouts survive an island reset, the edit page has its own
    function reset() {
        var d = s.defaults();
        if (Array.isArray(s.raw.tiles))
            d.tiles = s.raw.tiles;
        if (Array.isArray(s.raw.glanceTiles))
            d.glanceTiles = s.raw.glanceTiles;
        s._queue("kairos", d);
    }
    function _queue(path, value) {
        ShellSettings.patch(path, value);
    }
}
