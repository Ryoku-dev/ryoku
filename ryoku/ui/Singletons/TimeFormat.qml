pragma Singleton
import QtQuick
import Quickshell
import "buspath.js" as BusPath

// One wall-clock preference for the shell, Hub and lockscreen. The daemon is
// the sole writer; its settings topic keeps separate QML processes in sync.
Singleton {
    id: root

    property bool is24h: false
    property string formatLocale: ""
    readonly property var locale: formatLocale
        ? Qt.locale(formatLocale.split(".")[0].replace("-", "_")) : Qt.locale()

    function pattern(seconds = false) {
        return root.is24h ? (seconds ? "HH:mm:ss" : "HH:mm")
                         : (seconds ? "h:mm:ss AP" : "h:mm AP");
    }

    function is24HourPattern(pattern) {
        const unquoted = String(pattern).replace(/'(?:''|[^'])*'/g, "");
        return !/(?:AP|ap|A|a)/.test(unquoted);
    }

    // Keep a custom clock's separators, seconds and quoted text; only its hour
    // cycle belongs to the global preference.
    function withHourCycle(pattern) {
        var hours = false;
        var meridiem = false;
        var result = String(pattern).replace(/'(?:''|[^'])*'|AP|ap|A|a|H+|h+/g, function(token) {
            if (token[0] === "'") return token;
            if (/^[hH]+$/.test(token)) {
                hours = true;
                return root.is24h ? token.toUpperCase() : token.toLowerCase();
            }
            meridiem = true;
            return root.is24h ? "" : token;
        }).trim();
        return hours && !root.is24h && !meridiem ? result + " AP" : result;
    }

    function format(date, seconds = false) {
        return date.toLocaleTimeString(root.locale, root.pattern(seconds));
    }

    function digits(date) {
        const hour = root.is24h ? date.getHours() : (date.getHours() % 12 || 12);
        return String(hour).padStart(2, "0") + String(date.getMinutes()).padStart(2, "0");
    }

    function set24h(value) {
        ShellSettings.patch("general.clock_format_24_h", value === true);
    }

    function load() {
        root.is24h = ShellSettings.get("general.clock_format_24_h", false) === true;
        const fl = ShellSettings.get("formatLocale", "");
        root.formatLocale = typeof fl === "string" ? fl.trim() : "";
    }

    Connections {
        target: ShellSettings
        function onChanged(paths) {
            if (BusPath.touches("general.clock_format_24_h", paths) || BusPath.touches("formatLocale", paths))
                root.load();
        }
    }

    Component.onCompleted: root.load()
}
