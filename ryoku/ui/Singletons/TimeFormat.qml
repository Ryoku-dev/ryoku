pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

// One wall-clock preference for the shell, Hub and lockscreen. The daemon is
// the sole writer; watching its store also keeps separate QML processes in sync.
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

    function set24h(value) {
        ctl.queued += "call settings.patch " + JSON.stringify({
            path: "general.clock_format_24_h", value: value === true
        }) + "\n";
        if (ctl.connected) ctl.flushQueued();
        else ctl.connected = true;
    }

    function load() {
        var data = {};
        try { data = JSON.parse(store.text()) || {}; } catch (e) {}
        root.is24h = !!(data.general && data.general.clock_format_24_h === true);
        root.formatLocale = typeof data.formatLocale === "string" ? data.formatLocale.trim() : "";
    }

    FileView {
        id: store
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/shell.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onLoaded: root.load()
        onFileChanged: reload()
        onLoadFailed: { root.is24h = false; root.formatLocale = ""; }
    }

    Socket {
        id: ctl
        property string queued: ""
        path: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"
        function flushQueued() {
            if (!connected || !queued.length) return;
            write(queued); flush(); queued = "";
        }
        onConnectionStateChanged: if (connected) flushQueued()
    }
}
