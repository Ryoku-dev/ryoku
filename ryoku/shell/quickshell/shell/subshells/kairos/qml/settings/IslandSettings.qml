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
    readonly property bool dateWheel: s.bool("dateWheel", true)
    readonly property bool music: s.bool("music", true)
    readonly property bool tray: s.bool("tray", true)
    readonly property real musicPeek: s.clamp(s.num("musicPeek", M.musicPeekHeight), 70, 140)
    readonly property bool showLocation: s.bool("showLocation", true)

    readonly property real topGap: s.clamp(s.num("topGap", M.topGap), 0, 24)
    readonly property real restHeight: M.restHeight * s.scale
    readonly property real restFontSize: M.restFontSize * s.scale
    readonly property real restPadX: M.restPadX * s.scale
    readonly property real hoverWidth: M.hoverWidth * s.scale
    readonly property real hoverHeight: (s.dateWheel ? M.hoverHeight : M.hoverHeight * 0.7) * s.scale
    readonly property real hoverRadius: M.hoverRadius * s.scale
    readonly property real hoverFontSize: M.hoverFontSize * s.scale
    readonly property real hoverClockCentreY: M.hoverClockCentreY * s.scale
    readonly property real wheelTop: M.wheelTop * s.scale
    readonly property real wheelHeight: M.wheelHeight * s.scale

    function defaults() {
        return {
            scale: 1,
            topGap: M.topGap,
            clockSeconds: false,
            dateWheel: true,
            music: true,
            tray: true,
            musicPeek: M.musicPeekHeight,
            showLocation: true
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
    function reset() {
        s._queue("kairos", s.defaults());
    }
    function _queue(path, value) {
        ShellSettings.patch(path, value);
    }
}
