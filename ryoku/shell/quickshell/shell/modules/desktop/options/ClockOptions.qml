pragma ComponentBehavior: Bound
import QtQuick
import ".."
import "../Singletons"
import Ryoku.Ui.Singletons

// Right-click options for Ryoku's native Clock, holding the settings the removed
// Hub Widgets page owned that WidgetMenu's generic sections don't already show:
// the palette accent, the shared widget font, the time-format toggles, the date
// style, and the panel backing. Face, size, opacity, placement, lock and the
// Show-date toggle stay in WidgetMenu's generic rows. Writes land on the same
// widgets.json Config the Hub wrote.
Column {
    id: opts

    // Set by the menu loader to the widget scope; unused here (the native clock
    // keys are fixed) but kept so every options panel shares one API.
    property string widget: ""

    width: parent ? parent.width : 0
    spacing: Theme.s1

    function cap(s) { return (s && s.length > 0) ? s.charAt(0).toUpperCase() + s.slice(1) : s; }
    function cycle(list, cur) { const i = list.indexOf(cur); return list[(i + 1) % list.length]; }
    function accentLabel(v) { return v === "brand" ? I18n.tr("Brand") : v === "mono" ? I18n.tr("Mono") : I18n.tr("Palette"); }

    MenuSection { label: I18n.tr("Clock"); gloss: "時計" }
    MenuRow {
        label: I18n.tr("Accent")
        value: opts.accentLabel(Config.clockAccent)
        closeOnTrigger: false
        onTriggered: Config.set("clockAccent", opts.cycle(["palette", "brand", "mono"], Config.clockAccent))
    }
    MenuTextField {
        // Global across every desktop widget (this is where the Hub kept it);
        // blank falls back to the built-in Space Grotesk.
        label: I18n.tr("Widget font (all widgets)")
        placeholder: I18n.tr("Space Grotesk")
        text: Config.widgetFont
        onCommitted: (v) => Config.set("widgetFont", v)
    }

    MenuSection { label: I18n.tr("Time & date"); gloss: "時刻" }
    MenuRow {
        label: I18n.tr("24-hour clock")
        value: Config.clock24h ? I18n.tr("On") : I18n.tr("Off")
        on: Config.clock24h
        closeOnTrigger: false
        onTriggered: TimeFormat.set24h(!TimeFormat.is24h)
    }
    MenuRow {
        label: I18n.tr("Show seconds")
        value: Config.clockSeconds ? I18n.tr("On") : I18n.tr("Off")
        on: Config.clockSeconds
        closeOnTrigger: false
        onTriggered: Config.toggle("clockSeconds")
    }
    MenuRow {
        // The date on/off is WidgetMenu's generic clock row; its style only
        // matters once the date is shown.
        visible: Config.dateShow
        label: I18n.tr("Date style")
        value: opts.cap(Config.dateDesign)
        closeOnTrigger: false
        onTriggered: Config.set("dateDesign", opts.cycle(["inline", "badge", "stacked"], Config.dateDesign))
    }

    MenuSection { label: I18n.tr("Panel"); gloss: "台座" }
    MenuRow {
        label: I18n.tr("Background")
        value: opts.cap(Config.clockBg)
        closeOnTrigger: false
        onTriggered: Config.set("clockBg", opts.cycle(["none", "card", "glass"], Config.clockBg))
    }
    MenuSlider {
        id: radius
        // Corner radius rounds the card/glass backing; meaningless with no panel.
        visible: Config.clockBg !== "none"
        label: I18n.tr("Corner radius")
        from: 0
        to: 60
        step: 1
        value: Config.clockRadius
        valueText: Math.round(radius.value)
        onMoved: (v) => Config.setLive("clockRadius", Math.round(v))
        onReleased: (v) => Config.set("clockRadius", Math.round(v))
    }
}
